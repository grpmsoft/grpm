package install

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/state"
)

func TestNewUnmerger(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	installedPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "sys-libs/zlib",
			Version: "1.2.13",
		},
		InstallTime: time.Now(),
		Files:       []state.InstalledFile{},
	}

	opts := UninstallOptions{}

	unmerger := NewUnmerger(installer, installedPkg, opts)

	if unmerger == nil {
		t.Fatal("NewUnmerger() returned nil")
	}

	if unmerger.pkg != installedPkg {
		t.Error("pkg not set correctly")
	}

	if unmerger.installer != installer {
		t.Error("installer not set correctly")
	}

	if unmerger.removedFiles == nil {
		t.Error("removedFiles should be initialized")
	}
}

func TestUnmerger_Unmerge(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")

	// Create root directory
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test files to be removed
	binDir := filepath.Join(rootDir, "usr", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(binDir, "hello")
	if err := os.WriteFile(testFile, []byte("test"), 0755); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	installedPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "app-misc/hello",
			Version: "2.10",
		},
		InstallTime: time.Now(),
		Files: []state.InstalledFile{
			{Path: "/usr/bin/hello", Type: state.FileTypeRegular},
			{Path: "/usr/bin", Type: state.FileTypeDirectory},
			{Path: "/usr", Type: state.FileTypeDirectory},
		},
	}

	// Add package to database
	if err := db.Add(installedPkg); err != nil {
		t.Fatal(err)
	}

	opts := UninstallOptions{
		SkipHooks: true,
	}

	unmerger := NewUnmerger(installer, installedPkg, opts)

	err := unmerger.Unmerge()
	if err != nil {
		t.Fatalf("Unmerge() error = %v", err)
	}

	// Verify file was removed
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("File should have been removed")
	}

	// Verify database was updated
	atom := "app-misc/hello-2.10"
	if db.Has(atom) {
		t.Error("Package should have been removed from database")
	}
}

func TestUnmerger_Unmerge_WithSymlink(t *testing.T) {
	// Skip on Windows - symlinks require admin privileges
	if os.Getenv("OS") == "Windows_NT" {
		t.Skip("Skipping symlink test on Windows (requires admin privileges)")
	}

	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")

	// Create root directory
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test files including symlink
	libDir := filepath.Join(rootDir, "usr", "lib")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatal(err)
	}

	realFile := filepath.Join(libDir, "libtest.so.1.0.0")
	if err := os.WriteFile(realFile, []byte("library"), 0644); err != nil {
		t.Fatal(err)
	}

	symlinkPath := filepath.Join(libDir, "libtest.so")
	if err := os.Symlink("libtest.so.1.0.0", symlinkPath); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	installedPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "dev-libs/libtest",
			Version: "1.0.0",
		},
		InstallTime: time.Now(),
		Files: []state.InstalledFile{
			{Path: "/usr/lib/libtest.so.1.0.0", Type: state.FileTypeRegular},
			{Path: "/usr/lib/libtest.so", Type: state.FileTypeSymlink, Target: "libtest.so.1.0.0"},
			{Path: "/usr/lib", Type: state.FileTypeDirectory},
		},
	}

	if err := db.Add(installedPkg); err != nil {
		t.Fatal(err)
	}

	opts := UninstallOptions{SkipHooks: true}
	unmerger := NewUnmerger(installer, installedPkg, opts)

	err := unmerger.Unmerge()
	if err != nil {
		t.Fatalf("Unmerge() error = %v", err)
	}

	// Verify both file and symlink were removed
	if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
		t.Error("Symlink should have been removed")
	}

	if _, err := os.Stat(realFile); !os.IsNotExist(err) {
		t.Error("Real file should have been removed")
	}
}

func TestUnmerger_removeFiles_FileNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")

	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	// Package with file that doesn't exist
	installedPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "test/pkg",
			Version: "1.0",
		},
		Files: []state.InstalledFile{
			{Path: "/nonexistent/file", Type: state.FileTypeRegular},
		},
	}

	if err := db.Add(installedPkg); err != nil {
		t.Fatal(err)
	}

	opts := UninstallOptions{SkipHooks: true}
	unmerger := NewUnmerger(installer, installedPkg, opts)

	// Should not fail when file doesn't exist
	err := unmerger.removeFiles()
	if err != nil {
		t.Errorf("removeFiles() should not fail for non-existent files: %v", err)
	}
}

func TestUnmerger_removeRegularFile(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")

	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test file
	testFile := filepath.Join(rootDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	installedPkg := &state.InstalledPackage{
		Package: &pkg.Package{Name: "test"},
	}

	unmerger := NewUnmerger(installer, installedPkg, UninstallOptions{})

	err := unmerger.removeRegularFile(testFile, "/test.txt")
	if err != nil {
		t.Fatalf("removeRegularFile() error = %v", err)
	}

	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("File should have been removed")
	}
}

func TestUnmerger_removeSymlink(t *testing.T) {
	// Skip on Windows - symlinks require admin privileges
	if os.Getenv("OS") == "Windows_NT" {
		t.Skip("Skipping symlink test on Windows (requires admin privileges)")
	}

	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")

	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create symlink
	targetFile := filepath.Join(rootDir, "target.txt")
	if err := os.WriteFile(targetFile, []byte("target"), 0644); err != nil {
		t.Fatal(err)
	}

	symlinkPath := filepath.Join(rootDir, "link.txt")
	if err := os.Symlink("target.txt", symlinkPath); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	installedPkg := &state.InstalledPackage{
		Package: &pkg.Package{Name: "test"},
	}

	unmerger := NewUnmerger(installer, installedPkg, UninstallOptions{})

	err := unmerger.removeSymlink(symlinkPath, "/link.txt")
	if err != nil {
		t.Fatalf("removeSymlink() error = %v", err)
	}

	if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
		t.Error("Symlink should have been removed")
	}

	// Target should still exist
	if _, err := os.Stat(targetFile); err != nil {
		t.Error("Target file should not be removed")
	}
}

func TestUnmerger_removeEmptyDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")

	// Create nested empty directories
	deepDir := filepath.Join(rootDir, "usr", "share", "doc", "test")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	installedPkg := &state.InstalledPackage{
		Package: &pkg.Package{Name: "test"},
		Files: []state.InstalledFile{
			{Path: "/usr/share/doc/test", Type: state.FileTypeDirectory},
			{Path: "/usr/share/doc", Type: state.FileTypeDirectory},
			{Path: "/usr/share", Type: state.FileTypeDirectory},
		},
	}

	unmerger := NewUnmerger(installer, installedPkg, UninstallOptions{})

	err := unmerger.removeEmptyDirectories()
	if err != nil {
		t.Fatalf("removeEmptyDirectories() error = %v", err)
	}

	// Test directory should be removed
	if _, err := os.Stat(deepDir); !os.IsNotExist(err) {
		t.Error("Empty directory should have been removed")
	}
}

func TestUnmerger_removeEmptyDirectories_NotEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")

	// Create directory with content
	docDir := filepath.Join(rootDir, "usr", "share", "doc")
	if err := os.MkdirAll(docDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Add a file that should prevent deletion
	otherFile := filepath.Join(docDir, "other.txt")
	if err := os.WriteFile(otherFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	installedPkg := &state.InstalledPackage{
		Package: &pkg.Package{Name: "test"},
		Files: []state.InstalledFile{
			{Path: "/usr/share/doc", Type: state.FileTypeDirectory},
		},
	}

	unmerger := NewUnmerger(installer, installedPkg, UninstallOptions{})

	err := unmerger.removeEmptyDirectories()
	if err != nil {
		t.Fatalf("removeEmptyDirectories() error = %v", err)
	}

	// Directory should NOT be removed because it has content
	if _, err := os.Stat(docDir); os.IsNotExist(err) {
		t.Error("Non-empty directory should not have been removed")
	}
}

func TestUnmerger_removeRegularFile_ModifiedBackup(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")

	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test file
	testFile := filepath.Join(rootDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("modified content"), 0644); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	// Create installed package with different hash
	installedPkg := &state.InstalledPackage{
		Package: &pkg.Package{Name: "test"},
		Files: []state.InstalledFile{
			{Path: "/test.txt", Type: state.FileTypeRegular, Hash: "originalhash"},
		},
	}

	unmerger := NewUnmerger(installer, installedPkg, UninstallOptions{})

	err := unmerger.removeRegularFile(testFile, "/test.txt")
	if err != nil {
		t.Fatalf("removeRegularFile() error = %v", err)
	}

	// Original file should be removed, backup should exist
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Original file should have been removed/renamed")
	}

	backupFile := testFile + ".grpm-backup"
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		t.Error("Backup file should exist for modified file")
	}
}

func TestUnmerger_updateDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")

	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	installedPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "test/pkg",
			Version: "1.0",
		},
	}

	// Add package to database
	if err := db.Add(installedPkg); err != nil {
		t.Fatal(err)
	}

	// Verify it's in database
	atom := "test/pkg-1.0"
	if !db.Has(atom) {
		t.Fatal("Package should be in database")
	}

	unmerger := NewUnmerger(installer, installedPkg, UninstallOptions{})

	err := unmerger.updateDatabase()
	if err != nil {
		t.Fatalf("updateDatabase() error = %v", err)
	}

	// Verify it's removed
	if db.Has(atom) {
		t.Error("Package should have been removed from database")
	}
}

func TestUnmerger_runPreRemoveHooks(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")

	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	installedPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "test/pkg",
			Version: "1.0",
		},
	}

	unmerger := NewUnmerger(installer, installedPkg, UninstallOptions{})

	// Pre-remove hooks should not fail (currently just a stub)
	err := unmerger.runPreRemoveHooks()
	if err != nil {
		t.Errorf("runPreRemoveHooks() error = %v", err)
	}
}

func TestUnmerger_runPostRemoveHooks(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")

	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	installedPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "test/pkg",
			Version: "1.0",
		},
	}

	unmerger := NewUnmerger(installer, installedPkg, UninstallOptions{})

	// Post-remove hooks may fail in test environment (e.g., ldconfig)
	err := unmerger.runPostRemoveHooks()
	if err != nil {
		t.Logf("runPostRemoveHooks() error = %v (expected in test environment)", err)
	}
}

func TestUnmerger_Unmerge_WithHooks(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")

	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test file
	binDir := filepath.Join(rootDir, "usr", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(binDir, "test-app")
	if err := os.WriteFile(testFile, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	installedPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "test/app",
			Version: "1.0",
		},
		InstallTime: time.Now(),
		Files: []state.InstalledFile{
			{Path: "/usr/bin/test-app", Type: state.FileTypeRegular},
			{Path: "/usr/bin", Type: state.FileTypeDirectory},
		},
	}

	if err := db.Add(installedPkg); err != nil {
		t.Fatal(err)
	}

	// Run with hooks enabled (SkipHooks = false)
	opts := UninstallOptions{
		SkipHooks: false,
	}

	unmerger := NewUnmerger(installer, installedPkg, opts)

	err := unmerger.Unmerge()
	if err != nil {
		t.Fatalf("Unmerge() with hooks error = %v", err)
	}

	// File should be removed
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("File should have been removed")
	}
}

func TestUnmerger_removeFiles_UnknownFileType(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")

	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a regular file
	testFile := filepath.Join(rootDir, "unknown")
	if err := os.WriteFile(testFile, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	// Package with unknown file type (invalid type value)
	installedPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "test/pkg",
			Version: "1.0",
		},
		Files: []state.InstalledFile{
			{Path: "/unknown", Type: state.FileType(99)}, // Unknown type
		},
	}

	if err := db.Add(installedPkg); err != nil {
		t.Fatal(err)
	}

	opts := UninstallOptions{SkipHooks: true}
	unmerger := NewUnmerger(installer, installedPkg, opts)

	// Should not fail - unknown types are skipped
	err := unmerger.removeFiles()
	if err != nil {
		t.Errorf("removeFiles() should skip unknown types: %v", err)
	}
}

func TestUninstallOptions_SkipHooks(t *testing.T) {
	tests := []struct {
		name      string
		skipHooks bool
	}{
		{"with_hooks", false},
		{"without_hooks", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := UninstallOptions{SkipHooks: tt.skipHooks}
			if opts.SkipHooks != tt.skipHooks {
				t.Errorf("SkipHooks = %v, want %v", opts.SkipHooks, tt.skipHooks)
			}
		})
	}
}

func BenchmarkUnmerger_Unmerge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		tmpDir := b.TempDir()
		rootDir := filepath.Join(tmpDir, "root")
		_ = os.MkdirAll(rootDir, 0755)

		// Create 100 files
		files := make([]state.InstalledFile, 100)
		for j := 0; j < 100; j++ {
			filePath := filepath.Join(rootDir, "usr", "share", "doc", "file"+string(rune('0'+j%10)))
			_ = os.MkdirAll(filepath.Dir(filePath), 0755)
			_ = os.WriteFile(filePath, []byte("content"), 0644)
			files[j] = state.InstalledFile{
				Path: "/usr/share/doc/file" + string(rune('0'+j%10)),
				Type: state.FileTypeRegular,
			}
		}

		db := state.NewPackageDatabase(rootDir)
		installer := NewInstaller(rootDir, db)

		installedPkg := &state.InstalledPackage{
			Package: &pkg.Package{Name: "test", Version: "1.0"},
			Files:   files,
		}
		_ = db.Add(installedPkg)

		b.StartTimer()

		unmerger := NewUnmerger(installer, installedPkg, UninstallOptions{SkipHooks: true})
		_ = unmerger.Unmerge()
	}
}
