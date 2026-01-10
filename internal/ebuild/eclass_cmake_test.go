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
// CmakeEclass Tests
// ============================================================================

func TestCmakeEclass_Name(t *testing.T) {
	eclass := &CmakeEclass{}
	if got := eclass.Name(); got != "cmake" {
		t.Errorf("Name() = %q, want %q", got, "cmake")
	}
}

func TestCmakeEclass_ExportedFunctions(t *testing.T) {
	eclass := &CmakeEclass{}
	exported := eclass.ExportedFunctions()

	expected := []string{
		"src_prepare",
		"src_configure",
		"src_compile",
		"src_test",
		"src_install",
	}

	if len(exported) != len(expected) {
		t.Errorf("ExportedFunctions() returned %d functions, want %d", len(exported), len(expected))
		return
	}

	for i, want := range expected {
		if exported[i] != want {
			t.Errorf("ExportedFunctions()[%d] = %q, want %q", i, exported[i], want)
		}
	}
}

func TestCmakeEclass_Variables(t *testing.T) {
	eclass := &CmakeEclass{}
	vars := eclass.Variables()

	tests := []struct {
		key   string
		value string
	}{
		{"CMAKE_MAKEFILE_GENERATOR", "ninja"},
		{"CMAKE_BUILD_TYPE", "Release"},
		{"CMAKE_WARN_UNUSED_CLI", "yes"},
	}

	for _, tt := range tests {
		if got := vars[tt.key]; got != tt.value {
			t.Errorf("Variables()[%q] = %q, want %q", tt.key, got, tt.value)
		}
	}
}

// ============================================================================
// CmakeUse Tests
// ============================================================================

func TestCmakeUse(t *testing.T) {
	tests := []struct {
		name     string
		useFlags map[string]bool
		args     []string
		want     string
		wantErr  bool
	}{
		{
			name:     "flag enabled with default option",
			useFlags: map[string]bool{"ssl": true},
			args:     []string{"ssl"},
			want:     "-DSSL=ON",
		},
		{
			name:     "flag disabled with default option",
			useFlags: map[string]bool{"ssl": false},
			args:     []string{"ssl"},
			want:     "-DSSL=OFF",
		},
		{
			name:     "flag enabled with custom option",
			useFlags: map[string]bool{"ssl": true},
			args:     []string{"ssl", "OPENSSL_SUPPORT"},
			want:     "-DOPENSSL_SUPPORT=ON",
		},
		{
			name:     "flag disabled with custom option",
			useFlags: map[string]bool{"ssl": false},
			args:     []string{"ssl", "OPENSSL_SUPPORT"},
			want:     "-DOPENSSL_SUPPORT=OFF",
		},
		{
			name:    "missing argument",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			p := &pkg.Package{
				Name:     "test/test",
				Version:  "1.0",
				UseFlags: tt.useFlags,
			}
			env, _ := NewEnvironment(p, "/tmp", "/var/db/repos/gentoo", "/var/cache/distfiles")
			h := NewHelpers(env, &stdout, nil)

			err := h.CmakeUse(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("CmakeUse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && stdout.String() != tt.want {
				t.Errorf("CmakeUse() output = %q, want %q", stdout.String(), tt.want)
			}
		})
	}
}

// ============================================================================
// CmakeUseFindPackage Tests
// ============================================================================

func TestCmakeUseFindPackage(t *testing.T) {
	tests := []struct {
		name     string
		useFlags map[string]bool
		args     []string
		want     string
		wantErr  bool
	}{
		{
			name:     "flag enabled - DISABLE is OFF",
			useFlags: map[string]bool{"ssl": true},
			args:     []string{"ssl", "OpenSSL"},
			want:     "-DCMAKE_DISABLE_FIND_PACKAGE_OpenSSL=OFF",
		},
		{
			name:     "flag disabled - DISABLE is ON",
			useFlags: map[string]bool{"ssl": false},
			args:     []string{"ssl", "OpenSSL"},
			want:     "-DCMAKE_DISABLE_FIND_PACKAGE_OpenSSL=ON",
		},
		{
			name:     "different package name",
			useFlags: map[string]bool{"zlib": true},
			args:     []string{"zlib", "ZLIB"},
			want:     "-DCMAKE_DISABLE_FIND_PACKAGE_ZLIB=OFF",
		},
		{
			name:    "missing arguments",
			args:    []string{"ssl"},
			wantErr: true,
		},
		{
			name:    "no arguments",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			p := &pkg.Package{
				Name:     "test/test",
				Version:  "1.0",
				UseFlags: tt.useFlags,
			}
			env, _ := NewEnvironment(p, "/tmp", "/var/db/repos/gentoo", "/var/cache/distfiles")
			h := NewHelpers(env, &stdout, nil)

			err := h.CmakeUseFindPackage(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("CmakeUseFindPackage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && stdout.String() != tt.want {
				t.Errorf("CmakeUseFindPackage() output = %q, want %q", stdout.String(), tt.want)
			}
		})
	}
}

// ============================================================================
// CmakeCommentAddSubdirectory Tests
// ============================================================================

func TestCmakeCommentAddSubdirectory(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		subdir      string
		wantContent string
		wantErr     bool
	}{
		{
			name:        "comment simple subdirectory",
			content:     "add_subdirectory(tests)\nadd_subdirectory(src)\n",
			subdir:      "tests",
			wantContent: "# add_subdirectory(tests) # Commented by cmake_comment_add_subdirectory\nadd_subdirectory(src)\n",
		},
		{
			name:        "comment with spaces",
			content:     "add_subdirectory( tests )\nadd_subdirectory(src)\n",
			subdir:      "tests",
			wantContent: "# add_subdirectory( tests ) # Commented by cmake_comment_add_subdirectory\nadd_subdirectory(src)\n",
		},
		{
			name:        "already commented - no change",
			content:     "# add_subdirectory(tests)\nadd_subdirectory(src)\n",
			subdir:      "tests",
			wantContent: "# add_subdirectory(tests)\nadd_subdirectory(src)\n",
		},
		{
			name:        "subdirectory not found",
			content:     "add_subdirectory(src)\n",
			subdir:      "tests",
			wantContent: "add_subdirectory(src)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory structure
			tmpDir, err := os.MkdirTemp("", "cmake-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()

			// Create CMakeLists.txt
			cmakeFile := filepath.Join(tmpDir, "CMakeLists.txt")
			if err := os.WriteFile(cmakeFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write CMakeLists.txt: %v", err)
			}

			// Create helpers with the temp directory as source
			var stdout, stderr bytes.Buffer
			p := &pkg.Package{
				Name:     "test/test",
				Version:  "1.0",
				UseFlags: make(map[string]bool),
			}
			env, _ := NewEnvironment(p, "/tmp", "/var/db/repos/gentoo", "/var/cache/distfiles")
			env.S = tmpDir
			h := NewHelpers(env, &stdout, &stderr)

			// Set CMAKE_USE_DIR
			env.SetVar("CMAKE_USE_DIR", tmpDir)

			err = h.CmakeCommentAddSubdirectory([]string{tt.subdir})
			if (err != nil) != tt.wantErr {
				t.Errorf("CmakeCommentAddSubdirectory() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Read back the file
			got, err := os.ReadFile(cmakeFile)
			if err != nil {
				t.Fatalf("Failed to read CMakeLists.txt: %v", err)
			}

			if string(got) != tt.wantContent {
				t.Errorf("CmakeCommentAddSubdirectory() result = %q, want %q", string(got), tt.wantContent)
			}
		})
	}
}

func TestCmakeCommentAddSubdirectory_MissingArgs(t *testing.T) {
	h := NewHelpers(nil, nil, nil)
	err := h.CmakeCommentAddSubdirectory([]string{})
	if err == nil {
		t.Error("CmakeCommentAddSubdirectory() expected error for missing args")
	}
}

// ============================================================================
// CmakeRunIn Tests
// ============================================================================

func TestCmakeRunIn_MissingArgs(t *testing.T) {
	h := NewHelpers(nil, nil, nil)

	tests := []struct {
		name string
		args []string
	}{
		{"no args", []string{}},
		{"only dir", []string{"/tmp"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.CmakeRunIn(tt.args)
			if err == nil {
				t.Errorf("CmakeRunIn() expected error for args: %v", tt.args)
			}
		})
	}
}

// ============================================================================
// CmakeBuildType Tests
// ============================================================================

func TestCmakeBuildType(t *testing.T) {
	tests := []struct {
		name      string
		buildType string
		want      string
	}{
		{"default Release", "", "Release"},
		{"explicit Debug", "Debug", "Debug"},
		{"explicit RelWithDebInfo", "RelWithDebInfo", "RelWithDebInfo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			p := &pkg.Package{
				Name:     "test/test",
				Version:  "1.0",
				UseFlags: make(map[string]bool),
			}
			env, _ := NewEnvironment(p, "/tmp", "/var/db/repos/gentoo", "/var/cache/distfiles")
			if tt.buildType != "" {
				env.SetVar("CMAKE_BUILD_TYPE", tt.buildType)
			}
			h := NewHelpers(env, &stdout, nil)

			err := h.CmakeBuildType(nil)
			if err != nil {
				t.Errorf("CmakeBuildType() error = %v", err)
				return
			}

			if stdout.String() != tt.want {
				t.Errorf("CmakeBuildType() = %q, want %q", stdout.String(), tt.want)
			}
		})
	}
}

// ============================================================================
// SetupCmakeEclass Tests
// ============================================================================

func TestSetupCmakeEclass(t *testing.T) {
	p := &pkg.Package{
		Name:     "test/test",
		Version:  "1.0",
		UseFlags: make(map[string]bool),
	}
	env, _ := NewEnvironment(p, "/tmp", "/var/db/repos/gentoo", "/var/cache/distfiles")
	h := NewHelpers(env, nil, nil)

	err := h.SetupCmakeEclass()
	if err != nil {
		t.Errorf("SetupCmakeEclass() error = %v", err)
		return
	}

	// Check default variables are set
	tests := []struct {
		key   string
		check func(string) bool
	}{
		{"CMAKE_MAKEFILE_GENERATOR", func(v string) bool { return v == "ninja" }},
		{"CMAKE_BUILD_TYPE", func(v string) bool { return v == "Release" }},
		{"CMAKE_WARN_UNUSED_CLI", func(v string) bool { return v == "yes" }},
		{"BUILD_DIR", func(v string) bool { return strings.Contains(v, "_build") }},
		{"CMAKE_USE_DIR", func(v string) bool { return v == env.S }},
	}

	for _, tt := range tests {
		val := env.GetVar(tt.key)
		if !tt.check(val) {
			t.Errorf("SetupCmakeEclass() %s = %q, check failed", tt.key, val)
		}
	}
}

func TestSetupCmakeEclass_PreservesExisting(t *testing.T) {
	p := &pkg.Package{
		Name:     "test/test",
		Version:  "1.0",
		UseFlags: make(map[string]bool),
	}
	env, _ := NewEnvironment(p, "/tmp", "/var/db/repos/gentoo", "/var/cache/distfiles")

	// Pre-set a value
	env.SetVar("CMAKE_BUILD_TYPE", "Debug")

	h := NewHelpers(env, nil, nil)
	err := h.SetupCmakeEclass()
	if err != nil {
		t.Errorf("SetupCmakeEclass() error = %v", err)
		return
	}

	// Check that pre-existing value is preserved
	if got := env.GetVar("CMAKE_BUILD_TYPE"); got != "Debug" {
		t.Errorf("SetupCmakeEclass() CMAKE_BUILD_TYPE = %q, want %q", got, "Debug")
	}
}

func TestSetupCmakeEclass_NilEnv(t *testing.T) {
	h := NewHelpers(nil, nil, nil)
	err := h.SetupCmakeEclass()
	if err != nil {
		t.Errorf("SetupCmakeEclass() with nil env should not error, got: %v", err)
	}
}

// ============================================================================
// CmakeSrcPrepare Tests
// ============================================================================

func TestCmakeSrcPrepare_RemovesModules(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "cmake-prepare-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create cmake/Modules directory with bundled modules
	modulesDir := filepath.Join(tmpDir, "cmake", "Modules")
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		t.Fatalf("Failed to create modules dir: %v", err)
	}

	// Create bundled modules
	modules := []string{"FindBLAS.cmake", "FindLAPACK.cmake", "FindZLIB.cmake"}
	for _, m := range modules {
		path := filepath.Join(modulesDir, m)
		if err := os.WriteFile(path, []byte("# Bundled module\n"), 0644); err != nil {
			t.Fatalf("Failed to create module %s: %v", m, err)
		}
	}

	// Create helpers
	var stdout, stderr bytes.Buffer
	p := &pkg.Package{
		Name:     "test/test",
		Version:  "1.0",
		UseFlags: make(map[string]bool),
	}
	env, _ := NewEnvironment(p, "/tmp", "/var/db/repos/gentoo", "/var/cache/distfiles")
	env.S = tmpDir
	h := NewHelpers(env, &stdout, &stderr)

	// Set CMAKE_REMOVE_MODULES_LIST
	env.SetVar("CMAKE_REMOVE_MODULES_LIST", "BLAS LAPACK")
	env.SetVar("CMAKE_USE_DIR", tmpDir)

	err = h.CmakeSrcPrepare(nil)
	if err != nil {
		t.Errorf("CmakeSrcPrepare() error = %v", err)
		return
	}

	// Check that BLAS and LAPACK modules were removed
	if _, err := os.Stat(filepath.Join(modulesDir, "FindBLAS.cmake")); !os.IsNotExist(err) {
		t.Error("CmakeSrcPrepare() should have removed FindBLAS.cmake")
	}
	if _, err := os.Stat(filepath.Join(modulesDir, "FindLAPACK.cmake")); !os.IsNotExist(err) {
		t.Error("CmakeSrcPrepare() should have removed FindLAPACK.cmake")
	}

	// Check that ZLIB module was NOT removed (not in list)
	if _, err := os.Stat(filepath.Join(modulesDir, "FindZLIB.cmake")); os.IsNotExist(err) {
		t.Error("CmakeSrcPrepare() should NOT have removed FindZLIB.cmake")
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestCmakeEclass_Integration(t *testing.T) {
	// Test that cmake functions can be called through the interpreter

	p := &pkg.Package{
		Name:     "test/cmake-test",
		Version:  "1.0",
		UseFlags: map[string]bool{"ssl": true, "zlib": false},
	}
	env, err := NewEnvironment(p, "/tmp", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	var stdout, stderr bytes.Buffer
	interp := NewInterpreter(env, &stdout, &stderr)

	// Test cmake_use command
	tests := []struct {
		script string
		want   string
	}{
		{`cmake_use ssl`, "-DSSL=ON"},
		{`cmake_use zlib`, "-DZLIB=OFF"},
		{`cmake_use ssl ENABLE_SSL`, "-DENABLE_SSL=ON"},
		{`cmake_use_find_package ssl OpenSSL`, "-DCMAKE_DISABLE_FIND_PACKAGE_OpenSSL=OFF"},
		{`cmake_use_find_package zlib ZLIB`, "-DCMAKE_DISABLE_FIND_PACKAGE_ZLIB=ON"},
	}

	for _, tt := range tests {
		t.Run(tt.script, func(t *testing.T) {
			stdout.Reset()
			err := interp.Run(t.Context(), tt.script)
			if err != nil {
				t.Errorf("Run(%q) error = %v", tt.script, err)
				return
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("Run(%q) output = %q, want %q", tt.script, got, tt.want)
			}
		})
	}
}

// ============================================================================
// CmakeRemoveModulesFromList Tests
// ============================================================================

func TestCmakeRemoveModulesFromList(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "cmake-list-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	content := `set(MODULES
    FindBLAS
    FindLAPACK
    FindZLIB
    FindPNG
)`
	if err := os.WriteFile(tmpFile.Name(), []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	h := NewHelpers(nil, nil, nil)
	err = h.CmakeRemoveModulesFromList(tmpFile.Name(), []string{"BLAS", "LAPACK"})
	if err != nil {
		t.Errorf("CmakeRemoveModulesFromList() error = %v", err)
		return
	}

	// Read back
	got, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read temp file: %v", err)
	}

	if strings.Contains(string(got), "BLAS") {
		t.Error("CmakeRemoveModulesFromList() should have removed BLAS")
	}
	if strings.Contains(string(got), "LAPACK") {
		t.Error("CmakeRemoveModulesFromList() should have removed LAPACK")
	}
	if !strings.Contains(string(got), "ZLIB") {
		t.Error("CmakeRemoveModulesFromList() should NOT have removed ZLIB")
	}
}

func TestCmakeRemoveModulesFromList_NonexistentFile(t *testing.T) {
	h := NewHelpers(nil, nil, nil)
	err := h.CmakeRemoveModulesFromList("/nonexistent/file.txt", []string{"BLAS"})
	if err != nil {
		t.Errorf("CmakeRemoveModulesFromList() should not error for nonexistent file, got: %v", err)
	}
}
