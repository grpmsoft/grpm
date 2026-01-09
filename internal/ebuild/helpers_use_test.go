package ebuild

import (
	"errors"
	"testing"

	"mvdan.cc/sh/v3/interp"
)

// ============================================================================
// Has Tests
// ============================================================================

func TestHelpers_Has_Found(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	err := helpers.Has([]string{"foo", "bar", "foo", "baz"})
	if err != nil {
		t.Errorf("expected nil error when found, got: %v", err)
	}
}

func TestHelpers_Has_NotFound(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	err := helpers.Has([]string{"qux", "bar", "foo", "baz"})
	if err == nil {
		t.Error("expected error when not found")
	}

	var exitErr interp.ExitStatus
	if !errors.As(err, &exitErr) {
		t.Errorf("expected exit status error, got: %T", err)
	}
	if uint8(exitErr) != 1 {
		t.Errorf("expected exit code 1, got: %d", uint8(exitErr))
	}
}

func TestHelpers_Has_TooFewArgs(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	err := helpers.Has([]string{"foo"})
	if err == nil {
		t.Error("expected error with only needle")
	}
}

func TestHelpers_Has_EmptyArgs(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	err := helpers.Has([]string{})
	if err == nil {
		t.Error("expected error with no args")
	}
}

// ============================================================================
// Use Tests
// ============================================================================

func TestHelpers_Use_Enabled(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// ssl is enabled
	err := helpers.Use([]string{"ssl"})
	if err != nil {
		t.Errorf("expected nil for enabled flag, got: %v", err)
	}
}

func TestHelpers_Use_Disabled(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// doc is disabled
	err := helpers.Use([]string{"doc"})
	if err == nil {
		t.Error("expected error for disabled flag")
	}
}

func TestHelpers_Use_Negation_DisabledFlag(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// !doc should return success because doc is disabled
	err := helpers.Use([]string{"!doc"})
	if err != nil {
		t.Errorf("expected nil for negated disabled flag, got: %v", err)
	}
}

func TestHelpers_Use_Negation_EnabledFlag(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// !ssl should return error because ssl is enabled
	err := helpers.Use([]string{"!ssl"})
	if err == nil {
		t.Error("expected error for negated enabled flag")
	}
}

func TestHelpers_Use_UnknownFlag(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// unknown flag should return error (not in IUSE)
	err := helpers.Use([]string{"unknownflag"})
	if err == nil {
		t.Error("expected error for unknown flag")
	}
}

func TestHelpers_Use_NoArgs(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	err := helpers.Use([]string{})
	if err == nil {
		t.Error("expected error with no args")
	}
}

// ============================================================================
// Usev Tests
// ============================================================================

func TestHelpers_Usev_Enabled(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Usev([]string{"ssl"})
	if err != nil {
		t.Errorf("expected nil for enabled flag, got: %v", err)
	}

	output := stdout.String()
	if output != "ssl" {
		t.Errorf("expected 'ssl', got: %s", output)
	}
}

func TestHelpers_Usev_Disabled(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Usev([]string{"doc"})
	if err == nil {
		t.Error("expected error for disabled flag")
	}

	// Should not output anything
	if stdout.Len() > 0 {
		t.Errorf("expected no output for disabled flag, got: %s", stdout.String())
	}
}

func TestHelpers_Usev_CustomValue(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Usev([]string{"ssl", "--with-openssl"})
	if err != nil {
		t.Errorf("expected nil for enabled flag, got: %v", err)
	}

	output := stdout.String()
	if output != "--with-openssl" {
		t.Errorf("expected '--with-openssl', got: %s", output)
	}
}

// ============================================================================
// Usex Tests
// ============================================================================

func TestHelpers_Usex_Enabled_Default(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Usex([]string{"ssl"})
	if err != nil {
		t.Fatalf("Usex failed: %v", err)
	}

	output := stdout.String()
	if output != "yes" {
		t.Errorf("expected 'yes', got: %s", output)
	}
}

func TestHelpers_Usex_Disabled_Default(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Usex([]string{"doc"})
	if err != nil {
		t.Fatalf("Usex failed: %v", err)
	}

	output := stdout.String()
	if output != "no" {
		t.Errorf("expected 'no', got: %s", output)
	}
}

func TestHelpers_Usex_CustomValues(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Usex([]string{"ssl", "ON", "OFF"})
	if err != nil {
		t.Fatalf("Usex failed: %v", err)
	}

	output := stdout.String()
	if output != "ON" {
		t.Errorf("expected 'ON', got: %s", output)
	}
}

func TestHelpers_Usex_WithSuffix(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Usex([]string{"ssl", "yes", "no", "-ssl", ""})
	if err != nil {
		t.Fatalf("Usex failed: %v", err)
	}

	output := stdout.String()
	if output != "yes-ssl" {
		t.Errorf("expected 'yes-ssl', got: %s", output)
	}
}

func TestHelpers_Usex_NoArgs(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	err := helpers.Usex([]string{})
	if err == nil {
		t.Error("expected error with no args")
	}
}

// ============================================================================
// InIuse Tests
// ============================================================================

func TestHelpers_InIuse_Present(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// ssl is in UseFlags map (IUSE)
	err := helpers.InIuse([]string{"ssl"})
	if err != nil {
		t.Errorf("expected nil for flag in IUSE, got: %v", err)
	}
}

func TestHelpers_InIuse_NotPresent(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// unknownflag is not in UseFlags map
	err := helpers.InIuse([]string{"unknownflag"})
	if err == nil {
		t.Error("expected error for flag not in IUSE")
	}
}

func TestHelpers_InIuse_NoArgs(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	err := helpers.InIuse([]string{})
	if err == nil {
		t.Error("expected error with no args")
	}
}

// ============================================================================
// UseEnable Tests (PMS Section 11.3.2.2)
// ============================================================================

func TestHelpers_UseEnable_EnabledFlag_OneArg(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// ssl is enabled
	err := helpers.UseEnable([]string{"ssl"})
	if err != nil {
		t.Fatalf("UseEnable failed: %v", err)
	}

	output := stdout.String()
	if output != "--enable-ssl" {
		t.Errorf("expected '--enable-ssl', got: %s", output)
	}
}

func TestHelpers_UseEnable_DisabledFlag_OneArg(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// doc is disabled
	err := helpers.UseEnable([]string{"doc"})
	if err != nil {
		t.Fatalf("UseEnable failed: %v", err)
	}

	output := stdout.String()
	if output != "--disable-doc" {
		t.Errorf("expected '--disable-doc', got: %s", output)
	}
}

func TestHelpers_UseEnable_EnabledFlag_TwoArgs(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// ssl is enabled, use custom option name
	err := helpers.UseEnable([]string{"ssl", "openssl"})
	if err != nil {
		t.Fatalf("UseEnable failed: %v", err)
	}

	output := stdout.String()
	if output != "--enable-openssl" {
		t.Errorf("expected '--enable-openssl', got: %s", output)
	}
}

func TestHelpers_UseEnable_DisabledFlag_TwoArgs(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// doc is disabled, use custom option name
	err := helpers.UseEnable([]string{"doc", "documentation"})
	if err != nil {
		t.Fatalf("UseEnable failed: %v", err)
	}

	output := stdout.String()
	if output != "--disable-documentation" {
		t.Errorf("expected '--disable-documentation', got: %s", output)
	}
}

func TestHelpers_UseEnable_EnabledFlag_ThreeArgs(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// ssl is enabled: use_enable ssl openssl yes
	err := helpers.UseEnable([]string{"ssl", "openssl", "yes"})
	if err != nil {
		t.Fatalf("UseEnable failed: %v", err)
	}

	output := stdout.String()
	if output != "--enable-openssl=yes" {
		t.Errorf("expected '--enable-openssl=yes', got: %s", output)
	}
}

func TestHelpers_UseEnable_DisabledFlag_ThreeArgs(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// doc is disabled: use_enable doc docs yes -> --disable-docs
	err := helpers.UseEnable([]string{"doc", "docs", "yes"})
	if err != nil {
		t.Fatalf("UseEnable failed: %v", err)
	}

	output := stdout.String()
	if output != "--disable-docs" {
		t.Errorf("expected '--disable-docs', got: %s", output)
	}
}

func TestHelpers_UseEnable_EnabledFlag_FourArgs(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// ssl is enabled: use_enable ssl openssl yes no -> --openssl=yes
	err := helpers.UseEnable([]string{"ssl", "openssl", "yes", "no"})
	if err != nil {
		t.Fatalf("UseEnable failed: %v", err)
	}

	output := stdout.String()
	if output != "--openssl=yes" {
		t.Errorf("expected '--openssl=yes', got: %s", output)
	}
}

func TestHelpers_UseEnable_DisabledFlag_FourArgs(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// doc is disabled: use_enable doc docs yes no -> --docs=no
	err := helpers.UseEnable([]string{"doc", "docs", "yes", "no"})
	if err != nil {
		t.Fatalf("UseEnable failed: %v", err)
	}

	output := stdout.String()
	if output != "--docs=no" {
		t.Errorf("expected '--docs=no', got: %s", output)
	}
}

func TestHelpers_UseEnable_NoArgs(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	err := helpers.UseEnable([]string{})
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestHelpers_UseEnable_UnknownFlag(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// unknownflag is not in IUSE, should be treated as disabled
	err := helpers.UseEnable([]string{"unknownflag"})
	if err != nil {
		t.Fatalf("UseEnable failed: %v", err)
	}

	output := stdout.String()
	if output != "--disable-unknownflag" {
		t.Errorf("expected '--disable-unknownflag', got: %s", output)
	}
}

// ============================================================================
// UseWith Tests (PMS Section 11.3.2.3)
// ============================================================================

func TestHelpers_UseWith_EnabledFlag_OneArg(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// ssl is enabled
	err := helpers.UseWith([]string{"ssl"})
	if err != nil {
		t.Fatalf("UseWith failed: %v", err)
	}

	output := stdout.String()
	if output != "--with-ssl" {
		t.Errorf("expected '--with-ssl', got: %s", output)
	}
}

func TestHelpers_UseWith_DisabledFlag_OneArg(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// doc is disabled
	err := helpers.UseWith([]string{"doc"})
	if err != nil {
		t.Fatalf("UseWith failed: %v", err)
	}

	output := stdout.String()
	if output != "--without-doc" {
		t.Errorf("expected '--without-doc', got: %s", output)
	}
}

func TestHelpers_UseWith_EnabledFlag_TwoArgs(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// ssl is enabled, use custom option name
	err := helpers.UseWith([]string{"ssl", "openssl"})
	if err != nil {
		t.Fatalf("UseWith failed: %v", err)
	}

	output := stdout.String()
	if output != "--with-openssl" {
		t.Errorf("expected '--with-openssl', got: %s", output)
	}
}

func TestHelpers_UseWith_DisabledFlag_TwoArgs(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// doc is disabled, use custom option name
	err := helpers.UseWith([]string{"doc", "documentation"})
	if err != nil {
		t.Fatalf("UseWith failed: %v", err)
	}

	output := stdout.String()
	if output != "--without-documentation" {
		t.Errorf("expected '--without-documentation', got: %s", output)
	}
}

func TestHelpers_UseWith_EnabledFlag_ThreeArgs(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// ssl is enabled: use_with ssl openssl /usr
	err := helpers.UseWith([]string{"ssl", "openssl", "/usr"})
	if err != nil {
		t.Fatalf("UseWith failed: %v", err)
	}

	output := stdout.String()
	if output != "--with-openssl=/usr" {
		t.Errorf("expected '--with-openssl=/usr', got: %s", output)
	}
}

func TestHelpers_UseWith_DisabledFlag_ThreeArgs(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// doc is disabled: use_with doc docs /path -> --without-docs
	err := helpers.UseWith([]string{"doc", "docs", "/path"})
	if err != nil {
		t.Fatalf("UseWith failed: %v", err)
	}

	output := stdout.String()
	if output != "--without-docs" {
		t.Errorf("expected '--without-docs', got: %s", output)
	}
}

func TestHelpers_UseWith_EnabledFlag_FourArgs(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// ssl is enabled: use_with ssl openssl /usr/lib /none -> --openssl=/usr/lib
	err := helpers.UseWith([]string{"ssl", "openssl", "/usr/lib", "/none"})
	if err != nil {
		t.Fatalf("UseWith failed: %v", err)
	}

	output := stdout.String()
	if output != "--openssl=/usr/lib" {
		t.Errorf("expected '--openssl=/usr/lib', got: %s", output)
	}
}

func TestHelpers_UseWith_DisabledFlag_FourArgs(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// doc is disabled: use_with doc docs yes no -> --docs=no
	err := helpers.UseWith([]string{"doc", "docs", "yes", "no"})
	if err != nil {
		t.Fatalf("UseWith failed: %v", err)
	}

	output := stdout.String()
	if output != "--docs=no" {
		t.Errorf("expected '--docs=no', got: %s", output)
	}
}

func TestHelpers_UseWith_NoArgs(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	err := helpers.UseWith([]string{})
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestHelpers_UseWith_UnknownFlag(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	// unknownflag is not in IUSE, should be treated as disabled
	err := helpers.UseWith([]string{"unknownflag"})
	if err != nil {
		t.Fatalf("UseWith failed: %v", err)
	}

	output := stdout.String()
	if output != "--without-unknownflag" {
		t.Errorf("expected '--without-unknownflag', got: %s", output)
	}
}

// ============================================================================
// Toolchain Function Tests
// ============================================================================

func TestHelpers_TcGetCC(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.TcGetCC([]string{})
	if err != nil {
		t.Fatalf("TcGetCC failed: %v", err)
	}

	output := stdout.String()
	if output != "gcc" {
		t.Errorf("expected 'gcc', got: %s", output)
	}
}

func TestHelpers_TcGetCXX(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.TcGetCXX([]string{})
	if err != nil {
		t.Fatalf("TcGetCXX failed: %v", err)
	}

	output := stdout.String()
	if output != "g++" {
		t.Errorf("expected 'g++', got: %s", output)
	}
}

func TestHelpers_TcGetLD(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.TcGetLD([]string{})
	if err != nil {
		t.Fatalf("TcGetLD failed: %v", err)
	}

	output := stdout.String()
	if output != "ld" {
		t.Errorf("expected 'ld', got: %s", output)
	}
}

func TestHelpers_TcArch(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.TcArch([]string{})
	if err != nil {
		t.Fatalf("TcArch failed: %v", err)
	}

	output := stdout.String()
	if output == "" {
		t.Error("expected non-empty architecture")
	}

	// Should be a valid Gentoo arch
	validArches := []string{"amd64", "x86", "arm", "arm64", "ppc64", "riscv", "s390", "mips"}
	found := false
	for _, arch := range validArches {
		if output == arch {
			found = true
			break
		}
	}
	if !found {
		t.Logf("architecture '%s' may be valid but not in common list", output)
	}
}
