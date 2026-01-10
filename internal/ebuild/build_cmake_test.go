// Package ebuild implements ebuild execution engine.
//
// This file contains tests for CMake build system support.
package ebuild

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// ============================================================================
// Test Utilities for CMake
// ============================================================================

// createCmakeTestHelpers creates a Helpers instance with CMake-ready environment.
func createCmakeTestHelpers(t *testing.T) (*Helpers, string, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	// Create temporary directories
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	sourceDir := filepath.Join(workDir, "test-1.0.0")
	buildDir := filepath.Join(workDir, "test-1.0.0_build")
	imageDir := filepath.Join(tmpDir, "image")

	for _, dir := range []string{workDir, sourceDir, buildDir, imageDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	testPkg := &pkg.Package{
		Name:    "dev-libs/test",
		Version: "1.0.0",
		Slot:    pkg.Slot{Name: "0"},
		UseFlags: map[string]bool{
			"ssl": true,
		},
	}

	env, err := NewEnvironment(testPkg, tmpDir, "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	// Override directories for testing
	env.WORKDIR = workDir
	env.S = sourceDir
	env.D = imageDir
	env.MAKEOPTS = "-j4"

	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(env, &stdout, &stderr)

	return helpers, tmpDir, &stdout, &stderr
}

// ============================================================================
// CMake Generator Tests
// ============================================================================

func TestHelpers_getCmakeGenerator_Default(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	// Clear CMAKE_MAKEFILE_GENERATOR
	origGen := os.Getenv("CMAKE_MAKEFILE_GENERATOR")
	_ = os.Unsetenv("CMAKE_MAKEFILE_GENERATOR")
	defer func() { _ = os.Setenv("CMAKE_MAKEFILE_GENERATOR", origGen) }()

	gen := helpers.getCmakeGenerator()
	if gen != CMakeGeneratorNinja {
		t.Errorf("getCmakeGenerator() = %q, want %q", gen, CMakeGeneratorNinja)
	}
}

func TestHelpers_getCmakeGenerator_FromEnv(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	testCases := []struct {
		envValue string
		expected string
	}{
		{"Ninja", CMakeGeneratorNinja},
		{"ninja", CMakeGeneratorNinja},
		{"Unix Makefiles", CMakeGeneratorUnixMakefiles},
		{"emake", CMakeGeneratorUnixMakefiles},
		{"make", CMakeGeneratorUnixMakefiles},
		{"Custom Generator", "Custom Generator"},
	}

	for _, tc := range testCases {
		t.Run(tc.envValue, func(t *testing.T) {
			origGen := os.Getenv("CMAKE_MAKEFILE_GENERATOR")
			_ = os.Setenv("CMAKE_MAKEFILE_GENERATOR", tc.envValue)
			defer func() { _ = os.Setenv("CMAKE_MAKEFILE_GENERATOR", origGen) }()

			gen := helpers.getCmakeGenerator()
			if gen != tc.expected {
				t.Errorf("getCmakeGenerator() with %q = %q, want %q", tc.envValue, gen, tc.expected)
			}
		})
	}
}

// ============================================================================
// CMake Build Type Tests
// ============================================================================

func TestHelpers_getCmakeBuildType_Default(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	// Clear CMAKE_BUILD_TYPE
	origType := os.Getenv("CMAKE_BUILD_TYPE")
	_ = os.Unsetenv("CMAKE_BUILD_TYPE")
	defer func() { _ = os.Setenv("CMAKE_BUILD_TYPE", origType) }()

	buildType := helpers.getCmakeBuildType()
	if buildType != CMakeBuildTypeRelease {
		t.Errorf("getCmakeBuildType() = %q, want %q", buildType, CMakeBuildTypeRelease)
	}
}

func TestHelpers_getCmakeBuildType_FromEnv(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	testCases := []struct {
		envValue string
		expected string
	}{
		{CMakeBuildTypeDebug, CMakeBuildTypeDebug},
		{CMakeBuildTypeRelease, CMakeBuildTypeRelease},
		{CMakeBuildTypeRelWithDebInfo, CMakeBuildTypeRelWithDebInfo},
		{CMakeBuildTypeMinSizeRel, CMakeBuildTypeMinSizeRel},
	}

	for _, tc := range testCases {
		t.Run(tc.envValue, func(t *testing.T) {
			origType := os.Getenv("CMAKE_BUILD_TYPE")
			_ = os.Setenv("CMAKE_BUILD_TYPE", tc.envValue)
			defer func() { _ = os.Setenv("CMAKE_BUILD_TYPE", origType) }()

			buildType := helpers.getCmakeBuildType()
			if buildType != tc.expected {
				t.Errorf("getCmakeBuildType() = %q, want %q", buildType, tc.expected)
			}
		})
	}
}

// ============================================================================
// CMake Directory Tests
// ============================================================================

func TestHelpers_getCmakeUseDir_Default(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	// Clear CMAKE_USE_DIR
	origDir := os.Getenv("CMAKE_USE_DIR")
	_ = os.Unsetenv("CMAKE_USE_DIR")
	defer func() { _ = os.Setenv("CMAKE_USE_DIR", origDir) }()

	useDir := helpers.getCmakeUseDir()
	if useDir != helpers.env.S {
		t.Errorf("getCmakeUseDir() = %q, want %q", useDir, helpers.env.S)
	}
}

func TestHelpers_getCmakeUseDir_FromEnv(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	customDir := "/custom/source/dir"
	origDir := os.Getenv("CMAKE_USE_DIR")
	_ = os.Setenv("CMAKE_USE_DIR", customDir)
	defer func() { _ = os.Setenv("CMAKE_USE_DIR", origDir) }()

	useDir := helpers.getCmakeUseDir()
	if useDir != customDir {
		t.Errorf("getCmakeUseDir() = %q, want %q", useDir, customDir)
	}
}

func TestHelpers_getCmakeBuildDir_Default(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	// Clear BUILD_DIR
	origDir := os.Getenv("BUILD_DIR")
	_ = os.Unsetenv("BUILD_DIR")
	defer func() { _ = os.Setenv("BUILD_DIR", origDir) }()

	buildDir := helpers.getCmakeBuildDir()
	expected := filepath.Join(helpers.env.WORKDIR, helpers.env.P+"_build")
	if buildDir != expected {
		t.Errorf("getCmakeBuildDir() = %q, want %q", buildDir, expected)
	}
}

func TestHelpers_getCmakeBuildDir_FromEnv(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	customDir := "/custom/build/dir"
	origDir := os.Getenv("BUILD_DIR")
	_ = os.Setenv("BUILD_DIR", customDir)
	defer func() { _ = os.Setenv("BUILD_DIR", origDir) }()

	buildDir := helpers.getCmakeBuildDir()
	if buildDir != customDir {
		t.Errorf("getCmakeBuildDir() = %q, want %q", buildDir, customDir)
	}
}

// ============================================================================
// buildCmakeArgs Tests
// ============================================================================

func TestHelpers_buildCmakeArgs_StandardOptions(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	// Clear environment variables for deterministic test
	origVars := make(map[string]string)
	for _, v := range []string{"CC", "CXX", "CMAKE_BUILD_TYPE", "CMAKE_MAKEFILE_GENERATOR", "CMAKE_VERBOSE"} {
		origVars[v] = os.Getenv(v)
		_ = os.Unsetenv(v)
	}
	defer func() {
		for k, v := range origVars {
			_ = os.Setenv(k, v)
		}
	}()

	args := helpers.buildCmakeArgs()

	// Check for required arguments
	requiredPrefixes := []string{
		"-G",
		"-DCMAKE_INSTALL_PREFIX=",
		"-DCMAKE_BUILD_TYPE=",
		"-DCMAKE_INSTALL_LIBDIR=",
		"-DCMAKE_INSTALL_MANDIR=",
		"-DCMAKE_INSTALL_INFODIR=",
		"-DCMAKE_C_COMPILER=",
		"-DCMAKE_CXX_COMPILER=",
	}

	for _, prefix := range requiredPrefixes {
		found := false
		for _, arg := range args {
			if strings.HasPrefix(arg, prefix) || arg == prefix {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("buildCmakeArgs() missing argument starting with %q", prefix)
		}
	}
}

func TestHelpers_buildCmakeArgs_Generator(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	origGen := os.Getenv("CMAKE_MAKEFILE_GENERATOR")
	_ = os.Unsetenv("CMAKE_MAKEFILE_GENERATOR")
	defer func() { _ = os.Setenv("CMAKE_MAKEFILE_GENERATOR", origGen) }()

	args := helpers.buildCmakeArgs()

	// Find the -G argument and its value
	for i, arg := range args {
		if arg == "-G" && i+1 < len(args) {
			if args[i+1] != CMakeGeneratorNinja {
				t.Errorf("buildCmakeArgs() generator = %q, want %q", args[i+1], CMakeGeneratorNinja)
			}
			return
		}
	}
	t.Error("buildCmakeArgs() missing -G argument")
}

func TestHelpers_buildCmakeArgs_Compilers(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	// Set custom compilers
	origCC := os.Getenv("CC")
	origCXX := os.Getenv("CXX")
	_ = os.Setenv("CC", "custom-gcc")
	_ = os.Setenv("CXX", "custom-g++")
	defer func() {
		_ = os.Setenv("CC", origCC)
		_ = os.Setenv("CXX", origCXX)
	}()

	args := helpers.buildCmakeArgs()

	foundCC := false
	foundCXX := false
	for _, arg := range args {
		if arg == "-DCMAKE_C_COMPILER=custom-gcc" {
			foundCC = true
		}
		if arg == "-DCMAKE_CXX_COMPILER=custom-g++" {
			foundCXX = true
		}
	}

	if !foundCC {
		t.Error("buildCmakeArgs() missing custom CC compiler")
	}
	if !foundCXX {
		t.Error("buildCmakeArgs() missing custom CXX compiler")
	}
}

func TestHelpers_buildCmakeArgs_Verbose(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	// Test with CMAKE_VERBOSE=1
	origVerbose := os.Getenv("CMAKE_VERBOSE")
	_ = os.Setenv("CMAKE_VERBOSE", "1")
	defer func() { _ = os.Setenv("CMAKE_VERBOSE", origVerbose) }()

	args := helpers.buildCmakeArgs()

	found := false
	for _, arg := range args {
		if arg == "-DCMAKE_VERBOSE_MAKEFILE=ON" {
			found = true
			break
		}
	}

	if !found {
		t.Error("buildCmakeArgs() with CMAKE_VERBOSE=1 should include -DCMAKE_VERBOSE_MAKEFILE=ON")
	}
}

// ============================================================================
// extractParallelJobs Tests
// ============================================================================

func TestHelpers_extractParallelJobs(t *testing.T) {
	testCases := []struct {
		name     string
		makeopts string
		want     int // 0 means no specific number expected, but > 0
	}{
		{"separate -j 4", "-j 4", 4},
		{"combined -j4", "-j4", 4},
		{"with other options", "-j8 -l5", 8},
		{"no -j", "-l5", 0}, // Will return NumCPU
		{"empty", "", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			helpers, _, _, _ := createCmakeTestHelpers(t)
			helpers.env.MAKEOPTS = tc.makeopts

			got := helpers.extractParallelJobs()

			if tc.makeopts == "" {
				if got != 0 {
					t.Errorf("extractParallelJobs() with empty MAKEOPTS = %d, want 0", got)
				}
			} else if tc.want > 0 {
				if got != tc.want {
					t.Errorf("extractParallelJobs() = %d, want %d", got, tc.want)
				}
			} else if tc.makeopts != "" && got == 0 {
				// For non-empty MAKEOPTS without -j, should return NumCPU
				t.Errorf("extractParallelJobs() with MAKEOPTS=%q should return > 0", tc.makeopts)
			}
		})
	}
}

// ============================================================================
// CmakeSrcConfigure Tests
// ============================================================================

func TestHelpers_CmakeSrcConfigure_NoBuildDir(t *testing.T) {
	helpers, _, stdout, _ := createCmakeTestHelpers(t)

	// CmakeSrcConfigure should create build directory
	err := helpers.CmakeSrcConfigure(nil)

	// Will fail because cmake is not available, but should try
	if err == nil {
		t.Log("cmake_src_configure succeeded (cmake is available)")
	} else {
		// Check that we got past directory creation
		output := stdout.String()
		if strings.Contains(output, "cmake_src_configure") {
			t.Log("cmake_src_configure started correctly")
		}
	}
}

func TestHelpers_CmakeSrcConfigure_WithMycmakeargs(t *testing.T) {
	helpers, _, stdout, _ := createCmakeTestHelpers(t)

	// Set MYCMAKEARGS
	origArgs := os.Getenv("MYCMAKEARGS")
	_ = os.Setenv("MYCMAKEARGS", "-DENABLE_FEATURE=ON -DDISABLE_TESTS=OFF")
	defer func() { _ = os.Setenv("MYCMAKEARGS", origArgs) }()

	// Will fail because cmake is not available, but check output
	_ = helpers.CmakeSrcConfigure(nil)

	output := stdout.String()
	if !strings.Contains(output, "cmake_src_configure") {
		t.Error("expected cmake_src_configure message in output")
	}
}

// ============================================================================
// CmakeSrcCompile Tests
// ============================================================================

func TestHelpers_CmakeSrcCompile_NoBuildDir(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	// Remove build directory
	buildDir := helpers.getCmakeBuildDir()
	_ = os.RemoveAll(buildDir)

	err := helpers.CmakeSrcCompile(nil)
	if err == nil {
		t.Error("expected error when build directory doesn't exist")
	}

	var dieErr *DieError
	if errors.As(err, &dieErr) {
		if !strings.Contains(dieErr.Message, "build directory does not exist") {
			t.Errorf("unexpected error message: %s", dieErr.Message)
		}
	}
}

func TestHelpers_CmakeSrcCompile_WithBuildDir(t *testing.T) {
	helpers, _, stdout, _ := createCmakeTestHelpers(t)

	// Create build directory
	buildDir := helpers.getCmakeBuildDir()
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("failed to create build dir: %v", err)
	}

	// Will fail because cmake is not configured, but check the command
	_ = helpers.CmakeSrcCompile(nil)

	output := stdout.String()
	if !strings.Contains(output, "cmake_src_compile") {
		t.Error("expected cmake_src_compile message in output")
	}
}

// ============================================================================
// CmakeSrcInstall Tests
// ============================================================================

func TestHelpers_CmakeSrcInstall_NoBuildDir(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	// Remove build directory
	buildDir := helpers.getCmakeBuildDir()
	_ = os.RemoveAll(buildDir)

	err := helpers.CmakeSrcInstall(nil)
	if err == nil {
		t.Error("expected error when build directory doesn't exist")
	}

	var dieErr *DieError
	if errors.As(err, &dieErr) {
		if !strings.Contains(dieErr.Message, "build directory does not exist") {
			t.Errorf("unexpected error message: %s", dieErr.Message)
		}
	}
}

func TestHelpers_CmakeSrcInstall_NoDestDir(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	// Create build directory
	buildDir := helpers.getCmakeBuildDir()
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("failed to create build dir: %v", err)
	}

	// Clear D
	helpers.env.D = ""

	err := helpers.CmakeSrcInstall(nil)
	if err == nil {
		t.Error("expected error when D is not set")
	}

	var dieErr *DieError
	if errors.As(err, &dieErr) {
		if !strings.Contains(dieErr.Message, "destination directory") {
			t.Errorf("unexpected error message: %s", dieErr.Message)
		}
	}
}

func TestHelpers_CmakeSrcInstall_WithDirs(t *testing.T) {
	helpers, _, stdout, _ := createCmakeTestHelpers(t)

	// Create build directory
	buildDir := helpers.getCmakeBuildDir()
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("failed to create build dir: %v", err)
	}

	// Will fail because cmake is not configured, but check the command
	_ = helpers.CmakeSrcInstall(nil)

	output := stdout.String()
	if !strings.Contains(output, "cmake_src_install") {
		t.Error("expected cmake_src_install message in output")
	}
	if !strings.Contains(output, "DESTDIR=") {
		t.Error("expected DESTDIR in output")
	}
}

// ============================================================================
// CmakeSrcTest Tests
// ============================================================================

func TestHelpers_CmakeSrcTest_NoBuildDir(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	// Remove build directory
	buildDir := helpers.getCmakeBuildDir()
	_ = os.RemoveAll(buildDir)

	err := helpers.CmakeSrcTest(nil)
	if err == nil {
		t.Error("expected error when build directory doesn't exist")
	}

	var dieErr *DieError
	if errors.As(err, &dieErr) {
		if !strings.Contains(dieErr.Message, "build directory does not exist") {
			t.Errorf("unexpected error message: %s", dieErr.Message)
		}
	}
}

func TestHelpers_CmakeSrcTest_WithBuildDir(t *testing.T) {
	helpers, _, stdout, _ := createCmakeTestHelpers(t)

	// Create build directory
	buildDir := helpers.getCmakeBuildDir()
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("failed to create build dir: %v", err)
	}

	// Will fail because ctest is not available or no tests configured
	_ = helpers.CmakeSrcTest(nil)

	output := stdout.String()
	if !strings.Contains(output, "cmake_src_test") {
		t.Error("expected cmake_src_test message in output")
	}
}

// ============================================================================
// Eninja Tests
// ============================================================================

func TestHelpers_Eninja_NoBuildDir(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	// Remove build directory
	buildDir := helpers.getCmakeBuildDir()
	_ = os.RemoveAll(buildDir)

	err := helpers.Eninja(nil)
	if err == nil {
		t.Error("expected error when build directory doesn't exist")
	}

	var dieErr *DieError
	if errors.As(err, &dieErr) {
		if !strings.Contains(dieErr.Message, "build directory does not exist") {
			t.Errorf("unexpected error message: %s", dieErr.Message)
		}
	}
}

func TestHelpers_Eninja_WithBuildDir(t *testing.T) {
	helpers, _, stdout, _ := createCmakeTestHelpers(t)

	// Create build directory
	buildDir := helpers.getCmakeBuildDir()
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("failed to create build dir: %v", err)
	}

	// Will fail because ninja is not available or not configured
	_ = helpers.Eninja(nil)

	output := stdout.String()
	if !strings.Contains(output, "ninja") {
		t.Error("expected ninja command in output")
	}
}

func TestHelpers_Eninja_WithTargets(t *testing.T) {
	helpers, _, stdout, _ := createCmakeTestHelpers(t)

	// Create build directory
	buildDir := helpers.getCmakeBuildDir()
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("failed to create build dir: %v", err)
	}

	// Call with targets
	_ = helpers.Eninja([]string{"clean", "all"})

	output := stdout.String()
	if !strings.Contains(output, "clean") || !strings.Contains(output, "all") {
		t.Error("expected targets in ninja command output")
	}
}

// ============================================================================
// Cmake Low-Level Tests
// ============================================================================

func TestHelpers_Cmake_CreatesDir(t *testing.T) {
	helpers, tmpDir, _, _ := createCmakeTestHelpers(t)

	// Set a custom build directory that doesn't exist
	customBuildDir := filepath.Join(tmpDir, "custom_build")
	_ = os.Setenv("BUILD_DIR", customBuildDir)
	defer func() { _ = os.Unsetenv("BUILD_DIR") }()

	// Cmake should create the build directory
	_ = helpers.Cmake(nil)

	if _, err := os.Stat(customBuildDir); os.IsNotExist(err) {
		t.Error("Cmake should have created the build directory")
	}
}

// ============================================================================
// runCommandInDir Tests
// ============================================================================

func TestHelpers_runCommandInDir_EmptyDir(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	err := helpers.runCommandInDir("test", nil, "")
	if err == nil {
		t.Error("expected error with empty directory")
	}

	var dieErr *DieError
	if errors.As(err, &dieErr) {
		if !strings.Contains(dieErr.Message, "working directory not set") {
			t.Errorf("unexpected error message: %s", dieErr.Message)
		}
	}
}

func TestHelpers_runCommandInDir_NonexistentDir(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	err := helpers.runCommandInDir("test", nil, "/nonexistent/directory")
	if err == nil {
		t.Error("expected error with nonexistent directory")
	}

	var dieErr *DieError
	if errors.As(err, &dieErr) {
		if !strings.Contains(dieErr.Message, "does not exist") {
			t.Errorf("unexpected error message: %s", dieErr.Message)
		}
	}
}

// ============================================================================
// runCommandWithEnv Tests
// ============================================================================

func TestHelpers_runCommandWithEnv_NonexistentDir(t *testing.T) {
	helpers, _, _, _ := createCmakeTestHelpers(t)

	// Set a nonexistent work directory
	helpers.env.S = "/nonexistent"

	err := helpers.runCommandWithEnv("test", nil, "/nonexistent/directory", nil)
	if err == nil {
		t.Error("expected error with nonexistent directory")
	}
}

// ============================================================================
// Nil Environment Tests
// ============================================================================

func TestHelpers_getCmakeBuildDir_NilEnv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	buildDir := helpers.getCmakeBuildDir()
	if buildDir != "" {
		t.Errorf("getCmakeBuildDir() with nil env = %q, want empty string", buildDir)
	}
}

func TestHelpers_getCmakeUseDir_NilEnv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	useDir := helpers.getCmakeUseDir()
	if useDir != "" {
		t.Errorf("getCmakeUseDir() with nil env = %q, want empty string", useDir)
	}
}

func TestHelpers_getDestDir_NilEnv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	// Clear D env var
	origD := os.Getenv("D")
	_ = os.Unsetenv("D")
	defer func() { _ = os.Setenv("D", origD) }()

	destDir := helpers.getDestDir()
	if destDir != "" {
		t.Errorf("getDestDir() with nil env = %q, want empty string", destDir)
	}
}

func TestHelpers_getEprefix_NilEnv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	// Clear EPREFIX env var
	origEprefix := os.Getenv("EPREFIX")
	_ = os.Unsetenv("EPREFIX")
	defer func() { _ = os.Setenv("EPREFIX", origEprefix) }()

	eprefix := helpers.getEprefix()
	if eprefix != "" {
		t.Errorf("getEprefix() with nil env = %q, want empty string", eprefix)
	}
}

// ============================================================================
// Constants Tests
// ============================================================================

func TestCmakeConstants(t *testing.T) {
	// Verify constants are properly defined
	if CMakeGeneratorNinja != "Ninja" {
		t.Errorf("CMakeGeneratorNinja = %q, want %q", CMakeGeneratorNinja, "Ninja")
	}
	if CMakeGeneratorUnixMakefiles != "Unix Makefiles" {
		t.Errorf("CMakeGeneratorUnixMakefiles = %q, want %q", CMakeGeneratorUnixMakefiles, "Unix Makefiles")
	}
	if CMakeBuildTypeRelease != "Release" {
		t.Errorf("CMakeBuildTypeRelease = %q, want %q", CMakeBuildTypeRelease, "Release")
	}
	if CMakeBuildTypeDebug != "Debug" {
		t.Errorf("CMakeBuildTypeDebug = %q, want %q", CMakeBuildTypeDebug, "Debug")
	}
	if CMakeBuildTypeRelWithDebInfo != "RelWithDebInfo" {
		t.Errorf("CMakeBuildTypeRelWithDebInfo = %q, want %q", CMakeBuildTypeRelWithDebInfo, "RelWithDebInfo")
	}
	if CMakeBuildTypeMinSizeRel != "MinSizeRel" {
		t.Errorf("CMakeBuildTypeMinSizeRel = %q, want %q", CMakeBuildTypeMinSizeRel, "MinSizeRel")
	}
}

// ============================================================================
// Integration-style Tests
// ============================================================================

func TestHelpers_CmakeWorkflow_DirectorySetup(t *testing.T) {
	helpers, tmpDir, _, _ := createCmakeTestHelpers(t)

	// Verify directory structure is set up correctly
	sourceDir := helpers.getCmakeUseDir()
	buildDir := helpers.getCmakeBuildDir()

	if sourceDir == "" {
		t.Error("getCmakeUseDir() returned empty string")
	}
	if buildDir == "" {
		t.Error("getCmakeBuildDir() returned empty string")
	}

	// Source and build directories should be different
	if sourceDir == buildDir {
		t.Error("source and build directories should be different for out-of-tree builds")
	}

	// Both should be under the temp directory
	if !strings.HasPrefix(sourceDir, tmpDir) {
		t.Errorf("source directory %q not under temp dir %q", sourceDir, tmpDir)
	}
}
