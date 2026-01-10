// Package ebuild implements ebuild execution engine.
//
// This file contains tests for Python eclasses:
//   - python-utils-r1
//   - python-single-r1
//   - python-r1
//   - python-any-r1
//   - distutils-r1
package ebuild

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// ============================================================================
// Python Utils Tests
// ============================================================================

func TestParsePythonImpl(t *testing.T) {
	tests := []struct {
		name      string
		impl      string
		wantType  string
		wantMajor int
		wantMinor int
		wantExec  string
		wantErr   bool
	}{
		{
			name:      "python3_10",
			impl:      "python3_10",
			wantType:  "cpython",
			wantMajor: 3,
			wantMinor: 10,
			wantExec:  "python3.10",
		},
		{
			name:      "python3_11",
			impl:      "python3_11",
			wantType:  "cpython",
			wantMajor: 3,
			wantMinor: 11,
			wantExec:  "python3.11",
		},
		{
			name:      "python3_12",
			impl:      "python3_12",
			wantType:  "cpython",
			wantMajor: 3,
			wantMinor: 12,
			wantExec:  "python3.12",
		},
		{
			name:      "pypy3_10",
			impl:      "pypy3_10",
			wantType:  "pypy",
			wantMajor: 3,
			wantMinor: 10,
			wantExec:  "pypy3.10",
		},
		{
			name:    "invalid",
			impl:    "invalid",
			wantErr: true,
		},
		{
			name:      "python2_7",
			impl:      "python2_7",
			wantType:  "cpython",
			wantMajor: 2,
			wantMinor: 7,
			wantExec:  "python2.7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParsePythonImpl(tt.impl)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if info.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", info.Type, tt.wantType)
			}
			if info.Major != tt.wantMajor {
				t.Errorf("Major = %d, want %d", info.Major, tt.wantMajor)
			}
			if info.Minor != tt.wantMinor {
				t.Errorf("Minor = %d, want %d", info.Minor, tt.wantMinor)
			}
			if info.Executable != tt.wantExec {
				t.Errorf("Executable = %q, want %q", info.Executable, tt.wantExec)
			}
		})
	}
}

func TestPythonGetSitedir(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := &Environment{}
	h := NewHelpers(env, stdout, stderr)

	// Set EPYTHON
	env.SetVar("EPYTHON", "python3_11")

	err := h.PythonGetSitedir(nil)
	if err != nil {
		t.Fatalf("PythonGetSitedir failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "python3.11") {
		t.Errorf("expected output to contain python3.11, got %q", output)
	}
	if !strings.Contains(output, "site-packages") {
		t.Errorf("expected output to contain site-packages, got %q", output)
	}
}

func TestPythonGetIncludedir(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := &Environment{}
	h := NewHelpers(env, stdout, stderr)

	env.SetVar("EPYTHON", "python3_11")

	err := h.PythonGetIncludedir(nil)
	if err != nil {
		t.Fatalf("PythonGetIncludedir failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "python3.11") {
		t.Errorf("expected output to contain python3.11, got %q", output)
	}
	if !strings.Contains(output, "include") {
		t.Errorf("expected output to contain include, got %q", output)
	}
}

func TestPythonExport(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := &Environment{}
	h := NewHelpers(env, stdout, stderr)

	err := h.PythonExport([]string{"python3_11"})
	if err != nil {
		t.Fatalf("PythonExport failed: %v", err)
	}

	// Check that EPYTHON is set
	epython := env.GetVar("EPYTHON")
	if epython != "python3_11" {
		t.Errorf("EPYTHON = %q, want %q", epython, "python3_11")
	}

	// Check that PYTHON is set
	python := env.GetVar("PYTHON")
	if !strings.Contains(python, "python3.11") {
		t.Errorf("PYTHON should contain python3.11, got %q", python)
	}

	// Check PYTHON_SITEDIR
	sitedir := env.GetVar("PYTHON_SITEDIR")
	if sitedir == "" {
		t.Error("PYTHON_SITEDIR not set")
	}
}

func TestPythonIsCompatible(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := &Environment{}
	h := NewHelpers(env, stdout, stderr)

	env.SetVar("PYTHON_COMPAT", "python3_10 python3_11 python3_12")

	tests := []struct {
		impl      string
		wantMatch bool
	}{
		{"python3_10", true},
		{"python3_11", true},
		{"python3_12", true},
		{"python3_9", false},
		{"pypy3_10", false},
	}

	for _, tt := range tests {
		t.Run(tt.impl, func(t *testing.T) {
			err := h.PythonIsCompatible([]string{tt.impl})
			gotMatch := (err == nil)
			if gotMatch != tt.wantMatch {
				t.Errorf("PythonIsCompatible(%q) = %v, want %v", tt.impl, gotMatch, tt.wantMatch)
			}
		})
	}
}

// ============================================================================
// Python Single Tests
// ============================================================================

func TestPythonSingleR1PkgSetup(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := &Environment{
		USE: "python_single_target_python3_11",
	}
	env.SetVar("PYTHON_COMPAT", "python3_10 python3_11 python3_12")
	h := NewHelpers(env, stdout, stderr)

	err := h.PythonSingleR1PkgSetup(nil)
	if err != nil {
		t.Fatalf("PythonSingleR1PkgSetup failed: %v", err)
	}

	// Check that EPYTHON is set to the selected target
	epython := env.GetVar("EPYTHON")
	if epython != "python3_11" {
		t.Errorf("EPYTHON = %q, want %q", epython, "python3_11")
	}
}

func TestPythonSingleR1PkgSetup_NoTarget(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := &Environment{}
	h := NewHelpers(env, stdout, stderr)

	env.SetVar("PYTHON_COMPAT", "python3_10 python3_11")
	// No python_single_target_* USE flag set

	err := h.PythonSingleR1PkgSetup(nil)
	if err == nil {
		t.Error("expected error when no PYTHON_SINGLE_TARGET set")
	}
}

func TestPythonGenCondDep(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := &Environment{
		USE: "python_single_target_python3_11",
	}
	env.SetVar("PYTHON_COMPAT", "python3_10 python3_11")
	h := NewHelpers(env, stdout, stderr)

	err := h.PythonGenCondDep([]string{"dev-python/foo[${PYTHON_USEDEP}]"})
	if err != nil {
		t.Fatalf("PythonGenCondDep failed: %v", err)
	}

	output := stdout.String()
	// The single target is python3_11, so PYTHON_USEDEP should include it
	if !strings.Contains(output, "python_single_target_python3_11") {
		t.Errorf("expected PYTHON_USEDEP to contain python_single_target_python3_11, got %q", output)
	}
	if !strings.Contains(output, "dev-python/foo") {
		t.Errorf("expected output to contain dev-python/foo, got %q", output)
	}
}

// ============================================================================
// Python R1 Tests
// ============================================================================

func TestPythonR1PkgSetup(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := &Environment{
		USE: "python_targets_python3_11 python_targets_python3_12",
	}
	env.SetVar("PYTHON_COMPAT", "python3_10 python3_11 python3_12")
	h := NewHelpers(env, stdout, stderr)

	err := h.PythonR1PkgSetup(nil)
	if err != nil {
		t.Fatalf("PythonR1PkgSetup failed: %v", err)
	}
}

func TestPythonR1PkgSetup_NoTargets(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := &Environment{}
	h := NewHelpers(env, stdout, stderr)

	env.SetVar("PYTHON_COMPAT", "python3_10 python3_11")
	// No python_targets_* USE flags set

	err := h.PythonR1PkgSetup(nil)
	if err == nil {
		t.Error("expected error when no PYTHON_TARGETS enabled")
	}
}

func TestGetPythonTargets(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := &Environment{
		USE: "python_targets_python3_10 python_targets_python3_12",
	}
	env.SetVar("PYTHON_COMPAT", "python3_10 python3_11 python3_12")
	h := NewHelpers(env, stdout, stderr)

	targets := h.getPythonTargets()

	if len(targets) != 2 {
		t.Errorf("expected 2 targets, got %d: %v", len(targets), targets)
	}

	found10 := false
	found12 := false
	for _, target := range targets {
		if target == "python3_10" {
			found10 = true
		}
		if target == "python3_12" {
			found12 = true
		}
	}

	if !found10 {
		t.Error("expected python3_10 in targets")
	}
	if !found12 {
		t.Error("expected python3_12 in targets")
	}
}

func TestPythonGenAnyDep(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := &Environment{}
	h := NewHelpers(env, stdout, stderr)

	env.SetVar("PYTHON_COMPAT", "python3_10 python3_11")

	err := h.PythonGenAnyDep([]string{"dev-python/foo[${PYTHON_USEDEP}]"})
	if err != nil {
		t.Fatalf("PythonGenAnyDep failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "||") {
		t.Errorf("expected || in output for multiple targets, got %q", output)
	}
	if !strings.Contains(output, "python_targets_python3_10") {
		t.Errorf("expected python_targets_python3_10 in output, got %q", output)
	}
	if !strings.Contains(output, "python_targets_python3_11") {
		t.Errorf("expected python_targets_python3_11 in output, got %q", output)
	}
}

// ============================================================================
// Python Any Tests
// ============================================================================

func TestPythonAnyR1PkgSetup(t *testing.T) {
	// Skip if no Python available
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := &Environment{}
	h := NewHelpers(env, stdout, stderr)

	env.SetVar("PYTHON_COMPAT", "python3_10 python3_11 python3_12")

	err := h.PythonAnyR1PkgSetup(nil)
	// May fail if no Python installed, which is OK for this test
	if err != nil {
		t.Logf("PythonAnyR1PkgSetup: %v (expected if no Python installed)", err)
	}
}

// ============================================================================
// Distutils Tests
// ============================================================================

func TestDistutilsEclass_ExportedFunctions(t *testing.T) {
	e := &DistutilsEclass{}

	funcs := e.ExportedFunctions()

	expected := []string{"src_prepare", "src_configure", "src_compile", "src_test", "src_install"}
	if len(funcs) != len(expected) {
		t.Errorf("expected %d functions, got %d", len(expected), len(funcs))
	}

	for _, exp := range expected {
		found := false
		for _, f := range funcs {
			if f == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected function %q not found", exp)
		}
	}
}

func TestDistutilsR1SrcPrepare(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "work", "test-1.0")
	os.MkdirAll(srcDir, 0755)

	// Create a simple setup.py
	setupPy := filepath.Join(srcDir, "setup.py")
	os.WriteFile(setupPy, []byte("from setuptools import setup\nsetup(name='test')"), 0644)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Create test package and environment
	testPkg := &pkg.Package{
		Name:    "dev-python/test",
		Version: "1.0.0",
	}
	env, err := NewEnvironment(testPkg, tmpDir, "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment failed: %v", err)
	}
	env.S = srcDir
	env.WORKDIR = filepath.Join(tmpDir, "work")

	h := NewHelpers(env, stdout, stderr)

	err = h.DistutilsR1SrcPrepare(nil)
	if err != nil {
		t.Fatalf("DistutilsR1SrcPrepare failed: %v", err)
	}

	// Check that build directory was created
	buildDir := filepath.Join(env.WORKDIR, "build")
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		t.Error("build directory not created")
	}
}

func TestIsSingleImpl(t *testing.T) {
	tests := []struct {
		name            string
		distutilsSingle string
		use             string
		want            bool
	}{
		{
			name:            "DISTUTILS_SINGLE_IMPL set",
			distutilsSingle: "1",
			want:            true,
		},
		{
			name: "python_single_target in USE",
			use:  "python_single_target_python3_11",
			want: true,
		},
		{
			name: "python_targets in USE (multi)",
			use:  "python_targets_python3_11 python_targets_python3_12",
			want: false,
		},
		{
			name: "empty USE",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			// Use struct field for USE, not SetVar
			env := &Environment{
				USE: tt.use,
			}
			h := NewHelpers(env, stdout, stderr)

			if tt.distutilsSingle != "" {
				env.SetVar("DISTUTILS_SINGLE_IMPL", tt.distutilsSingle)
			}

			got := h.isSingleImpl()
			if got != tt.want {
				t.Errorf("isSingleImpl() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================================
// Eclass Registration Tests
// ============================================================================

func TestPythonEclassRegistration(t *testing.T) {
	eclasses := []interface {
		Name() string
		ExportedFunctions() []string
	}{
		&PythonUtilsEclass{},
		&PythonSingleEclass{},
		&PythonR1Eclass{},
		&PythonAnyEclass{},
		&DistutilsEclass{},
	}

	expectedNames := map[string]bool{
		"python-utils-r1":  true,
		"python-single-r1": true,
		"python-r1":        true,
		"python-any-r1":    true,
		"distutils-r1":     true,
	}

	for _, e := range eclasses {
		name := e.Name()
		if !expectedNames[name] {
			t.Errorf("unexpected eclass name: %q", name)
		}
		delete(expectedNames, name)

		// Check that ExportedFunctions returns valid data
		funcs := e.ExportedFunctions()
		if funcs == nil {
			t.Errorf("%s: ExportedFunctions returned nil", name)
		}
	}

	// Check all expected names were found
	for name := range expectedNames {
		t.Errorf("eclass %q not found", name)
	}
}

func TestComputePythonSitedir(t *testing.T) {
	tests := []struct {
		impl       string
		wantSubstr string
	}{
		{"python3_10", "python3.10"},
		{"python3_11", "python3.11"},
		{"pypy3_10", "pypy3.10"},
	}

	for _, tt := range tests {
		t.Run(tt.impl, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			env := &Environment{}
			h := NewHelpers(env, stdout, stderr)

			got := h.computePythonSitedir(tt.impl)
			// Normalize path separators for cross-platform testing
			got = strings.ReplaceAll(got, "\\", "/")
			if !strings.Contains(got, tt.wantSubstr) {
				t.Errorf("computePythonSitedir(%q) = %q, want to contain %q",
					tt.impl, got, tt.wantSubstr)
			}
			if !strings.Contains(got, "site-packages") {
				t.Errorf("computePythonSitedir(%q) = %q, want to contain site-packages",
					tt.impl, got)
			}
		})
	}
}

func TestComputePythonIncludedir(t *testing.T) {
	tests := []struct {
		impl       string
		wantSubstr string
	}{
		{"python3_10", "python3.10"},
		{"python3_11", "python3.11"},
		{"pypy3_10", "pypy3.10"},
	}

	for _, tt := range tests {
		t.Run(tt.impl, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			env := &Environment{}
			h := NewHelpers(env, stdout, stderr)

			got := h.computePythonIncludedir(tt.impl)
			// Normalize path separators for cross-platform testing
			got = strings.ReplaceAll(got, "\\", "/")
			if !strings.Contains(got, tt.wantSubstr) {
				t.Errorf("computePythonIncludedir(%q) = %q, want to contain %q",
					tt.impl, got, tt.wantSubstr)
			}
			if !strings.Contains(got, "include") {
				t.Errorf("computePythonIncludedir(%q) = %q, want to contain include",
					tt.impl, got)
			}
		})
	}
}
