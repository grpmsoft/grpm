package mask

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/grpmsoft/grpm/internal/config"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// TestMaskManager_IsMasked_Simple tests basic mask matching.
func TestMaskManager_IsMasked_Simple(t *testing.T) {
	// Create temporary test directory
	tmpDir := t.TempDir()

	// Create package.mask with simple entries
	maskDir := filepath.Join(tmpDir, "package.mask")
	if err := os.MkdirAll(maskDir, 0o755); err != nil {
		t.Fatalf("Failed to create mask dir: %v", err)
	}

	maskContent := `# Test package masks
>=sys-devel/gcc-16.0
=dev-lang/python-3.13.0
~app-misc/hello-2.99
`
	if err := os.WriteFile(filepath.Join(maskDir, "testing"), []byte(maskContent), 0o644); err != nil {
		t.Fatalf("Failed to write mask file: %v", err)
	}

	// Create config and mask manager
	cfg, err := config.LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	manager, err := NewMaskManager(cfg, "", "")
	if err != nil {
		t.Fatalf("Failed to create MaskManager: %v", err)
	}

	tests := []struct {
		name     string
		category string
		pkg      string
		version  string
		slot     string
		want     bool
	}{
		{
			name:     "gcc-16.0.9999 should be masked",
			category: "sys-devel",
			pkg:      "gcc",
			version:  "16.0.9999",
			slot:     "16",
			want:     true,
		},
		{
			name:     "gcc-15.0 should NOT be masked",
			category: "sys-devel",
			pkg:      "gcc",
			version:  "15.0",
			slot:     "15",
			want:     false,
		},
		{
			name:     "gcc-16.0 exact should be masked",
			category: "sys-devel",
			pkg:      "gcc",
			version:  "16.0",
			slot:     "16",
			want:     true,
		},
		{
			name:     "python-3.13.0 exact match should be masked",
			category: "dev-lang",
			pkg:      "python",
			version:  "3.13.0",
			slot:     "3.13",
			want:     true,
		},
		{
			name:     "python-3.12.0 should NOT be masked",
			category: "dev-lang",
			pkg:      "python",
			version:  "3.12.0",
			slot:     "3.12",
			want:     false,
		},
		{
			name:     "hello-2.99 should be masked by revision match",
			category: "app-misc",
			pkg:      "hello",
			version:  "2.99",
			slot:     "0",
			want:     true,
		},
		{
			name:     "hello-2.99-r1 should be masked by revision match",
			category: "app-misc",
			pkg:      "hello",
			version:  "2.99-r1",
			slot:     "0",
			want:     true,
		},
		{
			name:     "hello-2.10 should NOT be masked",
			category: "app-misc",
			pkg:      "hello",
			version:  "2.10",
			slot:     "0",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manager.IsMasked(tt.category, tt.pkg, tt.version, tt.slot)
			if got != tt.want {
				t.Errorf("IsMasked(%s/%s-%s) = %v, want %v",
					tt.category, tt.pkg, tt.version, got, tt.want)
			}
		})
	}
}

// TestMaskManager_UserUnmask tests that user unmasks override masks.
func TestMaskManager_UserUnmask(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package.mask
	maskContent := `>=sys-devel/gcc-16.0
=dev-lang/python-3.13.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.mask"), []byte(maskContent), 0o644); err != nil {
		t.Fatalf("Failed to write mask file: %v", err)
	}

	// Create package.unmask to override one mask
	unmaskContent := `=sys-devel/gcc-16.0.9999
`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.unmask"), []byte(unmaskContent), 0o644); err != nil {
		t.Fatalf("Failed to write unmask file: %v", err)
	}

	cfg, err := config.LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	manager, err := NewMaskManager(cfg, "", "")
	if err != nil {
		t.Fatalf("Failed to create MaskManager: %v", err)
	}

	tests := []struct {
		name     string
		category string
		pkg      string
		version  string
		slot     string
		want     bool
	}{
		{
			name:     "gcc-16.0.9999 should be unmasked by user unmask",
			category: "sys-devel",
			pkg:      "gcc",
			version:  "16.0.9999",
			slot:     "16",
			want:     false, // Unmasked!
		},
		{
			name:     "gcc-16.1 should still be masked (not unmasked)",
			category: "sys-devel",
			pkg:      "gcc",
			version:  "16.1",
			slot:     "16",
			want:     true,
		},
		{
			name:     "python-3.13.0 should still be masked",
			category: "dev-lang",
			pkg:      "python",
			version:  "3.13.0",
			slot:     "3.13",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manager.IsMasked(tt.category, tt.pkg, tt.version, tt.slot)
			if got != tt.want {
				t.Errorf("IsMasked(%s/%s-%s) = %v, want %v",
					tt.category, tt.pkg, tt.version, got, tt.want)
			}
		})
	}
}

// TestMaskManager_NegationInMask tests negation entries in package.mask.
func TestMaskManager_NegationInMask(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package.mask with negation
	maskContent := `# First mask all gcc 16
>=sys-devel/gcc-16.0
# Then unmask a specific version with negation
-=sys-devel/gcc-16.0.1
`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.mask"), []byte(maskContent), 0o644); err != nil {
		t.Fatalf("Failed to write mask file: %v", err)
	}

	cfg, err := config.LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	manager, err := NewMaskManager(cfg, "", "")
	if err != nil {
		t.Fatalf("Failed to create MaskManager: %v", err)
	}

	// gcc-16.0.1 should NOT be masked (negated)
	if manager.IsMasked("sys-devel", "gcc", "16.0.1", "16") {
		t.Error("gcc-16.0.1 should NOT be masked due to negation")
	}

	// gcc-16.0.2 should still be masked
	if !manager.IsMasked("sys-devel", "gcc", "16.0.2", "16") {
		t.Error("gcc-16.0.2 SHOULD be masked")
	}
}

// TestMaskManager_GetMaskAtom tests retrieving the matching mask atom.
func TestMaskManager_GetMaskAtom(t *testing.T) {
	tmpDir := t.TempDir()

	maskContent := `>=sys-devel/gcc-16.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.mask"), []byte(maskContent), 0o644); err != nil {
		t.Fatalf("Failed to write mask file: %v", err)
	}

	cfg, err := config.LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	manager, err := NewMaskManager(cfg, "", "")
	if err != nil {
		t.Fatalf("Failed to create MaskManager: %v", err)
	}

	// Get mask atom for masked package
	atom := manager.GetMaskAtom("sys-devel", "gcc", "16.0.9999", "16")
	if atom == nil {
		t.Fatal("Expected mask atom, got nil")
	}
	if atom.Raw != ">=sys-devel/gcc-16.0" {
		t.Errorf("Expected atom '>=sys-devel/gcc-16.0', got '%s'", atom.Raw)
	}

	// No mask atom for unmasked package
	atom = manager.GetMaskAtom("sys-devel", "gcc", "15.0", "15")
	if atom != nil {
		t.Errorf("Expected nil atom for unmasked package, got %v", atom)
	}
}

// TestMaskManager_GetMaskReason tests retrieving mask reason and source.
func TestMaskManager_GetMaskReason(t *testing.T) {
	tmpDir := t.TempDir()

	maskContent := `>=sys-devel/gcc-16.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.mask"), []byte(maskContent), 0o644); err != nil {
		t.Fatalf("Failed to write mask file: %v", err)
	}

	cfg, err := config.LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	manager, err := NewMaskManager(cfg, "", "")
	if err != nil {
		t.Fatalf("Failed to create MaskManager: %v", err)
	}

	atom, source := manager.GetMaskReason("sys-devel", "gcc", "16.0.9999", "16")
	if atom == "" {
		t.Fatal("Expected mask atom, got empty string")
	}
	if source != "user" {
		t.Errorf("Expected source 'user', got '%s'", source)
	}
}

// TestMaskManager_IsPackageMasked tests the Package struct wrapper.
func TestMaskManager_IsPackageMasked(t *testing.T) {
	tmpDir := t.TempDir()

	maskContent := `>=sys-devel/gcc-16.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.mask"), []byte(maskContent), 0o644); err != nil {
		t.Fatalf("Failed to write mask file: %v", err)
	}

	cfg, err := config.LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	manager, err := NewMaskManager(cfg, "", "")
	if err != nil {
		t.Fatalf("Failed to create MaskManager: %v", err)
	}

	// Test with Package struct
	maskedPkg := &pkg.Package{
		Name:    "sys-devel/gcc",
		Version: "16.0.9999",
		Slot:    pkg.Slot{Name: "16"},
	}
	if !manager.IsPackageMasked(maskedPkg) {
		t.Error("Expected package to be masked")
	}

	unmaskedPkg := &pkg.Package{
		Name:    "sys-devel/gcc",
		Version: "15.0",
		Slot:    pkg.Slot{Name: "15"},
	}
	if manager.IsPackageMasked(unmaskedPkg) {
		t.Error("Expected package to NOT be masked")
	}

	// Test nil package
	if manager.IsPackageMasked(nil) {
		t.Error("Expected nil package to return false")
	}
}

// TestMaskManager_GetStats tests statistics retrieval.
func TestMaskManager_GetStats(t *testing.T) {
	tmpDir := t.TempDir()

	maskContent := `>=sys-devel/gcc-16.0
=dev-lang/python-3.13.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.mask"), []byte(maskContent), 0o644); err != nil {
		t.Fatalf("Failed to write mask file: %v", err)
	}

	unmaskContent := `=sys-devel/gcc-16.0.9999
`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.unmask"), []byte(unmaskContent), 0o644); err != nil {
		t.Fatalf("Failed to write unmask file: %v", err)
	}

	cfg, err := config.LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	manager, err := NewMaskManager(cfg, "", "")
	if err != nil {
		t.Fatalf("Failed to create MaskManager: %v", err)
	}

	stats := manager.GetStats()
	if stats.UserMaskCount != 2 {
		t.Errorf("Expected 2 user masks, got %d", stats.UserMaskCount)
	}
	if stats.UserUnmaskCount != 1 {
		t.Errorf("Expected 1 user unmask, got %d", stats.UserUnmaskCount)
	}
	if stats.TotalMaskedCP != 2 {
		t.Errorf("Expected 2 masked CP, got %d", stats.TotalMaskedCP)
	}
}

// TestMaskManager_GetAllMaskedPackages tests listing all masked packages.
func TestMaskManager_GetAllMaskedPackages(t *testing.T) {
	tmpDir := t.TempDir()

	maskContent := `>=sys-devel/gcc-16.0
=dev-lang/python-3.13.0
=app-misc/hello-2.99
`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.mask"), []byte(maskContent), 0o644); err != nil {
		t.Fatalf("Failed to write mask file: %v", err)
	}

	cfg, err := config.LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	manager, err := NewMaskManager(cfg, "", "")
	if err != nil {
		t.Fatalf("Failed to create MaskManager: %v", err)
	}

	packages := manager.GetAllMaskedPackages()
	if len(packages) != 3 {
		t.Errorf("Expected 3 masked packages, got %d: %v", len(packages), packages)
	}

	// Verify sorted order
	expected := []string{"app-misc/hello", "dev-lang/python", "sys-devel/gcc"}
	for i, exp := range expected {
		if i >= len(packages) || packages[i] != exp {
			t.Errorf("Expected packages[%d] = %s, got %s", i, exp, packages[i])
		}
	}
}

// TestMaskManager_EmptyConfig tests behavior with no masks.
func TestMaskManager_EmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()

	cfg, err := config.LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	manager, err := NewMaskManager(cfg, "", "")
	if err != nil {
		t.Fatalf("Failed to create MaskManager: %v", err)
	}

	// Nothing should be masked
	if manager.IsMasked("sys-devel", "gcc", "16.0.9999", "16") {
		t.Error("Expected no packages to be masked with empty config")
	}

	stats := manager.GetStats()
	if stats.UserMaskCount != 0 {
		t.Errorf("Expected 0 user masks, got %d", stats.UserMaskCount)
	}
}

// TestMaskManager_VersionOperators tests various version operators in masks.
func TestMaskManager_VersionOperators(t *testing.T) {
	tmpDir := t.TempDir()

	maskContent := `# Greater than or equal
>=sys-devel/gcc-16.0
# Less than
<dev-lang/python-3.10
# Exact match
=app-misc/hello-2.10
# Greater than (exclusive)
>net-misc/curl-8.0
# Less than or equal
<=sys-libs/glibc-2.37
`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.mask"), []byte(maskContent), 0o644); err != nil {
		t.Fatalf("Failed to write mask file: %v", err)
	}

	cfg, err := config.LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	manager, err := NewMaskManager(cfg, "", "")
	if err != nil {
		t.Fatalf("Failed to create MaskManager: %v", err)
	}

	tests := []struct {
		name     string
		category string
		pkg      string
		version  string
		want     bool
	}{
		// >=sys-devel/gcc-16.0
		{"gcc-16.0 masked by >=", "sys-devel", "gcc", "16.0", true},
		{"gcc-16.1 masked by >=", "sys-devel", "gcc", "16.1", true},
		{"gcc-15.0 not masked", "sys-devel", "gcc", "15.0", false},

		// <dev-lang/python-3.10
		{"python-3.9 masked by <", "dev-lang", "python", "3.9", true},
		{"python-3.10 not masked (boundary)", "dev-lang", "python", "3.10", false},
		{"python-3.11 not masked", "dev-lang", "python", "3.11", false},

		// =app-misc/hello-2.10
		{"hello-2.10 masked by =", "app-misc", "hello", "2.10", true},
		{"hello-2.11 not masked", "app-misc", "hello", "2.11", false},
		{"hello-2.9 not masked", "app-misc", "hello", "2.9", false},

		// >net-misc/curl-8.0
		{"curl-8.1 masked by >", "net-misc", "curl", "8.1", true},
		{"curl-8.0 not masked (boundary)", "net-misc", "curl", "8.0", false},
		{"curl-7.0 not masked", "net-misc", "curl", "7.0", false},

		// <=sys-libs/glibc-2.37
		{"glibc-2.37 masked by <=", "sys-libs", "glibc", "2.37", true},
		{"glibc-2.36 masked by <=", "sys-libs", "glibc", "2.36", true},
		{"glibc-2.38 not masked", "sys-libs", "glibc", "2.38", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manager.IsMasked(tt.category, tt.pkg, tt.version, "0")
			if got != tt.want {
				t.Errorf("IsMasked(%s/%s-%s) = %v, want %v",
					tt.category, tt.pkg, tt.version, got, tt.want)
			}
		})
	}
}

// TestMaskManager_SlotMask tests slot-specific masks.
func TestMaskManager_SlotMask(t *testing.T) {
	tmpDir := t.TempDir()

	maskContent := `# Mask specific slot
dev-lang/python:3.13
`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.mask"), []byte(maskContent), 0o644); err != nil {
		t.Fatalf("Failed to write mask file: %v", err)
	}

	cfg, err := config.LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	manager, err := NewMaskManager(cfg, "", "")
	if err != nil {
		t.Fatalf("Failed to create MaskManager: %v", err)
	}

	tests := []struct {
		name    string
		version string
		slot    string
		want    bool
	}{
		{"python-3.13.0 in slot 3.13 masked", "3.13.0", "3.13", true},
		{"python-3.13.1 in slot 3.13 masked", "3.13.1", "3.13", true},
		{"python-3.12.0 in slot 3.12 not masked", "3.12.0", "3.12", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manager.IsMasked("dev-lang", "python", tt.version, tt.slot)
			if got != tt.want {
				t.Errorf("IsMasked(dev-lang/python-%s:%s) = %v, want %v",
					tt.version, tt.slot, got, tt.want)
			}
		})
	}
}

// TestMaskManager_MaskDirectory tests loading masks from a directory.
func TestMaskManager_MaskDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package.mask directory
	maskDir := filepath.Join(tmpDir, "package.mask")
	if err := os.MkdirAll(maskDir, 0o755); err != nil {
		t.Fatalf("Failed to create mask dir: %v", err)
	}

	// Create multiple mask files
	file1Content := `>=sys-devel/gcc-16.0
`
	file2Content := `=dev-lang/python-3.13.0
`
	if err := os.WriteFile(filepath.Join(maskDir, "01-gcc"), []byte(file1Content), 0o644); err != nil {
		t.Fatalf("Failed to write mask file 1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(maskDir, "02-python"), []byte(file2Content), 0o644); err != nil {
		t.Fatalf("Failed to write mask file 2: %v", err)
	}

	cfg, err := config.LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	manager, err := NewMaskManager(cfg, "", "")
	if err != nil {
		t.Fatalf("Failed to create MaskManager: %v", err)
	}

	// Both masks should be loaded
	if !manager.IsMasked("sys-devel", "gcc", "16.0.9999", "16") {
		t.Error("gcc-16.0.9999 should be masked from file 01-gcc")
	}
	if !manager.IsMasked("dev-lang", "python", "3.13.0", "3.13") {
		t.Error("python-3.13.0 should be masked from file 02-python")
	}

	stats := manager.GetStats()
	if stats.UserMaskCount != 2 {
		t.Errorf("Expected 2 user masks from directory, got %d", stats.UserMaskCount)
	}
}
