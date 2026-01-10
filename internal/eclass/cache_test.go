package eclass

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	c := NewCache()
	if c == nil {
		t.Fatal("NewCache returned nil")
	}

	if len(c.eclasses) != 0 {
		t.Errorf("expected empty eclasses, got %d", len(c.eclasses))
	}

	if len(c.locations) != 0 {
		t.Errorf("expected empty locations, got %d", len(c.locations))
	}
}

func TestCacheScanDirectory(t *testing.T) {
	// Create a temporary eclass directory
	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatalf("failed to create eclass dir: %v", err)
	}

	// Create test eclass files
	eclasses := []string{"test", "foo", "bar"}
	for _, name := range eclasses {
		content := "# Copyright 2025\n# " + name + ".eclass\n"
		path := filepath.Join(eclassDir, name+".eclass")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create eclass %s: %v", name, err)
		}
	}

	// Create a non-eclass file (should be ignored)
	if err := os.WriteFile(filepath.Join(eclassDir, "README.txt"), []byte("readme"), 0644); err != nil {
		t.Fatalf("failed to create README: %v", err)
	}

	// Create cache and add location
	c, err := NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatalf("NewCacheWithLocations failed: %v", err)
	}

	// Verify all eclasses were found
	for _, name := range eclasses {
		if !c.Has(name) {
			t.Errorf("eclass %s not found in cache", name)
		}

		eclass, err := c.Get(name)
		if err != nil {
			t.Errorf("Get(%s) failed: %v", name, err)
			continue
		}

		if eclass.Name != name {
			t.Errorf("expected name %s, got %s", name, eclass.Name)
		}

		expectedPath := filepath.Join(eclassDir, name+".eclass")
		if eclass.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, eclass.Path)
		}

		if eclass.EclassDir != eclassDir {
			t.Errorf("expected eclassDir %s, got %s", eclassDir, eclass.EclassDir)
		}
	}

	// Verify list returns all eclasses
	list := c.List()
	if len(list) != len(eclasses) {
		t.Errorf("expected %d eclasses in list, got %d", len(eclasses), len(list))
	}
}

func TestCacheGetNotFound(t *testing.T) {
	c := NewCache()

	_, err := c.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent eclass")
	}

	if !IsEclassNotFound(err) {
		t.Errorf("expected EclassNotFoundError, got %T", err)
	}
}

func TestCacheOverlayPriority(t *testing.T) {
	// Create two eclass directories simulating master + overlay
	tmpDir := t.TempDir()

	masterDir := filepath.Join(tmpDir, "gentoo", "eclass")
	overlayDir := filepath.Join(tmpDir, "overlay", "eclass")

	if err := os.MkdirAll(masterDir, 0755); err != nil {
		t.Fatalf("failed to create master dir: %v", err)
	}
	if err := os.MkdirAll(overlayDir, 0755); err != nil {
		t.Fatalf("failed to create overlay dir: %v", err)
	}

	// Create test.eclass in both directories
	masterContent := "# master version\nECLASS_VERSION=master"
	overlayContent := "# overlay version\nECLASS_VERSION=overlay"

	// Write master first
	masterPath := filepath.Join(masterDir, "test.eclass")
	if err := os.WriteFile(masterPath, []byte(masterContent), 0644); err != nil {
		t.Fatalf("failed to create master eclass: %v", err)
	}

	// Wait a bit and write overlay with different mtime
	time.Sleep(10 * time.Millisecond)
	overlayPath := filepath.Join(overlayDir, "test.eclass")
	if err := os.WriteFile(overlayPath, []byte(overlayContent), 0644); err != nil {
		t.Fatalf("failed to create overlay eclass: %v", err)
	}

	// Create cache with master first, then overlay (higher priority)
	c, err := NewCacheWithLocations([]string{masterDir, overlayDir})
	if err != nil {
		t.Fatalf("NewCacheWithLocations failed: %v", err)
	}

	// Overlay should win
	eclass, err := c.Get("test")
	if err != nil {
		t.Fatalf("Get(test) failed: %v", err)
	}

	if eclass.Path != overlayPath {
		t.Errorf("expected overlay path %s, got %s", overlayPath, eclass.Path)
	}
}

func TestCacheInvalidate(t *testing.T) {
	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatalf("failed to create eclass dir: %v", err)
	}

	path := filepath.Join(eclassDir, "test.eclass")
	if err := os.WriteFile(path, []byte("# test"), 0644); err != nil {
		t.Fatalf("failed to create eclass: %v", err)
	}

	c, err := NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatalf("NewCacheWithLocations failed: %v", err)
	}

	// Verify eclass exists
	if !c.Has("test") {
		t.Error("expected test eclass to exist")
	}

	// Invalidate
	c.Invalidate("test")

	// Verify it's gone
	if c.Has("test") {
		t.Error("expected test eclass to be invalidated")
	}
}

func TestCacheRefresh(t *testing.T) {
	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatalf("failed to create eclass dir: %v", err)
	}

	// Create initial eclass
	path := filepath.Join(eclassDir, "test.eclass")
	if err := os.WriteFile(path, []byte("# test v1"), 0644); err != nil {
		t.Fatalf("failed to create eclass: %v", err)
	}

	c, err := NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatalf("NewCacheWithLocations failed: %v", err)
	}

	initialList := c.List()
	if len(initialList) != 1 {
		t.Errorf("expected 1 eclass, got %d", len(initialList))
	}

	// Add a new eclass
	path2 := filepath.Join(eclassDir, "new.eclass")
	if err := os.WriteFile(path2, []byte("# new"), 0644); err != nil {
		t.Fatalf("failed to create new eclass: %v", err)
	}

	// Refresh
	if err := c.Refresh(); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Verify new eclass is found
	refreshedList := c.List()
	if len(refreshedList) != 2 {
		t.Errorf("expected 2 eclasses after refresh, got %d", len(refreshedList))
	}

	if !c.Has("new") {
		t.Error("expected 'new' eclass after refresh")
	}
}

func TestCacheGetWithChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatalf("failed to create eclass dir: %v", err)
	}

	content := "# test eclass content"
	path := filepath.Join(eclassDir, "test.eclass")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create eclass: %v", err)
	}

	c, err := NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatalf("NewCacheWithLocations failed: %v", err)
	}

	// First Get - checksum not computed
	eclass, err := c.Get("test")
	if err != nil {
		t.Fatalf("Get(test) failed: %v", err)
	}
	if eclass.Checksum != "" {
		t.Errorf("expected empty checksum on Get, got %s", eclass.Checksum)
	}

	// GetWithChecksum - checksum computed
	eclassWithSum, err := c.GetWithChecksum("test")
	if err != nil {
		t.Fatalf("GetWithChecksum(test) failed: %v", err)
	}
	if eclassWithSum.Checksum == "" {
		t.Error("expected checksum to be computed")
	}

	// Subsequent Get should return cached checksum
	eclassAgain, err := c.Get("test")
	if err != nil {
		t.Fatalf("Get(test) after checksum failed: %v", err)
	}
	if eclassAgain.Checksum != eclassWithSum.Checksum {
		t.Errorf("checksum mismatch: %s vs %s", eclassAgain.Checksum, eclassWithSum.Checksum)
	}
}

func TestCacheValidateEclass(t *testing.T) {
	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatalf("failed to create eclass dir: %v", err)
	}

	path := filepath.Join(eclassDir, "test.eclass")
	if err := os.WriteFile(path, []byte("# test"), 0644); err != nil {
		t.Fatalf("failed to create eclass: %v", err)
	}

	c, err := NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatalf("NewCacheWithLocations failed: %v", err)
	}

	// Should be valid initially
	valid, err := c.ValidateEclass("test")
	if err != nil {
		t.Fatalf("ValidateEclass failed: %v", err)
	}
	if !valid {
		t.Error("expected eclass to be valid")
	}

	// Modify file
	time.Sleep(10 * time.Millisecond) // Ensure mtime changes
	if err := os.WriteFile(path, []byte("# modified"), 0644); err != nil {
		t.Fatalf("failed to modify eclass: %v", err)
	}

	// Should now be invalid
	valid, err = c.ValidateEclass("test")
	if err != nil {
		t.Fatalf("ValidateEclass after modification failed: %v", err)
	}
	if valid {
		t.Error("expected eclass to be invalid after modification")
	}
}

func TestCacheLocationsString(t *testing.T) {
	tmpDir := t.TempDir()

	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir2")
	if err := os.MkdirAll(dir1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatal(err)
	}

	c, err := NewCacheWithLocations([]string{dir1, dir2})
	if err != nil {
		t.Fatalf("NewCacheWithLocations failed: %v", err)
	}

	locStr := c.LocationsString()

	// Highest priority should come first in the string
	// dir2 was added last, so it should be first in the output
	if !contains(locStr, dir1) || !contains(locStr, dir2) {
		t.Errorf("LocationsString missing dirs: %s", locStr)
	}
}

func TestCacheAddRepoWithMasters(t *testing.T) {
	tmpDir := t.TempDir()

	// Create gentoo (master) repo structure
	gentooRoot := filepath.Join(tmpDir, "gentoo")
	gentooEclass := filepath.Join(gentooRoot, "eclass")
	if err := os.MkdirAll(gentooEclass, 0755); err != nil {
		t.Fatal(err)
	}

	// Create overlay repo structure
	overlayRoot := filepath.Join(tmpDir, "overlay")
	overlayEclass := filepath.Join(overlayRoot, "eclass")
	if err := os.MkdirAll(overlayEclass, 0755); err != nil {
		t.Fatal(err)
	}

	// Create master eclass
	masterPath := filepath.Join(gentooEclass, "base.eclass")
	if err := os.WriteFile(masterPath, []byte("# base from gentoo"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create overlay-specific eclass
	overlayOnlyPath := filepath.Join(overlayEclass, "custom.eclass")
	if err := os.WriteFile(overlayOnlyPath, []byte("# custom from overlay"), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewCache()
	err := c.AddRepoWithMasters("overlay", overlayRoot, []string{gentooRoot})
	if err != nil {
		t.Fatalf("AddRepoWithMasters failed: %v", err)
	}

	// Should have both eclasses
	if !c.Has("base") {
		t.Error("expected 'base' eclass from master")
	}
	if !c.Has("custom") {
		t.Error("expected 'custom' eclass from overlay")
	}

	// Verify base comes from gentoo
	base, err := c.Get("base")
	if err != nil {
		t.Fatalf("Get(base) failed: %v", err)
	}
	if base.Path != masterPath {
		t.Errorf("expected base path %s, got %s", masterPath, base.Path)
	}

	// Verify custom comes from overlay
	custom, err := c.Get("custom")
	if err != nil {
		t.Fatalf("Get(custom) failed: %v", err)
	}
	if custom.Path != overlayOnlyPath {
		t.Errorf("expected custom path %s, got %s", overlayOnlyPath, custom.Path)
	}
}

func TestCacheGetEclassData(t *testing.T) {
	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test eclasses
	for _, name := range []string{"foo", "bar", "baz"} {
		path := filepath.Join(eclassDir, name+".eclass")
		if err := os.WriteFile(path, []byte("# "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	c, err := NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatal(err)
	}

	// Get subset
	data, err := c.GetEclassData([]string{"foo", "bar"})
	if err != nil {
		t.Fatalf("GetEclassData failed: %v", err)
	}

	if len(data) != 2 {
		t.Errorf("expected 2 eclasses, got %d", len(data))
	}

	if _, ok := data["foo"]; !ok {
		t.Error("expected 'foo' in data")
	}
	if _, ok := data["bar"]; !ok {
		t.Error("expected 'bar' in data")
	}

	// Request missing eclass
	_, err = c.GetEclassData([]string{"foo", "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent eclass")
	}
}

func TestEclassID(t *testing.T) {
	e := &Eclass{
		Name:     "test",
		RepoName: "gentoo",
	}

	expected := "test@gentoo"
	if e.ID() != expected {
		t.Errorf("expected ID %s, got %s", expected, e.ID())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
