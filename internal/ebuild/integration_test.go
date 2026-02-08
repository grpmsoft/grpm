package ebuild

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/fetch"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// ============================================================================
// Integration Test Suite for Ebuild Workflow
// ============================================================================

// TestIntegration_ExecutorCreation tests creating an Executor with various options.
func TestIntegration_ExecutorCreation(t *testing.T) {
	t.Run("valid package", func(t *testing.T) {
		testPkg := createTestPackage("app-misc/hello", "2.10")

		opts := ExecutorOptions{
			TmpDir:  t.TempDir(),
			PortDir: t.TempDir(),
			DistDir: t.TempDir(),
		}

		executor, err := NewExecutor(testPkg, opts)
		if err != nil {
			t.Fatalf("NewExecutor failed: %v", err)
		}

		if executor.Package != testPkg {
			t.Error("Package not set correctly")
		}
		if executor.Env == nil {
			t.Error("Environment not initialized")
		}
		if executor.EnableSandbox != false {
			t.Error("EnableSandbox should be false by default in options")
		}
	})

	t.Run("nil package", func(t *testing.T) {
		opts := ExecutorOptions{
			TmpDir:  t.TempDir(),
			PortDir: t.TempDir(),
			DistDir: t.TempDir(),
		}

		_, err := NewExecutor(nil, opts)
		if err == nil {
			t.Error("Expected error for nil package")
		}
	})

	t.Run("with fetcher", func(t *testing.T) {
		testPkg := createTestPackage("app-misc/hello", "2.10")
		mf := &mockFetcher{}

		opts := ExecutorOptions{
			TmpDir:  t.TempDir(),
			PortDir: t.TempDir(),
			DistDir: t.TempDir(),
			Fetcher: mf,
		}

		executor, err := NewExecutor(testPkg, opts)
		if err != nil {
			t.Fatalf("NewExecutor failed: %v", err)
		}

		if executor.Fetcher == nil {
			t.Error("Fetcher should be set")
		}
	})

	t.Run("default options", func(t *testing.T) {
		opts := DefaultOptions()

		if opts.TmpDir != "/var/tmp/portage" {
			t.Errorf("TmpDir = %q, want /var/tmp/portage", opts.TmpDir)
		}
		if opts.PortDir != "/var/db/repos/gentoo" {
			t.Errorf("PortDir = %q, want /var/db/repos/gentoo", opts.PortDir)
		}
		if opts.DistDir != "/var/cache/distfiles" {
			t.Errorf("DistDir = %q, want /var/cache/distfiles", opts.DistDir)
		}
		if opts.EnableSandbox != true {
			t.Error("EnableSandbox should be true by default")
		}
	})
}

// TestIntegration_EnvironmentSetup tests environment variable setup.
func TestIntegration_EnvironmentSetup(t *testing.T) {
	testPkg := createTestPackage("sys-libs/zlib", "1.2.13")

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(),
		DistDir: t.TempDir(),
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	env := executor.Env

	// Verify package metadata variables
	if env.PN != "zlib" {
		t.Errorf("PN = %q, want %q", env.PN, "zlib")
	}
	if env.PV != "1.2.13" {
		t.Errorf("PV = %q, want %q", env.PV, "1.2.13")
	}
	if env.P != "zlib-1.2.13" {
		t.Errorf("P = %q, want %q", env.P, "zlib-1.2.13")
	}
	if env.CATEGORY != "sys-libs" {
		t.Errorf("CATEGORY = %q, want %q", env.CATEGORY, "sys-libs")
	}

	// Verify directory variables
	if env.WORKDIR == "" {
		t.Error("WORKDIR should be set")
	}
	if env.S == "" {
		t.Error("S should be set")
	}
	if env.D == "" {
		t.Error("D should be set")
	}
	if env.T == "" {
		t.Error("T should be set")
	}

	// Verify ToSlice includes PATH
	slice := env.ToSlice()
	hasPath := false
	for _, s := range slice {
		if strings.HasPrefix(s, "PATH=") {
			hasPath = true
			break
		}
	}
	if !hasPath {
		t.Error("ToSlice should include PATH from environment")
	}
}

// TestIntegration_DirectoryCreation tests creating build directories.
func TestIntegration_DirectoryCreation(t *testing.T) {
	testPkg := createTestPackage("app-misc/hello", "2.10")
	tmpDir := t.TempDir()

	opts := ExecutorOptions{
		TmpDir:  tmpDir,
		PortDir: t.TempDir(),
		DistDir: t.TempDir(),
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// Create directories
	if err := executor.Env.CreateDirectories(); err != nil {
		t.Fatalf("CreateDirectories failed: %v", err)
	}

	// Verify directories exist
	dirs := []string{
		executor.Env.WORKDIR,
		executor.Env.S,
		executor.Env.D,
		executor.Env.T,
		executor.Env.HOME,
	}

	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("Directory %s does not exist: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}
}

// TestIntegration_PhaseSetup tests the setup phase.
func TestIntegration_PhaseSetup(t *testing.T) {
	testPkg := createTestPackage("app-misc/hello", "2.10")

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(),
		DistDir: t.TempDir(),
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	result := executor.ExecutePhaseReal(PhaseSetup)

	if !result.Success {
		t.Errorf("Setup phase failed: %v", result.Error)
	}
	if result.Output == "" {
		t.Error("Setup phase should produce output")
	}
	if result.Duration < 0 {
		t.Error("Duration should be non-negative")
	}
}

// TestIntegration_PhaseUnpack_NoTarball tests unpack phase when no tarball exists.
func TestIntegration_PhaseUnpack_NoTarball(t *testing.T) {
	testPkg := createTestPackage("app-misc/hello", "2.10")

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(),
		DistDir: t.TempDir(),
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// Create directories first
	if err := executor.Env.CreateDirectories(); err != nil {
		t.Fatalf("CreateDirectories failed: %v", err)
	}

	result := executor.ExecutePhaseReal(PhaseUnpack)

	// Should succeed even without tarball (ebuild-only package)
	if !result.Success {
		t.Errorf("Unpack phase failed unexpectedly: %v", result.Error)
	}
	if !strings.Contains(result.Output, "No source tarball") {
		t.Errorf("Expected 'No source tarball' message, got: %s", result.Output)
	}
}

// TestIntegration_PhaseUnpack_WithTarball tests extracting a tarball.
func TestIntegration_PhaseUnpack_WithTarball(t *testing.T) {
	testPkg := createTestPackage("app-misc/hello", "2.10")
	distDir := t.TempDir()

	// Create a test tarball
	tarballPath := filepath.Join(distDir, "hello-2.10.tar.gz")
	if err := createTestTarball(tarballPath, "hello-2.10", map[string]string{
		"README":                 "Hello World",
		"configure":              "#!/bin/sh\necho configured",
		"src/main.c":             "int main() { return 0; }",
		"src/hello.h":            "#ifndef HELLO_H\n#define HELLO_H\n#endif",
		"Makefile.in":            "all:\n\techo building",
		"doc/hello.1":            ".TH HELLO 1",
		"COPYING":                "GPL License",
		"ChangeLog":              "Version 2.10",
		"src/subdir/helper.c":    "void helper() {}",
		"include/public/hello.h": "#pragma once",
	}); err != nil {
		t.Fatalf("Failed to create test tarball: %v", err)
	}

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(),
		DistDir: distDir,
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// Create directories first
	if err := executor.Env.CreateDirectories(); err != nil {
		t.Fatalf("CreateDirectories failed: %v", err)
	}

	result := executor.ExecutePhaseReal(PhaseUnpack)

	if !result.Success {
		t.Errorf("Unpack phase failed: %v", result.Error)
	}

	// Verify files were extracted
	expectedFiles := []string{
		filepath.Join(executor.Env.WORKDIR, "hello-2.10", "README"),
		filepath.Join(executor.Env.WORKDIR, "hello-2.10", "configure"),
		filepath.Join(executor.Env.WORKDIR, "hello-2.10", "src", "main.c"),
	}

	for _, f := range expectedFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("Expected file not found: %s", f)
		}
	}
}

// TestIntegration_PhasePrepare tests the prepare phase.
func TestIntegration_PhasePrepare(t *testing.T) {
	testPkg := createTestPackage("app-misc/hello", "2.10")

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(),
		DistDir: t.TempDir(),
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// Create directories first
	if err := executor.Env.CreateDirectories(); err != nil {
		t.Fatalf("CreateDirectories failed: %v", err)
	}

	result := executor.ExecutePhaseReal(PhasePrepare)

	// Should succeed even without source directory
	if !result.Success {
		t.Errorf("Prepare phase failed: %v", result.Error)
	}
}

// TestIntegration_PhaseConfigure_NoConfigureScript tests configure without script.
func TestIntegration_PhaseConfigure_NoConfigureScript(t *testing.T) {
	testPkg := createTestPackage("app-misc/hello", "2.10")

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(),
		DistDir: t.TempDir(),
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// Create directories first
	if err := executor.Env.CreateDirectories(); err != nil {
		t.Fatalf("CreateDirectories failed: %v", err)
	}

	result := executor.ExecutePhaseReal(PhaseConfigure)

	// Should skip when no configure script
	if !result.Success {
		t.Errorf("Configure phase failed: %v", result.Error)
	}
	if !strings.Contains(result.Output, "No configure") {
		t.Errorf("Expected 'No configure' message, got: %s", result.Output)
	}
}

// TestIntegration_PhaseCompile_NoMakefile tests compile without Makefile.
func TestIntegration_PhaseCompile_NoMakefile(t *testing.T) {
	testPkg := createTestPackage("app-misc/hello", "2.10")

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(),
		DistDir: t.TempDir(),
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// Create directories first
	if err := executor.Env.CreateDirectories(); err != nil {
		t.Fatalf("CreateDirectories failed: %v", err)
	}

	result := executor.ExecutePhaseReal(PhaseCompile)

	// Should skip when no Makefile
	if !result.Success {
		t.Errorf("Compile phase failed: %v", result.Error)
	}
	if !strings.Contains(result.Output, "No Makefile") {
		t.Errorf("Expected 'No Makefile' message, got: %s", result.Output)
	}
}

// TestIntegration_PhaseInstall_NoMakefile tests install without Makefile.
// Per PMS, packages without Makefiles (virtual/*, acct-*, Python packages)
// should succeed with a skip message rather than fail.
func TestIntegration_PhaseInstall_NoMakefile(t *testing.T) {
	testPkg := createTestPackage("app-misc/hello", "2.10")

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(),
		DistDir: t.TempDir(),
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// Create directories first
	if err := executor.Env.CreateDirectories(); err != nil {
		t.Fatalf("CreateDirectories failed: %v", err)
	}

	result := executor.ExecutePhaseReal(PhaseInstall)

	// Should succeed (skip gracefully) when no Makefile
	if !result.Success {
		t.Errorf("Install phase should succeed without Makefile, got error: %v", result.Error)
	}
}

// TestIntegration_PhaseTest tests the test phase.
func TestIntegration_PhaseTest(t *testing.T) {
	testPkg := createTestPackage("app-misc/hello", "2.10")

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(),
		DistDir: t.TempDir(),
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// Create directories first
	if err := executor.Env.CreateDirectories(); err != nil {
		t.Fatalf("CreateDirectories failed: %v", err)
	}

	result := executor.ExecutePhaseReal(PhaseTest)

	// Should skip when no test target
	if !result.Success {
		t.Errorf("Test phase failed: %v", result.Error)
	}
}

// TestIntegration_HookPhases tests hook phases (preinst, postinst, etc.).
func TestIntegration_HookPhases(t *testing.T) {
	testPkg := createTestPackage("app-misc/hello", "2.10")

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(),
		DistDir: t.TempDir(),
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	hookPhases := []Phase{PhasePreinst, PhasePostinst, PhasePrerem, PhasePostrm}

	for _, phase := range hookPhases {
		t.Run(phase.String(), func(t *testing.T) {
			result := executor.ExecutePhaseReal(phase)

			if !result.Success {
				t.Errorf("Hook phase %s failed: %v", phase, result.Error)
			}
			if !strings.Contains(result.Output, "completed") {
				t.Errorf("Expected 'completed' message for %s, got: %s", phase, result.Output)
			}
		})
	}
}

// TestIntegration_UnknownPhase tests handling of unknown phases.
func TestIntegration_UnknownPhase(t *testing.T) {
	testPkg := createTestPackage("app-misc/hello", "2.10")

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(),
		DistDir: t.TempDir(),
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	result := executor.ExecutePhaseReal(Phase("unknown"))

	if result.Success {
		t.Error("Unknown phase should fail")
	}
	if result.Error == nil {
		t.Error("Unknown phase should have error")
	}
}

// TestIntegration_ExecutePhases tests executing multiple phases.
func TestIntegration_ExecutePhases(t *testing.T) {
	testPkg := createTestPackage("app-misc/hello", "2.10")

	opts := ExecutorOptions{
		TmpDir:   t.TempDir(),
		PortDir:  t.TempDir(),
		DistDir:  t.TempDir(),
		KeepWork: true, // Keep work directory for inspection
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// Track progress
	var progressEvents []string
	executor.OnProgress = func(phase Phase, status string) {
		progressEvents = append(progressEvents, phase.String()+":"+status)
	}

	// Execute limited phases that should succeed without external tools
	phases := []Phase{PhaseSetup, PhaseUnpack, PhasePrepare}

	results, err := executor.ExecutePhases(phases)
	if err != nil {
		t.Fatalf("ExecutePhases failed: %v", err)
	}

	// Check results
	if len(results) != len(phases) {
		t.Errorf("Expected %d results, got %d", len(phases), len(results))
	}

	for _, result := range results {
		if !result.Success {
			t.Errorf("Phase %s failed: %v", result.Phase, result.Error)
		}
	}

	// Check progress events
	if len(progressEvents) == 0 {
		t.Error("Expected progress events")
	}
}

// TestIntegration_ExecutePhases_SkipTests tests that test phase is skipped when disabled.
func TestIntegration_ExecutePhases_SkipTests(t *testing.T) {
	testPkg := createTestPackage("app-misc/hello", "2.10")

	opts := ExecutorOptions{
		TmpDir:      t.TempDir(),
		PortDir:     t.TempDir(),
		DistDir:     t.TempDir(),
		EnableTests: false,
		KeepWork:    true,
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// Track progress
	testPhaseExecuted := false
	executor.OnProgress = func(phase Phase, status string) {
		if phase == PhaseTest && status == "starting" {
			testPhaseExecuted = true
		}
	}

	phases := []Phase{PhaseSetup, PhaseTest}
	_, _ = executor.ExecutePhases(phases)

	if testPhaseExecuted {
		t.Error("Test phase should be skipped when EnableTests is false")
	}
}

// TestIntegration_ExecutePhases_WithFetch tests fetch integration in phase execution.
func TestIntegration_ExecutePhases_WithFetch(t *testing.T) {
	testPkg := createTestPackage("app-misc/hello", "2.10")

	// Create repo with Manifest
	repoDir := t.TempDir()
	pkgDir := filepath.Join(repoDir, "app-misc", "hello")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir: %v", err)
	}

	manifestContent := "DIST hello-2.10.tar.gz 725946 BLAKE2B abc123 SHA512 def456\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "Manifest"), []byte(manifestContent), 0644); err != nil {
		t.Fatalf("Failed to write Manifest: %v", err)
	}

	mf := &mockFetcher{}

	opts := ExecutorOptions{
		TmpDir:   t.TempDir(),
		PortDir:  repoDir,
		DistDir:  t.TempDir(),
		Fetcher:  mf,
		KeepWork: true,
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// Execute phases including unpack (which triggers fetch)
	phases := []Phase{PhaseSetup, PhaseUnpack}
	_, _ = executor.ExecutePhases(phases)

	// Verify fetch was called before unpack
	if !mf.fetchCalled {
		t.Error("Fetcher should be called before unpack phase")
	}
}

// TestIntegration_GetDirectories tests directory accessor methods.
func TestIntegration_GetDirectories(t *testing.T) {
	testPkg := createTestPackage("app-misc/hello", "2.10")

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(),
		DistDir: t.TempDir(),
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	imageDir := executor.GetImageDirectory()
	if imageDir == "" {
		t.Error("GetImageDirectory should return non-empty path")
	}
	if imageDir != executor.Env.D {
		t.Errorf("GetImageDirectory = %q, want %q", imageDir, executor.Env.D)
	}

	workDir := executor.GetWorkDirectory()
	if workDir == "" {
		t.Error("GetWorkDirectory should return non-empty path")
	}
	if workDir != executor.Env.WORKDIR {
		t.Errorf("GetWorkDirectory = %q, want %q", workDir, executor.Env.WORKDIR)
	}
}

// TestIntegration_ParseEbuild tests ebuild parsing stub.
func TestIntegration_ParseEbuild(t *testing.T) {
	testPkg := createTestPackage("app-misc/hello", "2.10")

	repoDir := t.TempDir()

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: repoDir,
		DistDir: t.TempDir(),
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// ParseEbuild is currently a stub, should not fail
	if err := executor.ParseEbuild(); err != nil {
		t.Errorf("ParseEbuild failed: %v", err)
	}

	// EbuildPath should be set after parsing
	if executor.EbuildPath == "" {
		t.Error("EbuildPath should be set after ParseEbuild")
	}
}

// TestIntegration_Cleanup tests directory cleanup.
func TestIntegration_Cleanup(t *testing.T) {
	testPkg := createTestPackage("app-misc/hello", "2.10")

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(),
		DistDir: t.TempDir(),
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// Create directories
	if err := executor.Env.CreateDirectories(); err != nil {
		t.Fatalf("CreateDirectories failed: %v", err)
	}

	// Verify directories exist
	if _, err := os.Stat(executor.Env.WORKDIR); os.IsNotExist(err) {
		t.Fatal("WORKDIR should exist")
	}

	// Cleanup
	if err := executor.Env.Cleanup(); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify work directory is removed
	workBase := filepath.Dir(executor.Env.WORKDIR)
	if _, err := os.Stat(workBase); !os.IsNotExist(err) {
		t.Error("Work base directory should be removed after cleanup")
	}
}

// ============================================================================
// Tarball Extraction Tests
// ============================================================================

// TestIntegration_ExtractTarball_Gzip tests extracting .tar.gz files.
func TestIntegration_ExtractTarball_Gzip(t *testing.T) {
	destDir := t.TempDir()
	tarballPath := filepath.Join(t.TempDir(), "test.tar.gz")

	if err := createTestTarball(tarballPath, "test-1.0", map[string]string{
		"file.txt":       "content",
		"dir/nested.txt": "nested content",
	}); err != nil {
		t.Fatalf("Failed to create tarball: %v", err)
	}

	if err := extractTarball(tarballPath, destDir); err != nil {
		t.Fatalf("extractTarball failed: %v", err)
	}

	// Verify extraction
	expectedFile := filepath.Join(destDir, "test-1.0", "file.txt")
	if content, err := os.ReadFile(expectedFile); err != nil {
		t.Errorf("Failed to read extracted file: %v", err)
	} else if string(content) != "content" {
		t.Errorf("Content = %q, want %q", string(content), "content")
	}

	nestedFile := filepath.Join(destDir, "test-1.0", "dir", "nested.txt")
	if _, err := os.Stat(nestedFile); os.IsNotExist(err) {
		t.Error("Nested file should exist")
	}
}

// TestIntegration_ExtractTarball_UnsupportedFormat tests unsupported format handling.
func TestIntegration_ExtractTarball_UnsupportedFormat(t *testing.T) {
	destDir := t.TempDir()
	tarballPath := filepath.Join(t.TempDir(), "test.tar.unknown")

	// Create an empty file with unsupported extension
	if err := os.WriteFile(tarballPath, []byte("not a tarball"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := extractTarball(tarballPath, destDir)
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("Error should mention unsupported format: %v", err)
	}
}

// TestIntegration_ExtractTarball_NonexistentFile tests handling of missing files.
func TestIntegration_ExtractTarball_NonexistentFile(t *testing.T) {
	destDir := t.TempDir()

	err := extractTarball("/nonexistent/file.tar.gz", destDir)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

// ============================================================================
// Compression Reader Tests
// ============================================================================

// TestIntegration_CompressionReader tests createCompressionReader for various formats.
func TestIntegration_CompressionReader(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		content   []byte
		wantError bool
	}{
		{
			name:     "gzip",
			filename: "test.tar.gz",
			content:  createGzipContent(t),
		},
		{
			name:     "tgz",
			filename: "test.tgz",
			content:  createGzipContent(t),
		},
		{
			name:     "bzip2",
			filename: "test.tar.bz2",
			content:  nil, // bzip2 reader doesn't need valid content for creation
		},
		{
			name:     "plain tar",
			filename: "test.tar",
			content:  []byte("tar content"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), tt.filename)

			content := tt.content
			if content == nil {
				content = []byte("dummy content")
			}

			if err := os.WriteFile(tmpFile, content, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			file, err := os.Open(tmpFile)
			if err != nil {
				t.Fatalf("Failed to open test file: %v", err)
			}
			defer func() { _ = file.Close() }()

			reader, err := createCompressionReader(file, tmpFile)
			if tt.wantError && err == nil {
				t.Error("Expected error")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.wantError && reader == nil {
				t.Error("Reader should not be nil")
			}
		})
	}
}

// ============================================================================
// Phase Classification Tests
// ============================================================================

// TestIntegration_PhaseClassification tests phase type classification methods.
func TestIntegration_PhaseClassification(t *testing.T) {
	tests := []struct {
		phase   Phase
		isBuild bool
		isTest  bool
		isHook  bool
		isFetch bool
	}{
		{PhaseFetch, true, false, false, true},
		{PhaseSetup, true, false, false, false},
		{PhaseUnpack, true, false, false, false},
		{PhasePrepare, true, false, false, false},
		{PhaseConfigure, true, false, false, false},
		{PhaseCompile, true, false, false, false},
		{PhaseTest, false, true, false, false},
		{PhaseInstall, true, false, false, false},
		{PhasePreinst, false, false, true, false},
		{PhasePostinst, false, false, true, false},
		{PhasePrerem, false, false, true, false},
		{PhasePostrm, false, false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.phase.String(), func(t *testing.T) {
			if got := tt.phase.IsBuildPhase(); got != tt.isBuild {
				t.Errorf("IsBuildPhase() = %v, want %v", got, tt.isBuild)
			}
			if got := tt.phase.IsTestPhase(); got != tt.isTest {
				t.Errorf("IsTestPhase() = %v, want %v", got, tt.isTest)
			}
			if got := tt.phase.IsHookPhase(); got != tt.isHook {
				t.Errorf("IsHookPhase() = %v, want %v", got, tt.isHook)
			}
			if got := tt.phase.IsFetchPhase(); got != tt.isFetch {
				t.Errorf("IsFetchPhase() = %v, want %v", got, tt.isFetch)
			}
		})
	}
}

// TestIntegration_StandardPhases tests StandardPhases function.
func TestIntegration_StandardPhases(t *testing.T) {
	phases := StandardPhases()

	expectedOrder := []Phase{
		PhaseSetup,
		PhaseUnpack,
		PhasePrepare,
		PhaseConfigure,
		PhaseCompile,
		PhaseTest,
		PhaseInstall,
	}

	if len(phases) != len(expectedOrder) {
		t.Errorf("StandardPhases length = %d, want %d", len(phases), len(expectedOrder))
	}

	for i, phase := range phases {
		if phase != expectedOrder[i] {
			t.Errorf("StandardPhases[%d] = %s, want %s", i, phase, expectedOrder[i])
		}
	}
}

// TestIntegration_InstallPhases tests InstallPhases function.
func TestIntegration_InstallPhases(t *testing.T) {
	phases := InstallPhases()

	if len(phases) != 2 {
		t.Errorf("InstallPhases length = %d, want 2", len(phases))
	}
	if phases[0] != PhasePreinst {
		t.Errorf("InstallPhases[0] = %s, want %s", phases[0], PhasePreinst)
	}
	if phases[1] != PhasePostinst {
		t.Errorf("InstallPhases[1] = %s, want %s", phases[1], PhasePostinst)
	}
}

// TestIntegration_RemovalPhases tests RemovalPhases function.
func TestIntegration_RemovalPhases(t *testing.T) {
	phases := RemovalPhases()

	if len(phases) != 2 {
		t.Errorf("RemovalPhases length = %d, want 2", len(phases))
	}
	if phases[0] != PhasePrerem {
		t.Errorf("RemovalPhases[0] = %s, want %s", phases[0], PhasePrerem)
	}
	if phases[1] != PhasePostrm {
		t.Errorf("RemovalPhases[1] = %s, want %s", phases[1], PhasePostrm)
	}
}

// ============================================================================
// Package Revision Tests
// ============================================================================

// TestIntegration_PackageWithRevision tests handling of package revisions.
func TestIntegration_PackageWithRevision(t *testing.T) {
	testPkg := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13-r1",
		Slot:    pkg.NewSlot("0", "1.2.13"),
	}

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(),
		DistDir: t.TempDir(),
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	env := executor.Env

	if env.PV != "1.2.13" {
		t.Errorf("PV = %q, want %q", env.PV, "1.2.13")
	}
	// PR is extracted as "r1" (includes 'r' prefix) from "-r1" suffix
	if env.PR != "r1" {
		t.Errorf("PR = %q, want %q", env.PR, "r1")
	}
	if env.PF != "zlib-1.2.13-r1" {
		t.Errorf("PF = %q, want %q", env.PF, "zlib-1.2.13-r1")
	}
}

// ============================================================================
// USE Flag Tests
// ============================================================================

// TestIntegration_USEFlags tests USE flag handling in environment.
func TestIntegration_USEFlags(t *testing.T) {
	testPkg := &pkg.Package{
		Name:    "app-misc/hello",
		Version: "2.10",
		Slot:    pkg.NewSlot("0", ""),
		UseFlags: map[string]bool{
			"nls":    true,
			"doc":    true,
			"debug":  false,
			"static": false,
		},
	}

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(),
		DistDir: t.TempDir(),
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	useFlags := executor.Env.USE

	// Should contain enabled flags
	if !strings.Contains(useFlags, "nls") {
		t.Error("USE should contain 'nls'")
	}
	if !strings.Contains(useFlags, "doc") {
		t.Error("USE should contain 'doc'")
	}

	// Should not contain disabled flags
	if strings.Contains(useFlags, "debug") {
		t.Error("USE should not contain 'debug'")
	}
	if strings.Contains(useFlags, "static") {
		t.Error("USE should not contain 'static'")
	}
}

// ============================================================================
// Error Handling Tests
// ============================================================================

// TestIntegration_InvalidPackageName tests handling of invalid package names.
func TestIntegration_InvalidPackageName(t *testing.T) {
	tests := []struct {
		name    string
		pkgName string
	}{
		{"no category", "hello"},
		{"empty", ""},
		{"triple slash", "a/b/c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testPkg := &pkg.Package{
				Name:    tt.pkgName,
				Version: "1.0",
			}

			opts := ExecutorOptions{
				TmpDir:  t.TempDir(),
				PortDir: t.TempDir(),
				DistDir: t.TempDir(),
			}

			_, err := NewExecutor(testPkg, opts)
			if err == nil {
				t.Error("Expected error for invalid package name")
			}
		})
	}
}

// ============================================================================
// Platform-Specific Tests
// ============================================================================

// TestIntegration_SymlinkExtraction tests symlink extraction.
func TestIntegration_SymlinkExtraction(t *testing.T) {
	// Skip on Windows - symlinks require admin privileges
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows")
	}

	destDir := t.TempDir()
	tarballPath := filepath.Join(t.TempDir(), "test.tar.gz")

	// Create tarball with symlink
	if err := createTestTarballWithSymlink(tarballPath); err != nil {
		t.Fatalf("Failed to create tarball with symlink: %v", err)
	}

	if err := extractTarball(tarballPath, destDir); err != nil {
		t.Fatalf("extractTarball failed: %v", err)
	}

	// Verify symlink was extracted
	symlinkPath := filepath.Join(destDir, "test-1.0", "link")
	info, err := os.Lstat(symlinkPath)
	if err != nil {
		t.Fatalf("Failed to stat symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("Expected symlink")
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// createTestPackage creates a test package with the given name and version.
func createTestPackage(name, version string) *pkg.Package {
	return &pkg.Package{
		Name:    name,
		Version: version,
		Slot:    pkg.NewSlot("0", ""),
	}
}

// createTestTarball creates a test .tar.gz file with the given files.
func createTestTarball(path, rootDir string, files map[string]string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	gzWriter := gzip.NewWriter(file)
	defer func() { _ = gzWriter.Close() }()

	tarWriter := tar.NewWriter(gzWriter)
	defer func() { _ = tarWriter.Close() }()

	// Add root directory
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     rootDir + "/",
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}); err != nil {
		return err
	}

	// Add files
	for name, content := range files {
		fullPath := rootDir + "/" + name

		// Create parent directories
		dir := filepath.Dir(fullPath)
		if dir != rootDir {
			parts := strings.Split(dir, "/")
			for i := range parts {
				if i == 0 {
					continue // Skip root
				}
				partialDir := strings.Join(parts[:i+1], "/") + "/"
				_ = tarWriter.WriteHeader(&tar.Header{
					Name:     partialDir,
					Mode:     0755,
					Typeflag: tar.TypeDir,
				})
			}
		}

		// Write file
		if err := tarWriter.WriteHeader(&tar.Header{
			Name: fullPath,
			Mode: 0644,
			Size: int64(len(content)),
		}); err != nil {
			return err
		}
		if _, err := tarWriter.Write([]byte(content)); err != nil {
			return err
		}
	}

	return nil
}

// createTestTarballWithSymlink creates a test tarball containing a symlink.
func createTestTarballWithSymlink(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	gzWriter := gzip.NewWriter(file)
	defer func() { _ = gzWriter.Close() }()

	tarWriter := tar.NewWriter(gzWriter)
	defer func() { _ = tarWriter.Close() }()

	// Add directory
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     "test-1.0/",
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}); err != nil {
		return err
	}

	// Add target file
	content := "target content"
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: "test-1.0/target",
		Mode: 0644,
		Size: int64(len(content)),
	}); err != nil {
		return err
	}
	if _, err := tarWriter.Write([]byte(content)); err != nil {
		return err
	}

	// Add symlink
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     "test-1.0/link",
		Mode:     0777,
		Typeflag: tar.TypeSymlink,
		Linkname: "target",
	}); err != nil {
		return err
	}

	return nil
}

// createGzipContent creates valid gzip-compressed content for testing.
func createGzipContent(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	if _, err := gzWriter.Write([]byte("test content")); err != nil {
		t.Fatalf("Failed to write gzip content: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		t.Fatalf("Failed to close gzip writer: %v", err)
	}
	return buf.Bytes()
}

// Ensure mockFetcher implements fetch.Fetcher interface.
var _ fetch.Fetcher = (*mockFetcher)(nil)

// Mock fetcher for testing - reuse from executor_fetch_test.go if possible.
// Note: If mockFetcher is already defined in executor_fetch_test.go and is in
// the same package, it will be available here.
