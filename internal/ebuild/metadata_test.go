package ebuild

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// TestMetadataEvaluator_SimpleEbuild tests SRC_URI evaluation for a simple ebuild
// without eclass inheritance.
func TestMetadataEvaluator_SimpleEbuild(t *testing.T) {
	// Create temporary directory for test repository
	tmpDir := t.TempDir()

	// Create eclass directory (empty, no eclasses needed)
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatalf("Failed to create eclass dir: %v", err)
	}

	// Create package directory
	pkgDir := filepath.Join(tmpDir, "app-misc", "hello")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir: %v", err)
	}

	// Create a simple ebuild with explicit SRC_URI
	ebuildContent := `# Copyright 1999-2025 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=8

DESCRIPTION="Hello World program"
HOMEPAGE="https://www.gnu.org/software/hello/"
SRC_URI="mirror://gnu/${PN}/${P}.tar.gz"

LICENSE="GPL-3+"
SLOT="0"
KEYWORDS="~amd64 ~x86"
`
	ebuildPath := filepath.Join(pkgDir, "hello-2.10.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatalf("Failed to write ebuild: %v", err)
	}

	// Create evaluator
	evaluator, err := NewMetadataEvaluator(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create evaluator: %v", err)
	}

	// Create package info
	pkgInfo := &pkg.Package{
		Name:    "app-misc/hello",
		Version: "2.10",
		Slot:    pkg.NewSlot("0", ""),
	}

	// Evaluate SRC_URI
	ctx := context.Background()
	srcURI, err := evaluator.EvaluateSrcURI(ctx, ebuildPath, pkgInfo)
	if err != nil {
		t.Fatalf("EvaluateSrcURI failed: %v", err)
	}

	// Check that SRC_URI contains expected content
	// The variables should be expanded: ${PN} -> "hello", ${P} -> "hello-2.10"
	if !strings.Contains(srcURI, "hello") {
		t.Errorf("SRC_URI should contain 'hello', got: %s", srcURI)
	}

	// Either dynamic evaluation or fallback should return something with mirror://gnu
	if !strings.Contains(srcURI, "gnu") && !strings.Contains(srcURI, "mirror") {
		t.Errorf("SRC_URI should contain 'gnu' or 'mirror', got: %s", srcURI)
	}
}

// TestMetadataEvaluator_VariableExpansion tests that package variables are expanded.
func TestMetadataEvaluator_VariableExpansion(t *testing.T) {
	tmpDir := t.TempDir()

	// Create eclass directory
	if err := os.MkdirAll(filepath.Join(tmpDir, "eclass"), 0755); err != nil {
		t.Fatalf("Failed to create eclass dir: %v", err)
	}

	// Create package directory
	pkgDir := filepath.Join(tmpDir, "dev-libs", "testpkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir: %v", err)
	}

	// Create ebuild with various variable references
	ebuildContent := `EAPI=8
SRC_URI="https://example.com/${PN}-${PV}.tar.gz"
`
	ebuildPath := filepath.Join(pkgDir, "testpkg-1.2.3.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatalf("Failed to write ebuild: %v", err)
	}

	evaluator, err := NewMetadataEvaluator(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create evaluator: %v", err)
	}

	pkgInfo := &pkg.Package{
		Name:    "dev-libs/testpkg",
		Version: "1.2.3",
		Slot:    pkg.NewSlot("0", ""),
	}

	ctx := context.Background()
	srcURI, err := evaluator.EvaluateSrcURI(ctx, ebuildPath, pkgInfo)
	if err != nil {
		t.Fatalf("EvaluateSrcURI failed: %v", err)
	}

	// The evaluated SRC_URI should have variables expanded
	// Either the interpreter or fallback should expand ${PN} and ${PV}
	if !strings.Contains(srcURI, "testpkg") || !strings.Contains(srcURI, "1.2.3") {
		t.Errorf("Expected SRC_URI to contain 'testpkg' and '1.2.3', got: %s", srcURI)
	}
}

// TestMetadataEvaluator_EmptySrcURI tests handling of ebuilds without SRC_URI.
func TestMetadataEvaluator_EmptySrcURI(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmpDir, "eclass"), 0755); err != nil {
		t.Fatalf("Failed to create eclass dir: %v", err)
	}

	pkgDir := filepath.Join(tmpDir, "virtual", "test")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir: %v", err)
	}

	// Virtual packages often have no SRC_URI
	ebuildContent := `EAPI=8
DESCRIPTION="Virtual package for testing"
RDEPEND="dev-libs/foo"
`
	ebuildPath := filepath.Join(pkgDir, "test-1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatalf("Failed to write ebuild: %v", err)
	}

	evaluator, err := NewMetadataEvaluator(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create evaluator: %v", err)
	}

	pkgInfo := &pkg.Package{
		Name:    "virtual/test",
		Version: "1.0",
		Slot:    pkg.NewSlot("0", ""),
	}

	ctx := context.Background()
	srcURI, err := evaluator.EvaluateSrcURI(ctx, ebuildPath, pkgInfo)
	if err != nil {
		t.Fatalf("EvaluateSrcURI failed: %v", err)
	}

	// Empty SRC_URI should return empty string
	if srcURI != "" {
		t.Errorf("Expected empty SRC_URI for virtual package, got: %s", srcURI)
	}
}

// TestMetadataEvaluator_InvalidPath tests error handling for invalid paths.
func TestMetadataEvaluator_InvalidPath(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmpDir, "eclass"), 0755); err != nil {
		t.Fatalf("Failed to create eclass dir: %v", err)
	}

	evaluator, err := NewMetadataEvaluator(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create evaluator: %v", err)
	}

	pkgInfo := &pkg.Package{
		Name:    "app-misc/nonexistent",
		Version: "1.0",
		Slot:    pkg.NewSlot("0", ""),
	}

	ctx := context.Background()
	_, err = evaluator.EvaluateSrcURI(ctx, "/nonexistent/path.ebuild", pkgInfo)
	if err == nil {
		t.Error("Expected error for nonexistent ebuild path")
	}
}

// TestMetadataEvaluator_EmptyPath tests error handling for empty path.
func TestMetadataEvaluator_EmptyPath(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmpDir, "eclass"), 0755); err != nil {
		t.Fatalf("Failed to create eclass dir: %v", err)
	}

	evaluator, err := NewMetadataEvaluator(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create evaluator: %v", err)
	}

	pkgInfo := &pkg.Package{
		Name:    "app-misc/test",
		Version: "1.0",
		Slot:    pkg.NewSlot("0", ""),
	}

	ctx := context.Background()
	_, err = evaluator.EvaluateSrcURI(ctx, "", pkgInfo)
	if err == nil {
		t.Error("Expected error for empty ebuild path")
	}
}

// TestMetadataEvaluator_NilPackageInfo tests error handling for nil package info.
func TestMetadataEvaluator_NilPackageInfo(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmpDir, "eclass"), 0755); err != nil {
		t.Fatalf("Failed to create eclass dir: %v", err)
	}

	// Create a dummy ebuild
	pkgDir := filepath.Join(tmpDir, "app-misc", "test")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir: %v", err)
	}

	ebuildPath := filepath.Join(pkgDir, "test-1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte("EAPI=8\n"), 0644); err != nil {
		t.Fatalf("Failed to write ebuild: %v", err)
	}

	evaluator, err := NewMetadataEvaluator(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create evaluator: %v", err)
	}

	ctx := context.Background()
	_, err = evaluator.EvaluateSrcURI(ctx, ebuildPath, nil)
	if err == nil {
		t.Error("Expected error for nil package info")
	}
}

// TestExtractEbuildMetadata tests extraction of multiple metadata variables.
func TestExtractEbuildMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmpDir, "eclass"), 0755); err != nil {
		t.Fatalf("Failed to create eclass dir: %v", err)
	}

	pkgDir := filepath.Join(tmpDir, "dev-libs", "testpkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir: %v", err)
	}

	ebuildContent := `EAPI=8
DESCRIPTION="Test package"
SRC_URI="https://example.com/${P}.tar.gz"
IUSE="debug doc"
LICENSE="MIT"
`
	ebuildPath := filepath.Join(pkgDir, "testpkg-1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatalf("Failed to write ebuild: %v", err)
	}

	evaluator, err := NewMetadataEvaluator(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create evaluator: %v", err)
	}

	pkgInfo := &pkg.Package{
		Name:    "dev-libs/testpkg",
		Version: "1.0",
		Slot:    pkg.NewSlot("0", ""),
	}

	ctx := context.Background()
	metadata, err := evaluator.ExtractEbuildMetadata(ctx, ebuildPath, pkgInfo, []string{"SRC_URI", "IUSE", "LICENSE"})
	if err != nil {
		t.Fatalf("ExtractEbuildMetadata failed: %v", err)
	}

	// Check that we got some metadata back (exact values depend on interpreter behavior)
	if len(metadata) == 0 {
		t.Error("Expected metadata to be extracted, got empty map")
	}
}

// TestBuildUSEString tests the USE flag string builder.
func TestBuildUSEString(t *testing.T) {
	tests := []struct {
		name  string
		flags map[string]bool
		want  []string // All expected flags (order doesn't matter)
	}{
		{
			name:  "empty",
			flags: nil,
			want:  nil,
		},
		{
			name:  "single flag enabled",
			flags: map[string]bool{"ssl": true},
			want:  []string{"ssl"},
		},
		{
			name:  "mixed flags",
			flags: map[string]bool{"ssl": true, "debug": false, "doc": true},
			want:  []string{"ssl", "doc"}, // Only enabled flags
		},
		{
			name:  "all disabled",
			flags: map[string]bool{"ssl": false, "debug": false},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildUSEString(tt.flags)

			if tt.want == nil && got != "" {
				t.Errorf("buildUSEString() = %q, want empty", got)
				return
			}

			if tt.want != nil {
				// Check each expected flag is present
				for _, flag := range tt.want {
					if !strings.Contains(got, flag) {
						t.Errorf("buildUSEString() = %q, missing flag %q", got, flag)
					}
				}
			}
		})
	}
}

// TestBuildMetadataExtractionScript tests script generation.
func TestBuildMetadataExtractionScript(t *testing.T) {
	ebuildContent := `EAPI=8
SRC_URI="https://example.com/test.tar.gz"
`
	script := buildMetadataExtractionScript(ebuildContent)

	// Check that script contains shebang
	if !strings.HasPrefix(script, "#!/bin/bash") {
		t.Error("Script should start with #!/bin/bash")
	}

	// Check that script contains the ebuild content
	if !strings.Contains(script, "SRC_URI=") {
		t.Error("Script should contain SRC_URI definition")
	}

	// Check that script echoes SRC_URI at the end
	if !strings.Contains(script, "echo \"$SRC_URI\"") {
		t.Error("Script should echo SRC_URI")
	}

	// Check that stub functions are defined
	stubFuncs := []string{"pkg_setup", "src_compile", "src_install"}
	for _, fn := range stubFuncs {
		if !strings.Contains(script, fn+"()") {
			t.Errorf("Script should define stub function %s", fn)
		}
	}
}
