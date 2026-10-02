package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
	"github.com/grpmsoft/grpm/internal/solver"
)

const closedFixtureDir = "../../tests/fixtures/gentoo-closed"

func loadClosedFixtureRepo(t *testing.T) repo.Repository {
	t.Helper()

	cacheDir := filepath.Join(closedFixtureDir, "metadata", "md5-cache")
	info, err := os.Stat(cacheDir)
	if err != nil || !info.IsDir() {
		t.Skip("closed fixture not available")
	}

	absPath, err := filepath.Abs(closedFixtureDir)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	r, err := repo.NewPortageRepository(absPath)
	if err != nil {
		t.Fatalf("failed to load closed fixture repo: %v", err)
	}
	return r
}

func newFilteredResolver(t *testing.T, r repo.Repository) *solver.PortageResolver {
	t.Helper()
	return solver.NewResolverWithFilters(r, nil, []string{"amd64", "~amd64"})
}

func resolveAndAssert(t *testing.T, res *solver.PortageResolver, atoms []string) solver.ResolveResult {
	t.Helper()

	start := time.Now()
	result, err := res.Resolve(atoms)
	elapsed := time.Since(start)

	status := "SAT"
	if err != nil {
		status = "UNSAT"
	}
	t.Logf("%v: %v (%s, %d pkgs, postPass=%d)", atoms, elapsed, status, len(result), res.PostPassAdded)

	if err != nil {
		t.Fatalf("UNSAT is a test failure in closed fixture: %v", err)
	}
	if res.PostPassAdded != 0 {
		t.Errorf("PostPassAdded=%d, want 0 — SAT encoding incomplete", res.PostPassAdded)
	}
	return result
}

func TestClosedFixture_LibassuanResolves(t *testing.T) {
	r := loadClosedFixtureRepo(t)
	res := newFilteredResolver(t, r)
	result := resolveAndAssert(t, res, []string{"dev-libs/libassuan"})

	key := pkg.SlotKey{Name: "dev-libs/libgpg-error", Slot: "0"}
	if _, ok := result[key]; !ok {
		t.Error("result missing dev-libs/libgpg-error (transitive dep)")
	}
}

func TestClosedFixture_SqliteResolves(t *testing.T) {
	r := loadClosedFixtureRepo(t)
	res := newFilteredResolver(t, r)
	result := resolveAndAssert(t, res, []string{"dev-db/sqlite"})

	sqliteKey := pkg.SlotKey{Name: "dev-db/sqlite", Slot: "3"}
	entry, ok := result[sqliteKey]
	if !ok {
		t.Fatalf("result missing dev-db/sqlite:3, keys: %v", result)
	}
	if entry.Package.Slot.Name != "3" {
		t.Errorf("sqlite slot = %s, want 3", entry.Package.Slot.Name)
	}

	zlibKey := pkg.SlotKey{Name: "sys-libs/zlib", Slot: "0"}
	if _, ok := result[zlibKey]; !ok {
		t.Error("result missing sys-libs/zlib (DEPEND of sqlite via virtual/zlib)")
	}
}

func TestClosedFixture_VirtualPkgconfig(t *testing.T) {
	r := loadClosedFixtureRepo(t)
	res := newFilteredResolver(t, r)
	result := resolveAndAssert(t, res, []string{"virtual/pkgconfig"})

	key := pkg.SlotKey{Name: "dev-util/pkgconf", Slot: "0"}
	if _, ok := result[key]; !ok {
		t.Error("result missing dev-util/pkgconf (provider of virtual/pkgconfig)")
	}
}

func TestClosedFixture_MultiRoot(t *testing.T) {
	r := loadClosedFixtureRepo(t)
	res := newFilteredResolver(t, r)
	result := resolveAndAssert(t, res, []string{"dev-libs/libassuan", "dev-db/sqlite"})

	if len(result) < 4 {
		t.Errorf("expected at least 4 packages (libassuan + sqlite + deps), got %d", len(result))
	}
}

func TestClosedFixture_AllPackagesResolve(t *testing.T) {
	r := loadClosedFixtureRepo(t)

	packages := []string{
		"dev-libs/libassuan",
		"dev-libs/libgpg-error",
		"app-arch/xz-utils",
		"app-portage/elt-patches",
		"sys-apps/findutils",
		"sys-apps/gentoo-functions",
		"dev-db/sqlite",
		"app-arch/bzip2",
		"app-arch/unzip",
		"virtual/pkgconfig",
		"sys-libs/readline",
		"sys-libs/ncurses",
		"sys-libs/zlib",
		"app-alternatives/bzip2",
		"dev-util/pkgconf",
		"virtual/zlib",
	}

	for _, p := range packages {
		t.Run(p, func(t *testing.T) {
			res := newFilteredResolver(t, r)
			resolveAndAssert(t, res, []string{p})
		})
	}
}

// TestClosedFixture_PythonMultiSlot verifies that dev-lang/python resolves
// with all 3 slots (3.11, 3.12, 3.13) available as SAT candidates.
// Each slot is a separate package in the result since they have different slot names.
// BDEPEND is stripped to keep the fixture closed; only DEPEND/RDEPEND on existing
// fixture packages (bzip2, xz-utils, virtual/zlib, readline, ncurses, sqlite).
func TestClosedFixture_PythonMultiSlot(t *testing.T) {
	r := loadClosedFixtureRepo(t)
	res := newFilteredResolver(t, r)

	// Resolve all 3 slots individually
	for _, slot := range []string{"3.11", "3.12", "3.13"} {
		t.Run("slot_"+slot, func(t *testing.T) {
			result := resolveAndAssert(t, res, []string{"dev-lang/python:" + slot})
			key := pkg.SlotKey{Name: "dev-lang/python", Slot: slot}
			entry, ok := result[key]
			if !ok {
				t.Fatalf("result missing dev-lang/python:%s", slot)
			}
			if entry.Package.Slot.Name != slot {
				t.Errorf("python slot = %s, want %s", entry.Package.Slot.Name, slot)
			}
		})
	}

	// Resolve without slot constraint — should pick one version
	t.Run("any_slot", func(t *testing.T) {
		result := resolveAndAssert(t, res, []string{"dev-lang/python"})

		count := 0
		for k := range result {
			if k.Name == "dev-lang/python" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected exactly 1 python in result when no slot specified, got %d", count)
		}
	})
}

func BenchmarkClosedFixture_Resolve(b *testing.B) {
	cacheDir := filepath.Join(closedFixtureDir, "metadata", "md5-cache")
	info, err := os.Stat(cacheDir)
	if err != nil || !info.IsDir() {
		b.Skip("closed fixture not available")
	}

	absPath, err := filepath.Abs(closedFixtureDir)
	if err != nil {
		b.Fatalf("failed to get absolute path: %v", err)
	}

	r, err := repo.NewPortageRepository(absPath)
	if err != nil {
		b.Fatalf("failed to load fixture: %v", err)
	}

	cases := []struct {
		name  string
		atoms []string
	}{
		{"libassuan", []string{"dev-libs/libassuan"}},
		{"sqlite", []string{"dev-db/sqlite"}},
		{"multi-root", []string{"dev-libs/libassuan", "dev-db/sqlite"}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			for range b.N {
				res := solver.NewResolverWithFilters(r, nil, []string{"amd64", "~amd64"})
				_, err := res.Resolve(tc.atoms)
				if err != nil {
					b.Fatalf("resolve failed: %v", err)
				}
			}
		})
	}
}
