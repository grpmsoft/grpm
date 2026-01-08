package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewConfigProtect(t *testing.T) {
	cp := NewConfigProtect()

	if cp == nil {
		t.Fatal("NewConfigProtect returned nil")
	}

	// Check default protected paths
	if len(cp.Protected) != 2 {
		t.Errorf("expected 2 protected paths, got %d", len(cp.Protected))
	}

	if cp.Protected[0] != "/etc" {
		t.Errorf("expected /etc as first protected path, got %s", cp.Protected[0])
	}

	if cp.Protected[1] != "/usr/share/config" {
		t.Errorf("expected /usr/share/config as second protected path, got %s", cp.Protected[1])
	}

	// Check default masked paths
	if len(cp.Masked) != 2 {
		t.Errorf("expected 2 masked paths, got %d", len(cp.Masked))
	}

	if cp.Masked[0] != "/etc/env.d" {
		t.Errorf("expected /etc/env.d as first masked path, got %s", cp.Masked[0])
	}
}

func TestConfigProtect_IsProtected(t *testing.T) {
	cp := NewConfigProtect()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"etc root", "/etc", true},
		{"etc subdir", "/etc/conf.d", true},
		{"etc file", "/etc/foo.conf", true},
		{"etc deep file", "/etc/conf.d/hostname", true},
		{"usr share config", "/usr/share/config", true},
		{"usr share config file", "/usr/share/config/foo", true},
		{"usr bin", "/usr/bin/foo", false},
		{"home", "/home/user/.config", false},
		{"root", "/", false},
		{"var", "/var/lib", false},
		{"etc prefix no slash", "/etc-backup", false},
		{"etcfoo", "/etcfoo", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cp.IsProtected(tt.path)
			if result != tt.expected {
				t.Errorf("IsProtected(%s) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestConfigProtect_IsMasked(t *testing.T) {
	cp := NewConfigProtect()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"env.d root", "/etc/env.d", true},
		{"env.d file", "/etc/env.d/99local", true},
		{"gconf root", "/etc/gconf", true},
		{"gconf file", "/etc/gconf/schemas", true},
		{"etc root", "/etc", false},
		{"etc file", "/etc/foo.conf", false},
		{"etc conf.d", "/etc/conf.d/hostname", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cp.IsMasked(tt.path)
			if result != tt.expected {
				t.Errorf("IsMasked(%s) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestConfigProtect_ShouldProtect(t *testing.T) {
	cp := NewConfigProtect()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"etc file", "/etc/foo.conf", true},
		{"etc conf.d", "/etc/conf.d/hostname", true},
		{"env.d file", "/etc/env.d/99local", false},  // masked
		{"gconf file", "/etc/gconf/schemas", false},  // masked
		{"usr bin", "/usr/bin/foo", false},           // not protected
		{"home config", "/home/user/.config", false}, // not protected
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cp.ShouldProtect(tt.path)
			if result != tt.expected {
				t.Errorf("ShouldProtect(%s) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestConfigProtect_GenerateProtectedName(t *testing.T) {
	// Create temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "grpm-protect-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cp := NewConfigProtect()

	// Test first generation (should be ._cfg0000_)
	path := filepath.Join(tmpDir, "foo.conf")
	expected := filepath.Join(tmpDir, "._cfg0000_foo.conf")

	result := cp.GenerateProtectedName(path)
	if result != expected {
		t.Errorf("first generation: got %s, expected %s", result, expected)
	}

	// Create the first protected file
	if err := os.WriteFile(result, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test second generation (should be ._cfg0001_)
	expected = filepath.Join(tmpDir, "._cfg0001_foo.conf")
	result = cp.GenerateProtectedName(path)
	if result != expected {
		t.Errorf("second generation: got %s, expected %s", result, expected)
	}

	// Create second protected file
	if err := os.WriteFile(result, []byte("test2"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test third generation (should be ._cfg0002_)
	expected = filepath.Join(tmpDir, "._cfg0002_foo.conf")
	result = cp.GenerateProtectedName(path)
	if result != expected {
		t.Errorf("third generation: got %s, expected %s", result, expected)
	}
}

func TestConfigProtect_GenerateProtectedName_Concurrent(t *testing.T) {
	// Create temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "grpm-protect-concurrent")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cp := NewConfigProtect()
	path := filepath.Join(tmpDir, "test.conf")

	// Generate multiple names concurrently
	done := make(chan string, 10)
	for i := 0; i < 10; i++ {
		go func() {
			name := cp.GenerateProtectedName(path)
			// Immediately create the file to simulate real usage
			if name != "" {
				_ = os.WriteFile(name, []byte("test"), 0644)
			}
			done <- name
		}()
	}

	// Collect results
	names := make(map[string]bool)
	for i := 0; i < 10; i++ {
		name := <-done
		if name == "" {
			continue
		}
		if names[name] {
			t.Errorf("duplicate protected name generated: %s", name)
		}
		names[name] = true
	}
}

func TestConfigProtect_FindExistingConfigs(t *testing.T) {
	// Create temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "grpm-protect-find")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create test files
	testFiles := []string{
		"._cfg0000_foo.conf",
		"._cfg0001_foo.conf",
		"._cfg0002_foo.conf",
		"._cfg0000_bar.conf", // Different base name
		"foo.conf",           // Original file
		"other.txt",          // Unrelated file
	}

	for _, f := range testFiles {
		if err := os.WriteFile(filepath.Join(tmpDir, f), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cp := NewConfigProtect()

	// Test finding configs for foo.conf
	configs, err := cp.FindExistingConfigs(filepath.Join(tmpDir, "foo.conf"))
	if err != nil {
		t.Fatalf("FindExistingConfigs failed: %v", err)
	}

	if len(configs) != 3 {
		t.Errorf("expected 3 configs, got %d: %v", len(configs), configs)
	}

	// Verify order (should be sorted)
	expected := []string{
		filepath.Join(tmpDir, "._cfg0000_foo.conf"),
		filepath.Join(tmpDir, "._cfg0001_foo.conf"),
		filepath.Join(tmpDir, "._cfg0002_foo.conf"),
	}

	for i, exp := range expected {
		if i >= len(configs) {
			break
		}
		if configs[i] != exp {
			t.Errorf("config[%d] = %s, expected %s", i, configs[i], exp)
		}
	}

	// Test finding configs for bar.conf
	configs, err = cp.FindExistingConfigs(filepath.Join(tmpDir, "bar.conf"))
	if err != nil {
		t.Fatalf("FindExistingConfigs for bar.conf failed: %v", err)
	}

	if len(configs) != 1 {
		t.Errorf("expected 1 config for bar.conf, got %d", len(configs))
	}

	// Test finding configs for non-existent file
	configs, err = cp.FindExistingConfigs(filepath.Join(tmpDir, "nonexistent.conf"))
	if err != nil {
		t.Fatalf("FindExistingConfigs for nonexistent failed: %v", err)
	}

	if len(configs) != 0 {
		t.Errorf("expected 0 configs for nonexistent, got %d", len(configs))
	}
}

func TestConfigProtect_FindExistingConfigs_NonExistentDir(t *testing.T) {
	cp := NewConfigProtect()

	configs, err := cp.FindExistingConfigs("/nonexistent/path/foo.conf")
	if err != nil {
		t.Fatalf("FindExistingConfigs should not error for non-existent dir: %v", err)
	}

	if len(configs) != 0 {
		t.Errorf("expected empty slice for non-existent dir, got %d", len(configs))
	}
}

func TestCompareFiles(t *testing.T) {
	// Create temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "grpm-compare")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	file3 := filepath.Join(tmpDir, "file3.txt")

	if err := os.WriteFile(file1, []byte("same content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("same content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file3, []byte("different content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test identical files
	equal, err := CompareFiles(file1, file2)
	if err != nil {
		t.Fatalf("CompareFiles failed: %v", err)
	}
	if !equal {
		t.Error("expected files with same content to be equal")
	}

	// Test different files
	equal, err = CompareFiles(file1, file3)
	if err != nil {
		t.Fatalf("CompareFiles failed: %v", err)
	}
	if equal {
		t.Error("expected files with different content to be unequal")
	}

	// Test with non-existent file
	_, err = CompareFiles(file1, filepath.Join(tmpDir, "nonexistent.txt"))
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestConfigProtect_AddProtected(t *testing.T) {
	cp := NewConfigProtect()
	initialCount := len(cp.Protected)

	cp.AddProtected("/custom/path")

	if len(cp.Protected) != initialCount+1 {
		t.Errorf("expected %d protected paths, got %d", initialCount+1, len(cp.Protected))
	}

	// Verify the path works for protection check
	if !cp.IsProtected("/custom/path/file") {
		t.Error("expected /custom/path/file to be protected after AddProtected")
	}

	// Test path cleaning (trailing slash should be removed)
	cp.AddProtected("/another/path/")
	if !cp.IsProtected("/another/path/subdir") {
		t.Error("expected /another/path/subdir to be protected after AddProtected with trailing slash")
	}
}

func TestConfigProtect_AddMasked(t *testing.T) {
	cp := NewConfigProtect()
	initialCount := len(cp.Masked)

	cp.AddMasked("/custom/masked")

	if len(cp.Masked) != initialCount+1 {
		t.Errorf("expected %d masked paths, got %d", initialCount+1, len(cp.Masked))
	}

	// Verify the new path works
	if !cp.IsMasked("/custom/masked/file") {
		t.Error("expected /custom/masked/file to be masked")
	}
}

func TestConfigProtect_SetProtected(t *testing.T) {
	cp := NewConfigProtect()

	newPaths := []string{"/path1", "/path2/", "/path3"}
	cp.SetProtected(newPaths)

	if len(cp.Protected) != 3 {
		t.Errorf("expected 3 protected paths, got %d", len(cp.Protected))
	}

	// Verify path with trailing slash works for protection check
	if !cp.IsProtected("/path2/subdir") {
		t.Error("/path2/subdir should be protected after SetProtected with trailing slash")
	}

	// Verify old paths are replaced
	if cp.IsProtected("/etc/foo") {
		t.Error("/etc should no longer be protected")
	}

	if !cp.IsProtected("/path1/file") {
		t.Error("/path1/file should be protected")
	}
}

func TestConfigProtect_SetMasked(t *testing.T) {
	cp := NewConfigProtect()

	newPaths := []string{"/masked1", "/masked2"}
	cp.SetMasked(newPaths)

	if len(cp.Masked) != 2 {
		t.Errorf("expected 2 masked paths, got %d", len(cp.Masked))
	}

	// Verify old paths are replaced
	if cp.IsMasked("/etc/env.d/file") {
		t.Error("/etc/env.d should no longer be masked")
	}

	if !cp.IsMasked("/masked1/file") {
		t.Error("/masked1/file should be masked")
	}
}

func TestConfigProtect_GetCounts(t *testing.T) {
	cp := NewConfigProtect()

	if cp.GetProtectedCount() != 2 {
		t.Errorf("expected 2 protected paths, got %d", cp.GetProtectedCount())
	}

	if cp.GetMaskedCount() != 2 {
		t.Errorf("expected 2 masked paths, got %d", cp.GetMaskedCount())
	}

	cp.AddProtected("/new/path")
	if cp.GetProtectedCount() != 3 {
		t.Errorf("expected 3 protected paths after add, got %d", cp.GetProtectedCount())
	}
}

func TestIsDigits(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"0000", true},
		{"1234", true},
		{"9999", true},
		{"000a", false},
		{"abcd", false},
		{"12 34", false},
		{"", true}, // empty string has no non-digits
		{"1", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isDigits(tt.input)
			if result != tt.expected {
				t.Errorf("isDigits(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConfigProtect_PathNormalization(t *testing.T) {
	cp := NewConfigProtect()

	// Test with various path formats
	tests := []struct {
		path     string
		expected bool
	}{
		{"/etc/foo.conf", true},
		{"/etc//foo.conf", true},     // double slash
		{"/etc/./foo.conf", true},    // current dir
		{"/etc/../etc/foo", true},    // parent dir
		{"/usr/share/config/", true}, // trailing slash
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := cp.IsProtected(tt.path)
			if result != tt.expected {
				t.Errorf("IsProtected(%s) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}
