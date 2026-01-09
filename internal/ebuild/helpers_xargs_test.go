package ebuild

import (
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Xargs Tests
// ============================================================================

func TestHelpers_Xargs_NoStdin(t *testing.T) {
	helpers, _, _, stderr := createBuildTestHelpers(t)

	// Without stdin context, xargs should warn
	err := helpers.Xargs([]string{"echo"})
	if err != nil {
		t.Fatalf("Xargs should not error: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "warning") {
		t.Errorf("expected warning about no stdin, got: %s", output)
	}
}

func TestHelpers_XargsWithStdin_Basic(t *testing.T) {
	helpers, _, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	// Simulate stdin with newline-separated input
	stdin := strings.NewReader("file1.txt\nfile2.txt\nfile3.txt\n")

	err := helpers.XargsWithStdin(stdin, []string{"echo"})
	if err != nil {
		t.Fatalf("XargsWithStdin failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "file1.txt") {
		t.Errorf("expected 'file1.txt' in output, got: %s", output)
	}
	if !strings.Contains(output, "file2.txt") {
		t.Errorf("expected 'file2.txt' in output, got: %s", output)
	}
	if !strings.Contains(output, "file3.txt") {
		t.Errorf("expected 'file3.txt' in output, got: %s", output)
	}
}

func TestHelpers_XargsWithStdin_NoCommand(t *testing.T) {
	helpers, _, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	// No command - default to echo
	stdin := strings.NewReader("hello world\n")

	err := helpers.XargsWithStdin(stdin, []string{})
	if err != nil {
		t.Fatalf("XargsWithStdin failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "hello") {
		t.Errorf("expected 'hello' in output, got: %s", output)
	}
}

func TestHelpers_XargsWithStdin_NoRunIfEmpty(t *testing.T) {
	helpers, _, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	// Empty stdin with -r flag - should not run command
	stdin := strings.NewReader("")

	err := helpers.XargsWithStdin(stdin, []string{"-r", "echo", "test"})
	if err != nil {
		t.Fatalf("XargsWithStdin failed: %v", err)
	}

	output := stdout.String()
	if output != "" {
		t.Errorf("expected no output with -r and empty stdin, got: %s", output)
	}
}

func TestHelpers_XargsWithStdin_NullDelimiter(t *testing.T) {
	helpers, _, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	// Null-delimited input (like find -print0)
	stdin := strings.NewReader("file1.txt\x00file2.txt\x00file3.txt\x00")

	err := helpers.XargsWithStdin(stdin, []string{"-0", "echo"})
	if err != nil {
		t.Fatalf("XargsWithStdin failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "file1.txt") {
		t.Errorf("expected 'file1.txt' in output, got: %s", output)
	}
	if !strings.Contains(output, "file2.txt") {
		t.Errorf("expected 'file2.txt' in output, got: %s", output)
	}
}

func TestHelpers_XargsWithStdin_MaxArgs(t *testing.T) {
	helpers, _, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	// Test -n flag (max args per command)
	stdin := strings.NewReader("a\nb\nc\nd\ne\n")

	err := helpers.XargsWithStdin(stdin, []string{"-n", "2", "echo"})
	if err != nil {
		t.Fatalf("XargsWithStdin failed: %v", err)
	}

	output := stdout.String()
	// Should have run echo multiple times with batches of 2
	if !strings.Contains(output, "a") {
		t.Errorf("expected 'a' in output, got: %s", output)
	}
	if !strings.Contains(output, "e") {
		t.Errorf("expected 'e' in output, got: %s", output)
	}
}

func TestHelpers_XargsWithStdin_CustomDelimiter(t *testing.T) {
	helpers, _, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	// Custom delimiter
	stdin := strings.NewReader("file1:file2:file3")

	err := helpers.XargsWithStdin(stdin, []string{"-d", ":", "echo"})
	if err != nil {
		t.Fatalf("XargsWithStdin failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "file1") {
		t.Errorf("expected 'file1' in output, got: %s", output)
	}
	if !strings.Contains(output, "file2") {
		t.Errorf("expected 'file2' in output, got: %s", output)
	}
}

func TestHelpers_XargsWithStdin_InitialArgs(t *testing.T) {
	helpers, tmpDir, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	// Create test files
	createTestFile(t, tmpDir, "file1.txt", "content1")
	createTestFile(t, tmpDir, "file2.txt", "content2")

	// xargs with initial arguments
	stdin := strings.NewReader(filepath.Join(tmpDir, "file1.txt") + "\n")

	err := helpers.XargsWithStdin(stdin, []string{"ls", "-la"})
	if err != nil {
		t.Fatalf("XargsWithStdin failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "file1.txt") {
		t.Errorf("expected file info in output, got: %s", output)
	}
}

func TestHelpers_XargsWithStdin_Verbose(t *testing.T) {
	helpers, _, _, stderr := createBuildTestHelpers(t)

	// Test -t flag (verbose)
	stdin := strings.NewReader("hello\n")

	err := helpers.XargsWithStdin(stdin, []string{"-t", "echo"})
	if err != nil {
		t.Fatalf("XargsWithStdin failed: %v", err)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "echo") {
		t.Errorf("expected command in stderr with -t, got: %s", errOutput)
	}
}

func TestHelpers_XargsWithStdin_NilStdin(t *testing.T) {
	helpers, _, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	// Nil stdin - should execute command with no input args
	err := helpers.XargsWithStdin(nil, []string{"echo", "static"})
	if err != nil {
		t.Fatalf("XargsWithStdin failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "static") {
		t.Errorf("expected 'static' in output, got: %s", output)
	}
}

func TestHelpers_XargsWithStdin_WhitespaceInput(t *testing.T) {
	helpers, _, stdout, _ := createBuildTestHelpers(t)
	stdout.Reset()

	// Whitespace-separated input
	stdin := strings.NewReader("arg1 arg2 arg3\narg4 arg5\n")

	err := helpers.XargsWithStdin(stdin, []string{"echo"})
	if err != nil {
		t.Fatalf("XargsWithStdin failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "arg1") {
		t.Errorf("expected 'arg1' in output, got: %s", output)
	}
	if !strings.Contains(output, "arg5") {
		t.Errorf("expected 'arg5' in output, got: %s", output)
	}
}

func TestHelpers_ParseXargsOptions(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	tests := []struct {
		name          string
		args          []string
		wantNull      bool
		wantMaxArgs   int
		wantNoRun     bool
		wantDelimiter string
		wantVerbose   bool
		wantCmdIdx    int
	}{
		{
			name:       "no options",
			args:       []string{"echo", "hello"},
			wantCmdIdx: 0,
		},
		{
			name:       "null delimiter",
			args:       []string{"-0", "echo"},
			wantNull:   true,
			wantCmdIdx: 1,
		},
		{
			name:        "max args",
			args:        []string{"-n", "5", "echo"},
			wantMaxArgs: 5,
			wantCmdIdx:  2,
		},
		{
			name:        "max args equals format",
			args:        []string{"--max-args=10", "echo"},
			wantMaxArgs: 10,
			wantCmdIdx:  1,
		},
		{
			name:       "no run if empty",
			args:       []string{"-r", "echo"},
			wantNoRun:  true,
			wantCmdIdx: 1,
		},
		{
			name:          "delimiter",
			args:          []string{"-d", ":", "echo"},
			wantDelimiter: ":",
			wantCmdIdx:    2,
		},
		{
			name:          "delimiter short format",
			args:          []string{"-d:", "echo"},
			wantDelimiter: ":",
			wantCmdIdx:    1,
		},
		{
			name:        "verbose",
			args:        []string{"-t", "echo"},
			wantVerbose: true,
			wantCmdIdx:  1,
		},
		{
			name:        "combined options",
			args:        []string{"-0", "-r", "-n", "3", "rm", "-f"},
			wantNull:    true,
			wantNoRun:   true,
			wantMaxArgs: 3,
			wantCmdIdx:  4,
		},
		{
			name:       "double dash ends options",
			args:       []string{"--", "-rf"},
			wantCmdIdx: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, cmdIdx := helpers.parseXargsOptions(tt.args)

			if opts.NullDelimiter != tt.wantNull {
				t.Errorf("NullDelimiter = %v, want %v", opts.NullDelimiter, tt.wantNull)
			}
			if opts.MaxArgs != tt.wantMaxArgs {
				t.Errorf("MaxArgs = %v, want %v", opts.MaxArgs, tt.wantMaxArgs)
			}
			if opts.NoRunIfEmpty != tt.wantNoRun {
				t.Errorf("NoRunIfEmpty = %v, want %v", opts.NoRunIfEmpty, tt.wantNoRun)
			}
			if opts.Delimiter != tt.wantDelimiter {
				t.Errorf("Delimiter = %q, want %q", opts.Delimiter, tt.wantDelimiter)
			}
			if opts.Verbose != tt.wantVerbose {
				t.Errorf("Verbose = %v, want %v", opts.Verbose, tt.wantVerbose)
			}
			if cmdIdx != tt.wantCmdIdx {
				t.Errorf("cmdIdx = %v, want %v", cmdIdx, tt.wantCmdIdx)
			}
		})
	}
}

func TestHelpers_XargsReadNullDelimited(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	stdin := strings.NewReader("file1\x00file2\x00file3\x00")
	args := helpers.xargsReadNullDelimited(stdin)

	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d: %v", len(args), args)
	}

	expected := []string{"file1", "file2", "file3"}
	for i, exp := range expected {
		if i >= len(args) || args[i] != exp {
			t.Errorf("arg[%d] = %q, want %q", i, args[i], exp)
		}
	}
}

func TestHelpers_XargsReadDelimited(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	stdin := strings.NewReader("a:b:c:d")
	args := helpers.xargsReadDelimited(stdin, ":")

	if len(args) != 4 {
		t.Errorf("expected 4 args, got %d: %v", len(args), args)
	}
}

func TestHelpers_XargsReadWhitespaceDelimited(t *testing.T) {
	helpers, _, _, _ := createBuildTestHelpers(t)

	stdin := strings.NewReader("a b c\nd e f\n")
	args := helpers.xargsReadWhitespaceDelimited(stdin)

	if len(args) != 6 {
		t.Errorf("expected 6 args, got %d: %v", len(args), args)
	}

	expected := []string{"a", "b", "c", "d", "e", "f"}
	for i, exp := range expected {
		if i >= len(args) || args[i] != exp {
			t.Errorf("arg[%d] = %q, want %q", i, args[i], exp)
		}
	}
}
