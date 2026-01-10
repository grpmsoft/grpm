// Package ebuild implements ebuild execution engine.
//
// This file contains tests for cargo eclass.
package ebuild

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Cargo Eclass Registration Tests
// ============================================================================

func TestCargoEclass_Name(t *testing.T) {
	eclass := &CargoEclass{}
	if eclass.Name() != "cargo" {
		t.Errorf("expected cargo, got %s", eclass.Name())
	}
}

func TestCargoEclass_ExportedFunctions(t *testing.T) {
	eclass := &CargoEclass{}
	funcs := eclass.ExportedFunctions()

	expected := []string{"src_unpack", "src_configure", "src_compile", "src_test", "src_install"}
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

func TestCargoEclass_Variables(t *testing.T) {
	eclass := &CargoEclass{}
	vars := eclass.Variables()

	if _, ok := vars["CARGO_HOME"]; !ok {
		t.Error("missing CARGO_HOME variable")
	}
	if vars["CARGO_NET_OFFLINE"] != "true" {
		t.Error("CARGO_NET_OFFLINE should be 'true'")
	}
}

// ============================================================================
// Crate Name Parsing Tests
// ============================================================================

func TestParseCrateName(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	tests := []struct {
		input       string
		wantName    string
		wantVersion string
	}{
		{"libc-0.2.150", "libc", "0.2.150"},
		{"proc-macro2-1.0.69", "proc-macro2", "1.0.69"},
		{"quote-1.0.33", "quote", "1.0.33"},
		{"syn-2.0.39", "syn", "2.0.39"},
		{"serde-1.0.193", "serde", "1.0.193"},
		{"unicode-ident-1.0.12", "unicode-ident", "1.0.12"},
		{"cfg-if-1.0.0", "cfg-if", "1.0.0"},
		{"once_cell-1.18.0", "once_cell", "1.18.0"},
		{"regex-1.10.2", "regex", "1.10.2"},
		{"anyhow-1.0.75", "anyhow", "1.0.75"},
		// Edge cases
		{"a-1.0", "a", "1.0"},
		{"test-crate-name-0.1.0", "test-crate-name", "0.1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, version := h.parseCrateName(tt.input)
			if name != tt.wantName {
				t.Errorf("parseCrateName(%q) name = %q, want %q", tt.input, name, tt.wantName)
			}
			if version != tt.wantVersion {
				t.Errorf("parseCrateName(%q) version = %q, want %q", tt.input, version, tt.wantVersion)
			}
		})
	}
}

func TestParseCrateName_Invalid(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	tests := []string{
		"nocrate",
		"no-version",
		"",
	}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			name, version := h.parseCrateName(tt)
			if name != "" || version != "" {
				t.Errorf("parseCrateName(%q) should return empty, got name=%q version=%q", tt, name, version)
			}
		})
	}
}

// ============================================================================
// Crate URI Generation Tests
// ============================================================================

func TestCrateDependencyURI(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	tests := []struct {
		crate    string
		expected string
	}{
		{
			"libc-0.2.150",
			"https://crates.io/api/v1/crates/libc/0.2.150/download -> libc-0.2.150.crate",
		},
		{
			"proc-macro2-1.0.69",
			"https://crates.io/api/v1/crates/proc-macro2/1.0.69/download -> proc-macro2-1.0.69.crate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.crate, func(t *testing.T) {
			uri := h.crateDependencyURI(tt.crate)
			if uri != tt.expected {
				t.Errorf("crateDependencyURI(%q) = %q, want %q", tt.crate, uri, tt.expected)
			}
		})
	}
}

func TestCargoCrateUris(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	crates := []string{"libc-0.2.150", "proc-macro2-1.0.69"}
	err := h.CargoCrateUris(crates)
	if err != nil {
		t.Fatalf("CargoCrateUris error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "libc/0.2.150/download") {
		t.Error("output should contain libc URI")
	}
	if !strings.Contains(output, "proc-macro2/1.0.69/download") {
		t.Error("output should contain proc-macro2 URI")
	}
}

// ============================================================================
// CFLAGS to RUSTFLAGS Conversion Tests
// ============================================================================

func TestConvertCflagsToRustflags(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	tests := []struct {
		name     string
		cflags   string
		contains []string
		excludes []string
	}{
		{
			name:     "Optimization O2",
			cflags:   "-O2",
			contains: []string{"-C", "opt-level=2"},
		},
		{
			name:     "Optimization O3",
			cflags:   "-O3",
			contains: []string{"-C", "opt-level=3"},
		},
		{
			name:     "Debug info",
			cflags:   "-g",
			contains: []string{"-C", "debuginfo=2"},
		},
		{
			name:     "March native",
			cflags:   "-march=native",
			contains: []string{"-C", "target-cpu=native"},
		},
		{
			name:     "PIC flag",
			cflags:   "-fPIC",
			contains: []string{"-C", "relocation-model=pic"},
		},
		{
			name:     "Pipe flag ignored",
			cflags:   "-pipe",
			excludes: []string{"pipe"},
		},
		{
			name:     "Combined flags",
			cflags:   "-O2 -g -march=native",
			contains: []string{"opt-level=2", "debuginfo=2", "target-cpu=native"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.convertCflagsToRustflags(tt.cflags)

			for _, c := range tt.contains {
				if !strings.Contains(result, c) {
					t.Errorf("result should contain %q, got %q", c, result)
				}
			}

			for _, e := range tt.excludes {
				if strings.Contains(result, e) {
					t.Errorf("result should not contain %q, got %q", e, result)
				}
			}
		})
	}
}

// ============================================================================
// Rust Target Conversion Tests
// ============================================================================

func TestChostToRustTarget(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	tests := []struct {
		chost    string
		expected string
	}{
		{"x86_64-pc-linux-gnu", "x86_64-unknown-linux-gnu"},
		{"i686-pc-linux-gnu", "i686-unknown-linux-gnu"},
		{"aarch64-unknown-linux-gnu", "aarch64-unknown-linux-gnu"},
		{"armv7-unknown-linux-gnueabihf", "armv7-unknown-linux-gnueabihf"},
		{"powerpc64-unknown-linux-gnu", "powerpc64-unknown-linux-gnu"},
	}

	for _, tt := range tests {
		t.Run(tt.chost, func(t *testing.T) {
			result := h.chostToRustTarget(tt.chost)
			if result != tt.expected {
				t.Errorf("chostToRustTarget(%q) = %q, want %q", tt.chost, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// Cargo Config Generation Tests
// ============================================================================

func TestGenerateCargoConfig(t *testing.T) {
	env := &Environment{}
	env.SetVar("MAKEOPTS_JOBS", "4")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "cargo-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	vendorDir := filepath.Join(tmpDir, "vendor")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatalf("failed to create vendor dir: %v", err)
	}

	// Generate config
	if err := h.generateCargoConfig(tmpDir, vendorDir); err != nil {
		t.Fatalf("generateCargoConfig error: %v", err)
	}

	// Check config file
	configPath := filepath.Join(tmpDir, ".cargo", "config.toml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	configStr := string(content)

	if !strings.Contains(configStr, "[source.crates-io]") {
		t.Error("config should contain [source.crates-io]")
	}
	if !strings.Contains(configStr, "replace-with = \"vendored-sources\"") {
		t.Error("config should replace with vendored-sources")
	}
	if !strings.Contains(configStr, "[source.vendored-sources]") {
		t.Error("config should have vendored-sources section")
	}
	if !strings.Contains(configStr, "offline = true") {
		t.Error("config should enable offline mode")
	}
	if !strings.Contains(configStr, "jobs = 4") {
		t.Error("config should set jobs")
	}
}

// ============================================================================
// Cargo Environment Setup Tests
// ============================================================================

func TestSetupCargoEnv(t *testing.T) {
	workdir := filepath.Join("var", "tmp", "portage", "test")
	env := &Environment{
		WORKDIR: workdir,
		CFLAGS:  "-O2 -march=native",
	}
	env.SetVar("MAKEOPTS_JOBS", "8")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	h.setupCargoEnv()

	// Check CARGO_HOME (path-separator independent)
	cargoHome := env.ExtraVars["CARGO_HOME"]
	expectedHome := filepath.Join(workdir, ".cargo")
	if cargoHome != expectedHome {
		t.Errorf("CARGO_HOME = %q, want %q", cargoHome, expectedHome)
	}

	// Check ECARGO_VENDOR
	vendor := env.ExtraVars["ECARGO_VENDOR"]
	expectedVendor := filepath.Join(workdir, ".cargo", "vendor")
	if vendor != expectedVendor {
		t.Errorf("ECARGO_VENDOR = %q, want %q", vendor, expectedVendor)
	}

	// Check offline mode
	offline := env.ExtraVars["CARGO_NET_OFFLINE"]
	if offline != "true" {
		t.Errorf("CARGO_NET_OFFLINE = %q, want true", offline)
	}

	// Check jobs
	jobs := env.ExtraVars["CARGO_BUILD_JOBS"]
	if jobs != "8" {
		t.Errorf("CARGO_BUILD_JOBS = %q, want 8", jobs)
	}

	// Check RUSTFLAGS conversion
	rustflags := env.ExtraVars["CARGO_ENCODED_RUSTFLAGS"]
	if !strings.Contains(rustflags, "opt-level=2") {
		t.Error("RUSTFLAGS should contain opt-level=2")
	}
}

func TestCargoEnv(t *testing.T) {
	env := &Environment{
		WORKDIR: "/tmp/test",
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	err := h.CargoEnv(nil)
	if err != nil {
		t.Fatalf("CargoEnv error: %v", err)
	}

	if env.ExtraVars["CARGO_HOME"] == "" {
		t.Error("CARGO_HOME should be set")
	}
}

// ============================================================================
// Cargo Phase Function Tests
// ============================================================================

func TestCargoSrcConfigure(t *testing.T) {
	env := &Environment{
		WORKDIR: "/tmp/test",
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	err := h.CargoSrcConfigure(nil)
	if err != nil {
		t.Fatalf("CargoSrcConfigure error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Cargo configure complete") {
		t.Error("should output configure message")
	}
}

// ============================================================================
// File Copy Tests
// ============================================================================

func TestCopyFilePreserve(t *testing.T) {
	env := &Environment{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	h := NewHelpers(env, stdout, stderr)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "copy-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create source file with executable bit
	srcPath := filepath.Join(tmpDir, "source")
	content := []byte("test content")
	if err := os.WriteFile(srcPath, content, 0755); err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	// Copy file
	dstPath := filepath.Join(tmpDir, "dest")
	if err := h.copyFilePreserve(srcPath, dstPath); err != nil {
		t.Fatalf("copyFilePreserve error: %v", err)
	}

	// Verify content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read dest: %v", err)
	}
	if string(dstContent) != string(content) {
		t.Error("content mismatch")
	}

	// Verify file was created (mode preservation is platform-specific)
	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("failed to stat dest: %v", err)
	}
	if info.Size() != int64(len(content)) {
		t.Errorf("size mismatch: got %d, want %d", info.Size(), len(content))
	}
}

// ============================================================================
// isDigit Tests
// ============================================================================

func TestIsDigit(t *testing.T) {
	tests := []struct {
		b    byte
		want bool
	}{
		{'0', true},
		{'1', true},
		{'9', true},
		{'a', false},
		{'-', false},
		{'.', false},
	}

	for _, tt := range tests {
		t.Run(string(tt.b), func(t *testing.T) {
			if got := isDigit(tt.b); got != tt.want {
				t.Errorf("isDigit(%q) = %v, want %v", tt.b, got, tt.want)
			}
		})
	}
}
