// Package sets implements Portage package sets.
//
// This file contains tests for package sets.
package sets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/profile"
)

// ============================================================================
// Selected Set Tests
// ============================================================================

func TestSelectedSet_Packages(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	os.MkdirAll(varLib, 0755)

	// Create world file
	worldFile := filepath.Join(varLib, "world")
	content := `app-editors/vim
dev-lang/go
sys-apps/portage
# comment line
www-client/firefox
`
	os.WriteFile(worldFile, []byte(content), 0644)

	// Create selected set
	selected := NewSelectedSet(tmpDir)

	// Test Packages()
	atoms, err := selected.Packages()
	if err != nil {
		t.Fatalf("Packages() error: %v", err)
	}

	if len(atoms) != 4 {
		t.Errorf("expected 4 packages, got %d", len(atoms))
	}

	// Verify packages
	expected := []string{"app-editors/vim", "dev-lang/go", "sys-apps/portage", "www-client/firefox"}
	for i, exp := range expected {
		if i >= len(atoms) {
			break
		}
		got := atoms[i].Category + "/" + atoms[i].Package
		if got != exp {
			t.Errorf("package %d: expected %s, got %s", i, exp, got)
		}
	}
}

func TestSelectedSet_Add(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	os.MkdirAll(varLib, 0755)

	// Create empty world file
	worldFile := filepath.Join(varLib, "world")
	os.WriteFile(worldFile, []byte(""), 0644)

	// Create selected set
	selected := NewSelectedSet(tmpDir)

	// Add a package
	atom, _ := pkg.ParseAtom("dev-lang/rust")
	err := selected.Add(atom)
	if err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	// Verify it's in the set
	atoms, _ := selected.Packages()
	if len(atoms) != 1 {
		t.Errorf("expected 1 package, got %d", len(atoms))
	}
	if atoms[0].Category != "dev-lang" || atoms[0].Package != "rust" {
		t.Errorf("unexpected package: %s/%s", atoms[0].Category, atoms[0].Package)
	}

	// Add duplicate (should not add)
	err = selected.Add(atom)
	if err != nil {
		t.Fatalf("Add() duplicate error: %v", err)
	}
	atoms, _ = selected.Packages()
	if len(atoms) != 1 {
		t.Errorf("duplicate add should not increase count, got %d", len(atoms))
	}
}

func TestSelectedSet_Remove(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	os.MkdirAll(varLib, 0755)

	// Create world file with packages
	worldFile := filepath.Join(varLib, "world")
	content := `app-editors/vim
dev-lang/go
`
	os.WriteFile(worldFile, []byte(content), 0644)

	// Create selected set
	selected := NewSelectedSet(tmpDir)

	// Remove a package
	atom, _ := pkg.ParseAtom("app-editors/vim")
	err := selected.Remove(atom)
	if err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	// Verify it's removed
	atoms, _ := selected.Packages()
	if len(atoms) != 1 {
		t.Errorf("expected 1 package after remove, got %d", len(atoms))
	}
	if atoms[0].Package != "go" {
		t.Errorf("wrong package remaining: %s", atoms[0].Package)
	}
}

func TestSelectedSet_Contains(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	os.MkdirAll(varLib, 0755)

	// Create world file
	worldFile := filepath.Join(varLib, "world")
	os.WriteFile(worldFile, []byte("dev-lang/go\n"), 0644)

	selected := NewSelectedSet(tmpDir)

	// Test Contains
	atom, _ := pkg.ParseAtom("dev-lang/go")
	if !selected.Contains(atom) {
		t.Error("Contains() should return true for existing package")
	}

	atom2, _ := pkg.ParseAtom("dev-lang/rust")
	if selected.Contains(atom2) {
		t.Error("Contains() should return false for non-existing package")
	}
}

// ============================================================================
// System Set Tests
// ============================================================================

func TestSystemSet_Packages(t *testing.T) {
	// Create temp directory with profile structure
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, "var", "db", "repos", "gentoo", "profiles", "base")
	os.MkdirAll(profileDir, 0755)

	// Create packages file
	packagesFile := filepath.Join(profileDir, "packages")
	content := `# Base system packages
*sys-apps/baselayout
*sys-libs/glibc
*sys-apps/openrc
# Non-system package (no *)
app-misc/hello
`
	os.WriteFile(packagesFile, []byte(content), 0644)

	// Create profile
	prof := &profile.Profile{
		Path: profileDir,
	}

	// Create system set
	system := NewSystemSet(tmpDir, prof)

	// Test Packages()
	atoms, err := system.Packages()
	if err != nil {
		t.Fatalf("Packages() error: %v", err)
	}

	// Should have 3 system packages (lines starting with *)
	if len(atoms) != 3 {
		t.Errorf("expected 3 system packages, got %d", len(atoms))
	}
}

func TestSystemSet_NilProfile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create system set with nil profile
	system := NewSystemSet(tmpDir, nil)

	// Should not error, just return empty
	atoms, err := system.Packages()
	if err != nil {
		t.Fatalf("Packages() with nil profile error: %v", err)
	}

	if len(atoms) != 0 {
		t.Errorf("expected 0 packages with nil profile, got %d", len(atoms))
	}
}

// ============================================================================
// World Set Tests
// ============================================================================

func TestWorldSet_Packages(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create world file
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	os.MkdirAll(varLib, 0755)
	worldFile := filepath.Join(varLib, "world")
	os.WriteFile(worldFile, []byte("app-editors/vim\n"), 0644)

	// Create profile with system packages
	profileDir := filepath.Join(tmpDir, "var", "db", "repos", "gentoo", "profiles", "base")
	os.MkdirAll(profileDir, 0755)
	packagesFile := filepath.Join(profileDir, "packages")
	os.WriteFile(packagesFile, []byte("*sys-libs/glibc\n"), 0644)

	prof := &profile.Profile{Path: profileDir}

	// Create sets
	selected := NewSelectedSet(tmpDir)
	system := NewSystemSet(tmpDir, prof)
	world := NewWorldSet(selected, system)

	// Test Packages() - should return both system and selected
	atoms, err := world.Packages()
	if err != nil {
		t.Fatalf("Packages() error: %v", err)
	}

	if len(atoms) != 2 {
		t.Errorf("expected 2 packages in @world, got %d", len(atoms))
	}
}

func TestWorldSet_Deduplicate(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create world file with same package as system
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	os.MkdirAll(varLib, 0755)
	worldFile := filepath.Join(varLib, "world")
	os.WriteFile(worldFile, []byte("sys-libs/glibc\n"), 0644)

	// Create profile with system packages
	profileDir := filepath.Join(tmpDir, "var", "db", "repos", "gentoo", "profiles", "base")
	os.MkdirAll(profileDir, 0755)
	packagesFile := filepath.Join(profileDir, "packages")
	os.WriteFile(packagesFile, []byte("*sys-libs/glibc\n"), 0644)

	prof := &profile.Profile{Path: profileDir}

	// Create sets
	selected := NewSelectedSet(tmpDir)
	system := NewSystemSet(tmpDir, prof)
	world := NewWorldSet(selected, system)

	// Test Packages() - should deduplicate
	atoms, err := world.Packages()
	if err != nil {
		t.Fatalf("Packages() error: %v", err)
	}

	if len(atoms) != 1 {
		t.Errorf("expected 1 package after dedup, got %d", len(atoms))
	}
}

// ============================================================================
// Registry Tests
// ============================================================================

func TestRegistry_GetSet(t *testing.T) {
	tmpDir := t.TempDir()

	// Create necessary directories
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	os.MkdirAll(varLib, 0755)
	os.WriteFile(filepath.Join(varLib, "world"), []byte(""), 0644)

	registry := NewRegistry(tmpDir, nil)

	tests := []struct {
		name    string
		setName string
		wantErr bool
	}{
		{"@world exists", "@world", false},
		{"@system exists", "@system", false},
		{"@selected exists", "@selected", false},
		{"@preserved-rebuild exists", "@preserved-rebuild", false},
		{"world without @ prefix", "world", false},
		{"unknown set", "@nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set, err := registry.GetSet(tt.setName)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if set == nil {
					t.Error("expected set, got nil")
				}
			}
		})
	}
}

func TestRegistry_RegisterSet(t *testing.T) {
	tmpDir := t.TempDir()

	// Create necessary directories
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	os.MkdirAll(varLib, 0755)
	os.WriteFile(filepath.Join(varLib, "world"), []byte(""), 0644)

	registry := NewRegistry(tmpDir, nil)

	// Create a custom file set
	customSetFile := filepath.Join(tmpDir, "custom")
	os.WriteFile(customSetFile, []byte("app-misc/hello\n"), 0644)

	customSet := NewFileSet("@custom", customSetFile)

	// Register it
	err := registry.RegisterSet("@custom", customSet)
	if err != nil {
		t.Fatalf("RegisterSet() error: %v", err)
	}

	// Should be able to get it
	set, err := registry.GetSet("@custom")
	if err != nil {
		t.Fatalf("GetSet() error: %v", err)
	}
	if set.Name() != "@custom" {
		t.Errorf("expected @custom, got %s", set.Name())
	}

	// Registering again should fail
	err = registry.RegisterSet("@custom", customSet)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestRegistry_ListSets(t *testing.T) {
	tmpDir := t.TempDir()

	// Create necessary directories
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	os.MkdirAll(varLib, 0755)
	os.WriteFile(filepath.Join(varLib, "world"), []byte(""), 0644)

	registry := NewRegistry(tmpDir, nil)

	sets := registry.ListSets()

	// Should have at least the 4 built-in sets
	if len(sets) < 4 {
		t.Errorf("expected at least 4 sets, got %d", len(sets))
	}

	// Check that built-in sets are present
	setMap := make(map[string]bool)
	for _, s := range sets {
		setMap[s] = true
	}

	required := []string{"@world", "@system", "@selected", "@preserved-rebuild"}
	for _, name := range required {
		if !setMap[name] {
			t.Errorf("missing required set: %s", name)
		}
	}
}

// ============================================================================
// File Set Tests
// ============================================================================

func TestFileSet_Packages(t *testing.T) {
	tmpDir := t.TempDir()

	// Create custom set file
	setFile := filepath.Join(tmpDir, "mydevtools")
	content := `# Development tools
dev-lang/go
dev-lang/rust
app-editors/neovim
# Nested set (skipped)
@some-other-set
`
	os.WriteFile(setFile, []byte(content), 0644)

	set := NewFileSet("@mydevtools", setFile)

	// Test Name()
	if set.Name() != "@mydevtools" {
		t.Errorf("expected @mydevtools, got %s", set.Name())
	}

	// Test Packages()
	atoms, err := set.Packages()
	if err != nil {
		t.Fatalf("Packages() error: %v", err)
	}

	if len(atoms) != 3 {
		t.Errorf("expected 3 packages, got %d", len(atoms))
	}
}

func TestLoadCustomSets(t *testing.T) {
	tmpDir := t.TempDir()

	// Create custom sets directory
	setsDir := filepath.Join(tmpDir, "etc", "portage", "sets")
	os.MkdirAll(setsDir, 0755)

	// Create some set files
	os.WriteFile(filepath.Join(setsDir, "mytools"), []byte("dev-lang/go\n"), 0644)
	os.WriteFile(filepath.Join(setsDir, "devstuff"), []byte("app-editors/vim\n"), 0644)

	// Load custom sets
	sets, err := LoadCustomSets(tmpDir)
	if err != nil {
		t.Fatalf("LoadCustomSets() error: %v", err)
	}

	if len(sets) != 2 {
		t.Errorf("expected 2 custom sets, got %d", len(sets))
	}
}

// ============================================================================
// Preserved Rebuild Set Tests
// ============================================================================

func TestPreservedRebuildSet_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	set := NewPreservedRebuildSet(tmpDir)

	// No registry file - should return empty
	atoms, err := set.Packages()
	if err != nil {
		t.Fatalf("Packages() error: %v", err)
	}

	if len(atoms) != 0 {
		t.Errorf("expected 0 packages for missing registry, got %d", len(atoms))
	}
}

func TestPreservedRebuildSet_WithPackages(t *testing.T) {
	tmpDir := t.TempDir()

	// Create preserved libs registry
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	os.MkdirAll(varLib, 0755)

	registryFile := filepath.Join(varLib, "preserved_libs_registry")
	content := `sys-libs/glibc-2.38: [libpthread.so.0, libc.so.6]
media-libs/libpng-1.6.40: [libpng16.so.16]
`
	os.WriteFile(registryFile, []byte(content), 0644)

	set := NewPreservedRebuildSet(tmpDir)

	atoms, err := set.Packages()
	if err != nil {
		t.Fatalf("Packages() error: %v", err)
	}

	if len(atoms) != 2 {
		t.Errorf("expected 2 packages, got %d", len(atoms))
	}
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func TestDeduplicate(t *testing.T) {
	atom1, _ := pkg.ParseAtom("dev-lang/go")
	atom2, _ := pkg.ParseAtom("dev-lang/rust")
	atom3, _ := pkg.ParseAtom("dev-lang/go") // duplicate

	atoms := []*pkg.Atom{atom1, atom2, atom3}
	result := Deduplicate(atoms)

	if len(result) != 2 {
		t.Errorf("expected 2 unique atoms, got %d", len(result))
	}
}

func TestIsSetReference(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"@world", true},
		{"@system", true},
		{"@custom-set", true},
		{"dev-lang/go", false},
		{"world", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsSetReference(tt.input)
			if got != tt.want {
				t.Errorf("IsSetReference(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
