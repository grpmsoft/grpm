package install

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/state"
)

// TestNewInstaller tests installer creation.
func TestNewInstaller(t *testing.T) {
	root := "/test/root"
	db := state.NewPackageDatabase(root)

	installer := NewInstaller(root, db)

	if installer.Root != root {
		t.Errorf("expected root %s, got %s", root, installer.Root)
	}

	if installer.DB != db {
		t.Error("expected DB to be set")
	}

	if installer.DryRun {
		t.Error("expected DryRun to be false by default")
	}

	if installer.Verbose {
		t.Error("expected Verbose to be false by default")
	}
}

// TestInstallOptions tests install options.
func TestInstallOptions(t *testing.T) {
	opts := InstallOptions{
		Replace:   true,
		Force:     false,
		KeepWork:  true,
		Pretend:   false,
		FetchOnly: false,
		SkipHooks: false,
		WorkDir:   "/var/tmp/portage/test",
	}

	if !opts.Replace {
		t.Error("expected Replace to be true")
	}

	if opts.Force {
		t.Error("expected Force to be false")
	}

	if opts.WorkDir != "/var/tmp/portage/test" {
		t.Errorf("expected WorkDir /var/tmp/portage/test, got %s", opts.WorkDir)
	}
}

// TestUninstallOptions tests uninstall options.
func TestUninstallOptions(t *testing.T) {
	opts := UninstallOptions{
		Force:        false,
		Pretend:      true,
		CleanDepends: false,
		SkipHooks:    true,
	}

	if opts.Force {
		t.Error("expected Force to be false")
	}

	if !opts.Pretend {
		t.Error("expected Pretend to be true")
	}

	if !opts.SkipHooks {
		t.Error("expected SkipHooks to be true")
	}
}

// TestInstallNilPackage tests installation with nil package.
func TestInstallNilPackage(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	opts := InstallOptions{
		WorkDir: tmpDir,
	}

	err := installer.Install(nil, opts)
	if err == nil {
		t.Error("expected error when installing nil package")
	}

	if err.Error() != "package is nil" {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestInstallMissingWorkDir tests installation without WorkDir.
func TestInstallMissingWorkDir(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
	}

	opts := InstallOptions{
		WorkDir: "", // Missing
	}

	err := installer.Install(p, opts)
	if err == nil {
		t.Error("expected error when WorkDir is missing")
	}
}

// TestInstallNonExistentWorkDir tests installation with non-existent WorkDir.
func TestInstallNonExistentWorkDir(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
	}

	opts := InstallOptions{
		WorkDir: "/nonexistent/work/dir",
	}

	err := installer.Install(p, opts)
	if err == nil {
		t.Error("expected error when WorkDir doesn't exist")
	}
}

// TestInstallPretendMode tests pretend (dry-run) mode.
func TestInstallPretendMode(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
	}

	opts := InstallOptions{
		WorkDir: workDir,
		Pretend: true,
	}

	err := installer.Install(p, opts)
	if err != nil {
		t.Errorf("pretend install failed: %v", err)
	}

	// Package should not be installed
	atom := "sys-libs/zlib-1.2.13"
	if db.Has(atom) {
		t.Error("package should not be installed in pretend mode")
	}
}

// TestInstallDryRunMode tests dry-run mode via installer flag.
func TestInstallDryRunMode(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)
	installer.DryRun = true

	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
	}

	opts := InstallOptions{
		WorkDir: workDir,
	}

	err := installer.Install(p, opts)
	if err != nil {
		t.Errorf("dry-run install failed: %v", err)
	}

	// Package should not be installed
	atom := "sys-libs/zlib-1.2.13"
	if db.Has(atom) {
		t.Error("package should not be installed in dry-run mode")
	}
}

// TestVerifyNotInstalled tests verifying non-installed package.
func TestVerifyNotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	problems, err := installer.Verify("sys-libs/zlib-1.2.13")
	if err == nil {
		t.Error("expected error when verifying non-installed package")
	}

	if problems != nil {
		t.Error("expected nil problems for non-installed package")
	}
}

// TestListNotInstalled tests listing files of non-installed package.
func TestListNotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	files, err := installer.List("sys-libs/zlib-1.2.13")
	if err == nil {
		t.Error("expected error when listing non-installed package")
	}

	if files != nil {
		t.Error("expected nil files for non-installed package")
	}
}

// TestUninstallNotInstalled tests uninstalling non-installed package.
func TestUninstallNotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	opts := UninstallOptions{}

	err := installer.Uninstall("sys-libs/zlib-1.2.13", opts)
	if err == nil {
		t.Error("expected error when uninstalling non-installed package")
	}
}

// TestUninstallPretendMode tests uninstall pretend mode.
func TestUninstallPretendMode(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)

	// Add a package to database
	installedPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "sys-libs/zlib",
			Version: "1.2.13",
			Slot:    pkg.Slot{Name: "0"},
		},
		InstallTime: time.Now(),
		Files:       []state.InstalledFile{},
	}
	if err := db.Add(installedPkg); err != nil {
		t.Fatal(err)
	}

	installer := NewInstaller(tmpDir, db)

	opts := UninstallOptions{
		Pretend: true,
	}

	err := installer.Uninstall("sys-libs/zlib-1.2.13", opts)
	if err != nil {
		t.Errorf("pretend uninstall failed: %v", err)
	}

	// Package should still be installed
	if !db.Has("sys-libs/zlib-1.2.13") {
		t.Error("package should still be installed in pretend mode")
	}
}

// TestUpgradeNotInstalled tests upgrading non-installed package.
func TestUpgradeNotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	newPkg := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.14",
		Slot:    pkg.Slot{Name: "0"},
	}

	opts := InstallOptions{
		WorkDir: tmpDir,
	}

	err := installer.Upgrade("sys-libs/zlib-1.2.13", newPkg, opts)
	if err == nil {
		t.Error("expected error when upgrading non-installed package")
	}
}

// TestProgressCallback tests progress callback functionality.
func TestProgressCallback(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	progressCalled := false
	installer.OnProgress = func(status string) {
		progressCalled = true
	}

	// This will fail but should call progress callback
	_ = installer.Uninstall("nonexistent", UninstallOptions{})

	if !progressCalled {
		t.Error("expected progress callback to be called")
	}
}

// TestVerboseMode tests verbose logging.
func TestVerboseMode(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	installer.Verbose = true

	// This should produce verbose output (we can't easily test stdout)
	_ = installer.Uninstall("nonexistent", UninstallOptions{})

	// Just verify verbose flag is set
	if !installer.Verbose {
		t.Error("expected Verbose to be true")
	}
}

// BenchmarkNewInstaller benchmarks installer creation.
func BenchmarkNewInstaller(b *testing.B) {
	root := "/test/root"
	db := state.NewPackageDatabase(root)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewInstaller(root, db)
	}
}
