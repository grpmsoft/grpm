package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/state"
)

func TestNewMerger(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
	}

	opts := InstallOptions{
		WorkDir: filepath.Join(tmpDir, "work"),
	}

	merger := NewMerger(installer, p, opts)

	if merger == nil {
		t.Fatal("NewMerger() returned nil")
	}

	if merger.pkg != p {
		t.Error("pkg not set correctly")
	}

	if merger.installer != installer {
		t.Error("installer not set correctly")
	}

	expectedImageDir := filepath.Join(opts.WorkDir, "image")
	if merger.imageDir != expectedImageDir {
		t.Errorf("imageDir = %q, want %q", merger.imageDir, expectedImageDir)
	}

	if merger.protect == nil {
		t.Error("protect should be initialized")
	}
}

func TestNewMergerWithProtect(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
	}

	protect := NewConfigProtect()
	protect.AddProtected("/etc")

	opts := InstallOptions{
		WorkDir: filepath.Join(tmpDir, "work"),
	}

	merger := NewMergerWithProtect(installer, p, opts, protect)

	if merger.protect != protect {
		t.Error("protect not set correctly")
	}
}

func TestMerger_Merge_ImageDirNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
	}

	opts := InstallOptions{
		WorkDir: filepath.Join(tmpDir, "nonexistent"),
	}

	merger := NewMerger(installer, p, opts)

	err := merger.Merge()
	if err == nil {
		t.Error("Merge() should fail when image directory doesn't exist")
	}
}

func TestMerger_Merge_WithFiles(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	workDir := filepath.Join(tmpDir, "work")
	imageDir := filepath.Join(workDir, "image")

	// Create directories
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test files in image directory
	binDir := filepath.Join(imageDir, "usr", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a test executable
	testFile := filepath.Join(binDir, "hello")
	if err := os.WriteFile(testFile, []byte("#!/bin/bash\necho hello"), 0755); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	p := &pkg.Package{
		Name:    "app-misc/hello",
		Version: "2.10",
		Slot:    pkg.Slot{Name: "0"},
	}

	opts := InstallOptions{
		WorkDir:   workDir,
		SkipHooks: true,
		Force:     true, // Force to bypass collision check in tests
	}

	merger := NewMerger(installer, p, opts)

	err := merger.Merge()
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	// Verify file was installed
	installedFile := filepath.Join(rootDir, "usr", "bin", "hello")
	if _, err := os.Stat(installedFile); err != nil {
		t.Errorf("File not installed: %v", err)
	}

	// Verify database was updated
	atom := "app-misc/hello-2.10"
	if !db.Has(atom) {
		t.Error("Package not added to database")
	}
}

func TestMerger_Merge_WithSymlink(t *testing.T) {
	// Skip on Windows - symlinks require admin privileges
	if os.Getenv("OS") == "Windows_NT" {
		t.Skip("Skipping symlink test on Windows (requires admin privileges)")
	}

	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	workDir := filepath.Join(tmpDir, "work")
	imageDir := filepath.Join(workDir, "image")

	// Create directories
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test file and symlink in image directory
	libDir := filepath.Join(imageDir, "usr", "lib")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create actual file
	libFile := filepath.Join(libDir, "libtest.so.1.0.0")
	if err := os.WriteFile(libFile, []byte("library content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create symlink
	symlinkPath := filepath.Join(libDir, "libtest.so")
	if err := os.Symlink("libtest.so.1.0.0", symlinkPath); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	p := &pkg.Package{
		Name:    "dev-libs/libtest",
		Version: "1.0.0",
	}

	opts := InstallOptions{
		WorkDir:   workDir,
		SkipHooks: true,
		Force:     true, // Force to bypass collision check in tests
	}

	merger := NewMerger(installer, p, opts)

	err := merger.Merge()
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	// Verify symlink was installed
	installedSymlink := filepath.Join(rootDir, "usr", "lib", "libtest.so")
	info, err := os.Lstat(installedSymlink)
	if err != nil {
		t.Errorf("Symlink not installed: %v", err)
	} else if info.Mode()&os.ModeSymlink == 0 {
		t.Error("Installed file is not a symlink")
	}
}

func TestMerger_installDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")

	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	p := &pkg.Package{Name: "test"}
	opts := InstallOptions{WorkDir: tmpDir}
	merger := NewMerger(installer, p, opts)

	// Create a mock directory info
	targetPath := filepath.Join(rootDir, "usr", "share", "doc")
	info, _ := os.Stat(tmpDir) // Use tmpDir as source for mode

	err := merger.installDirectory("usr/share/doc", targetPath, info)
	if err != nil {
		t.Fatalf("installDirectory() error = %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(targetPath); err != nil {
		t.Errorf("Directory not created: %v", err)
	}

	// Verify it was tracked
	if len(merger.installedFiles) != 1 {
		t.Errorf("installedFiles length = %d, want 1", len(merger.installedFiles))
	}

	if merger.installedFiles[0].Type != state.FileTypeDirectory {
		t.Errorf("File type = %v, want %v", merger.installedFiles[0].Type, state.FileTypeDirectory)
	}
}

func TestMerger_copyFile(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")

	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	p := &pkg.Package{Name: "test"}
	opts := InstallOptions{WorkDir: tmpDir}
	merger := NewMerger(installer, p, opts)

	// Create source file
	srcContent := []byte("test content")
	srcPath := filepath.Join(tmpDir, "source.txt")
	if err := os.WriteFile(srcPath, srcContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Copy file
	destPath := filepath.Join(rootDir, "dest.txt")
	if err := merger.copyFile(srcPath, destPath); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	// Verify content
	destContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(destContent) != string(srcContent) {
		t.Errorf("Copied content = %q, want %q", destContent, srcContent)
	}
}

func TestMerger_calculateHash(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	merger := NewMerger(installer, &pkg.Package{Name: "test"}, InstallOptions{})

	// Create test file
	testContent := []byte("test content for hashing")
	testPath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0644); err != nil {
		t.Fatal(err)
	}

	hash, err := merger.calculateHash(testPath)
	if err != nil {
		t.Fatalf("calculateHash() error = %v", err)
	}

	if hash == "" {
		t.Error("hash should not be empty")
	}

	// Hash should be 64 characters (SHA256 hex)
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash))
	}
}

func TestMerger_calculateHash_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	merger := NewMerger(installer, &pkg.Package{Name: "test"}, InstallOptions{})

	_, err := merger.calculateHash("/nonexistent/file")
	if err == nil {
		t.Error("calculateHash() should fail for non-existent file")
	}
}

func TestMerger_calculateTotalSize(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	merger := NewMerger(installer, &pkg.Package{Name: "test"}, InstallOptions{})

	// Add some tracked files with sizes
	merger.installedFiles = []state.InstalledFile{
		{Path: "/file1", Size: 100},
		{Path: "/file2", Size: 200},
		{Path: "/file3", Size: 300},
	}

	totalSize := merger.calculateTotalSize()
	if totalSize != 600 {
		t.Errorf("calculateTotalSize() = %d, want 600", totalSize)
	}
}

func TestMerger_GetProtectedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	merger := NewMerger(installer, &pkg.Package{Name: "test"}, InstallOptions{})

	// Initially empty
	if len(merger.GetProtectedFiles()) != 0 {
		t.Error("GetProtectedFiles() should return empty slice initially")
	}

	if merger.HasProtectedFiles() {
		t.Error("HasProtectedFiles() should return false initially")
	}

	// Add protected file
	merger.protectedFiles = []ProtectedFile{
		{Original: "/etc/test.conf", Protected: "/etc/._cfg0000_test.conf"},
	}

	if !merger.HasProtectedFiles() {
		t.Error("HasProtectedFiles() should return true")
	}

	if merger.GetProtectedCount() != 1 {
		t.Errorf("GetProtectedCount() = %d, want 1", merger.GetProtectedCount())
	}
}

func TestMerger_SetConfigProtect(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	merger := NewMerger(installer, &pkg.Package{Name: "test"}, InstallOptions{})

	newProtect := NewConfigProtect()
	newProtect.AddProtected("/custom/path")

	merger.SetConfigProtect(newProtect)

	if merger.protect != newProtect {
		t.Error("SetConfigProtect() did not update protect")
	}
}

func TestMerger_Merge_Force(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	workDir := filepath.Join(tmpDir, "work")
	imageDir := filepath.Join(workDir, "image")

	// Create directories
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test file in image directory
	binDir := filepath.Join(imageDir, "usr", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(binDir, "hello"), []byte("new"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create existing file in root (collision)
	existingDir := filepath.Join(rootDir, "usr", "bin")
	if err := os.MkdirAll(existingDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(existingDir, "hello"), []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(rootDir)
	installer := NewInstaller(rootDir, db)

	p := &pkg.Package{
		Name:    "app-misc/hello",
		Version: "2.10",
	}

	opts := InstallOptions{
		WorkDir:   workDir,
		SkipHooks: true,
		Force:     true, // Force overwrite
	}

	merger := NewMerger(installer, p, opts)

	err := merger.Merge()
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	// Verify file was overwritten
	content, _ := os.ReadFile(filepath.Join(rootDir, "usr", "bin", "hello"))
	if string(content) != "new" {
		t.Errorf("File content = %q, want %q", content, "new")
	}
}

func BenchmarkMerger_calculateHash(b *testing.B) {
	tmpDir := b.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	merger := NewMerger(installer, &pkg.Package{Name: "test"}, InstallOptions{})

	// Create test file
	testPath := filepath.Join(tmpDir, "test.bin")
	testData := make([]byte, 1024*1024) // 1MB
	_ = os.WriteFile(testPath, testData, 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = merger.calculateHash(testPath)
	}
}

func BenchmarkMerger_copyFile(b *testing.B) {
	tmpDir := b.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	merger := NewMerger(installer, &pkg.Package{Name: "test"}, InstallOptions{})

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.bin")
	testData := make([]byte, 1024*1024) // 1MB
	_ = os.WriteFile(srcPath, testData, 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		destPath := filepath.Join(tmpDir, "dest", string(rune('0'+i%10))+".bin")
		_ = os.MkdirAll(filepath.Dir(destPath), 0755)
		_ = merger.copyFile(srcPath, destPath)
	}
}
