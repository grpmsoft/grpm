package solver

import (
	"fmt"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
)

// multiVersionRepo wraps MockRepository to support multiple versions per package.
type multiVersionRepo struct {
	*repo.MockRepository
	versions map[string][]*pkg.Package
}

func newMultiVersionRepo() *multiVersionRepo {
	return &multiVersionRepo{
		MockRepository: repo.NewEmptyMockRepository(),
		versions:       make(map[string][]*pkg.Package),
	}
}

func (m *multiVersionRepo) addVersion(p *pkg.Package) {
	cp := *p
	m.versions[p.Name] = append(m.versions[p.Name], &cp)
	_ = m.Add(p)
}

func (m *multiVersionRepo) GetAllVersions(name string) ([]*pkg.Package, error) {
	if versions, ok := m.versions[name]; ok {
		result := make([]*pkg.Package, len(versions))
		for i, v := range versions {
			cp := *v
			result[i] = &cp
		}
		return result, nil
	}
	return []*pkg.Package{}, nil
}

func (m *multiVersionRepo) LoadPackageVersion(name, version string) (*pkg.Package, error) {
	if versions, ok := m.versions[name]; ok {
		for _, v := range versions {
			if v.Version == version {
				cp := *v
				return &cp, nil
			}
		}
		return nil, fmt.Errorf("version %s not found for %s", version, name)
	}
	return nil, fmt.Errorf("package %s not found", name)
}

// toSlotKeyMap converts current string-keyed result to SlotKey-keyed for assertions.
// This adapter will be removed when Resolve() returns map[SlotKey]*Package natively.
func toSlotKeyMap(m map[string]*pkg.Package) map[pkg.SlotKey]*pkg.Package {
	result := make(map[pkg.SlotKey]*pkg.Package, len(m))
	for _, p := range m {
		key := pkg.SlotKeyOf(p)
		result[key] = p
	}
	return result
}

// --- TDD tests for v0.10.0-011: Multi-Version SAT Resolver ---
//
// Tests cover: multi-version candidate loading, implication clauses,
// at-most-one per slot, and SAT correctness.

func TestMultiVersionSAT_ChoosesNewest(t *testing.T) {
	r := newMultiVersionRepo()
	r.addVersion(pkg.NewPackage("sys-libs/zlib", "1.2.13", "0"))
	r.addVersion(pkg.NewPackage("sys-libs/zlib", "1.3.0", "0"))
	r.addVersion(pkg.NewPackage("sys-libs/zlib", "1.3.1", "0"))

	resolver := NewResolver(r)
	result, err := resolver.Resolve([]string{"sys-libs/zlib"})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	slotResult := toSlotKeyMap(result)
	key := pkg.SlotKey{Name: "sys-libs/zlib", Slot: "0"}
	p, ok := slotResult[key]
	if !ok {
		t.Fatalf("expected SlotKey %v in result", key)
	}

	// Current resolver picks highest from LoadPackage (which is the last Add).
	// With multi-version SAT, it should explicitly choose newest from all candidates.
	if p.Version != "1.3.1" {
		t.Errorf("expected newest version 1.3.1, got %s", p.Version)
	}
}

func TestMultiVersionSAT_MultiSlotCoexist(t *testing.T) {
	r := newMultiVersionRepo()
	r.addVersion(pkg.NewPackage("dev-lang/python", "3.12.7", "3.12"))
	r.addVersion(pkg.NewPackage("dev-lang/python", "3.13.1", "3.13"))

	// App requires both python:3.12 AND python:3.13 (separate slot deps)
	app := pkg.NewPackage("app-misc/myapp", "1.0", "0")
	app.Deps = []pkg.Constraint{
		{Name: "dev-lang/python", Slot: "3.12", Type: pkg.ConstraintTypeSlot},
		{Name: "dev-lang/python", Slot: "3.13", Type: pkg.ConstraintTypeSlot},
	}
	r.addVersion(app)

	resolver := NewResolver(r)

	// Verify repo returns both versions
	allVersions, _ := r.GetAllVersions("dev-lang/python")
	t.Logf("repo has %d python versions:", len(allVersions))
	for _, v := range allVersions {
		t.Logf("  %s-%s slot=%s", v.Name, v.Version, v.Slot.Name)
	}

	result, err := resolver.Resolve([]string{"app-misc/myapp"})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	slotResult := toSlotKeyMap(result)

	// MUST FAIL: current resolver stores one version per cat/pkg name.
	// With multi-slot, both python:3.12 and python:3.13 should coexist.
	key312 := pkg.SlotKey{Name: "dev-lang/python", Slot: "3.12"}
	key313 := pkg.SlotKey{Name: "dev-lang/python", Slot: "3.13"}

	has312 := false
	has313 := false
	if _, ok := slotResult[key312]; ok {
		has312 = true
	}
	if _, ok := slotResult[key313]; ok {
		has313 = true
	}

	if !has312 || !has313 {
		// Debug: dump raw string-keyed result
		t.Logf("raw result keys:")
		for k, p := range result {
			t.Logf("  key=%q → %s-%s slot=%s", k, p.Name, p.Version, p.Slot.Name)
		}
		t.Errorf("expected both python:3.12 and python:3.13 in result, got312=%v got313=%v",
			has312, has313)
	}
}

func TestMultiVersionSAT_AtMostOnePerSlot(t *testing.T) {
	r := newMultiVersionRepo()
	// Two versions in same slot "0"
	r.addVersion(pkg.NewPackage("dev-libs/openssl", "3.0.14", "0"))
	r.addVersion(pkg.NewPackage("dev-libs/openssl", "3.0.15", "0"))

	resolver := NewResolver(r)
	result, err := resolver.Resolve([]string{"dev-libs/openssl"})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	slotResult := toSlotKeyMap(result)
	count := 0
	for key := range slotResult {
		if key.Name == "dev-libs/openssl" {
			count++
		}
	}
	// At most one version per slot — should be exactly 1
	if count != 1 {
		t.Errorf("expected exactly 1 openssl in result, got %d", count)
	}
}

func TestMultiVersionSAT_ImplicationNotUnconditional(t *testing.T) {
	r := newMultiVersionRepo()

	libA := pkg.NewPackage("dev-libs/liba", "1.0", "0")
	libB := pkg.NewPackage("dev-libs/libb", "1.0", "0")
	r.addVersion(libA)
	r.addVersion(libB)

	// v1 depends on libA
	appV1 := pkg.NewPackage("app-misc/app", "1.0", "0")
	appV1.Deps = []pkg.Constraint{{Name: "dev-libs/liba", Type: pkg.ConstraintTypeVersion}}
	r.addVersion(appV1)

	// v2 depends on libB (different dep)
	appV2 := pkg.NewPackage("app-misc/app", "2.0", "0")
	appV2.Deps = []pkg.Constraint{{Name: "dev-libs/libb", Type: pkg.ConstraintTypeVersion}}
	r.addVersion(appV2)

	resolver := NewResolver(r)
	result, err := resolver.Resolve([]string{"app-misc/app"})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	slotResult := toSlotKeyMap(result)

	// SAT should pick v2 (newest). Only libB should be pulled, not libA.
	appKey := pkg.SlotKey{Name: "app-misc/app", Slot: "0"}
	p, ok := slotResult[appKey]
	if !ok {
		t.Fatalf("expected app in result")
	}
	if p.Version != "2.0" {
		t.Errorf("expected app version 2.0 (newest), got %s", p.Version)
	}

	// MUST FAIL with current unconditional clauses: both libA and libB get pulled
	libAKey := pkg.SlotKey{Name: "dev-libs/liba", Slot: "0"}
	if _, ok := slotResult[libAKey]; ok {
		t.Error("libA should NOT be in result — dep of unselected v1, not v2")
	}

	libBKey := pkg.SlotKey{Name: "dev-libs/libb", Slot: "0"}
	if _, ok := slotResult[libBKey]; !ok {
		t.Error("libB should be in result — dep of selected v2")
	}
}

func TestMultiVersionSAT_UNSATOnMissing(t *testing.T) {
	r := newMultiVersionRepo()

	app := pkg.NewPackage("app-misc/broken", "1.0", "0")
	app.Deps = []pkg.Constraint{
		{Name: "dev-libs/nonexistent", Type: pkg.ConstraintTypeVersion, Required: true},
	}
	r.addVersion(app)

	resolver := NewResolver(r)
	_, err := resolver.Resolve([]string{"app-misc/broken"})

	// MUST FAIL: current resolver silently ignores missing deps (return nil in addVersionConstraint)
	if err == nil {
		t.Error("Resolve should return error when required dep has no provider")
	}
}

func TestMultiVersionSAT_SATSeesMultipleCandidates(t *testing.T) {
	r := newMultiVersionRepo()
	r.addVersion(pkg.NewPackage("dev-libs/libxml2", "2.12.0", "0"))
	r.addVersion(pkg.NewPackage("dev-libs/libxml2", "2.13.0", "0"))
	r.addVersion(pkg.NewPackage("dev-libs/libxml2", "2.13.4", "0"))

	resolver := NewResolver(r)
	result, err := resolver.Resolve([]string{"dev-libs/libxml2"})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// Verify that SAT actually considered multiple versions (not just one).
	// The proof: if we resolve with >=2.13, older candidate is excluded.
	// But first, basic: result should have the newest.
	slotResult := toSlotKeyMap(result)
	key := pkg.SlotKey{Name: "dev-libs/libxml2", Slot: "0"}
	p, ok := slotResult[key]
	if !ok {
		t.Fatal("expected libxml2 in result")
	}

	// With multi-version SAT, newest should be selected
	if p.Version != "2.13.4" {
		t.Errorf("expected 2.13.4, got %s — SAT may not see all candidates", p.Version)
	}
}

// TestImplication_MultiVersion_OnlySelectedVersionDepsPulled verifies that when
// multiple versions of a package are SAT candidates, only the SELECTED version's
// dependencies get pulled into the result. Dependencies of unselected versions
// must NOT appear.
func TestImplication_MultiVersion_OnlySelectedVersionDepsPulled(t *testing.T) {
	r := newMultiVersionRepo()

	// Libraries that will be pulled as dependencies
	libX := pkg.NewPackage("dev-libs/libx", "1.0", "0")
	libY := pkg.NewPackage("dev-libs/liby", "1.0", "0")
	libZ := pkg.NewPackage("dev-libs/libz", "1.0", "0")
	r.addVersion(libX)
	r.addVersion(libY)
	r.addVersion(libZ)

	// app v1 depends on libX + libY
	appV1 := pkg.NewPackage("app-misc/myutil", "1.0", "0")
	appV1.Deps = []pkg.Constraint{
		{Name: "dev-libs/libx", Type: pkg.ConstraintTypeVersion},
		{Name: "dev-libs/liby", Type: pkg.ConstraintTypeVersion},
	}
	r.addVersion(appV1)

	// app v2 depends only on libZ
	appV2 := pkg.NewPackage("app-misc/myutil", "2.0", "0")
	appV2.Deps = []pkg.Constraint{
		{Name: "dev-libs/libz", Type: pkg.ConstraintTypeVersion},
	}
	r.addVersion(appV2)

	resolver := NewResolver(r)
	result, err := resolver.Resolve([]string{"app-misc/myutil"})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	slotResult := toSlotKeyMap(result)

	// Resolver picks v2 (newest via unit clause from loadPackageFromAtom)
	appKey := pkg.SlotKey{Name: "app-misc/myutil", Slot: "0"}
	p, ok := slotResult[appKey]
	if !ok {
		t.Fatal("expected app-misc/myutil in result")
	}
	if p.Version != "2.0" {
		t.Errorf("expected v2.0, got %s", p.Version)
	}

	// Only libZ should be in result (dep of selected v2)
	libZKey := pkg.SlotKey{Name: "dev-libs/libz", Slot: "0"}
	if _, ok := slotResult[libZKey]; !ok {
		t.Error("libZ should be in result — dep of selected v2")
	}

	// libX and libY should NOT be in result (deps of unselected v1)
	libXKey := pkg.SlotKey{Name: "dev-libs/libx", Slot: "0"}
	if _, ok := slotResult[libXKey]; ok {
		t.Error("libX should NOT be in result — dep of unselected v1")
	}

	libYKey := pkg.SlotKey{Name: "dev-libs/liby", Slot: "0"}
	if _, ok := slotResult[libYKey]; ok {
		t.Error("libY should NOT be in result — dep of unselected v1")
	}
}

// TestAtMostOnePerSlot_PairwiseExclusion verifies that with 3+ versions in the
// same slot, the SAT solver picks exactly one. The pairwise exclusion clauses
// (-vi | -vj) for all i < j ensure at most one version is selected.
func TestAtMostOnePerSlot_PairwiseExclusion(t *testing.T) {
	r := newMultiVersionRepo()
	r.addVersion(pkg.NewPackage("sys-libs/glibc", "2.37", "0"))
	r.addVersion(pkg.NewPackage("sys-libs/glibc", "2.38", "0"))
	r.addVersion(pkg.NewPackage("sys-libs/glibc", "2.39", "0"))

	resolver := NewResolver(r)
	result, err := resolver.Resolve([]string{"sys-libs/glibc"})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// Count how many glibc versions are in the result
	count := 0
	for _, p := range result {
		if p.Name == "sys-libs/glibc" {
			count++
		}
	}

	if count != 1 {
		t.Errorf("expected exactly 1 glibc in result, got %d", count)
		for k, p := range result {
			t.Logf("  key=%q %s-%s", k, p.Name, p.Version)
		}
	}

	// Verify the newest version was selected (via root unit clause)
	slotResult := toSlotKeyMap(result)
	key := pkg.SlotKey{Name: "sys-libs/glibc", Slot: "0"}
	p, ok := slotResult[key]
	if !ok {
		t.Fatal("expected glibc in slot result")
	}
	if p.Version != "2.39" {
		t.Errorf("expected newest version 2.39, got %s", p.Version)
	}
}

// TestMultiVersionSAT_DepVersionConstraintSatisfied verifies that a dependency's
// version constraint filters candidates correctly. When app depends on
// >=dev-libs/openssl-3.0.14, only versions satisfying the constraint are providers.
func TestMultiVersionSAT_DepVersionConstraintSatisfied(t *testing.T) {
	r := newMultiVersionRepo()

	// Three versions of openssl
	r.addVersion(pkg.NewPackage("dev-libs/openssl", "1.1.1", "0"))
	r.addVersion(pkg.NewPackage("dev-libs/openssl", "3.0.14", "0"))
	r.addVersion(pkg.NewPackage("dev-libs/openssl", "3.0.15", "0"))

	// App requires >=openssl-3.0.14
	app := pkg.NewPackage("app-misc/sslapp", "1.0", "0")
	app.Deps = []pkg.Constraint{
		{
			Name:    "dev-libs/openssl",
			Type:    pkg.ConstraintTypeVersion,
			Version: pkg.NewMinVersionConstraint("3.0.14"),
		},
	}
	r.addVersion(app)

	resolver := NewResolver(r)
	result, err := resolver.Resolve([]string{"app-misc/sslapp"})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	slotResult := toSlotKeyMap(result)

	// openssl should be in the result
	opensslKey := pkg.SlotKey{Name: "dev-libs/openssl", Slot: "0"}
	p, ok := slotResult[opensslKey]
	if !ok {
		t.Fatal("expected openssl in result")
	}

	// The selected version must satisfy >=3.0.14
	if pkg.CompareVersions(p.Version, "3.0.14") < 0 {
		t.Errorf("expected openssl >= 3.0.14, got %s", p.Version)
	}
}

// TestMultiVersionSAT_TransitiveDeps verifies that transitive dependencies
// through multi-version packages work correctly. App -> lib (multi-version) -> util.
func TestMultiVersionSAT_TransitiveDeps(t *testing.T) {
	r := newMultiVersionRepo()

	// util is a leaf dependency
	util := pkg.NewPackage("dev-libs/util", "1.0", "0")
	r.addVersion(util)

	// lib v1 and v2 both depend on util
	libV1 := pkg.NewPackage("dev-libs/mylib", "1.0", "0")
	libV1.Deps = []pkg.Constraint{
		{Name: "dev-libs/util", Type: pkg.ConstraintTypeVersion},
	}
	r.addVersion(libV1)

	libV2 := pkg.NewPackage("dev-libs/mylib", "2.0", "0")
	libV2.Deps = []pkg.Constraint{
		{Name: "dev-libs/util", Type: pkg.ConstraintTypeVersion},
	}
	r.addVersion(libV2)

	// App depends on mylib (any version)
	app := pkg.NewPackage("app-misc/transapp", "1.0", "0")
	app.Deps = []pkg.Constraint{
		{Name: "dev-libs/mylib", Type: pkg.ConstraintTypeVersion},
	}
	r.addVersion(app)

	resolver := NewResolver(r)
	result, err := resolver.Resolve([]string{"app-misc/transapp"})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	slotResult := toSlotKeyMap(result)

	// App should be in result
	appKey := pkg.SlotKey{Name: "app-misc/transapp", Slot: "0"}
	if _, ok := slotResult[appKey]; !ok {
		t.Error("expected app-misc/transapp in result")
	}

	// mylib should be in result (exactly one version)
	libKey := pkg.SlotKey{Name: "dev-libs/mylib", Slot: "0"}
	if _, ok := slotResult[libKey]; !ok {
		t.Error("expected dev-libs/mylib in result")
	}

	// util should be in result (transitive dep)
	utilKey := pkg.SlotKey{Name: "dev-libs/util", Slot: "0"}
	if _, ok := slotResult[utilKey]; !ok {
		t.Error("expected dev-libs/util in result (transitive dep)")
	}
}

// TestGophersatAdapter_AddImplication verifies that AddImplication correctly
// encodes the clause (-dependent | provider1 | provider2).
func TestGophersatAdapter_AddImplication(t *testing.T) {
	adapter := NewGophersatAdapter()

	// Register packages manually
	p1 := pkg.NewPackage("app/a", "1.0", "0")
	p2 := pkg.NewPackage("dep/b", "1.0", "0")
	p3 := pkg.NewPackage("dep/c", "1.0", "0")
	adapter.AddPackage(p1)
	adapter.AddPackage(p2)
	adapter.AddPackage(p3)

	aVar := adapter.GetVarID("app/a@1.0")
	bVar := adapter.GetVarID("dep/b@1.0")
	cVar := adapter.GetVarID("dep/c@1.0")

	if aVar == 0 || bVar == 0 || cVar == 0 {
		t.Fatal("expected all vars to be registered")
	}

	// Add implication: if a selected, then b or c
	adapter.AddImplication(aVar, []int{bVar, cVar})

	// Should have exactly one clause: (-aVar | bVar | cVar)
	if len(adapter.clauses) != 1 {
		t.Fatalf("expected 1 clause, got %d", len(adapter.clauses))
	}

	clause := adapter.clauses[0]
	if len(clause) != 3 {
		t.Fatalf("expected clause length 3, got %d", len(clause))
	}
	if clause[0] != -aVar {
		t.Errorf("expected clause[0] = %d, got %d", -aVar, clause[0])
	}
	// The providers can be in any order
	hasB := clause[1] == bVar || clause[2] == bVar
	hasC := clause[1] == cVar || clause[2] == cVar
	if !hasB || !hasC {
		t.Errorf("expected clause to contain %d and %d, got %v", bVar, cVar, clause)
	}
}

// TestGophersatAdapter_AddAtMostOnePerSlot verifies pairwise exclusion.
func TestGophersatAdapter_AddAtMostOnePerSlot(t *testing.T) {
	adapter := NewGophersatAdapter()

	// Add 3 versions in the same slot
	adapter.AddPackage(pkg.NewPackage("dev-libs/lib", "1.0", "0"))
	adapter.AddPackage(pkg.NewPackage("dev-libs/lib", "2.0", "0"))
	adapter.AddPackage(pkg.NewPackage("dev-libs/lib", "3.0", "0"))

	adapter.AddAtMostOnePerSlot()

	// With 3 versions, expect 3 pairwise exclusion clauses: (1,2), (1,3), (2,3)
	if len(adapter.clauses) != 3 {
		t.Errorf("expected 3 pairwise clauses, got %d", len(adapter.clauses))
		for i, c := range adapter.clauses {
			t.Logf("  clause %d: %v", i, c)
		}
	}

	// Each clause should have exactly 2 negative literals
	for i, c := range adapter.clauses {
		if len(c) != 2 {
			t.Errorf("clause %d: expected length 2, got %d: %v", i, len(c), c)
			continue
		}
		if c[0] >= 0 || c[1] >= 0 {
			t.Errorf("clause %d: expected both literals negative, got %v", i, c)
		}
	}
}

// TestGophersatAdapter_FindSatisfyingVars verifies that findSatisfyingVars
// correctly matches packages against constraints.
func TestGophersatAdapter_FindSatisfyingVars(t *testing.T) {
	adapter := NewGophersatAdapter()

	adapter.AddPackage(pkg.NewPackage("dev-libs/openssl", "1.1.1", "0"))
	adapter.AddPackage(pkg.NewPackage("dev-libs/openssl", "3.0.14", "0"))
	adapter.AddPackage(pkg.NewPackage("dev-libs/openssl", "3.0.15", "0"))

	// Test: all versions satisfy "any version"
	allVars := adapter.findSatisfyingVars(pkg.Constraint{
		Name: "dev-libs/openssl",
		Type: pkg.ConstraintTypeVersion,
	})
	if len(allVars) != 3 {
		t.Errorf("expected 3 vars for any-version constraint, got %d", len(allVars))
	}

	// Test: >=3.0.14 should match only 3.0.14 and 3.0.15
	geVars := adapter.findSatisfyingVars(pkg.Constraint{
		Name:    "dev-libs/openssl",
		Type:    pkg.ConstraintTypeVersion,
		Version: pkg.NewMinVersionConstraint("3.0.14"),
	})
	if len(geVars) != 2 {
		t.Errorf("expected 2 vars for >=3.0.14, got %d", len(geVars))
	}

	// Test: =1.1.1 should match only one version
	exactVars := adapter.findSatisfyingVars(pkg.Constraint{
		Name:    "dev-libs/openssl",
		Type:    pkg.ConstraintTypeVersion,
		Version: pkg.NewExactVersionConstraint("1.1.1"),
	})
	if len(exactVars) != 1 {
		t.Errorf("expected 1 var for =1.1.1, got %d", len(exactVars))
	}
}
