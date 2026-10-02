package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/repo"
	"github.com/grpmsoft/grpm/internal/solver"
)

const fixtureDir = "../../tests/fixtures/gentoo"

func hasFixtures() bool {
	info, err := os.Stat(filepath.Join(fixtureDir, "metadata", "md5-cache"))
	return err == nil && info.IsDir()
}

func loadFixtureRepo(t *testing.T) repo.Repository {
	t.Helper()
	if !hasFixtures() {
		t.Skip("gentoo fixtures not available — run from repo root with tests/fixtures/gentoo/metadata/md5-cache")
	}
	absPath, err := filepath.Abs(fixtureDir)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}
	r, err := repo.NewPortageRepository(absPath)
	if err != nil {
		t.Fatalf("failed to load fixture repo: %v", err)
	}
	return r
}

func TestRealTree_ResolveSimple(t *testing.T) {
	r := loadFixtureRepo(t)
	resolver := solver.NewResolver(r)

	start := time.Now()
	result, err := resolver.Resolve([]string{"app-misc/hello"})
	elapsed := time.Since(start)

	t.Logf("app-misc/hello: %v, %d packages", elapsed, len(result))
	if err != nil {
		t.Logf("expected: may fail due to missing transitive deps in fixture subset")
		return
	}
}

func TestRealTree_PythonMultiSlot(t *testing.T) {
	r := loadFixtureRepo(t)
	resolver := solver.NewResolver(r)

	start := time.Now()
	_, err := resolver.Resolve([]string{"dev-lang/python"})
	elapsed := time.Since(start)

	t.Logf("dev-lang/python (49 versions): %v", elapsed)
	if elapsed > 5*time.Second {
		t.Errorf("resolve took %v — exceeds 5s budget for 49-version package", elapsed)
	}
	if err != nil {
		t.Logf("resolve error (expected — fixture subset): %v", err)
	}
}

func TestRealTree_GCCMultiVersion(t *testing.T) {
	r := loadFixtureRepo(t)
	resolver := solver.NewResolver(r)

	start := time.Now()
	_, err := resolver.Resolve([]string{"sys-devel/gcc"})
	elapsed := time.Since(start)

	t.Logf("sys-devel/gcc (46 versions): %v", elapsed)
	if elapsed > 5*time.Second {
		t.Errorf("resolve took %v — exceeds 5s budget for 46-version package", elapsed)
	}
	if err != nil {
		t.Logf("resolve error (expected — fixture subset): %v", err)
	}
}

func TestRealTree_PerfBudget(t *testing.T) {
	r := loadFixtureRepo(t)

	packages := []string{
		"app-misc/hello",
		"sys-libs/zlib",
		"dev-libs/openssl",
		"dev-lang/python",
		"sys-devel/gcc",
	}

	for _, pkg := range packages {
		t.Run(pkg, func(t *testing.T) {
			resolver := solver.NewResolver(r)
			start := time.Now()
			result, err := resolver.Resolve([]string{pkg})
			elapsed := time.Since(start)

			status := "OK"
			if err != nil {
				status = "UNSAT"
			}
			t.Logf("%s: %v (%s, %d pkgs)",
				pkg, elapsed, status, len(result))

			if elapsed > 5*time.Second {
				t.Errorf("resolve exceeded 5s budget: %v", elapsed)
			}
		})
	}
}
