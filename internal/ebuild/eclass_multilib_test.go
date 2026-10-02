// Package ebuild implements ebuild execution engine.
//
// This file contains tests for multilib eclasses.
package ebuild

import (
	"bytes"
	"context"
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

// ============================================================================
// CallFunction / callMultilibPhase Integration Tests
// ============================================================================

// createMultilibTestEnv creates an environment with CHOST set for multilib tests.
func createMultilibTestEnv(t *testing.T) *Environment {
	t.Helper()
	env := createTestEnvironment(t)
	env.SetVar("CHOST", "x86_64-pc-linux-gnu")
	return env
}

func TestCallMultilibPhase_CallsBashFunction(t *testing.T) {
	// Test that callMultilibPhase invokes a bash-defined function via the interpreter.
	// Define a bash function, then call it through callMultilibPhase.
	env := createMultilibTestEnv(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// Define a function and call it through callMultilibPhase in one script.
	script := `
multilib_src_configure() {
	echo "custom_configure_called"
}
# Now invoke multilib-minimal phase which should call our function.
multilib-minimal_src_configure
`
	err := interp.Run(context.Background(), script)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "custom_configure_called") {
		t.Errorf("expected bash function to be called, got output: %s", output)
	}
}

func TestCallMultilibPhase_FallbackWhenNotDefined(t *testing.T) {
	// When multilib_src_test is not defined in the ebuild, callMultilibPhase
	// should fall back to the default (no-op for src_test).
	env := createMultilibTestEnv(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// Call multilib-minimal_src_test WITHOUT defining multilib_src_test.
	// The default fallback for src_test is no-op, so no error expected.
	script := `multilib-minimal_src_test`
	err := interp.Run(context.Background(), script)
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}
}

func TestCallMultilibPhase_PropagatesBashFunctionError(t *testing.T) {
	// If the bash function exists but fails (returns non-zero),
	// the error should be propagated, not swallowed.
	env := createMultilibTestEnv(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	script := `
multilib_src_configure() {
	echo "will fail"
	return 1
}
multilib-minimal_src_configure
`
	err := interp.Run(context.Background(), script)
	if err == nil {
		t.Fatal("expected error from failing bash function")
	}
}

func TestCallBashFunction_NoInterpreter(t *testing.T) {
	// Without a function caller (unit test mode), callBashFunction returns error.
	env := &Environment{}
	var stdout, stderr bytes.Buffer
	h := NewHelpers(env, &stdout, &stderr)

	err := h.callBashFunction("some_function", nil)
	if err == nil {
		t.Fatal("expected error when no interpreter available")
	}
	if !strings.Contains(err.Error(), "no interpreter") {
		t.Errorf("expected 'no interpreter' in error, got: %s", err.Error())
	}
}

func TestCallMultilibPhase_FallbackWithoutInterpreter(t *testing.T) {
	// Without function caller, callMultilibPhase always uses fallback.
	env := &Environment{}
	var stdout, stderr bytes.Buffer
	h := NewHelpers(env, &stdout, &stderr)

	fallbackCalled := false
	err := h.callMultilibPhase("nonexistent_func", func() error {
		fallbackCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fallbackCalled {
		t.Error("expected fallback to be called when no interpreter available")
	}
}

func TestMultilibForeachABI_CallsBashFunction(t *testing.T) {
	// Test that multilib_foreach_abi can invoke a bash-defined function.
	env := createMultilibTestEnv(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	script := `
my_custom_abi_func() {
	echo "abi_func_for_${ABI}"
}
multilib_foreach_abi my_custom_abi_func
`
	err := interp.Run(context.Background(), script)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	// Should have called the function for at least the native ABI
	if !strings.Contains(output, "abi_func_for_") {
		t.Errorf("expected bash function to be called per-ABI, got: %s", output)
	}
}

func TestMultilibForeachABI_WithGoHelper(t *testing.T) {
	// Test that multilib_foreach_abi works with Go helpers (like einfo).
	env := createMultilibTestEnv(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	script := `multilib_foreach_abi einfo "building"`
	err := interp.Run(context.Background(), script)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestCallFunction_ActiveRunner(t *testing.T) {
	// Verify CallFunction works during Run() and fails outside.
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// Outside Run(), CallFunction should fail.
	err := interp.CallFunction("echo test")
	if err == nil {
		t.Fatal("expected error when calling CallFunction outside Run()")
	}
	if !strings.Contains(err.Error(), "no active runner") {
		t.Errorf("expected 'no active runner' error, got: %s", err.Error())
	}

	// During Run(), it should succeed.
	err = interp.Run(context.Background(), `
test_func() {
	echo "from_test_func"
}
test_func
`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "from_test_func") {
		t.Errorf("expected function output, got: %s", stdout.String())
	}
}

func TestMultilibBuildSrcInstall_CallsEbuildFunction(t *testing.T) {
	// Verify that multilib-minimal_src_install calls the ebuild's
	// multilib_src_install function instead of the hardcoded default.
	env := createMultilibTestEnv(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	script := `
multilib_src_install() {
	echo "custom_install_for_${ABI}"
}
multilib-minimal_src_install
`
	err := interp.Run(context.Background(), script)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "custom_install_for_") {
		t.Errorf("expected custom install function, got: %s", output)
	}
}

func TestMultilibBuildSrcCompile_CallsEbuildFunction(t *testing.T) {
	// Verify multilib-minimal_src_compile calls multilib_src_compile.
	env := createMultilibTestEnv(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	script := `
multilib_src_compile() {
	echo "custom_compile_for_${ABI}"
}
multilib-minimal_src_compile
`
	err := interp.Run(context.Background(), script)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "custom_compile_for_") {
		t.Errorf("expected custom compile function, got: %s", output)
	}
}
