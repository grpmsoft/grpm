package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/grpmsoft/grpm/internal/config"
	"github.com/grpmsoft/grpm/internal/fetch"
	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/solver"
)

func TestParseJobsFromMakeOpts(t *testing.T) {
	app := &App{}

	tests := []struct {
		name     string
		makeOpts string
		expected int
	}{
		{
			name:     "empty returns default",
			makeOpts: "",
			expected: 4,
		},
		{
			name:     "simple -j4",
			makeOpts: "-j4",
			expected: 4,
		},
		{
			name:     "simple -j8",
			makeOpts: "-j8",
			expected: 8,
		},
		{
			name:     "double digit -j16",
			makeOpts: "-j16",
			expected: 16,
		},
		{
			name:     "with load average -j8 -l4",
			makeOpts: "-j8 -l4",
			expected: 8,
		},
		{
			name:     "other flags before -j",
			makeOpts: "-l4 -j12",
			expected: 12,
		},
		{
			name:     "no -j flag returns default",
			makeOpts: "-l4",
			expected: 4,
		},
		{
			name:     "invalid -j returns default",
			makeOpts: "-jX",
			expected: 4,
		},
		{
			name:     "-j1",
			makeOpts: "-j1",
			expected: 1,
		},
		{
			name:     "triple digit -j128",
			makeOpts: "-j128",
			expected: 128,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := app.parseJobsFromMakeOpts(tt.makeOpts)
			if result != tt.expected {
				t.Errorf("parseJobsFromMakeOpts(%q) = %d, expected %d", tt.makeOpts, result, tt.expected)
			}
		})
	}
}

func TestCreateFetcherWithConfig(t *testing.T) {
	app := &App{verbose: false}

	t.Run("uses config mirrors when set", func(t *testing.T) {
		cfg := &config.Config{
			MakeConf: &config.MakeConf{
				GENTOO_MIRRORS: []string{
					"https://custom.mirror.com/",
				},
			},
		}

		fetcher := app.createFetcherWithConfig("/tmp/distfiles", cfg)
		if fetcher == nil {
			t.Fatal("createFetcherWithConfig returned nil")
		}

		// Type assertion to verify it's an HTTPDownloader
		_, ok := fetcher.(*fetch.HTTPDownloader)
		if !ok {
			t.Error("Expected *fetch.HTTPDownloader")
		}
	})

	t.Run("falls back to defaults when mirrors empty", func(t *testing.T) {
		cfg := &config.Config{
			MakeConf: &config.MakeConf{
				GENTOO_MIRRORS: []string{},
			},
		}

		fetcher := app.createFetcherWithConfig("/tmp/distfiles", cfg)
		if fetcher == nil {
			t.Fatal("createFetcherWithConfig returned nil")
		}
	})

	t.Run("handles nil MakeConf", func(t *testing.T) {
		cfg := &config.Config{
			MakeConf: nil,
		}

		fetcher := app.createFetcherWithConfig("/tmp/distfiles", cfg)
		if fetcher == nil {
			t.Fatal("createFetcherWithConfig returned nil")
		}
	})
}

func TestLoadPortageConfig(t *testing.T) {
	t.Run("returns default config when /etc/portage missing", func(t *testing.T) {
		app := &App{verbose: false}
		cfg := app.loadPortageConfig()

		if cfg == nil {
			t.Fatal("loadPortageConfig returned nil")
		}
		if cfg.MakeConf == nil {
			t.Fatal("MakeConf is nil")
		}

		// Should have default values
		if cfg.GetDistDir() != "/var/cache/distfiles" {
			t.Errorf("Expected default DISTDIR, got %s", cfg.GetDistDir())
		}
	})

	t.Run("loads config from temp directory", func(t *testing.T) {
		// Create a temporary /etc/portage structure
		tmpDir := t.TempDir()
		makeConfPath := filepath.Join(tmpDir, "make.conf")
		content := `GENTOO_MIRRORS="https://test.mirror.com/"
DISTDIR="/test/distfiles"
`
		if err := os.WriteFile(makeConfPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		// We can't easily test the actual loadPortageConfig since it reads from
		// /etc/portage, but we can test the config loading itself
		cfg, err := config.LoadConfig(tmpDir)
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		mirrors := cfg.GetGentooMirrors()
		if len(mirrors) != 1 || mirrors[0] != "https://test.mirror.com/" {
			t.Errorf("Expected mirror https://test.mirror.com/, got %v", mirrors)
		}

		if cfg.GetDistDir() != "/test/distfiles" {
			t.Errorf("Expected DISTDIR /test/distfiles, got %s", cfg.GetDistDir())
		}
	})
}

func TestCreateFetcher(t *testing.T) {
	// Test that the deprecated createFetcher still works
	app := &App{verbose: false}

	fetcher := app.createFetcher("/tmp/distfiles")
	if fetcher == nil {
		t.Fatal("createFetcher returned nil")
	}

	// Should be an HTTPDownloader
	_, ok := fetcher.(*fetch.HTTPDownloader)
	if !ok {
		t.Error("Expected *fetch.HTTPDownloader")
	}
}

func TestGetOrCreatePackageDBWithRoot(t *testing.T) {
	t.Run("creates VarDB in custom root", func(t *testing.T) {
		tmpDir := t.TempDir()
		app := &App{verbose: false}

		db, err := app.getOrCreatePackageDBWithRoot(tmpDir)
		if err != nil {
			t.Fatalf("getOrCreatePackageDBWithRoot failed: %v", err)
		}
		if db == nil {
			t.Fatal("returned nil database")
		}

		// Verify VarDB directory was created
		vardbPath := filepath.Join(tmpDir, "var/db/pkg")
		if _, err := os.Stat(vardbPath); os.IsNotExist(err) {
			t.Errorf("VarDB directory not created at %s", vardbPath)
		}
	})

	t.Run("default root uses /var/db/pkg", func(t *testing.T) {
		app := &App{verbose: false}

		// This test verifies the getOrCreatePackageDB uses "/" as default
		// We can't actually test writing to /var/db/pkg without root permissions
		// so we just verify the function doesn't panic
		_, _ = app.getOrCreatePackageDB()
		// No assertion needed - just verify it doesn't crash
	})

	t.Run("handles nested root paths", func(t *testing.T) {
		tmpDir := t.TempDir()
		nestedRoot := filepath.Join(tmpDir, "chroot", "gentoo")
		app := &App{verbose: false}

		db, err := app.getOrCreatePackageDBWithRoot(nestedRoot)
		if err != nil {
			t.Fatalf("getOrCreatePackageDBWithRoot failed: %v", err)
		}
		if db == nil {
			t.Fatal("returned nil database")
		}

		// Verify nested VarDB directory was created
		vardbPath := filepath.Join(nestedRoot, "var/db/pkg")
		if _, err := os.Stat(vardbPath); os.IsNotExist(err) {
			t.Errorf("VarDB directory not created at %s", vardbPath)
		}
	})
}

func TestParallelBuildOptionsRoot(t *testing.T) {
	t.Run("root field is set in options", func(t *testing.T) {
		opts := &parallelBuildOptions{
			repoPath: "/var/db/repos/gentoo",
			distDir:  "/var/cache/distfiles",
			tmpDir:   "/var/tmp/portage",
			root:     "/mnt/gentoo",
		}

		if opts.root != "/mnt/gentoo" {
			t.Errorf("Expected root /mnt/gentoo, got %s", opts.root)
		}
	})

	t.Run("default root is /", func(t *testing.T) {
		opts := &parallelBuildOptions{
			root: "/",
		}

		if opts.root != "/" {
			t.Errorf("Expected root /, got %s", opts.root)
		}
	})
}

func TestFilterTargetPackages(t *testing.T) {
	app := &App{verbose: false}

	// Create test packages
	hello := pkg.NewPackage("app-misc/hello", "2.10", "0")
	zlib := pkg.NewPackage("sys-libs/zlib", "1.3", "0")
	gcc := pkg.NewPackage("sys-devel/gcc", "13.4.1_p20250807", "13")

	sk := func(name, slot string) pkg.SlotKey { return pkg.SlotKey{Name: name, Slot: slot} }

	tests := []struct {
		name             string
		solution         solver.ResolveResult
		packages         []string
		wantLen          int
		shouldBeFiltered []string
		shouldRemain     []string
	}{
		{
			name:             "filter simple package name",
			solution:         solver.ResolveResult{sk("app-misc/hello", "0"): hello, sk("sys-libs/zlib", "0"): zlib},
			packages:         []string{"app-misc/hello"},
			wantLen:          1,
			shouldBeFiltered: []string{"app-misc/hello"},
			shouldRemain:     []string{"sys-libs/zlib"},
		},
		{
			name:             "filter versioned atom",
			solution:         solver.ResolveResult{sk("sys-devel/gcc", "13"): gcc, sk("sys-libs/zlib", "0"): zlib},
			packages:         []string{"=sys-devel/gcc-13.4.1_p20250807"},
			wantLen:          1,
			shouldBeFiltered: []string{"sys-devel/gcc"},
			shouldRemain:     []string{"sys-libs/zlib"},
		},
		{
			name:             "filter multiple packages",
			solution:         solver.ResolveResult{sk("app-misc/hello", "0"): hello, sk("sys-libs/zlib", "0"): zlib, sk("sys-devel/gcc", "13"): gcc},
			packages:         []string{"app-misc/hello", "sys-devel/gcc"},
			wantLen:          1,
			shouldBeFiltered: []string{"app-misc/hello", "sys-devel/gcc"},
			shouldRemain:     []string{"sys-libs/zlib"},
		},
		{
			name:             "filter with >= operator",
			solution:         solver.ResolveResult{sk("sys-devel/gcc", "13"): gcc, sk("sys-libs/zlib", "0"): zlib},
			packages:         []string{">=sys-devel/gcc-13.0.0"},
			wantLen:          1,
			shouldBeFiltered: []string{"sys-devel/gcc"},
			shouldRemain:     []string{"sys-libs/zlib"},
		},
		{
			name:             "all packages filtered",
			solution:         solver.ResolveResult{sk("app-misc/hello", "0"): hello},
			packages:         []string{"app-misc/hello"},
			wantLen:          0,
			shouldBeFiltered: []string{"app-misc/hello"},
			shouldRemain:     []string{},
		},
		{
			name:             "no packages filtered (target not in solution)",
			solution:         solver.ResolveResult{sk("sys-libs/zlib", "0"): zlib},
			packages:         []string{"app-misc/hello"},
			wantLen:          1,
			shouldBeFiltered: []string{},
			shouldRemain:     []string{"sys-libs/zlib"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := app.filterTargetPackages(tt.solution, tt.packages)

			if len(result) != tt.wantLen {
				t.Errorf("filterTargetPackages() returned %d packages, want %d", len(result), tt.wantLen)
			}

			for _, name := range tt.shouldBeFiltered {
				if len(result.FindByName(name)) > 0 {
					t.Errorf("filterTargetPackages() should have filtered %s, but it remains", name)
				}
			}

			for _, name := range tt.shouldRemain {
				if len(result.FindByName(name)) == 0 {
					t.Errorf("filterTargetPackages() should have kept %s, but it was filtered", name)
				}
			}
		})
	}
}

func TestTopologicalSort(t *testing.T) {
	sk := func(name, slot string) pkg.SlotKey { return pkg.SlotKey{Name: name, Slot: slot} }

	t.Run("deps before dependents", func(t *testing.T) {
		zlib := pkg.NewPackage("sys-libs/zlib", "1.3", "0")
		hello := pkg.NewPackage("app-misc/hello", "2.10", "0")
		hello.Deps = []pkg.Constraint{{Name: "sys-libs/zlib"}}

		solution := solver.ResolveResult{
			sk("app-misc/hello", "0"): hello,
			sk("sys-libs/zlib", "0"):  zlib,
		}

		order := topologicalSort(solution)
		zlibIdx, helloIdx := -1, -1
		for i, key := range order {
			if key.Name == "sys-libs/zlib" {
				zlibIdx = i
			}
			if key.Name == "app-misc/hello" {
				helloIdx = i
			}
		}
		if zlibIdx >= helloIdx {
			t.Errorf("zlib (idx=%d) should come before hello (idx=%d)", zlibIdx, helloIdx)
		}
	})

	t.Run("chain A->B->C", func(t *testing.T) {
		c := pkg.NewPackage("cat/c", "1.0", "0")
		b := pkg.NewPackage("cat/b", "1.0", "0")
		b.Deps = []pkg.Constraint{{Name: "cat/c"}}
		a := pkg.NewPackage("cat/a", "1.0", "0")
		a.Deps = []pkg.Constraint{{Name: "cat/b"}}

		solution := solver.ResolveResult{sk("cat/a", "0"): a, sk("cat/b", "0"): b, sk("cat/c", "0"): c}
		order := topologicalSort(solution)

		idx := make(map[string]int)
		for i, k := range order {
			idx[k.Name] = i
		}
		if idx["cat/c"] >= idx["cat/b"] || idx["cat/b"] >= idx["cat/a"] {
			t.Errorf("expected c < b < a, got order: %v", order)
		}
	})

	t.Run("no deps — deterministic sorted order", func(t *testing.T) {
		solution := solver.ResolveResult{
			sk("z/pkg", "0"): pkg.NewPackage("z/pkg", "1.0", "0"),
			sk("a/pkg", "0"): pkg.NewPackage("a/pkg", "1.0", "0"),
			sk("m/pkg", "0"): pkg.NewPackage("m/pkg", "1.0", "0"),
		}
		order := topologicalSort(solution)
		if order[0].Name != "a/pkg" || order[1].Name != "m/pkg" || order[2].Name != "z/pkg" {
			t.Errorf("expected alphabetical order, got %v", order)
		}
	})

	t.Run("empty solution", func(t *testing.T) {
		order := topologicalSort(solver.ResolveResult{})
		if len(order) != 0 {
			t.Errorf("expected empty, got %v", order)
		}
	})

	t.Run("PDEPEND edges excluded — no false cycle", func(t *testing.T) {
		a := pkg.NewPackage("cat/a", "1.0", "0")
		b := pkg.NewPackage("cat/b", "1.0", "0")
		a.Deps = []pkg.Constraint{{Name: "cat/b", DepType: pkg.DepTypeRuntime}}
		b.Deps = []pkg.Constraint{{Name: "cat/a", DepType: pkg.DepTypePostMerge}}

		solution := solver.ResolveResult{sk("cat/a", "0"): a, sk("cat/b", "0"): b}
		order := topologicalSort(solution)

		idx := make(map[string]int)
		for i, k := range order {
			idx[k.Name] = i
		}
		if idx["cat/b"] >= idx["cat/a"] {
			t.Errorf("b should come before a (PDEPEND excluded), got order: %v", order)
		}
	})

	t.Run("real cycle — deterministic sorted fallback", func(t *testing.T) {
		a := pkg.NewPackage("cat/a", "1.0", "0")
		b := pkg.NewPackage("cat/b", "1.0", "0")
		c := pkg.NewPackage("cat/c", "1.0", "0")
		d := pkg.NewPackage("cat/d", "1.0", "0")
		a.Deps = []pkg.Constraint{{Name: "cat/b", DepType: pkg.DepTypeRuntime}}
		b.Deps = []pkg.Constraint{{Name: "cat/c", DepType: pkg.DepTypeRuntime}}
		c.Deps = []pkg.Constraint{{Name: "cat/a", DepType: pkg.DepTypeRuntime}}
		d.Deps = []pkg.Constraint{{Name: "cat/a", DepType: pkg.DepTypeRuntime}}

		solution := solver.ResolveResult{sk("cat/a", "0"): a, sk("cat/b", "0"): b, sk("cat/c", "0"): c, sk("cat/d", "0"): d}

		first := topologicalSort(solution)
		for i := range 20 {
			order := topologicalSort(solution)
			for j, k := range order {
				if k != first[j] {
					t.Fatalf("non-deterministic: run %d got %v, expected %v", i, order, first)
				}
			}
		}
		if len(first) != 4 {
			t.Errorf("expected 4 elements, got %d", len(first))
		}
	})
}
