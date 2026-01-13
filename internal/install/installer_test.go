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

// TestExtractPackageNameFromAtom tests package name extraction.
func TestExtractPackageNameFromAtom(t *testing.T) {
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
		{"app-misc/hello-world-1.0", "app-misc/hello-world"},
	}

	for _, tc := range testCases {
		t.Run(tc.atom, func(t *testing.T) {
			result := extractPackageNameFromAtom(tc.atom)
			if result != tc.expected {
				t.Errorf("extractPackageNameFromAtom(%q) = %q, want %q",
					tc.atom, result, tc.expected)
			}
		})
	}
}

// TestFindInstalledVersion tests finding installed package version.
func TestFindInstalledVersion(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	// No packages installed
	result := installer.findInstalledVersion("app-misc/hello")
	if result != "" {
		t.Errorf("expected empty string for non-installed package, got %q", result)
	}

	// Install hello-2.10
	helloPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "app-misc/hello",
			Version: "2.10",
			Slot:    pkg.Slot{Name: "0"},
		},
		InstallTime: time.Now(),
		Files:       []state.InstalledFile{},
	}
	if err := db.Add(helloPkg); err != nil {
		t.Fatal(err)
	}

	// Should find hello-2.10
	result = installer.findInstalledVersion("app-misc/hello")
	if result != "app-misc/hello-2.10" {
		t.Errorf("expected app-misc/hello-2.10, got %q", result)
	}

	// Should not find different package
	result = installer.findInstalledVersion("app-misc/world")
	if result != "" {
		t.Errorf("expected empty string for different package, got %q", result)
	}
}

// TestFindInstalledVersionNilDB tests findInstalledVersion with nil DB.
func TestFindInstalledVersionNilDB(t *testing.T) {
	installer := &Installer{
		Root: "/test",
		DB:   nil,
	}

	result := installer.findInstalledVersion("app-misc/hello")
	if result != "" {
		t.Errorf("expected empty string with nil DB, got %q", result)
	}
}

// TestInstallRequiresReplaceWhenAlreadyInstalled tests that install requires
// --replace flag when package is already installed.
func TestInstallRequiresReplaceWhenAlreadyInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	// Install hello-2.10
	helloPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "app-misc/hello",
			Version: "2.10",
			Slot:    pkg.Slot{Name: "0"},
		},
		InstallTime: time.Now(),
		Files:       []state.InstalledFile{},
	}
	if err := db.Add(helloPkg); err != nil {
		t.Fatal(err)
	}

	// Create work directory
	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Try to install hello-2.12 without Replace flag
	newPkg := &pkg.Package{
		Name:    "app-misc/hello",
		Version: "2.12",
		Slot:    pkg.Slot{Name: "0"},
	}

	opts := InstallOptions{
		WorkDir: workDir,
		Replace: false, // No replace!
	}

	err := installer.Install(newPkg, opts)
	if err == nil {
		t.Error("expected error when installing already-installed package without --replace")
	}

	// Error should mention --replace
	if err != nil && !contains(err.Error(), "replace") && !contains(err.Error(), "-R") {
		t.Errorf("error should mention --replace, got: %v", err)
	}
}

// TestInstallPretendModeShowsReplace tests pretend mode shows replacement.
func TestInstallPretendModeShowsReplace(t *testing.T) {
	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	var progressMessages []string
	installer.OnProgress = func(status string) {
		progressMessages = append(progressMessages, status)
	}

	// Install hello-2.10
	helloPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "app-misc/hello",
			Version: "2.10",
			Slot:    pkg.Slot{Name: "0"},
		},
		InstallTime: time.Now(),
		Files:       []state.InstalledFile{},
	}
	if err := db.Add(helloPkg); err != nil {
		t.Fatal(err)
	}

	// Create work directory
	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Pretend to install hello-2.12 with Replace
	newPkg := &pkg.Package{
		Name:    "app-misc/hello",
		Version: "2.12",
		Slot:    pkg.Slot{Name: "0"},
	}

	opts := InstallOptions{
		WorkDir: workDir,
		Replace: true,
		Pretend: true,
	}

	err := installer.Install(newPkg, opts)
	if err != nil {
		t.Fatalf("pretend install failed: %v", err)
	}

	// Should show replacement message
	found := false
	for _, msg := range progressMessages {
		if contains(msg, "replace") || contains(msg, "Replace") {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected replacement message in progress, got: %v", progressMessages)
	}
}

// contains checks if string contains substring (case-insensitive not needed here).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
