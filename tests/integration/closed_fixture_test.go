package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/repo"
	"github.com/grpmsoft/grpm/internal/solver"
)

// closedFixtureDir points to the transitively closed fixture.
// All packages in this fixture have their transitive deps present,
// so Resolve() must return SAT with postPass=0.
const closedFixtureDir = "../../tests/fixtures/gentoo-closed"

// loadClosedFixtureRepo loads the closed fixture repository.
// Skips the test if the fixture is not available (e.g., running from wrong dir).
func loadClosedFixtureRepo(t *testing.T) repo.Repository {
	t.Helper()

	cacheDir := filepath.Join(closedFixtureDir, "metadata", "md5-cache")
	info, err := os.Stat(cacheDir)
	if err != nil || !info.IsDir() {
		t.Skip("closed fixture not available — run from repo root with tests/fixtures/gentoo-closed/metadata/md5-cache")
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

// TestClosedFixture_HelloResolves validates zero-dep baseline.
// app-misc/hello has no dependencies, so it must always resolve.
func TestClosedFixture_HelloResolves(t *testing.T) {
	r := loadClosedFixtureRepo(t)
	res := solver.NewResolver(r)

	start := time.Now()
	result, err := res.Resolve([]string{"app-misc/hello"})
	elapsed := time.Since(start)

	t.Logf("app-misc/hello: %v, %d packages, postPass=%d", elapsed, len(result), res.PostPassAdded)

	if err != nil {
		t.Fatalf("app-misc/hello must resolve (UNSAT is a test failure): %v", err)
	}
	if res.PostPassAdded != 0 {
		t.Errorf("PostPassAdded=%d, want 0 — SAT encoding incomplete", res.PostPassAdded)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 package, got %d", len(result))
	}
}

// TestClosedFixture_LibassuanResolves validates a two-package dependency chain.
// libassuan depends on >=dev-libs/libgpg-error-1.33, and libgpg-error-1.51
// is present in the fixture. This MUST resolve to SAT.
func TestClosedFixture_LibassuanResolves(t *testing.T) {
	r := loadClosedFixtureRepo(t)
	res := solver.NewResolver(r)

	start := time.Now()
	result, err := res.Resolve([]string{"dev-libs/libassuan"})
	elapsed := time.Since(start)

	t.Logf("dev-libs/libassuan: %v, %d packages, postPass=%d", elapsed, len(result), res.PostPassAdded)

	if err != nil {
		t.Fatalf("dev-libs/libassuan must resolve (closed fixture): %v", err)
	}
	if res.PostPassAdded != 0 {
		t.Errorf("PostPassAdded=%d, want 0 — SAT encoding incomplete", res.PostPassAdded)
	}

	// Result is keyed by packageSlotKey: "name" for slot 0, "name:slot" otherwise.
	if _, ok := result["dev-libs/libassuan"]; !ok {
		t.Errorf("result missing dev-libs/libassuan")
	}
	if _, ok := result["dev-libs/libgpg-error"]; !ok {
		t.Errorf("result missing dev-libs/libgpg-error (transitive dep)")
	}
	// Verify versions
	if p, ok := result["dev-libs/libassuan"]; ok && p.Version != "2.5.7" {
		t.Errorf("libassuan version = %s, want 2.5.7", p.Version)
	}
	if p, ok := result["dev-libs/libgpg-error"]; ok && p.Version != "1.51" {
		t.Errorf("libgpg-error version = %s, want 1.51", p.Version)
	}
}

// TestClosedFixture_NpthResolves validates another chain through libgpg-error.
// npth depends on >=dev-libs/libgpg-error-1.17 (a different version constraint
// than libassuan's >=1.33, but both satisfied by 1.51).
func TestClosedFixture_NpthResolves(t *testing.T) {
	r := loadClosedFixtureRepo(t)
	res := solver.NewResolver(r)

	start := time.Now()
	result, err := res.Resolve([]string{"dev-libs/npth"})
	elapsed := time.Since(start)

	t.Logf("dev-libs/npth: %v, %d packages, postPass=%d", elapsed, len(result), res.PostPassAdded)

	if err != nil {
		t.Fatalf("dev-libs/npth must resolve (closed fixture): %v", err)
	}
	if res.PostPassAdded != 0 {
		t.Errorf("PostPassAdded=%d, want 0", res.PostPassAdded)
	}

	// npth + libgpg-error = 2 packages
	if len(result) < 2 {
		t.Errorf("expected at least 2 packages, got %d", len(result))
	}
}

// TestClosedFixture_VirtualPkgconfigResolves validates virtual -> concrete chain.
// virtual/pkgconfig depends on dev-util/pkgconf.
func TestClosedFixture_VirtualPkgconfigResolves(t *testing.T) {
	r := loadClosedFixtureRepo(t)
	res := solver.NewResolver(r)

	start := time.Now()
	result, err := res.Resolve([]string{"virtual/pkgconfig"})
	elapsed := time.Since(start)

	t.Logf("virtual/pkgconfig: %v, %d packages, postPass=%d", elapsed, len(result), res.PostPassAdded)

	if err != nil {
		t.Fatalf("virtual/pkgconfig must resolve (closed fixture): %v", err)
	}
	if res.PostPassAdded != 0 {
		t.Errorf("PostPassAdded=%d, want 0", res.PostPassAdded)
	}

	// virtual/pkgconfig + dev-util/pkgconf = 2 packages
	if len(result) < 2 {
		t.Errorf("expected at least 2 packages, got %d", len(result))
	}
}

// TestClosedFixture_ZlibResolves validates a leaf package with no active deps.
// zlib's DEPEND/RDEPEND are blockers only (skipped by parser).
func TestClosedFixture_ZlibResolves(t *testing.T) {
	r := loadClosedFixtureRepo(t)
	res := solver.NewResolver(r)

	start := time.Now()
	result, err := res.Resolve([]string{"sys-libs/zlib"})
	elapsed := time.Since(start)

	t.Logf("sys-libs/zlib: %v, %d packages, postPass=%d", elapsed, len(result), res.PostPassAdded)

	if err != nil {
		t.Fatalf("sys-libs/zlib must resolve (closed fixture): %v", err)
	}
	if res.PostPassAdded != 0 {
		t.Errorf("PostPassAdded=%d, want 0", res.PostPassAdded)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 package, got %d", len(result))
	}
}

// TestClosedFixture_MultiRoot validates resolving two roots simultaneously.
// Both libassuan and npth depend on libgpg-error — the solver must handle
// the shared dependency correctly without duplication.
func TestClosedFixture_MultiRoot(t *testing.T) {
	r := loadClosedFixtureRepo(t)
	res := solver.NewResolver(r)

	start := time.Now()
	result, err := res.Resolve([]string{"dev-libs/libassuan", "dev-libs/npth"})
	elapsed := time.Since(start)

	t.Logf("libassuan+npth: %v, %d packages, postPass=%d", elapsed, len(result), res.PostPassAdded)

	if err != nil {
		t.Fatalf("multi-root resolve must succeed (closed fixture): %v", err)
	}
	if res.PostPassAdded != 0 {
		t.Errorf("PostPassAdded=%d, want 0", res.PostPassAdded)
	}

	// libassuan + npth + libgpg-error = 3 packages
	if len(result) < 3 {
		t.Errorf("expected at least 3 packages, got %d", len(result))
	}
}

// TestClosedFixture_AllPackagesResolve iterates over every package in the
// closed fixture and asserts that each one resolves to SAT.
// This is the UNSAT-as-failure regression gate.
func TestClosedFixture_AllPackagesResolve(t *testing.T) {
	r := loadClosedFixtureRepo(t)

	packages := []string{
		"app-misc/hello",
		"dev-libs/libgpg-error",
		"dev-libs/libassuan",
		"dev-libs/npth",
		"sys-libs/zlib",
		"dev-util/pkgconf",
		"virtual/pkgconfig",
	}

	for _, pkg := range packages {
		t.Run(pkg, func(t *testing.T) {
			res := solver.NewResolver(r)

			start := time.Now()
			result, err := res.Resolve([]string{pkg})
			elapsed := time.Since(start)

			status := "SAT"
			if err != nil {
				status = "UNSAT"
			}
			t.Logf("%s: %v (%s, %d pkgs, postPass=%d)",
				pkg, elapsed, status, len(result), res.PostPassAdded)

			if err != nil {
				t.Fatalf("UNSAT is a test failure in closed fixture: %v", err)
			}
			if res.PostPassAdded != 0 {
				t.Errorf("PostPassAdded=%d, want 0 — SAT encoding has gaps", res.PostPassAdded)
			}
		})
	}
}

// BenchmarkClosedFixture_Resolve benchmarks resolve time on the closed fixture.
// This establishes a performance baseline for SAT resolution on small trees.
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
		{"hello", []string{"app-misc/hello"}},
		{"libassuan", []string{"dev-libs/libassuan"}},
		{"npth", []string{"dev-libs/npth"}},
		{"virtual-pkgconfig", []string{"virtual/pkgconfig"}},
		{"multi-root", []string{"dev-libs/libassuan", "dev-libs/npth"}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			for range b.N {
				res := solver.NewResolver(r)
				_, err := res.Resolve(tc.atoms)
				if err != nil {
					b.Fatalf("resolve failed: %v", err)
				}
			}
		})
	}
}
