package tools

import (
	"os"
	"runtime"
	"testing"
)

func TestNewDetector(t *testing.T) {
	r := NewRegistry()
	d := NewDetector(r)

	if d == nil {
		t.Fatal("expected non-nil detector")
	}
	if d.registry != r {
		t.Error("expected detector to reference provided registry")
	}
}

func TestDetectorIsAvailable(t *testing.T) {
	r := NewRegistry()

	// Register tools we expect to exist on most systems
	if runtime.GOOS == "windows" {
		r.Register(NewTool("cmd", "cmd.exe", "n/a", "Windows command processor"))
	} else {
		r.Register(NewTool("sh", "sh", "n/a", "POSIX shell"))
	}

	// Register a tool that definitely doesn't exist
	r.Register(NewTool("nonexistent-tool-12345", "nonexistent-tool-12345", "n/a", "Does not exist"))

	d := NewDetector(r)

	// Test existing tool
	if runtime.GOOS == "windows" {
		if !d.IsAvailable("cmd") {
			t.Error("expected cmd.exe to be available on Windows")
		}
	} else {
		if !d.IsAvailable("sh") {
			t.Error("expected sh to be available on Unix")
		}
	}

	// Test nonexistent tool
	if d.IsAvailable("nonexistent-tool-12345") {
		t.Error("expected nonexistent-tool-12345 to not be available")
	}
}

func TestDetectorFindBinary(t *testing.T) {
	r := NewRegistry()

	// Register a tool we expect to exist
	if runtime.GOOS == "windows" {
		r.Register(NewTool("cmd", "cmd.exe", "n/a", "Windows command processor"))
	} else {
		r.Register(NewTool("sh", "sh", "n/a", "POSIX shell"))
	}

	d := NewDetector(r)

	// Test existing tool
	var toolName string
	if runtime.GOOS == "windows" {
		toolName = "cmd"
	} else {
		toolName = "sh"
	}

	path, found := d.FindBinary(toolName)
	if !found {
		t.Errorf("expected to find %s", toolName)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}

	// Verify file exists
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("path does not exist: %s", path)
	} else if info.IsDir() {
		t.Errorf("path is a directory, not a file: %s", path)
	}
}

func TestDetectorCaching(t *testing.T) {
	r := NewRegistry()
	r.Register(NewTool("test-cache", "nonexistent-binary-for-caching", "n/a", "Test"))

	d := NewDetector(r)

	// First call should perform detection
	result1 := d.IsAvailable("test-cache")

	// Second call should use cache
	result2 := d.IsAvailable("test-cache")

	// Results should be consistent
	if result1 != result2 {
		t.Error("cached result differs from initial result")
	}

	// Verify cache was populated
	d.mu.RLock()
	_, inCache := d.cache["test-cache"]
	d.mu.RUnlock()

	if !inCache {
		t.Error("expected result to be cached")
	}
}

func TestDetectorReset(t *testing.T) {
	r := NewRegistry()
	r.Register(NewTool("test-reset", "nonexistent", "n/a", "Test"))

	d := NewDetector(r)

	// Populate cache
	d.IsAvailable("test-reset")

	// Verify cache is populated
	d.mu.RLock()
	_, inCache := d.cache["test-reset"]
	d.mu.RUnlock()
	if !inCache {
		t.Error("expected cache to be populated before reset")
	}

	// Reset cache
	d.Reset()

	// Verify cache is empty
	d.mu.RLock()
	_, inCache = d.cache["test-reset"]
	d.mu.RUnlock()
	if inCache {
		t.Error("expected cache to be empty after reset")
	}
}

func TestDetectorCheckAll(t *testing.T) {
	r := NewRegistry()
	r.Register(NewTool("tool1", "nonexistent1", "n/a", "Test 1"))
	r.Register(NewTool("tool2", "nonexistent2", "n/a", "Test 2"))

	d := NewDetector(r)

	result := d.CheckAll()

	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}

	if _, ok := result["tool1"]; !ok {
		t.Error("expected tool1 in results")
	}
	if _, ok := result["tool2"]; !ok {
		t.Error("expected tool2 in results")
	}
}

func TestDetectorCheckCategory(t *testing.T) {
	r := NewRegistry()
	r.Register(NewTool("build1", "nonexistent1", "n/a", "Build 1").
		WithCategories(CategoryBuildSystem))
	r.Register(NewTool("build2", "nonexistent2", "n/a", "Build 2").
		WithCategories(CategoryBuildSystem))
	r.Register(NewTool("compiler1", "nonexistent3", "n/a", "Compiler").
		WithCategories(CategoryCompiler))

	d := NewDetector(r)

	// Check build system category
	buildResult := d.CheckCategory(CategoryBuildSystem)
	if len(buildResult) != 2 {
		t.Errorf("expected 2 build system results, got %d", len(buildResult))
	}

	// Check compiler category
	compilerResult := d.CheckCategory(CategoryCompiler)
	if len(compilerResult) != 1 {
		t.Errorf("expected 1 compiler result, got %d", len(compilerResult))
	}
}

func TestDetectorMissing(t *testing.T) {
	r := NewRegistry()
	r.Register(NewTool("missing1", "nonexistent1", "n/a", "Missing 1"))
	r.Register(NewTool("missing2", "nonexistent2", "n/a", "Missing 2"))

	d := NewDetector(r)

	missing := d.Missing()

	if len(missing) != 2 {
		t.Errorf("expected 2 missing tools, got %d", len(missing))
	}
}

func TestDetectorMissingForEclass(t *testing.T) {
	r := NewRegistry()
	r.Register(NewTool("cmake", "nonexistent-cmake", "dev-build/cmake", "Build system").
		WithRequiredBy("cmake"))
	r.Register(NewTool("ninja", "nonexistent-ninja", "dev-build/ninja", "Build tool").
		WithRequiredBy("cmake"))
	r.Register(NewTool("optional", "nonexistent-optional", "n/a", "Optional").
		WithRequiredBy("cmake").WithOptional())

	d := NewDetector(r)

	missing := d.MissingForEclass("cmake")

	// Should only have 2 (cmake and ninja), not the optional tool
	if len(missing) != 2 {
		t.Errorf("expected 2 missing required tools, got %d", len(missing))
	}

	// Verify optional tool is not included
	for _, tool := range missing {
		if tool.Name == "optional" {
			t.Error("optional tool should not be in missing required list")
		}
	}
}

func TestDetectorMissingForEclasses(t *testing.T) {
	r := NewRegistry()
	r.Register(NewTool("cmake", "nonexistent-cmake", "dev-build/cmake", "Build system").
		WithRequiredBy("cmake"))
	r.Register(NewTool("meson", "nonexistent-meson", "dev-build/meson", "Build system").
		WithRequiredBy("meson"))
	r.Register(NewTool("ninja", "nonexistent-ninja", "dev-build/ninja", "Build tool").
		WithRequiredBy("cmake", "meson")) // Shared by both

	d := NewDetector(r)

	missing := d.MissingForEclasses([]string{"cmake", "meson"})

	// Should have 3 unique tools
	if len(missing) != 3 {
		t.Errorf("expected 3 missing tools, got %d", len(missing))
	}

	// Ninja should only appear once even though required by both eclasses
	ninjaCount := 0
	for _, tool := range missing {
		if tool.Name == "ninja" {
			ninjaCount++
		}
	}
	if ninjaCount != 1 {
		t.Errorf("expected ninja to appear once, appeared %d times", ninjaCount)
	}
}

func TestDetectorAvailable(t *testing.T) {
	r := NewRegistry()

	// Add a tool that exists on most systems
	if runtime.GOOS == "windows" {
		r.Register(NewTool("cmd", "cmd.exe", "n/a", "Windows command processor"))
	} else {
		r.Register(NewTool("sh", "sh", "n/a", "POSIX shell"))
	}

	// Add a tool that doesn't exist
	r.Register(NewTool("nonexistent", "nonexistent-binary", "n/a", "Does not exist"))

	d := NewDetector(r)

	available := d.Available()

	// Should have at least 1 available tool (sh or cmd)
	if len(available) < 1 {
		t.Error("expected at least 1 available tool")
	}

	// Verify nonexistent is not in available list
	for _, tool := range available {
		if tool.Name == "nonexistent" {
			t.Error("nonexistent tool should not be in available list")
		}
	}
}

func TestDetectorSummary(t *testing.T) {
	r := NewRegistry()
	r.Register(NewTool("missing1", "nonexistent1", "n/a", "Missing").
		WithCategories(CategoryBuildSystem))
	r.Register(NewTool("missing2", "nonexistent2", "n/a", "Missing").
		WithCategories(CategoryCompiler))

	d := NewDetector(r)

	summary := d.Summary()

	if summary.Total != 2 {
		t.Errorf("expected Total=2, got %d", summary.Total)
	}
	if summary.Missing != 2 {
		t.Errorf("expected Missing=2, got %d", summary.Missing)
	}
	if summary.Available != 0 {
		t.Errorf("expected Available=0, got %d", summary.Available)
	}
	if len(summary.MissingTools) != 2 {
		t.Errorf("expected 2 missing tools, got %d", len(summary.MissingTools))
	}

	// Check category summary
	buildSummary := summary.ByCategory[CategoryBuildSystem]
	if buildSummary.Total != 1 {
		t.Errorf("expected build system Total=1, got %d", buildSummary.Total)
	}
	if buildSummary.Missing != 1 {
		t.Errorf("expected build system Missing=1, got %d", buildSummary.Missing)
	}
}

func TestDetectionSummaryAvailabilityPercent(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		available int
		expected  float64
	}{
		{"all available", 10, 10, 100.0},
		{"none available", 10, 0, 0.0},
		{"half available", 10, 5, 50.0},
		{"empty", 0, 0, 100.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := &DetectionSummary{
				Total:     tt.total,
				Available: tt.available,
			}
			result := summary.AvailabilityPercent()
			if result != tt.expected {
				t.Errorf("expected %.1f%%, got %.1f%%", tt.expected, result)
			}
		})
	}
}

func TestLookupPaths(t *testing.T) {
	paths := LookupPaths()

	// Should have at least one path on any system
	if len(paths) == 0 {
		t.Skip("no PATH directories found (unusual but valid)")
	}

	// All paths should be valid directories
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("path does not exist: %s", path)
			continue
		}
		if !info.IsDir() {
			t.Errorf("path is not a directory: %s", path)
		}
	}
}

func TestGetResult(t *testing.T) {
	r := NewRegistry()
	r.Register(NewTool("test", "nonexistent-test-binary", "n/a", "Test"))

	d := NewDetector(r)

	result := d.GetResult("test")

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Available {
		t.Error("expected Available to be false")
	}
	if result.Path != "" {
		t.Errorf("expected empty Path, got %q", result.Path)
	}
}

func TestDetectorWithBinaryOverride(t *testing.T) {
	r := NewRegistry()

	// Register tool with different binary name
	if runtime.GOOS == "windows" {
		r.Register(NewTool("windows-cmd", "cmd.exe", "n/a", "Windows command with different name"))
	} else {
		r.Register(NewTool("shell-tool", "sh", "n/a", "Shell tool with different name"))
	}

	d := NewDetector(r)

	// Should find by tool name, searching for the binary
	var toolName string
	if runtime.GOOS == "windows" {
		toolName = "windows-cmd"
	} else {
		toolName = "shell-tool"
	}

	if !d.IsAvailable(toolName) {
		t.Errorf("expected %s to be available (binary override)", toolName)
	}
}
