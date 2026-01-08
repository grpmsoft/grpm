package ebuild

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
	"mvdan.cc/sh/v3/interp"
)

// createTestHelpers creates a Helpers instance for testing.
func createTestHelpers(t *testing.T) (*Helpers, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	testPkg := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		UseFlags: map[string]bool{
			"ssl":     true,
			"zlib":    true,
			"doc":     false,
			"static":  false,
			"minizip": true,
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
// Die Tests
// ============================================================================

func TestHelpers_Die(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	err := helpers.Die([]string{"Something failed"})
	if err == nil {
		t.Fatal("expected error from Die")
	}

	var dieErr *DieError
	if !errors.As(err, &dieErr) {
		t.Errorf("expected DieError, got: %T", err)
	}

	if dieErr.Message != "Something failed" {
		t.Errorf("expected message 'Something failed', got: %s", dieErr.Message)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "Something failed") {
		t.Errorf("expected 'Something failed' in stderr, got: %s", errOutput)
	}
}

func TestHelpers_Die_EmptyMessage(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	err := helpers.Die([]string{})
	if err == nil {
		t.Fatal("expected error from Die")
	}

	var dieErr *DieError
	if !errors.As(err, &dieErr) {
		t.Errorf("expected DieError, got: %T", err)
	}

	if dieErr.Error() != "die called" {
		t.Errorf("expected 'die called', got: %s", dieErr.Error())
	}
}

// ============================================================================
// Einfo Tests
// ============================================================================

func TestHelpers_Einfo(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Einfo([]string{"Building", "package"})
	if err != nil {
		t.Fatalf("Einfo failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Building package") {
		t.Errorf("expected 'Building package', got: %s", output)
	}
	if !strings.Contains(output, "*") {
		t.Errorf("expected asterisk marker, got: %s", output)
	}
}

func TestHelpers_Einfo_WithColor(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Einfo([]string{"Test message"})
	if err != nil {
		t.Fatalf("Einfo failed: %v", err)
	}

	output := stdout.String()
	// Check for green color code
	if !strings.Contains(output, "\033[32m") {
		t.Errorf("expected green color code, got: %s", output)
	}
}

// ============================================================================
// Ewarn Tests
// ============================================================================

func TestHelpers_Ewarn(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	err := helpers.Ewarn([]string{"Warning message"})
	if err != nil {
		t.Fatalf("Ewarn failed: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "Warning message") {
		t.Errorf("expected 'Warning message' in stderr, got: %s", output)
	}
}

func TestHelpers_Ewarn_WithColor(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	err := helpers.Ewarn([]string{"Test"})
	if err != nil {
		t.Fatalf("Ewarn failed: %v", err)
	}

	output := stderr.String()
	// Check for yellow color code
	if !strings.Contains(output, "\033[33m") {
		t.Errorf("expected yellow color code, got: %s", output)
	}
}

// ============================================================================
// Eerror Tests
// ============================================================================

func TestHelpers_Eerror(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	err := helpers.Eerror([]string{"Error message"})
	if err != nil {
		t.Fatalf("Eerror failed: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "Error message") {
		t.Errorf("expected 'Error message' in stderr, got: %s", output)
	}
}

func TestHelpers_Eerror_WithColor(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	err := helpers.Eerror([]string{"Test"})
	if err != nil {
		t.Fatalf("Eerror failed: %v", err)
	}

	output := stderr.String()
	// Check for red color code
	if !strings.Contains(output, "\033[31m") {
		t.Errorf("expected red color code, got: %s", output)
	}
}

// ============================================================================
// Elog Tests
// ============================================================================

func TestHelpers_Elog(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Elog([]string{"Log message"})
	if err != nil {
		t.Fatalf("Elog failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Log message") {
		t.Errorf("expected 'Log message', got: %s", output)
	}
	if !strings.Contains(output, "LOG:") {
		t.Errorf("expected 'LOG:' prefix, got: %s", output)
	}
}

// ============================================================================
// Ebegin/Eend Tests
// ============================================================================

func TestHelpers_Ebegin(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Ebegin([]string{"Running task"})
	if err != nil {
		t.Fatalf("Ebegin failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Running task") {
		t.Errorf("expected 'Running task', got: %s", output)
	}
	if !strings.Contains(output, "...") {
		t.Errorf("expected '...' suffix, got: %s", output)
	}
}

func TestHelpers_Eend_Success(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Eend([]string{"0"})
	if err != nil {
		t.Fatalf("Eend failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "ok") {
		t.Errorf("expected 'ok' for success, got: %s", output)
	}
}

func TestHelpers_Eend_Failure(t *testing.T) {
	helpers, stdout, stderr := createTestHelpers(t)

	err := helpers.Eend([]string{"1", "Task failed"})
	if err != nil {
		t.Fatalf("Eend failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "!!") {
		t.Errorf("expected '!!' for failure, got: %s", output)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "Task failed") {
		t.Errorf("expected 'Task failed' in stderr, got: %s", errOutput)
	}
}

func TestHelpers_Eend_NoArgs(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Eend([]string{})
	if err != nil {
		t.Fatalf("Eend failed: %v", err)
	}

	output := stdout.String()
	// Default is success (exit code 0)
	if !strings.Contains(output, "ok") {
		t.Errorf("expected 'ok' for default success, got: %s", output)
	}
}

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

// ============================================================================
// Exit Status Tests
// ============================================================================

func TestExitFalse(t *testing.T) {
	err := exitFalse()
	if err == nil {
		t.Fatal("expected error from exitFalse")
	}

	var exitErr interp.ExitStatus
	if !errors.As(err, &exitErr) {
		t.Errorf("expected exit status error, got: %T", err)
	}
	if uint8(exitErr) != 1 {
		t.Errorf("expected exit code 1, got: %d", uint8(exitErr))
	}
}

// ============================================================================
// Nil Environment Tests
// ============================================================================

func TestHelpers_NilEnvironment(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	// USE check should fail with nil env
	err := helpers.Use([]string{"ssl"})
	if err == nil {
		t.Error("expected error with nil environment")
	}

	// Messaging should still work
	err = helpers.Einfo([]string{"test"})
	if err != nil {
		t.Errorf("Einfo should work with nil env: %v", err)
	}

	// Toolchain should return defaults
	stdout.Reset()
	err = helpers.TcGetCC([]string{})
	if err != nil {
		t.Errorf("TcGetCC should work with nil env: %v", err)
	}
	if stdout.String() != "gcc" {
		t.Errorf("expected default 'gcc', got: %s", stdout.String())
	}
}

// ============================================================================
// DieError Tests
// ============================================================================

func TestDieError_Error(t *testing.T) {
	err := &DieError{Message: "test error"}
	if err.Error() != "die: test error" {
		t.Errorf("expected 'die: test error', got: %s", err.Error())
	}

	emptyErr := &DieError{}
	if emptyErr.Error() != "die called" {
		t.Errorf("expected 'die called', got: %s", emptyErr.Error())
	}
}

// ============================================================================
// NewHelpers Tests
// ============================================================================

func TestNewHelpers_DefaultValues(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	if helpers.insDestTree != "/usr" {
		t.Errorf("expected insDestTree '/usr', got: %s", helpers.insDestTree)
	}
	if helpers.exeDestTree != "" {
		t.Errorf("expected exeDestTree '', got: %s", helpers.exeDestTree)
	}
	if helpers.docDestTree != "" {
		t.Errorf("expected docDestTree '', got: %s", helpers.docDestTree)
	}
	if helpers.destTree != "/usr" {
		t.Errorf("expected destTree '/usr', got: %s", helpers.destTree)
	}
	if helpers.libDir == "" {
		t.Error("expected libDir to be set")
	}
}

// ============================================================================
// Install Helper Tests
// ============================================================================

// createInstallTestHelpers creates a Helpers instance with a temporary D directory.
func createInstallTestHelpers(t *testing.T) (*Helpers, string) {
	t.Helper()

	tmpDir := t.TempDir()
	imageDir := tmpDir + "/image"

	testPkg := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
		UseFlags: map[string]bool{
			"ssl":  true,
			"zlib": true,
		},
	}

	env, err := NewEnvironment(testPkg, tmpDir, "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}
	env.D = imageDir

	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(env, &stdout, &stderr)

	// Create image directory
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		t.Fatalf("failed to create image dir: %v", err)
	}

	return helpers, tmpDir
}

// createTestFile creates a test file in the given directory.
func createTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create %s: %v", path, err)
	}
	return path
}

// ============================================================================
// Directory Setting Tests
// ============================================================================

func TestHelpers_Insinto(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Insinto([]string{"/usr/share/myapp"})
	if err != nil {
		t.Fatalf("Insinto failed: %v", err)
	}
	if helpers.insDestTree != "/usr/share/myapp" {
		t.Errorf("expected insDestTree '/usr/share/myapp', got: %s", helpers.insDestTree)
	}
}

func TestHelpers_Insinto_NoArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Insinto([]string{})
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestHelpers_Exeinto(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Exeinto([]string{"/usr/libexec"})
	if err != nil {
		t.Fatalf("Exeinto failed: %v", err)
	}
	if helpers.exeDestTree != "/usr/libexec" {
		t.Errorf("expected exeDestTree '/usr/libexec', got: %s", helpers.exeDestTree)
	}
}

func TestHelpers_Docinto(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Docinto([]string{"examples"})
	if err != nil {
		t.Fatalf("Docinto failed: %v", err)
	}
	if helpers.docDestTree != "examples" {
		t.Errorf("expected docDestTree 'examples', got: %s", helpers.docDestTree)
	}

	// Reset with "/"
	err = helpers.Docinto([]string{"/"})
	if err != nil {
		t.Fatalf("Docinto reset failed: %v", err)
	}
	if helpers.docDestTree != "" {
		t.Errorf("expected docDestTree '', got: %s", helpers.docDestTree)
	}
}

// ============================================================================
// Option Setting Tests
// ============================================================================

func TestHelpers_Insopts(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Insopts([]string{"-m0600"})
	if err != nil {
		t.Fatalf("Insopts failed: %v", err)
	}
	if helpers.insOpts != "-m0600" {
		t.Errorf("expected insOpts '-m0600', got: %s", helpers.insOpts)
	}
}

func TestHelpers_Exeopts(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Exeopts([]string{"-m0700"})
	if err != nil {
		t.Fatalf("Exeopts failed: %v", err)
	}
	if helpers.exeOpts != "-m0700" {
		t.Errorf("expected exeOpts '-m0700', got: %s", helpers.exeOpts)
	}
}

func TestHelpers_Diropts(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Diropts([]string{"-m0700"})
	if err != nil {
		t.Fatalf("Diropts failed: %v", err)
	}
	if helpers.dirOpts != "-m0700" {
		t.Errorf("expected dirOpts '-m0700', got: %s", helpers.dirOpts)
	}
}

// ============================================================================
// Binary Installation Tests
// ============================================================================

func TestHelpers_Dobin(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	// Create a test executable
	exePath := createTestFile(t, tmpDir, "myapp", "#!/bin/sh\necho hello")

	err := helpers.Dobin([]string{exePath})
	if err != nil {
		t.Fatalf("Dobin failed: %v", err)
	}

	// Verify file was installed
	installedPath := filepath.Join(helpers.env.D, "usr", "bin", "myapp")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Dobin_NoFiles(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Dobin([]string{})
	if err == nil {
		t.Error("expected error with no files")
	}
}

func TestHelpers_Dobin_MissingFile(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Dobin([]string{"/nonexistent/file"})
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestHelpers_Dosbin(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	exePath := createTestFile(t, tmpDir, "mydaemon", "#!/bin/sh\necho daemon")

	err := helpers.Dosbin([]string{exePath})
	if err != nil {
		t.Fatalf("Dosbin failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "sbin", "mydaemon")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Newbin(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	exePath := createTestFile(t, tmpDir, "src.sh", "#!/bin/sh\necho src")

	err := helpers.Newbin([]string{exePath, "dest"})
	if err != nil {
		t.Fatalf("Newbin failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "bin", "dest")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Newsbin(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	exePath := createTestFile(t, tmpDir, "src.sh", "#!/bin/sh\necho src")

	err := helpers.Newsbin([]string{exePath, "dest"})
	if err != nil {
		t.Fatalf("Newsbin failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "sbin", "dest")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doexe(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	// Set EXEDESTTREE first
	err := helpers.Exeinto([]string{"/usr/libexec"})
	if err != nil {
		t.Fatalf("Exeinto failed: %v", err)
	}

	exePath := createTestFile(t, tmpDir, "script.sh", "#!/bin/sh\necho script")

	err = helpers.Doexe([]string{exePath})
	if err != nil {
		t.Fatalf("Doexe failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "libexec", "script.sh")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doexe_NoExeinto(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	exePath := createTestFile(t, tmpDir, "script.sh", "#!/bin/sh\necho script")

	err := helpers.Doexe([]string{exePath})
	if err == nil {
		t.Error("expected error when EXEDESTTREE not set")
	}
}

// ============================================================================
// File Installation Tests
// ============================================================================

func TestHelpers_Doins(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	// Set install directory
	err := helpers.Insinto([]string{"/usr/share/myapp"})
	if err != nil {
		t.Fatalf("Insinto failed: %v", err)
	}

	filePath := createTestFile(t, tmpDir, "config.conf", "key=value")

	err = helpers.Doins([]string{filePath})
	if err != nil {
		t.Fatalf("Doins failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "myapp", "config.conf")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doins_Recursive(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	// Create a directory with files
	subDir := filepath.Join(tmpDir, "subdir")
	createTestFile(t, subDir, "file1.txt", "content1")
	createTestFile(t, subDir, "file2.txt", "content2")

	err := helpers.Insinto([]string{"/usr/share/myapp"})
	if err != nil {
		t.Fatalf("Insinto failed: %v", err)
	}

	err = helpers.Doins([]string{"-r", subDir})
	if err != nil {
		t.Fatalf("Doins -r failed: %v", err)
	}

	// Verify files were installed
	installedPath1 := filepath.Join(helpers.env.D, "usr", "share", "myapp", "subdir", "file1.txt")
	if _, err := os.Stat(installedPath1); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath1)
	}
}

func TestHelpers_Doins_DirectoryWithoutRecursive(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	err := helpers.Doins([]string{subDir})
	if err == nil {
		t.Error("expected error for directory without -r")
	}
}

func TestHelpers_Newins(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	err := helpers.Insinto([]string{"/etc"})
	if err != nil {
		t.Fatalf("Insinto failed: %v", err)
	}

	filePath := createTestFile(t, tmpDir, "source.conf", "key=value")

	err = helpers.Newins([]string{filePath, "dest.conf"})
	if err != nil {
		t.Fatalf("Newins failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "etc", "dest.conf")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

// ============================================================================
// Documentation Tests
// ============================================================================

func TestHelpers_Dodoc(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	filePath := createTestFile(t, tmpDir, "README", "This is readme")

	err := helpers.Dodoc([]string{filePath})
	if err != nil {
		t.Fatalf("Dodoc failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "doc", helpers.env.PF, "README")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Dodoc_Recursive(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	docDir := filepath.Join(tmpDir, "docs")
	createTestFile(t, docDir, "intro.txt", "intro")
	createTestFile(t, docDir, "guide.txt", "guide")

	err := helpers.Dodoc([]string{"-r", docDir})
	if err != nil {
		t.Fatalf("Dodoc -r failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "doc", helpers.env.PF, "docs", "intro.txt")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Dodoc_WithDocinto(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	err := helpers.Docinto([]string{"examples"})
	if err != nil {
		t.Fatalf("Docinto failed: %v", err)
	}

	filePath := createTestFile(t, tmpDir, "example.txt", "example content")

	err = helpers.Dodoc([]string{filePath})
	if err != nil {
		t.Fatalf("Dodoc failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "doc", helpers.env.PF, "examples", "example.txt")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Newdoc(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	filePath := createTestFile(t, tmpDir, "README.md", "# Title")

	err := helpers.Newdoc([]string{filePath, "README"})
	if err != nil {
		t.Fatalf("Newdoc failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "doc", helpers.env.PF, "README")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doman(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	manPath := createTestFile(t, tmpDir, "foo.1", ".TH FOO 1")

	err := helpers.Doman([]string{manPath})
	if err != nil {
		t.Fatalf("Doman failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "man", "man1", "foo.1")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doman_Section8(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	manPath := createTestFile(t, tmpDir, "bar.8", ".TH BAR 8")

	err := helpers.Doman([]string{manPath})
	if err != nil {
		t.Fatalf("Doman failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "man", "man8", "bar.8")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Newman(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	manPath := createTestFile(t, tmpDir, "foo.man", ".TH FOO 1")

	err := helpers.Newman([]string{manPath, "foo.1"})
	if err != nil {
		t.Fatalf("Newman failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "share", "man", "man1", "foo.1")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

// ============================================================================
// Library/Header Tests
// ============================================================================

func TestHelpers_Dolib(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	libPath := createTestFile(t, tmpDir, "libfoo.so", "ELF binary")

	err := helpers.Dolib([]string{libPath})
	if err != nil {
		t.Fatalf("Dolib failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", helpers.libDir, "libfoo.so")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_DolibSo(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	libPath := createTestFile(t, tmpDir, "libfoo.so.1.0", "ELF binary")

	err := helpers.DolibSo([]string{libPath})
	if err != nil {
		t.Fatalf("DolibSo failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", helpers.libDir, "libfoo.so.1.0")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_DolibA(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	libPath := createTestFile(t, tmpDir, "libfoo.a", "static lib")

	err := helpers.DolibA([]string{libPath})
	if err != nil {
		t.Fatalf("DolibA failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", helpers.libDir, "libfoo.a")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doheader(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	headerPath := createTestFile(t, tmpDir, "foo.h", "#ifndef FOO_H")

	err := helpers.Doheader([]string{headerPath})
	if err != nil {
		t.Fatalf("Doheader failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "include", "foo.h")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doheader_Recursive(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	incDir := filepath.Join(tmpDir, "include")
	createTestFile(t, incDir, "foo.h", "#ifndef FOO_H")
	createTestFile(t, incDir, "bar.h", "#ifndef BAR_H")

	err := helpers.Doheader([]string{"-r", incDir})
	if err != nil {
		t.Fatalf("Doheader -r failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "usr", "include", "include", "foo.h")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

// ============================================================================
// Directory Creation Tests
// ============================================================================

func TestHelpers_Dodir(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Dodir([]string{"/usr/share/myapp", "/etc/myapp"})
	if err != nil {
		t.Fatalf("Dodir failed: %v", err)
	}

	dir1 := filepath.Join(helpers.env.D, "usr", "share", "myapp")
	if info, err := os.Stat(dir1); os.IsNotExist(err) || !info.IsDir() {
		t.Errorf("expected directory at %s", dir1)
	}

	dir2 := filepath.Join(helpers.env.D, "etc", "myapp")
	if info, err := os.Stat(dir2); os.IsNotExist(err) || !info.IsDir() {
		t.Errorf("expected directory at %s", dir2)
	}
}

func TestHelpers_Dodir_NoArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Dodir([]string{})
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestHelpers_Keepdir(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Keepdir([]string{"/var/lib/myapp"})
	if err != nil {
		t.Fatalf("Keepdir failed: %v", err)
	}

	dir := filepath.Join(helpers.env.D, "var", "lib", "myapp")
	if info, err := os.Stat(dir); os.IsNotExist(err) || !info.IsDir() {
		t.Errorf("expected directory at %s", dir)
	}

	// Check for .keep file
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}

	found := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".keep") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected .keep file in %s", dir)
	}
}

// ============================================================================
// parseMode Tests
// ============================================================================

func TestParseMode(t *testing.T) {
	tests := []struct {
		input    string
		expected os.FileMode
		wantErr  bool
	}{
		{"-m0644", 0644, false},
		{"-m0755", 0755, false},
		{"-m0600", 0600, false},
		{"-m0700", 0700, false},
		{"", 0644, false}, // Default
		{"-minvalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			mode, err := parseMode(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mode != tt.expected {
				t.Errorf("expected %o, got %o", tt.expected, mode)
			}
		})
	}
}

// ============================================================================
// Symlink Tests
// ============================================================================

func TestHelpers_InstallFile_Symlink(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	// Create a regular file and a symlink to it
	targetPath := createTestFile(t, tmpDir, "target.txt", "target content")
	symlinkPath := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink("target.txt", symlinkPath); err != nil {
		// Symlinks require admin privileges on Windows, skip if not available
		t.Skipf("skipping symlink test: %v", err)
	}

	err := helpers.Insinto([]string{"/usr/share/test"})
	if err != nil {
		t.Fatalf("Insinto failed: %v", err)
	}

	// Install the target file first
	err = helpers.Doins([]string{targetPath})
	if err != nil {
		t.Fatalf("Doins target failed: %v", err)
	}

	// Install the symlink
	err = helpers.Doins([]string{symlinkPath})
	if err != nil {
		t.Fatalf("Doins symlink failed: %v", err)
	}

	// Verify symlink was preserved
	installedLink := filepath.Join(helpers.env.D, "usr", "share", "test", "link.txt")
	info, err := os.Lstat(installedLink)
	if err != nil {
		t.Fatalf("failed to stat installed symlink: %v", err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink to be preserved")
	}
}

// ============================================================================
// Error Handling Tests
// ============================================================================

func TestHelpers_Dobin_NilEnvironment(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	err := helpers.Dobin([]string{"somefile"})
	if err == nil {
		t.Error("expected error with nil environment")
	}

	var dieErr *DieError
	if !errors.As(err, &dieErr) {
		t.Errorf("expected DieError, got: %T", err)
	}
}

func TestHelpers_Dodir_NilEnvironment(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	err := helpers.Dodir([]string{"/some/dir"})
	if err == nil {
		t.Error("expected error with nil environment")
	}
}

func TestHelpers_Newbin_TooFewArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Newbin([]string{"onlyonearg"})
	if err == nil {
		t.Error("expected error with only one arg")
	}
}

func TestHelpers_Newins_TooFewArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Newins([]string{"onlyonearg"})
	if err == nil {
		t.Error("expected error with only one arg")
	}
}

func TestHelpers_Newdoc_TooFewArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Newdoc([]string{"onlyonearg"})
	if err == nil {
		t.Error("expected error with only one arg")
	}
}

func TestHelpers_Newman_TooFewArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Newman([]string{"onlyonearg"})
	if err == nil {
		t.Error("expected error with only one arg")
	}
}

func TestHelpers_Doman_NoExtension(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	manPath := createTestFile(t, tmpDir, "foo", ".TH FOO 1")

	err := helpers.Doman([]string{manPath})
	if err == nil {
		t.Error("expected error for file without section extension")
	}
}

func TestHelpers_Dobin_Directory(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	err := helpers.Dobin([]string{subDir})
	if err == nil {
		t.Error("expected error for directory")
	}
}

// ============================================================================
// Stage 3: Build Helper Tests
// ============================================================================

// createBuildTestHelpers creates a Helpers instance for build command tests.
func createBuildTestHelpers(t *testing.T) (*Helpers, string, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	sourceDir := filepath.Join(workDir, "zlib-1.2.13")

	testPkg := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
		UseFlags: map[string]bool{
			"ssl":  true,
			"zlib": true,
		},
	}

	env, err := NewEnvironment(testPkg, tmpDir, "/var/db/repos/gentoo", tmpDir)
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}
	env.WORKDIR = workDir
	env.S = sourceDir
	env.DISTDIR = tmpDir
	env.MAKEOPTS = "-j4"
	env.A = "zlib-1.2.13.tar.gz"

	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(env, &stdout, &stderr)

	// Create directories
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	return helpers, tmpDir, &stdout, &stderr
}

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

	err := helpers.DefaultSrcInstall([]string{})
	if err == nil {
		t.Error("expected error with no Makefile")
	}
}

func TestHelpers_Default_UnknownPhase(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	// Unknown phase should succeed (do nothing)
	err := helpers.Default([]string{})
	if err != nil {
		t.Errorf("Default should succeed for unknown phase: %v", err)
	}
}

// ============================================================================
// Version Manipulation Tests
// ============================================================================

func TestHelpers_VerCut(t *testing.T) {
	tests := []struct {
		args     []string
		expected string
		wantErr  bool
	}{
		{[]string{"1", "1.2.3"}, "1", false},
		{[]string{"2", "1.2.3"}, "2", false},
		{[]string{"3", "1.2.3"}, "3", false},
		{[]string{"1-2", "1.2.3"}, "1.2", false},
		{[]string{"2-3", "1.2.3"}, "2.3", false},
		{[]string{"1-3", "1.2.3"}, "1.2.3", false},
		{[]string{"2-", "1.2.3"}, "2.3", false},
		{[]string{"-2", "1.2.3"}, "1.2", false},
		{[]string{"5", "1.2.3"}, "", false}, // Out of range
		{[]string{"1"}, "", true},           // Too few args
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, "_"), func(t *testing.T) {
			helpers, _, stdout, _ := createBuildTestHelpers(t)
			stdout.Reset()

			err := helpers.VerCut(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("VerCut failed: %v", err)
			}

			output := stdout.String()
			if output != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, output)
			}
		})
	}
}

func TestHelpers_VerRs(t *testing.T) {
	tests := []struct {
		args     []string
		expected string
		wantErr  bool
	}{
		{[]string{"1", "-", "1.2.3"}, "1-2.3", false},
		{[]string{"1-2", "-", "1.2.3"}, "1-2-3", false},
		{[]string{"1"}, "", true}, // Too few args
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, "_"), func(t *testing.T) {
			helpers, _, stdout, _ := createBuildTestHelpers(t)
			stdout.Reset()

			err := helpers.VerRs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("VerRs failed: %v", err)
			}

			output := stdout.String()
			if output != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, output)
			}
		})
	}
}

func TestHelpers_SplitVersion(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	tests := []struct {
		version  string
		expected []string
	}{
		{"1.2.3", []string{"1", "2", "3"}},
		{"1_2_3", []string{"1", "2", "3"}},
		{"1-2-3", []string{"1", "2", "3"}},
		{"1.2.3_rc1", []string{"1", "2", "3", "rc1"}},
		{"1", []string{"1"}},
		{"", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			result := helpers.splitVersion(tt.version)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected, result)
					return
				}
			}
		})
	}
}

// ============================================================================
// Dosym Tests
// ============================================================================

func TestHelpers_Dosym(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Dosym([]string{"/usr/lib/libfoo.so.1", "/usr/lib/libfoo.so"})
	if err != nil {
		// Symlinks require admin on Windows
		if strings.Contains(err.Error(), "not permitted") || strings.Contains(err.Error(), "privilege") {
			t.Skipf("skipping symlink test: %v", err)
		}
		t.Fatalf("Dosym failed: %v", err)
	}

	linkPath := filepath.Join(helpers.env.D, "usr", "lib", "libfoo.so")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("failed to stat link: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink")
	}
}

func TestHelpers_Dosym_NoArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Dosym([]string{"only_one_arg"})
	if err == nil {
		t.Error("expected error with only one arg")
	}
}

// ============================================================================
// Fperms Tests
// ============================================================================

func TestHelpers_Fperms(t *testing.T) {
	// Skip on Windows - chmod doesn't work the same way
	if runtime.GOOS == "windows" {
		t.Skip("skipping fperms test on Windows (permissions not supported)")
	}

	helpers, tmpDir := createInstallTestHelpers(t)

	// Create file in image
	filePath := filepath.Join(helpers.env.D, "usr", "bin")
	if err := os.MkdirAll(filePath, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	testFile := filepath.Join(filePath, "myapp")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	err := helpers.Fperms([]string{"0755", "/usr/bin/myapp"})
	if err != nil {
		t.Fatalf("Fperms failed: %v", err)
	}

	// Verify permissions
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected 0755, got %o", info.Mode().Perm())
	}

	_ = tmpDir
}

func TestHelpers_Fperms_NoArgs(t *testing.T) {
	helpers, _ := createInstallTestHelpers(t)

	err := helpers.Fperms([]string{"0755"})
	if err == nil {
		t.Error("expected error with no file args")
	}
}

// ============================================================================
// Utility Function Tests (sed, cat, mkdir, etc.)
// ============================================================================

func TestHelpers_Cat(t *testing.T) {
	helpers, tmpDir, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	filePath := createTestFile(t, tmpDir, "test.txt", "Hello World")

	err := helpers.Cat([]string{filePath})
	if err != nil {
		t.Fatalf("Cat failed: %v", err)
	}

	output := stdout.String()
	if output != "Hello World" {
		t.Errorf("expected 'Hello World', got: %s", output)
	}
}

func TestHelpers_Cat_NoArgs(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	err := helpers.Cat([]string{})
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestHelpers_Mkdir(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	newDir := filepath.Join(tmpDir, "newdir")
	err := helpers.Mkdir([]string{newDir})
	if err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	info, err := os.Stat(newDir)
	if err != nil || !info.IsDir() {
		t.Error("expected directory to be created")
	}
}

func TestHelpers_Mkdir_WithParents(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	deepDir := filepath.Join(tmpDir, "a", "b", "c")
	err := helpers.Mkdir([]string{"-p", deepDir})
	if err != nil {
		t.Fatalf("Mkdir -p failed: %v", err)
	}

	info, err := os.Stat(deepDir)
	if err != nil || !info.IsDir() {
		t.Error("expected deep directory to be created")
	}
}

func TestHelpers_Rm(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	filePath := createTestFile(t, tmpDir, "todelete.txt", "delete me")

	err := helpers.Rm([]string{filePath})
	if err != nil {
		t.Fatalf("Rm failed: %v", err)
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestHelpers_Rm_Recursive(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	subDir := filepath.Join(tmpDir, "deleteme")
	createTestFile(t, subDir, "file.txt", "content")

	err := helpers.Rm([]string{"-r", subDir})
	if err != nil {
		t.Fatalf("Rm -r failed: %v", err)
	}

	if _, err := os.Stat(subDir); !os.IsNotExist(err) {
		t.Error("expected directory to be deleted")
	}
}

func TestHelpers_Cp(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	src := createTestFile(t, tmpDir, "source.txt", "content")
	dst := filepath.Join(tmpDir, "dest.txt")

	err := helpers.Cp([]string{src, dst})
	if err != nil {
		t.Fatalf("Cp failed: %v", err)
	}

	content, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dest file: %v", err)
	}
	if string(content) != "content" {
		t.Errorf("expected 'content', got: %s", string(content))
	}
}

func TestHelpers_Mv(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	src := createTestFile(t, tmpDir, "tomove.txt", "content")
	dst := filepath.Join(tmpDir, "moved.txt")

	err := helpers.Mv([]string{src, dst})
	if err != nil {
		t.Fatalf("Mv failed: %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("expected source to be removed")
	}

	content, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dest file: %v", err)
	}
	if string(content) != "content" {
		t.Errorf("expected 'content', got: %s", string(content))
	}
}

func TestHelpers_Chmod(t *testing.T) {
	// Skip on Windows - chmod doesn't work the same way
	if runtime.GOOS == "windows" {
		t.Skip("skipping chmod test on Windows (permissions not supported)")
	}

	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	filePath := createTestFile(t, tmpDir, "chmodtest.txt", "content")

	err := helpers.Chmod([]string{"0755", filePath})
	if err != nil {
		t.Fatalf("Chmod failed: %v", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected 0755, got %o", info.Mode().Perm())
	}
}

func TestHelpers_Ln(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	target := createTestFile(t, tmpDir, "target.txt", "content")
	link := filepath.Join(tmpDir, "link.txt")

	err := helpers.Ln([]string{"-s", target, link})
	if err != nil {
		// Symlinks require admin on Windows
		if strings.Contains(err.Error(), "not permitted") || strings.Contains(err.Error(), "privilege") {
			t.Skipf("skipping symlink test: %v", err)
		}
		t.Fatalf("Ln failed: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("failed to stat link: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink")
	}
}

func TestHelpers_Touch(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	newFile := filepath.Join(tmpDir, "touched.txt")

	err := helpers.Touch([]string{newFile})
	if err != nil {
		t.Fatalf("Touch failed: %v", err)
	}

	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Error("expected file to be created")
	}
}

func TestHelpers_Find(t *testing.T) {
	helpers, tmpDir, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	createTestFile(t, tmpDir, "find1.txt", "content")
	createTestFile(t, tmpDir, "find2.txt", "content")
	createTestFile(t, filepath.Join(tmpDir, "subdir"), "find3.txt", "content")

	err := helpers.Find([]string{tmpDir, "-name", "*.txt"})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "find1.txt") {
		t.Errorf("expected find1.txt in output, got: %s", output)
	}
	if !strings.Contains(output, "find3.txt") {
		t.Errorf("expected find3.txt in output, got: %s", output)
	}
}

func TestHelpers_Grep_Found(t *testing.T) {
	helpers, tmpDir, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	filePath := createTestFile(t, tmpDir, "greptest.txt", "hello world\nfoo bar\nhello again")

	err := helpers.Grep([]string{"hello", filePath})
	if err != nil {
		t.Fatalf("Grep failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "hello world") {
		t.Errorf("expected 'hello world' in output, got: %s", output)
	}
	if !strings.Contains(output, "hello again") {
		t.Errorf("expected 'hello again' in output, got: %s", output)
	}
}

func TestHelpers_Grep_NotFound(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	filePath := createTestFile(t, tmpDir, "greptest.txt", "hello world")

	err := helpers.Grep([]string{"notfound", filePath})
	if err == nil {
		t.Error("expected exit status error when pattern not found")
	}
}

func TestHelpers_Sed_InPlace(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	filePath := createTestFile(t, tmpDir, "sedtest.txt", "hello world")

	err := helpers.Sed([]string{"-i", "s/hello/goodbye/", filePath})
	if err != nil {
		t.Fatalf("Sed failed: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "goodbye world" {
		t.Errorf("expected 'goodbye world', got: %s", string(content))
	}
}

func TestHelpers_Sed_GlobalReplace(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	filePath := createTestFile(t, tmpDir, "sedtest.txt", "hello hello hello")

	err := helpers.Sed([]string{"-i", "s/hello/bye/g", filePath})
	if err != nil {
		t.Fatalf("Sed failed: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "bye bye bye" {
		t.Errorf("expected 'bye bye bye', got: %s", string(content))
	}
}

func TestHelpers_Install_File(t *testing.T) {
	// Skip permission check on Windows - chmod doesn't work the same way
	if runtime.GOOS == "windows" {
		t.Skip("skipping install permission test on Windows (permissions not supported)")
	}

	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	src := createTestFile(t, tmpDir, "installsrc.txt", "content")
	dst := filepath.Join(tmpDir, "installdst.txt")

	err := helpers.Install([]string{"-m", "0755", src, dst})
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("failed to stat installed file: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected 0755, got %o", info.Mode().Perm())
	}
}

func TestHelpers_Install_Directory(t *testing.T) {
	helpers, tmpDir, _, _ := createBuildTestHelpers(t)

	newDir := filepath.Join(tmpDir, "installdir")

	err := helpers.Install([]string{"-d", newDir})
	if err != nil {
		t.Fatalf("Install -d failed: %v", err)
	}

	info, err := os.Stat(newDir)
	if err != nil || !info.IsDir() {
		t.Error("expected directory to be created")
	}
}

func TestHelpers_Which(t *testing.T) {
	helpers, _, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	// 'go' should be in PATH on development machines
	err := helpers.Which([]string{"go"})
	if err != nil {
		t.Logf("which go failed (may not be in PATH): %v", err)
		return
	}

	output := stdout.String()
	if output == "" {
		t.Log("which returned empty (go not in PATH)")
	}
}

// ============================================================================
// Additional Installation Helper Tests
// ============================================================================

func TestHelpers_Doconfd(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	filePath := createTestFile(t, tmpDir, "myapp.conf", "# config")

	err := helpers.Doconfd([]string{filePath})
	if err != nil {
		t.Fatalf("Doconfd failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "etc", "conf.d", "myapp.conf")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doinitd(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	filePath := createTestFile(t, tmpDir, "myapp", "#!/sbin/openrc-run")

	err := helpers.Doinitd([]string{filePath})
	if err != nil {
		t.Fatalf("Doinitd failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "etc", "init.d", "myapp")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Doenvd(t *testing.T) {
	helpers, tmpDir := createInstallTestHelpers(t)

	filePath := createTestFile(t, tmpDir, "99myapp", "PATH=/opt/myapp/bin")

	err := helpers.Doenvd([]string{filePath})
	if err != nil {
		t.Fatalf("Doenvd failed: %v", err)
	}

	installedPath := filepath.Join(helpers.env.D, "etc", "env.d", "99myapp")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", installedPath)
	}
}

func TestHelpers_Inherit(t *testing.T) {
	helpers, _, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	err := helpers.Inherit([]string{"cmake", "flag-o-matic"})
	if err != nil {
		t.Fatalf("Inherit failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "cmake") {
		t.Errorf("expected 'cmake' in output, got: %s", output)
	}
	if !strings.Contains(output, "flag-o-matic") {
		t.Errorf("expected 'flag-o-matic' in output, got: %s", output)
	}
}
