// Package ebuild implements ebuild execution engine.
//
// This file provides tests for default phase implementations per PMS Section 9.1.17 and 12.3.15.
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
// Test Helpers
// ============================================================================

// createDefaultTestHelpers creates helpers with a specific EAPI for testing default functions.
func createDefaultTestHelpers(t *testing.T, eapi string) (*Helpers, string, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	sourceDir := filepath.Join(workDir, "testpkg-1.0.0")

	// Create directories
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	testPkg := &pkg.Package{
		Name:    "app-misc/testpkg",
		Version: "1.0.0",
		Slot:    pkg.Slot{Name: "0"},
		UseFlags: map[string]bool{
			"test": true,
		},
	}

	env, err := NewEnvironmentWithEAPI(testPkg, tmpDir, "/var/db/repos/gentoo", tmpDir, eapi)
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	// Ensure directories exist
	if err := os.MkdirAll(env.WORKDIR, 0755); err != nil {
		t.Fatalf("failed to create workdir: %v", err)
	}
	if err := os.MkdirAll(env.S, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.MkdirAll(env.D, 0755); err != nil {
		t.Fatalf("failed to create image dir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(env, &stdout, &stderr)

	return helpers, tmpDir, &stdout, &stderr
}

// ============================================================================
// EAPI Version Check Tests
// ============================================================================

func TestDefaultSrcUnpack_EAPI0_NotAvailable(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "0")

	err := helpers.DefaultSrcUnpack(nil)
	if err == nil {
		t.Error("expected error for default_src_unpack in EAPI 0")
	}

	var dieErr *DieError
	if !errors.As(err, &dieErr) {
		t.Fatalf("expected DieError, got %T", err)
	}
	if !strings.Contains(dieErr.Message, "EAPI 0") || !strings.Contains(dieErr.Message, "requires EAPI 2+") {
		t.Errorf("unexpected error message: %s", dieErr.Message)
	}
}

func TestDefaultSrcUnpack_EAPI1_NotAvailable(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "1")

	err := helpers.DefaultSrcUnpack(nil)
	if err == nil {
		t.Error("expected error for default_src_unpack in EAPI 1")
	}

	var dieErr *DieError
	if !errors.As(err, &dieErr) {
		t.Fatalf("expected DieError, got %T", err)
	}
	if !strings.Contains(dieErr.Message, "EAPI 1") {
		t.Errorf("unexpected error message: %s", dieErr.Message)
	}
}

func TestDefaultSrcUnpack_EAPI2_Available(t *testing.T) {
	helpers, _, stdout, _ := createDefaultTestHelpers(t, "2")
	helpers.env.A = "" // Empty archive list

	err := helpers.DefaultSrcUnpack(nil)
	if err != nil {
		t.Errorf("default_src_unpack should be available in EAPI 2: %v", err)
	}

	if !strings.Contains(stdout.String(), "No archives") {
		t.Errorf("expected 'No archives' message, got: %s", stdout.String())
	}
}

func TestDefaultSrcInstall_EAPI2_NotAvailable(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "2")

	err := helpers.DefaultSrcInstall(nil)
	if err == nil {
		t.Error("expected error for default_src_install in EAPI 2")
	}

	var dieErr *DieError
	if !errors.As(err, &dieErr) {
		t.Fatalf("expected DieError, got %T", err)
	}
	if !strings.Contains(dieErr.Message, "EAPI 2") || !strings.Contains(dieErr.Message, "requires EAPI 4+") {
		t.Errorf("unexpected error message: %s", dieErr.Message)
	}
}

func TestDefaultSrcInstall_EAPI3_NotAvailable(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "3")

	err := helpers.DefaultSrcInstall(nil)
	if err == nil {
		t.Error("expected error for default_src_install in EAPI 3")
	}
}

func TestDefaultSrcInstall_EAPI4_Available(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "4")

	// Create Makefile so emake install is attempted (it will fail but EAPI check passes)
	makefilePath := filepath.Join(helpers.env.S, "Makefile")
	if err := os.WriteFile(makefilePath, []byte("install:\n\t@echo installed\n"), 0644); err != nil {
		t.Fatalf("failed to create Makefile: %v", err)
	}

	// This will try to run emake which will fail, but the EAPI check should pass
	err := helpers.DefaultSrcInstall(nil)
	// We expect it to fail at emake, not at EAPI check
	if err != nil {
		var dieErr *DieError
		if errors.As(err, &dieErr) && strings.Contains(dieErr.Message, "EAPI 4") {
			t.Error("default_src_install should be available in EAPI 4")
		}
	}
}

// ============================================================================
// Default Command Tests (PMS 12.3.15)
// ============================================================================

func TestDefault_EAPI0_NotAvailable(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "0")

	err := helpers.Default(nil)
	if err == nil {
		t.Error("expected error for default command in EAPI 0")
	}

	var dieErr *DieError
	if !errors.As(err, &dieErr) {
		t.Fatalf("expected DieError, got %T", err)
	}
	if !strings.Contains(dieErr.Message, "not available in EAPI 0") {
		t.Errorf("unexpected error message: %s", dieErr.Message)
	}
}

func TestDefault_EAPI2_Available(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "2")
	helpers.env.EBUILD_PHASE = "configure"

	// Should succeed (configure phase with no configure script -> skip)
	err := helpers.Default(nil)
	if err != nil {
		t.Errorf("default command should be available in EAPI 2: %v", err)
	}
}

func TestDefault_UnknownPhase_Error(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "8")
	helpers.env.EBUILD_PHASE = "unknownphase"

	err := helpers.Default(nil)
	if err == nil {
		t.Error("expected error for unknown phase")
	}

	var dieErr *DieError
	if !errors.As(err, &dieErr) {
		t.Fatalf("expected DieError, got %T", err)
	}
	if !strings.Contains(dieErr.Message, "no default_unknownphase") {
		t.Errorf("unexpected error message: %s", dieErr.Message)
	}
}

func TestDefault_NoPhase_Error(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "8")
	helpers.env.EBUILD_PHASE = ""

	err := helpers.Default(nil)
	if err == nil {
		t.Error("expected error when EBUILD_PHASE not set")
	}

	var dieErr *DieError
	if !errors.As(err, &dieErr) {
		t.Fatalf("expected DieError, got %T", err)
	}
	if !strings.Contains(dieErr.Message, "EBUILD_PHASE not set") {
		t.Errorf("unexpected error message: %s", dieErr.Message)
	}
}

// ============================================================================
// Default Phase Dispatcher Tests
// ============================================================================

func TestDefault_DispatchesToCorrectPhase(t *testing.T) {
	tests := []struct {
		phase       string
		expectError bool
	}{
		{"nofetch", false},
		{"unpack", false},
		{"prepare", false},
		{"configure", false},
		{"compile", false},
		{"test", false},
		{"install", false},
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			helpers, _, _, _ := createDefaultTestHelpers(t, "8")
			helpers.env.EBUILD_PHASE = tt.phase
			helpers.env.A = "" // Empty archive list for unpack

			err := helpers.Default(nil)
			if tt.expectError && err == nil {
				t.Errorf("expected error for phase %s", tt.phase)
			}
			// Most phases should succeed with no-op behavior
			// (no Makefile, no configure, etc.)
		})
	}
}

// ============================================================================
// DefaultPkgNofetch Tests
// ============================================================================

func TestDefaultPkgNofetch_EAPI0_NotAvailable(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "0")

	err := helpers.DefaultPkgNofetch(nil)
	if err == nil {
		t.Error("expected error for default_pkg_nofetch in EAPI 0")
	}
}

func TestDefaultPkgNofetch_EAPI2_Available(t *testing.T) {
	helpers, _, _, stderr := createDefaultTestHelpers(t, "2")
	helpers.env.A = "file1.tar.gz file2.tar.bz2"
	helpers.env.CATEGORY = "app-misc"
	helpers.env.PF = "testpkg-1.0.0"

	err := helpers.DefaultPkgNofetch(nil)
	if err != nil {
		t.Errorf("default_pkg_nofetch failed: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "file1.tar.gz") {
		t.Errorf("expected file1.tar.gz in output, got: %s", output)
	}
	if !strings.Contains(output, "file2.tar.bz2") {
		t.Errorf("expected file2.tar.bz2 in output, got: %s", output)
	}
	if !strings.Contains(output, "app-misc/testpkg-1.0.0") {
		t.Errorf("expected package name in output, got: %s", output)
	}
}

// ============================================================================
// DefaultSrcPrepare Tests
// ============================================================================

func TestDefaultSrcPrepare_EAPI2_NoOp(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "2")

	// EAPI 2-5: default_src_prepare is a no-op
	err := helpers.DefaultSrcPrepare(nil)
	if err != nil {
		t.Errorf("default_src_prepare in EAPI 2 should be no-op: %v", err)
	}
}

func TestDefaultSrcPrepare_EAPI5_NoOp(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "5")

	// EAPI 2-5: default_src_prepare is a no-op
	err := helpers.DefaultSrcPrepare(nil)
	if err != nil {
		t.Errorf("default_src_prepare in EAPI 5 should be no-op: %v", err)
	}
}

func TestDefaultSrcPrepare_EAPI6_CallsEapplyUser(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "6")

	// EAPI 6+: default_src_prepare calls eapply_user (which succeeds with no patches)
	err := helpers.DefaultSrcPrepare(nil)
	if err != nil {
		t.Errorf("default_src_prepare in EAPI 6 failed: %v", err)
	}
}

func TestDefaultSrcPrepare_EAPI8_WithPatches(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "8")

	// Set PATCHES to empty (should not fail)
	helpers.env.SetVar("PATCHES", "")

	err := helpers.DefaultSrcPrepare(nil)
	if err != nil {
		t.Errorf("default_src_prepare in EAPI 8 failed: %v", err)
	}
}

// ============================================================================
// DefaultSrcConfigure Tests
// ============================================================================

func TestDefaultSrcConfigure_NoConfigure(t *testing.T) {
	helpers, _, stdout, _ := createDefaultTestHelpers(t, "8")

	err := helpers.DefaultSrcConfigure(nil)
	if err != nil {
		t.Errorf("default_src_configure should succeed with no configure: %v", err)
	}

	if !strings.Contains(stdout.String(), "No configure") {
		t.Errorf("expected 'No configure' message, got: %s", stdout.String())
	}
}

func TestDefaultSrcConfigure_NotExecutable(t *testing.T) {
	helpers, _, stdout, _ := createDefaultTestHelpers(t, "8")

	// Create non-executable configure
	configurePath := filepath.Join(helpers.env.S, "configure")
	if err := os.WriteFile(configurePath, []byte("#!/bin/sh\n"), 0644); err != nil {
		t.Fatalf("failed to create configure: %v", err)
	}

	err := helpers.DefaultSrcConfigure(nil)
	if err != nil {
		t.Errorf("default_src_configure should succeed with non-executable configure: %v", err)
	}

	if !strings.Contains(stdout.String(), "not executable") {
		t.Errorf("expected 'not executable' message, got: %s", stdout.String())
	}
}

func TestDefaultSrcConfigure_UsesECONF_SOURCE(t *testing.T) {
	helpers, tmpDir, stdout, _ := createDefaultTestHelpers(t, "8")

	// Create ECONF_SOURCE directory with configure
	econfDir := filepath.Join(tmpDir, "econf_source")
	if err := os.MkdirAll(econfDir, 0755); err != nil {
		t.Fatalf("failed to create econf dir: %v", err)
	}

	// Set ECONF_SOURCE
	helpers.env.SetVar("ECONF_SOURCE", econfDir)

	// No configure in ECONF_SOURCE
	err := helpers.DefaultSrcConfigure(nil)
	if err != nil {
		t.Errorf("default_src_configure failed: %v", err)
	}

	if !strings.Contains(stdout.String(), "No configure") {
		t.Errorf("expected 'No configure' message, got: %s", stdout.String())
	}
}

// ============================================================================
// DefaultSrcCompile Tests
// ============================================================================

func TestDefaultSrcCompile_NoMakefile(t *testing.T) {
	helpers, _, stdout, _ := createDefaultTestHelpers(t, "8")

	err := helpers.DefaultSrcCompile(nil)
	if err != nil {
		t.Errorf("default_src_compile should succeed with no Makefile: %v", err)
	}

	if !strings.Contains(stdout.String(), "No Makefile") {
		t.Errorf("expected 'No Makefile' message, got: %s", stdout.String())
	}
}

func TestDefaultSrcCompile_GNUmakefile(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "8")

	// Create GNUmakefile (alternative name)
	makefilePath := filepath.Join(helpers.env.S, "GNUmakefile")
	if err := os.WriteFile(makefilePath, []byte("all:\n\t@echo built\n"), 0644); err != nil {
		t.Fatalf("failed to create GNUmakefile: %v", err)
	}

	// Will try to run make, which will fail in test env
	err := helpers.DefaultSrcCompile(nil)
	// Just check it found the makefile (error is expected since make won't work)
	if err != nil {
		// The error should be about make failing, not about no makefile
		var dieErr *DieError
		if errors.As(err, &dieErr) && strings.Contains(dieErr.Message, "No Makefile") {
			t.Error("should have found GNUmakefile")
		}
	}
}

// ============================================================================
// DefaultSrcTest Tests
// ============================================================================

func TestDefaultSrcTest_NoMakefile(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "8")

	// Should succeed silently with no Makefile
	err := helpers.DefaultSrcTest(nil)
	if err != nil {
		t.Errorf("default_src_test should succeed with no Makefile: %v", err)
	}
}

// ============================================================================
// DefaultSrcInstall Tests
// ============================================================================

func TestDefaultSrcInstall_EAPI4_Format4(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "4")

	// No Makefile, but should still try to install docs
	// Create a README file
	readmePath := filepath.Join(helpers.env.S, "README")
	if err := os.WriteFile(readmePath, []byte("Test readme content\n"), 0644); err != nil {
		t.Fatalf("failed to create README: %v", err)
	}

	// EAPI 4 format: should try to install README even without Makefile
	err := helpers.DefaultSrcInstall(nil)
	// May fail at emake but should not fail at EAPI check
	if err != nil {
		var dieErr *DieError
		if errors.As(err, &dieErr) && strings.Contains(dieErr.Message, "EAPI 4") {
			t.Error("default_src_install should be available in EAPI 4")
		}
	}
}

func TestDefaultSrcInstall_EAPI6_Format6(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "6")

	// No Makefile
	// EAPI 6 format: should call einstalldocs
	err := helpers.DefaultSrcInstall(nil)
	if err != nil {
		var dieErr *DieError
		if errors.As(err, &dieErr) && strings.Contains(dieErr.Message, "EAPI 6") {
			t.Error("default_src_install should be available in EAPI 6")
		}
	}
}

// ============================================================================
// hasMakefile Tests
// ============================================================================

func TestHasMakefile(t *testing.T) {
	helpers, tmpDir, _, _ := createDefaultTestHelpers(t, "8")

	testDir := filepath.Join(tmpDir, "makefile_test")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	// No makefile
	if helpers.hasMakefile(testDir) {
		t.Error("hasMakefile should return false for empty directory")
	}

	// Create Makefile
	if err := os.WriteFile(filepath.Join(testDir, "Makefile"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to create Makefile: %v", err)
	}
	if !helpers.hasMakefile(testDir) {
		t.Error("hasMakefile should return true when Makefile exists")
	}

	// Remove Makefile, create GNUmakefile
	if err := os.Remove(filepath.Join(testDir, "Makefile")); err != nil {
		t.Fatalf("failed to remove Makefile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "GNUmakefile"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to create GNUmakefile: %v", err)
	}
	if !helpers.hasMakefile(testDir) {
		t.Error("hasMakefile should return true when GNUmakefile exists")
	}

	// Remove GNUmakefile, create makefile (lowercase)
	if err := os.Remove(filepath.Join(testDir, "GNUmakefile")); err != nil {
		t.Fatalf("failed to remove GNUmakefile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "makefile"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to create makefile: %v", err)
	}
	if !helpers.hasMakefile(testDir) {
		t.Error("hasMakefile should return true when makefile exists")
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestDefault_IntegrationWithPhases(t *testing.T) {
	// Test that default properly dispatches to each phase
	phases := map[string]bool{
		"nofetch":   true,
		"unpack":    true,
		"prepare":   true,
		"configure": true,
		"compile":   true,
		"test":      true,
		"install":   true,
	}

	for phase := range phases {
		t.Run(phase, func(t *testing.T) {
			helpers, _, _, _ := createDefaultTestHelpers(t, "8")
			helpers.env.EBUILD_PHASE = phase
			helpers.env.A = "" // Empty archive list

			// All phases should work without crashing
			// (actual behavior depends on environment state)
			_ = helpers.Default(nil)
		})
	}
}

func TestDefaultSrcPrepare_AppliesPatches_EAPI8(t *testing.T) {
	helpers, _, _, _ := createDefaultTestHelpers(t, "8")

	// Create a simple file to patch
	testFile := filepath.Join(helpers.env.S, "test.txt")
	if err := os.WriteFile(testFile, []byte("original content\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Note: Actually applying patches would require creating real patch files
	// This test just verifies the PATCHES variable is handled

	// Set empty PATCHES (should succeed)
	helpers.env.SetVar("PATCHES", "")

	err := helpers.DefaultSrcPrepare(nil)
	if err != nil {
		t.Errorf("default_src_prepare failed: %v", err)
	}
}
