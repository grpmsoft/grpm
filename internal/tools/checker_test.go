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
