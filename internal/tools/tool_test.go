package tools

import (
	"testing"
)

func TestNewTool(t *testing.T) {
	tool := NewTool("cmake", "cmake", "dev-build/cmake", "Cross-platform build system")

	if tool.Name != "cmake" {
		t.Errorf("expected Name 'cmake', got %q", tool.Name)
	}
	if tool.Binary != "cmake" {
		t.Errorf("expected Binary 'cmake', got %q", tool.Binary)
	}
	if tool.Package != "dev-build/cmake" {
		t.Errorf("expected Package 'dev-build/cmake', got %q", tool.Package)
	}
	if tool.Description != "Cross-platform build system" {
		t.Errorf("expected Description 'Cross-platform build system', got %q", tool.Description)
	}
	if len(tool.Categories) != 0 {
		t.Errorf("expected empty Categories, got %v", tool.Categories)
	}
	if len(tool.RequiredBy) != 0 {
		t.Errorf("expected empty RequiredBy, got %v", tool.RequiredBy)
	}
	if tool.Optional {
		t.Error("expected Optional to be false")
	}
}

func TestToolWithCategories(t *testing.T) {
	tool := NewTool("cmake", "cmake", "dev-build/cmake", "Build system").
		WithCategories(CategoryBuildSystem, CategoryUtility)

	if len(tool.Categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(tool.Categories))
	}
	if tool.Categories[0] != CategoryBuildSystem {
		t.Errorf("expected CategoryBuildSystem, got %q", tool.Categories[0])
	}
	if tool.Categories[1] != CategoryUtility {
		t.Errorf("expected CategoryUtility, got %q", tool.Categories[1])
	}
}

func TestToolWithRequiredBy(t *testing.T) {
	tool := NewTool("cmake", "cmake", "dev-build/cmake", "Build system").
		WithRequiredBy("cmake", "cmake-multilib")

	if len(tool.RequiredBy) != 2 {
		t.Fatalf("expected 2 eclasses, got %d", len(tool.RequiredBy))
	}
	if tool.RequiredBy[0] != "cmake" {
		t.Errorf("expected 'cmake', got %q", tool.RequiredBy[0])
	}
	if tool.RequiredBy[1] != "cmake-multilib" {
		t.Errorf("expected 'cmake-multilib', got %q", tool.RequiredBy[1])
	}
}

func TestToolWithOptional(t *testing.T) {
	tool := NewTool("clang", "clang", "sys-devel/clang", "LLVM compiler").
		WithOptional()

	if !tool.Optional {
		t.Error("expected Optional to be true")
	}
}

func TestToolHasCategory(t *testing.T) {
	tool := NewTool("cmake", "cmake", "dev-build/cmake", "Build system").
		WithCategories(CategoryBuildSystem)

	if !tool.HasCategory(CategoryBuildSystem) {
		t.Error("expected HasCategory(CategoryBuildSystem) to be true")
	}
	if tool.HasCategory(CategoryCompiler) {
		t.Error("expected HasCategory(CategoryCompiler) to be false")
	}
}

func TestToolIsRequiredByEclass(t *testing.T) {
	tool := NewTool("cmake", "cmake", "dev-build/cmake", "Build system").
		WithRequiredBy("cmake")

	if !tool.IsRequiredByEclass("cmake") {
		t.Error("expected IsRequiredByEclass('cmake') to be true")
	}
	if tool.IsRequiredByEclass("meson") {
		t.Error("expected IsRequiredByEclass('meson') to be false")
	}
}

func TestToolString(t *testing.T) {
	tool := NewTool("cmake", "cmake", "dev-build/cmake", "Build system")
	expected := "cmake (dev-build/cmake)"
	if tool.String() != expected {
		t.Errorf("expected %q, got %q", expected, tool.String())
	}
}

func TestToolInstallHint(t *testing.T) {
	tool := NewTool("cmake", "cmake", "dev-build/cmake", "Build system")
	expected := "Run: grpm install dev-build/cmake"
	if tool.InstallHint() != expected {
		t.Errorf("expected %q, got %q", expected, tool.InstallHint())
	}
}

func TestRegistryBasicOperations(t *testing.T) {
	r := NewRegistry()

	// Test empty registry
	if r.Count() != 0 {
		t.Errorf("expected empty registry, got %d tools", r.Count())
	}

	// Register a tool
	tool := NewTool("cmake", "cmake", "dev-build/cmake", "Build system")
	r.Register(tool)

	// Test Count
	if r.Count() != 1 {
		t.Errorf("expected 1 tool, got %d", r.Count())
	}

	// Test Get
	retrieved := r.Get("cmake")
	if retrieved == nil {
		t.Fatal("expected to retrieve cmake tool")
	}
	if retrieved.Name != "cmake" {
		t.Errorf("expected Name 'cmake', got %q", retrieved.Name)
	}

	// Test Has
	if !r.Has("cmake") {
		t.Error("expected Has('cmake') to be true")
	}
	if r.Has("nonexistent") {
		t.Error("expected Has('nonexistent') to be false")
	}

	// Test Get for nonexistent
	if r.Get("nonexistent") != nil {
		t.Error("expected Get('nonexistent') to return nil")
	}
}

func TestRegistryAll(t *testing.T) {
	r := NewRegistry()
	r.Register(NewTool("cmake", "cmake", "dev-build/cmake", "Build system"))
	r.Register(NewTool("ninja", "ninja", "dev-build/ninja", "Build tool"))
	r.Register(NewTool("make", "make", "sys-devel/make", "Build tool"))

	all := r.All()
	if len(all) != 3 {
		t.Errorf("expected 3 tools, got %d", len(all))
	}

	// Verify returned slice is a copy
	all[0] = nil
	if r.Get("cmake") == nil {
		t.Error("modifying returned slice affected registry")
	}
}

func TestRegistryByCategory(t *testing.T) {
	r := NewRegistry()
	r.Register(NewTool("cmake", "cmake", "dev-build/cmake", "Build system").
		WithCategories(CategoryBuildSystem))
	r.Register(NewTool("ninja", "ninja", "dev-build/ninja", "Build tool").
		WithCategories(CategoryBuildSystem))
	r.Register(NewTool("gcc", "gcc", "sys-devel/gcc", "Compiler").
		WithCategories(CategoryCompiler))

	buildTools := r.ByCategory(CategoryBuildSystem)
	if len(buildTools) != 2 {
		t.Errorf("expected 2 build system tools, got %d", len(buildTools))
	}

	compilers := r.ByCategory(CategoryCompiler)
	if len(compilers) != 1 {
		t.Errorf("expected 1 compiler, got %d", len(compilers))
	}

	languages := r.ByCategory(CategoryLanguage)
	if len(languages) != 0 {
		t.Errorf("expected 0 language tools, got %d", len(languages))
	}
}

func TestRegistryByEclass(t *testing.T) {
	r := NewRegistry()
	r.Register(NewTool("cmake", "cmake", "dev-build/cmake", "Build system").
		WithRequiredBy("cmake"))
	r.Register(NewTool("ninja", "ninja", "dev-build/ninja", "Build tool").
		WithRequiredBy("cmake", "meson"))
	r.Register(NewTool("meson", "meson", "dev-build/meson", "Build system").
		WithRequiredBy("meson"))

	cmakeTools := r.ByEclass("cmake")
	if len(cmakeTools) != 2 {
		t.Errorf("expected 2 cmake tools, got %d", len(cmakeTools))
	}

	mesonTools := r.ByEclass("meson")
	if len(mesonTools) != 2 {
		t.Errorf("expected 2 meson tools, got %d", len(mesonTools))
	}

	// Test with .eclass suffix
	cmakeToolsWithSuffix := r.ByEclass("cmake.eclass")
	if len(cmakeToolsWithSuffix) != 2 {
		t.Errorf("expected 2 cmake tools (with .eclass suffix), got %d", len(cmakeToolsWithSuffix))
	}

	autotools := r.ByEclass("autotools")
	if len(autotools) != 0 {
		t.Errorf("expected 0 autotools tools, got %d", len(autotools))
	}
}

func TestRegistryReplace(t *testing.T) {
	r := NewRegistry()

	// Register initial tool
	r.Register(NewTool("cmake", "cmake", "dev-build/cmake", "Old description").
		WithCategories(CategoryBuildSystem))

	// Replace with new tool
	r.Register(NewTool("cmake", "cmake", "dev-build/cmake", "New description").
		WithCategories(CategoryUtility))

	// Should still have only 1 tool
	if r.Count() != 1 {
		t.Errorf("expected 1 tool after replacement, got %d", r.Count())
	}

	// Should have new description
	tool := r.Get("cmake")
	if tool.Description != "New description" {
		t.Errorf("expected 'New description', got %q", tool.Description)
	}

	// Category cache should be updated
	buildTools := r.ByCategory(CategoryBuildSystem)
	if len(buildTools) != 0 {
		t.Errorf("expected 0 build tools after replacement, got %d", len(buildTools))
	}

	utilTools := r.ByCategory(CategoryUtility)
	if len(utilTools) != 1 {
		t.Errorf("expected 1 utility tool after replacement, got %d", len(utilTools))
	}
}

func TestRegistryCategories(t *testing.T) {
	r := NewRegistry()
	r.Register(NewTool("cmake", "cmake", "dev-build/cmake", "Build system").
		WithCategories(CategoryBuildSystem))
	r.Register(NewTool("gcc", "gcc", "sys-devel/gcc", "Compiler").
		WithCategories(CategoryCompiler))

	categories := r.Categories()
	if len(categories) != 2 {
		t.Errorf("expected 2 categories, got %d", len(categories))
	}

	// Check that both categories are present
	hasBuildSystem := false
	hasCompiler := false
	for _, cat := range categories {
		if cat == CategoryBuildSystem {
			hasBuildSystem = true
		}
		if cat == CategoryCompiler {
			hasCompiler = true
		}
	}
	if !hasBuildSystem {
		t.Error("expected CategoryBuildSystem in categories")
	}
	if !hasCompiler {
		t.Error("expected CategoryCompiler in categories")
	}
}

func TestRegistryEclasses(t *testing.T) {
	r := NewRegistry()
	r.Register(NewTool("cmake", "cmake", "dev-build/cmake", "Build system").
		WithRequiredBy("cmake"))
	r.Register(NewTool("ninja", "ninja", "dev-build/ninja", "Build tool").
		WithRequiredBy("cmake", "meson"))

	eclasses := r.Eclasses()
	if len(eclasses) != 2 {
		t.Errorf("expected 2 eclasses, got %d", len(eclasses))
	}
}

func TestAllCategories(t *testing.T) {
	categories := AllCategories()

	if len(categories) < 5 {
		t.Errorf("expected at least 5 categories, got %d", len(categories))
	}

	// Verify expected categories exist
	expected := []ToolCategory{
		CategoryCompiler,
		CategoryBuildSystem,
		CategoryLanguage,
		CategoryUtility,
		CategoryCompression,
	}

	for _, cat := range expected {
		found := false
		for _, c := range categories {
			if c == cat {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected category %q in AllCategories()", cat)
		}
	}
}
