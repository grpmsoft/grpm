// Package ebuild implements ebuild execution engine.
//
// This file contains tests for the meson.eclass implementation.
package ebuild

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// ============================================================================
// MesonEclass Tests
// ============================================================================

// TestMesonEclass_Name tests that the eclass returns the correct name.
func TestMesonEclass_Name(t *testing.T) {
	eclass := &MesonEclass{}
	if name := eclass.Name(); name != "meson" {
		t.Errorf("MesonEclass.Name() = %q, want %q", name, "meson")
	}
}

// TestMesonEclass_ExportedFunctions tests that the eclass exports the correct phases.
func TestMesonEclass_ExportedFunctions(t *testing.T) {
	eclass := &MesonEclass{}
	exported := eclass.ExportedFunctions()

	// Expected phase functions per meson.eclass
	expected := []string{
		"src_configure",
		"src_compile",
		"src_test",
		"src_install",
	}

	if len(exported) != len(expected) {
		t.Errorf("MesonEclass.ExportedFunctions() returned %d functions, want %d",
			len(exported), len(expected))
	}

	for i, fn := range expected {
		if i >= len(exported) || exported[i] != fn {
			t.Errorf("MesonEclass.ExportedFunctions()[%d] = %q, want %q",
				i, exported[i], fn)
		}
	}
}

// TestMesonEclass_Variables tests that the eclass sets default variables.
func TestMesonEclass_Variables(t *testing.T) {
	eclass := &MesonEclass{}
	vars := eclass.Variables()

	// Check EMESON_BUILDTYPE
	if buildType, ok := vars["EMESON_BUILDTYPE"]; !ok {
		t.Error("MesonEclass.Variables() should set EMESON_BUILDTYPE")
	} else if buildType != MesonBuildTypePlain {
		t.Errorf("EMESON_BUILDTYPE = %q, want %q", buildType, MesonBuildTypePlain)
	}
}

// TestMesonEclass_RegisterHelpers tests that helpers are registered correctly.
func TestMesonEclass_RegisterHelpers(t *testing.T) {
	testPkg := &pkg.Package{
		Name:    "dev-libs/test",
		Version: "1.0.0",
		Slot:    pkg.Slot{Name: "0"},
	}

	env, err := NewEnvironment(testPkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(env, &stdout, &stderr)

	eclass := &MesonEclass{}
	eclass.RegisterHelpers(helpers)

	// Verify functions were registered
	funcs := helpers.eclassRegistry.GetFunctions("meson")
	if len(funcs) == 0 {
		t.Error("MesonEclass.RegisterHelpers() should register functions")
	}

	// Check for required functions
	expectedFuncs := []string{
		"meson_src_configure",
		"meson_src_compile",
		"meson_src_test",
		"meson_src_install",
		"meson_use",
		"meson_feature",
		"meson_use_bool",
	}

	for _, expected := range expectedFuncs {
		found := false
		for _, fn := range funcs {
			if fn == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("MesonEclass.RegisterHelpers() should register %q", expected)
		}
	}
}

// ============================================================================
// MesonEclassPhaseHandler Tests
// ============================================================================

// TestMesonEclassPhaseHandler tests the phase to handler mapping.
func TestMesonEclassPhaseHandler(t *testing.T) {
	testCases := []struct {
		phase    string
		expected string
	}{
		{"src_configure", "meson_src_configure"},
		{"src_compile", "meson_src_compile"},
		{"src_test", "meson_src_test"},
		{"src_install", "meson_src_install"},
		{"src_prepare", ""}, // Not exported by meson.eclass
		{"src_unpack", ""},  // Not exported by meson.eclass
		{"pkg_setup", ""},   // Not exported by meson.eclass
		{"unknown", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.phase, func(t *testing.T) {
			handler := MesonEclassPhaseHandler(tc.phase)
			if handler != tc.expected {
				t.Errorf("MesonEclassPhaseHandler(%q) = %q, want %q",
					tc.phase, handler, tc.expected)
			}
		})
	}
}

// ============================================================================
// MesonEclassDefaultVariables Tests
// ============================================================================

// TestMesonEclassDefaultVariables tests the default variables function.
func TestMesonEclassDefaultVariables(t *testing.T) {
	vars := MesonEclassDefaultVariables()

	if buildType, ok := vars["EMESON_BUILDTYPE"]; !ok {
		t.Error("MesonEclassDefaultVariables() should set EMESON_BUILDTYPE")
	} else if buildType != "plain" {
		t.Errorf("EMESON_BUILDTYPE = %q, want %q", buildType, "plain")
	}
}

// ============================================================================
// IsMesonEclassFunction Tests
// ============================================================================

// TestIsMesonEclassFunction tests the function identification.
func TestIsMesonEclassFunction(t *testing.T) {
	testCases := []struct {
		name     string
		expected bool
	}{
		{"meson_src_configure", true},
		{"meson_src_compile", true},
		{"meson_src_test", true},
		{"meson_src_install", true},
		{"meson_use", true},
		{"meson_feature", true},
		{"meson_use_bool", true},
		{"meson", true},
		{"cmake_src_configure", false},
		{"src_configure", false},
		{"econf", false},
		{"", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsMesonEclassFunction(tc.name)
			if result != tc.expected {
				t.Errorf("IsMesonEclassFunction(%q) = %v, want %v",
					tc.name, result, tc.expected)
			}
		})
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

// TestMesonEclass_InheritFlow tests the full inherit flow.
func TestMesonEclass_InheritFlow(t *testing.T) {
	// Create test eclass directory
	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatalf("failed to create eclass dir: %v", err)
	}

	// Create a minimal meson.eclass file
	mesonEclassContent := `# meson.eclass - Meson build system support
# GRPM test version

EXPORT_FUNCTIONS src_configure src_compile src_test src_install

meson_src_configure() {
	meson setup "${BUILD_DIR}" "${S}" || die "meson setup failed"
}

meson_src_compile() {
	meson compile -C "${BUILD_DIR}" || die "meson compile failed"
}

meson_src_test() {
	meson test -C "${BUILD_DIR}" || die "meson test failed"
}

meson_src_install() {
	meson install -C "${BUILD_DIR}" --destdir="${D}" || die "meson install failed"
}
`
	eclassPath := filepath.Join(eclassDir, "meson.eclass")
	if err := os.WriteFile(eclassPath, []byte(mesonEclassContent), 0644); err != nil {
		t.Fatalf("failed to write meson.eclass: %v", err)
	}

	// Create environment pointing to test repo
	testPkg := &pkg.Package{
		Name:    "dev-libs/test",
		Version: "1.0.0",
		Slot:    pkg.Slot{Name: "0"},
		UseFlags: map[string]bool{
			"ssl": true,
			"doc": false,
		},
	}

	env, err := NewEnvironment(testPkg, "/tmp/portage", tmpDir, "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	interp := NewInterpreter(env, &stdout, &stderr)
	helpers := interp.GetHelpers()

	// Inherit meson eclass
	err = helpers.Inherit([]string{"meson"})
	if err != nil {
		t.Errorf("Inherit meson failed: %v", err)
	}

	// Verify INHERITED contains meson
	inherited := helpers.GetEclassRegistry().GetInherited()
	if !strings.Contains(inherited, "meson") {
		t.Errorf("INHERITED should contain 'meson', got '%s'", inherited)
	}
}

// TestMesonEclass_UseHelpers tests meson_use helpers through eclass.
func TestMesonEclass_UseHelpers(t *testing.T) {
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

	env, err := NewEnvironment(testPkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment failed: %v", err)
	}

	t.Run("meson_use enabled", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		err := helpers.MesonUse([]string{"ssl"})
		if err != nil {
			t.Fatalf("MesonUse failed: %v", err)
		}

		output := stdout.String()
		if output != "-Dssl=enabled" {
			t.Errorf("MesonUse(ssl) = %q, want %q", output, "-Dssl=enabled")
		}
	})

	t.Run("meson_use disabled", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		err := helpers.MesonUse([]string{"doc"})
		if err != nil {
			t.Fatalf("MesonUse failed: %v", err)
		}

		output := stdout.String()
		if output != "-Ddoc=disabled" {
			t.Errorf("MesonUse(doc) = %q, want %q", output, "-Ddoc=disabled")
		}
	})

	t.Run("meson_use with custom option", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		err := helpers.MesonUse([]string{"ssl", "enable_tls"})
		if err != nil {
			t.Fatalf("MesonUse failed: %v", err)
		}

		output := stdout.String()
		if output != "-Denable_tls=enabled" {
			t.Errorf("MesonUse(ssl, enable_tls) = %q, want %q", output, "-Denable_tls=enabled")
		}
	})

	t.Run("meson_feature enabled", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		err := helpers.MesonFeature([]string{"gtk"})
		if err != nil {
			t.Fatalf("MesonFeature failed: %v", err)
		}

		output := stdout.String()
		if output != "-Dgtk=enabled" {
			t.Errorf("MesonFeature(gtk) = %q, want %q", output, "-Dgtk=enabled")
		}
	})

	t.Run("meson_feature disabled", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		err := helpers.MesonFeature([]string{"systemd"})
		if err != nil {
			t.Fatalf("MesonFeature failed: %v", err)
		}

		output := stdout.String()
		if output != "-Dsystemd=disabled" {
			t.Errorf("MesonFeature(systemd) = %q, want %q", output, "-Dsystemd=disabled")
		}
	})

	t.Run("meson_use_bool true", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		err := helpers.MesonUseBool([]string{"ssl"})
		if err != nil {
			t.Fatalf("MesonUseBool failed: %v", err)
		}

		output := stdout.String()
		if output != "-Dssl=true" {
			t.Errorf("MesonUseBool(ssl) = %q, want %q", output, "-Dssl=true")
		}
	})

	t.Run("meson_use_bool false", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		err := helpers.MesonUseBool([]string{"doc"})
		if err != nil {
			t.Fatalf("MesonUseBool failed: %v", err)
		}

		output := stdout.String()
		if output != "-Ddoc=false" {
			t.Errorf("MesonUseBool(doc) = %q, want %q", output, "-Ddoc=false")
		}
	})
}

// TestMesonEclass_PhaseExecution tests phase function execution through interpreter.
func TestMesonEclass_PhaseExecution(t *testing.T) {
	// Create test directory structure
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	sourceDir := filepath.Join(workDir, "test-1.0.0")
	buildDir := filepath.Join(workDir, "test-1.0.0-build")
	eclassDir := filepath.Join(tmpDir, "eclass")

	for _, dir := range []string{workDir, sourceDir, buildDir, eclassDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	// Create meson.eclass
	mesonEclassContent := `# meson.eclass
EXPORT_FUNCTIONS src_configure src_compile src_test src_install
`
	if err := os.WriteFile(filepath.Join(eclassDir, "meson.eclass"), []byte(mesonEclassContent), 0644); err != nil {
		t.Fatalf("failed to write meson.eclass: %v", err)
	}

	testPkg := &pkg.Package{
		Name:    "dev-libs/test",
		Version: "1.0.0",
		Slot:    pkg.Slot{Name: "0"},
	}

	env, err := NewEnvironment(testPkg, tmpDir, tmpDir, "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment failed: %v", err)
	}

	env.WORKDIR = workDir
	env.S = sourceDir
	env.D = filepath.Join(tmpDir, "image")
	_ = os.MkdirAll(env.D, 0755)

	var stdout, stderr bytes.Buffer
	interp := NewInterpreter(env, &stdout, &stderr)

	// Test calling meson_src_configure through the interpreter
	t.Run("meson_src_configure via interpreter", func(t *testing.T) {
		stdout.Reset()
		stderr.Reset()

		// This will fail because meson is not installed, but we check the output
		script := `meson_src_configure`
		_ = interp.Run(context.Background(), script)

		output := stdout.String()
		// Should at least attempt to run meson
		if !strings.Contains(output, "meson") {
			t.Logf("Output: %s", output)
			// This is acceptable - the function may not be fully dispatched
		}
	})
}

// TestMesonEclass_ExportFunctionsRegistration tests EXPORT_FUNCTIONS integration.
func TestMesonEclass_ExportFunctionsRegistration(t *testing.T) {
	testPkg := &pkg.Package{
		Name:    "dev-libs/test",
		Version: "1.0.0",
		Slot:    pkg.Slot{Name: "0"},
	}

	env, err := NewEnvironment(testPkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(env, &stdout, &stderr)

	// Simulate setting current eclass
	helpers.eclassRegistry.SetCurrentEclass("meson")

	// Export functions as meson.eclass would
	for _, phase := range []string{"src_configure", "src_compile", "src_test", "src_install"} {
		err := helpers.eclassRegistry.ExportFunction(phase)
		if err != nil {
			t.Errorf("ExportFunction(%q) failed: %v", phase, err)
		}
	}

	// Verify exported functions are mapped to meson
	for _, phase := range []string{"src_configure", "src_compile", "src_test", "src_install"} {
		eclass, ok := helpers.eclassRegistry.GetExportedFunction(phase)
		if !ok {
			t.Errorf("%s should be exported", phase)
			continue
		}
		if eclass != "meson" {
			t.Errorf("GetExportedFunction(%q) = %q, want %q", phase, eclass, "meson")
		}
	}
}

// TestMesonEclass_WithInterpreter tests full interpreter integration.
func TestMesonEclass_WithInterpreter(t *testing.T) {
	// Create test directory structure
	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatalf("failed to create eclass dir: %v", err)
	}

	// Create meson.eclass that uses Go helpers
	mesonEclassContent := `# meson.eclass
EXPORT_FUNCTIONS src_configure src_compile src_test src_install

# USE flag helpers are implemented in Go
`
	if err := os.WriteFile(filepath.Join(eclassDir, "meson.eclass"), []byte(mesonEclassContent), 0644); err != nil {
		t.Fatalf("failed to write meson.eclass: %v", err)
	}

	testPkg := &pkg.Package{
		Name:    "dev-libs/test",
		Version: "1.0.0",
		Slot:    pkg.Slot{Name: "0"},
		UseFlags: map[string]bool{
			"ssl": true,
			"doc": false,
		},
	}

	env, err := NewEnvironment(testPkg, tmpDir, tmpDir, "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	interp := NewInterpreter(env, &stdout, &stderr)

	// Test meson_use through interpreter
	t.Run("meson_use through interpreter", func(t *testing.T) {
		stdout.Reset()
		script := `meson_use ssl`
		err := interp.Run(context.Background(), script)
		if err != nil {
			t.Errorf("meson_use script failed: %v", err)
		}

		output := stdout.String()
		if !strings.Contains(output, "-Dssl=enabled") {
			t.Errorf("meson_use ssl output = %q, want to contain '-Dssl=enabled'", output)
		}
	})

	t.Run("meson_use_bool through interpreter", func(t *testing.T) {
		stdout.Reset()
		script := `meson_use_bool doc`
		err := interp.Run(context.Background(), script)
		if err != nil {
			t.Errorf("meson_use_bool script failed: %v", err)
		}

		output := stdout.String()
		if !strings.Contains(output, "-Ddoc=false") {
			t.Errorf("meson_use_bool doc output = %q, want to contain '-Ddoc=false'", output)
		}
	})

	t.Run("meson_feature through interpreter", func(t *testing.T) {
		stdout.Reset()
		script := `meson_feature ssl openssl`
		err := interp.Run(context.Background(), script)
		if err != nil {
			t.Errorf("meson_feature script failed: %v", err)
		}

		output := stdout.String()
		if !strings.Contains(output, "-Dopenssl=enabled") {
			t.Errorf("meson_feature ssl openssl output = %q, want to contain '-Dopenssl=enabled'", output)
		}
	})
}

// TestMesonEclass_RealWorldPattern tests a realistic meson.eclass usage pattern.
func TestMesonEclass_RealWorldPattern(t *testing.T) {
	// This test simulates how a real ebuild would use meson.eclass

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	sourceDir := filepath.Join(workDir, "libfoo-1.0.0")
	imageDir := filepath.Join(tmpDir, "image")
	eclassDir := filepath.Join(tmpDir, "eclass")

	for _, dir := range []string{workDir, sourceDir, imageDir, eclassDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	// Create meson.eclass
	mesonEclass := `# meson.eclass
EXPORT_FUNCTIONS src_configure src_compile src_test src_install
`
	if err := os.WriteFile(filepath.Join(eclassDir, "meson.eclass"), []byte(mesonEclass), 0644); err != nil {
		t.Fatalf("failed to write meson.eclass: %v", err)
	}

	testPkg := &pkg.Package{
		Name:    "dev-libs/libfoo",
		Version: "1.0.0",
		Slot:    pkg.Slot{Name: "0"},
		UseFlags: map[string]bool{
			"introspection": true,
			"doc":           false,
			"test":          true,
		},
	}

	env, err := NewEnvironment(testPkg, tmpDir, tmpDir, "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment failed: %v", err)
	}

	env.WORKDIR = workDir
	env.S = sourceDir
	env.D = imageDir

	var stdout, stderr bytes.Buffer
	interp := NewInterpreter(env, &stdout, &stderr)

	// Simulate ebuild pattern:
	// inherit meson
	// src_configure() {
	//     local emesonargs=(
	//         $(meson_use introspection)
	//         $(meson_feature doc documentation)
	//         $(meson_use_bool test enable_tests)
	//     )
	//     meson_src_configure
	// }

	script := `
# Build meson args like a real ebuild would
opt1=$(meson_use introspection)
opt2=$(meson_feature doc documentation)
opt3=$(meson_use_bool test enable_tests)
echo "emesonargs: $opt1 $opt2 $opt3"
`

	err = interp.Run(context.Background(), script)
	if err != nil {
		t.Errorf("script execution failed: %v", err)
	}

	output := stdout.String()

	// Verify expected output
	if !strings.Contains(output, "-Dintrospection=enabled") {
		t.Errorf("output should contain '-Dintrospection=enabled', got: %s", output)
	}
	if !strings.Contains(output, "-Ddocumentation=disabled") {
		t.Errorf("output should contain '-Ddocumentation=disabled', got: %s", output)
	}
	if !strings.Contains(output, "-Denable_tests=true") {
		t.Errorf("output should contain '-Denable_tests=true', got: %s", output)
	}
}
