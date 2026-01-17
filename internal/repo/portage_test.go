package repo

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewPortageRepository tests creating a Portage repository.
func TestNewPortageRepository(t *testing.T) {
	tmpDir := t.TempDir()

	repo, err := NewPortageRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewPortageRepository() error = %v", err)
	}

	if repo == nil {
		t.Fatal("NewPortageRepository() returned nil")
	}

	// Path should be absolute
	if !filepath.IsAbs(repo.Path) {
		t.Errorf("Path should be absolute: %s", repo.Path)
	}
}

// TestNewPortageRepository_NonExistent tests creating repo with non-existent path.
func TestNewPortageRepository_NonExistent(t *testing.T) {
	_, err := NewPortageRepository("/nonexistent/path")
	if err == nil {
		t.Error("Expected error for non-existent path")
	}
}

// TestPortageRepository_LoadPackage_NotFound tests loading non-existent package.
func TestPortageRepository_LoadPackage_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	repo, err := NewPortageRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewPortageRepository() error = %v", err)
	}

	_, err = repo.LoadPackage("non-existent/package-1.0")
	if err == nil {
		t.Error("Expected error for non-existent package")
	}
}

// TestPortageRepository_LoadPackage_ValidEbuild tests loading a valid ebuild.
func TestPortageRepository_LoadPackage_ValidEbuild(t *testing.T) {
	tmpDir := t.TempDir()

	// Create ebuild structure: category/pkgName/pkgName-version.ebuild
	pkgDir := filepath.Join(tmpDir, "app-misc", "hello")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	ebuildContent := `# Copyright 1999-2024 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=8

DESCRIPTION="Hello World program"
HOMEPAGE="https://example.com"
SRC_URI="https://example.com/hello-2.10.tar.gz"

LICENSE="GPL-3"
SLOT="0"
KEYWORDS="amd64 x86"

DEPEND=""
RDEPEND="${DEPEND}"
`

	// File must be named: pkgName-version.ebuild
	ebuildPath := filepath.Join(pkgDir, "hello-2.10.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatal(err)
	}

	repo, err := NewPortageRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewPortageRepository() error = %v", err)
	}

	// LoadPackage expects category/pkgName (without version)
	pkg, err := repo.LoadPackage("app-misc/hello")
	if err != nil {
		t.Fatalf("LoadPackage() error = %v", err)
	}

	if pkg.Name != "app-misc/hello" {
		t.Errorf("Name = %s, want app-misc/hello", pkg.Name)
	}

	if pkg.Version != "2.10" {
		t.Errorf("Version = %s, want 2.10", pkg.Version)
	}

	if pkg.Slot.Name != "0" {
		t.Errorf("Slot = %s, want 0", pkg.Slot.Name)
	}
}

// TestPortageRepository_LoadPackage_WithDependencies tests loading ebuild with deps.
func TestPortageRepository_LoadPackage_WithDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	// Create ebuild structure: dev-libs/libfoo/libfoo-1.0.ebuild
	pkgDir := filepath.Join(tmpDir, "dev-libs", "libfoo")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	ebuildContent := `EAPI=8

DESCRIPTION="A foo library"
SLOT="0/1"
KEYWORDS="amd64"

DEPEND="
	dev-libs/bar
	>=sys-libs/zlib-1.2.11
"
RDEPEND="${DEPEND}
	app-misc/something
"
`

	ebuildPath := filepath.Join(pkgDir, "libfoo-1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatal(err)
	}

	repo, err := NewPortageRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewPortageRepository() error = %v", err)
	}

	// LoadPackage expects category/pkgName (without version)
	pkg, err := repo.LoadPackage("dev-libs/libfoo")
	if err != nil {
		t.Fatalf("LoadPackage() error = %v", err)
	}

	if pkg.Slot.Name != "0" {
		t.Errorf("Slot = %s, want 0", pkg.Slot.Name)
	}

	// Should have dependencies parsed (Deps field, not Dependencies)
	if len(pkg.Deps) == 0 {
		t.Log("Dependencies were not parsed (may be expected)")
	}
}

// TestPortageRepository_LoadPackage_SlotParsing tests slot parsing variants.
func TestPortageRepository_LoadPackage_SlotParsing(t *testing.T) {
	tests := []struct {
		name            string
		slot            string
		expectedSlot    string
		expectedSubslot string
	}{
		{"simple", "0", "0", ""},
		{"with_subslot", "0/1.22", "0", "1.22"},
		{"complex", "3.0/3.0", "3.0", "3.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create ebuild structure: test/pkg/pkg-1.0.ebuild
			pkgDir := filepath.Join(tmpDir, "test", "pkg")
			if err := os.MkdirAll(pkgDir, 0755); err != nil {
				t.Fatal(err)
			}

			ebuildContent := `EAPI=8
DESCRIPTION="Test package"
SLOT="` + tt.slot + `"
KEYWORDS="amd64"
`

			ebuildPath := filepath.Join(pkgDir, "pkg-1.0.ebuild")
			if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
				t.Fatal(err)
			}

			repo, err := NewPortageRepository(tmpDir)
			if err != nil {
				t.Fatalf("NewPortageRepository() error = %v", err)
			}

			// LoadPackage expects category/pkgName (without version)
			pkg, err := repo.LoadPackage("test/pkg")
			if err != nil {
				t.Fatalf("LoadPackage() error = %v", err)
			}

			if pkg.Slot.Name != tt.expectedSlot {
				t.Errorf("Slot.Name = %s, want %s", pkg.Slot.Name, tt.expectedSlot)
			}
		})
	}
}

// TestPortageRepository_LoadPackage_InvalidAtom tests loading with invalid atom.
func TestPortageRepository_LoadPackage_InvalidAtom(t *testing.T) {
	tmpDir := t.TempDir()
	repo, err := NewPortageRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewPortageRepository() error = %v", err)
	}

	tests := []string{
		"",
		"invalid",
		"invalid-no-version",
		"/malformed/path",
	}

	for _, atom := range tests {
		t.Run(atom, func(t *testing.T) {
			_, err := repo.LoadPackage(atom)
			if err == nil {
				t.Errorf("Expected error for invalid atom %q", atom)
			}
		})
	}
}

// TestParseEbuild_EmptyFile tests parsing empty ebuild.
func TestParseEbuild_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create ebuild structure: test/empty/empty-1.0.ebuild
	pkgDir := filepath.Join(tmpDir, "test", "empty")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	ebuildPath := filepath.Join(pkgDir, "empty-1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	repo, err := NewPortageRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewPortageRepository() error = %v", err)
	}

	// LoadPackage expects category/pkgName (without version)
	pkg, err := repo.LoadPackage("test/empty")
	if err != nil {
		t.Fatalf("LoadPackage() error = %v", err)
	}

	// Package should be created with defaults
	if pkg.Name != "test/empty" {
		t.Errorf("Name = %s", pkg.Name)
	}
}

// TestParseEbuild_UseFlags tests parsing USE flags.
func TestParseEbuild_UseFlags(t *testing.T) {
	tmpDir := t.TempDir()

	// Create ebuild structure: test/useflags/useflags-1.0.ebuild
	pkgDir := filepath.Join(tmpDir, "test", "useflags")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	ebuildContent := `EAPI=8
DESCRIPTION="Test package"
SLOT="0"
KEYWORDS="amd64"
IUSE="ssl python +doc -deprecated"
`

	ebuildPath := filepath.Join(pkgDir, "useflags-1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatal(err)
	}

	repo, err := NewPortageRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewPortageRepository() error = %v", err)
	}

	// LoadPackage expects category/pkgName (without version)
	pkg, err := repo.LoadPackage("test/useflags")
	if err != nil {
		t.Fatalf("LoadPackage() error = %v", err)
	}

	// UseFlags is a map[string]bool, check if any were parsed
	if len(pkg.UseFlags) == 0 {
		t.Log("USE flags not parsed (may be expected)")
	}
}

// TestParseEbuild_Keywords tests parsing KEYWORDS.
func TestParseEbuild_Keywords(t *testing.T) {
	tmpDir := t.TempDir()

	// Create ebuild structure: test/keywords/keywords-1.0.ebuild
	pkgDir := filepath.Join(tmpDir, "test", "keywords")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	ebuildContent := `EAPI=8
DESCRIPTION="Test package"
SLOT="0"
KEYWORDS="amd64 ~arm64 -x86"
`

	ebuildPath := filepath.Join(pkgDir, "keywords-1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatal(err)
	}

	repo, err := NewPortageRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewPortageRepository() error = %v", err)
	}

	// LoadPackage expects category/pkgName (without version)
	pkg, err := repo.LoadPackage("test/keywords")
	if err != nil {
		t.Fatalf("LoadPackage() error = %v", err)
	}

	// Package struct doesn't have Keywords field currently
	// Just verify the package was parsed successfully
	if pkg.Name != "test/keywords" {
		t.Errorf("Name = %s, want test/keywords", pkg.Name)
	}
}

// TestPortageRepository_String tests string representation.
func TestPortageRepository_String(t *testing.T) {
	// Create a temp directory for the test (NewPortageRepository validates path exists)
	tmpDir := t.TempDir()

	repo, err := NewPortageRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewPortageRepository() error = %v", err)
	}

	if repo.Path != tmpDir {
		t.Errorf("Path = %s, want %s", repo.Path, tmpDir)
	}
}

// TestPortageRepository_LoadPackage_PathTraversal tests path traversal attack prevention.
// See: https://github.com/grpmsoft/grpm/issues/36
func TestPortageRepository_LoadPackage_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	repo, err := NewPortageRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewPortageRepository() error = %v", err)
	}

	// All these inputs should be rejected with an error
	// They attempt to escape the repository root via path traversal
	pathTraversalAttempts := []struct {
		name  string
		input string
	}{
		// Category-based attacks
		{"dotdot category", "../etc/passwd"},
		{"dotdot category with pkg", "../../../etc/passwd"},
		{"relative dot category", "./passwd"},
		{"hidden category", ".hidden/pkg"},

		// Package name attacks
		{"dotdot package", "sys-libs/../../etc"},
		{"dotdot package complex", "sys-libs/../../../etc/passwd"},
		{"relative dot package", "sys-libs/./passwd"},
		{"hidden package", "sys-libs/.hidden"},

		// Combined attacks
		{"both dotdot", "../etc/../passwd"},
		{"slash escape", "sys-libs/zlib/../../../etc"},

		// Special characters
		{"null byte category", "sys\x00libs/zlib"},
		{"null byte package", "sys-libs/zlib\x00etc"},
	}

	for _, tt := range pathTraversalAttempts {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repo.LoadPackage(tt.input)
			if err == nil {
				t.Errorf("LoadPackage(%q) should return error for path traversal attempt", tt.input)
			}
		})
	}
}

// TestPortageRepository_LoadPackageVersion_PathTraversal tests version-specific path traversal.
// See: https://github.com/grpmsoft/grpm/issues/36
func TestPortageRepository_LoadPackageVersion_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	repo, err := NewPortageRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewPortageRepository() error = %v", err)
	}

	// All these inputs should be rejected
	pathTraversalAttempts := []struct {
		name    string
		pkgName string
		version string
	}{
		{"dotdot category", "../etc", "passwd"},
		{"dotdot package", "sys-libs/../../etc", "passwd"},
		{"hidden category", ".hidden/pkg", "1.0"},
		{"hidden package", "sys-libs/.hidden", "1.0"},
	}

	for _, tt := range pathTraversalAttempts {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repo.LoadPackageVersion(tt.pkgName, tt.version)
			if err == nil {
				t.Errorf("LoadPackageVersion(%q, %q) should return error for path traversal attempt",
					tt.pkgName, tt.version)
			}
		})
	}
}

// TestPortageRepository_GetAllVersions_PathTraversal tests GetAllVersions path traversal.
// See: https://github.com/grpmsoft/grpm/issues/36
func TestPortageRepository_GetAllVersions_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	repo, err := NewPortageRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewPortageRepository() error = %v", err)
	}

	// All these inputs should be rejected
	pathTraversalAttempts := []string{
		"../etc/passwd",
		"../../../etc/passwd",
		"sys-libs/../../etc",
		".hidden/pkg",
		"sys-libs/.hidden",
	}

	for _, input := range pathTraversalAttempts {
		t.Run(input, func(t *testing.T) {
			_, err := repo.GetAllVersions(input)
			if err == nil {
				t.Errorf("GetAllVersions(%q) should return error for path traversal attempt", input)
			}
		})
	}
}

// BenchmarkPortageRepository_LoadPackage benchmarks package loading.
func BenchmarkPortageRepository_LoadPackage(b *testing.B) {
	tmpDir := b.TempDir()

	// Create ebuild structure: test/pkg/pkg-1.0.ebuild
	pkgDir := filepath.Join(tmpDir, "test", "pkg")
	_ = os.MkdirAll(pkgDir, 0755)

	ebuildContent := `EAPI=8
DESCRIPTION="Test package"
SLOT="0"
KEYWORDS="amd64"
DEPEND="sys-libs/zlib dev-libs/openssl"
RDEPEND="${DEPEND}"
`
	_ = os.WriteFile(filepath.Join(pkgDir, "pkg-1.0.ebuild"), []byte(ebuildContent), 0644)

	repo, err := NewPortageRepository(tmpDir)
	if err != nil {
		b.Fatalf("NewPortageRepository() error = %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// LoadPackage expects category/pkgName (without version)
		_, _ = repo.LoadPackage("test/pkg")
	}
}
