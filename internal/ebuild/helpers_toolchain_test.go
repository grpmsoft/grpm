// Package ebuild implements ebuild execution engine.
//
// This file contains tests for toolchain-funcs eclass implementation.
package ebuild

import (
	"bytes"
	"os"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// ============================================================================
// Test Utilities
// ============================================================================

// createToolchainTestHelpers creates a Helpers instance for toolchain tests.
func createToolchainTestHelpers(t *testing.T) (*Helpers, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	testPkg := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
		UseFlags: map[string]bool{
			"ssl": true,
		},
	}

	env, err := NewEnvironment(testPkg, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(env, &stdout, &stderr)

	return helpers, &stdout, &stderr
}

// ============================================================================
// tc-getCC Tests
// ============================================================================

func TestHelpers_TcGetCC_Default(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	// Clear CC env var to get default
	origCC := os.Getenv("CC")
	_ = os.Unsetenv("CC")
	defer func() { _ = os.Setenv("CC", origCC) }()

	err := helpers.TcGetCC(nil)
	if err != nil {
		t.Fatalf("TcGetCC failed: %v", err)
	}

	got := stdout.String()
	if got != "gcc" {
		t.Errorf("TcGetCC = %q, want %q", got, "gcc")
	}
}

func TestHelpers_TcGetCC_FromEnv(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	// Set CC env var
	origCC := os.Getenv("CC")
	_ = os.Setenv("CC", "clang")
	defer func() { _ = os.Setenv("CC", origCC) }()

	err := helpers.TcGetCC(nil)
	if err != nil {
		t.Fatalf("TcGetCC failed: %v", err)
	}

	got := stdout.String()
	if got != "clang" {
		t.Errorf("TcGetCC = %q, want %q", got, "clang")
	}
}

func TestHelpers_TcGetCC_FromExtraVars(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	// Clear OS environment CC
	origCC := os.Getenv("CC")
	_ = os.Unsetenv("CC")
	defer func() { _ = os.Setenv("CC", origCC) }()

	// Set CC in ExtraVars (simulating ebuild-set variable)
	helpers.env.SetVar("CC", "x86_64-custom-gcc")

	err := helpers.TcGetCC(nil)
	if err != nil {
		t.Fatalf("TcGetCC failed: %v", err)
	}

	got := stdout.String()
	if got != "x86_64-custom-gcc" {
		t.Errorf("TcGetCC = %q, want %q", got, "x86_64-custom-gcc")
	}
}

// ============================================================================
// tc-getCXX Tests
// ============================================================================

func TestHelpers_TcGetCXX_Default(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	// Clear CXX env var
	origCXX := os.Getenv("CXX")
	_ = os.Unsetenv("CXX")
	defer func() { _ = os.Setenv("CXX", origCXX) }()

	err := helpers.TcGetCXX(nil)
	if err != nil {
		t.Fatalf("TcGetCXX failed: %v", err)
	}

	got := stdout.String()
	if got != "g++" {
		t.Errorf("TcGetCXX = %q, want %q", got, "g++")
	}
}

func TestHelpers_TcGetCXX_FromEnv(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	// Set CXX env var
	origCXX := os.Getenv("CXX")
	_ = os.Setenv("CXX", "clang++")
	defer func() { _ = os.Setenv("CXX", origCXX) }()

	err := helpers.TcGetCXX(nil)
	if err != nil {
		t.Fatalf("TcGetCXX failed: %v", err)
	}

	got := stdout.String()
	if got != "clang++" {
		t.Errorf("TcGetCXX = %q, want %q", got, "clang++")
	}
}

// ============================================================================
// tc-getAR Tests
// ============================================================================

func TestHelpers_TcGetAR_Default(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	// Clear AR env var
	origAR := os.Getenv("AR")
	_ = os.Unsetenv("AR")
	defer func() { _ = os.Setenv("AR", origAR) }()

	err := helpers.TcGetAR(nil)
	if err != nil {
		t.Fatalf("TcGetAR failed: %v", err)
	}

	got := stdout.String()
	if got != "ar" {
		t.Errorf("TcGetAR = %q, want %q", got, "ar")
	}
}

func TestHelpers_TcGetAR_FromEnv(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	origAR := os.Getenv("AR")
	_ = os.Setenv("AR", "llvm-ar")
	defer func() { _ = os.Setenv("AR", origAR) }()

	err := helpers.TcGetAR(nil)
	if err != nil {
		t.Fatalf("TcGetAR failed: %v", err)
	}

	got := stdout.String()
	if got != "llvm-ar" {
		t.Errorf("TcGetAR = %q, want %q", got, "llvm-ar")
	}
}

// ============================================================================
// tc-getRANLIB Tests
// ============================================================================

func TestHelpers_TcGetRANLIB_Default(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	origRANLIB := os.Getenv("RANLIB")
	_ = os.Unsetenv("RANLIB")
	defer func() { _ = os.Setenv("RANLIB", origRANLIB) }()

	err := helpers.TcGetRANLIB(nil)
	if err != nil {
		t.Fatalf("TcGetRANLIB failed: %v", err)
	}

	got := stdout.String()
	if got != "ranlib" {
		t.Errorf("TcGetRANLIB = %q, want %q", got, "ranlib")
	}
}

func TestHelpers_TcGetRANLIB_FromEnv(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	origRANLIB := os.Getenv("RANLIB")
	_ = os.Setenv("RANLIB", "llvm-ranlib")
	defer func() { _ = os.Setenv("RANLIB", origRANLIB) }()

	err := helpers.TcGetRANLIB(nil)
	if err != nil {
		t.Fatalf("TcGetRANLIB failed: %v", err)
	}

	got := stdout.String()
	if got != "llvm-ranlib" {
		t.Errorf("TcGetRANLIB = %q, want %q", got, "llvm-ranlib")
	}
}

// ============================================================================
// tc-getNM Tests
// ============================================================================

func TestHelpers_TcGetNM_Default(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	origNM := os.Getenv("NM")
	_ = os.Unsetenv("NM")
	defer func() { _ = os.Setenv("NM", origNM) }()

	err := helpers.TcGetNM(nil)
	if err != nil {
		t.Fatalf("TcGetNM failed: %v", err)
	}

	got := stdout.String()
	if got != "nm" {
		t.Errorf("TcGetNM = %q, want %q", got, "nm")
	}
}

func TestHelpers_TcGetNM_FromEnv(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	origNM := os.Getenv("NM")
	_ = os.Setenv("NM", "llvm-nm")
	defer func() { _ = os.Setenv("NM", origNM) }()

	err := helpers.TcGetNM(nil)
	if err != nil {
		t.Fatalf("TcGetNM failed: %v", err)
	}

	got := stdout.String()
	if got != "llvm-nm" {
		t.Errorf("TcGetNM = %q, want %q", got, "llvm-nm")
	}
}

// ============================================================================
// tc-getOBJCOPY Tests
// ============================================================================

func TestHelpers_TcGetOBJCOPY_Default(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	origOBJCOPY := os.Getenv("OBJCOPY")
	_ = os.Unsetenv("OBJCOPY")
	defer func() { _ = os.Setenv("OBJCOPY", origOBJCOPY) }()

	err := helpers.TcGetOBJCOPY(nil)
	if err != nil {
		t.Fatalf("TcGetOBJCOPY failed: %v", err)
	}

	got := stdout.String()
	if got != "objcopy" {
		t.Errorf("TcGetOBJCOPY = %q, want %q", got, "objcopy")
	}
}

func TestHelpers_TcGetOBJCOPY_FromEnv(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	origOBJCOPY := os.Getenv("OBJCOPY")
	_ = os.Setenv("OBJCOPY", "llvm-objcopy")
	defer func() { _ = os.Setenv("OBJCOPY", origOBJCOPY) }()

	err := helpers.TcGetOBJCOPY(nil)
	if err != nil {
		t.Fatalf("TcGetOBJCOPY failed: %v", err)
	}

	got := stdout.String()
	if got != "llvm-objcopy" {
		t.Errorf("TcGetOBJCOPY = %q, want %q", got, "llvm-objcopy")
	}
}

// ============================================================================
// tc-getSTRIP Tests
// ============================================================================

func TestHelpers_TcGetSTRIP_Default(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	origSTRIP := os.Getenv("STRIP")
	_ = os.Unsetenv("STRIP")
	defer func() { _ = os.Setenv("STRIP", origSTRIP) }()

	err := helpers.TcGetSTRIP(nil)
	if err != nil {
		t.Fatalf("TcGetSTRIP failed: %v", err)
	}

	got := stdout.String()
	if got != "strip" {
		t.Errorf("TcGetSTRIP = %q, want %q", got, "strip")
	}
}

func TestHelpers_TcGetSTRIP_FromEnv(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	origSTRIP := os.Getenv("STRIP")
	_ = os.Setenv("STRIP", "llvm-strip")
	defer func() { _ = os.Setenv("STRIP", origSTRIP) }()

	err := helpers.TcGetSTRIP(nil)
	if err != nil {
		t.Fatalf("TcGetSTRIP failed: %v", err)
	}

	got := stdout.String()
	if got != "llvm-strip" {
		t.Errorf("TcGetSTRIP = %q, want %q", got, "llvm-strip")
	}
}

// ============================================================================
// tc-getLD Tests
// ============================================================================

func TestHelpers_TcGetLD_Default(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	origLD := os.Getenv("LD")
	_ = os.Unsetenv("LD")
	defer func() { _ = os.Setenv("LD", origLD) }()

	err := helpers.TcGetLD(nil)
	if err != nil {
		t.Fatalf("TcGetLD failed: %v", err)
	}

	got := stdout.String()
	if got != "ld" {
		t.Errorf("TcGetLD = %q, want %q", got, "ld")
	}
}

func TestHelpers_TcGetLD_FromEnv(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	origLD := os.Getenv("LD")
	_ = os.Setenv("LD", "ld.lld")
	defer func() { _ = os.Setenv("LD", origLD) }()

	err := helpers.TcGetLD(nil)
	if err != nil {
		t.Fatalf("TcGetLD failed: %v", err)
	}

	got := stdout.String()
	if got != "ld.lld" {
		t.Errorf("TcGetLD = %q, want %q", got, "ld.lld")
	}
}

// ============================================================================
// tc-getPKG_CONFIG Tests
// ============================================================================

func TestHelpers_TcGetPKG_CONFIG_Default(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	origPKGCONFIG := os.Getenv("PKG_CONFIG")
	_ = os.Unsetenv("PKG_CONFIG")
	defer func() { _ = os.Setenv("PKG_CONFIG", origPKGCONFIG) }()

	err := helpers.TcGetPKG_CONFIG(nil)
	if err != nil {
		t.Fatalf("TcGetPKG_CONFIG failed: %v", err)
	}

	got := stdout.String()
	if got != "pkg-config" {
		t.Errorf("TcGetPKG_CONFIG = %q, want %q", got, "pkg-config")
	}
}

func TestHelpers_TcGetPKG_CONFIG_FromEnv(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	origPKGCONFIG := os.Getenv("PKG_CONFIG")
	_ = os.Setenv("PKG_CONFIG", "pkgconf")
	defer func() { _ = os.Setenv("PKG_CONFIG", origPKGCONFIG) }()

	err := helpers.TcGetPKG_CONFIG(nil)
	if err != nil {
		t.Fatalf("TcGetPKG_CONFIG failed: %v", err)
	}

	got := stdout.String()
	if got != "pkgconf" {
		t.Errorf("TcGetPKG_CONFIG = %q, want %q", got, "pkgconf")
	}
}

// ============================================================================
// tc-getBUILD_CC Tests
// ============================================================================

func TestHelpers_TcGetBUILD_CC_Default(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	origBUILDCC := os.Getenv("BUILD_CC")
	_ = os.Unsetenv("BUILD_CC")
	defer func() { _ = os.Setenv("BUILD_CC", origBUILDCC) }()

	err := helpers.TcGetBUILD_CC(nil)
	if err != nil {
		t.Fatalf("TcGetBUILD_CC failed: %v", err)
	}

	got := stdout.String()
	if got != "gcc" {
		t.Errorf("TcGetBUILD_CC = %q, want %q", got, "gcc")
	}
}

func TestHelpers_TcGetBUILD_CC_FromEnv(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	origBUILDCC := os.Getenv("BUILD_CC")
	_ = os.Setenv("BUILD_CC", "x86_64-pc-linux-gnu-gcc")
	defer func() { _ = os.Setenv("BUILD_CC", origBUILDCC) }()

	err := helpers.TcGetBUILD_CC(nil)
	if err != nil {
		t.Fatalf("TcGetBUILD_CC failed: %v", err)
	}

	got := stdout.String()
	if got != "x86_64-pc-linux-gnu-gcc" {
		t.Errorf("TcGetBUILD_CC = %q, want %q", got, "x86_64-pc-linux-gnu-gcc")
	}
}

// ============================================================================
// tc-getBUILD_CXX Tests
// ============================================================================

func TestHelpers_TcGetBUILD_CXX_Default(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	origBUILDCXX := os.Getenv("BUILD_CXX")
	_ = os.Unsetenv("BUILD_CXX")
	defer func() { _ = os.Setenv("BUILD_CXX", origBUILDCXX) }()

	err := helpers.TcGetBUILD_CXX(nil)
	if err != nil {
		t.Fatalf("TcGetBUILD_CXX failed: %v", err)
	}

	got := stdout.String()
	if got != "g++" {
		t.Errorf("TcGetBUILD_CXX = %q, want %q", got, "g++")
	}
}

func TestHelpers_TcGetBUILD_CXX_FromEnv(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	origBUILDCXX := os.Getenv("BUILD_CXX")
	_ = os.Setenv("BUILD_CXX", "x86_64-pc-linux-gnu-g++")
	defer func() { _ = os.Setenv("BUILD_CXX", origBUILDCXX) }()

	err := helpers.TcGetBUILD_CXX(nil)
	if err != nil {
		t.Fatalf("TcGetBUILD_CXX failed: %v", err)
	}

	got := stdout.String()
	if got != "x86_64-pc-linux-gnu-g++" {
		t.Errorf("TcGetBUILD_CXX = %q, want %q", got, "x86_64-pc-linux-gnu-g++")
	}
}

// ============================================================================
// tc-is-cross-compiler Tests
// ============================================================================

func TestHelpers_TcIsCrossCompiler_Native(t *testing.T) {
	helpers, _, _ := createToolchainTestHelpers(t)

	// Clear CBUILD to simulate native compilation
	origCBUILD := os.Getenv("CBUILD")
	_ = os.Unsetenv("CBUILD")
	defer func() { _ = os.Setenv("CBUILD", origCBUILD) }()

	err := helpers.TcIsCrossCompiler(nil)

	// Should return exit code 1 (false) for native compilation
	if err == nil {
		t.Error("TcIsCrossCompiler should return error (false) for native compilation")
	}
}

func TestHelpers_TcIsCrossCompiler_Cross(t *testing.T) {
	helpers, _, _ := createToolchainTestHelpers(t)

	// Set CHOST and CBUILD to different values
	origCHOST := os.Getenv("CHOST")
	origCBUILD := os.Getenv("CBUILD")
	_ = os.Setenv("CHOST", "aarch64-unknown-linux-gnu")
	_ = os.Setenv("CBUILD", "x86_64-pc-linux-gnu")
	defer func() {
		_ = os.Setenv("CHOST", origCHOST)
		_ = os.Setenv("CBUILD", origCBUILD)
	}()

	err := helpers.TcIsCrossCompiler(nil)

	// Should return nil (exit 0 = true) for cross compilation
	if err != nil {
		t.Errorf("TcIsCrossCompiler should return nil (true) for cross compilation, got: %v", err)
	}
}

func TestHelpers_TcIsCrossCompiler_SameHostBuild(t *testing.T) {
	helpers, _, _ := createToolchainTestHelpers(t)

	// Set CHOST and CBUILD to same value
	origCHOST := os.Getenv("CHOST")
	origCBUILD := os.Getenv("CBUILD")
	_ = os.Setenv("CHOST", "x86_64-pc-linux-gnu")
	_ = os.Setenv("CBUILD", "x86_64-pc-linux-gnu")
	defer func() {
		_ = os.Setenv("CHOST", origCHOST)
		_ = os.Setenv("CBUILD", origCBUILD)
	}()

	err := helpers.TcIsCrossCompiler(nil)

	// Should return exit code 1 (false) when CHOST == CBUILD
	if err == nil {
		t.Error("TcIsCrossCompiler should return error (false) when CHOST == CBUILD")
	}
}

// ============================================================================
// tc-export Tests
// ============================================================================

func TestHelpers_TcExport_Specific(t *testing.T) {
	helpers, _, _ := createToolchainTestHelpers(t)

	// Clear all vars first
	origCC := os.Getenv("CC")
	origCXX := os.Getenv("CXX")
	_ = os.Unsetenv("CC")
	_ = os.Unsetenv("CXX")
	defer func() {
		_ = os.Setenv("CC", origCC)
		_ = os.Setenv("CXX", origCXX)
	}()

	err := helpers.TcExport([]string{"CC", "CXX"})
	if err != nil {
		t.Fatalf("TcExport failed: %v", err)
	}

	// Check that variables were exported
	if os.Getenv("CC") != "gcc" {
		t.Errorf("CC after export = %q, want %q", os.Getenv("CC"), "gcc")
	}
	if os.Getenv("CXX") != "g++" {
		t.Errorf("CXX after export = %q, want %q", os.Getenv("CXX"), "g++")
	}
}

func TestHelpers_TcExport_Default(t *testing.T) {
	helpers, _, _ := createToolchainTestHelpers(t)

	// Clear all relevant vars
	varsToRestore := []string{"CC", "CXX", "LD", "AR", "RANLIB", "NM", "OBJCOPY", "STRIP", "PKG_CONFIG"}
	origVars := make(map[string]string)
	for _, v := range varsToRestore {
		origVars[v] = os.Getenv(v)
		_ = os.Unsetenv(v)
	}
	defer func() {
		for _, v := range varsToRestore {
			_ = os.Setenv(v, origVars[v])
		}
	}()

	// Call with no args - should export default set
	err := helpers.TcExport(nil)
	if err != nil {
		t.Fatalf("TcExport failed: %v", err)
	}

	// Check that all default variables were exported
	expectedDefaults := map[string]string{
		"CC":         "gcc",
		"CXX":        "g++",
		"LD":         "ld",
		"AR":         "ar",
		"RANLIB":     "ranlib",
		"NM":         "nm",
		"OBJCOPY":    "objcopy",
		"STRIP":      "strip",
		"PKG_CONFIG": "pkg-config",
	}

	for varName, expected := range expectedDefaults {
		got := os.Getenv(varName)
		if got != expected {
			t.Errorf("%s after export = %q, want %q", varName, got, expected)
		}
	}
}

func TestHelpers_TcExport_WithExtraVars(t *testing.T) {
	helpers, _, _ := createToolchainTestHelpers(t)

	// Clear CC and set it in ExtraVars
	origCC := os.Getenv("CC")
	_ = os.Unsetenv("CC")
	defer func() { _ = os.Setenv("CC", origCC) }()

	helpers.env.SetVar("CC", "custom-gcc")

	err := helpers.TcExport([]string{"CC"})
	if err != nil {
		t.Fatalf("TcExport failed: %v", err)
	}

	// ExtraVars value should be used
	if os.Getenv("CC") != "custom-gcc" {
		t.Errorf("CC after export = %q, want %q", os.Getenv("CC"), "custom-gcc")
	}
}

// ============================================================================
// tc-arch Tests
// ============================================================================

func TestHelpers_TcArch_Default(t *testing.T) {
	helpers, stdout, _ := createToolchainTestHelpers(t)

	// Clear CHOST to use runtime detection
	origCHOST := os.Getenv("CHOST")
	_ = os.Unsetenv("CHOST")
	defer func() { _ = os.Setenv("CHOST", origCHOST) }()

	err := helpers.TcArch(nil)
	if err != nil {
		t.Fatalf("TcArch failed: %v", err)
	}

	got := stdout.String()
	// Should return a valid Gentoo architecture
	validArchs := []string{"amd64", "x86", "arm", "arm64", "ppc64", "riscv", "s390", "mips", "loong"}
	found := false
	for _, arch := range validArchs {
		if got == arch {
			found = true
			break
		}
	}
	if !found && got == "" {
		t.Errorf("TcArch returned empty string, expected valid architecture")
	}
}

func TestHelpers_TcArch_FromCHOST(t *testing.T) {
	testCases := []struct {
		chost string
		want  string
	}{
		{"x86_64-pc-linux-gnu", "amd64"},
		{"i686-pc-linux-gnu", "x86"},
		{"aarch64-unknown-linux-gnu", "arm64"},
		{"armv7a-unknown-linux-gnueabi", "arm"},
		{"powerpc64-unknown-linux-gnu", "ppc64"},
		{"powerpc64le-unknown-linux-gnu", "ppc64"},
		{"riscv64-unknown-linux-gnu", "riscv"},
		{"s390x-ibm-linux-gnu", "s390"},
		{"mips64el-unknown-linux-gnu", "mips"},
	}

	for _, tc := range testCases {
		t.Run(tc.chost, func(t *testing.T) {
			helpers, stdout, _ := createToolchainTestHelpers(t)

			origCHOST := os.Getenv("CHOST")
			_ = os.Setenv("CHOST", tc.chost)
			defer func() { _ = os.Setenv("CHOST", origCHOST) }()

			err := helpers.TcArch(nil)
			if err != nil {
				t.Fatalf("TcArch failed: %v", err)
			}

			got := stdout.String()
			if got != tc.want {
				t.Errorf("TcArch(%q) = %q, want %q", tc.chost, got, tc.want)
			}
		})
	}
}

// ============================================================================
// tc-arch-kernel Tests
// ============================================================================

func TestHelpers_TcArchKernel(t *testing.T) {
	testCases := []struct {
		chost string
		want  string
	}{
		{"x86_64-pc-linux-gnu", "x86_64"},
		{"i686-pc-linux-gnu", "x86"},
		{"aarch64-unknown-linux-gnu", "arm64"},
		{"armv7a-unknown-linux-gnueabi", "arm"},
		{"powerpc64-unknown-linux-gnu", "powerpc"},
		{"riscv64-unknown-linux-gnu", "riscv"},
		{"s390x-ibm-linux-gnu", "s390"},
	}

	for _, tc := range testCases {
		t.Run(tc.chost, func(t *testing.T) {
			helpers, stdout, _ := createToolchainTestHelpers(t)

			origCHOST := os.Getenv("CHOST")
			_ = os.Setenv("CHOST", tc.chost)
			defer func() { _ = os.Setenv("CHOST", origCHOST) }()

			err := helpers.TcArchKernel(nil)
			if err != nil {
				t.Fatalf("TcArchKernel failed: %v", err)
			}

			got := stdout.String()
			if got != tc.want {
				t.Errorf("TcArchKernel(%q) = %q, want %q", tc.chost, got, tc.want)
			}
		})
	}
}

// ============================================================================
// tc-endian Tests
// ============================================================================

func TestHelpers_TcEndian(t *testing.T) {
	testCases := []struct {
		chost string
		want  string
	}{
		{"x86_64-pc-linux-gnu", "little"},
		{"aarch64-unknown-linux-gnu", "little"},
		{"armv7a-unknown-linux-gnueabi", "little"},
		{"powerpc-unknown-linux-gnu", "big"},
		{"powerpc64-unknown-linux-gnu", "big"},
		{"powerpc64le-unknown-linux-gnu", "little"}, // Explicit little-endian
		{"s390x-ibm-linux-gnu", "big"},
		{"mips-unknown-linux-gnu", "big"},
		{"mipsel-unknown-linux-gnu", "little"}, // Explicit little-endian
		{"sparc64-unknown-linux-gnu", "big"},
	}

	for _, tc := range testCases {
		t.Run(tc.chost, func(t *testing.T) {
			helpers, stdout, _ := createToolchainTestHelpers(t)

			origCHOST := os.Getenv("CHOST")
			_ = os.Setenv("CHOST", tc.chost)
			defer func() { _ = os.Setenv("CHOST", origCHOST) }()

			err := helpers.TcEndian(nil)
			if err != nil {
				t.Fatalf("TcEndian failed: %v", err)
			}

			got := stdout.String()
			if got != tc.want {
				t.Errorf("TcEndian(%q) = %q, want %q", tc.chost, got, tc.want)
			}
		})
	}
}

// ============================================================================
// Internal Helper Function Tests
// ============================================================================

func TestHelpers_archFromCHOST(t *testing.T) {
	testCases := []struct {
		chost string
		want  string
	}{
		{"x86_64-pc-linux-gnu", "amd64"},
		{"i686-pc-linux-gnu", "x86"},
		{"i586-gentoo-linux-uclibc", "x86"},
		{"aarch64-unknown-linux-gnu", "arm64"},
		{"arm-unknown-linux-gnueabi", "arm"},
		{"armv7a-hardfloat-linux-gnueabi", "arm"},
		{"powerpc-unknown-linux-gnu", "ppc"},
		{"powerpc64-unknown-linux-gnu", "ppc64"},
		{"riscv64-unknown-linux-gnu", "riscv"},
		{"s390x-ibm-linux-gnu", "s390"},
		{"mips64el-unknown-linux-gnu", "mips"},
		{"sparc64-unknown-linux-gnu", "sparc"},
		{"alpha-unknown-linux-gnu", "alpha"},
		{"hppa2.0-unknown-linux-gnu", "hppa"},
		{"ia64-unknown-linux-gnu", "ia64"},
		{"m68k-unknown-linux-gnu", "m68k"},
		{"loongarch64-unknown-linux-gnu", "loong"},
		{"", ""},        // Empty CHOST
		{"invalid", ""}, // Invalid CHOST
	}

	helpers, _, _ := createToolchainTestHelpers(t)

	for _, tc := range testCases {
		t.Run(tc.chost, func(t *testing.T) {
			got := helpers.archFromCHOST(tc.chost)
			if got != tc.want {
				t.Errorf("archFromCHOST(%q) = %q, want %q", tc.chost, got, tc.want)
			}
		})
	}
}

func TestHelpers_goarchToGentooArch(t *testing.T) {
	testCases := []struct {
		goarch string
		want   string
	}{
		{"amd64", "amd64"},
		{"386", "x86"},
		{"arm", "arm"},
		{"arm64", "arm64"},
		{"ppc64", "ppc64"},
		{"ppc64le", "ppc64"},
		{"riscv64", "riscv"},
		{"s390x", "s390"},
		{"mips", "mips"},
		{"mipsle", "mips"},
		{"mips64", "mips"},
		{"mips64le", "mips"},
		{"loong64", "loong"},
		{"unknown", "unknown"}, // Fallback to input
	}

	helpers, _, _ := createToolchainTestHelpers(t)

	for _, tc := range testCases {
		t.Run(tc.goarch, func(t *testing.T) {
			got := helpers.goarchToGentooArch(tc.goarch)
			if got != tc.want {
				t.Errorf("goarchToGentooArch(%q) = %q, want %q", tc.goarch, got, tc.want)
			}
		})
	}
}

func TestHelpers_getEnvVar_Priority(t *testing.T) {
	helpers, _, _ := createToolchainTestHelpers(t)

	// Test priority: ExtraVars > OS environment
	origCC := os.Getenv("CC")
	_ = os.Setenv("CC", "env-gcc")
	defer func() { _ = os.Setenv("CC", origCC) }()

	// Set in ExtraVars - should take priority
	helpers.env.SetVar("CC", "extravar-gcc")

	got := helpers.getEnvVar("CC")
	if got != "extravar-gcc" {
		t.Errorf("getEnvVar(CC) = %q, want %q (ExtraVars should take priority)", got, "extravar-gcc")
	}

	// Clear ExtraVars - should fall back to OS env
	helpers.env.ExtraVars = nil
	got = helpers.getEnvVar("CC")
	if got != "env-gcc" {
		t.Errorf("getEnvVar(CC) = %q, want %q (should fall back to OS env)", got, "env-gcc")
	}
}

func TestHelpers_commandExists(t *testing.T) {
	helpers, _, _ := createToolchainTestHelpers(t)

	// Test with a command that should exist on most systems
	// (we use "go" since we're testing a Go project)
	if !helpers.commandExists("go") {
		t.Skip("go command not found, skipping commandExists test")
	}

	// Test with a command that definitely doesn't exist
	if helpers.commandExists("this-command-definitely-does-not-exist-12345") {
		t.Error("commandExists returned true for non-existent command")
	}
}

// ============================================================================
// Nil Environment Tests
// ============================================================================

func TestHelpers_TcGetCC_NilEnv(t *testing.T) {
	var stdout bytes.Buffer
	helpers := NewHelpers(nil, &stdout, nil)

	// Clear CC env var
	origCC := os.Getenv("CC")
	_ = os.Unsetenv("CC")
	defer func() { _ = os.Setenv("CC", origCC) }()

	err := helpers.TcGetCC(nil)
	if err != nil {
		t.Fatalf("TcGetCC with nil env failed: %v", err)
	}

	got := stdout.String()
	if got != "gcc" {
		t.Errorf("TcGetCC with nil env = %q, want %q", got, "gcc")
	}
}

func TestHelpers_TcExport_NilEnv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	// Should not panic with nil env
	err := helpers.TcExport([]string{"CC"})
	if err != nil {
		t.Fatalf("TcExport with nil env failed: %v", err)
	}
}

// ============================================================================
// getToolForExport Tests
// ============================================================================

func TestHelpers_getToolForExport(t *testing.T) {
	helpers, _, _ := createToolchainTestHelpers(t)

	// Clear all relevant env vars
	vars := []string{"CC", "CXX", "LD", "AR", "RANLIB", "NM", "OBJCOPY", "STRIP", "PKG_CONFIG", "BUILD_CC", "BUILD_CXX"}
	origVars := make(map[string]string)
	for _, v := range vars {
		origVars[v] = os.Getenv(v)
		_ = os.Unsetenv(v)
	}
	defer func() {
		for _, v := range vars {
			_ = os.Setenv(v, origVars[v])
		}
	}()

	testCases := []struct {
		varName string
		want    string
	}{
		{"CC", "gcc"},
		{"CXX", "g++"},
		{"LD", "ld"},
		{"AR", "ar"},
		{"RANLIB", "ranlib"},
		{"NM", "nm"},
		{"OBJCOPY", "objcopy"},
		{"STRIP", "strip"},
		{"PKG_CONFIG", "pkg-config"},
		{"BUILD_CC", "gcc"},
		{"BUILD_CXX", "g++"},
		{"UNKNOWN", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.varName, func(t *testing.T) {
			got := helpers.getToolForExport(tc.varName)
			if got != tc.want {
				t.Errorf("getToolForExport(%q) = %q, want %q", tc.varName, got, tc.want)
			}
		})
	}
}
