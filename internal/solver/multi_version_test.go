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
	m.MockRepository.Add(p) //nolint:errcheck
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
// These tests describe the TARGET behavior. They MUST FAIL against the current
// implementation. When all tests pass, the feature is complete.

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
