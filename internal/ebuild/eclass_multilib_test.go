// Package ebuild implements ebuild execution engine.
//
// This file contains tests for multilib eclasses.
package ebuild

import (
	"bytes"
	"strings"
	"testing"
)

// ============================================================================
// ABI Tests
// ============================================================================

func TestCommonABIs(t *testing.T) {
	// Test that common ABIs are defined
	archs := []string{"amd64", "x86", "arm64", "arm"}
	for _, arch := range archs {
		if _, ok := CommonABIs[arch]; !ok {
			t.Errorf("CommonABIs missing architecture: %s", arch)
		}
	}

	// Test amd64 has both 64 and 32-bit ABIs
	amd64ABIs := CommonABIs["amd64"]
	if len(amd64ABIs) < 2 {
		t.Error("amd64 should have at least 2 ABIs (64-bit and 32-bit)")
	}

	// Verify amd64 ABI
	if amd64ABIs[0].Name != "amd64" {
		t.Errorf("expected amd64, got %s", amd64ABIs[0].Name)
	}
	if amd64ABIs[0].LibDir != "lib64" {
		t.Errorf("expected lib64, got %s", amd64ABIs[0].LibDir)
	}
}

// ============================================================================
// Libdir Tests
// ============================================================================

func TestComputeLibdir_AMD64(t *testing.T) {
	env := &Environment{}
	env.SetVar("CHOST", "x86_64-pc-linux-gnu")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	libdir := h.computeLibdir()
	if libdir != "lib64" {
		t.Errorf("expected lib64, got %s", libdir)
	}
}

func TestComputeLibdir_X86(t *testing.T) {
	env := &Environment{}
	env.SetVar("CHOST", "i686-pc-linux-gnu")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	libdir := h.computeLibdir()
	if libdir != "lib" {
		t.Errorf("expected lib, got %s", libdir)
	}
}

func TestComputeLibdir_WithABI(t *testing.T) {
	env := &Environment{}
	env.SetVar("CHOST", "x86_64-pc-linux-gnu")
	env.SetVar("ABI", "x86")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	libdir := h.computeLibdir()
	if libdir != "lib32" {
		t.Errorf("expected lib32 for x86 ABI on multilib, got %s", libdir)
	}
}

func TestComputeABILibdir(t *testing.T) {
	tests := []struct {
		abi      string
		expected string
	}{
		{"amd64", "lib64"},
		{"x86", "lib32"},
		{"arm64", "lib64"},
		{"arm", "lib"},
	}

	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	for _, tt := range tests {
		t.Run(tt.abi, func(t *testing.T) {
			got := h.computeABILibdir(tt.abi)
			if got != tt.expected {
				t.Errorf("computeABILibdir(%s) = %s, want %s", tt.abi, got, tt.expected)
			}
		})
	}
}

func TestGetABILibdir(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	err := h.GetABILibdir([]string{"amd64"})
	if err != nil {
		t.Fatalf("GetABILibdir error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != "lib64" {
		t.Errorf("expected lib64, got %s", got)
	}
}

// ============================================================================
// CHOST Tests
// ============================================================================

func TestGetABIChost(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	err := h.GetABIChost([]string{"amd64"})
	if err != nil {
		t.Fatalf("GetABIChost error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != "x86_64-pc-linux-gnu" {
		t.Errorf("expected x86_64-pc-linux-gnu, got %s", got)
	}
}

// ============================================================================
// CFLAGS Tests
// ============================================================================

func TestGetABICflags(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	// x86 ABI should have -m32
	err := h.GetABICflags([]string{"x86"})
	if err != nil {
		t.Fatalf("GetABICflags error: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != "-m32" {
		t.Errorf("expected -m32, got %s", got)
	}

	// amd64 should have empty cflags
	stdout.Reset()
	err = h.GetABICflags([]string{"amd64"})
	if err != nil {
		t.Fatalf("GetABICflags error: %v", err)
	}

	got = strings.TrimSpace(stdout.String())
	if got != "" {
		t.Errorf("expected empty, got %s", got)
	}
}

// ============================================================================
// Multilib Environment Tests
// ============================================================================

func TestSetupABIEnvironment(t *testing.T) {
	// Test with amd64 ABI which has unambiguous configuration
	env := &Environment{}
	env.SetVar("CFLAGS", "-O2")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	err := h.setupABIEnvironment("amd64")
	if err != nil {
		t.Fatalf("setupABIEnvironment error: %v", err)
	}

	// Check ABI is set (stored in ExtraVars)
	if env.ExtraVars["ABI"] != "amd64" {
		t.Errorf("ABI not set correctly, got: %s", env.ExtraVars["ABI"])
	}

	// Check LIBDIR_amd64 is set (stored in ExtraVars)
	if env.ExtraVars["LIBDIR_amd64"] != "lib64" {
		t.Errorf("LIBDIR_amd64 not set correctly, got: %s", env.ExtraVars["LIBDIR_amd64"])
	}

	// Check CHOST_amd64 is set
	if env.ExtraVars["CHOST_amd64"] != "x86_64-pc-linux-gnu" {
		t.Errorf("CHOST_amd64 not set correctly, got: %s", env.ExtraVars["CHOST_amd64"])
	}
}

func TestSetupABIEnvironment_UnknownABI(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	err := h.setupABIEnvironment("unknown_abi")
	if err == nil {
		t.Error("expected error for unknown ABI")
	}
}

// ============================================================================
// Native ABI Tests
// ============================================================================

func TestGetDefaultABI(t *testing.T) {
	tests := []struct {
		chost    string
		expected string
	}{
		{"x86_64-pc-linux-gnu", "amd64"},
		{"i686-pc-linux-gnu", "x86"},
		{"aarch64-unknown-linux-gnu", "arm64"},
		{"armv7a-hardfloat-linux-gnueabi", "arm"},
	}

	for _, tt := range tests {
		t.Run(tt.chost, func(t *testing.T) {
			env := &Environment{}
			env.SetVar("CHOST", tt.chost)
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			h := NewHelpers(env, stdout, stderr)

			got := h.getDefaultABI()
			if got != tt.expected {
				t.Errorf("getDefaultABI() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestMultilibIsNativeABI(t *testing.T) {
	env := &Environment{}
	env.SetVar("CHOST", "x86_64-pc-linux-gnu")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	// No ABI set - should be native
	err := h.MultilibIsNativeABI(nil)
	if err != nil {
		t.Error("expected native ABI when ABI not set")
	}

	// Set ABI to native
	env.SetVar("ABI", "amd64")
	err = h.MultilibIsNativeABI(nil)
	if err != nil {
		t.Error("expected native ABI for amd64")
	}

	// Set ABI to non-native
	env.SetVar("ABI", "x86")
	err = h.MultilibIsNativeABI(nil)
	if err == nil {
		t.Error("expected non-native for x86 on amd64 system")
	}
}

// ============================================================================
// Enabled ABIs Tests
// ============================================================================

func TestGetEnabledABIs_Default(t *testing.T) {
	env := &Environment{}
	env.SetVar("CHOST", "x86_64-pc-linux-gnu")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	abis := h.getEnabledABIs()
	if len(abis) == 0 {
		t.Error("expected at least one ABI")
	}

	// Should default to native ABI
	if abis[0].Name != "amd64" {
		t.Errorf("expected amd64 as default, got %s", abis[0].Name)
	}
}

func TestGetEnabledABIs_UseFlags(t *testing.T) {
	env := &Environment{
		USE: "abi_x86_64 abi_x86_32",
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	abis := h.getEnabledABIs()
	if len(abis) != 2 {
		t.Errorf("expected 2 ABIs, got %d", len(abis))
	}
}

// ============================================================================
// Multilib Build Eclass Tests
// ============================================================================

func TestMultilibBuildEclass(t *testing.T) {
	eclass := &MultilibBuildEclass{}

	if eclass.Name() != "multilib-build" {
		t.Errorf("expected multilib-build, got %s", eclass.Name())
	}

	funcs := eclass.ExportedFunctions()
	expected := []string{"src_configure", "src_compile", "src_test", "src_install"}
	for _, exp := range expected {
		found := false
		for _, f := range funcs {
			if f == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing exported function: %s", exp)
		}
	}
}

// ============================================================================
// Multilib Usedep Tests
// ============================================================================

func TestComputeMultilibUsedep(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	usedep := h.computeMultilibUsedep()
	if !strings.Contains(usedep, "abi_x86_64") {
		t.Errorf("usedep should contain abi_x86_64: %s", usedep)
	}
	if !strings.Contains(usedep, "abi_x86_32") {
		t.Errorf("usedep should contain abi_x86_32: %s", usedep)
	}
}

func TestMultilibUsedep(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	err := h.MultilibUsedep(nil)
	if err != nil {
		t.Fatalf("MultilibUsedep error: %v", err)
	}

	got := stdout.String()
	if got == "" {
		t.Error("expected non-empty usedep string")
	}
}
