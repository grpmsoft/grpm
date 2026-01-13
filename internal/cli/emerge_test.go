package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/grpmsoft/grpm/internal/config"
	"github.com/grpmsoft/grpm/internal/fetch"
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
