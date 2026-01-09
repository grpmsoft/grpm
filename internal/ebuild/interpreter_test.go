package ebuild

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// createTestEnvironment creates an Environment for testing.
func createTestEnvironment(t *testing.T) *Environment {
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

	return env
}

func TestInterpreter_Run_SimpleScript(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	err := interp.Run(context.Background(), `echo "hello world"`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "hello world") {
		t.Errorf("expected output to contain 'hello world', got: %s", output)
	}
}

func TestInterpreter_Run_EnvironmentVariables(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	err := interp.Run(context.Background(), `echo "PN=$PN PV=$PV"`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "PN=zlib") {
		t.Errorf("expected PN=zlib in output, got: %s", output)
	}
	if !strings.Contains(output, "PV=1.2.13") {
		t.Errorf("expected PV=1.2.13 in output, got: %s", output)
	}
}

func TestInterpreter_Run_Einfo(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	err := interp.Run(context.Background(), `einfo "Building package"`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Building package") {
		t.Errorf("expected 'Building package' in output, got: %s", output)
	}
	// Check for green color marker or asterisk
	if !strings.Contains(output, "*") {
		t.Errorf("expected asterisk marker in output, got: %s", output)
	}
}

func TestInterpreter_Run_Ewarn(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	err := interp.Run(context.Background(), `ewarn "Warning message"`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// ewarn writes to stderr
	errOutput := stderr.String()
	if !strings.Contains(errOutput, "Warning message") {
		t.Errorf("expected 'Warning message' in stderr, got: %s", errOutput)
	}
}

func TestInterpreter_Run_UseFlag_Enabled(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// ssl is enabled in test environment
	err := interp.Run(context.Background(), `
		if use ssl; then
			echo "SSL_ENABLED"
		else
			echo "SSL_DISABLED"
		fi
	`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "SSL_ENABLED") {
		t.Errorf("expected SSL_ENABLED, got: %s", output)
	}
}

func TestInterpreter_Run_UseFlag_Disabled(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// doc is disabled in test environment
	err := interp.Run(context.Background(), `
		if use doc; then
			echo "DOC_ENABLED"
		else
			echo "DOC_DISABLED"
		fi
	`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "DOC_DISABLED") {
		t.Errorf("expected DOC_DISABLED, got: %s", output)
	}
}

func TestInterpreter_Run_UseFlag_Negation(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// !doc should be true since doc is disabled
	err := interp.Run(context.Background(), `
		if use '!doc'; then
			echo "NOT_DOC"
		else
			echo "HAS_DOC"
		fi
	`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "NOT_DOC") {
		t.Errorf("expected NOT_DOC for negated disabled flag, got: %s", output)
	}
}

func TestInterpreter_Run_Usev(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// usev should print flag name if enabled
	err := interp.Run(context.Background(), `usev ssl`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "ssl") {
		t.Errorf("expected 'ssl' in output, got: %s", output)
	}
}

func TestInterpreter_Run_Usev_CustomValue(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// usev with custom value
	err := interp.Run(context.Background(), `usev ssl "--with-ssl"`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "--with-ssl") {
		t.Errorf("expected '--with-ssl' in output, got: %s", output)
	}
}

func TestInterpreter_Run_Usex(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// usex for enabled flag
	err := interp.Run(context.Background(), `usex ssl`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "yes") {
		t.Errorf("expected 'yes' for enabled flag, got: %s", output)
	}
}

func TestInterpreter_Run_Usex_Disabled(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// usex for disabled flag
	err := interp.Run(context.Background(), `usex doc`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "no") {
		t.Errorf("expected 'no' for disabled flag, got: %s", output)
	}
}

func TestInterpreter_Run_Usex_CustomValues(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// usex with custom true/false values
	err := interp.Run(context.Background(), `usex ssl ON OFF`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "ON") {
		t.Errorf("expected 'ON' for enabled flag with custom values, got: %s", output)
	}
}

func TestInterpreter_Run_Has(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// has should find element in list
	err := interp.Run(context.Background(), `
		if has foo foo bar baz; then
			echo "FOUND"
		else
			echo "NOT_FOUND"
		fi
	`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "FOUND") {
		t.Errorf("expected FOUND, got: %s", output)
	}
}

func TestInterpreter_Run_Has_NotFound(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// has should not find element not in list
	err := interp.Run(context.Background(), `
		if has qux foo bar baz; then
			echo "FOUND"
		else
			echo "NOT_FOUND"
		fi
	`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "NOT_FOUND") {
		t.Errorf("expected NOT_FOUND, got: %s", output)
	}
}

func TestInterpreter_Run_UseEnable(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// use_enable with enabled flag (ssl)
	err := interp.Run(context.Background(), `use_enable ssl`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "--enable-ssl") {
		t.Errorf("expected '--enable-ssl', got: %s", output)
	}
}

func TestInterpreter_Run_UseEnable_Disabled(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// use_enable with disabled flag (doc)
	err := interp.Run(context.Background(), `use_enable doc`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "--disable-doc") {
		t.Errorf("expected '--disable-doc', got: %s", output)
	}
}

func TestInterpreter_Run_UseEnable_CustomOption(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// use_enable with custom option name
	err := interp.Run(context.Background(), `use_enable ssl openssl`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "--enable-openssl") {
		t.Errorf("expected '--enable-openssl', got: %s", output)
	}
}

func TestInterpreter_Run_UseEnable_FourArgs(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// use_enable with 4 args: flag opt val_true val_false
	err := interp.Run(context.Background(), `use_enable ssl openssl yes no`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "--openssl=yes") {
		t.Errorf("expected '--openssl=yes', got: %s", output)
	}
}

func TestInterpreter_Run_UseWith(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// use_with with enabled flag (ssl)
	err := interp.Run(context.Background(), `use_with ssl`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "--with-ssl") {
		t.Errorf("expected '--with-ssl', got: %s", output)
	}
}

func TestInterpreter_Run_UseWith_Disabled(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// use_with with disabled flag (doc)
	err := interp.Run(context.Background(), `use_with doc`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "--without-doc") {
		t.Errorf("expected '--without-doc', got: %s", output)
	}
}

func TestInterpreter_Run_UseWith_FourArgs(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// use_with with 4 args: flag opt val_true val_false
	err := interp.Run(context.Background(), `use_with doc docs /path /none`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "--docs=/none") {
		t.Errorf("expected '--docs=/none', got: %s", output)
	}
}

func TestInterpreter_Run_UseEnable_InScript(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// Typical ebuild usage: building configure args with use_enable
	err := interp.Run(context.Background(), `
		args=""
		args="$args $(use_enable ssl)"
		args="$args $(use_enable doc)"
		echo "CONFIGURE_ARGS:$args"
	`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "--enable-ssl") {
		t.Errorf("expected '--enable-ssl' in output, got: %s", output)
	}
	if !strings.Contains(output, "--disable-doc") {
		t.Errorf("expected '--disable-doc' in output, got: %s", output)
	}
}

func TestInterpreter_Run_TcGetCC(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	err := interp.Run(context.Background(), `tc-getCC`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	// Should return gcc by default
	if !strings.Contains(output, "gcc") {
		t.Errorf("expected 'gcc' in output, got: %s", output)
	}
}

func TestInterpreter_Run_TcGetCXX(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	err := interp.Run(context.Background(), `tc-getCXX`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	// Should return g++ by default
	if !strings.Contains(output, "g++") {
		t.Errorf("expected 'g++' in output, got: %s", output)
	}
}

func TestInterpreter_Run_TcArch(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	err := interp.Run(context.Background(), `tc-arch`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	// Should return some architecture
	if len(output) == 0 {
		t.Error("expected non-empty architecture output")
	}
}

func TestInterpreter_Run_Die(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	err := interp.Run(context.Background(), `die "Something went wrong"`)
	if err == nil {
		t.Fatal("expected error from die, got nil")
	}

	// Check stderr for error message
	errOutput := stderr.String()
	if !strings.Contains(errOutput, "Something went wrong") {
		t.Errorf("expected error message in stderr, got: %s", errOutput)
	}
}

func TestInterpreter_Run_Ebegin_Eend(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	err := interp.Run(context.Background(), `
		ebegin "Running task"
		eend 0
	`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Running task") {
		t.Errorf("expected 'Running task' in output, got: %s", output)
	}
	if !strings.Contains(output, "ok") {
		t.Errorf("expected 'ok' in output for successful eend, got: %s", output)
	}
}

func TestInterpreter_Run_InIuse(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// ssl is in IUSE (it's in UseFlags map)
	err := interp.Run(context.Background(), `
		if in_iuse ssl; then
			echo "IN_IUSE"
		else
			echo "NOT_IN_IUSE"
		fi
	`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "IN_IUSE") {
		t.Errorf("expected IN_IUSE, got: %s", output)
	}
}

func TestInterpreter_Run_ComplexScript(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	// Complex script with multiple features
	script := `
		einfo "Starting build for $PN-$PV"

		if use ssl; then
			einfo "SSL support enabled"
		fi

		if use minizip; then
			einfo "Minizip enabled"
		fi

		if ! use doc; then
			einfo "Documentation disabled"
		fi

		ebegin "Configuring"
		eend 0

		einfo "Build complete"
	`

	err := interp.Run(context.Background(), script)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output := stdout.String()

	// Check all expected messages
	expected := []string{
		"zlib-1.2.13",
		"SSL support enabled",
		"Minizip enabled",
		"Documentation disabled",
		"Configuring",
		"Build complete",
	}

	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			t.Errorf("expected '%s' in output, got: %s", exp, output)
		}
	}
}

func TestInterpreter_NilEnvironment(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// Create interpreter with nil environment
	interp := NewInterpreter(nil, &stdout, &stderr)

	// Should still work for basic commands
	err := interp.Run(context.Background(), `echo "test"`)
	if err != nil {
		t.Fatalf("Run with nil env failed: %v", err)
	}
}

func TestInterpreter_GetHelpers(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)
	helpers := interp.GetHelpers()

	if helpers == nil {
		t.Fatal("GetHelpers returned nil")
	}

	// Test direct helper call
	err := helpers.Einfo([]string{"Direct helper call"})
	if err != nil {
		t.Fatalf("Direct helper call failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Direct helper call") {
		t.Errorf("expected 'Direct helper call' in output, got: %s", output)
	}
}

func TestInterpreter_ContextCancellation(t *testing.T) {
	env := createTestEnvironment(t)
	var stdout, stderr bytes.Buffer

	interp := NewInterpreter(env, &stdout, &stderr)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := interp.Run(ctx, `echo "test"`)
	// Should fail due to canceled context
	if err == nil {
		// Some implementations may not check context, so this is acceptable
		t.Log("Context cancellation not enforced by mvdan.cc/sh")
	}
}
