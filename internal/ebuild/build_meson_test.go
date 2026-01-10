// Package ebuild implements ebuild execution engine.
//
// This file contains tests for Meson build system support.
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
// Test Utilities for Meson
// ============================================================================

// createMesonTestHelpers creates a Helpers instance with Meson-ready environment.
func createMesonTestHelpers(t *testing.T) (*Helpers, string, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	// Create temporary directories
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	sourceDir := filepath.Join(workDir, "test-1.0.0")
	buildDir := filepath.Join(workDir, "test-1.0.0-build") // Note: meson uses "-build" suffix
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
			"ssl":     true,
			"doc":     false,
			"gtk":     true,
			"systemd": false,
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
// Meson Constants Tests
// ============================================================================

func TestMesonConstants(t *testing.T) {
	// Verify build type constants
	testCases := []struct {
		constant string
		expected string
	}{
		{MesonBuildTypePlain, "plain"},
		{MesonBuildTypeDebug, "debug"},
		{MesonBuildTypeDebugOptimized, "debugoptimized"},
		{MesonBuildTypeRelease, "release"},
		{MesonBuildTypeMinSize, "minsize"},
		{MesonWrapModeNoDownload, "nodownload"},
		{MesonWrapModeNone, "nofallback"},
		{MesonFeatureEnabled, "enabled"},
		{MesonFeatureDisabled, "disabled"},
		{MesonFeatureAuto, "auto"},
	}

	for _, tc := range testCases {
		if tc.constant != tc.expected {
			t.Errorf("constant = %q, want %q", tc.constant, tc.expected)
		}
	}
}

// ============================================================================
// Meson Build Type Tests
// ============================================================================

func TestHelpers_getMesonBuildType_Default(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Clear EMESON_BUILDTYPE
	origType := os.Getenv("EMESON_BUILDTYPE")
	_ = os.Unsetenv("EMESON_BUILDTYPE")
	defer func() { _ = os.Setenv("EMESON_BUILDTYPE", origType) }()

	buildType := helpers.getMesonBuildType()
	if buildType != MesonBuildTypePlain {
		t.Errorf("getMesonBuildType() = %q, want %q", buildType, MesonBuildTypePlain)
	}
}

func TestHelpers_getMesonBuildType_FromEnv(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	testCases := []struct {
		envValue string
		expected string
	}{
		{MesonBuildTypePlain, MesonBuildTypePlain},
		{MesonBuildTypeDebug, MesonBuildTypeDebug},
		{MesonBuildTypeRelease, MesonBuildTypeRelease},
		{MesonBuildTypeDebugOptimized, MesonBuildTypeDebugOptimized},
		{MesonBuildTypeMinSize, MesonBuildTypeMinSize},
	}

	for _, tc := range testCases {
		t.Run(tc.envValue, func(t *testing.T) {
			origType := os.Getenv("EMESON_BUILDTYPE")
			_ = os.Setenv("EMESON_BUILDTYPE", tc.envValue)
			defer func() { _ = os.Setenv("EMESON_BUILDTYPE", origType) }()

			buildType := helpers.getMesonBuildType()
			if buildType != tc.expected {
				t.Errorf("getMesonBuildType() = %q, want %q", buildType, tc.expected)
			}
		})
	}
}

// ============================================================================
// Meson Wrap Mode Tests
// ============================================================================

func TestHelpers_getMesonWrapMode_Default(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Clear EMESON_WRAP_MODE
	origMode := os.Getenv("EMESON_WRAP_MODE")
	_ = os.Unsetenv("EMESON_WRAP_MODE")
	defer func() { _ = os.Setenv("EMESON_WRAP_MODE", origMode) }()

	wrapMode := helpers.getMesonWrapMode()
	if wrapMode != MesonWrapModeNoDownload {
		t.Errorf("getMesonWrapMode() = %q, want %q", wrapMode, MesonWrapModeNoDownload)
	}
}

func TestHelpers_getMesonWrapMode_FromEnv(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	origMode := os.Getenv("EMESON_WRAP_MODE")
	_ = os.Setenv("EMESON_WRAP_MODE", "nofallback")
	defer func() { _ = os.Setenv("EMESON_WRAP_MODE", origMode) }()

	wrapMode := helpers.getMesonWrapMode()
	if wrapMode != "nofallback" {
		t.Errorf("getMesonWrapMode() = %q, want %q", wrapMode, "nofallback")
	}
}

// ============================================================================
// Meson Directory Tests
// ============================================================================

func TestHelpers_getMesonSource_Default(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Clear EMESON_SOURCE
	origDir := os.Getenv("EMESON_SOURCE")
	_ = os.Unsetenv("EMESON_SOURCE")
	defer func() { _ = os.Setenv("EMESON_SOURCE", origDir) }()

	sourceDir := helpers.getMesonSource()
	if sourceDir != helpers.env.S {
		t.Errorf("getMesonSource() = %q, want %q", sourceDir, helpers.env.S)
	}
}

func TestHelpers_getMesonSource_FromEnv(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	customDir := "/custom/source/dir"
	origDir := os.Getenv("EMESON_SOURCE")
	_ = os.Setenv("EMESON_SOURCE", customDir)
	defer func() { _ = os.Setenv("EMESON_SOURCE", origDir) }()

	sourceDir := helpers.getMesonSource()
	if sourceDir != customDir {
		t.Errorf("getMesonSource() = %q, want %q", sourceDir, customDir)
	}
}

func TestHelpers_getMesonBuildDir_Default(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Clear BUILD_DIR
	origDir := os.Getenv("BUILD_DIR")
	_ = os.Unsetenv("BUILD_DIR")
	defer func() { _ = os.Setenv("BUILD_DIR", origDir) }()

	buildDir := helpers.getMesonBuildDir()
	expected := filepath.Join(helpers.env.WORKDIR, helpers.env.P+"-build")
	if buildDir != expected {
		t.Errorf("getMesonBuildDir() = %q, want %q", buildDir, expected)
	}
}

func TestHelpers_getMesonBuildDir_FromEnv(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	customDir := "/custom/build/dir"
	origDir := os.Getenv("BUILD_DIR")
	_ = os.Setenv("BUILD_DIR", customDir)
	defer func() { _ = os.Setenv("BUILD_DIR", origDir) }()

	buildDir := helpers.getMesonBuildDir()
	if buildDir != customDir {
		t.Errorf("getMesonBuildDir() = %q, want %q", buildDir, customDir)
	}
}

func TestHelpers_getMesonBuildDir_NilEnv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	buildDir := helpers.getMesonBuildDir()
	if buildDir != "" {
		t.Errorf("getMesonBuildDir() with nil env = %q, want empty string", buildDir)
	}
}

func TestHelpers_getMesonSource_NilEnv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	sourceDir := helpers.getMesonSource()
	if sourceDir != "" {
		t.Errorf("getMesonSource() with nil env = %q, want empty string", sourceDir)
	}
}

// ============================================================================
// buildMesonArgs Tests
// ============================================================================

func TestHelpers_buildMesonArgs_StandardOptions(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Clear environment variables for deterministic test
	origVars := make(map[string]string)
	for _, v := range []string{"EMESON_BUILDTYPE", "EMESON_WRAP_MODE", "CHOST", "CBUILD"} {
		origVars[v] = os.Getenv(v)
		_ = os.Unsetenv(v)
	}
	defer func() {
		for k, v := range origVars {
			_ = os.Setenv(k, v)
		}
	}()

	args := helpers.buildMesonArgs()

	// Check for required arguments
	requiredPrefixes := []string{
		"--prefix=",
		"--libdir=",
		"--localstatedir=",
		"--sysconfdir=",
		"--buildtype=",
		"--wrap-mode=",
		"--backend=",
		"--mandir=",
		"--infodir=",
		"--datadir=",
		"--bindir=",
		"--sbindir=",
		"--includedir=",
		"--libexecdir=",
	}

	for _, prefix := range requiredPrefixes {
		found := false
		for _, arg := range args {
			if strings.HasPrefix(arg, prefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("buildMesonArgs() missing argument starting with %q", prefix)
		}
	}
}

func TestHelpers_buildMesonArgs_NinjaBackend(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	args := helpers.buildMesonArgs()

	found := false
	for _, arg := range args {
		if arg == "--backend=ninja" {
			found = true
			break
		}
	}

	if !found {
		t.Error("buildMesonArgs() should include --backend=ninja")
	}
}

func TestHelpers_buildMesonArgs_WrapMode(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Clear wrap mode env var
	origMode := os.Getenv("EMESON_WRAP_MODE")
	_ = os.Unsetenv("EMESON_WRAP_MODE")
	defer func() { _ = os.Setenv("EMESON_WRAP_MODE", origMode) }()

	args := helpers.buildMesonArgs()

	found := false
	for _, arg := range args {
		if arg == "--wrap-mode=nodownload" {
			found = true
			break
		}
	}

	if !found {
		t.Error("buildMesonArgs() should include --wrap-mode=nodownload")
	}
}

func TestHelpers_buildMesonArgs_BuildType(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Clear build type env var
	origType := os.Getenv("EMESON_BUILDTYPE")
	_ = os.Unsetenv("EMESON_BUILDTYPE")
	defer func() { _ = os.Setenv("EMESON_BUILDTYPE", origType) }()

	args := helpers.buildMesonArgs()

	found := false
	for _, arg := range args {
		if arg == "--buildtype=plain" {
			found = true
			break
		}
	}

	if !found {
		t.Error("buildMesonArgs() should include --buildtype=plain")
	}
}

// ============================================================================
// MesonSrcConfigure Tests
// ============================================================================

func TestHelpers_MesonSrcConfigure_Output(t *testing.T) {
	helpers, _, stdout, _ := createMesonTestHelpers(t)

	// Will fail because meson is not available, but check output
	_ = helpers.MesonSrcConfigure(nil)

	output := stdout.String()
	if !strings.Contains(output, "meson_src_configure") {
		t.Error("expected meson_src_configure message in output")
	}
}

func TestHelpers_MesonSrcConfigure_WithMymesonargs(t *testing.T) {
	helpers, _, stdout, _ := createMesonTestHelpers(t)

	// Set MYMESONARGS
	origArgs := os.Getenv("MYMESONARGS")
	_ = os.Setenv("MYMESONARGS", "-Denable_feature=true -Ddisable_tests=false")
	defer func() { _ = os.Setenv("MYMESONARGS", origArgs) }()

	// Will fail because meson is not available, but check output
	_ = helpers.MesonSrcConfigure(nil)

	output := stdout.String()
	if !strings.Contains(output, "meson_src_configure") {
		t.Error("expected meson_src_configure message in output")
	}
}

func TestHelpers_MesonSrcConfigure_WithEmesonargs(t *testing.T) {
	helpers, _, stdout, _ := createMesonTestHelpers(t)

	// Set emesonargs via ExtraVars
	helpers.env.SetVar("emesonargs", "-Dfeature=enabled")

	_ = helpers.MesonSrcConfigure(nil)

	output := stdout.String()
	if !strings.Contains(output, "meson_src_configure") {
		t.Error("expected meson_src_configure message in output")
	}
}

// ============================================================================
// MesonSrcCompile Tests
// ============================================================================

func TestHelpers_MesonSrcCompile_NoBuildDir(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Remove build directory
	buildDir := helpers.getMesonBuildDir()
	_ = os.RemoveAll(buildDir)

	err := helpers.MesonSrcCompile(nil)
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

func TestHelpers_MesonSrcCompile_WithBuildDir(t *testing.T) {
	helpers, _, stdout, _ := createMesonTestHelpers(t)

	// Create build directory
	buildDir := helpers.getMesonBuildDir()
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("failed to create build dir: %v", err)
	}

	// Will fail because meson is not configured, but check the command
	_ = helpers.MesonSrcCompile(nil)

	output := stdout.String()
	if !strings.Contains(output, "meson_src_compile") {
		t.Error("expected meson_src_compile message in output")
	}
}

func TestHelpers_MesonSrcCompile_VerboseMode(t *testing.T) {
	helpers, _, stdout, _ := createMesonTestHelpers(t)

	// Create build directory
	buildDir := helpers.getMesonBuildDir()
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("failed to create build dir: %v", err)
	}

	// Enable verbose mode
	origVerbose := os.Getenv("MESON_VERBOSE")
	_ = os.Setenv("MESON_VERBOSE", "1")
	defer func() { _ = os.Setenv("MESON_VERBOSE", origVerbose) }()

	_ = helpers.MesonSrcCompile(nil)

	output := stdout.String()
	if !strings.Contains(output, "-v") {
		t.Error("expected -v flag in output when MESON_VERBOSE=1")
	}
}

// ============================================================================
// MesonSrcInstall Tests
// ============================================================================

func TestHelpers_MesonSrcInstall_NoBuildDir(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Remove build directory
	buildDir := helpers.getMesonBuildDir()
	_ = os.RemoveAll(buildDir)

	err := helpers.MesonSrcInstall(nil)
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

func TestHelpers_MesonSrcInstall_NoDestDir(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Create build directory
	buildDir := helpers.getMesonBuildDir()
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("failed to create build dir: %v", err)
	}

	// Clear D
	helpers.env.D = ""

	err := helpers.MesonSrcInstall(nil)
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

func TestHelpers_MesonSrcInstall_WithDirs(t *testing.T) {
	helpers, _, stdout, _ := createMesonTestHelpers(t)

	// Create build directory
	buildDir := helpers.getMesonBuildDir()
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("failed to create build dir: %v", err)
	}

	// Will fail because meson is not configured, but check the command
	_ = helpers.MesonSrcInstall(nil)

	output := stdout.String()
	if !strings.Contains(output, "meson_src_install") {
		t.Error("expected meson_src_install message in output")
	}
	if !strings.Contains(output, "--destdir") {
		t.Error("expected --destdir in output")
	}
	if !strings.Contains(output, "--no-rebuild") {
		t.Error("expected --no-rebuild in output")
	}
}

// ============================================================================
// MesonSrcTest Tests
// ============================================================================

func TestHelpers_MesonSrcTest_NoBuildDir(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Remove build directory
	buildDir := helpers.getMesonBuildDir()
	_ = os.RemoveAll(buildDir)

	err := helpers.MesonSrcTest(nil)
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

func TestHelpers_MesonSrcTest_WithBuildDir(t *testing.T) {
	helpers, _, stdout, _ := createMesonTestHelpers(t)

	// Create build directory
	buildDir := helpers.getMesonBuildDir()
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("failed to create build dir: %v", err)
	}

	// Will fail because meson test requires configured project
	_ = helpers.MesonSrcTest(nil)

	output := stdout.String()
	if !strings.Contains(output, "meson_src_test") {
		t.Error("expected meson_src_test message in output")
	}
}

func TestHelpers_MesonSrcTest_Flags(t *testing.T) {
	helpers, _, stdout, _ := createMesonTestHelpers(t)

	// Create build directory
	buildDir := helpers.getMesonBuildDir()
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("failed to create build dir: %v", err)
	}

	_ = helpers.MesonSrcTest(nil)

	output := stdout.String()
	if !strings.Contains(output, "--no-rebuild") {
		t.Error("expected --no-rebuild in output")
	}
	if !strings.Contains(output, "--print-errorlogs") {
		t.Error("expected --print-errorlogs in output")
	}
}

// ============================================================================
// MesonUse Tests
// ============================================================================

func TestHelpers_MesonUse_Enabled(t *testing.T) {
	helpers, _, stdout, _ := createMesonTestHelpers(t)

	// ssl is enabled in our test package
	err := helpers.MesonUse([]string{"ssl"})
	if err != nil {
		t.Fatalf("MesonUse failed: %v", err)
	}

	output := stdout.String()
	if output != "-Dssl=enabled" {
		t.Errorf("MesonUse(ssl) = %q, want %q", output, "-Dssl=enabled")
	}
}

func TestHelpers_MesonUse_Disabled(t *testing.T) {
	helpers, _, stdout, _ := createMesonTestHelpers(t)

	// doc is disabled in our test package
	err := helpers.MesonUse([]string{"doc"})
	if err != nil {
		t.Fatalf("MesonUse failed: %v", err)
	}

	output := stdout.String()
	if output != "-Ddoc=disabled" {
		t.Errorf("MesonUse(doc) = %q, want %q", output, "-Ddoc=disabled")
	}
}

func TestHelpers_MesonUse_CustomOption(t *testing.T) {
	helpers, _, stdout, _ := createMesonTestHelpers(t)

	// ssl -> openssl mapping
	err := helpers.MesonUse([]string{"ssl", "openssl"})
	if err != nil {
		t.Fatalf("MesonUse failed: %v", err)
	}

	output := stdout.String()
	if output != "-Dopenssl=enabled" {
		t.Errorf("MesonUse(ssl, openssl) = %q, want %q", output, "-Dopenssl=enabled")
	}
}

func TestHelpers_MesonUse_MissingArg(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	err := helpers.MesonUse(nil)
	if err == nil {
		t.Error("expected error with no arguments")
	}

	var dieErr *DieError
	if errors.As(err, &dieErr) {
		if !strings.Contains(dieErr.Message, "missing USE flag") {
			t.Errorf("unexpected error message: %s", dieErr.Message)
		}
	}
}

// ============================================================================
// MesonFeature Tests
// ============================================================================

func TestHelpers_MesonFeature_Enabled(t *testing.T) {
	helpers, _, stdout, _ := createMesonTestHelpers(t)

	err := helpers.MesonFeature([]string{"gtk"})
	if err != nil {
		t.Fatalf("MesonFeature failed: %v", err)
	}

	output := stdout.String()
	if output != "-Dgtk=enabled" {
		t.Errorf("MesonFeature(gtk) = %q, want %q", output, "-Dgtk=enabled")
	}
}

func TestHelpers_MesonFeature_Disabled(t *testing.T) {
	helpers, _, stdout, _ := createMesonTestHelpers(t)

	err := helpers.MesonFeature([]string{"systemd"})
	if err != nil {
		t.Fatalf("MesonFeature failed: %v", err)
	}

	output := stdout.String()
	if output != "-Dsystemd=disabled" {
		t.Errorf("MesonFeature(systemd) = %q, want %q", output, "-Dsystemd=disabled")
	}
}

// ============================================================================
// MesonUseBool Tests
// ============================================================================

func TestHelpers_MesonUseBool_True(t *testing.T) {
	helpers, _, stdout, _ := createMesonTestHelpers(t)

	err := helpers.MesonUseBool([]string{"ssl"})
	if err != nil {
		t.Fatalf("MesonUseBool failed: %v", err)
	}

	output := stdout.String()
	if output != "-Dssl=true" {
		t.Errorf("MesonUseBool(ssl) = %q, want %q", output, "-Dssl=true")
	}
}

func TestHelpers_MesonUseBool_False(t *testing.T) {
	helpers, _, stdout, _ := createMesonTestHelpers(t)

	err := helpers.MesonUseBool([]string{"doc"})
	if err != nil {
		t.Fatalf("MesonUseBool failed: %v", err)
	}

	output := stdout.String()
	if output != "-Ddoc=false" {
		t.Errorf("MesonUseBool(doc) = %q, want %q", output, "-Ddoc=false")
	}
}

func TestHelpers_MesonUseBool_CustomOption(t *testing.T) {
	helpers, _, stdout, _ := createMesonTestHelpers(t)

	err := helpers.MesonUseBool([]string{"ssl", "enable_crypto"})
	if err != nil {
		t.Fatalf("MesonUseBool failed: %v", err)
	}

	output := stdout.String()
	if output != "-Denable_crypto=true" {
		t.Errorf("MesonUseBool(ssl, enable_crypto) = %q, want %q", output, "-Denable_crypto=true")
	}
}

func TestHelpers_MesonUseBool_MissingArg(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	err := helpers.MesonUseBool(nil)
	if err == nil {
		t.Error("expected error with no arguments")
	}

	var dieErr *DieError
	if errors.As(err, &dieErr) {
		if !strings.Contains(dieErr.Message, "missing USE flag") {
			t.Errorf("unexpected error message: %s", dieErr.Message)
		}
	}
}

// ============================================================================
// hasUseFlag Tests
// ============================================================================

func TestHelpers_hasUseFlag_FromPackage(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	testCases := []struct {
		flag     string
		expected bool
	}{
		{"ssl", true},
		{"doc", false},
		{"gtk", true},
		{"systemd", false},
		{"nonexistent", false},
	}

	for _, tc := range testCases {
		t.Run(tc.flag, func(t *testing.T) {
			result := helpers.hasUseFlag(tc.flag)
			if result != tc.expected {
				t.Errorf("hasUseFlag(%q) = %v, want %v", tc.flag, result, tc.expected)
			}
		})
	}
}

func TestHelpers_hasUseFlag_FromEnv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	// Set USE environment variable
	origUse := os.Getenv("USE")
	_ = os.Setenv("USE", "ssl doc -gtk")
	defer func() { _ = os.Setenv("USE", origUse) }()

	testCases := []struct {
		flag     string
		expected bool
	}{
		{"ssl", true},
		{"doc", true},
		{"gtk", false},
		{"other", false},
	}

	for _, tc := range testCases {
		t.Run(tc.flag, func(t *testing.T) {
			result := helpers.hasUseFlag(tc.flag)
			if result != tc.expected {
				t.Errorf("hasUseFlag(%q) = %v, want %v", tc.flag, result, tc.expected)
			}
		})
	}
}

// ============================================================================
// Cross-Compilation Tests
// ============================================================================

func TestHelpers_isCrossCompiling_Native(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Clear CBUILD to simulate native compilation
	origCBuild := os.Getenv("CBUILD")
	_ = os.Unsetenv("CBUILD")
	defer func() { _ = os.Setenv("CBUILD", origCBuild) }()

	if helpers.isCrossCompiling() {
		t.Error("isCrossCompiling() should return false when CBUILD is not set")
	}
}

func TestHelpers_isCrossCompiling_Cross(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Set different CHOST and CBUILD
	origCHost := os.Getenv("CHOST")
	origCBuild := os.Getenv("CBUILD")
	_ = os.Setenv("CHOST", "aarch64-unknown-linux-gnu")
	_ = os.Setenv("CBUILD", "x86_64-pc-linux-gnu")
	defer func() {
		_ = os.Setenv("CHOST", origCHost)
		_ = os.Setenv("CBUILD", origCBuild)
	}()

	if !helpers.isCrossCompiling() {
		t.Error("isCrossCompiling() should return true when CHOST != CBUILD")
	}
}

func TestHelpers_isCrossCompiling_Same(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Set same CHOST and CBUILD
	origCHost := os.Getenv("CHOST")
	origCBuild := os.Getenv("CBUILD")
	_ = os.Setenv("CHOST", "x86_64-pc-linux-gnu")
	_ = os.Setenv("CBUILD", "x86_64-pc-linux-gnu")
	defer func() {
		_ = os.Setenv("CHOST", origCHost)
		_ = os.Setenv("CBUILD", origCBuild)
	}()

	if helpers.isCrossCompiling() {
		t.Error("isCrossCompiling() should return false when CHOST == CBUILD")
	}
}

func TestHelpers_generateMesonCrossFile_NativeCompilation(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Clear CBUILD to simulate native compilation
	origCBuild := os.Getenv("CBUILD")
	_ = os.Unsetenv("CBUILD")
	defer func() { _ = os.Setenv("CBUILD", origCBuild) }()

	crossFile := helpers.generateMesonCrossFile()
	if crossFile != "" {
		t.Errorf("generateMesonCrossFile() = %q, want empty for native compilation", crossFile)
	}
}

func TestHelpers_generateMesonCrossFile_CustomFile(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Set cross-compilation environment
	origCHost := os.Getenv("CHOST")
	origCBuild := os.Getenv("CBUILD")
	origCrossFile := os.Getenv("EMESON_CROSS_FILE")
	_ = os.Setenv("CHOST", "aarch64-unknown-linux-gnu")
	_ = os.Setenv("CBUILD", "x86_64-pc-linux-gnu")
	_ = os.Setenv("EMESON_CROSS_FILE", "/custom/cross-file.txt")
	defer func() {
		_ = os.Setenv("CHOST", origCHost)
		_ = os.Setenv("CBUILD", origCBuild)
		_ = os.Setenv("EMESON_CROSS_FILE", origCrossFile)
	}()

	crossFile := helpers.generateMesonCrossFile()
	if crossFile != "/custom/cross-file.txt" {
		t.Errorf("generateMesonCrossFile() = %q, want /custom/cross-file.txt", crossFile)
	}
}

func TestHelpers_generateMesonCrossFile_Generated(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Set cross-compilation environment
	origCHost := os.Getenv("CHOST")
	origCBuild := os.Getenv("CBUILD")
	origCrossFile := os.Getenv("EMESON_CROSS_FILE")
	_ = os.Setenv("CHOST", "aarch64-unknown-linux-gnu")
	_ = os.Setenv("CBUILD", "x86_64-pc-linux-gnu")
	_ = os.Unsetenv("EMESON_CROSS_FILE")
	defer func() {
		_ = os.Setenv("CHOST", origCHost)
		_ = os.Setenv("CBUILD", origCBuild)
		_ = os.Setenv("EMESON_CROSS_FILE", origCrossFile)
	}()

	crossFile := helpers.generateMesonCrossFile()

	// Check that cross-file was generated
	if crossFile == "" {
		t.Error("generateMesonCrossFile() should return a path for cross-compilation")
		return
	}

	// Check that file exists
	if _, err := os.Stat(crossFile); os.IsNotExist(err) {
		t.Errorf("generated cross-file %q does not exist", crossFile)
	}

	// Check file content
	content, err := os.ReadFile(crossFile)
	if err != nil {
		t.Fatalf("failed to read cross-file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "[binaries]") {
		t.Error("cross-file should contain [binaries] section")
	}
	if !strings.Contains(contentStr, "[host_machine]") {
		t.Error("cross-file should contain [host_machine] section")
	}
	if !strings.Contains(contentStr, "cpu_family = 'aarch64'") {
		t.Error("cross-file should contain correct cpu_family")
	}
}

// ============================================================================
// Meson Low-Level Tests
// ============================================================================

func TestHelpers_Meson_CreatesDir(t *testing.T) {
	helpers, tmpDir, _, _ := createMesonTestHelpers(t)

	// Set a custom build directory that doesn't exist
	customBuildDir := filepath.Join(tmpDir, "custom_meson_build")
	_ = os.Setenv("BUILD_DIR", customBuildDir)
	defer func() { _ = os.Unsetenv("BUILD_DIR") }()

	// Meson should create the build directory
	_ = helpers.Meson(nil)

	if _, err := os.Stat(customBuildDir); os.IsNotExist(err) {
		t.Error("Meson should have created the build directory")
	}
}

func TestHelpers_Meson_OutputCommand(t *testing.T) {
	helpers, _, stdout, _ := createMesonTestHelpers(t)

	_ = helpers.Meson(nil)

	output := stdout.String()
	if !strings.Contains(output, "meson setup") {
		t.Error("expected 'meson setup' in output")
	}
}

// ============================================================================
// Integration-style Tests
// ============================================================================

func TestHelpers_MesonWorkflow_DirectorySetup(t *testing.T) {
	helpers, tmpDir, _, _ := createMesonTestHelpers(t)

	// Verify directory structure is set up correctly
	sourceDir := helpers.getMesonSource()
	buildDir := helpers.getMesonBuildDir()

	if sourceDir == "" {
		t.Error("getMesonSource() returned empty string")
	}
	if buildDir == "" {
		t.Error("getMesonBuildDir() returned empty string")
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

func TestHelpers_MesonBuildDirSuffix(t *testing.T) {
	helpers, _, _, _ := createMesonTestHelpers(t)

	// Clear BUILD_DIR to get default
	origDir := os.Getenv("BUILD_DIR")
	_ = os.Unsetenv("BUILD_DIR")
	defer func() { _ = os.Setenv("BUILD_DIR", origDir) }()

	buildDir := helpers.getMesonBuildDir()

	// Meson convention uses "-build" suffix
	if !strings.HasSuffix(buildDir, "-build") {
		t.Errorf("getMesonBuildDir() = %q, should end with '-build'", buildDir)
	}
}
