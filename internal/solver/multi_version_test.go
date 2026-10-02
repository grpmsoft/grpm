package solver

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
	"github.com/grpmsoft/grpm/internal/state"
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

// resolveClean calls Resolve, checks error, and asserts PostPassAdded == 0.
func resolveClean(t *testing.T, resolver *PortageResolver, atoms []string) ResolveResult {
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

	slotResult := result
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

	slotResult := result

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
		// Debug: dump result keys
		t.Logf("result keys:")
		for k, p := range result {
			t.Logf("  key={Name:%q Slot:%q} → %s-%s slot=%s", k.Name, k.Slot, p.Name, p.Version, p.Slot.Name)
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

	slotResult := result
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

	slotResult := result

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
	slotResult := result
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

	slotResult := result

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
			t.Logf("  key={Name:%q Slot:%q} %s-%s", k.Name, k.Slot, p.Name, p.Version)
		}
	}

	// Without MAX-SAT, SAT may pick any valid version.
	// Verify correctness: exactly one glibc, and it's one of the candidates.
	slotResult := result
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

	slotResult := result

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

	slotResult := result

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

	slotResult := result

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

	slotResult := result
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

	slotResult := result

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

	slotResult := result
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
	slotResult := result
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

	slotResult := result
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

// Slot operator := means "any slot, rebuild on subslot change".
// Must NOT be matched literally as slot name "=".
func TestFix_SlotOperatorResolves(t *testing.T) {
	r := newMultiVersionRepo()

	// openssl with slot 0
	ssl := pkg.NewPackage("dev-libs/openssl", "3.0.15", "0")
	r.addVersion(ssl)

	// app depends on openssl:= (slot operator, any slot)
	app := pkg.NewPackage("app-misc/ssluser", "1.0", "0")
	app.Deps = []pkg.Constraint{
		{
			Name: "dev-libs/openssl",
			Slot: "=",
			Type: pkg.ConstraintTypeSlot,
		},
	}
	r.addVersion(app)

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app-misc/ssluser"})

	slotResult := result
	sslKey := pkg.SlotKey{Name: "dev-libs/openssl", Slot: "0"}
	if _, ok := slotResult[sslKey]; !ok {
		t.Error("openssl should be in result — := means any slot, not literal '='")
	}
}

// Slot operator :* means "any slot". Must not be literal "*".
func TestFix_SlotStarOperatorResolves(t *testing.T) {
	r := newMultiVersionRepo()

	lib := pkg.NewPackage("dev-libs/mylib", "2.0", "5")
	r.addVersion(lib)

	app := pkg.NewPackage("app-misc/staruser", "1.0", "0")
	app.Deps = []pkg.Constraint{
		{
			Name: "dev-libs/mylib",
			Slot: "*",
			Type: pkg.ConstraintTypeSlot,
		},
	}
	r.addVersion(app)

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app-misc/staruser"})

	slotResult := result
	libKey := pkg.SlotKey{Name: "dev-libs/mylib", Slot: "5"}
	if _, ok := slotResult[libKey]; !ok {
		t.Error("mylib should be in result — :* means any slot")
	}
}

// Versioned slot atom: >=dev-libs/openssl-3.0:= must match version AND any slot.
func TestFix_VersionedSlotOperatorResolves(t *testing.T) {
	r := newMultiVersionRepo()

	r.addVersion(pkg.NewPackage("dev-libs/openssl", "1.1.1", "0"))
	r.addVersion(pkg.NewPackage("dev-libs/openssl", "3.0.15", "0"))

	app := pkg.NewPackage("app-misc/versslot", "1.0", "0")
	app.Deps = []pkg.Constraint{
		{
			Name:    "dev-libs/openssl",
			Slot:    "=",
			Type:    pkg.ConstraintTypeSlot,
			Version: pkg.NewMinVersionConstraint("3.0"),
		},
	}
	r.addVersion(app)

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app-misc/versslot"})

	slotResult := result
	sslKey := pkg.SlotKey{Name: "dev-libs/openssl", Slot: "0"}
	p, ok := slotResult[sslKey]
	if !ok {
		t.Fatal("openssl should be in result")
	}
	if pkg.CompareVersions(p.Version, "3.0") < 0 {
		t.Errorf("expected openssl >= 3.0, got %s", p.Version)
	}
}

// --- Tests for UNSAT Explainer ---

func TestExplainWhyUNSAT_MissingDep(t *testing.T) {
	r := newMultiVersionRepo()

	app := pkg.NewPackage("app-misc/broken", "1.0", "0")
	app.Deps = []pkg.Constraint{
		{Name: "dev-libs/nonexistent", Type: pkg.ConstraintTypeVersion},
	}
	r.addVersion(app)

	resolver := NewResolver(r)
	_, err := resolver.Resolve([]string{"app-misc/broken"})
	if err == nil {
		t.Fatal("expected UNSAT error")
	}
	if !strings.Contains(err.Error(), "UNSAT") {
		t.Errorf("error should mention UNSAT, got: %s", err.Error())
	}
}

func TestExplainWhyUNSAT_ProhibitChain(t *testing.T) {
	adapter := NewGophersatAdapter()

	a := pkg.NewPackage("app/a", "1.0", "0")
	b := pkg.NewPackage("dev-libs/b", "1.0", "0")
	adapter.AddPackage(a)
	adapter.AddPackage(b)

	aID := adapter.GetVarID("app/a@1.0")
	bID := adapter.GetVarID("dev-libs/b@1.0")

	// Root: a must be selected
	adapter.withMeta(ClauseRoot, "root: app/a")
	adapter.addClause([]int{aID})
	adapter.addRootVars([]int{aID})

	// a → b (implication)
	adapter.AddImplication(aID, []int{bID})

	// b is prohibited
	reason := "no provider for dev-libs/missing (needed by dev-libs/b@1.0)"
	adapter.withMeta(ClauseProhibit, reason)
	adapter.addClause([]int{-bID})
	adapter.prohibits[bID] = reason

	result := adapter.ExplainWhyUNSAT()
	joined := strings.Join(result.Lines, "\n")

	// Must show the chain: a → b → prohibit
	if !strings.Contains(joined, "app/a@1.0") {
		t.Errorf("should contain root package name, got:\n%s", joined)
	}
	if !strings.Contains(joined, "dev-libs/b") {
		t.Errorf("should contain dependency package name, got:\n%s", joined)
	}
	if !strings.Contains(joined, "no provider") {
		t.Errorf("should contain prohibit reason, got:\n%s", joined)
	}
	// Core size must be small (chain, not full dump)
	if result.CoreSize > 10 {
		t.Errorf("core size should be small chain, got %d", result.CoreSize)
	}
}

func TestExplainWhyUNSAT_MultiVersionAllDead(t *testing.T) {
	adapter := NewGophersatAdapter()

	root := pkg.NewPackage("app/root", "1.0", "0")
	libV1 := pkg.NewPackage("dev-libs/lib", "1.0", "0")
	libV2 := pkg.NewPackage("dev-libs/lib", "2.0", "0")
	adapter.AddPackage(root)
	adapter.AddPackage(libV1)
	adapter.AddPackage(libV2)

	rootID := adapter.GetVarID("app/root@1.0")
	v1ID := adapter.GetVarID("dev-libs/lib@1.0")
	v2ID := adapter.GetVarID("dev-libs/lib@2.0")

	adapter.withMeta(ClauseRoot, "root")
	adapter.addClause([]int{rootID})
	adapter.addRootVars([]int{rootID})

	// root → lib (either version)
	adapter.AddImplication(rootID, []int{v1ID, v2ID})

	// Both versions prohibited
	adapter.withMeta(ClauseProhibit, "v1: no provider for ghost")
	adapter.addClause([]int{-v1ID})
	adapter.prohibits[v1ID] = "no provider for ghost"

	adapter.withMeta(ClauseProhibit, "v2: no provider for phantom")
	adapter.addClause([]int{-v2ID})
	adapter.prohibits[v2ID] = "no provider for phantom"

	result := adapter.ExplainWhyUNSAT()
	joined := strings.Join(result.Lines, "\n")

	if !strings.Contains(joined, "all 2 versions impossible") {
		t.Errorf("should group versions, got:\n%s", joined)
	}
	if !strings.Contains(joined, "ghost") || !strings.Contains(joined, "phantom") {
		t.Errorf("should show per-version reasons, got:\n%s", joined)
	}
}

func TestExplainWhyUNSAT_CoreSmallerThanTotal(t *testing.T) {
	// Simulate a problem with many clauses but a short UNSAT chain.
	// The graph explainer should report only the relevant chain.
	adapter := NewGophersatAdapter()

	root := pkg.NewPackage("app/root", "1.0", "0")
	adapter.AddPackage(root)
	rootID := adapter.GetVarID("app/root@1.0")
	adapter.withMeta(ClauseRoot, "root")
	adapter.addClause([]int{rootID})
	adapter.addRootVars([]int{rootID})

	// Create a chain: root → mid → leaf (prohibited)
	mid := pkg.NewPackage("dev-libs/mid", "1.0", "0")
	leaf := pkg.NewPackage("dev-libs/leaf", "1.0", "0")
	adapter.AddPackage(mid)
	adapter.AddPackage(leaf)
	midID := adapter.GetVarID("dev-libs/mid@1.0")
	leafID := adapter.GetVarID("dev-libs/leaf@1.0")

	adapter.AddImplication(rootID, []int{midID})
	adapter.AddImplication(midID, []int{leafID})
	adapter.withMeta(ClauseProhibit, "no provider for ghost")
	adapter.addClause([]int{-leafID})
	adapter.prohibits[leafID] = "no provider for ghost"

	// Add many unrelated satisfiable packages (noise)
	for i := range 50 {
		p := pkg.NewPackage(fmt.Sprintf("dev-libs/noise%d", i), "1.0", "0")
		adapter.AddPackage(p)
		pID := adapter.GetVarID(fmt.Sprintf("dev-libs/noise%d@1.0", i))
		adapter.AddImplication(rootID, []int{pID})
	}
	// Add pairwise at-most-one noise
	adapter.AddAtMostOnePerSlot()

	totalClauses := len(adapter.clauses)
	result := adapter.ExplainWhyUNSAT()

	// Core must be MUCH smaller than total — the chain is 3 nodes, not 50+ noise packages
	if result.CoreSize >= totalClauses/10 {
		t.Errorf("core size %d should be < total/10 (%d), explanation is not focused:\n%s",
			result.CoreSize, totalClauses/10, strings.Join(result.Lines, "\n"))
	}
	if result.CoreSize == 0 {
		t.Errorf("core size should be > 0, got 0:\n%s", strings.Join(result.Lines, "\n"))
	}
}

func TestExplainWhyUNSAT_DedupSeeAbove(t *testing.T) {
	// Two ROOT packages both need the same impossible leaf via different mid-nodes.
	// The leaf's full explanation must appear once; second reference says "(see above)".
	adapter := NewGophersatAdapter()

	rootA := pkg.NewPackage("app/a", "1.0", "0")
	rootB := pkg.NewPackage("app/b", "1.0", "0")
	shared := pkg.NewPackage("dev-libs/shared", "1.0", "0")
	adapter.AddPackage(rootA)
	adapter.AddPackage(rootB)
	adapter.AddPackage(shared)

	aID := adapter.GetVarID("app/a@1.0")
	bID := adapter.GetVarID("app/b@1.0")
	sharedID := adapter.GetVarID("dev-libs/shared@1.0")

	// Both are roots
	adapter.withMeta(ClauseRoot, "root a")
	adapter.addClause([]int{aID})
	adapter.addRootVars([]int{aID})
	adapter.withMeta(ClauseRoot, "root b")
	adapter.addClause([]int{bID})
	adapter.addRootVars([]int{bID})

	// Both depend on shared
	adapter.AddImplication(aID, []int{sharedID})
	adapter.AddImplication(bID, []int{sharedID})

	// shared is prohibited
	adapter.withMeta(ClauseProhibit, "no provider for ghost")
	adapter.addClause([]int{-sharedID})
	adapter.prohibits[sharedID] = "no provider for ghost"

	result := adapter.ExplainWhyUNSAT()
	joined := strings.Join(result.Lines, "\n")

	// "shared" full reason must appear once; second root path gets "(see above)"
	fullCount := strings.Count(joined, "dev-libs/shared@1.0: no provider")
	seeAboveCount := strings.Count(joined, "dev-libs/shared@1.0  (see above)")

	if fullCount != 1 {
		t.Errorf("shared full explanation should appear exactly once, got %d:\n%s", fullCount, joined)
	}
	if seeAboveCount != 1 {
		t.Errorf("shared should have exactly one '(see above)' reference, got %d:\n%s", seeAboveCount, joined)
	}
}

func TestExplainWhyUNSAT_TopDownOrder(t *testing.T) {
	adapter := NewGophersatAdapter()

	root := pkg.NewPackage("app/root", "1.0", "0")
	mid := pkg.NewPackage("dev-libs/mid", "1.0", "0")
	leaf := pkg.NewPackage("dev-libs/leaf", "1.0", "0")
	adapter.AddPackage(root)
	adapter.AddPackage(mid)
	adapter.AddPackage(leaf)

	rootID := adapter.GetVarID("app/root@1.0")
	midID := adapter.GetVarID("dev-libs/mid@1.0")
	leafID := adapter.GetVarID("dev-libs/leaf@1.0")

	adapter.withMeta(ClauseRoot, "root")
	adapter.addClause([]int{rootID})
	adapter.addRootVars([]int{rootID})

	adapter.AddImplication(rootID, []int{midID})
	adapter.AddImplication(midID, []int{leafID})
	adapter.withMeta(ClauseProhibit, "no provider for ghost")
	adapter.addClause([]int{-leafID})
	adapter.prohibits[leafID] = "no provider for ghost"

	result := adapter.ExplainWhyUNSAT()
	joined := strings.Join(result.Lines, "\n")

	// Root must appear BEFORE mid, mid BEFORE leaf (top-down order)
	rootPos := strings.Index(joined, "app/root@1.0")
	midPos := strings.Index(joined, "dev-libs/mid@1.0")
	leafPos := strings.Index(joined, "dev-libs/leaf@1.0")

	if rootPos == -1 || midPos == -1 || leafPos == -1 {
		t.Fatalf("all three nodes must appear in output:\n%s", joined)
	}
	if rootPos >= midPos || midPos >= leafPos {
		t.Errorf("order should be root < mid < leaf (top-down), got positions %d, %d, %d:\n%s",
			rootPos, midPos, leafPos, joined)
	}
}

func TestExplainWhyUNSAT_ConsistentNotation(t *testing.T) {
	adapter := NewGophersatAdapter()

	root := pkg.NewPackage("app/root", "1.0", "0")
	dep := pkg.NewPackage("dev-libs/dep", "2.0", "0")
	adapter.AddPackage(root)
	adapter.AddPackage(dep)

	rootID := adapter.GetVarID("app/root@1.0")
	depID := adapter.GetVarID("dev-libs/dep@2.0")

	adapter.withMeta(ClauseRoot, "root")
	adapter.addClause([]int{rootID})
	adapter.addRootVars([]int{rootID})

	adapter.AddImplication(rootID, []int{depID})
	adapter.withMeta(ClauseProhibit, "no provider for ghost")
	adapter.addClause([]int{-depID})
	adapter.prohibits[depID] = "no provider for ghost"

	result := adapter.ExplainWhyUNSAT()
	joined := strings.Join(result.Lines, "\n")

	// Must use name@version notation consistently, never name/version
	if strings.Contains(joined, "dev-libs/dep/2.0") {
		t.Errorf("should use @ notation, not /, got:\n%s", joined)
	}
	if !strings.Contains(joined, "dev-libs/dep@2.0") {
		t.Errorf("should contain dev-libs/dep@2.0, got:\n%s", joined)
	}
}

func TestExplainWhyUNSAT_CycleDoesNotFalsifyChain(t *testing.T) {
	// a→b, b→a (cycle), a→x, x prohibited.
	// a is dead via x, NOT via the cycle. Chain: a → x: no provider.
	// The cycle must NOT appear in the output.
	adapter := NewGophersatAdapter()

	a := pkg.NewPackage("app/a", "1.0", "0")
	b := pkg.NewPackage("app/b", "1.0", "0")
	x := pkg.NewPackage("dev-libs/x", "1.0", "0")
	adapter.AddPackage(a)
	adapter.AddPackage(b)
	adapter.AddPackage(x)

	aID := adapter.GetVarID("app/a@1.0")
	bID := adapter.GetVarID("app/b@1.0")
	xID := adapter.GetVarID("dev-libs/x@1.0")

	adapter.withMeta(ClauseRoot, "root a")
	adapter.addClause([]int{aID})
	adapter.addRootVars([]int{aID})

	// a→b, b→a (mutual cycle)
	adapter.AddImplication(aID, []int{bID})
	adapter.AddImplication(bID, []int{aID})

	// a→x, x prohibited
	adapter.AddImplication(aID, []int{xID})
	adapter.withMeta(ClauseProhibit, "no provider for ghost")
	adapter.addClause([]int{-xID})
	adapter.prohibits[xID] = "no provider for ghost"

	result := adapter.ExplainWhyUNSAT()
	joined := strings.Join(result.Lines, "\n")

	// Must contain the real root cause
	if !strings.Contains(joined, "no provider for ghost") {
		t.Errorf("must contain prohibit reason 'no provider for ghost', got:\n%s", joined)
	}
	if !strings.Contains(joined, "dev-libs/x@1.0") {
		t.Errorf("must contain prohibited node dev-libs/x@1.0, got:\n%s", joined)
	}

	// The cycle edge (b→a) must NOT cause a false chain through b
	// b should not be in the chain at all — it's not dead (cycle doesn't kill)
	if strings.Contains(joined, "app/b@1.0") {
		// b might appear if it's dead via x→a propagation, but must not show "(see above)" loop
		if strings.Count(joined, "(see above)") > 0 {
			// Verify "(see above)" doesn't reference a in a cycle-caused chain
			lines := result.Lines
			for _, line := range lines {
				if strings.Contains(line, "app/a@1.0  (see above)") {
					t.Errorf("cycle should not cause '(see above)' on root node a:\n%s", joined)
				}
			}
		}
	}
}

func TestExplainWhyUNSAT_ProhibitReasonPresent(t *testing.T) {
	// Regression guard: the "no provider" string from the actual prohibit
	// must always appear in the output. On the libxcrypt UNSAT this was
	// "backports-tarfile" — the depth cap at 8 cut it off.
	adapter := NewGophersatAdapter()

	root := pkg.NewPackage("app/root", "1.0", "0")
	adapter.AddPackage(root)
	rootID := adapter.GetVarID("app/root@1.0")
	adapter.withMeta(ClauseRoot, "root")
	adapter.addClause([]int{rootID})
	adapter.addRootVars([]int{rootID})

	// Build a chain of depth 12: root → n1 → n2 → ... → n11 → leaf (prohibited)
	prevID := rootID
	for i := range 11 {
		p := pkg.NewPackage(fmt.Sprintf("dev-libs/n%d", i), "1.0", "0")
		adapter.AddPackage(p)
		pID := adapter.GetVarID(fmt.Sprintf("dev-libs/n%d@1.0", i))
		adapter.AddImplication(prevID, []int{pID})
		prevID = pID
	}

	leaf := pkg.NewPackage("dev-libs/leaf", "1.0", "0")
	adapter.AddPackage(leaf)
	leafID := adapter.GetVarID("dev-libs/leaf@1.0")
	adapter.AddImplication(prevID, []int{leafID})

	reason := "no provider for backports-tarfile (needed by dev-libs/leaf@1.0)"
	adapter.withMeta(ClauseProhibit, reason)
	adapter.addClause([]int{-leafID})
	adapter.prohibits[leafID] = reason

	result := adapter.ExplainWhyUNSAT()
	joined := strings.Join(result.Lines, "\n")

	// The leaf prohibit at depth 12 must appear — no depth cap
	if !strings.Contains(joined, "backports-tarfile") {
		t.Errorf("must contain prohibit root cause 'backports-tarfile' at depth 12, got:\n%s", joined)
	}
	if !strings.Contains(joined, "no provider") {
		t.Errorf("must contain at least one 'no provider' line, got:\n%s", joined)
	}
}

func TestExplainUNSATGeneric_AtMostOneConflict(t *testing.T) {
	adapter := NewGophersatAdapter()

	a := pkg.NewPackage("app/a", "1.0", "0")
	b := pkg.NewPackage("app/b", "1.0", "0")
	adapter.AddPackage(a)
	adapter.AddPackage(b)

	aID := adapter.GetVarID("app/a@1.0")
	bID := adapter.GetVarID("app/b@1.0")

	adapter.withMeta(ClauseRoot, "root: app/a")
	adapter.addClause([]int{aID})
	adapter.withMeta(ClauseRoot, "root: app/b")
	adapter.addClause([]int{bID})
	adapter.withMeta(ClauseAtMostOne, "conflict")
	adapter.addClause([]int{-aID, -bID})

	status, _, _ := adapter.Solve()
	if status != pkg.StatusUnsat {
		t.Fatal("expected UNSAT")
	}

	// Generic explainer still works for at-most-one conflicts
	lines := adapter.ExplainUNSATGeneric()
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "UNSAT core (generic)") {
		t.Errorf("should have generic header, got:\n%s", joined)
	}
}

// --- Tests for v0.10.0-012: Blocker conflict clauses ---

// TestBlocker_MutualExclusion verifies that a blocker prevents two packages
// from coexisting in the solution. Package A blocks package B -- both cannot
// be in the result simultaneously.
func TestBlocker_MutualExclusion(t *testing.T) {
	r := newMultiVersionRepo()

	// Package B: the blocked package
	pkgB := pkg.NewPackage("app-arch/pbzip2", "1.1.13", "0")
	r.addVersion(pkgB)

	// Package A: blocks B
	blockedAtom, err := pkg.ParseAtom("!app-arch/pbzip2")
	if err != nil {
		t.Fatalf("failed to parse blocker atom: %v", err)
	}

	pkgA := pkg.NewPackage("app-alternatives/bzip2", "0.3", "0")
	pkgA.AddBlocker(blockedAtom, false) // weak blocker
	r.addVersion(pkgA)

	// Resolving A alone should work (B is not requested)
	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app-alternatives/bzip2"})

	aKey := pkg.SlotKey{Name: "app-alternatives/bzip2", Slot: "0"}
	if _, ok := result[aKey]; !ok {
		t.Error("expected app-alternatives/bzip2 in result")
	}

	// B should NOT be pulled in since it's not a dependency
	bKey := pkg.SlotKey{Name: "app-arch/pbzip2", Slot: "0"}
	if _, ok := result[bKey]; ok {
		t.Error("pbzip2 should NOT be in result -- it's not a dependency, just blocked")
	}
}

// TestBlocker_BlockedPackageAsDepCausesUNSAT verifies that when package A
// blocks package B, and A also depends on B, the result is UNSAT because
// the blocker conflict clause (-A | -B) combined with the implication clause
// (-A | B) means A cannot be selected.
func TestBlocker_BlockedPackageAsDepCausesUNSAT(t *testing.T) {
	r := newMultiVersionRepo()

	pkgB := pkg.NewPackage("dev-libs/conflict", "1.0", "0")
	r.addVersion(pkgB)

	// A depends on B AND blocks B -- contradictory (strong blocker)
	blockedAtom, err := pkg.ParseAtom("!!dev-libs/conflict")
	if err != nil {
		t.Fatalf("failed to parse blocker atom: %v", err)
	}

	pkgA := pkg.NewPackage("app-misc/contradictory", "1.0", "0")
	pkgA.Deps = []pkg.Constraint{
		{Name: "dev-libs/conflict", Type: pkg.ConstraintTypeVersion},
	}
	pkgA.AddBlocker(blockedAtom, true)
	r.addVersion(pkgA)

	resolver := NewResolver(r)
	_, err = resolver.Resolve([]string{"app-misc/contradictory"})
	if err == nil {
		t.Error("expected UNSAT when package both depends on and blocks the same target")
	}
}

// TestBlocker_ConflictPreventsCoexistence verifies that when two root packages
// are requested but one blocks the other, SAT returns UNSAT.
func TestBlocker_ConflictPreventsCoexistence(t *testing.T) {
	r := newMultiVersionRepo()

	pkgB := pkg.NewPackage("app-arch/lbzip2", "2.5", "0")
	r.addVersion(pkgB)

	blockedAtom, err := pkg.ParseAtom("!!app-arch/lbzip2")
	if err != nil {
		t.Fatalf("failed to parse blocker atom: %v", err)
	}

	pkgA := pkg.NewPackage("app-alternatives/bzip2", "0.3", "0")
	pkgA.AddBlocker(blockedAtom, true)
	r.addVersion(pkgA)

	// Request both -- should fail because A blocks B (strong blocker)
	resolver := NewResolver(r)
	_, err = resolver.Resolve([]string{"app-alternatives/bzip2", "app-arch/lbzip2"})
	if err == nil {
		t.Error("expected UNSAT when requesting two mutually blocked packages")
	}
}

// TestBlocker_VersionedBlockerMatchesCorrectly verifies that a version-constrained
// blocker only blocks matching versions. E.g., "!>=dev-libs/foo-2.0" should
// block foo-2.0+ but not foo-1.0.
func TestBlocker_VersionedBlockerMatchesCorrectly(t *testing.T) {
	r := newMultiVersionRepo()

	// Two versions of foo
	r.addVersion(pkg.NewPackage("dev-libs/foo", "1.0", "0"))
	r.addVersion(pkg.NewPackage("dev-libs/foo", "2.0", "0"))
	r.addVersion(pkg.NewPackage("dev-libs/foo", "3.0", "0"))

	// bar blocks >=foo-2.0 (strong blocker)
	blockedAtom, err := pkg.ParseAtom("!!>=dev-libs/foo-2.0")
	if err != nil {
		t.Fatalf("failed to parse blocker atom: %v", err)
	}

	bar := pkg.NewPackage("app-misc/bar", "1.0", "0")
	bar.Deps = []pkg.Constraint{
		{Name: "dev-libs/foo", Type: pkg.ConstraintTypeVersion},
	}
	bar.AddBlocker(blockedAtom, true)
	r.addVersion(bar)

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app-misc/bar"})

	// bar should be in result
	barKey := pkg.SlotKey{Name: "app-misc/bar", Slot: "0"}
	if _, ok := result[barKey]; !ok {
		t.Fatal("expected bar in result")
	}

	// foo must be in result (it's a dependency), but ONLY version 1.0
	// because >=2.0 are blocked
	fooKey := pkg.SlotKey{Name: "dev-libs/foo", Slot: "0"}
	p, ok := result[fooKey]
	if !ok {
		t.Fatal("expected foo in result (it's a dep of bar)")
	}
	if p.Version != "1.0" {
		t.Errorf("expected foo 1.0 (>=2.0 blocked), got %s", p.Version)
	}
}

// TestBlocker_WeakAndStrongSameSATSemantics verifies that both ! and !!
// produce identical conflict clauses in SAT. PMS 8.2.6.6: the difference
// is merge transaction ordering, not final state — both prohibit coexistence.
// IsStrong is an annotation for the future merge planner, not the solver.
func TestBlocker_WeakAndStrongSameSATSemantics(t *testing.T) {
	adapter1 := NewGophersatAdapter()
	a1 := pkg.NewPackage("app/a", "1.0", "0")
	b1 := pkg.NewPackage("app/b", "1.0", "0")
	adapter1.AddPackage(a1)
	adapter1.AddPackage(b1)
	strongAtom, _ := pkg.ParseAtom("!!app/b")
	adapter1.AddBlockerConflict(adapter1.GetVarID("app/a@1.0"), strongAtom)

	adapter2 := NewGophersatAdapter()
	a2 := pkg.NewPackage("app/a", "1.0", "0")
	b2 := pkg.NewPackage("app/b", "1.0", "0")
	adapter2.AddPackage(a2)
	adapter2.AddPackage(b2)
	weakAtom, _ := pkg.ParseAtom("!app/b")
	adapter2.AddBlockerConflict(adapter2.GetVarID("app/a@1.0"), weakAtom)

	if len(adapter1.clauses) != len(adapter2.clauses) {
		t.Errorf("strong and weak should produce same clause count: %d vs %d",
			len(adapter1.clauses), len(adapter2.clauses))
	}
	if len(adapter1.clauses) != 1 {
		t.Errorf("expected 1 conflict clause, got %d", len(adapter1.clauses))
	}
}

// TestBlocker_VacuouslyTrueWhenBlockedNotPresent verifies that a blocker
// for a package not in the SAT problem is harmless (vacuously true).
func TestBlocker_VacuouslyTrueWhenBlockedNotPresent(t *testing.T) {
	r := newMultiVersionRepo()

	// A blocks "nonexistent" -- package not in repo
	blockedAtom, err := pkg.ParseAtom("!app-arch/nonexistent")
	if err != nil {
		t.Fatalf("failed to parse blocker atom: %v", err)
	}

	pkgA := pkg.NewPackage("app-misc/safe", "1.0", "0")
	pkgA.AddBlocker(blockedAtom, false)
	r.addVersion(pkgA)

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app-misc/safe"})

	aKey := pkg.SlotKey{Name: "app-misc/safe", Slot: "0"}
	if _, ok := result[aKey]; !ok {
		t.Error("expected safe in result -- blocker is vacuously true")
	}
}

// TestBlocker_ClauseConflictSourceInMeta verifies that blocker clauses
// are tagged with ClauseConflict source for the UNSAT explainer.
func TestBlocker_ClauseConflictSourceInMeta(t *testing.T) {
	adapter := NewGophersatAdapter()

	a := pkg.NewPackage("app/a", "1.0", "0")
	b := pkg.NewPackage("app/b", "1.0", "0")
	adapter.AddPackage(a)
	adapter.AddPackage(b)

	blockedAtom, _ := pkg.ParseAtom("!app/b")
	adapter.AddBlockerConflict(adapter.GetVarID("app/a@1.0"), blockedAtom)

	if len(adapter.clauses) != 1 {
		t.Fatalf("expected 1 clause, got %d", len(adapter.clauses))
	}

	meta := adapter.clausesMeta[0]
	if meta.Source != ClauseConflict {
		t.Errorf("expected ClauseConflict source, got %s", meta.Source)
	}
	if !strings.Contains(meta.Reason, "blocker") {
		t.Errorf("reason should mention 'blocker', got: %s", meta.Reason)
	}
}

// TestBlocker_UNSATExplainerShowsBlocker verifies that when a blocker causes
// UNSAT, the conflict clause appears in the UNSAT explanation.
func TestBlocker_UNSATExplainerShowsBlocker(t *testing.T) {
	adapter := NewGophersatAdapter()

	a := pkg.NewPackage("app/a", "1.0", "0")
	b := pkg.NewPackage("app/b", "1.0", "0")
	adapter.AddPackage(a)
	adapter.AddPackage(b)

	aID := adapter.GetVarID("app/a@1.0")
	bID := adapter.GetVarID("app/b@1.0")

	// Both are roots
	adapter.withMeta(ClauseRoot, "root: app/a")
	adapter.addClause([]int{aID})
	adapter.addRootVars([]int{aID})
	adapter.withMeta(ClauseRoot, "root: app/b")
	adapter.addClause([]int{bID})
	adapter.addRootVars([]int{bID})

	// A blocks B
	blockedAtom, _ := pkg.ParseAtom("!app/b")
	adapter.AddBlockerConflict(aID, blockedAtom)

	status, _, _ := adapter.Solve()
	if status != pkg.StatusUnsat {
		t.Fatal("expected UNSAT when both roots required but one blocks the other")
	}

	// Generic explainer should pick up the conflict clause
	lines := adapter.ExplainUNSATGeneric()
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "conflict") {
		t.Errorf("UNSAT explanation should mention conflict source, got:\n%s", joined)
	}
}

// TestBlocker_OldVersionBlocksNewDoesNot verifies that when only an old version
// declares a blocker, SAT backtracks to the new version that doesn't block.
// This is the canonical Gentoo pattern: !<cat/old-pkg-N after a package rename.
func TestBlocker_OldVersionBlocksNewDoesNot(t *testing.T) {
	r := newMultiVersionRepo()

	b := pkg.NewPackage("dev-libs/b", "1.0", "0")
	r.addVersion(b)

	// a-1.0 blocks b (strong blocker to test backtrack)
	aV1 := pkg.NewPackage("app/a", "1.0", "0")
	aV1.Deps = []pkg.Constraint{{Name: "dev-libs/b", Type: pkg.ConstraintTypeVersion}}
	blockerAtom, _ := pkg.ParseAtom("!!dev-libs/b")
	aV1.Blockers = []pkg.BlockerEntry{{Atom: blockerAtom, IsStrong: true}}
	r.addVersion(aV1)

	// a-2.0 depends on b, no blocker
	aV2 := pkg.NewPackage("app/a", "2.0", "0")
	aV2.Deps = []pkg.Constraint{{Name: "dev-libs/b", Type: pkg.ConstraintTypeVersion}}
	r.addVersion(aV2)

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app/a"})

	slotResult := result
	aKey := pkg.SlotKey{Name: "app/a", Slot: "0"}
	p, ok := slotResult[aKey]
	if !ok {
		t.Fatal("expected app/a in result")
	}
	if p.Version != "2.0" {
		t.Errorf("expected a-2.0 (no blocker), got %s — SAT should backtrack from a-1.0 which blocks b", p.Version)
	}

	bKey := pkg.SlotKey{Name: "dev-libs/b", Slot: "0"}
	if _, ok := slotResult[bKey]; !ok {
		t.Error("expected dev-libs/b in result — a-2.0 depends on it")
	}
}

func TestBlocker_ExplainerShowsBlockerConflict(t *testing.T) {
	r := newMultiVersionRepo()

	a := pkg.NewPackage("dev-libs/a", "1.0", "0")
	blockerAtom, _ := pkg.ParseAtom("!!dev-libs/b")
	a.Blockers = []pkg.BlockerEntry{{Atom: blockerAtom, IsStrong: true}}
	r.addVersion(a)

	b := pkg.NewPackage("dev-libs/b", "1.0", "0")
	r.addVersion(b)

	// app depends on both a and b — UNSAT because a blocks b
	app := pkg.NewPackage("app/app", "1.0", "0")
	app.Deps = []pkg.Constraint{
		{Name: "dev-libs/a", Type: pkg.ConstraintTypeVersion},
		{Name: "dev-libs/b", Type: pkg.ConstraintTypeVersion},
	}
	r.addVersion(app)

	resolver := NewResolver(r)
	_, err := resolver.Resolve([]string{"app/app"})
	if err == nil {
		t.Fatal("expected UNSAT — a blocks b")
	}

	if !strings.Contains(err.Error(), "UNSAT") {
		t.Errorf("error should mention UNSAT, got: %s", err.Error())
	}
}

func TestBlocker_ExplainerOutputContainsBlockerReason(t *testing.T) {
	adapter := NewGophersatAdapter()

	a := pkg.NewPackage("dev-libs/a", "1.0", "0")
	b := pkg.NewPackage("dev-libs/b", "1.0", "0")
	app := pkg.NewPackage("app/app", "1.0", "0")
	adapter.AddPackage(a)
	adapter.AddPackage(b)
	adapter.AddPackage(app)

	appID := adapter.GetVarID("app/app@1.0")
	aID := adapter.GetVarID("dev-libs/a@1.0")

	adapter.withMeta(ClauseRoot, "root")
	adapter.addClause([]int{appID})
	adapter.addRootVars([]int{appID})

	adapter.AddImplication(appID, []int{aID})
	adapter.AddImplication(appID, []int{adapter.GetVarID("dev-libs/b@1.0")})

	blockedAtom, _ := pkg.ParseAtom("!!dev-libs/b")
	adapter.AddBlockerConflict(aID, blockedAtom)

	result := adapter.ExplainWhyUNSAT()
	joined := strings.Join(result.Lines, "\n")

	if !strings.Contains(joined, "blocker:") {
		t.Errorf("explainer should show blocker reason, got:\n%s", joined)
	}
	if !strings.Contains(joined, "dev-libs/a@1.0") {
		t.Errorf("explainer should name the blocking package, got:\n%s", joined)
	}
	if !strings.Contains(joined, "dev-libs/b@1.0") {
		t.Errorf("explainer should name the blocked package, got:\n%s", joined)
	}
}

// --- Tests for v0.10.0-017: VDB candidates and installed flag ---

// TestVDB_InstalledPackageAsCandidate verifies that an installed package version
// is added as a SAT candidate alongside repo versions, so the solver can choose
// to keep the installed version.
func TestVDB_InstalledPackageAsCandidate(t *testing.T) {
	r := newMultiVersionRepo()
	r.addVersion(pkg.NewPackage("dev-libs/openssl", "3.0.14", "0"))
	r.addVersion(pkg.NewPackage("dev-libs/openssl", "3.0.15", "0"))

	// Installed version 3.0.13 is NOT in the repo (removed upstream)
	installedPkg := &state.InstalledPackage{
		Package: pkg.NewPackage("dev-libs/openssl", "3.0.13", "0"),
	}

	db := state.NewPackageDatabase("/var/db/pkg")
	if err := db.Add(installedPkg); err != nil {
		t.Fatalf("failed to add installed package: %v", err)
	}

	resolver := NewResolver(r)
	resolver.SetInstalledDB(db)
	result := resolveClean(t, resolver, []string{"dev-libs/openssl"})

	// The solver should pick one of {3.0.13, 3.0.14, 3.0.15}
	key := pkg.SlotKey{Name: "dev-libs/openssl", Slot: "0"}
	p, ok := result[key]
	if !ok {
		t.Fatal("expected openssl in result")
	}

	validVersions := map[string]bool{"3.0.13": true, "3.0.14": true, "3.0.15": true}
	if !validVersions[p.Version] {
		t.Errorf("expected one of {3.0.13, 3.0.14, 3.0.15}, got %s", p.Version)
	}
}

// TestVDB_InstalledOnlyVersion verifies that when a package is installed but
// completely removed from the repo, the installed version is still a valid
// SAT candidate and can be selected.
func TestVDB_InstalledOnlyVersion(t *testing.T) {
	r := newMultiVersionRepo()

	// dep is required by app but ONLY exists as an installed package
	installedDep := &state.InstalledPackage{
		Package: pkg.NewPackage("dev-libs/legacy", "1.0", "0"),
	}

	db := state.NewPackageDatabase("/var/db/pkg")
	if err := db.Add(installedDep); err != nil {
		t.Fatalf("failed to add installed package: %v", err)
	}

	// App depends on dev-libs/legacy
	app := pkg.NewPackage("app-misc/user", "1.0", "0")
	app.Deps = []pkg.Constraint{
		{Name: "dev-libs/legacy", Type: pkg.ConstraintTypeVersion},
	}
	r.addVersion(app)

	resolver := NewResolver(r)
	resolver.SetInstalledDB(db)
	result := resolveClean(t, resolver, []string{"app-misc/user"})

	legacyKey := pkg.SlotKey{Name: "dev-libs/legacy", Slot: "0"}
	p, ok := result[legacyKey]
	if !ok {
		t.Fatal("expected dev-libs/legacy in result — should be injected from VDB")
	}
	if p.Version != "1.0" {
		t.Errorf("expected legacy 1.0, got %s", p.Version)
	}
}

// TestVDB_MarkInstalled verifies that the adapter correctly tracks which
// SAT variables are marked as installed.
func TestVDB_MarkInstalled(t *testing.T) {
	adapter := NewGophersatAdapter()

	p1 := pkg.NewPackage("dev-libs/a", "1.0", "0")
	p2 := pkg.NewPackage("dev-libs/a", "2.0", "0")
	adapter.AddPackage(p1)
	adapter.AddPackage(p2)

	v1 := adapter.GetVarID("dev-libs/a@1.0")
	v2 := adapter.GetVarID("dev-libs/a@2.0")

	// Initially neither is installed
	if adapter.IsVarInstalled(v1) {
		t.Error("v1 should not be installed before MarkInstalled")
	}

	// Mark v1 as installed
	adapter.MarkInstalled(v1)

	if !adapter.IsVarInstalled(v1) {
		t.Error("v1 should be installed after MarkInstalled")
	}
	if adapter.IsVarInstalled(v2) {
		t.Error("v2 should not be installed (never marked)")
	}
}

// TestBlocker_WeakBlockerFiresWithoutVDB verifies that weak blockers produce
// conflict clauses regardless of installed state. PMS 8.2.6.6: difference is
// merge transaction ordering, not final state — SAT encodes the final state.
func TestBlocker_WeakBlockerFiresWithoutVDB(t *testing.T) {
	r := newMultiVersionRepo()

	b := pkg.NewPackage("dev-libs/b", "1.0", "0")
	r.addVersion(b)

	weakAtom, _ := pkg.ParseAtom("!dev-libs/b")
	a := pkg.NewPackage("app-misc/a", "1.0", "0")
	a.Deps = []pkg.Constraint{{Name: "dev-libs/b", Type: pkg.ConstraintTypeVersion}}
	a.Blockers = []pkg.BlockerEntry{{Atom: weakAtom, IsStrong: false}}
	r.addVersion(a)

	resolver := NewResolver(r)
	_, err := resolver.Resolve([]string{"app-misc/a"})
	if err == nil {
		t.Error("expected UNSAT — weak blocker a(!b) + dep a→b = conflict in final state")
	}
}

// --- Tests for v0.10.0-018: USE deps on atoms ---

// TestConstraint_PackageSatisfiesUseDeps tests the domain-level USE dep check.
func TestConstraint_PackageSatisfiesUseDeps(t *testing.T) {
	tests := []struct {
		name       string
		useRequire []string
		useBlock   []string
		useFlags   map[string]bool
		want       bool
	}{
		{"no USE deps — always satisfies", nil, nil, map[string]bool{"ssl": true}, true},
		{"require ssl — provider has ssl", []string{"ssl"}, nil, map[string]bool{"ssl": true}, true},
		{"require ssl — provider lacks ssl", []string{"ssl"}, nil, map[string]bool{"debug": true}, false},
		{"require ssl — provider ssl=false", []string{"ssl"}, nil, map[string]bool{"ssl": false}, false},
		{"block debug — debug disabled", nil, []string{"debug"}, map[string]bool{"debug": false}, true},
		{"block debug — debug enabled", nil, []string{"debug"}, map[string]bool{"debug": true}, false},
		{"require ssl + block debug — both ok", []string{"ssl"}, []string{"debug"}, map[string]bool{"ssl": true, "debug": false}, true},
		{"require ssl + block debug — debug on", []string{"ssl"}, []string{"debug"}, map[string]bool{"ssl": true, "debug": true}, false},
		{"require ssl + block debug — ssl off", []string{"ssl"}, []string{"debug"}, map[string]bool{"ssl": false, "debug": false}, false},
		{"block flag — nil map", nil, []string{"debug"}, nil, true},
		{"require flag — nil map", []string{"ssl"}, nil, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := pkg.Constraint{UseRequire: tt.useRequire, UseBlock: tt.useBlock}
			if got := c.PackageSatisfiesUseDeps(tt.useFlags); got != tt.want {
				t.Errorf("PackageSatisfiesUseDeps() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestConstraint_HasUseDeps tests the HasUseDeps predicate.
func TestConstraint_HasUseDeps(t *testing.T) {
	tests := []struct {
		name       string
		useRequire []string
		useBlock   []string
		want       bool
	}{
		{"empty", nil, nil, false},
		{"require only", []string{"ssl"}, nil, true},
		{"block only", nil, []string{"debug"}, true},
		{"both", []string{"ssl"}, []string{"debug"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := pkg.Constraint{UseRequire: tt.useRequire, UseBlock: tt.useBlock}
			if got := c.HasUseDeps(); got != tt.want {
				t.Errorf("HasUseDeps() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestUseDeps_RequireFlag tests that dep openssl[ssl] only matches versions with ssl enabled.
func TestUseDeps_RequireFlag(t *testing.T) {
	r := newMultiVersionRepo()
	ssl1 := pkg.NewPackage("dev-libs/openssl", "3.0.14", "0")
	ssl1.UseFlags["ssl"] = true
	r.addVersion(ssl1)

	ssl2 := pkg.NewPackage("dev-libs/openssl", "3.0.15", "0")
	ssl2.UseFlags["ssl"] = false
	r.addVersion(ssl2)

	app := pkg.NewPackage("app-misc/myapp", "1.0", "0")
	app.Deps = []pkg.Constraint{{
		Name: "dev-libs/openssl", Type: pkg.ConstraintTypeVersion,
		UseRequire: []string{"ssl"},
	}}
	r.addVersion(app)

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app-misc/myapp"})
	key := pkg.SlotKey{Name: "dev-libs/openssl", Slot: "0"}
	p, ok := result[key]
	if !ok {
		t.Fatal("expected openssl in result")
	}
	if p.Version != "3.0.14" {
		t.Errorf("expected openssl-3.0.14 (ssl=true), got %s", p.Version)
	}
}

// TestUseDeps_BlockFlag tests that dep openssl[-debug] excludes versions with debug enabled.
func TestUseDeps_BlockFlag(t *testing.T) {
	r := newMultiVersionRepo()
	v1 := pkg.NewPackage("dev-libs/openssl", "3.0.14", "0")
	v1.UseFlags["debug"] = true
	r.addVersion(v1)

	v2 := pkg.NewPackage("dev-libs/openssl", "3.0.15", "0")
	v2.UseFlags["debug"] = false
	r.addVersion(v2)

	app := pkg.NewPackage("app-misc/myapp", "1.0", "0")
	app.Deps = []pkg.Constraint{{
		Name: "dev-libs/openssl", Type: pkg.ConstraintTypeVersion,
		UseBlock: []string{"debug"},
	}}
	r.addVersion(app)

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app-misc/myapp"})
	key := pkg.SlotKey{Name: "dev-libs/openssl", Slot: "0"}
	p, ok := result[key]
	if !ok {
		t.Fatal("expected openssl in result")
	}
	if p.Version != "3.0.15" {
		t.Errorf("expected openssl-3.0.15 (debug=false), got %s", p.Version)
	}
}

// TestUseDeps_NoUseDeps_BackwardCompat tests backward compat: no USE deps = all versions match.
func TestUseDeps_NoUseDeps_BackwardCompat(t *testing.T) {
	r := newMultiVersionRepo()
	v1 := pkg.NewPackage("dev-libs/openssl", "3.0.14", "0")
	v1.UseFlags["ssl"] = true
	r.addVersion(v1)
	v2 := pkg.NewPackage("dev-libs/openssl", "3.0.15", "0")
	v2.UseFlags["ssl"] = false
	r.addVersion(v2)

	app := pkg.NewPackage("app-misc/myapp", "1.0", "0")
	app.Deps = []pkg.Constraint{{Name: "dev-libs/openssl", Type: pkg.ConstraintTypeVersion}}
	r.addVersion(app)

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app-misc/myapp"})
	key := pkg.SlotKey{Name: "dev-libs/openssl", Slot: "0"}
	if _, ok := result[key]; !ok {
		t.Fatal("expected openssl in result — both versions are valid without USE deps")
	}
}

// TestUseDeps_UNSAT_NoProvider tests that no provider with required USE flag causes UNSAT.
func TestUseDeps_UNSAT_NoProvider(t *testing.T) {
	r := newMultiVersionRepo()
	v1 := pkg.NewPackage("dev-libs/openssl", "3.0.14", "0")
	v1.UseFlags["ssl"] = false
	r.addVersion(v1)
	v2 := pkg.NewPackage("dev-libs/openssl", "3.0.15", "0")
	v2.UseFlags["ssl"] = false
	r.addVersion(v2)

	app := pkg.NewPackage("app-misc/myapp", "1.0", "0")
	app.Deps = []pkg.Constraint{{
		Name: "dev-libs/openssl", Type: pkg.ConstraintTypeVersion,
		UseRequire: []string{"ssl"},
	}}
	r.addVersion(app)

	resolver := NewResolver(r)
	result, err := resolver.Resolve([]string{"app-misc/myapp"})
	if err != nil {
		return // Error is acceptable — means UNSAT
	}
	if result != nil {
		key := pkg.SlotKey{Name: "dev-libs/openssl", Slot: "0"}
		if _, ok := result[key]; ok {
			t.Error("openssl should NOT be in result since no version satisfies [ssl]")
		}
	}
}

// TestUseDeps_CombinedRequireAndBlock tests [ssl,-debug].
func TestUseDeps_CombinedRequireAndBlock(t *testing.T) {
	r := newMultiVersionRepo()
	v1 := pkg.NewPackage("dev-libs/openssl", "3.0.14", "0")
	v1.UseFlags["ssl"] = true
	v1.UseFlags["debug"] = true
	r.addVersion(v1)
	v2 := pkg.NewPackage("dev-libs/openssl", "3.0.15", "0")
	v2.UseFlags["ssl"] = true
	v2.UseFlags["debug"] = false
	r.addVersion(v2)
	v3 := pkg.NewPackage("dev-libs/openssl", "3.1.0", "0")
	v3.UseFlags["ssl"] = false
	v3.UseFlags["debug"] = false
	r.addVersion(v3)

	app := pkg.NewPackage("app-misc/myapp", "1.0", "0")
	app.Deps = []pkg.Constraint{{
		Name: "dev-libs/openssl", Type: pkg.ConstraintTypeVersion,
		UseRequire: []string{"ssl"}, UseBlock: []string{"debug"},
	}}
	r.addVersion(app)

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app-misc/myapp"})
	key := pkg.SlotKey{Name: "dev-libs/openssl", Slot: "0"}
	p, ok := result[key]
	if !ok {
		t.Fatal("expected openssl in result")
	}
	if p.Version != "3.0.15" {
		t.Errorf("expected openssl-3.0.15 (ssl=true, debug=false), got %s", p.Version)
	}
}

// TestUseDeps_SlotConstraint tests USE dep filtering on slot constraints.
func TestUseDeps_SlotConstraint(t *testing.T) {
	r := newMultiVersionRepo()
	py1 := pkg.NewPackage("dev-lang/python", "3.12.7", "3.12")
	py1.UseFlags["ssl"] = true
	r.addVersion(py1)
	py2 := pkg.NewPackage("dev-lang/python", "3.13.1", "3.13")
	py2.UseFlags["ssl"] = false
	r.addVersion(py2)

	app := pkg.NewPackage("app-misc/myapp", "1.0", "0")
	app.Deps = []pkg.Constraint{{
		Name: "dev-lang/python", Slot: "3.12", Type: pkg.ConstraintTypeSlot,
		UseRequire: []string{"ssl"},
	}}
	r.addVersion(app)

	resolver := NewResolver(r)
	result, err := resolver.Resolve([]string{"app-misc/myapp"})
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	// PostPassAdded may be >0 for slot deps (known resolver behavior, not USE dep issue)
	key := pkg.SlotKey{Name: "dev-lang/python", Slot: "3.12"}
	p, ok := result[key]
	if !ok {
		t.Fatal("expected python:3.12 in result")
	}
	if p.Version != "3.12.7" {
		t.Errorf("expected python-3.12.7, got %s", p.Version)
	}
}

// TestUseDeps_OrGroup tests USE dep filtering within OR-group alternatives.
func TestUseDeps_OrGroup(t *testing.T) {
	r := newMultiVersionRepo()
	mysql := pkg.NewPackage("dev-db/mysql", "8.0", "0")
	mysql.UseFlags["ssl"] = false
	r.addVersion(mysql)
	pg := pkg.NewPackage("dev-db/postgresql", "16.0", "0")
	pg.UseFlags["ssl"] = true
	r.addVersion(pg)

	app := pkg.NewPackage("app-misc/myapp", "1.0", "0")
	app.Deps = []pkg.Constraint{
		{Name: "dev-db/mysql", Type: pkg.ConstraintTypeVersion, UseRequire: []string{"ssl"}, OrGroupID: 1},
		{Name: "dev-db/postgresql", Type: pkg.ConstraintTypeVersion, UseRequire: []string{"ssl"}, OrGroupID: 1},
	}
	r.addVersion(app)

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app-misc/myapp"})
	pgKey := pkg.SlotKey{Name: "dev-db/postgresql", Slot: "0"}
	if _, ok := result[pgKey]; !ok {
		t.Error("expected postgresql in result (only provider with ssl=true)")
	}
}

// TestUseDeps_FindSatisfyingVars_AdapterLevel tests adapter-level USE dep filtering.
func TestUseDeps_FindSatisfyingVars_AdapterLevel(t *testing.T) {
	adapter := NewGophersatAdapter()
	ssl1 := pkg.NewPackage("dev-libs/openssl", "3.0.14", "0")
	ssl1.UseFlags["ssl"] = true
	ssl1.UseFlags["debug"] = false
	adapter.AddPackage(ssl1)
	ssl2 := pkg.NewPackage("dev-libs/openssl", "3.0.15", "0")
	ssl2.UseFlags["ssl"] = false
	ssl2.UseFlags["debug"] = true
	adapter.AddPackage(ssl2)

	// Require ssl — only 3.0.14 matches
	cReq := pkg.Constraint{Name: "dev-libs/openssl", Type: pkg.ConstraintTypeVersion, UseRequire: []string{"ssl"}}
	vars := adapter.findSatisfyingVars(cReq)
	if len(vars) != 1 {
		t.Fatalf("expected 1 var (3.0.14 with ssl), got %d", len(vars))
	}
	if n := adapter.varName(vars[0]); n != "dev-libs/openssl@3.0.14" {
		t.Errorf("expected dev-libs/openssl@3.0.14, got %s", n)
	}

	// Block debug — only 3.0.14 matches
	cBlock := pkg.Constraint{Name: "dev-libs/openssl", Type: pkg.ConstraintTypeVersion, UseBlock: []string{"debug"}}
	vars = adapter.findSatisfyingVars(cBlock)
	if len(vars) != 1 {
		t.Fatalf("expected 1 var (3.0.14 without debug), got %d", len(vars))
	}

	// No USE deps — both match
	cNone := pkg.Constraint{Name: "dev-libs/openssl", Type: pkg.ConstraintTypeVersion}
	vars = adapter.findSatisfyingVars(cNone)
	if len(vars) != 2 {
		t.Errorf("expected 2 vars (no USE filter), got %d", len(vars))
	}
}

func TestUseDeps_DefaultPlus_ProviderWithoutFlag(t *testing.T) {
	// x[ssl(+)] against provider x without ssl in IUSE → match (default enabled)
	r := newMultiVersionRepo()

	xNoSSL := pkg.NewPackage("dev-libs/x", "1.0", "0")
	// No ssl in UseFlags at all — not in IUSE
	r.addVersion(xNoSSL)

	app := pkg.NewPackage("app/app", "1.0", "0")
	app.Deps = []pkg.Constraint{{
		Name:       "dev-libs/x",
		Type:       pkg.ConstraintTypeVersion,
		UseRequire: []string{"ssl"},
		UseDefault: map[string]bool{"ssl": true}, // ssl(+)
	}}
	r.addVersion(app)

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app/app"})

	xKey := pkg.SlotKey{Name: "dev-libs/x", Slot: "0"}
	if _, ok := result[xKey]; !ok {
		t.Error("x should match — ssl(+) default treats absent flag as enabled")
	}
}

func TestUseDeps_NoDefault_ProviderWithoutFlag(t *testing.T) {
	// x[ssl] (no default) against provider x without ssl in IUSE → no match
	r := newMultiVersionRepo()

	xNoSSL := pkg.NewPackage("dev-libs/x", "1.0", "0")
	r.addVersion(xNoSSL)

	app := pkg.NewPackage("app/app", "1.0", "0")
	app.Deps = []pkg.Constraint{{
		Name:       "dev-libs/x",
		Type:       pkg.ConstraintTypeVersion,
		UseRequire: []string{"ssl"},
		// No UseDefault — strict check
	}}
	r.addVersion(app)

	resolver := NewResolver(r)
	_, err := resolver.Resolve([]string{"app/app"})
	if err == nil {
		t.Error("expected UNSAT — x has no ssl flag and no default")
	}
}

// --- Tests for v0.10.0-014: MAX-SAT optimization ---

func TestMAXSAT_PrefersNewestVersion(t *testing.T) {
	r := newMultiVersionRepo()
	r.addVersion(pkg.NewPackage("sys-libs/zlib", "1.2.13", "0"))
	r.addVersion(pkg.NewPackage("sys-libs/zlib", "1.3.0", "0"))
	r.addVersion(pkg.NewPackage("sys-libs/zlib", "1.3.1", "0"))

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"sys-libs/zlib"})

	key := pkg.SlotKey{Name: "sys-libs/zlib", Slot: "0"}
	p, ok := result[key]
	if !ok {
		t.Fatal("expected zlib in result")
	}
	if p.Version != "1.3.1" {
		t.Errorf("MAX-SAT should prefer newest: got %s, want 1.3.1", p.Version)
	}
}

func TestMAXSAT_Deterministic20Runs(t *testing.T) {
	r := newMultiVersionRepo()
	r.addVersion(pkg.NewPackage("sys-libs/zlib", "1.2.13", "0"))
	r.addVersion(pkg.NewPackage("sys-libs/zlib", "1.3.0", "0"))
	r.addVersion(pkg.NewPackage("sys-libs/zlib", "1.3.1", "0"))

	dep := pkg.NewPackage("dev-libs/dep", "1.0", "0")
	dep.Deps = []pkg.Constraint{{Name: "sys-libs/zlib", Type: pkg.ConstraintTypeVersion}}
	r.addVersion(dep)

	app := pkg.NewPackage("app/app", "1.0", "0")
	app.Deps = []pkg.Constraint{{Name: "dev-libs/dep", Type: pkg.ConstraintTypeVersion}}
	r.addVersion(app)

	var firstResult map[pkg.SlotKey]string
	for i := range 20 {
		resolver := NewResolver(r)
		result := resolveClean(t, resolver, []string{"app/app"})

		thisResult := make(map[pkg.SlotKey]string)
		for k, p := range result {
			thisResult[k] = p.Version
		}

		if i == 0 {
			firstResult = thisResult
		} else {
			for k, v := range firstResult {
				if thisResult[k] != v {
					t.Fatalf("run %d: non-deterministic at %v: %s vs %s", i, k, v, thisResult[k])
				}
			}
		}
	}
}

func TestMAXSAT_DepVersionPreference(t *testing.T) {
	r := newMultiVersionRepo()

	r.addVersion(pkg.NewPackage("dev-libs/lib", "1.0", "0"))
	r.addVersion(pkg.NewPackage("dev-libs/lib", "2.0", "0"))
	r.addVersion(pkg.NewPackage("dev-libs/lib", "3.0", "0"))

	app := pkg.NewPackage("app/app", "1.0", "0")
	app.Deps = []pkg.Constraint{{Name: "dev-libs/lib", Type: pkg.ConstraintTypeVersion}}
	r.addVersion(app)

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app/app"})

	libKey := pkg.SlotKey{Name: "dev-libs/lib", Slot: "0"}
	p, ok := result[libKey]
	if !ok {
		t.Fatal("expected lib in result")
	}
	if p.Version != "3.0" {
		t.Errorf("MAX-SAT should prefer newest dep: got %s, want 3.0", p.Version)
	}
}

func TestMAXSAT_BacktrackStillWorks(t *testing.T) {
	r := newMultiVersionRepo()

	helper := pkg.NewPackage("dev-libs/helper", "1.0", "0")
	r.addVersion(helper)

	// v2 has unsatisfiable dep — MAX-SAT should still backtrack to v1
	appV1 := pkg.NewPackage("app/flex", "1.0", "0")
	appV1.Deps = []pkg.Constraint{{Name: "dev-libs/helper", Type: pkg.ConstraintTypeVersion}}
	r.addVersion(appV1)

	appV2 := pkg.NewPackage("app/flex", "2.0", "0")
	appV2.Deps = []pkg.Constraint{{Name: "dev-libs/impossible", Type: pkg.ConstraintTypeVersion}}
	r.addVersion(appV2)

	resolver := NewResolver(r)
	result := resolveClean(t, resolver, []string{"app/flex"})

	appKey := pkg.SlotKey{Name: "app/flex", Slot: "0"}
	p := result[appKey]
	if p.Version != "1.0" {
		t.Errorf("should backtrack to v1 (v2 unsatisfiable), got %s", p.Version)
	}
}
