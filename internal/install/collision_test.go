package install

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/state"
)

// TestNewCollisionDetector tests detector creation.
func TestNewCollisionDetector(t *testing.T) {
	db := state.NewPackageDatabase("/test/root")
	detector := NewCollisionDetector(db)

	if detector.db != db {
		t.Error("expected db to be set")
	}

	if len(detector.protectedPaths) == 0 {
		t.Error("expected some protected paths by default")
	}
}

// TestCollisionTypeString tests collision type string representation.
func TestCollisionTypeString(t *testing.T) {
	tests := []struct {
		collisionType CollisionType
		expected      string
	}{
		{CollisionFileExists, "file exists"},
		{CollisionOwnedByOther, "owned by other package"},
		{CollisionProtected, "protected path"},
		{CollisionType(999), "unknown"},
	}

	for _, tt := range tests {
		result := tt.collisionType.String()
		if result != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, result)
		}
	}
}

// TestDetectNoCollisions tests detection with no collisions.
func TestDetectNoCollisions(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	detector := NewCollisionDetector(db)

	// Non-existent files should have no collisions
	filesToInstall := []string{
		filepath.Join(tmpDir, "usr/bin/test"),
		filepath.Join(tmpDir, "usr/lib/libtest.so"),
	}

	collisions, err := detector.Detect(filesToInstall, "sys-libs/test-1.0")
	if err != nil {
		t.Errorf("detection failed: %v", err)
	}

	if len(collisions) != 0 {
		t.Errorf("expected no collisions, got %d", len(collisions))
	}
}

// TestDetectFileExists tests detection of untracked files.
func TestDetectFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	detector := NewCollisionDetector(db)

	// Create a file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	filesToInstall := []string{testFile}

	collisions, err := detector.Detect(filesToInstall, "sys-libs/test-1.0")
	if err != nil {
		t.Errorf("detection failed: %v", err)
	}

	if len(collisions) != 1 {
		t.Fatalf("expected 1 collision, got %d", len(collisions))
	}

	if collisions[0].Type != CollisionFileExists {
		t.Errorf("expected CollisionFileExists, got %v", collisions[0].Type)
	}

	if collisions[0].Path != testFile {
		t.Errorf("expected path %s, got %s", testFile, collisions[0].Path)
	}
}

// TestDetectOwnedByOther tests detection of files owned by other packages.
func TestDetectOwnedByOther(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)

	// Install a package with a file (using absolute path in tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	installedPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "sys-apps/existing",
			Version: "1.0",
			Slot:    pkg.Slot{Name: "0"},
		},
		InstallTime: time.Now(),
		Files: []state.InstalledFile{
			{
				Path: testFile,
				Type: state.FileTypeRegular,
			},
		},
	}

	if err := db.Add(installedPkg); err != nil {
		t.Fatal(err)
	}

	// Create the actual file
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	detector := NewCollisionDetector(db)
	filesToInstall := []string{testFile}

	collisions, err := detector.Detect(filesToInstall, "sys-apps/new-1.0")
	if err != nil {
		t.Errorf("detection failed: %v", err)
	}

	if len(collisions) != 1 {
		t.Fatalf("expected 1 collision, got %d", len(collisions))
	}

	if collisions[0].Type != CollisionOwnedByOther {
		t.Errorf("expected CollisionOwnedByOther, got %v", collisions[0].Type)
	}

	if collisions[0].ExistingOwner != "sys-apps/existing-1.0" {
		t.Errorf("expected owner sys-apps/existing-1.0, got %s", collisions[0].ExistingOwner)
	}
}

// TestDetectProtectedPath tests detection of protected paths.
func TestDetectProtectedPath(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	detector := NewCollisionDetector(db)

	// Try to install to protected path
	filesToInstall := []string{"/etc/passwd"}

	collisions, err := detector.Detect(filesToInstall, "sys-apps/test-1.0")
	if err != nil {
		t.Errorf("detection failed: %v", err)
	}

	if len(collisions) != 1 {
		t.Fatalf("expected 1 collision, got %d", len(collisions))
	}

	if collisions[0].Type != CollisionProtected {
		t.Errorf("expected CollisionProtected, got %v", collisions[0].Type)
	}
}

// TestIsProtected tests protected path checking.
func TestIsProtected(t *testing.T) {
	db := state.NewPackageDatabase("/test/root")
	detector := NewCollisionDetector(db)

	tests := []struct {
		path     string
		expected bool
	}{
		{"/etc/passwd", true},
		{"/etc/shadow", true},
		{"/etc/group", true},
		{"/etc/fstab", true},
		{"/boot/vmlinuz", true},
		{"/usr/bin/test", false},
		{"/home/user/file", false},
	}

	for _, tt := range tests {
		result := detector.isProtected(tt.path)
		if result != tt.expected {
			t.Errorf("isProtected(%s) = %v, expected %v", tt.path, result, tt.expected)
		}
	}
}

// TestAddProtectedPath tests adding custom protected paths.
func TestAddProtectedPath(t *testing.T) {
	db := state.NewPackageDatabase("/test/root")
	detector := NewCollisionDetector(db)

	customPath := "/custom/protected/file"
	detector.AddProtectedPath(customPath)

	if !detector.isProtected(customPath) {
		t.Errorf("expected %s to be protected after adding", customPath)
	}
}

// TestCollisionString tests collision string representation.
func TestCollisionString(t *testing.T) {
	tests := []struct {
		collision Collision
		contains  string
	}{
		{
			Collision{
				Path:     "/test/file",
				Type:     CollisionFileExists,
				NewOwner: "pkg-1.0",
			},
			"file exists",
		},
		{
			Collision{
				Path:          "/test/file",
				Type:          CollisionOwnedByOther,
				ExistingOwner: "pkg1-1.0",
				NewOwner:      "pkg2-1.0",
			},
			"file conflict",
		},
		{
			Collision{
				Path:     "/etc/passwd",
				Type:     CollisionProtected,
				NewOwner: "pkg-1.0",
			},
			"protected file",
		},
	}

	for _, tt := range tests {
		result := tt.collision.String()
		if len(result) == 0 {
			t.Error("expected non-empty string")
		}
	}
}

// TestDetectSamePackageUpgrade tests that same package can replace its files.
func TestDetectSamePackageUpgrade(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)

	// Install a package with a file (using absolute path in tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	packageName := "sys-apps/test-1.0"
	installedPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "sys-apps/test",
			Version: "1.0",
			Slot:    pkg.Slot{Name: "0"},
		},
		InstallTime: time.Now(),
		Files: []state.InstalledFile{
			{
				Path: testFile,
				Type: state.FileTypeRegular,
			},
		},
	}

	if err := db.Add(installedPkg); err != nil {
		t.Fatal(err)
	}

	// Create the actual file
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	detector := NewCollisionDetector(db)
	filesToInstall := []string{testFile}

	// Try to install same package (upgrade scenario)
	collisions, err := detector.Detect(filesToInstall, packageName)
	if err != nil {
		t.Errorf("detection failed: %v", err)
	}

	// Should have no collision because it's the same package
	if len(collisions) != 0 {
		t.Errorf("expected no collisions for same package upgrade, got %d", len(collisions))
	}
}

// TestDetectMultipleCollisions tests detection of multiple collisions.
func TestDetectMultipleCollisions(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)

	// Create multiple files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("test1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("test2"), 0644); err != nil {
		t.Fatal(err)
	}

	detector := NewCollisionDetector(db)
	filesToInstall := []string{file1, file2, "/etc/passwd"}

	collisions, err := detector.Detect(filesToInstall, "sys-apps/test-1.0")
	if err != nil {
		t.Errorf("detection failed: %v", err)
	}

	// Should have 3 collisions: 2 file exists + 1 protected
	if len(collisions) != 3 {
		t.Errorf("expected 3 collisions, got %d", len(collisions))
	}
}

// BenchmarkDetect benchmarks collision detection.
func BenchmarkDetect(b *testing.B) {
	tmpDir := b.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	detector := NewCollisionDetector(db)

	filesToInstall := []string{
		"/usr/bin/test1",
		"/usr/bin/test2",
		"/usr/lib/libtest.so",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = detector.Detect(filesToInstall, "sys-apps/test-1.0")
	}
}

// BenchmarkIsProtected benchmarks protected path checking.
func BenchmarkIsProtected(b *testing.B) {
	db := state.NewPackageDatabase("/test/root")
	detector := NewCollisionDetector(db)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = detector.isProtected("/usr/bin/test")
	}
}

// TestExtractPackageName tests extracting package name from atom.
func TestExtractPackageName(t *testing.T) {
	testCases := []struct {
		atom     string
		expected string
	}{
		{"app-misc/hello-2.12", "app-misc/hello"},
		{"sys-libs/zlib-1.2.13", "sys-libs/zlib"},
		{"sys-libs/zlib-1.2.13-r1", "sys-libs/zlib"},
		{"dev-lang/python-3.11.5", "dev-lang/python"},
		{"app-misc/hello", "app-misc/hello"}, // no version
		{"x11-libs/gtk+-3.24.38", "x11-libs/gtk+"},
		{"www-client/firefox-115.0.3", "www-client/firefox"},
		{"media-libs/libpng-1.6.40", "media-libs/libpng"},
		{"dev-util/cmake-3.27.4", "dev-util/cmake"},
		{"sys-apps/portage-2.3.99", "sys-apps/portage"},
		// Edge cases
		{"zlib-1.2.13", "zlib-1.2.13"}, // no category - returns as-is
		{"app-misc/hello-world-1.0", "app-misc/hello-world"},
		{"dev-libs/openssl-3.0.10_beta1", "dev-libs/openssl"},
	}

	for _, tc := range testCases {
		t.Run(tc.atom, func(t *testing.T) {
			result := extractPackageName(tc.atom)
			if result != tc.expected {
				t.Errorf("extractPackageName(%q) = %q, want %q", tc.atom, result, tc.expected)
			}
		})
	}
}

// TestIsSamePackage tests same package comparison.
func TestIsSamePackage(t *testing.T) {
	testCases := []struct {
		atom1    string
		atom2    string
		expected bool
	}{
		// Same package, different versions
		{"app-misc/hello-2.12", "app-misc/hello-2.10", true},
		{"sys-libs/zlib-1.2.13", "sys-libs/zlib-1.2.11-r1", true},
		{"dev-lang/python-3.11.5", "dev-lang/python-3.10.12", true},

		// Same package, one without version
		{"app-misc/hello-2.12", "app-misc/hello", true},
		{"sys-libs/zlib", "sys-libs/zlib-1.2.13", true},

		// Different packages
		{"app-misc/hello-2.12", "app-misc/world-1.0", false},
		{"sys-libs/zlib-1.2.13", "sys-libs/glibc-2.38", false},
		{"dev-lang/python-3.11.5", "dev-lang/ruby-3.2.2", false},

		// Different categories
		{"app-misc/hello-2.12", "sys-apps/hello-1.0", false},

		// Edge cases
		{"app-misc/hello", "app-misc/hello", true},
		// Note: packages without category are not valid in real scenarios,
		// but extractPackageName handles them by returning as-is
	}

	for _, tc := range testCases {
		name := tc.atom1 + "_vs_" + tc.atom2
		t.Run(name, func(t *testing.T) {
			result := isSamePackage(tc.atom1, tc.atom2)
			if result != tc.expected {
				t.Errorf("isSamePackage(%q, %q) = %v, want %v",
					tc.atom1, tc.atom2, result, tc.expected)
			}
		})
	}
}

// TestDetectSamePackageDifferentVersions tests that files owned by same
// package but different version don't cause collisions (protect-owned behavior).
func TestDetectSamePackageDifferentVersions(t *testing.T) {
	tmpDir := t.TempDir()
	root := tmpDir

	db := state.NewPackageDatabase(root)

	// Install "hello-2.10" - use absolute path for consistency
	testFile := filepath.Join(root, "usr/bin/hello")
	if err := os.MkdirAll(filepath.Dir(testFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, []byte("hello 2.10"), 0755); err != nil {
		t.Fatal(err)
	}

	// Store file with same absolute path as will be used in detection
	oldPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "app-misc/hello",
			Version: "2.10",
		},
		Files: []state.InstalledFile{
			{Path: testFile, Type: state.FileTypeRegular}, // Use same path!
		},
	}
	if err := db.Add(oldPkg); err != nil {
		t.Fatal(err)
	}

	// Now try to install "hello-2.12" - should NOT have collision
	detector := NewCollisionDetector(db)
	filesToInstall := []string{testFile}

	// Use different version of same package
	collisions, err := detector.Detect(filesToInstall, "app-misc/hello-2.12")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	// Should have NO collisions (protect-owned behavior)
	if len(collisions) > 0 {
		t.Errorf("Expected 0 collisions for same package upgrade, got %d: %v",
			len(collisions), collisions)
	}
}

// TestDetectDifferentPackageCollision tests that files owned by different
// package DO cause collisions.
func TestDetectDifferentPackageCollision(t *testing.T) {
	tmpDir := t.TempDir()
	root := tmpDir

	db := state.NewPackageDatabase(root)

	// Install "hello-2.10" - use absolute path for consistency
	testFile := filepath.Join(root, "usr/bin/hello")
	if err := os.MkdirAll(filepath.Dir(testFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, []byte("hello 2.10"), 0755); err != nil {
		t.Fatal(err)
	}

	// Store file with same absolute path as will be used in detection
	oldPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "app-misc/hello",
			Version: "2.10",
		},
		Files: []state.InstalledFile{
			{Path: testFile, Type: state.FileTypeRegular}, // Use same path!
		},
	}
	if err := db.Add(oldPkg); err != nil {
		t.Fatal(err)
	}

	// Now try to install different package that wants same file
	detector := NewCollisionDetector(db)
	filesToInstall := []string{testFile}

	// Use completely different package
	collisions, err := detector.Detect(filesToInstall, "app-misc/world-1.0")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	// Should have collision
	if len(collisions) != 1 {
		t.Errorf("Expected 1 collision for different package, got %d", len(collisions))
	}

	if len(collisions) > 0 && collisions[0].Type != CollisionOwnedByOther {
		t.Errorf("Expected CollisionOwnedByOther, got %v", collisions[0].Type)
	}
}
