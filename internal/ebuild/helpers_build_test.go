package ebuild

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Unpack Tests
// ============================================================================

func TestHelpers_Unpack_NoArgs(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	err := helpers.Unpack([]string{})
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestHelpers_Unpack_NoWorkdir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	err := helpers.Unpack([]string{"file.tar.gz"})
	if err == nil {
		t.Error("expected error with no WORKDIR set")
	}
}

func TestHelpers_Unpack_UnsupportedFormat(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	// Create unsupported file
	unsupportedFile := filepath.Join(tmpDir, "file.unknown")
	if err := os.WriteFile(unsupportedFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	err := helpers.Unpack([]string{unsupportedFile})
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestHelpers_Unpack_FileNotFound(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	err := helpers.Unpack([]string{"nonexistent.tar.gz"})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// ============================================================================
// Econf Tests
// ============================================================================

func TestHelpers_Econf_NoWorkdir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	err := helpers.Econf([]string{})
	if err == nil {
		t.Error("expected error with no working directory")
	}
}

func TestHelpers_Econf_NoConfigureScript(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	// Source directory exists but has no configure script
	err := helpers.Econf([]string{})
	if err == nil {
		t.Error("expected error when no configure script exists")
	}
}

func TestHelpers_BuildConfArgs(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	args := helpers.buildConfArgs()

	// Check for standard arguments
	expected := []string{
		"--prefix=/usr",
		"--sysconfdir=/etc",
		"--localstatedir=/var",
		"--mandir=/usr/share/man",
		"--infodir=/usr/share/info",
	}

	for _, exp := range expected {
		found := false
		for _, arg := range args {
			if arg == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected argument %s in conf args", exp)
		}
	}

	// Should have --libdir
	hasLibdir := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "--libdir=") {
			hasLibdir = true
			break
		}
	}
	if !hasLibdir {
		t.Error("expected --libdir argument")
	}
}

// ============================================================================
// Emake Tests
// ============================================================================

func TestHelpers_Emake_NoWorkdir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	err := helpers.Emake([]string{})
	if err == nil {
		t.Error("expected error with no working directory")
	}
}

func TestHelpers_GetMakeOpts(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	opts := helpers.getMakeOpts()
	if len(opts) == 0 {
		t.Error("expected MAKEOPTS to be parsed")
	}

	// Should contain -j4
	found := false
	for _, opt := range opts {
		if opt == "-j4" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected -j4 in MAKEOPTS, got: %v", opts)
	}
}

// ============================================================================
// Eapply Tests
// ============================================================================

func TestHelpers_Eapply_NoArgs(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	err := helpers.Eapply([]string{})
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestHelpers_Eapply_NoWorkdir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	err := helpers.Eapply([]string{"file.patch"})
	if err == nil {
		t.Error("expected error with no working directory")
	}
}

func TestHelpers_Eapply_FileNotFound(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	err := helpers.Eapply([]string{"nonexistent.patch"})
	if err == nil {
		t.Error("expected error for nonexistent patch")
	}
}

// ============================================================================
// EapplyUser Tests
// ============================================================================

func TestHelpers_EapplyUser_NoEnv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	// Should succeed silently with no environment
	err := helpers.EapplyUser([]string{})
	if err != nil {
		t.Errorf("EapplyUser should succeed with nil env: %v", err)
	}
}

func TestHelpers_EapplyUser_NoPatchesDir(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	// Should succeed silently when no patches directory exists
	err := helpers.EapplyUser([]string{})
	if err != nil {
		t.Errorf("EapplyUser should succeed with no patches dir: %v", err)
	}
}

// ============================================================================
// Default Phase Tests
// ============================================================================

func TestHelpers_DefaultSrcUnpack_NoEnv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	err := helpers.DefaultSrcUnpack([]string{})
	if err == nil {
		t.Error("expected error with nil environment")
	}
}

func TestHelpers_DefaultSrcUnpack_EmptyA(t *testing.T) {
	helpers, _, stdout, _ := createBuildTestHelpers(t)
	helpers.env.A = "" // Empty archive list

	err := helpers.DefaultSrcUnpack([]string{})
	if err != nil {
		t.Errorf("DefaultSrcUnpack should succeed with empty A: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "No archives") {
		t.Errorf("expected 'No archives' message, got: %s", output)
	}
}

func TestHelpers_DefaultSrcPrepare(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	// Should succeed (calls EapplyUser which succeeds with no patches)
	err := helpers.DefaultSrcPrepare([]string{})
	if err != nil {
		t.Errorf("DefaultSrcPrepare failed: %v", err)
	}
}

func TestHelpers_DefaultSrcConfigure_NoConfigure(t *testing.T) {
	helpers, _, stdout, _ := createBuildTestHelpers(t)

	err := helpers.DefaultSrcConfigure([]string{})
	if err != nil {
		t.Errorf("DefaultSrcConfigure should succeed with no configure: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "No configure") {
		t.Errorf("expected 'No configure' message, got: %s", output)
	}
}

func TestHelpers_DefaultSrcCompile_NoMakefile(t *testing.T) {
	helpers, _, stdout, _ := createBuildTestHelpers(t)

	err := helpers.DefaultSrcCompile([]string{})
	if err != nil {
		t.Errorf("DefaultSrcCompile should succeed with no Makefile: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "No Makefile") {
		t.Errorf("expected 'No Makefile' message, got: %s", output)
	}
}

func TestHelpers_DefaultSrcTest_NoMakefile(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	// Should succeed silently with no Makefile
	err := helpers.DefaultSrcTest([]string{})
	if err != nil {
		t.Errorf("DefaultSrcTest should succeed with no Makefile: %v", err)
	}
}

func TestHelpers_DefaultSrcInstall_NoEnv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	err := helpers.DefaultSrcInstall([]string{})
	if err == nil {
		t.Error("expected error with nil environment")
	}
}

func TestHelpers_DefaultSrcInstall_NoMakefile(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	// Per PMS Section 9.1.9: default_src_install in EAPI 6+ runs emake install
	// only IF a Makefile exists, then calls einstalldocs.
	// With no Makefile, it should still succeed (just calls einstalldocs).
	err := helpers.DefaultSrcInstall([]string{})
	if err != nil {
		t.Errorf("default_src_install should succeed with no Makefile (EAPI 6+ format): %v", err)
	}
}

func TestHelpers_Default_UnknownPhase(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)
	helpers.env.EBUILD_PHASE = "unknownphase"

	// Per PMS Section 12.3.15: default must not be called if no default_
	// function exists for the current phase. Unknown phases should error.
	err := helpers.Default([]string{})
	if err == nil {
		t.Error("Default should fail for unknown phase")
	}
}
