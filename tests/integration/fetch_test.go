//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/grpmsoft/grpm/internal/repo"
)

// TestFetch_SrcURIParsing tests SRC_URI parsing with variable expansion.
//
// This validates the v0.7.3 fix for proper SRC_URI extraction with
// package metadata variables (P, PN, PV, etc.).
func TestFetch_SrcURIParsing(t *testing.T) {
	tests := []struct {
		name          string
		category      string
		pkgName       string
		version       string
		srcURI        string
		expectedFiles []string
		expectedURLs  map[string]string // filename -> expected URL substring
	}{
		{
			name:          "simple tarball",
			category:      "app-misc",
			pkgName:       "hello",
			version:       "2.10",
			srcURI:        `SRC_URI="https://ftp.gnu.org/gnu/hello/${P}.tar.gz"`,
			expectedFiles: []string{"hello-2.10.tar.gz"},
			expectedURLs: map[string]string{
				"hello-2.10.tar.gz": "ftp.gnu.org/gnu/hello",
			},
		},
		{
			name:     "multiple files",
			category: "sys-libs",
			pkgName:  "zlib",
			version:  "1.3.1",
			srcURI: `SRC_URI="
				https://zlib.net/${P}.tar.xz
				https://github.com/madler/zlib/releases/download/v${PV}/${P}.tar.xz
			"`,
			expectedFiles: []string{"zlib-1.3.1.tar.xz"},
			expectedURLs: map[string]string{
				"zlib-1.3.1.tar.xz": "zlib.net",
			},
		},
		{
			name:          "arrow rename syntax",
			category:      "dev-libs",
			pkgName:       "openssl",
			version:       "3.1.0",
			srcURI:        `SRC_URI="https://www.openssl.org/source/${P}.tar.gz -> ${P}-renamed.tar.gz"`,
			expectedFiles: []string{"openssl-3.1.0-renamed.tar.gz"},
			expectedURLs: map[string]string{
				"openssl-3.1.0-renamed.tar.gz": "openssl.org/source",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			repoDir := filepath.Join(tmpDir, "gentoo")
			pkgDir := filepath.Join(repoDir, tc.category, tc.pkgName)
			if err := os.MkdirAll(pkgDir, 0755); err != nil {
				t.Fatalf("failed to create package dir: %v", err)
			}

			ebuildContent := `EAPI=8
DESCRIPTION="Test package"
HOMEPAGE="https://example.com"
` + tc.srcURI + `
LICENSE="MIT"
SLOT="0"
KEYWORDS="amd64"
`
			ebuildPath := filepath.Join(pkgDir, tc.pkgName+"-"+tc.version+".ebuild")
			if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
				t.Fatalf("failed to write ebuild: %v", err)
			}

			// Parse the ebuild
			content, err := os.ReadFile(ebuildPath)
			if err != nil {
				t.Fatalf("failed to read ebuild: %v", err)
			}

			meta := repo.NewPackageMetadata(tc.category, tc.pkgName, tc.version)
			parser := repo.NewEbuildParserWithMetadata(string(content), meta)
			srcURI := parser.ExtractVariable("SRC_URI")

			if srcURI == "" {
				t.Fatal("SRC_URI not extracted")
			}

			// Parse SRC_URI entries
			vars := map[string]string{
				"P": meta.P, "PN": meta.PN, "PV": meta.PV,
				"PR": meta.PR, "PVR": meta.PVR, "PF": meta.PF,
				"CATEGORY": meta.Category,
			}

			entries, err := repo.ParseSrcURI(srcURI, nil, vars)
			if err != nil {
				t.Fatalf("ParseSrcURI failed: %v", err)
			}

			// Verify expected files
			fileMap := make(map[string]bool)
			for _, entry := range entries {
				fileMap[entry.Filename] = true
			}

			for _, expectedFile := range tc.expectedFiles {
				if !fileMap[expectedFile] {
					t.Errorf("expected file %q not found in parsed entries", expectedFile)
				}
			}

			// Verify URLs contain expected substrings
			for _, entry := range entries {
				if expectedURL, ok := tc.expectedURLs[entry.Filename]; ok {
					if entry.URL == "" {
						t.Errorf("file %q has no URL", entry.Filename)
					} else if !contains(entry.URL, expectedURL) {
						t.Errorf("file %q URL %q does not contain %q",
							entry.Filename, entry.URL, expectedURL)
					}
				}
			}
		})
	}
}

// TestFetch_SrcURIConditionals tests USE flag conditional handling in SRC_URI.
//
// This validates the v0.7.3 fix where nil activeFlags includes ALL conditionals,
// which is essential for fetching all distfiles regardless of USE flags.
func TestFetch_SrcURIConditionals(t *testing.T) {
	tests := []struct {
		name          string
		srcURI        string
		activeFlags   map[string]bool
		expectedFiles []string
		excludedFiles []string
	}{
		{
			name: "nil flags includes all conditionals",
			srcURI: `
				https://example.com/main.tar.gz
				doc? ( https://example.com/docs.tar.gz )
				test? ( https://example.com/tests.tar.gz )
			`,
			activeFlags:   nil, // nil = include everything
			expectedFiles: []string{"main.tar.gz", "docs.tar.gz", "tests.tar.gz"},
			excludedFiles: []string{},
		},
		{
			name: "empty flags excludes conditionals",
			srcURI: `
				https://example.com/main.tar.gz
				doc? ( https://example.com/docs.tar.gz )
			`,
			activeFlags:   map[string]bool{}, // empty = exclude conditionals
			expectedFiles: []string{"main.tar.gz"},
			excludedFiles: []string{"docs.tar.gz"},
		},
		{
			name: "specific flags enabled",
			srcURI: `
				https://example.com/main.tar.gz
				doc? ( https://example.com/docs.tar.gz )
				test? ( https://example.com/tests.tar.gz )
			`,
			activeFlags:   map[string]bool{"doc": true},
			expectedFiles: []string{"main.tar.gz", "docs.tar.gz"},
			excludedFiles: []string{"tests.tar.gz"},
		},
		{
			name: "negated conditionals",
			srcURI: `
				https://example.com/main.tar.gz
				!minimal? ( https://example.com/extra.tar.gz )
			`,
			activeFlags:   map[string]bool{"minimal": false},
			expectedFiles: []string{"main.tar.gz", "extra.tar.gz"},
			excludedFiles: []string{},
		},
		{
			name: "verify-sig pattern (real-world)",
			srcURI: `
				https://zlib.net/zlib-1.3.1.tar.xz
				verify-sig? (
					https://zlib.net/zlib-1.3.1.tar.xz.asc
				)
			`,
			activeFlags:   nil, // fetch command uses nil
			expectedFiles: []string{"zlib-1.3.1.tar.xz", "zlib-1.3.1.tar.xz.asc"},
			excludedFiles: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entries, err := repo.ParseSrcURI(tc.srcURI, tc.activeFlags, nil)
			if err != nil {
				t.Fatalf("ParseSrcURI failed: %v", err)
			}

			fileMap := make(map[string]bool)
			for _, entry := range entries {
				fileMap[entry.Filename] = true
			}

			for _, expected := range tc.expectedFiles {
				if !fileMap[expected] {
					t.Errorf("expected file %q not found", expected)
				}
			}

			for _, excluded := range tc.excludedFiles {
				if fileMap[excluded] {
					t.Errorf("file %q should be excluded", excluded)
				}
			}
		})
	}
}

// TestFetch_RealRepository tests SRC_URI parsing against real Gentoo packages.
//
// This test requires a real Gentoo repository at /var/db/repos/gentoo.
func TestFetch_RealRepository(t *testing.T) {
	repoPath := getRepoPath()
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		t.Skip("Gentoo repository not found at", repoPath)
	}

	// Test packages known to have interesting SRC_URI patterns
	packages := []struct {
		atom        string
		minFiles    int
		description string
	}{
		{"app-misc/hello", 1, "simple GNU hello"},
		{"sys-libs/zlib", 1, "compression library with signatures"},
		{"dev-libs/openssl", 1, "crypto library"},
	}

	for _, pkg := range packages {
		t.Run(pkg.atom, func(t *testing.T) {
			portageRepo, err := repo.NewPortageRepository(repoPath)
			if err != nil {
				t.Fatalf("NewPortageRepository failed: %v", err)
			}

			loadedPkg, err := portageRepo.LoadPackage(pkg.atom)
			if err != nil {
				t.Skipf("package %s not found: %v", pkg.atom, err)
			}

			// Extract and parse SRC_URI
			parts := splitAtom(pkg.atom)
			if len(parts) != 2 {
				t.Fatalf("invalid atom: %s", pkg.atom)
			}
			category, pkgName := parts[0], parts[1]

			meta := repo.NewPackageMetadata(category, pkgName, loadedPkg.Version)

			// Read ebuild content
			ebuildPath := filepath.Join(repoPath, category, pkgName,
				pkgName+"-"+loadedPkg.Version+".ebuild")
			content, err := os.ReadFile(ebuildPath)
			if err != nil {
				t.Fatalf("failed to read ebuild: %v", err)
			}

			parser := repo.NewEbuildParserWithMetadata(string(content), meta)
			srcURI := parser.ExtractVariable("SRC_URI")

			if srcURI == "" {
				t.Skip("package has no SRC_URI")
			}

			vars := map[string]string{
				"P": meta.P, "PN": meta.PN, "PV": meta.PV,
				"PR": meta.PR, "PVR": meta.PVR, "PF": meta.PF,
				"CATEGORY": meta.Category,
			}

			entries, err := repo.ParseSrcURI(srcURI, nil, vars)
			if err != nil {
				t.Fatalf("ParseSrcURI failed: %v", err)
			}

			if len(entries) < pkg.minFiles {
				t.Errorf("expected at least %d files, got %d",
					pkg.minFiles, len(entries))
			}

			t.Logf("%s (%s): %d distfiles", pkg.atom, loadedPkg.Version, len(entries))
			for _, entry := range entries {
				t.Logf("  - %s", entry.Filename)
			}
		})
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func splitAtom(atom string) []string {
	for i, c := range atom {
		if c == '/' {
			return []string{atom[:i], atom[i+1:]}
		}
	}
	return nil
}

func getRepoPath() string {
	if path := os.Getenv("GRPM_REPO_PATH"); path != "" {
		return path
	}
	return DefaultRepoPath
}
