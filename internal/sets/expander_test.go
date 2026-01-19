// Package sets implements Portage package sets.
//
// This file contains tests for the SetExpander service.
package sets

import (
	"os"
	"path/filepath"
	"testing"
)

// ============================================================================
// SetExpander Tests
// ============================================================================

// TestNewExpander tests SetExpander creation.
func TestNewExpander(t *testing.T) {
	tmpDir := t.TempDir()

	// Create necessary directories
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	if err := os.MkdirAll(varLib, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(varLib, "world"), []byte("dev-lang/go\n"), 0644); err != nil {
		t.Fatalf("failed to create world file: %v", err)
	}

	// Create expander
	expander := NewExpander(tmpDir, nil, nil)
	if expander == nil {
		t.Fatal("NewExpander returned nil")
	}

	// Check that registry is initialized
	if expander.registry == nil {
		t.Error("registry should not be nil")
	}

	// Check that sets are available
	sets := expander.ListSets()
	if len(sets) < 3 {
		t.Errorf("expected at least 3 sets, got %d", len(sets))
	}
}

// TestNewExpanderWithConfig tests expander creation with config.
func TestNewExpanderWithConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create necessary directories
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	if err := os.MkdirAll(varLib, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(varLib, "world"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to create world file: %v", err)
	}

	// Create with nil config (should use defaults)
	expander := NewExpanderWithConfig(nil)
	if expander == nil {
		t.Fatal("NewExpanderWithConfig(nil) returned nil")
	}

	// Create with explicit config
	cfg := &ExpanderConfig{
		RootDir:     tmpDir,
		PortageDir:  "/var/lib/portage",
		ProfilePath: "", // No profile
	}
	expander = NewExpanderWithConfig(cfg)
	if expander == nil {
		t.Fatal("NewExpanderWithConfig(cfg) returned nil")
	}
}

// TestExpand tests the main Expand method.
func TestExpand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create world file with packages
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	if err := os.MkdirAll(varLib, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	worldContent := `dev-lang/go
app-editors/vim
sys-apps/portage
`
	if err := os.WriteFile(filepath.Join(varLib, "world"), []byte(worldContent), 0644); err != nil {
		t.Fatalf("failed to create world file: %v", err)
	}

	expander := NewExpander(tmpDir, nil, nil)

	tests := []struct {
		name     string
		args     []string
		wantLen  int
		wantErr  bool
		contains []string
	}{
		{
			name:     "empty args",
			args:     []string{},
			wantLen:  0,
			wantErr:  false,
			contains: nil,
		},
		{
			name:     "regular package only",
			args:     []string{"app-misc/hello"},
			wantLen:  1,
			wantErr:  false,
			contains: []string{"app-misc/hello"},
		},
		{
			name:     "multiple regular packages",
			args:     []string{"app-misc/hello", "dev-lang/rust"},
			wantLen:  2,
			wantErr:  false,
			contains: []string{"app-misc/hello", "dev-lang/rust"},
		},
		{
			name:     "@selected set",
			args:     []string{"@selected"},
			wantLen:  3, // dev-lang/go, app-editors/vim, sys-apps/portage
			wantErr:  false,
			contains: []string{"dev-lang/go", "app-editors/vim", "sys-apps/portage"},
		},
		{
			name:     "mixed: set and regular package",
			args:     []string{"@selected", "app-misc/hello"},
			wantLen:  4, // 3 from @selected + 1 regular
			wantErr:  false,
			contains: []string{"dev-lang/go", "app-misc/hello"},
		},
		{
			name:     "unknown set",
			args:     []string{"@nonexistent"},
			wantLen:  0,
			wantErr:  true,
			contains: nil,
		},
		{
			name:     "deduplication",
			args:     []string{"dev-lang/go", "dev-lang/go"},
			wantLen:  1,
			wantErr:  false,
			contains: []string{"dev-lang/go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expander.Expand(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if len(result) != tt.wantLen {
				t.Errorf("expected %d results, got %d: %v", tt.wantLen, len(result), result)
			}
			// Check that expected packages are present
			resultSet := make(map[string]bool)
			for _, r := range result {
				resultSet[r] = true
			}
			for _, c := range tt.contains {
				if !resultSet[c] {
					t.Errorf("expected result to contain %s, got %v", c, result)
				}
			}
		})
	}
}

// TestExpandSet tests the ExpandSet method.
func TestExpandSet(t *testing.T) {
	tmpDir := t.TempDir()

	// Create world file
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	if err := os.MkdirAll(varLib, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(varLib, "world"), []byte("dev-lang/go\n"), 0644); err != nil {
		t.Fatalf("failed to create world file: %v", err)
	}

	expander := NewExpander(tmpDir, nil, nil)

	tests := []struct {
		name    string
		setName string
		wantErr bool
	}{
		{"@selected", "@selected", false},
		{"@system", "@system", false},
		{"@world", "@world", false},
		{"selected (no @)", "selected", false},
		{"world (no @)", "world", false},
		{"unknown", "@unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expander.ExpandSet(tt.setName)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				// Result can be empty for sets without packages
				if result == nil {
					t.Error("result should not be nil")
				}
			}
		})
	}
}

// TestHasSets tests the HasSets method.
func TestHasSets(t *testing.T) {
	tmpDir := t.TempDir()

	// Create minimal structure
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	if err := os.MkdirAll(varLib, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(varLib, "world"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to create world file: %v", err)
	}

	expander := NewExpander(tmpDir, nil, nil)

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"no sets", []string{"app-misc/hello", "dev-lang/go"}, false},
		{"has @world", []string{"app-misc/hello", "@world"}, true},
		{"has @selected", []string{"@selected"}, true},
		{"empty args", []string{}, false},
		{"only regular", []string{"sys-libs/zlib"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expander.HasSets(tt.args)
			if got != tt.want {
				t.Errorf("HasSets(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

// TestListSets tests the ListSets method.
func TestListSets(t *testing.T) {
	tmpDir := t.TempDir()

	// Create minimal structure
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	if err := os.MkdirAll(varLib, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(varLib, "world"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to create world file: %v", err)
	}

	expander := NewExpander(tmpDir, nil, nil)
	sets := expander.ListSets()

	// Should have built-in sets
	if len(sets) < 3 {
		t.Errorf("expected at least 3 sets, got %d", len(sets))
	}

	// Check required sets are present
	setMap := make(map[string]bool)
	for _, s := range sets {
		setMap[s] = true
	}

	required := []string{"@world", "@system", "@selected"}
	for _, name := range required {
		if !setMap[name] {
			t.Errorf("missing required set: %s", name)
		}
	}
}

// TestExpandWithSystemSet tests expansion with @system set.
func TestExpandWithSystemSet(t *testing.T) {
	tmpDir := t.TempDir()

	// Create world file
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	if err := os.MkdirAll(varLib, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(varLib, "world"), []byte("app-misc/hello\n"), 0644); err != nil {
		t.Fatalf("failed to create world file: %v", err)
	}

	// Create profile with system packages
	profileDir := filepath.Join(tmpDir, "profiles", "base")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("failed to create profile directory: %v", err)
	}
	profileContent := `*sys-libs/glibc
*sys-apps/baselayout
`
	if err := os.WriteFile(filepath.Join(profileDir, "packages"), []byte(profileContent), 0644); err != nil {
		t.Fatalf("failed to create packages file: %v", err)
	}

	// Note: Without loading the actual profile, @system will be empty
	// This tests the behavior without a loaded profile
	expander := NewExpander(tmpDir, nil, nil)

	// @system without profile should return empty (not error)
	result, err := expander.Expand([]string{"@system"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// @system is empty because no profile was loaded
	if result == nil {
		t.Error("result should not be nil")
	}
}

// TestExpandWithWorld tests @world expansion (selected + system).
func TestExpandWithWorld(t *testing.T) {
	tmpDir := t.TempDir()

	// Create world file with selected packages
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	if err := os.MkdirAll(varLib, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	worldContent := `dev-lang/go
dev-lang/rust
`
	if err := os.WriteFile(filepath.Join(varLib, "world"), []byte(worldContent), 0644); err != nil {
		t.Fatalf("failed to create world file: %v", err)
	}

	expander := NewExpander(tmpDir, nil, nil)

	// @world should include @selected (since @system is empty without profile)
	result, err := expander.Expand([]string{"@world"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should have at least the selected packages
	if len(result) < 2 {
		t.Errorf("expected at least 2 packages from @world, got %d: %v", len(result), result)
	}

	// Verify selected packages are included
	resultSet := make(map[string]bool)
	for _, r := range result {
		resultSet[r] = true
	}
	if !resultSet["dev-lang/go"] {
		t.Error("@world should contain dev-lang/go from @selected")
	}
	if !resultSet["dev-lang/rust"] {
		t.Error("@world should contain dev-lang/rust from @selected")
	}
}

// TestDeduplicateArgs tests the internal deduplication helper.
func TestDeduplicateArgs(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect []string
	}{
		{
			name:   "no duplicates",
			input:  []string{"a", "b", "c"},
			expect: []string{"a", "b", "c"},
		},
		{
			name:   "with duplicates",
			input:  []string{"a", "b", "a", "c", "b"},
			expect: []string{"a", "b", "c"},
		},
		{
			name:   "all same",
			input:  []string{"a", "a", "a"},
			expect: []string{"a"},
		},
		{
			name:   "empty",
			input:  []string{},
			expect: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicateArgs(tt.input)
			if len(result) != len(tt.expect) {
				t.Errorf("expected %d elements, got %d", len(tt.expect), len(result))
			}
			for i, v := range tt.expect {
				if i >= len(result) || result[i] != v {
					t.Errorf("at index %d: expected %s, got %v", i, v, result)
				}
			}
		})
	}
}

// TestDefaultExpanderConfig tests default configuration values.
func TestDefaultExpanderConfig(t *testing.T) {
	cfg := DefaultExpanderConfig()

	if cfg.RootDir != "/" {
		t.Errorf("expected RootDir '/', got '%s'", cfg.RootDir)
	}
	if cfg.PortageDir != "/var/lib/portage" {
		t.Errorf("expected PortageDir '/var/lib/portage', got '%s'", cfg.PortageDir)
	}
	if cfg.ProfilePath != "/etc/portage/make.profile" {
		t.Errorf("expected ProfilePath '/etc/portage/make.profile', got '%s'", cfg.ProfilePath)
	}
}

// TestGetRegistry tests access to the underlying registry.
func TestGetRegistry(t *testing.T) {
	tmpDir := t.TempDir()

	// Create minimal structure
	varLib := filepath.Join(tmpDir, "var", "lib", "portage")
	if err := os.MkdirAll(varLib, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(varLib, "world"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to create world file: %v", err)
	}

	expander := NewExpander(tmpDir, nil, nil)
	registry := expander.GetRegistry()

	if registry == nil {
		t.Fatal("GetRegistry() returned nil")
	}

	// Verify we can use the registry directly
	set, err := registry.GetSet("@selected")
	if err != nil {
		t.Errorf("could not get @selected from registry: %v", err)
	}
	if set == nil {
		t.Error("@selected set should not be nil")
	}
}
