package tools

import (
	"testing"
)

func TestNewDefaultRegistry(t *testing.T) {
	r := NewDefaultRegistry()

	// Should have many tools registered
	if r.Count() < 30 {
		t.Errorf("expected at least 30 default tools, got %d", r.Count())
	}
}

func TestDefaultRegistryHasExpectedTools(t *testing.T) {
	r := NewDefaultRegistry()

	// Check for essential tools
	expectedTools := []string{
		// Compilers
		"gcc", "g++", "clang",
		// Build systems
		"make", "ninja", "cmake", "meson",
		// Languages
		"python", "perl", "go", "rustc",
		// Utilities
		"patch", "sed", "wget", "curl",
		// Compression
		"gzip", "bzip2", "xz", "tar",
		// VCS
		"git",
	}

	for _, name := range expectedTools {
		if !r.Has(name) {
			t.Errorf("expected default tool %q to be registered", name)
		}
	}
}

func TestDefaultRegistryCategories(t *testing.T) {
	r := NewDefaultRegistry()

	// Each category should have at least one tool
	for _, cat := range AllCategories() {
		tools := r.ByCategory(cat)
		if len(tools) == 0 {
			t.Errorf("expected category %q to have at least one tool", cat)
		}
	}
}

func TestDefaultRegistryEclassAssociations(t *testing.T) {
	r := NewDefaultRegistry()

	// Check specific eclass associations
	tests := []struct {
		eclass   string
		expected []string
	}{
		{"cmake", []string{"cmake", "ninja"}},
		{"meson", []string{"meson", "ninja"}},
		{"cargo", []string{"cargo", "rustc"}},
		{"go-module", []string{"go"}},
		{"git-r3", []string{"git"}},
	}

	for _, tt := range tests {
		tools := r.ByEclass(tt.eclass)
		toolNames := make(map[string]bool)
		for _, tool := range tools {
			toolNames[tool.Name] = true
		}

		for _, expected := range tt.expected {
			if !toolNames[expected] {
				t.Errorf("expected eclass %q to require tool %q", tt.eclass, expected)
			}
		}
	}
}

func TestDefaultRegistryToolPackages(t *testing.T) {
	r := NewDefaultRegistry()

	// All tools should have valid Gentoo packages
	for _, tool := range r.All() {
		if tool.Package == "" {
			t.Errorf("tool %q has empty Package field", tool.Name)
		}
		// Package should be in category/name format
		if tool.Package != "n/a" && !containsSlash(tool.Package) {
			t.Errorf("tool %q has invalid Package format: %q (expected category/name)", tool.Name, tool.Package)
		}
	}
}

func containsSlash(s string) bool {
	for _, c := range s {
		if c == '/' {
			return true
		}
	}
	return false
}

func TestDefaultRegistryToolBinaries(t *testing.T) {
	r := NewDefaultRegistry()

	// All tools should have non-empty Binary field
	for _, tool := range r.All() {
		if tool.Binary == "" {
			t.Errorf("tool %q has empty Binary field", tool.Name)
		}
	}
}

func TestDefaultRegistryToolDescriptions(t *testing.T) {
	r := NewDefaultRegistry()

	// All tools should have descriptions
	for _, tool := range r.All() {
		if tool.Description == "" {
			t.Errorf("tool %q has empty Description field", tool.Name)
		}
		// Description should be reasonably short
		if len(tool.Description) > 100 {
			t.Errorf("tool %q has overly long Description (%d chars)", tool.Name, len(tool.Description))
		}
	}
}

func TestEclassToolMap(t *testing.T) {
	m := EclassToolMap()

	// Should have many eclasses mapped
	if len(m) < 10 {
		t.Errorf("expected at least 10 eclasses in map, got %d", len(m))
	}

	// Check specific mappings
	tests := []struct {
		eclass   string
		expected []string
	}{
		{"cmake", []string{"cmake", "ninja", "make"}},
		{"meson", []string{"meson", "ninja"}},
		{"cargo", []string{"cargo", "rustc"}},
		{"go-module", []string{"go"}},
		{"python-single-r1", []string{"python", "python3"}},
	}

	for _, tt := range tests {
		tools, ok := m[tt.eclass]
		if !ok {
			t.Errorf("expected eclass %q in EclassToolMap", tt.eclass)
			continue
		}

		for _, expected := range tt.expected {
			found := false
			for _, tool := range tools {
				if tool == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected tool %q in EclassToolMap[%q]", expected, tt.eclass)
			}
		}
	}
}

func TestDefaultRegistryCompilerTools(t *testing.T) {
	r := NewDefaultRegistry()

	compilers := r.ByCategory(CategoryCompiler)

	// Should have GCC and Clang at minimum
	names := make(map[string]bool)
	for _, tool := range compilers {
		names[tool.Name] = true
	}

	if !names["gcc"] {
		t.Error("expected gcc in compilers")
	}
	if !names["clang"] {
		t.Error("expected clang in compilers")
	}
}

func TestDefaultRegistryBuildSystemTools(t *testing.T) {
	r := NewDefaultRegistry()

	buildSystems := r.ByCategory(CategoryBuildSystem)

	// Should have common build systems
	names := make(map[string]bool)
	for _, tool := range buildSystems {
		names[tool.Name] = true
	}

	expected := []string{"make", "ninja", "cmake", "meson", "autoconf", "automake"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected %q in build systems", name)
		}
	}
}

func TestDefaultRegistryVCSTools(t *testing.T) {
	r := NewDefaultRegistry()

	vcs := r.ByCategory(CategoryVCS)

	// Should have common VCS tools
	names := make(map[string]bool)
	for _, tool := range vcs {
		names[tool.Name] = true
	}

	if !names["git"] {
		t.Error("expected git in VCS tools")
	}
}

func TestDefaultRegistryOptionalTools(t *testing.T) {
	r := NewDefaultRegistry()

	// Some tools should be marked optional
	hasOptional := false
	for _, tool := range r.All() {
		if tool.Optional {
			hasOptional = true
			break
		}
	}

	if !hasOptional {
		t.Error("expected at least one optional tool in defaults")
	}

	// Clang should be optional (alternative to GCC)
	clang := r.Get("clang")
	if clang != nil && !clang.Optional {
		t.Error("expected clang to be marked as optional")
	}
}
