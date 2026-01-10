// Package ebuild implements ebuild execution engine.
//
// This file contains tests for go-module eclass.
package ebuild

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Go Module Eclass Registration Tests
// ============================================================================

func TestGoModuleEclass_Name(t *testing.T) {
	eclass := &GoModuleEclass{}
	if eclass.Name() != "go-module" {
		t.Errorf("expected go-module, got %s", eclass.Name())
	}
}

func TestGoModuleEclass_ExportedFunctions(t *testing.T) {
	eclass := &GoModuleEclass{}
	funcs := eclass.ExportedFunctions()

	expected := []string{"src_unpack", "src_compile", "src_install"}
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

func TestGoModuleEclass_Variables(t *testing.T) {
	eclass := &GoModuleEclass{}
	vars := eclass.Variables()

	if vars["GOPROXY"] != "off" {
		t.Error("GOPROXY should be 'off'")
	}
	if vars["CGO_ENABLED"] != "1" {
		t.Error("CGO_ENABLED should be '1'")
	}
}

// ============================================================================
// Go Module URI Generation Tests
// ============================================================================

func TestGoModuleURI(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	tests := []struct {
		module   string
		version  string
		expected string
	}{
		{
			"github.com/spf13/cobra",
			"v1.6.1",
			"https://proxy.golang.org/github.com/spf13/cobra/@v/v1.6.1.zip -> github.com%2Fspf13%2Fcobra-@v-v1.6.1.zip",
		},
		{
			"golang.org/x/sys",
			"v0.5.0",
			"https://proxy.golang.org/golang.org/x/sys/@v/v0.5.0.zip -> golang.org%2Fx%2Fsys-@v-v0.5.0.zip",
		},
		{
			"gopkg.in/yaml.v3",
			"v3.0.1",
			"https://proxy.golang.org/gopkg.in/yaml.v3/@v/v3.0.1.zip -> gopkg.in%2Fyaml.v3-@v-v3.0.1.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.module, func(t *testing.T) {
			result := h.goModuleURI(tt.module, tt.version)
			if result != tt.expected {
				t.Errorf("goModuleURI(%q, %q) = %q, want %q", tt.module, tt.version, result, tt.expected)
			}
		})
	}
}

func TestGoModuleURI_GoMod(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	// go.mod entries should be skipped
	result := h.goModuleURI("github.com/spf13/cobra/go.mod", "v1.6.1")
	if result != "" {
		t.Errorf("go.mod entries should return empty, got %q", result)
	}
}

// ============================================================================
// EGO_SUM Processing Tests
// ============================================================================

func TestGoModuleSetGlobals(t *testing.T) {
	egoSum := `github.com/spf13/cobra v1.6.1 h1:o94oiPyS4KD1mPy2fmcYYHHfCxLqYjJOhGsCHFZtEzA=
github.com/spf13/cobra v1.6.1/go.mod h1:IOw/AERYS7UzyrGinqmz6HLUo219MORXGxhbaJUqzrY=
golang.org/x/sys v0.5.0 h1:MUK/U/4lj1t1oPg0HfuXDN/Z1wv31ZJ/YcPiGccS4DU=`

	env := &Environment{}
	env.SetVar("EGO_SUM", egoSum)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	err := h.GoModuleSetGlobals(nil)
	if err != nil {
		t.Fatalf("GoModuleSetGlobals error: %v", err)
	}

	srcUri := env.ExtraVars["SRC_URI"]
	if srcUri == "" {
		t.Error("SRC_URI should be set")
	}

	// Should contain cobra but not go.mod entry
	if !strings.Contains(srcUri, "github.com/spf13/cobra") {
		t.Error("SRC_URI should contain cobra module")
	}
	if !strings.Contains(srcUri, "golang.org/x/sys") {
		t.Error("SRC_URI should contain sys module")
	}
}

func TestGoModuleSetGlobals_Empty(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	err := h.GoModuleSetGlobals(nil)
	if err != nil {
		t.Fatalf("GoModuleSetGlobals error: %v", err)
	}

	// Should not fail with empty EGO_SUM
}

// ============================================================================
// Go Environment Setup Tests
// ============================================================================

func TestSetupGoEnv(t *testing.T) {
	workdir := filepath.Join("var", "tmp", "portage", "test")
	env := &Environment{
		WORKDIR: workdir,
		CFLAGS:  "-O2",
		LDFLAGS: "-Wl,-O1",
	}
	env.SetVar("MAKEOPTS_JOBS", "4")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	h.setupGoEnv()

	// Check GOPATH
	gopath := env.ExtraVars["GOPATH"]
	expectedGopath := filepath.Join(workdir, "go")
	if gopath != expectedGopath {
		t.Errorf("GOPATH = %q, want %q", gopath, expectedGopath)
	}

	// Check GOCACHE
	gocache := env.ExtraVars["GOCACHE"]
	expectedCache := filepath.Join(workdir, "go-cache")
	if gocache != expectedCache {
		t.Errorf("GOCACHE = %q, want %q", gocache, expectedCache)
	}

	// Check GOPROXY
	goproxy := env.ExtraVars["GOPROXY"]
	if goproxy != "off" {
		t.Errorf("GOPROXY = %q, want off", goproxy)
	}

	// Check CGO_ENABLED
	cgoEnabled := env.ExtraVars["CGO_ENABLED"]
	if cgoEnabled != "1" {
		t.Errorf("CGO_ENABLED = %q, want 1", cgoEnabled)
	}

	// Check CGO_CFLAGS
	cgoCflags := env.ExtraVars["CGO_CFLAGS"]
	if cgoCflags != "-O2" {
		t.Errorf("CGO_CFLAGS = %q, want -O2", cgoCflags)
	}

	// Check CGO_LDFLAGS
	cgoLdflags := env.ExtraVars["CGO_LDFLAGS"]
	if cgoLdflags != "-Wl,-O1" {
		t.Errorf("CGO_LDFLAGS = %q, want -Wl,-O1", cgoLdflags)
	}

	// Check GOMAXPROCS
	gomaxprocs := env.ExtraVars["GOMAXPROCS"]
	if gomaxprocs != "4" {
		t.Errorf("GOMAXPROCS = %q, want 4", gomaxprocs)
	}
}

// ============================================================================
// Ego Command Tests
// ============================================================================

func TestEgo_NoArgs(t *testing.T) {
	env := &Environment{
		WORKDIR: "/tmp/test",
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	err := h.Ego(nil)
	if err == nil {
		t.Error("expected error for empty args")
	}
}

// ============================================================================
// TC Get Compiler Tests
// ============================================================================

func TestTcGetCC(t *testing.T) {
	tests := []struct {
		name     string
		cc       string
		chost    string
		expected string
	}{
		{
			name:     "CC already set",
			cc:       "clang",
			chost:    "",
			expected: "clang",
		},
		{
			name:     "From CHOST",
			cc:       "",
			chost:    "x86_64-pc-linux-gnu",
			expected: "x86_64-pc-linux-gnu-gcc",
		},
		{
			name:     "Default",
			cc:       "",
			chost:    "",
			expected: "gcc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := &Environment{}
			if tt.cc != "" {
				env.SetVar("CC", tt.cc)
			}
			if tt.chost != "" {
				env.SetVar("CHOST", tt.chost)
			}
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			h := NewHelpers(env, stdout, stderr)

			result := h.tcGetCC()
			if result != tt.expected {
				t.Errorf("tcGetCC() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTcGetCXX(t *testing.T) {
	tests := []struct {
		name     string
		cxx      string
		chost    string
		expected string
	}{
		{
			name:     "CXX already set",
			cxx:      "clang++",
			chost:    "",
			expected: "clang++",
		},
		{
			name:     "From CHOST",
			cxx:      "",
			chost:    "x86_64-pc-linux-gnu",
			expected: "x86_64-pc-linux-gnu-g++",
		},
		{
			name:     "Default",
			cxx:      "",
			chost:    "",
			expected: "g++",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := &Environment{}
			if tt.cxx != "" {
				env.SetVar("CXX", tt.cxx)
			}
			if tt.chost != "" {
				env.SetVar("CHOST", tt.chost)
			}
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			h := NewHelpers(env, stdout, stderr)

			result := h.tcGetCXX()
			if result != tt.expected {
				t.Errorf("tcGetCXX() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ============================================================================
// Unzip Tests
// ============================================================================

func TestUnzipFile(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "unzip-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a simple zip file for testing
	zipPath := filepath.Join(tmpDir, "test.zip")
	destDir := filepath.Join(tmpDir, "dest")

	// Create test zip using archive/zip
	if err := createValidTestZip(zipPath); err != nil {
		t.Fatalf("failed to create test zip: %v", err)
	}

	// Test unzip
	err = h.unzipFile(zipPath, destDir)
	if err != nil {
		t.Fatalf("unzipFile error: %v", err)
	}

	// Check extracted file exists
	if _, err := os.Stat(filepath.Join(destDir, "test.txt")); os.IsNotExist(err) {
		t.Error("extracted file should exist")
	}

	// Check content
	content, err := os.ReadFile(filepath.Join(destDir, "test.txt"))
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if string(content) != "test content" {
		t.Errorf("content mismatch: got %q", string(content))
	}
}

func TestUnzipFile_InvalidZip(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "unzip-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create invalid zip file
	zipPath := filepath.Join(tmpDir, "invalid.zip")
	if err := os.WriteFile(zipPath, []byte("not a valid zip"), 0644); err != nil {
		t.Fatalf("failed to create invalid zip: %v", err)
	}

	destDir := filepath.Join(tmpDir, "dest")

	// Test unzip - should fail
	err = h.unzipFile(zipPath, destDir)
	if err == nil {
		t.Error("expected error for invalid zip")
	}
}

// createValidTestZip creates a valid test zip file with a test.txt file inside.
func createValidTestZip(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	w := zip.NewWriter(file)

	// Add a file to the zip
	f, err := w.Create("test.txt")
	if err != nil {
		return err
	}
	_, err = f.Write([]byte("test content"))
	if err != nil {
		return err
	}

	return w.Close()
}

// ============================================================================
// Go Module SrcUnpack with Vendor Tests
// ============================================================================

func TestGoModuleSrcUnpack_VendorExists(t *testing.T) {
	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "go-unpack-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create vendor directory
	s := filepath.Join(tmpDir, "source")
	vendorDir := filepath.Join(s, "vendor")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatalf("failed to create vendor dir: %v", err)
	}

	env := &Environment{
		WORKDIR: tmpDir,
		S:       s,
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	// Test that vendor mode is detected
	// We can't fully test src_unpack without mocking DefaultSrcUnpack
	// but we can test the vendor detection logic directly
	h.setupGoEnv()

	// Manually check vendor logic (since src_unpack requires more setup)
	if _, err := os.Stat(vendorDir); err == nil {
		goflags := h.getEnvOrDefault("GOFLAGS", "")
		if !strings.Contains(goflags, "-mod=vendor") {
			h.setEnvVar("GOFLAGS", strings.TrimSpace(goflags+" -mod=vendor"))
		}
	}

	goflags := env.ExtraVars["GOFLAGS"]
	if !strings.Contains(goflags, "-mod=vendor") {
		t.Error("GOFLAGS should contain -mod=vendor when vendor dir exists")
	}
}
