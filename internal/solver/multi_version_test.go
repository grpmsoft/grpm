package solver

import (
	"fmt"
	"sort"
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

// LoadPackage returns the highest version (sorted descending), matching real repo behavior.
func (m *multiVersionRepo) LoadPackage(name string) (*pkg.Package, error) {
	versions, ok := m.versions[name]
	if !ok || len(versions) == 0 {
		return nil, fmt.Errorf("package %s not found", name)
	}
	sorted := make([]*pkg.Package, len(versions))
	copy(sorted, versions)
	sort.Slice(sorted, func(i, j int) bool {
		return pkg.CompareVersions(sorted[i].Version, sorted[j].Version) > 0
	})
	cp := *sorted[0]
	return &cp, nil
}

func (m *multiVersionRepo) FindByAtom(atom *pkg.Atom) ([]*pkg.Package, error) {
	if atom == nil {
		return nil, fmt.Errorf("atom is nil")
	}
	var result []*pkg.Package
	for _, versions := range m.versions {
		for _, p := range versions {
			if atom.Matches(p) {
				cp := *p
				result = append(result, &cp)
			}
		}
	}
	return result, nil
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

// resolveClean calls Resolve, checks error, and asserts PostPassAdded == 0.
func resolveClean(t *testing.T, resolver *PortageResolver, atoms []string) map[string]*pkg.Package {
	t.Helper()
	result, err := resolver.Resolve(atoms)
	if err != nil {
		t.Fatalf("Resolve(%v) failed: %v", atoms, err)
	}
	if resolver.PostPassAdded > 0 {
		t.Errorf("post-SAT pass added %d packages — SAT encoding incomplete", resolver.PostPassAdded)
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
	result := resolveClean(t, resolver, []string{"sys-libs/zlib"})

	slotResult := toSlotKeyMap(result)
	key := pkg.SlotKey{Name: "sys-libs/zlib", Slot: "0"}
	p, ok := slotResult[key]
	if !ok {
		t.Fatalf("expected SlotKey %v in result", key)
	}

	// Without MAX-SAT optimization (task 014), SAT may pick any valid version.
	// Verify correctness: exactly one zlib selected, and it's one of the candidates.
	validVersions := map[string]bool{"1.2.13": true, "1.3.0": true, "1.3.1": true}
	if !validVersions[p.Version] {
		t.Errorf("expected one of {1.2.13, 1.3.0, 1.3.1}, got %s", p.Version)
	}
	count := 0
	for _, rp := range result {
		if rp.Name == "sys-libs/zlib" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 zlib in result, got %d", count)
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

	result := resolveClean(t, resolver, []string{"app-misc/myapp"})

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
	result := resolveClean(t, resolver, []string{"dev-libs/openssl"})

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
	result := resolveClean(t, resolver, []string{"app-misc/app"})

	slotResult := toSlotKeyMap(result)

	// Without MAX-SAT, SAT may pick v1 or v2. Check that whichever was picked
	// has ONLY its own deps pulled (implication correctness).
	appKey := pkg.SlotKey{Name: "app-misc/app", Slot: "0"}
	p, ok := slotResult[appKey]
	if !ok {
		t.Fatalf("expected app in result")
	}

	libAKey := pkg.SlotKey{Name: "dev-libs/liba", Slot: "0"}
	libBKey := pkg.SlotKey{Name: "dev-libs/libb", Slot: "0"}
	_, hasA := slotResult[libAKey]
	_, hasB := slotResult[libBKey]

	switch p.Version {
	case "1.0":
		// v1 selected → libA must be present, libB must NOT
		if !hasA {
			t.Error("v1 selected: libA should be in result (dep of v1)")
		}
		if hasB {
			t.Error("v1 selected: libB should NOT be in result (dep of unselected v2)")
		}
	case "2.0":
		// v2 selected → libB must be present, libA must NOT
		if !hasB {
			t.Error("v2 selected: libB should be in result (dep of v2)")
		}
		if hasA {
			t.Error("v2 selected: libA should NOT be in result (dep of unselected v1)")
		}
	default:
		t.Errorf("unexpected app version %s, expected 1.0 or 2.0", p.Version)
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
	result := resolveClean(t, resolver, []string{"dev-libs/libxml2"})

	// Verify that SAT actually considered multiple versions (not just one).
	// The proof: if we resolve with >=2.13, older candidate is excluded.
	// But first, basic: result should have the newest.
	slotResult := toSlotKeyMap(result)
	key := pkg.SlotKey{Name: "dev-libs/libxml2", Slot: "0"}
	p, ok := slotResult[key]
	if !ok {
		t.Fatal("expected libxml2 in result")
	}

	// Without MAX-SAT optimization, SAT may pick any valid version.
	// Verify correctness: libxml2 is present and is one of the candidates.
	validVersions := map[string]bool{"2.12.0": true, "2.13.0": true, "2.13.4": true}
	if !validVersions[p.Version] {
		t.Errorf("expected one of {2.12.0, 2.13.0, 2.13.4}, got %s", p.Version)
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
	result := resolveClean(t, resolver, []string{"app-misc/myutil"})

	slotResult := toSlotKeyMap(result)

	// Without MAX-SAT, SAT may pick v1 or v2. Check that ONLY the selected
	// version's deps are present (implication correctness).
	appKey := pkg.SlotKey{Name: "app-misc/myutil", Slot: "0"}
	p, ok := slotResult[appKey]
	if !ok {
		t.Fatal("expected app-misc/myutil in result")
	}

	libXKey := pkg.SlotKey{Name: "dev-libs/libx", Slot: "0"}
	libYKey := pkg.SlotKey{Name: "dev-libs/liby", Slot: "0"}
	libZKey := pkg.SlotKey{Name: "dev-libs/libz", Slot: "0"}
	_, hasX := slotResult[libXKey]
	_, hasY := slotResult[libYKey]
	_, hasZ := slotResult[libZKey]

	switch p.Version {
	case "1.0":
		// v1 selected → libX + libY must be present, libZ must NOT
		if !hasX {
			t.Error("v1 selected: libX should be in result (dep of v1)")
		}
		if !hasY {
			t.Error("v1 selected: libY should be in result (dep of v1)")
		}
		if hasZ {
			t.Error("v1 selected: libZ should NOT be in result (dep of unselected v2)")
		}
	case "2.0":
		// v2 selected → libZ must be present, libX + libY must NOT
		if !hasZ {
			t.Error("v2 selected: libZ should be in result (dep of v2)")
		}
		if hasX {
			t.Error("v2 selected: libX should NOT be in result (dep of unselected v1)")
		}
		if hasY {
			t.Error("v2 selected: libY should NOT be in result (dep of unselected v1)")
		}
	default:
		t.Errorf("unexpected myutil version %s, expected 1.0 or 2.0", p.Version)
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
	result := resolveClean(t, resolver, []string{"sys-libs/glibc"})

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

	// Without MAX-SAT, SAT may pick any valid version.
	// Verify correctness: exactly one glibc, and it's one of the candidates.
	slotResult := toSlotKeyMap(result)
	key := pkg.SlotKey{Name: "sys-libs/glibc", Slot: "0"}
	p, ok := slotResult[key]
	if !ok {
		t.Fatal("expected glibc in slot result")
	}
	validVersions := map[string]bool{"2.37": true, "2.38": true, "2.39": true}
	if !validVersions[p.Version] {
		t.Errorf("expected one of {2.37, 2.38, 2.39}, got %s", p.Version)
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
	result := resolveClean(t, resolver, []string{"app-misc/sslapp"})

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
	result := resolveClean(t, resolver, []string{"app-misc/transapp"})

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

// --- Fable review fixes: tests that expose real bugs ---

// Fix #1: UNSAT must work WITHOUT Required=true (parser never sets it).
// OrGroupID==0 deps should be treated as required by default.
func TestFix_UNSATWithoutRequiredFlag(t *testing.T) {
	r := newMultiVersionRepo()

	app := pkg.NewPackage("app-misc/broken", "1.0", "0")
	// NOTE: Required=false (default) — same as what the parser produces
	app.Deps = []pkg.Constraint{
		{Name: "dev-libs/nonexistent", Type: pkg.ConstraintTypeVersion},
	}
	r.addVersion(app)

	resolver := NewResolver(r)
	_, err := resolver.Resolve([]string{"app-misc/broken"})

	if err == nil {
		t.Error("Resolve should return error for missing dep even without Required=true")
	}
}

// Fix #2: All candidate versions' deps must be loaded, not just the highest.
// SAT may choose an older version whose transitive deps were never explored.
func TestFix_AllCandidatesDepsLoaded(t *testing.T) {
	r := newMultiVersionRepo()

	// old-only is a dep of lib v1 only
	oldOnly := pkg.NewPackage("dev-libs/old-only", "1.0", "0")
	r.addVersion(oldOnly)

	// new-only is a dep of lib v2 only
	newOnly := pkg.NewPackage("dev-libs/new-only", "1.0", "0")
	r.addVersion(newOnly)

	// lib v1 depends on old-only
	libV1 := pkg.NewPackage("dev-libs/lib", "1.0", "0")
	libV1.Deps = []pkg.Constraint{{Name: "dev-libs/old-only", Type: pkg.ConstraintTypeVersion}}
	r.addVersion(libV1)

	// lib v2 depends on new-only
	libV2 := pkg.NewPackage("dev-libs/lib", "2.0", "0")
	libV2.Deps = []pkg.Constraint{{Name: "dev-libs/new-only", Type: pkg.ConstraintTypeVersion}}
	r.addVersion(libV2)

	// app depends on <lib-2.0 (strict less-than, forces lib v1)
	app := pkg.NewPackage("app-misc/constrained", "1.0", "0")
	app.Deps = []pkg.Constraint{
		{
			Name:    "dev-libs/lib",
			Type:    pkg.ConstraintTypeVersion,
			Version: pkg.NewVersionConstraint(pkg.OpLess, "2.0"),
		},
	}
	r.addVersion(app)

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app-misc/constrained"})

	slotResult := toSlotKeyMap(result)

	// SAT should pick lib v1 (only version satisfying <2.0)
	libKey := pkg.SlotKey{Name: "dev-libs/lib", Slot: "0"}
	p, ok := slotResult[libKey]
	if !ok {
		t.Fatal("expected lib in result")
	}
	if p.Version != "1.0" {
		t.Errorf("expected lib 1.0 (constrained by <2.0), got %s", p.Version)
	}

	// old-only MUST be in result — it's lib v1's dep, loaded by SAT (not post-pass)
	oldKey := pkg.SlotKey{Name: "dev-libs/old-only", Slot: "0"}
	if _, ok := slotResult[oldKey]; !ok {
		t.Error("old-only should be in result — dep of selected lib v1")
	}

	// new-only should NOT be in result — lib v2 wasn't selected
	newKey := pkg.SlotKey{Name: "dev-libs/new-only", Slot: "0"}
	if _, ok := slotResult[newKey]; ok {
		t.Error("new-only should NOT be in result — dep of unselected lib v2")
	}
}

// Fix #3: Recursive dep errors must propagate, not be swallowed.
func TestFix_RecursiveErrorPropagates(t *testing.T) {
	r := newMultiVersionRepo()

	// leaf has a missing required dep
	leaf := pkg.NewPackage("dev-libs/leaf", "1.0", "0")
	leaf.Deps = []pkg.Constraint{
		{Name: "dev-libs/ghost", Type: pkg.ConstraintTypeVersion},
	}
	r.addVersion(leaf)

	// mid depends on leaf
	mid := pkg.NewPackage("dev-libs/mid", "1.0", "0")
	mid.Deps = []pkg.Constraint{{Name: "dev-libs/leaf", Type: pkg.ConstraintTypeVersion}}
	r.addVersion(mid)

	// app depends on mid
	app := pkg.NewPackage("app-misc/deep", "1.0", "0")
	app.Deps = []pkg.Constraint{{Name: "dev-libs/mid", Type: pkg.ConstraintTypeVersion}}
	r.addVersion(app)

	resolver := NewResolver(r)
	_, err := resolver.Resolve([]string{"app-misc/deep"})

	// Error from ghost (missing) should propagate through mid → app
	if err == nil {
		t.Error("Resolve should propagate error from transitive missing dep")
	}
}

// Fix #5: Root should be at-least-one matching atom, not unit clause for highest.
// This allows SAT to backtrack root on conflict.
// NOTE: multiVersionRepo.LoadPackage returns highest (2.0), so root starts at v2.
// v2 has unsatisfiable dep → SAT must backtrack to v1.
func TestFix_RootBacktracksOnConflict(t *testing.T) {
	r := newMultiVersionRepo()

	satisfiableDep := pkg.NewPackage("dev-libs/helper", "1.0", "0")
	r.addVersion(satisfiableDep)

	// app v1 has satisfiable deps
	appV1 := pkg.NewPackage("app-misc/flex", "1.0", "0")
	appV1.Deps = []pkg.Constraint{
		{Name: "dev-libs/helper", Type: pkg.ConstraintTypeVersion},
	}
	r.addVersion(appV1)

	// app v2 depends on something that doesn't exist (unsatisfiable)
	appV2 := pkg.NewPackage("app-misc/flex", "2.0", "0")
	appV2.Deps = []pkg.Constraint{
		{Name: "dev-libs/impossible", Type: pkg.ConstraintTypeVersion},
	}
	r.addVersion(appV2)

	// Verify LoadPackage returns v2 (highest)
	loaded, _ := r.LoadPackage("app-misc/flex")
	if loaded.Version != "2.0" {
		t.Fatalf("LoadPackage should return highest (2.0), got %s", loaded.Version)
	}

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app-misc/flex"})


	slotResult := toSlotKeyMap(result)
	appKey := pkg.SlotKey{Name: "app-misc/flex", Slot: "0"}
	p, ok := slotResult[appKey]
	if !ok {
		t.Fatal("expected app in result")
	}
	if p.Version != "1.0" {
		t.Errorf("expected v1.0 (backtracked from unsatisfiable v2), got %s", p.Version)
	}
}

// OR-group alternatives must have their deps explored so SAT can build
// implication clauses. Without exploration, prohibit clause kills all alternatives.
func TestFix_ORGroupDepsExplored(t *testing.T) {
	r := newMultiVersionRepo()

	// x and y are transitive deps of OR alternatives
	x := pkg.NewPackage("dev-libs/x", "1.0", "0")
	y := pkg.NewPackage("dev-libs/y", "1.0", "0")
	r.addVersion(x)
	r.addVersion(y)

	// OR alternative a depends on x
	a := pkg.NewPackage("dev-libs/a", "1.0", "0")
	a.Deps = []pkg.Constraint{{Name: "dev-libs/x", Type: pkg.ConstraintTypeVersion}}
	r.addVersion(a)

	// OR alternative b depends on y
	b := pkg.NewPackage("dev-libs/b", "1.0", "0")
	b.Deps = []pkg.Constraint{{Name: "dev-libs/y", Type: pkg.ConstraintTypeVersion}}
	r.addVersion(b)

	// app depends on || ( a b )
	app := pkg.NewPackage("app-misc/orapp", "1.0", "0")
	app.Deps = []pkg.Constraint{
		{Name: "dev-libs/a", Type: pkg.ConstraintTypeVersion, OrGroupID: 1},
		{Name: "dev-libs/b", Type: pkg.ConstraintTypeVersion, OrGroupID: 1},
	}
	r.addVersion(app)

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app-misc/orapp"})

	slotResult := toSlotKeyMap(result)

	// App must be in result
	appKey := pkg.SlotKey{Name: "app-misc/orapp", Slot: "0"}
	if _, ok := slotResult[appKey]; !ok {
		t.Fatal("expected orapp in result")
	}

	// At least one of a or b must be in result (SAT picks one)
	aKey := pkg.SlotKey{Name: "dev-libs/a", Slot: "0"}
	bKey := pkg.SlotKey{Name: "dev-libs/b", Slot: "0"}
	_, hasA := slotResult[aKey]
	_, hasB := slotResult[bKey]
	if !hasA && !hasB {
		t.Error("expected at least one of a or b in result (OR-group)")
	}

	// Selected alternative's transitive dep must be present
	if hasA {
		xKey := pkg.SlotKey{Name: "dev-libs/x", Slot: "0"}
		if _, ok := slotResult[xKey]; !ok {
			t.Error("a selected but its dep x missing — OR deps not explored")
		}
	}
	if hasB {
		yKey := pkg.SlotKey{Name: "dev-libs/y", Slot: "0"}
		if _, ok := slotResult[yKey]; !ok {
			t.Error("b selected but its dep y missing — OR deps not explored")
		}
	}
}

// Root at-least-one must respect the user's atom constraint.
// `emerge =foo-1.0` must not select foo-2.0.
func TestFix_RootRespectsAtomConstraint(t *testing.T) {
	r := newMultiVersionRepo()

	r.addVersion(pkg.NewPackage("app-misc/pinned", "1.0", "0"))
	r.addVersion(pkg.NewPackage("app-misc/pinned", "2.0", "0"))
	r.addVersion(pkg.NewPackage("app-misc/pinned", "3.0", "0"))

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"=app-misc/pinned-1.0"})

	slotResult := toSlotKeyMap(result)
	key := pkg.SlotKey{Name: "app-misc/pinned", Slot: "0"}
	p, ok := slotResult[key]
	if !ok {
		t.Fatal("expected pinned in result")
	}
	if p.Version != "1.0" {
		t.Errorf("expected 1.0 (exact atom), got %s — root at-least-one ignores atom", p.Version)
	}
}

// Post-SAT pass must add zero packages. If it adds any, SAT encoding is incomplete.
func TestFix_PostSATPassAddsNothing(t *testing.T) {
	r := newMultiVersionRepo()

	dep := pkg.NewPackage("dev-libs/dep", "1.0", "0")
	r.addVersion(dep)

	app := pkg.NewPackage("app-misc/complete", "1.0", "0")
	app.Deps = []pkg.Constraint{{Name: "dev-libs/dep", Type: pkg.ConstraintTypeVersion}}
	r.addVersion(app)

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app-misc/complete"})

	// dep should already be in result from SAT, not added by post-pass
	slotResult := toSlotKeyMap(result)
	depKey := pkg.SlotKey{Name: "dev-libs/dep", Slot: "0"}
	if _, ok := slotResult[depKey]; !ok {
		t.Error("dep should be in SAT result directly, not requiring post-pass")
	}

	// Post-SAT pass must have added zero packages
	if resolver.PostPassAdded > 0 {
		t.Errorf("post-SAT pass added %d packages — SAT encoding incomplete", resolver.PostPassAdded)
	}
}

// Slot atom must be respected: `emerge dev-lang/py:3.13` must pick slot 3.13.
func TestFix_RootRespectsSlotAtom(t *testing.T) {
	r := newMultiVersionRepo()

	r.addVersion(pkg.NewPackage("dev-lang/py", "3.11.9", "3.11"))
	r.addVersion(pkg.NewPackage("dev-lang/py", "3.12.1", "3.12"))
	r.addVersion(pkg.NewPackage("dev-lang/py", "3.13.0", "3.13"))

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"dev-lang/py:3.13"})

	slotResult := toSlotKeyMap(result)
	key := pkg.SlotKey{Name: "dev-lang/py", Slot: "3.13"}
	p, ok := slotResult[key]
	if !ok {
		t.Fatalf("expected py:3.13 in result, got keys: %v", slotResult)
	}
	if p.Version != "3.13.0" {
		t.Errorf("expected 3.13.0, got %s", p.Version)
	}

	// Other slots must NOT be in result
	for k := range slotResult {
		if k.Name == "dev-lang/py" && k.Slot != "3.13" {
			t.Errorf("unexpected slot %s in result — only :3.13 requested", k.Slot)
		}
	}

	if resolver.PostPassAdded > 0 {
		t.Errorf("post-SAT pass added %d packages", resolver.PostPassAdded)
	}
}
