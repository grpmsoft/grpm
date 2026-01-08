package state

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
)

func createTestPackage(name string, version string, files []InstalledFile) *InstalledPackage {
	return &InstalledPackage{
		Package: &pkg.Package{
			Name:    name,
			Version: version,
			Slot:    pkg.Slot{Name: "0"},
		},
		InstallTime: time.Now(),
		Files:       files,
		USE:         []string{"ssl", "unicode"},
		CFLAGS:      "-O2 -pipe",
		Size:        1024 * 1024, // 1MB
	}
}

func TestNewPackageDatabase(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	if db.Root != "/var/db/pkg" {
		t.Errorf("Expected Root=/var/db/pkg, got %s", db.Root)
	}

	if db.Count() != 0 {
		t.Errorf("Expected empty database, got %d packages", db.Count())
	}
}

func TestAdd(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	pkg1 := createTestPackage("test-category/package1", "1.0.0", []InstalledFile{
		{Path: "/usr/bin/test1", Type: FileTypeRegular},
	})

	err := db.Add(pkg1)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if db.Count() != 1 {
		t.Errorf("Expected 1 package, got %d", db.Count())
	}

	// Test adding duplicate (should replace)
	pkg1Updated := createTestPackage("test-category/package1", "1.0.0", []InstalledFile{
		{Path: "/usr/bin/test1-updated", Type: FileTypeRegular},
	})

	err = db.Add(pkg1Updated)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if db.Count() != 1 {
		t.Errorf("Expected 1 package after update, got %d", db.Count())
	}

	// Verify file index was updated
	owners := db.FindFileOwners("/usr/bin/test1")
	if len(owners) != 0 {
		t.Errorf("Old file should not have owner")
	}

	owners = db.FindFileOwners("/usr/bin/test1-updated")
	if len(owners) != 1 {
		t.Errorf("New file should have 1 owner, got %d", len(owners))
	}
}

func TestAddNil(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	err := db.Add(nil)
	if err == nil {
		t.Error("Expected error when adding nil package")
	}

	pkg := &InstalledPackage{Package: nil}
	err = db.Add(pkg)
	if err == nil {
		t.Error("Expected error when package metadata is nil")
	}
}

func TestGet(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	pkg1 := createTestPackage("test-category/package1", "1.0.0", nil)
	_ = db.Add(pkg1)

	// Test successful get
	retrieved, err := db.Get("test-category/package1-1.0.0")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if retrieved.Package.Name != "test-category/package1" {
		t.Errorf("Expected package1-1.0.0, got %s", retrieved.Package.Name)
	}

	// Test get non-existent
	_, err = db.Get("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent package")
	}
}

func TestRemove(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	pkg1 := createTestPackage("test-category/package1", "1.0.0", []InstalledFile{
		{Path: "/usr/bin/test1", Type: FileTypeRegular},
	})
	_ = db.Add(pkg1)

	// Test successful remove
	err := db.Remove("test-category/package1-1.0.0")
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if db.Count() != 0 {
		t.Errorf("Expected 0 packages after remove, got %d", db.Count())
	}

	// Verify file index was cleaned up
	owners := db.FindFileOwners("/usr/bin/test1")
	if len(owners) != 0 {
		t.Errorf("File index should be empty after remove")
	}

	// Test remove non-existent
	err = db.Remove("nonexistent")
	if err == nil {
		t.Error("Expected error when removing non-existent package")
	}
}

func TestList(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	pkg1 := createTestPackage("test-category/package1", "1.0.0", nil)
	pkg2 := createTestPackage("test-category/package2", "2.0.0", nil)

	_ = db.Add(pkg1)
	_ = db.Add(pkg2)

	packages := db.List()

	if len(packages) != 2 {
		t.Errorf("Expected 2 packages, got %d", len(packages))
	}

	// Verify packages are in the list
	found := make(map[string]bool)
	for _, pkg := range packages {
		found[pkg.Package.Name] = true
	}

	if !found["test-category/package1"] || !found["test-category/package2"] {
		t.Error("Not all packages found in List()")
	}
}

func TestHas(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	pkg1 := createTestPackage("test-category/package1", "1.0.0", nil)
	_ = db.Add(pkg1)

	if !db.Has("test-category/package1-1.0.0") {
		t.Error("Expected Has() to return true for existing package")
	}

	if db.Has("nonexistent") {
		t.Error("Expected Has() to return false for non-existent package")
	}
}

func TestClear(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	pkg1 := createTestPackage("test-category/package1", "1.0.0", nil)
	pkg2 := createTestPackage("test-category/package2", "2.0.0", nil)

	_ = db.Add(pkg1)
	_ = db.Add(pkg2)

	if db.Count() != 2 {
		t.Fatalf("Expected 2 packages before clear")
	}

	db.Clear()

	if db.Count() != 0 {
		t.Errorf("Expected 0 packages after clear, got %d", db.Count())
	}
}

func TestFindFileOwners(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	pkg1 := createTestPackage("test-category/package1", "1.0.0", []InstalledFile{
		{Path: "/usr/bin/test1", Type: FileTypeRegular},
		{Path: "/usr/lib/test1.so", Type: FileTypeRegular},
	})

	pkg2 := createTestPackage("test-category/package2", "2.0.0", []InstalledFile{
		{Path: "/usr/bin/test2", Type: FileTypeRegular},
	})

	_ = db.Add(pkg1)
	_ = db.Add(pkg2)

	// Test finding owner
	owners := db.FindFileOwners("/usr/bin/test1")
	if len(owners) != 1 {
		t.Fatalf("Expected 1 owner, got %d", len(owners))
	}

	if owners[0].Package.Name != "test-category/package1" {
		t.Errorf("Wrong owner: %s", owners[0].Package.Name)
	}

	// Test file with no owner
	owners = db.FindFileOwners("/nonexistent")
	if len(owners) != 0 {
		t.Errorf("Expected 0 owners for nonexistent file, got %d", len(owners))
	}
}

func TestStats(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	pkg1 := createTestPackage("test-category/package1", "1.0.0", []InstalledFile{
		{Path: "/usr/bin/test1", Type: FileTypeRegular},
		{Path: "/usr/lib/test1.so", Type: FileTypeRegular},
	})
	pkg1.Size = 1024 * 1024 // 1MB

	pkg2 := createTestPackage("test-category/package2", "2.0.0", []InstalledFile{
		{Path: "/usr/bin/test2", Type: FileTypeRegular},
	})
	pkg2.Size = 2 * 1024 * 1024 // 2MB

	_ = db.Add(pkg1)
	_ = db.Add(pkg2)

	stats := db.Stats()

	if stats.PackageCount != 2 {
		t.Errorf("Expected PackageCount=2, got %d", stats.PackageCount)
	}

	if stats.FileCount != 3 {
		t.Errorf("Expected FileCount=3, got %d", stats.FileCount)
	}

	expectedSize := int64(3 * 1024 * 1024) // 3MB
	if stats.TotalSize != expectedSize {
		t.Errorf("Expected TotalSize=%d, got %d", expectedSize, stats.TotalSize)
	}
}

func TestFileTypeString(t *testing.T) {
	tests := []struct {
		ft   FileType
		want string
	}{
		{FileTypeRegular, "obj"},
		{FileTypeDirectory, "dir"},
		{FileTypeSymlink, "sym"},
		{FileTypeHardlink, "hardlink"},
		{FileType(999), "unknown"},
	}

	for _, tt := range tests {
		got := tt.ft.String()
		if got != tt.want {
			t.Errorf("FileType(%d).String() = %s, want %s", tt.ft, got, tt.want)
		}
	}
}

// Test concurrent access (thread safety)
func TestConcurrentAccess(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	// Concurrent adds
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				pkg := createTestPackage("test-category/package-"+string(rune(id)), "1.0.0", nil)
				_ = db.Add(pkg)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent reads
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = db.List()
				_ = db.Count()
			}
		}()
	}
	wg.Wait()

	// Concurrent queries
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = db.FindByCategory("test-category")
			}
		}()
	}
	wg.Wait()

	// Should not panic - test passes if no race conditions
}

// Benchmark tests
func BenchmarkAdd(b *testing.B) {
	db := NewPackageDatabase("/var/db/pkg")

	files := []InstalledFile{
		{Path: "/usr/bin/test", Type: FileTypeRegular},
		{Path: "/usr/lib/test.so", Type: FileTypeRegular},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pkg := createTestPackage("test-category/package", "1.0.0", files)
		_ = db.Add(pkg)
	}
}

func BenchmarkFindFileOwners(b *testing.B) {
	db := NewPackageDatabase("/var/db/pkg")

	// Add 1000 packages with 10 files each
	for i := 0; i < 1000; i++ {
		files := make([]InstalledFile, 10)
		for j := 0; j < 10; j++ {
			files[j] = InstalledFile{
				Path: "/usr/bin/test" + string(rune(i)) + string(rune(j)),
				Type: FileTypeRegular,
			}
		}
		pkg := createTestPackage("test-category/package-"+string(rune(i)), "1.0.0", files)
		_ = db.Add(pkg)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = db.FindFileOwners("/usr/bin/test500")
	}
}

func BenchmarkList(b *testing.B) {
	db := NewPackageDatabase("/var/db/pkg")

	// Add 1000 packages
	for i := 0; i < 1000; i++ {
		pkg := createTestPackage("test-category/package-"+string(rune(i)), "1.0.0", nil)
		_ = db.Add(pkg)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = db.List()
	}
}

func TestInstalledFile(t *testing.T) {
	tests := []struct {
		name string
		file InstalledFile
	}{
		{
			name: "regular file",
			file: InstalledFile{
				Path:  "/usr/bin/test",
				Type:  FileTypeRegular,
				Size:  1024,
				Mode:  0755,
				Hash:  "abc123",
				MTime: time.Now().Unix(),
			},
		},
		{
			name: "directory",
			file: InstalledFile{
				Path: "/usr/share/doc",
				Type: FileTypeDirectory,
			},
		},
		{
			name: "symlink",
			file: InstalledFile{
				Path:   "/usr/bin/link",
				Type:   FileTypeSymlink,
				Target: "/usr/bin/real",
				MTime:  time.Now().Unix(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify structure can be created
			if tt.file.Path == "" {
				t.Error("File path should not be empty")
			}
		})
	}
}

func TestBuildInfo(t *testing.T) {
	buildInfo := BuildInfo{
		Host:      "gentoo-build",
		BuildDate: time.Now(),
		CFLAGS:    "-O2 -pipe",
		CXXFLAGS:  "-O2 -pipe",
		LDFLAGS:   "-Wl,-O1",
		Features:  []string{"sandbox", "ccache"},
		EAPI:      "8",
	}

	if buildInfo.Host != "gentoo-build" {
		t.Error("BuildInfo Host not set correctly")
	}

	if len(buildInfo.Features) != 2 {
		t.Error("BuildInfo Features not set correctly")
	}
}

func TestDatabaseStats_String(t *testing.T) {
	stats := DatabaseStats{
		PackageCount: 100,
		FileCount:    5000,
		TotalSize:    100 * 1024 * 1024, // 100MB
	}

	// Just verify stats can be accessed
	if stats.PackageCount != 100 {
		t.Errorf("PackageCount = %d, want 100", stats.PackageCount)
	}

	if stats.FileCount != 5000 {
		t.Errorf("FileCount = %d, want 5000", stats.FileCount)
	}
}

func TestEmptyDatabase(t *testing.T) {
	db := NewPackageDatabase("/var/db/pkg")

	// Test operations on empty database
	if db.Count() != 0 {
		t.Error("Empty database should have 0 packages")
	}

	packages := db.List()
	if len(packages) != 0 {
		t.Error("Empty database List() should return empty slice")
	}

	owners := db.FindFileOwners("/any/file")
	if len(owners) != 0 {
		t.Error("Empty database should have no file owners")
	}

	stats := db.Stats()
	if !reflect.DeepEqual(stats, DatabaseStats{PackageCount: 0, FileCount: 0, TotalSize: 0}) {
		t.Errorf("Empty database stats incorrect: %+v", stats)
	}
}
