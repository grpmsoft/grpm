package analyze

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewResult verifies Result initialization.
func TestNewResult(t *testing.T) {
	result := NewResult("/test/repo")

	if result.RepoPath != "/test/repo" {
		t.Errorf("RepoPath = %q, want %q", result.RepoPath, "/test/repo")
	}
	if result.TotalPackages != 0 {
		t.Errorf("TotalPackages = %d, want 0", result.TotalPackages)
	}
	if result.ByCategory == nil {
		t.Error("ByCategory is nil")
	}
	if result.ByEclass == nil {
		t.Error("ByEclass is nil")
	}
	if result.ByBlocker == nil {
		t.Error("ByBlocker is nil")
	}
}

// TestPackageResult_AddBlocker verifies blocker handling.
func TestPackageResult_AddBlocker(t *testing.T) {
	pr := &PackageResult{
		Atom:      "app-misc/hello-2.10",
		Supported: true,
	}

	// Initially supported
	if !pr.Supported {
		t.Error("Package should be supported initially")
	}

	// Add blocker
	pr.AddBlocker(BlockerMissingEclass, "cmake", "eclass not found")

	// Should now be unsupported
	if pr.Supported {
		t.Error("Package should be unsupported after adding blocker")
	}
	if len(pr.Blockers) != 1 {
		t.Errorf("Expected 1 blocker, got %d", len(pr.Blockers))
	}
	if pr.Blockers[0].Type != BlockerMissingEclass {
		t.Errorf("Blocker type = %q, want %q", pr.Blockers[0].Type, BlockerMissingEclass)
	}
}

// TestResult_AddPackageResult verifies result aggregation.
func TestResult_AddPackageResult(t *testing.T) {
	result := NewResult("/test/repo")

	// Add supported package
	pr1 := &PackageResult{
		Atom:      "app-misc/hello-2.10",
		Category:  "app-misc",
		Name:      "hello",
		Version:   "2.10",
		EAPI:      "8",
		Supported: true,
		Inherits:  []string{"cmake", "multilib"},
	}
	result.AddPackageResult(pr1)

	if result.TotalPackages != 1 {
		t.Errorf("TotalPackages = %d, want 1", result.TotalPackages)
	}
	if result.SupportedPackages != 1 {
		t.Errorf("SupportedPackages = %d, want 1", result.SupportedPackages)
	}

	// Add unsupported package
	pr2 := &PackageResult{
		Atom:      "sys-libs/zlib-1.2.13",
		Category:  "sys-libs",
		Name:      "zlib",
		Version:   "1.2.13",
		EAPI:      "8",
		Supported: false,
		Blockers: []Blocker{
			{Type: BlockerMissingEclass, Name: "java-utils-2"},
		},
	}
	result.AddPackageResult(pr2)

	if result.TotalPackages != 2 {
		t.Errorf("TotalPackages = %d, want 2", result.TotalPackages)
	}
	if result.SupportedPackages != 1 {
		t.Errorf("SupportedPackages = %d, want 1", result.SupportedPackages)
	}
	if result.UnsupportedPackages != 1 {
		t.Errorf("UnsupportedPackages = %d, want 1", result.UnsupportedPackages)
	}

	// Check EAPI tracking
	if result.ByEAPI["8"] != 2 {
		t.Errorf("EAPI 8 count = %d, want 2", result.ByEAPI["8"])
	}

	// Check category tracking
	if cat, ok := result.ByCategory["app-misc"]; ok {
		if cat.TotalPackages != 1 {
			t.Errorf("app-misc total = %d, want 1", cat.TotalPackages)
		}
	} else {
		t.Error("app-misc category not found")
	}
}

// TestResult_Finalize verifies coverage calculation.
func TestResult_Finalize(t *testing.T) {
	result := NewResult("/test/repo")

	// Add 3 supported, 1 unsupported = 75%
	for i := 0; i < 3; i++ {
		result.AddPackageResult(&PackageResult{
			Atom:      "cat/pkg-1.0",
			Category:  "cat",
			Supported: true,
		})
	}
	result.AddPackageResult(&PackageResult{
		Atom:      "cat/pkg2-1.0",
		Category:  "cat",
		Supported: false,
	})

	result.Finalize()

	if result.Coverage != 75.0 {
		t.Errorf("Coverage = %.1f, want 75.0", result.Coverage)
	}
}

// TestResult_TopBlockers verifies blocker sorting.
func TestResult_TopBlockers(t *testing.T) {
	result := NewResult("/test/repo")

	// Add packages with different blockers
	for i := 0; i < 10; i++ {
		pr := &PackageResult{
			Atom:      "cat/pkg-1.0",
			Category:  "cat",
			Supported: false,
			Blockers:  []Blocker{{Type: BlockerMissingEclass, Name: "common"}},
		}
		result.AddPackageResult(pr)
	}
	for i := 0; i < 5; i++ {
		pr := &PackageResult{
			Atom:      "cat/pkg2-1.0",
			Category:  "cat",
			Supported: false,
			Blockers:  []Blocker{{Type: BlockerMissingHelper, Name: "rare"}},
		}
		result.AddPackageResult(pr)
	}

	top := result.TopBlockers(5)

	if len(top) != 2 {
		t.Fatalf("Expected 2 blockers, got %d", len(top))
	}
	// Most common should be first
	if top[0].Count != 10 {
		t.Errorf("Top blocker count = %d, want 10", top[0].Count)
	}
}

// TestBlocker_String verifies blocker string formatting.
func TestBlocker_String(t *testing.T) {
	tests := []struct {
		name     string
		blocker  Blocker
		expected string
	}{
		{
			name: "with details",
			blocker: Blocker{
				Type:    BlockerMissingEclass,
				Name:    "cmake",
				Details: "not found",
			},
			expected: "missing_eclass:cmake (not found)",
		},
		{
			name: "without details",
			blocker: Blocker{
				Type: BlockerUnsupportedEAPI,
				Name: "9",
			},
			expected: "unsupported_eapi:9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.blocker.String()
			if got != tt.expected {
				t.Errorf("Blocker.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestAnalyzer_extractEAPI verifies EAPI extraction.
func TestAnalyzer_extractEAPI(t *testing.T) {
	a := &Analyzer{}

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "EAPI 8",
			content:  "EAPI=8\n\nDESCRIPTION=\"Test\"",
			expected: "8",
		},
		{
			name:     "EAPI with quotes",
			content:  "EAPI=\"7\"\n\nDESCRIPTION=\"Test\"",
			expected: "7",
		},
		{
			name:     "EAPI after comment",
			content:  "# Copyright\n# License\nEAPI=6",
			expected: "6",
		},
		{
			name:     "No EAPI",
			content:  "DESCRIPTION=\"Test\"",
			expected: "",
		},
		{
			name:     "EAPI not first",
			content:  "DESCRIPTION=\"Test\"\nEAPI=5",
			expected: "", // EAPI must be first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.extractEAPI(tt.content)
			if got != tt.expected {
				t.Errorf("extractEAPI() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestAnalyzer_extractInherit verifies inherit extraction.
func TestAnalyzer_extractInherit(t *testing.T) {
	a := &Analyzer{}

	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "single inherit",
			content:  "EAPI=8\n\ninherit cmake",
			expected: []string{"cmake"},
		},
		{
			name:     "multiple eclasses",
			content:  "inherit cmake multilib python-r1",
			expected: []string{"cmake", "multilib", "python-r1"},
		},
		{
			name:     "no inherit",
			content:  "EAPI=8\nDESCRIPTION=\"Test\"",
			expected: nil,
		},
		{
			name:     "inherit in comment",
			content:  "# inherit cmake\nDESCRIPTION=\"Test\"",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.extractInherit(tt.content)
			if len(got) != len(tt.expected) {
				t.Errorf("extractInherit() = %v, want %v", got, tt.expected)
				return
			}
			for i, ec := range got {
				if ec != tt.expected[i] {
					t.Errorf("extractInherit()[%d] = %q, want %q", i, ec, tt.expected[i])
				}
			}
		})
	}
}

// TestAnalyzer_HasHelper verifies helper lookup.
func TestAnalyzer_HasHelper(t *testing.T) {
	a := &Analyzer{
		helpers: buildHelperMap(),
	}

	// Should have common helpers
	helpers := []string{"die", "use", "emake", "econf", "dosym", "cmake"}
	for _, h := range helpers {
		if !a.HasHelper(h) {
			t.Errorf("Expected helper %q to be available", h)
		}
	}

	// Should not have non-existent helpers
	if a.HasHelper("nonexistent_helper") {
		t.Error("nonexistent_helper should not be available")
	}
}

// TestReporter_Text verifies text output.
func TestReporter_Text(t *testing.T) {
	result := NewResult("/test/repo")
	result.AddPackageResult(&PackageResult{
		Atom:      "app-misc/hello-2.10",
		Category:  "app-misc",
		Supported: true,
		EAPI:      "8",
	})
	result.Finalize()

	var buf bytes.Buffer
	reporter := NewReporter(FormatText, false)
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	output := buf.String()

	// Check for expected sections
	expectedStrings := []string{
		"GRPM Coverage Analysis",
		"Repository: /test/repo",
		"Total packages: 1",
		"100.0%",
	}
	for _, s := range expectedStrings {
		if !strings.Contains(output, s) {
			t.Errorf("Output missing %q", s)
		}
	}
}

// TestReporter_JSON verifies JSON output.
func TestReporter_JSON(t *testing.T) {
	result := NewResult("/test/repo")
	result.AddPackageResult(&PackageResult{
		Atom:      "app-misc/hello-2.10",
		Category:  "app-misc",
		Supported: true,
		EAPI:      "8",
	})
	result.Finalize()

	var buf bytes.Buffer
	reporter := NewReporter(FormatJSON, false)
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	// Verify valid JSON
	var jsonResult JSONResult
	if err := json.Unmarshal(buf.Bytes(), &jsonResult); err != nil {
		t.Fatalf("Invalid JSON output: %v", err)
	}

	if jsonResult.TotalPackages != 1 {
		t.Errorf("JSON TotalPackages = %d, want 1", jsonResult.TotalPackages)
	}
	if jsonResult.Coverage != 100.0 {
		t.Errorf("JSON Coverage = %.1f, want 100.0", jsonResult.Coverage)
	}
}

// TestReporter_Markdown verifies markdown output.
func TestReporter_Markdown(t *testing.T) {
	result := NewResult("/test/repo")
	result.AddPackageResult(&PackageResult{
		Atom:      "app-misc/hello-2.10",
		Category:  "app-misc",
		Supported: true,
		EAPI:      "8",
	})
	result.Finalize()

	var buf bytes.Buffer
	reporter := NewReporter(FormatMarkdown, false)
	if err := reporter.Report(&buf, result); err != nil {
		t.Fatalf("Report failed: %v", err)
	}

	output := buf.String()

	// Check for markdown elements
	expectedStrings := []string{
		"# GRPM Coverage Analysis Report",
		"## Summary",
		"|--------|-------|",
		"*Generated by GRPM Coverage Analyzer*",
	}
	for _, s := range expectedStrings {
		if !strings.Contains(output, s) {
			t.Errorf("Output missing %q", s)
		}
	}
}

// TestParseOutputFormat verifies format parsing.
func TestParseOutputFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected OutputFormat
		hasError bool
	}{
		{"text", FormatText, false},
		{"txt", FormatText, false},
		{"", FormatText, false},
		{"json", FormatJSON, false},
		{"JSON", FormatJSON, false},
		{"markdown", FormatMarkdown, false},
		{"md", FormatMarkdown, false},
		{"xml", FormatText, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseOutputFormat(tt.input)
			if (err != nil) != tt.hasError {
				t.Errorf("ParseOutputFormat(%q) error = %v, wantErr %v", tt.input, err, tt.hasError)
			}
			if !tt.hasError && got != tt.expected {
				t.Errorf("ParseOutputFormat(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestResult_FilterByCategory verifies category filtering.
func TestResult_FilterByCategory(t *testing.T) {
	result := NewResult("/test/repo")
	result.AddPackageResult(&PackageResult{
		Atom:      "app-misc/hello-2.10",
		Category:  "app-misc",
		Supported: true,
	})
	result.AddPackageResult(&PackageResult{
		Atom:      "sys-libs/zlib-1.2.13",
		Category:  "sys-libs",
		Supported: true,
	})
	result.Finalize()

	filtered := result.FilterByCategory("app-misc")

	if filtered.TotalPackages != 1 {
		t.Errorf("Filtered TotalPackages = %d, want 1", filtered.TotalPackages)
	}
	if len(filtered.Packages) != 1 || filtered.Packages[0].Category != "app-misc" {
		t.Error("Filter did not work correctly")
	}
}

// TestResult_BlockersByType verifies blocker type grouping.
func TestResult_BlockersByType(t *testing.T) {
	result := NewResult("/test/repo")

	// Add packages with different blocker types
	result.AddPackageResult(&PackageResult{
		Atom:      "cat/pkg1",
		Category:  "cat",
		Supported: false,
		Blockers:  []Blocker{{Type: BlockerMissingEclass, Name: "a"}},
	})
	result.AddPackageResult(&PackageResult{
		Atom:      "cat/pkg2",
		Category:  "cat",
		Supported: false,
		Blockers:  []Blocker{{Type: BlockerMissingEclass, Name: "b"}},
	})
	result.AddPackageResult(&PackageResult{
		Atom:      "cat/pkg3",
		Category:  "cat",
		Supported: false,
		Blockers:  []Blocker{{Type: BlockerUnsupportedEAPI, Name: "9"}},
	})

	byType := result.BlockersByType()

	if byType[BlockerMissingEclass] != 2 {
		t.Errorf("missing_eclass count = %d, want 2", byType[BlockerMissingEclass])
	}
	if byType[BlockerUnsupportedEAPI] != 1 {
		t.Errorf("unsupported_eapi count = %d, want 1", byType[BlockerUnsupportedEAPI])
	}
}

// TestNewAnalyzer_InvalidPath verifies error on invalid path.
func TestNewAnalyzer_InvalidPath(t *testing.T) {
	_, err := NewAnalyzer("/nonexistent/path/to/repo")
	if err == nil {
		t.Error("Expected error for nonexistent path")
	}
}

// TestAnalyzer_WithMockRepo tests analyzer with mock repository structure.
func TestAnalyzer_WithMockRepo(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create category directory
	catDir := filepath.Join(tmpDir, "app-misc")
	if err := os.MkdirAll(catDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create package directory
	pkgDir := filepath.Join(catDir, "hello")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create ebuild file
	ebuildContent := `EAPI=8

DESCRIPTION="Test package"
HOMEPAGE="https://example.com"

inherit cmake

SRC_URI="https://example.com/hello-2.10.tar.gz"

SLOT="0"
KEYWORDS="amd64 x86"
`
	ebuildPath := filepath.Join(pkgDir, "hello-2.10.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create eclass directory
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create cmake.eclass
	eclassContent := `# cmake.eclass - CMake build system support`
	eclassPath := filepath.Join(eclassDir, "cmake.eclass")
	if err := os.WriteFile(eclassPath, []byte(eclassContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create analyzer
	analyzer, err := NewAnalyzer(tmpDir)
	if err != nil {
		t.Fatalf("NewAnalyzer failed: %v", err)
	}

	// Run analysis
	ctx := context.Background()
	result, err := analyzer.Analyze(ctx)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Verify results
	if result.TotalPackages != 1 {
		t.Errorf("TotalPackages = %d, want 1", result.TotalPackages)
	}
	if result.SupportedPackages != 1 {
		t.Errorf("SupportedPackages = %d, want 1", result.SupportedPackages)
	}
	if result.Coverage != 100.0 {
		t.Errorf("Coverage = %.1f, want 100.0", result.Coverage)
	}

	// Verify package details
	if len(result.Packages) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(result.Packages))
	}
	pkg := result.Packages[0]
	if pkg.EAPI != "8" {
		t.Errorf("EAPI = %q, want %q", pkg.EAPI, "8")
	}
	if len(pkg.Inherits) != 1 || pkg.Inherits[0] != "cmake" {
		t.Errorf("Inherits = %v, want [cmake]", pkg.Inherits)
	}
}

// TestAnalyzer_WithCategoryFilter tests category-specific analysis.
func TestAnalyzer_WithCategoryFilter(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create two categories
	for _, cat := range []string{"app-misc", "sys-libs"} {
		catDir := filepath.Join(tmpDir, cat)
		if err := os.MkdirAll(catDir, 0755); err != nil {
			t.Fatal(err)
		}

		pkgDir := filepath.Join(catDir, "pkg")
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			t.Fatal(err)
		}

		ebuildContent := "EAPI=8\nDESCRIPTION=\"Test\""
		ebuildPath := filepath.Join(pkgDir, "pkg-1.0.ebuild")
		if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Analyze only app-misc
	analyzer, err := NewAnalyzer(tmpDir, WithCategory("app-misc"))
	if err != nil {
		t.Fatalf("NewAnalyzer failed: %v", err)
	}

	ctx := context.Background()
	result, err := analyzer.Analyze(ctx)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalPackages != 1 {
		t.Errorf("TotalPackages = %d, want 1 (only app-misc)", result.TotalPackages)
	}
}

// TestFormatSummary verifies summary formatting.
func TestFormatSummary(t *testing.T) {
	result := NewResult("/test/repo")
	result.AddPackageResult(&PackageResult{
		Atom:      "app-misc/hello-2.10",
		Category:  "app-misc",
		Supported: true,
	})
	result.AddPackageResult(&PackageResult{
		Atom:      "sys-libs/zlib-1.2.13",
		Category:  "sys-libs",
		Supported: false,
		Blockers:  []Blocker{{Type: BlockerMissingEclass, Name: "test"}},
	})
	result.Finalize()

	summary := FormatSummary(result)

	if !strings.Contains(summary, "50.0%") {
		t.Errorf("Summary missing coverage percentage: %q", summary)
	}
	if !strings.Contains(summary, "1/2") {
		t.Errorf("Summary missing package counts: %q", summary)
	}
}
