package profile

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// setupTestProfile creates a test profile directory structure.
func setupTestProfile(t *testing.T, name string, files map[string]string) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	for filename, content := range files {
		path := filepath.Join(dir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

func TestLoadProfile(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		wantEAPI string
		wantUSE  string
		wantErr  bool
	}{
		{
			name: "basic profile",
			files: map[string]string{
				"eapi": "8",
				"make.defaults": `
USE="ssl unicode"
CFLAGS="-O2 -pipe"
`,
			},
			wantEAPI: "8",
			wantUSE:  "ssl unicode",
			wantErr:  false,
		},
		{
			name: "profile without EAPI",
			files: map[string]string{
				"make.defaults": `USE="ssl"`,
			},
			wantEAPI: "0", // Default
			wantUSE:  "ssl",
			wantErr:  false,
		},
		{
			name: "empty profile",
			files: map[string]string{
				"eapi": "8",
			},
			wantEAPI: "8",
			wantUSE:  "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupTestProfile(t, tt.name, tt.files)

			profile, err := LoadProfile(dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadProfile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if profile.EAPI != tt.wantEAPI {
				t.Errorf("EAPI = %v, want %v", profile.EAPI, tt.wantEAPI)
			}

			if tt.wantUSE != "" {
				use := profile.MakeDefaults["USE"]
				if use != tt.wantUSE {
					t.Errorf("USE = %v, want %v", use, tt.wantUSE)
				}
			}
		})
	}
}

func TestLoadProfile_InvalidPath(t *testing.T) {
	_, err := LoadProfile("/nonexistent/path")
	if err == nil {
		t.Error("Expected error for nonexistent path")
	}
}

func TestLoadUSEMask(t *testing.T) {
	files := map[string]string{
		"eapi": "8",
		"use.mask": `
# Comment
debug
test
selinux
`,
	}

	dir := setupTestProfile(t, "use_mask", files)
	profile, err := LoadProfile(dir)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"debug", "test", "selinux"}
	if !reflect.DeepEqual(profile.USEMask, want) {
		t.Errorf("USEMask = %v, want %v", profile.USEMask, want)
	}
}

func TestLoadUSEForce(t *testing.T) {
	files := map[string]string{
		"eapi": "8",
		"use.force": `
# Force these flags
elibc_glibc
kernel_linux
`,
	}

	dir := setupTestProfile(t, "use_force", files)
	profile, err := LoadProfile(dir)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"elibc_glibc", "kernel_linux"}
	if !reflect.DeepEqual(profile.USEForce, want) {
		t.Errorf("USEForce = %v, want %v", profile.USEForce, want)
	}
}

func TestLoadPackages(t *testing.T) {
	files := map[string]string{
		"eapi": "8",
		"packages": `
# System packages
*sys-apps/baselayout
*virtual/libc
*sys-apps/util-linux
# Comment in between
*app-shells/bash
`,
	}

	dir := setupTestProfile(t, "packages", files)
	profile, err := LoadProfile(dir)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"sys-apps/baselayout",
		"virtual/libc",
		"sys-apps/util-linux",
		"app-shells/bash",
	}

	if !reflect.DeepEqual(profile.Packages, want) {
		t.Errorf("Packages = %v, want %v", profile.Packages, want)
	}
}

func TestLoadPackageUse(t *testing.T) {
	files := map[string]string{
		"eapi": "8",
		"package.use": `
# Per-package USE flags
sys-libs/zlib minizip
app-editors/vim -python perl
dev-lang/python sqlite ssl
`,
	}

	dir := setupTestProfile(t, "package_use", files)
	profile, err := LoadProfile(dir)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		atom string
		want []string
	}{
		{"sys-libs/zlib", []string{"minizip"}},
		{"app-editors/vim", []string{"-python", "perl"}},
		{"dev-lang/python", []string{"sqlite", "ssl"}},
	}

	for _, tt := range tests {
		got := profile.PackageUse[tt.atom]
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("PackageUse[%s] = %v, want %v", tt.atom, got, tt.want)
		}
	}
}

func TestGetUSEFlags(t *testing.T) {
	files := map[string]string{
		"eapi":          "8",
		"make.defaults": `USE="ssl unicode -gtk"`,
		"use.force":     "kernel_linux",
		"use.mask":      "debug",
	}

	dir := setupTestProfile(t, "use_flags", files)
	profile, err := LoadProfile(dir)
	if err != nil {
		t.Fatal(err)
	}

	flags := profile.GetUSEFlags()

	// Should contain: ssl, unicode, -gtk, kernel_linux (forced), -debug (masked)
	expected := []string{"ssl", "unicode", "-gtk", "kernel_linux", "-debug"}

	// Check all expected flags are present
	flagMap := make(map[string]bool)
	for _, flag := range flags {
		flagMap[flag] = true
	}

	for _, exp := range expected {
		if !flagMap[exp] {
			t.Errorf("Expected flag %s not found in %v", exp, flags)
		}
	}
}

func TestGetSystemPackages(t *testing.T) {
	files := map[string]string{
		"eapi": "8",
		"packages": `
*sys-apps/baselayout
*virtual/libc
`,
	}

	dir := setupTestProfile(t, "system_packages", files)
	profile, err := LoadProfile(dir)
	if err != nil {
		t.Fatal(err)
	}

	packages := profile.GetSystemPackages()

	want := []string{"sys-apps/baselayout", "virtual/libc"}
	if !reflect.DeepEqual(packages, want) {
		t.Errorf("GetSystemPackages() = %v, want %v", packages, want)
	}
}

func TestProfileInheritance(t *testing.T) {
	// Create a parent profile
	parentFiles := map[string]string{
		"eapi": "8",
		"make.defaults": `
USE="parent-flag ssl"
ARCH="amd64"
`,
		"packages": `*sys-apps/baselayout`,
	}

	// Create child profile with parent reference
	childFiles := map[string]string{
		"eapi":          "8",
		"make.defaults": `USE="child-flag unicode"`,
		"packages":      `*virtual/libc`,
	}

	// Setup parent
	parentDir := setupTestProfile(t, "parent", parentFiles)

	// Setup child with relative parent reference
	tmpDir := t.TempDir()
	childDir := filepath.Join(tmpDir, "child")
	if err := os.MkdirAll(childDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create parent file pointing to parent directory
	parentPath := "../parent"
	childFiles["parent"] = parentPath

	// Copy parent to same temp directory
	parentDest := filepath.Join(tmpDir, "parent")
	if err := os.MkdirAll(parentDest, 0755); err != nil {
		t.Fatal(err)
	}

	for name := range parentFiles {
		src := filepath.Join(parentDir, name)
		dst := filepath.Join(parentDest, name)
		data, _ := os.ReadFile(src)
		if err := os.WriteFile(dst, data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Write child files
	for name, content := range childFiles {
		path := filepath.Join(childDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Load and resolve
	profile, err := LoadProfile(childDir)
	if err != nil {
		t.Fatal(err)
	}

	if err := profile.Resolve(); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// Check parent was loaded
	if len(profile.Parents) != 1 {
		t.Errorf("Expected 1 parent, got %d", len(profile.Parents))
	}

	// Check USE flags are merged
	flags := profile.GetUSEFlags()
	flagMap := make(map[string]bool)
	for _, flag := range flags {
		flagMap[flag] = true
	}

	// Should have flags from both parent and child
	expectedFlags := []string{"parent-flag", "ssl", "child-flag", "unicode"}
	for _, exp := range expectedFlags {
		if !flagMap[exp] {
			t.Errorf("Expected USE flag %s not found in %v", exp, flags)
		}
	}

	// Check system packages are merged
	packages := profile.GetSystemPackages()
	packageSet := make(map[string]bool)
	for _, pkg := range packages {
		packageSet[pkg] = true
	}

	if !packageSet["sys-apps/baselayout"] || !packageSet["virtual/libc"] {
		t.Errorf("System packages not merged correctly: %v", packages)
	}
}

func TestDeduplicateUSEFlags(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "no duplicates",
			input: []string{"ssl", "unicode", "gtk"},
			want:  []string{"ssl", "unicode", "gtk"},
		},
		{
			name:  "simple duplicates",
			input: []string{"ssl", "unicode", "ssl"},
			want:  []string{"unicode", "ssl"}, // Last occurrence
		},
		{
			name:  "negated flags",
			input: []string{"ssl", "-ssl", "unicode"},
			want:  []string{"-ssl", "unicode"}, // Last ssl occurrence is negated
		},
		{
			name:  "override negation",
			input: []string{"-ssl", "ssl", "unicode"},
			want:  []string{"ssl", "unicode"}, // Positive overrides negative
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicateUSEFlags(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("deduplicateUSEFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseUSEFlags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple flags",
			input: "ssl unicode gtk",
			want:  []string{"ssl", "unicode", "gtk"},
		},
		{
			name:  "flags with negation",
			input: "ssl -gtk unicode",
			want:  []string{"ssl", "-gtk", "unicode"},
		},
		{
			name:  "extra whitespace",
			input: "  ssl   unicode  ",
			want:  []string{"ssl", "unicode"},
		},
		{
			name:  "empty string",
			input: "",
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseUSEFlags(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseUSEFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetMakeDefault(t *testing.T) {
	parentFiles := map[string]string{
		"eapi": "8",
		"make.defaults": `
ARCH="amd64"
CFLAGS="-O2 -pipe"
`,
	}

	childFiles := map[string]string{
		"eapi": "8",
		"make.defaults": `
CFLAGS="-O2 -pipe -march=native"
USE="ssl"
`,
	}

	// Setup profiles
	tmpDir := t.TempDir()
	parentDir := filepath.Join(tmpDir, "parent")
	childDir := filepath.Join(tmpDir, "child")

	if err := os.MkdirAll(parentDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(childDir, 0755); err != nil {
		t.Fatal(err)
	}

	for name, content := range parentFiles {
		path := filepath.Join(parentDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	childFiles["parent"] = "../parent"
	for name, content := range childFiles {
		path := filepath.Join(childDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Load child profile
	profile, err := LoadProfile(childDir)
	if err != nil {
		t.Fatal(err)
	}

	if err := profile.Resolve(); err != nil {
		t.Fatal(err)
	}

	// Test GetMakeDefault
	tests := []struct {
		key  string
		want string
	}{
		{"ARCH", "amd64"},                     // From parent
		{"CFLAGS", "-O2 -pipe -march=native"}, // Overridden by child
		{"USE", "ssl"},                        // Only in child
		{"NONEXISTENT", ""},                   // Not defined
	}

	for _, tt := range tests {
		got := profile.GetMakeDefault(tt.key)
		if got != tt.want {
			t.Errorf("GetMakeDefault(%s) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestExtractProfileName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{
			path: "/var/db/repos/gentoo/profiles/default/linux/amd64/23.0",
			want: "default/linux/amd64/23.0",
		},
		{
			path: "/etc/portage/make.profile",
			want: "make.profile",
		},
		{
			path: "/usr/portage/profiles/base",
			want: "base",
		},
	}

	for _, tt := range tests {
		got := extractProfileName(tt.path)
		if got != tt.want {
			t.Errorf("extractProfileName(%s) = %s, want %s", tt.path, got, tt.want)
		}
	}
}

// Benchmark tests

func BenchmarkLoadProfile(b *testing.B) {
	files := map[string]string{
		"eapi": "8",
		"make.defaults": `
USE="ssl unicode gtk -qt -kde"
CFLAGS="-O2 -pipe -march=native"
CXXFLAGS="${CFLAGS}"
MAKEOPTS="-j8"
`,
		"use.mask": "debug\ntest\nselinux",
		"packages": "*sys-apps/baselayout\n*virtual/libc",
	}

	dir := filepath.Join(b.TempDir(), "profile")
	if err := os.MkdirAll(dir, 0755); err != nil {
		b.Fatal(err)
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadProfile(dir)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetUSEFlags(b *testing.B) {
	files := map[string]string{
		"eapi":          "8",
		"make.defaults": `USE="ssl unicode gtk qt kde gnome -debug -test"`,
		"use.force":     "kernel_linux\nelibc_glibc",
		"use.mask":      "selinux",
	}

	dir := filepath.Join(b.TempDir(), "profile")
	if err := os.MkdirAll(dir, 0755); err != nil {
		b.Fatal(err)
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatal(err)
		}
	}

	profile, err := LoadProfile(dir)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = profile.GetUSEFlags()
	}
}
