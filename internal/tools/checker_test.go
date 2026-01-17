package tools

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewChecker(t *testing.T) {
	c := NewChecker()

	if c == nil {
		t.Fatal("expected non-nil checker")
	}
	if c.detector == nil {
		t.Error("expected detector to be initialized")
	}
	if c.registry == nil {
		t.Error("expected registry to be initialized")
	}
}

func TestCheckerCheckForEclasses(t *testing.T) {
	// Create a test registry with known tools
	r := NewRegistry()
	r.Register(NewTool("cmake", "nonexistent-cmake-binary", "dev-build/cmake", "Build system").
		WithRequiredBy("cmake"))
	r.Register(NewTool("ninja", "nonexistent-ninja-binary", "dev-build/ninja", "Build tool").
		WithRequiredBy("cmake"))

	c := NewCheckerWithRegistry(r)

	result := c.CheckForEclasses([]string{"cmake"})

	if len(result.Required) != 2 {
		t.Errorf("expected 2 required tools, got %d", len(result.Required))
	}

	if len(result.Missing) != 2 {
		t.Errorf("expected 2 missing tools, got %d", len(result.Missing))
	}

	if result.CanBuild {
		t.Error("expected CanBuild to be false when tools are missing")
	}

	if len(result.Eclasses) != 1 || result.Eclasses[0] != "cmake" {
		t.Errorf("expected Eclasses=['cmake'], got %v", result.Eclasses)
	}
}

func TestCheckerCheckForEclassesWithOptional(t *testing.T) {
	// Create a registry with optional tools
	r := NewRegistry()
	r.Register(NewTool("cmake", "nonexistent-cmake", "dev-build/cmake", "Build system").
		WithRequiredBy("cmake"))
	r.Register(NewTool("ccache", "nonexistent-ccache", "dev-util/ccache", "Compiler cache").
		WithRequiredBy("cmake").WithOptional())

	c := NewCheckerWithRegistry(r)

	result := c.CheckForEclasses([]string{"cmake"})

	// Both tools required, but only cmake blocks build
	if len(result.Required) != 2 {
		t.Errorf("expected 2 required tools, got %d", len(result.Required))
	}

	// Only cmake should be in Missing (ccache is optional)
	if len(result.Missing) != 1 {
		t.Errorf("expected 1 missing tool (optional excluded), got %d", len(result.Missing))
	}

	if result.CanBuild {
		t.Error("expected CanBuild to be false when required cmake is missing")
	}
}

func TestCheckerCheckForEbuild(t *testing.T) {
	// Create a temporary ebuild file
	tmpDir := t.TempDir()
	ebuildPath := filepath.Join(tmpDir, "test.ebuild")

	content := `# Test ebuild
EAPI=8
inherit cmake

DESCRIPTION="Test package"
`
	if err := os.WriteFile(ebuildPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test ebuild: %v", err)
	}

	c := NewChecker()

	result, err := c.CheckForEbuild(ebuildPath)
	if err != nil {
		t.Fatalf("CheckForEbuild failed: %v", err)
	}

	if len(result.Eclasses) != 1 {
		t.Errorf("expected 1 eclass, got %d", len(result.Eclasses))
	}

	if result.Eclasses[0] != "cmake" {
		t.Errorf("expected eclass 'cmake', got %q", result.Eclasses[0])
	}
}

func TestCheckerCheckForEbuildNotFound(t *testing.T) {
	c := NewChecker()

	_, err := c.CheckForEbuild("/nonexistent/path/test.ebuild")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestExtractInherit(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "simple inherit",
			content:  "inherit cmake",
			expected: []string{"cmake"},
		},
		{
			name:     "multiple eclasses",
			content:  "inherit cmake python-single-r1 xdg-utils",
			expected: []string{"cmake", "python-single-r1", "xdg-utils"},
		},
		{
			name: "inherit with line continuation",
			content: `inherit cmake \
	ninja \
	python`,
			expected: []string{"cmake", "ninja", "python"},
		},
		{
			name:     "inherit after EAPI",
			content:  "EAPI=8\ninherit meson",
			expected: []string{"meson"},
		},
		{
			name:     "commented inherit ignored",
			content:  "# inherit cmake\ninherit meson",
			expected: []string{"meson"},
		},
		{
			name:     "no inherit",
			content:  "EAPI=8\nDESCRIPTION=\"Test\"",
			expected: nil,
		},
		{
			name: "multiple inherit statements",
			content: `EAPI=8
inherit cmake
inherit python-single-r1`,
			expected: []string{"cmake", "python-single-r1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractInherit(tt.content)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d eclasses, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("expected eclass[%d]=%q, got %q", i, expected, result[i])
				}
			}
		})
	}
}

func TestFormatMissingTools(t *testing.T) {
	missing := []*Tool{
		NewTool("cmake", "cmake", "dev-build/cmake", "Build system"),
		NewTool("ninja", "ninja", "dev-build/ninja", "Build tool"),
	}

	result := FormatMissingTools(missing)

	// Check that it contains expected information
	if !strings.Contains(result, "cmake") {
		t.Error("expected result to contain 'cmake'")
	}
	if !strings.Contains(result, "ninja") {
		t.Error("expected result to contain 'ninja'")
	}
	if !strings.Contains(result, "dev-build/cmake") {
		t.Error("expected result to contain package name")
	}
	if !strings.Contains(result, "grpm install") {
		t.Error("expected result to contain install hint")
	}
}

func TestFormatMissingToolsEmpty(t *testing.T) {
	result := FormatMissingTools(nil)

	if result != "" {
		t.Errorf("expected empty string for nil missing, got %q", result)
	}
}

func TestFormatCheckResult(t *testing.T) {
	result := &CheckResult{
		Eclasses:  []string{"cmake"},
		Required:  []*Tool{NewTool("cmake", "cmake", "dev-build/cmake", "Build")},
		Available: []*Tool{},
		Missing:   []*Tool{NewTool("cmake", "cmake", "dev-build/cmake", "Build")},
		CanBuild:  false,
	}

	output := FormatCheckResult(result)

	if !strings.Contains(output, "cmake") {
		t.Error("expected output to contain 'cmake'")
	}
	if !strings.Contains(output, "BLOCKED") {
		t.Error("expected output to contain 'BLOCKED'")
	}
}

func TestCheckerMustHaveTools(t *testing.T) {
	r := NewRegistry()
	r.Register(NewTool("missing", "nonexistent-binary", "test/missing", "Missing tool"))

	c := NewCheckerWithRegistry(r)

	err := c.MustHaveTools("missing")
	if err == nil {
		t.Fatal("expected error for missing tool")
	}

	// Check error type using errors.As
	var mtErr *MissingToolsError
	if !errors.As(err, &mtErr) {
		t.Fatalf("expected *MissingToolsError, got %T", err)
	}

	if len(mtErr.Missing) != 1 {
		t.Errorf("expected 1 missing tool, got %d", len(mtErr.Missing))
	}

	if mtErr.Missing[0].Name != "missing" {
		t.Errorf("expected missing tool 'missing', got %q", mtErr.Missing[0].Name)
	}
}

func TestMissingToolsError(t *testing.T) {
	err := &MissingToolsError{
		Missing: []*Tool{
			NewTool("cmake", "cmake", "dev-build/cmake", "Build system"),
			NewTool("ninja", "ninja", "dev-build/ninja", "Build tool"),
		},
	}

	// Test error message
	msg := err.Error()
	if !strings.Contains(msg, "cmake") {
		t.Error("expected error message to contain 'cmake'")
	}
	if !strings.Contains(msg, "ninja") {
		t.Error("expected error message to contain 'ninja'")
	}

	// Test GetMissing
	missing := err.GetMissing()
	if len(missing) != 2 {
		t.Errorf("expected 2 missing tools, got %d", len(missing))
	}

	// Test InstallHint
	hint := err.InstallHint()
	if !strings.Contains(hint, "dev-build/cmake") {
		t.Error("expected hint to contain package name")
	}
}

func TestMissingToolsErrorSingle(t *testing.T) {
	err := &MissingToolsError{
		Missing: []*Tool{
			NewTool("cmake", "cmake", "dev-build/cmake", "Build system"),
		},
	}

	msg := err.Error()
	if !strings.Contains(msg, "missing required tool: cmake") {
		t.Errorf("expected singular error message, got %q", msg)
	}
}

func TestCheckerMustHaveToolsUnknown(t *testing.T) {
	r := NewRegistry()
	c := NewCheckerWithRegistry(r)

	err := c.MustHaveTools("completely-unknown-tool")
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}

	// Check error type using errors.As
	var mtErr *MissingToolsError
	if !errors.As(err, &mtErr) {
		t.Fatalf("expected *MissingToolsError, got %T", err)
	}

	// Should create synthetic entry for unknown tool
	if mtErr.Missing[0].Package != "unknown" {
		t.Errorf("expected unknown package, got %q", mtErr.Missing[0].Package)
	}
}

// TestCheckForEclassesOnly_NoGlobalTools verifies that CheckForEclassesOnly
// only checks tools from EclassToolMap, not from the registry's ByEclass cache.
// This prevents packages like sys-libs/glibc from requiring Rust, Java, Ruby, etc.
func TestCheckForEclassesOnly_NoGlobalTools(t *testing.T) {
	// Create a registry with tools registered for "toolchain-funcs" eclass
	// which is inherited by MANY packages (including glibc)
	r := NewRegistry()

	// These tools are registered for toolchain-funcs in the default registry
	r.Register(NewTool("gcc", "gcc", "sys-devel/gcc", "C compiler").
		WithRequiredBy("toolchain-funcs"))
	r.Register(NewTool("g++", "g++", "sys-devel/gcc", "C++ compiler").
		WithRequiredBy("toolchain-funcs"))

	// These tools should NOT be required for packages that only inherit toolchain-funcs
	r.Register(NewTool("rustc", "nonexistent-rustc", "dev-lang/rust", "Rust compiler").
		WithRequiredBy("cargo"))
	r.Register(NewTool("cargo", "nonexistent-cargo", "dev-lang/rust", "Rust package manager").
		WithRequiredBy("cargo"))
	r.Register(NewTool("java", "nonexistent-java", "virtual/jre", "Java runtime").
		WithRequiredBy("java-pkg-2"))
	r.Register(NewTool("ruby", "nonexistent-ruby", "dev-lang/ruby", "Ruby interpreter").
		WithRequiredBy("ruby-ng"))

	c := NewCheckerWithRegistry(r)

	// Simulate glibc-like package that only inherits toolchain-funcs, multilib, etc.
	// These eclasses don't have entries in EclassToolMap
	glibcEclasses := []string{"toolchain-funcs", "multilib", "flag-o-matic", "linux-info"}

	result := c.CheckForEclassesOnly(glibcEclasses)

	// Should NOT require any tools because:
	// - toolchain-funcs is NOT in EclassToolMap (removed as it's always available)
	// - multilib, flag-o-matic, linux-info are NOT in EclassToolMap
	if len(result.Required) != 0 {
		var names []string
		for _, tool := range result.Required {
			names = append(names, tool.Name)
		}
		t.Errorf("expected 0 required tools for glibc-like package, got %d: %v",
			len(result.Required), names)
	}

	// Should NOT require Rust, Java, Ruby
	for _, tool := range result.Required {
		switch tool.Name {
		case "rustc", "cargo", "java", "ruby":
			t.Errorf("glibc-like package should not require %s", tool.Name)
		}
	}

	// CanBuild should be true (no missing tools)
	if !result.CanBuild {
		t.Error("expected CanBuild to be true for glibc-like package")
	}
}

// TestCheckForEclassesOnly_OnlyMappedTools verifies that only tools from
// EclassToolMap are checked, not the full registry.
func TestCheckForEclassesOnly_OnlyMappedTools(t *testing.T) {
	// Create registry with cmake tools
	r := NewRegistry()
	r.Register(NewTool("cmake", "nonexistent-cmake", "dev-build/cmake", "Build system"))
	r.Register(NewTool("ninja", "nonexistent-ninja", "dev-build/ninja", "Build tool"))
	r.Register(NewTool("make", "make", "sys-devel/make", "GNU Make")) // make is usually available

	c := NewCheckerWithRegistry(r)

	// Check for cmake eclass
	result := c.CheckForEclassesOnly([]string{"cmake"})

	// cmake eclass requires cmake, ninja, make (from EclassToolMap)
	if len(result.Required) != 3 {
		var names []string
		for _, tool := range result.Required {
			names = append(names, tool.Name)
		}
		t.Errorf("expected 3 required tools for cmake, got %d: %v",
			len(result.Required), names)
	}

	// Verify the correct tools are required
	toolNames := make(map[string]bool)
	for _, tool := range result.Required {
		toolNames[tool.Name] = true
	}

	for _, expected := range []string{"cmake", "ninja", "make"} {
		if !toolNames[expected] {
			t.Errorf("expected %s to be required for cmake eclass", expected)
		}
	}
}

// TestCheckForEclassesOnly_UnknownEclass verifies that unknown eclasses
// (not in EclassToolMap) don't require any tools.
func TestCheckForEclassesOnly_UnknownEclass(t *testing.T) {
	c := NewChecker()

	// Test with eclasses that are NOT in EclassToolMap
	result := c.CheckForEclassesOnly([]string{"unknown-eclass", "another-unknown"})

	// Should have no required tools
	if len(result.Required) != 0 {
		t.Errorf("expected 0 required tools for unknown eclasses, got %d", len(result.Required))
	}

	// CanBuild should be true
	if !result.CanBuild {
		t.Error("expected CanBuild to be true for unknown eclasses")
	}
}

// TestCheckForEclassesOnly_CargoRequiresRust verifies that packages inheriting
// cargo eclass DO require rustc and cargo.
func TestCheckForEclassesOnly_CargoRequiresRust(t *testing.T) {
	// Create registry with Rust tools (non-existent binaries for testing)
	r := NewRegistry()
	r.Register(NewTool("rustc", "nonexistent-rustc", "dev-lang/rust", "Rust compiler"))
	r.Register(NewTool("cargo", "nonexistent-cargo", "dev-lang/rust", "Rust package manager"))

	c := NewCheckerWithRegistry(r)

	// A Rust package inherits cargo eclass
	result := c.CheckForEclassesOnly([]string{"cargo"})

	// Should require rustc and cargo
	if len(result.Required) != 2 {
		var names []string
		for _, tool := range result.Required {
			names = append(names, tool.Name)
		}
		t.Errorf("expected 2 required tools for cargo eclass, got %d: %v",
			len(result.Required), names)
	}

	// Both tools should be missing (nonexistent binaries)
	if len(result.Missing) != 2 {
		t.Errorf("expected 2 missing tools, got %d", len(result.Missing))
	}

	// CanBuild should be false
	if result.CanBuild {
		t.Error("expected CanBuild to be false when rustc/cargo are missing")
	}
}

// TestCheckForEclassesOnly_EclassToolMapCoverage verifies that common eclasses
// have proper tool mappings.
func TestCheckForEclassesOnly_EclassToolMapCoverage(t *testing.T) {
	eclassMap := EclassToolMap()

	// Verify critical eclasses have mappings
	criticalEclasses := []struct {
		eclass   string
		expected []string
	}{
		{"cmake", []string{"cmake", "ninja", "make"}},
		{"meson", []string{"meson", "ninja"}},
		{"cargo", []string{"cargo", "rustc"}},
		{"go-module", []string{"go"}},
		{"java-pkg-2", []string{"java", "javac"}},
		{"ruby-ng", []string{"ruby"}},
		{"git-r3", []string{"git"}},
		{"mercurial", []string{"hg"}},
		{"subversion", []string{"svn"}},
	}

	for _, tc := range criticalEclasses {
		tools, ok := eclassMap[tc.eclass]
		if !ok {
			t.Errorf("expected EclassToolMap to have mapping for %s", tc.eclass)
			continue
		}

		// Check each expected tool is present
		toolSet := make(map[string]bool)
		for _, tool := range tools {
			toolSet[tool] = true
		}

		for _, expected := range tc.expected {
			if !toolSet[expected] {
				t.Errorf("expected %s eclass to require %s, tools: %v",
					tc.eclass, expected, tools)
			}
		}
	}
}

// TestCheckForEclassesOnly_NoToolchainFuncs verifies that toolchain-funcs
// is NOT in EclassToolMap (as it uses system compiler which is always available).
func TestCheckForEclassesOnly_NoToolchainFuncs(t *testing.T) {
	eclassMap := EclassToolMap()

	// toolchain-funcs should NOT be in the map
	if _, ok := eclassMap["toolchain-funcs"]; ok {
		t.Error("toolchain-funcs should NOT be in EclassToolMap - " +
			"it uses system compiler which is always available")
	}

	// Verify other "no-tool" eclasses are also not in the map
	noToolEclasses := []string{
		"multilib",
		"linux-info",
		"pam",
		"bash-completion-r1",
		"flag-o-matic",
		"xdg",
		"optfeature",
		"edo",
		"wrapper",
		"multiprocessing",
	}

	for _, eclass := range noToolEclasses {
		if _, ok := eclassMap[eclass]; ok {
			t.Errorf("%s should NOT be in EclassToolMap - it doesn't require special tools", eclass)
		}
	}
}
