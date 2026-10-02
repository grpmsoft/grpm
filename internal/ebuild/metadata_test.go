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

// TestMetadataEvaluator_EvalMode tests the evaluation mode configuration.
func TestMetadataEvaluator_EvalMode(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmpDir, "eclass"), 0755); err != nil {
		t.Fatalf("Failed to create eclass dir: %v", err)
	}

	evaluator, err := NewMetadataEvaluator(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create evaluator: %v", err)
	}

	// Default mode should be EvalModeGo
	if evaluator.Mode != EvalModeGo {
		t.Errorf("Default mode should be EvalModeGo, got %v", evaluator.Mode)
	}

	// Can change mode
	evaluator.Mode = EvalModeNativeBash
	if evaluator.Mode != EvalModeNativeBash {
		t.Errorf("Expected EvalModeNativeBash after setting, got %v", evaluator.Mode)
	}
}

// TestCreateMetadataEnvironment_ExtraVars tests that ExtraVars are properly set.
func TestCreateMetadataEnvironment_ExtraVars(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmpDir, "eclass"), 0755); err != nil {
		t.Fatalf("Failed to create eclass dir: %v", err)
	}

	evaluator, err := NewMetadataEvaluator(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create evaluator: %v", err)
	}

	pkgInfo := &pkg.Package{
		Name:    "sys-devel/gcc",
		Version: "13.4.1_p20250807",
		Slot:    pkg.NewSlot("13", ""),
	}

	env, err := evaluator.createMetadataEnvironment(pkgInfo, "/path/to/ebuild.ebuild")
	if err != nil {
		t.Fatalf("createMetadataEnvironment failed: %v", err)
	}

	// Check that ExtraVars contains the expected keys
	if env.ExtraVars == nil {
		t.Fatal("ExtraVars should not be nil")
	}

	// TOOLCHAIN_HAS_TESTS should be empty to disable python eclass inheritance
	if val, ok := env.ExtraVars["TOOLCHAIN_HAS_TESTS"]; !ok || val != "" {
		t.Errorf("TOOLCHAIN_HAS_TESTS should be empty string, got %q (exists: %v)", val, ok)
	}

	// TOOLCHAIN_USE_GIT_PATCHES should be empty to disable git-r3 inheritance
	if val, ok := env.ExtraVars["TOOLCHAIN_USE_GIT_PATCHES"]; !ok || val != "" {
		t.Errorf("TOOLCHAIN_USE_GIT_PATCHES should be empty string, got %q (exists: %v)", val, ok)
	}

	// CHOST should be set for tc-arch functions
	if val, ok := env.ExtraVars["CHOST"]; !ok || val == "" {
		t.Errorf("CHOST should be set, got %q (exists: %v)", val, ok)
	}
}

// TestEnvironment_ToMap_IncludesExtraVars tests that ToMap includes ExtraVars.
func TestEnvironment_ToMap_IncludesExtraVars(t *testing.T) {
	env := &Environment{
		P:        "test-1.0",
		PN:       "test",
		PV:       "1.0",
		PR:       "r0",
		PVR:      "1.0",
		PF:       "test-1.0",
		CATEGORY: "dev-libs",
		EAPI:     "8",
		SLOT:     "0",
		ExtraVars: map[string]string{
			"MY_VAR":              "my_value",
			"TOOLCHAIN_HAS_TESTS": "",
		},
	}

	envMap := env.ToMap()

	// Check that ExtraVars are in the map
	if val, ok := envMap["MY_VAR"]; !ok || val != "my_value" {
		t.Errorf("Expected MY_VAR=my_value in map, got %q (exists: %v)", val, ok)
	}

	// Empty string value should still be included
	if _, ok := envMap["TOOLCHAIN_HAS_TESTS"]; !ok {
		t.Error("TOOLCHAIN_HAS_TESTS should be in map even with empty value")
	}
}

// TestMetadataEvaluator_SimpleEclass tests evaluation with a simple eclass.
func TestMetadataEvaluator_SimpleEclass(t *testing.T) {
	tmpDir := t.TempDir()

	// Create eclass directory with a simple eclass
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatalf("Failed to create eclass dir: %v", err)
	}

	// Create a simple eclass that sets a variable
	simpleEclass := `# Copyright 1999-2025 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

# @ECLASS: simple.eclass
# @DESCRIPTION: Simple test eclass

_SIMPLE_URI="https://example.com/simple"

get_simple_uri() {
	echo "${_SIMPLE_URI}/${P}.tar.gz"
}
`
	if err := os.WriteFile(filepath.Join(eclassDir, "simple.eclass"), []byte(simpleEclass), 0644); err != nil {
		t.Fatalf("Failed to write eclass: %v", err)
	}

	// Create package directory
	pkgDir := filepath.Join(tmpDir, "dev-libs", "testpkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir: %v", err)
	}

	// Create ebuild that inherits the simple eclass
	ebuildContent := `# Copyright 1999-2025 Gentoo Authors
EAPI=8

inherit simple

DESCRIPTION="Test package"
SRC_URI="$(get_simple_uri)"
`
	ebuildPath := filepath.Join(pkgDir, "testpkg-1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatalf("Failed to write ebuild: %v", err)
	}

	// Create evaluator
	evaluator, err := NewMetadataEvaluator(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create evaluator: %v", err)
	}

	pkgInfo := &pkg.Package{
		Name:    "dev-libs/testpkg",
		Version: "1.0",
		Slot:    pkg.NewSlot("0", ""),
	}

	// Evaluate SRC_URI
	ctx := context.Background()
	srcURI, err := evaluator.EvaluateSrcURI(ctx, ebuildPath, pkgInfo)
	if err != nil {
		t.Fatalf("EvaluateSrcURI failed: %v", err)
	}

	// The SRC_URI should contain the function output
	if !strings.Contains(srcURI, "example.com") || !strings.Contains(srcURI, "testpkg") {
		t.Errorf("SRC_URI should contain 'example.com' and 'testpkg', got: %s", srcURI)
	}
}

// TestBuildNativeBashScript tests native bash script generation.
func TestBuildNativeBashScript(t *testing.T) {
	ebuildContent := `EAPI=8
SRC_URI="https://example.com/${P}.tar.gz"
`
	script := buildNativeBashScript(ebuildContent)

	// Check shebang
	if !strings.HasPrefix(script, "#!/bin/bash") {
		t.Error("Script should start with #!/bin/bash")
	}

	// Check that inherit function is defined
	if !strings.Contains(script, "inherit()") {
		t.Error("Script should define inherit function")
	}

	// Check that EXPORT_FUNCTIONS is defined
	if !strings.Contains(script, "EXPORT_FUNCTIONS()") {
		t.Error("Script should define EXPORT_FUNCTIONS")
	}

	// Check that ebuild content is included
	if !strings.Contains(script, "SRC_URI=") {
		t.Error("Script should contain SRC_URI definition")
	}

	// Check that output line exists
	if !strings.Contains(script, `echo "${SRC_URI}"`) {
		t.Error("Script should echo SRC_URI at the end")
	}
}
