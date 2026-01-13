package repo

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadPackage_SelectsHighestVersion verifies that LoadPackage
// correctly selects the highest version when multiple ebuilds exist.
//
// This test was added after bug discovery: GRPM was selecting hello-2.12
// instead of hello-2.12.2 because versions were sorted alphabetically
// instead of using Portage version comparison.
//
// Reference: PMS Chapter 3 - Version Comparison Algorithm
func TestLoadPackage_SelectsHighestVersion(t *testing.T) {
	// Create temporary repository structure
	tmpDir := t.TempDir()

	// Create test package directory with multiple versions
	pkgDir := filepath.Join(tmpDir, "app-misc", "hello")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create package directory: %v", err)
	}

	// Test cases: pairs of (lower version, higher version)
	// These represent real-world scenarios where alphabetical sorting fails
	testCases := []struct {
		name         string
		versions     []string
		expectedBest string
		description  string
	}{
		{
			name:         "patch_version",
			versions:     []string{"2.12", "2.12.2"},
			expectedBest: "2.12.2",
			description:  "2.12.2 > 2.12 (additional patch component)",
		},
		{
			name:         "minor_version",
			versions:     []string{"1.9", "1.10"},
			expectedBest: "1.10",
			description:  "1.10 > 1.9 (numeric comparison, not alphabetic)",
		},
		{
			name:         "major_version",
			versions:     []string{"2.0", "10.0"},
			expectedBest: "10.0",
			description:  "10.0 > 2.0 (numeric comparison)",
		},
		{
			name:         "suffix_ordering",
			versions:     []string{"1.0_alpha", "1.0_beta", "1.0_rc1", "1.0"},
			expectedBest: "1.0",
			description:  "1.0 > 1.0_rc1 > 1.0_beta > 1.0_alpha (PMS suffix order)",
		},
		{
			name:         "revision",
			versions:     []string{"1.0", "1.0-r1", "1.0-r2"},
			expectedBest: "1.0-r2",
			description:  "1.0-r2 > 1.0-r1 > 1.0 (revision comparison)",
		},
		{
			name:         "complex_real_world",
			versions:     []string{"2.12", "2.12.1", "2.12.2", "2.9", "2.10"},
			expectedBest: "2.12.2",
			description:  "Real-world scenario with multiple versions",
		},
		{
			name:         "letter_suffix",
			versions:     []string{"1.0a", "1.0b", "1.0"},
			expectedBest: "1.0",
			description:  "1.0 > 1.0b > 1.0a (letter suffix)",
		},
		{
			name:         "post_patch",
			versions:     []string{"1.0", "1.0_p1", "1.0_p2"},
			expectedBest: "1.0_p2",
			description:  "1.0_p2 > 1.0_p1 > 1.0 (post-release patches)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clean package directory
			entries, _ := os.ReadDir(pkgDir)
			for _, e := range entries {
				os.Remove(filepath.Join(pkgDir, e.Name()))
			}

			// Create ebuild files for all versions
			for _, v := range tc.versions {
				ebuildPath := filepath.Join(pkgDir, "hello-"+v+".ebuild")
				content := `EAPI=8
DESCRIPTION="Test package"
SLOT="0"
`
				if err := os.WriteFile(ebuildPath, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to create ebuild %s: %v", ebuildPath, err)
				}
			}

			// Create repository and load package
			repo, err := NewPortageRepository(tmpDir)
			if err != nil {
				t.Fatalf("Failed to create repository: %v", err)
			}

			pkg, err := repo.LoadPackage("app-misc/hello")
			if err != nil {
				t.Fatalf("LoadPackage failed: %v", err)
			}

			if pkg.Version != tc.expectedBest {
				t.Errorf("Version selection failed: got %q, want %q\nDescription: %s\nVersions: %v",
					pkg.Version, tc.expectedBest, tc.description, tc.versions)
			}
		})
	}
}

// TestLoadPackage_SingleVersion verifies behavior with only one ebuild.
func TestLoadPackage_SingleVersion(t *testing.T) {
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "dev-libs", "single")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create package directory: %v", err)
	}

	// Create single ebuild
	ebuildPath := filepath.Join(pkgDir, "single-1.0.ebuild")
	content := `EAPI=8
DESCRIPTION="Single version package"
SLOT="0"
`
	if err := os.WriteFile(ebuildPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create ebuild: %v", err)
	}

	repo, err := NewPortageRepository(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	pkg, err := repo.LoadPackage("dev-libs/single")
	if err != nil {
		t.Fatalf("LoadPackage failed: %v", err)
	}

	if pkg.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", pkg.Version)
	}
}

// TestLoadPackage_NoEbuilds verifies error handling when no ebuilds exist.
func TestLoadPackage_NoEbuilds(t *testing.T) {
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "dev-libs", "empty")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create package directory: %v", err)
	}

	repo, err := NewPortageRepository(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	_, err = repo.LoadPackage("dev-libs/empty")
	if err == nil {
		t.Error("Expected error for package with no ebuilds, got nil")
	}
}

// BenchmarkLoadPackage_ManyVersions benchmarks version selection
// with a large number of versions to ensure sorting is efficient.
func BenchmarkLoadPackage_ManyVersions(b *testing.B) {
	tmpDir := b.TempDir()
	pkgDir := filepath.Join(tmpDir, "dev-libs", "many")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		b.Fatalf("Failed to create package directory: %v", err)
	}

	// Create 100 versions
	for i := 0; i < 100; i++ {
		major := i / 10
		minor := i % 10
		ebuildPath := filepath.Join(pkgDir, "many-"+string(rune('0'+major))+"."+string(rune('0'+minor))+".ebuild")
		content := `EAPI=8
DESCRIPTION="Benchmark package"
SLOT="0"
`
		if err := os.WriteFile(ebuildPath, []byte(content), 0644); err != nil {
			b.Fatalf("Failed to create ebuild: %v", err)
		}
	}

	repo, err := NewPortageRepository(tmpDir)
	if err != nil {
		b.Fatalf("Failed to create repository: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = repo.LoadPackage("dev-libs/many")
	}
}
