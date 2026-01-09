package ebuild

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	pkgdomain "github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/state"
)

// TestEclassRegistry tests the EclassRegistry functionality.
func TestEclassRegistry(t *testing.T) {
	t.Run("NewEclassRegistry", func(t *testing.T) {
		registry := NewEclassRegistry("/var/db/repos/gentoo")

		if registry == nil {
			t.Fatal("NewEclassRegistry returned nil")
		}

		if len(registry.eclassLocations) == 0 {
			t.Error("eclassLocations should not be empty")
		}

		// Use filepath.Join for cross-platform compatibility
		expected := filepath.Join("/var/db/repos/gentoo", "eclass")
		if registry.eclassLocations[0] != expected {
			t.Errorf("first location should be %s, got %s", expected, registry.eclassLocations[0])
		}
	})

	t.Run("IsLoaded and MarkLoaded", func(t *testing.T) {
		registry := NewEclassRegistry("")

		if registry.IsLoaded("eutils") {
			t.Error("eutils should not be loaded initially")
		}

		registry.MarkLoaded("eutils", "/path/to/eutils.eclass")

		if !registry.IsLoaded("eutils") {
			t.Error("eutils should be loaded after MarkLoaded")
		}

		// Loading same eclass again should not duplicate in INHERITED
		registry.MarkLoaded("eutils", "/path/to/eutils.eclass")
		inherited := registry.GetInherited()
		if inherited != "eutils" {
			t.Errorf("INHERITED should be 'eutils', got '%s'", inherited)
		}
	})

	t.Run("GetInherited", func(t *testing.T) {
		registry := NewEclassRegistry("")

		registry.MarkLoaded("eutils", "/path/eutils.eclass")
		registry.MarkLoaded("toolchain-funcs", "/path/toolchain-funcs.eclass")

		inherited := registry.GetInherited()
		expected := "eutils toolchain-funcs"
		if inherited != expected {
			t.Errorf("INHERITED expected '%s', got '%s'", expected, inherited)
		}
	})

	t.Run("RegisterFunction and GetFunctions", func(t *testing.T) {
		registry := NewEclassRegistry("")

		registry.RegisterFunction("eutils", "epatch")
		registry.RegisterFunction("eutils", "eshopts_push")

		funcs := registry.GetFunctions("eutils")
		if len(funcs) != 2 {
			t.Errorf("expected 2 functions, got %d", len(funcs))
		}
	})

	t.Run("ExportFunction", func(t *testing.T) {
		registry := NewEclassRegistry("")

		// Without current eclass, should fail
		err := registry.ExportFunction("src_compile")
		if err == nil {
			t.Error("ExportFunction without current eclass should fail")
		}

		// Set current eclass
		registry.SetCurrentEclass("myeclass")
		err = registry.ExportFunction("src_compile")
		if err != nil {
			t.Errorf("ExportFunction should succeed: %v", err)
		}

		// Check exported function
		eclass, ok := registry.GetExportedFunction("src_compile")
		if !ok {
			t.Error("src_compile should be exported")
		}
		if eclass != "myeclass" {
			t.Errorf("expected myeclass, got %s", eclass)
		}
	})

	t.Run("IncrementDecrementDepth", func(t *testing.T) {
		registry := NewEclassRegistry("")

		depth := registry.IncrementDepth()
		if depth != 1 {
			t.Errorf("depth should be 1, got %d", depth)
		}

		depth = registry.IncrementDepth()
		if depth != 2 {
			t.Errorf("depth should be 2, got %d", depth)
		}

		depth = registry.DecrementDepth()
		if depth != 1 {
			t.Errorf("depth should be 1, got %d", depth)
		}
	})
}

// TestEclassStack tests the EclassStack functionality.
func TestEclassStack(t *testing.T) {
	t.Run("Push and Pop", func(t *testing.T) {
		stack := NewEclassStack()

		stack.Push("eshopts", "value1")
		stack.Push("eshopts", "value2")

		if stack.Len("eshopts") != 2 {
			t.Errorf("stack length should be 2, got %d", stack.Len("eshopts"))
		}

		value, ok := stack.Pop("eshopts")
		if !ok {
			t.Error("Pop should succeed")
		}
		if value != "value2" {
			t.Errorf("expected 'value2', got '%s'", value)
		}

		value, ok = stack.Pop("eshopts")
		if !ok {
			t.Error("Pop should succeed")
		}
		if value != "value1" {
			t.Errorf("expected 'value1', got '%s'", value)
		}

		_, ok = stack.Pop("eshopts")
		if ok {
			t.Error("Pop on empty stack should fail")
		}
	})

	t.Run("Multiple stacks", func(t *testing.T) {
		stack := NewEclassStack()

		stack.Push("stack1", "a")
		stack.Push("stack2", "b")

		if stack.Len("stack1") != 1 {
			t.Error("stack1 length should be 1")
		}
		if stack.Len("stack2") != 1 {
			t.Error("stack2 length should be 1")
		}

		val1, _ := stack.Pop("stack1")
		val2, _ := stack.Pop("stack2")

		if val1 != "a" || val2 != "b" {
			t.Error("values should be preserved per stack")
		}
	})
}

// TestEclassLoader tests the EclassLoader functionality.
func TestEclassLoader(t *testing.T) {
	t.Run("BuiltinEclass handling", func(t *testing.T) {
		pkg := &pkgdomain.Package{
			Name:     "app-misc/test",
			Version:  "1.0",
			UseFlags: map[string]bool{"ssl": true},
		}

		env, err := NewEnvironment(pkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
		if err != nil {
			t.Fatalf("NewEnvironment failed: %v", err)
		}

		var stdout, stderr bytes.Buffer
		interp := NewInterpreter(env, &stdout, &stderr)
		registry := NewEclassRegistry("")
		loader := NewEclassLoader(registry, interp)

		// Test that builtin eclasses are handled without file access
		// toolchain-funcs is a builtin - handleBuiltinEclass returns true
		// but doesn't call MarkLoaded (that's done in loadEclass)
		handled := loader.handleBuiltinEclass("toolchain-funcs")
		if !handled {
			t.Error("toolchain-funcs should be a builtin eclass")
		}
	})
}

// TestEutilsFunctions tests eutils eclass functions.
func TestEutilsFunctions(t *testing.T) {
	pkg := &pkgdomain.Package{
		Name:     "app-misc/test",
		Version:  "1.0",
		UseFlags: map[string]bool{"ssl": true},
	}

	env, err := NewEnvironment(pkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment failed: %v", err)
	}

	t.Run("eshopts_push and eshopts_pop", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		// Push without options
		err := helpers.EshoptsPush(nil)
		if err != nil {
			t.Errorf("EshoptsPush failed: %v", err)
		}

		// Push with options
		err = helpers.EshoptsPush([]string{"-s", "nullglob"})
		if err != nil {
			t.Errorf("EshoptsPush with options failed: %v", err)
		}

		// Pop should succeed
		err = helpers.EshoptsPop(nil)
		if err != nil {
			t.Errorf("EshoptsPop failed: %v", err)
		}

		err = helpers.EshoptsPop(nil)
		if err != nil {
			t.Errorf("EshoptsPop failed: %v", err)
		}

		// Pop on empty stack should fail
		err = helpers.EshoptsPop(nil)
		if err == nil {
			t.Error("EshoptsPop on empty stack should fail")
		}
	})

	t.Run("estack_push and estack_pop", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		err := helpers.EstackPush([]string{"mystack", "value1"})
		if err != nil {
			t.Errorf("EstackPush failed: %v", err)
		}

		err = helpers.EstackPush([]string{"mystack", "value2"})
		if err != nil {
			t.Errorf("EstackPush failed: %v", err)
		}

		// Pop with variable capture
		stdout.Reset()
		err = helpers.EstackPop([]string{"mystack", "VAR"})
		if err != nil {
			t.Errorf("EstackPop failed: %v", err)
		}
		if stdout.String() != "value2" {
			t.Errorf("expected 'value2', got '%s'", stdout.String())
		}

		// Pop without variable
		err = helpers.EstackPop([]string{"mystack"})
		if err != nil {
			t.Errorf("EstackPop failed: %v", err)
		}

		// Pop on empty stack should fail silently (exit code 1)
		err = helpers.EstackPop([]string{"mystack"})
		if err == nil {
			t.Error("EstackPop on empty stack should return error")
		}
	})

	t.Run("epatch", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		// Create temporary patch file
		tmpDir := t.TempDir()
		patchFile := filepath.Join(tmpDir, "test.patch")
		if err := os.WriteFile(patchFile, []byte{}, 0644); err != nil {
			t.Fatalf("failed to create patch file: %v", err)
		}

		// epatch should delegate to eapply
		err := helpers.Epatch([]string{patchFile})
		// May fail due to patch not being a real patch, but shouldn't panic
		_ = err

		if !bytes.Contains(stdout.Bytes(), []byte("deprecated")) {
			t.Error("epatch should warn about deprecation")
		}
	})
}

// TestToolchainFuncs tests toolchain-funcs eclass functions.
func TestToolchainFuncs(t *testing.T) {
	pkg := &pkgdomain.Package{
		Name:    "app-misc/test",
		Version: "1.0",
	}

	env, err := NewEnvironment(pkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment failed: %v", err)
	}

	t.Run("tc-is-gcc", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		// Default CC is gcc
		err := helpers.TcIsGcc(nil)
		if err != nil {
			t.Error("tc-is-gcc should return true for default gcc")
		}
	})

	t.Run("tc-is-clang", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		// Default CC is gcc, not clang
		err := helpers.TcIsClang(nil)
		if err == nil {
			t.Error("tc-is-clang should return false for default gcc")
		}
	})

	t.Run("tc-export", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		err := helpers.TcExport([]string{"CC", "CXX"})
		if err != nil {
			t.Errorf("tc-export failed: %v", err)
		}

		output := stdout.String()
		if !bytes.Contains([]byte(output), []byte("export CC=")) {
			t.Error("tc-export should output export CC=...")
		}
		if !bytes.Contains([]byte(output), []byte("export CXX=")) {
			t.Error("tc-export should output export CXX=...")
		}
	})

	t.Run("tc-getAR and friends", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		testCases := []struct {
			name     string
			fn       func([]string) error
			expected string
		}{
			{"tc-getAR", helpers.TcGetAR, "ar"},
			{"tc-getRANLIB", helpers.TcGetRANLIB, "ranlib"},
			{"tc-getNM", helpers.TcGetNM, "nm"},
			{"tc-getSTRIP", helpers.TcGetSTRIP, "strip"},
			{"tc-getOBJCOPY", helpers.TcGetOBJCOPY, "objcopy"},
			{"tc-getBUILD_CC", helpers.TcGetBUILD_CC, "gcc"},
		}

		for _, tc := range testCases {
			stdout.Reset()
			err := tc.fn(nil)
			if err != nil {
				t.Errorf("%s failed: %v", tc.name, err)
			}
			if stdout.String() != tc.expected {
				t.Errorf("%s: expected '%s', got '%s'", tc.name, tc.expected, stdout.String())
			}
		}
	})
}

// TestMultilibFunctions tests multilib eclass functions.
func TestMultilibFunctions(t *testing.T) {
	pkg := &pkgdomain.Package{
		Name:     "app-misc/test",
		Version:  "1.0",
		UseFlags: map[string]bool{"ssl": true, "zlib": false},
	}

	env, err := NewEnvironment(pkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment failed: %v", err)
	}

	t.Run("get_libdir", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		err := helpers.GetLibdir(nil)
		if err != nil {
			t.Errorf("get_libdir failed: %v", err)
		}

		libdir := stdout.String()
		if libdir != "lib" && libdir != "lib64" {
			t.Errorf("get_libdir should return 'lib' or 'lib64', got '%s'", libdir)
		}
	})

	t.Run("multilib_native_use_with", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		// ssl is enabled
		err := helpers.MultilibNativeUseWith([]string{"ssl"})
		if err != nil {
			t.Errorf("multilib_native_use_with failed: %v", err)
		}
		if stdout.String() != "--with-ssl" {
			t.Errorf("expected '--with-ssl', got '%s'", stdout.String())
		}

		// zlib is disabled
		stdout.Reset()
		err = helpers.MultilibNativeUseWith([]string{"zlib"})
		if err != nil {
			t.Errorf("multilib_native_use_with failed: %v", err)
		}
		if stdout.String() != "--without-zlib" {
			t.Errorf("expected '--without-zlib', got '%s'", stdout.String())
		}
	})

	t.Run("multilib_native_use_enable", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		// ssl is enabled
		err := helpers.MultilibNativeUseEnable([]string{"ssl"})
		if err != nil {
			t.Errorf("multilib_native_use_enable failed: %v", err)
		}
		if stdout.String() != "--enable-ssl" {
			t.Errorf("expected '--enable-ssl', got '%s'", stdout.String())
		}

		// zlib is disabled
		stdout.Reset()
		err = helpers.MultilibNativeUseEnable([]string{"zlib"})
		if err != nil {
			t.Errorf("multilib_native_use_enable failed: %v", err)
		}
		if stdout.String() != "--disable-zlib" {
			t.Errorf("expected '--disable-zlib', got '%s'", stdout.String())
		}
	})
}

// TestFlagOMatic tests flag-o-matic eclass functions.
func TestFlagOMatic(t *testing.T) {
	pkg := &pkgdomain.Package{
		Name:    "app-misc/test",
		Version: "1.0",
	}

	env, err := NewEnvironment(pkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment failed: %v", err)
	}

	t.Run("append-cflags", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		env.CFLAGS = "-O2"
		err := helpers.AppendCflags([]string{"-fPIC", "-Wall"})
		if err != nil {
			t.Errorf("append-cflags failed: %v", err)
		}

		if env.CFLAGS != "-O2 -fPIC -Wall" {
			t.Errorf("CFLAGS expected '-O2 -fPIC -Wall', got '%s'", env.CFLAGS)
		}
	})

	t.Run("append-ldflags", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		env.LDFLAGS = ""
		err := helpers.AppendLdflags([]string{"-Wl,-rpath,/usr/lib"})
		if err != nil {
			t.Errorf("append-ldflags failed: %v", err)
		}

		if env.LDFLAGS != "-Wl,-rpath,/usr/lib" {
			t.Errorf("LDFLAGS expected '-Wl,-rpath,/usr/lib', got '%s'", env.LDFLAGS)
		}
	})

	t.Run("filter-flags", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		env.CFLAGS = "-O2 -march=native -fPIC"
		env.CXXFLAGS = "-O2 -march=native -fPIC"
		err := helpers.FilterFlags([]string{"-O*", "-march=*"})
		if err != nil {
			t.Errorf("filter-flags failed: %v", err)
		}

		if env.CFLAGS != "-fPIC" {
			t.Errorf("CFLAGS expected '-fPIC', got '%s'", env.CFLAGS)
		}
	})

	t.Run("strip-flags", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		env.CFLAGS = "-O3 -march=native -mtune=native -fPIC"
		env.CXXFLAGS = "-O3 -march=native -mtune=native -fPIC"
		err := helpers.StripFlags(nil)
		if err != nil {
			t.Errorf("strip-flags failed: %v", err)
		}

		if env.CFLAGS != "-fPIC" {
			t.Errorf("CFLAGS expected '-fPIC', got '%s'", env.CFLAGS)
		}
	})

	t.Run("matchFlagPattern", func(t *testing.T) {
		testCases := []struct {
			flag    string
			pattern string
			match   bool
		}{
			{"-O2", "-O*", true},
			{"-O3", "-O*", true},
			{"-march=native", "-march=*", true},
			{"-fPIC", "-O*", false},
			{"-fPIC", "-fPIC", true},
			{"-fPIC", "-fPIE", false},
		}

		for _, tc := range testCases {
			result := matchFlagPattern(tc.flag, tc.pattern)
			if result != tc.match {
				t.Errorf("matchFlagPattern(%q, %q) = %v, want %v", tc.flag, tc.pattern, result, tc.match)
			}
		}
	})
}

// TestLinuxInfoFunctions tests linux-info eclass functions.
func TestLinuxInfoFunctions(t *testing.T) {
	pkg := &pkgdomain.Package{
		Name:    "app-misc/test",
		Version: "1.0",
	}

	env, err := NewEnvironment(pkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment failed: %v", err)
	}

	t.Run("get_version", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		err := helpers.GetVersion(nil)
		if err != nil {
			t.Errorf("get_version failed: %v", err)
		}

		// Should output something (either kernel version or "unknown")
		output := stdout.String()
		if output == "" {
			t.Error("get_version should output something")
		}
	})

	t.Run("linux_config_exists", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		// This will likely fail since we don't have a real kernel config
		err := helpers.LinuxConfigExists([]string{"CONFIG_MODULES"})
		// Error is expected in test environment
		_ = err
	})
}

// TestExportFunctions tests the EXPORT_FUNCTIONS helper.
func TestExportFunctions(t *testing.T) {
	pkg := &pkgdomain.Package{
		Name:    "app-misc/test",
		Version: "1.0",
	}

	env, err := NewEnvironment(pkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment failed: %v", err)
	}

	t.Run("EXPORT_FUNCTIONS without eclass", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		err := helpers.ExportFunctions([]string{"src_compile"})
		if err == nil {
			t.Error("EXPORT_FUNCTIONS without current eclass should fail")
		}
	})

	t.Run("EXPORT_FUNCTIONS with eclass", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		// Set current eclass
		helpers.eclassRegistry.SetCurrentEclass("myeclass")

		err := helpers.ExportFunctions([]string{"src_compile", "src_install"})
		if err != nil {
			t.Errorf("EXPORT_FUNCTIONS should succeed: %v", err)
		}

		// Check that functions are exported
		eclass, ok := helpers.eclassRegistry.GetExportedFunction("src_compile")
		if !ok {
			t.Error("src_compile should be exported")
		}
		if eclass != "myeclass" {
			t.Errorf("expected myeclass, got %s", eclass)
		}
	})
}

// TestMiscEclassFunctions tests miscellaneous eclass functions.
func TestMiscEclassFunctions(t *testing.T) {
	pkg := &pkgdomain.Package{
		Name:    "app-misc/test",
		Version: "1.0",
	}

	env, err := NewEnvironment(pkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment failed: %v", err)
	}

	t.Run("eqawarn", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		err := helpers.Eqawarn([]string{"something is wrong"})
		if err != nil {
			t.Errorf("eqawarn failed: %v", err)
		}

		if !bytes.Contains(stderr.Bytes(), []byte("QA Notice")) {
			t.Error("eqawarn should output QA Notice")
		}
	})

	t.Run("has_version without database", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		// Without database, should return not found (exit code 1)
		err := helpers.HasVersion([]string{">=sys-libs/zlib-1.2"})
		if err == nil {
			t.Error("has_version without database should return not found")
		}
	})

	t.Run("has_version with database - package found", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		// Create mock package database
		db := state.NewPackageDatabase("/var/db/pkg")
		installedPkg := &state.InstalledPackage{
			Package: &pkgdomain.Package{
				Name:    "sys-libs/zlib",
				Version: "1.2.13",
			},
		}
		if err := db.Add(installedPkg); err != nil {
			t.Fatalf("failed to add package: %v", err)
		}
		helpers.SetPackageDatabase(db)

		// Should find installed package
		err := helpers.HasVersion([]string{"sys-libs/zlib"})
		if err != nil {
			t.Error("has_version should find installed package")
		}
	})

	t.Run("has_version with database - version constraint", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		// Create mock package database
		db := state.NewPackageDatabase("/var/db/pkg")
		installedPkg := &state.InstalledPackage{
			Package: &pkgdomain.Package{
				Name:    "sys-libs/zlib",
				Version: "1.2.13",
			},
		}
		if err := db.Add(installedPkg); err != nil {
			t.Fatalf("failed to add package: %v", err)
		}
		helpers.SetPackageDatabase(db)

		// Should find package with >=1.2 constraint
		err := helpers.HasVersion([]string{">=sys-libs/zlib-1.2"})
		if err != nil {
			t.Error("has_version should find package with >=1.2 constraint")
		}

		// Should NOT find package with >=2.0 constraint
		err = helpers.HasVersion([]string{">=sys-libs/zlib-2.0"})
		if err == nil {
			t.Error("has_version should NOT find package with >=2.0 constraint")
		}
	})

	t.Run("has_version with database - package not found", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		// Create empty database
		db := state.NewPackageDatabase("/var/db/pkg")
		helpers.SetPackageDatabase(db)

		// Should NOT find missing package
		err := helpers.HasVersion([]string{"sys-libs/nonexistent"})
		if err == nil {
			t.Error("has_version should NOT find missing package")
		}
	})

	t.Run("best_version without database", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		// Without database, should return nothing (no error, no output)
		err := helpers.BestVersion([]string{"sys-libs/zlib"})
		if err != nil {
			t.Errorf("best_version without database should not fail: %v", err)
		}
		if stdout.Len() > 0 {
			t.Error("best_version without database should not produce output")
		}
	})

	t.Run("best_version with database - single version", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		// Create mock package database
		db := state.NewPackageDatabase("/var/db/pkg")
		installedPkg := &state.InstalledPackage{
			Package: &pkgdomain.Package{
				Name:    "sys-libs/zlib",
				Version: "1.2.13",
			},
		}
		if err := db.Add(installedPkg); err != nil {
			t.Fatalf("failed to add package: %v", err)
		}
		helpers.SetPackageDatabase(db)

		err := helpers.BestVersion([]string{"sys-libs/zlib"})
		if err != nil {
			t.Errorf("best_version should not fail: %v", err)
		}

		output := stdout.String()
		if output != "sys-libs/zlib-1.2.13" {
			t.Errorf("best_version output = %q, want %q", output, "sys-libs/zlib-1.2.13")
		}
	})

	t.Run("best_version with database - multiple versions", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		// Create mock package database with multiple versions
		db := state.NewPackageDatabase("/var/db/pkg")

		// Add older version
		oldPkg := &state.InstalledPackage{
			Package: &pkgdomain.Package{
				Name:    "sys-libs/zlib",
				Version: "1.2.11",
			},
		}
		if err := db.Add(oldPkg); err != nil {
			t.Fatalf("failed to add old package: %v", err)
		}

		// Add newer version (overwrites due to same atom key)
		newPkg := &state.InstalledPackage{
			Package: &pkgdomain.Package{
				Name:    "sys-libs/zlib",
				Version: "1.2.13",
			},
		}
		if err := db.Add(newPkg); err != nil {
			t.Fatalf("failed to add new package: %v", err)
		}

		helpers.SetPackageDatabase(db)

		err := helpers.BestVersion([]string{"sys-libs/zlib"})
		if err != nil {
			t.Errorf("best_version should not fail: %v", err)
		}

		output := stdout.String()
		// Should return the highest version
		if output != "sys-libs/zlib-1.2.13" {
			t.Errorf("best_version output = %q, want %q", output, "sys-libs/zlib-1.2.13")
		}
	})

	t.Run("best_version with database - package not found", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		helpers := NewHelpers(env, &stdout, &stderr)

		// Create empty database
		db := state.NewPackageDatabase("/var/db/pkg")
		helpers.SetPackageDatabase(db)

		err := helpers.BestVersion([]string{"sys-libs/nonexistent"})
		if err != nil {
			t.Errorf("best_version should not fail even for missing package: %v", err)
		}
		if stdout.Len() > 0 {
			t.Error("best_version should not produce output for missing package")
		}
	})
}

// TestInterpreterEclassIntegration tests eclass functions through the interpreter.
func TestInterpreterEclassIntegration(t *testing.T) {
	pkg := &pkgdomain.Package{
		Name:     "app-misc/test",
		Version:  "1.0",
		UseFlags: map[string]bool{"ssl": true},
	}

	env, err := NewEnvironment(pkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment failed: %v", err)
	}

	t.Run("tc-is-gcc in script", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		interp := NewInterpreter(env, &stdout, &stderr)

		// Script using tc-is-gcc
		script := `
if tc-is-gcc; then
	einfo "Using GCC"
else
	einfo "Not using GCC"
fi
`
		err := interp.Run(context.Background(), script)
		if err != nil {
			t.Errorf("script execution failed: %v", err)
		}

		if !bytes.Contains(stdout.Bytes(), []byte("Using GCC")) {
			t.Error("script should detect GCC")
		}
	})

	t.Run("append-flags in script", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		env.CFLAGS = "-O2"
		env.CXXFLAGS = "-O2"
		interp := NewInterpreter(env, &stdout, &stderr)

		script := `append-flags -fPIC`
		err := interp.Run(context.Background(), script)
		if err != nil {
			t.Errorf("script execution failed: %v", err)
		}

		// Flags should be appended
		helpers := interp.GetHelpers()
		if len(helpers.cflags) == 0 {
			t.Error("flags should be appended")
		}
	})

	t.Run("get_libdir in script", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		interp := NewInterpreter(env, &stdout, &stderr)

		script := `libdir=$(get_libdir); echo $libdir`
		err := interp.Run(context.Background(), script)
		if err != nil {
			t.Errorf("script execution failed: %v", err)
		}

		output := stdout.String()
		if output != "lib" && output != "lib64" && !bytes.Contains([]byte(output), []byte("lib")) {
			t.Errorf("get_libdir should return lib or lib64, got '%s'", output)
		}
	})
}

// TestParseAtom tests the parseAtom function.
func TestParseAtom(t *testing.T) {
	tests := []struct {
		name         string
		atom         string
		wantOperator string
		wantName     string
		wantVersion  string
	}{
		{
			name:         "simple package name",
			atom:         "sys-libs/zlib",
			wantOperator: "",
			wantName:     "sys-libs/zlib",
			wantVersion:  "",
		},
		{
			name:         "package with exact version",
			atom:         "=sys-libs/zlib-1.2.13",
			wantOperator: "=",
			wantName:     "sys-libs/zlib",
			wantVersion:  "1.2.13",
		},
		{
			name:         "package with >= constraint",
			atom:         ">=sys-libs/zlib-1.2",
			wantOperator: ">=",
			wantName:     "sys-libs/zlib",
			wantVersion:  "1.2",
		},
		{
			name:         "package with <= constraint",
			atom:         "<=sys-libs/zlib-1.3",
			wantOperator: "<=",
			wantName:     "sys-libs/zlib",
			wantVersion:  "1.3",
		},
		{
			name:         "package with > constraint",
			atom:         ">sys-libs/zlib-1.2",
			wantOperator: ">",
			wantName:     "sys-libs/zlib",
			wantVersion:  "1.2",
		},
		{
			name:         "package with < constraint",
			atom:         "<sys-libs/zlib-2.0",
			wantOperator: "<",
			wantName:     "sys-libs/zlib",
			wantVersion:  "2.0",
		},
		{
			name:         "package with slot",
			atom:         "sys-libs/zlib:0",
			wantOperator: "",
			wantName:     "sys-libs/zlib",
			wantVersion:  "",
		},
		{
			name:         "package with USE flag",
			atom:         "sys-libs/zlib[static-libs]",
			wantOperator: "",
			wantName:     "sys-libs/zlib",
			wantVersion:  "",
		},
		{
			name:         "complex atom with version, slot, and USE",
			atom:         ">=dev-lang/python-3.10:3.10[ssl]",
			wantOperator: ">=",
			wantName:     "dev-lang/python",
			wantVersion:  "3.10",
		},
		{
			name:         "app-misc package",
			atom:         "app-misc/hello",
			wantOperator: "",
			wantName:     "app-misc/hello",
			wantVersion:  "",
		},
		{
			name:         "package with revision",
			atom:         "=sys-libs/zlib-1.2.13-r1",
			wantOperator: "=",
			wantName:     "sys-libs/zlib",
			wantVersion:  "1.2.13-r1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOperator, gotName, gotVersion := parseAtom(tt.atom)

			if gotOperator != tt.wantOperator {
				t.Errorf("parseAtom(%q) operator = %q, want %q", tt.atom, gotOperator, tt.wantOperator)
			}
			if gotName != tt.wantName {
				t.Errorf("parseAtom(%q) name = %q, want %q", tt.atom, gotName, tt.wantName)
			}
			if gotVersion != tt.wantVersion {
				t.Errorf("parseAtom(%q) version = %q, want %q", tt.atom, gotVersion, tt.wantVersion)
			}
		})
	}
}

// TestInheritRealImplementation tests the real inherit functionality.
func TestInheritRealImplementation(t *testing.T) {
	t.Run("inherit with no eclassLoader falls back gracefully", func(t *testing.T) {
		pkg := &pkgdomain.Package{
			Name:    "app-misc/test",
			Version: "1.0",
		}

		env, err := NewEnvironment(pkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
		if err != nil {
			t.Fatalf("NewEnvironment failed: %v", err)
		}

		var stdout, stderr bytes.Buffer
		// Create helpers directly without going through interpreter
		// This simulates the case where eclassLoader is not wired up
		helpers := NewHelpers(env, &stdout, &stderr)

		// Inherit should succeed but use fallback behavior
		err = helpers.Inherit([]string{"toolchain-funcs"})
		if err != nil {
			t.Errorf("Inherit should not fail: %v", err)
		}

		// Should output fallback message
		output := stdout.String()
		if !bytes.Contains([]byte(output), []byte("no loader")) {
			t.Errorf("expected fallback message, got: %s", output)
		}
	})

	t.Run("inherit with eclassLoader uses real implementation", func(t *testing.T) {
		pkg := &pkgdomain.Package{
			Name:    "app-misc/test",
			Version: "1.0",
		}

		env, err := NewEnvironment(pkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
		if err != nil {
			t.Fatalf("NewEnvironment failed: %v", err)
		}

		var stdout, stderr bytes.Buffer
		// Create interpreter which wires up the eclassLoader
		interp := NewInterpreter(env, &stdout, &stderr)
		helpers := interp.GetHelpers()

		// Verify eclassLoader is wired up
		if helpers.GetEclassLoader() == nil {
			t.Fatal("eclassLoader should be set when using Interpreter")
		}

		// Inherit a builtin eclass (toolchain-funcs)
		err = helpers.Inherit([]string{"toolchain-funcs"})
		if err != nil {
			t.Errorf("Inherit should succeed for builtin eclass: %v", err)
		}

		// Check that INHERITED is populated
		inherited := helpers.GetEclassRegistry().GetInherited()
		if inherited != "toolchain-funcs" {
			t.Errorf("INHERITED should be 'toolchain-funcs', got '%s'", inherited)
		}
	})

	t.Run("inherit multiple builtin eclasses", func(t *testing.T) {
		pkg := &pkgdomain.Package{
			Name:    "app-misc/test",
			Version: "1.0",
		}

		env, err := NewEnvironment(pkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
		if err != nil {
			t.Fatalf("NewEnvironment failed: %v", err)
		}

		var stdout, stderr bytes.Buffer
		interp := NewInterpreter(env, &stdout, &stderr)
		helpers := interp.GetHelpers()

		// Inherit multiple builtin eclasses
		err = helpers.Inherit([]string{"toolchain-funcs", "eutils", "multilib"})
		if err != nil {
			t.Errorf("Inherit should succeed: %v", err)
		}

		inherited := helpers.GetEclassRegistry().GetInherited()
		if inherited != "toolchain-funcs eutils multilib" {
			t.Errorf("INHERITED should be 'toolchain-funcs eutils multilib', got '%s'", inherited)
		}
	})

	t.Run("inherit prevents double loading", func(t *testing.T) {
		pkg := &pkgdomain.Package{
			Name:    "app-misc/test",
			Version: "1.0",
		}

		env, err := NewEnvironment(pkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
		if err != nil {
			t.Fatalf("NewEnvironment failed: %v", err)
		}

		var stdout, stderr bytes.Buffer
		interp := NewInterpreter(env, &stdout, &stderr)
		helpers := interp.GetHelpers()

		// Inherit same eclass twice
		err = helpers.Inherit([]string{"toolchain-funcs"})
		if err != nil {
			t.Fatalf("First inherit failed: %v", err)
		}

		stdout.Reset()
		err = helpers.Inherit([]string{"toolchain-funcs"})
		if err != nil {
			t.Fatalf("Second inherit failed: %v", err)
		}

		// Should only appear once in INHERITED
		inherited := helpers.GetEclassRegistry().GetInherited()
		if inherited != "toolchain-funcs" {
			t.Errorf("INHERITED should be 'toolchain-funcs' (no duplicates), got '%s'", inherited)
		}

		// Output should mention already inherited
		output := stdout.String()
		if !bytes.Contains([]byte(output), []byte("already inherited")) {
			t.Logf("Note: output was '%s'", output)
		}
	})

	t.Run("inherit empty args does nothing", func(t *testing.T) {
		pkg := &pkgdomain.Package{
			Name:    "app-misc/test",
			Version: "1.0",
		}

		env, err := NewEnvironment(pkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
		if err != nil {
			t.Fatalf("NewEnvironment failed: %v", err)
		}

		var stdout, stderr bytes.Buffer
		interp := NewInterpreter(env, &stdout, &stderr)
		helpers := interp.GetHelpers()

		err = helpers.Inherit([]string{})
		if err != nil {
			t.Errorf("Inherit with empty args should succeed: %v", err)
		}

		inherited := helpers.GetEclassRegistry().GetInherited()
		if inherited != "" {
			t.Errorf("INHERITED should be empty, got '%s'", inherited)
		}
	})
}

// TestInheritWithRealEclassFile tests inherit with actual eclass files.
func TestInheritWithRealEclassFile(t *testing.T) {
	t.Run("inherit loads real eclass file", func(t *testing.T) {
		// Create a temporary directory for test eclasses
		tmpDir := t.TempDir()
		eclassDir := filepath.Join(tmpDir, "eclass")
		if err := os.MkdirAll(eclassDir, 0755); err != nil {
			t.Fatalf("failed to create eclass dir: %v", err)
		}

		// Create a simple test eclass file
		testEclassContent := `# test.eclass - Test eclass for GRPM
# This defines a simple function and variable

TEST_ECLASS_VAR="loaded"

test_eclass_func() {
	einfo "test_eclass_func called"
}
`
		eclassPath := filepath.Join(eclassDir, "test.eclass")
		if err := os.WriteFile(eclassPath, []byte(testEclassContent), 0644); err != nil {
			t.Fatalf("failed to write test eclass: %v", err)
		}

		// Create environment pointing to our test repo
		pkg := &pkgdomain.Package{
			Name:    "app-misc/test",
			Version: "1.0",
		}

		env, err := NewEnvironment(pkg, "/tmp/portage", tmpDir, "/var/cache/distfiles")
		if err != nil {
			t.Fatalf("NewEnvironment failed: %v", err)
		}

		var stdout, stderr bytes.Buffer
		interp := NewInterpreter(env, &stdout, &stderr)
		helpers := interp.GetHelpers()

		// Inherit the test eclass
		err = helpers.Inherit([]string{"test"})
		if err != nil {
			t.Errorf("Inherit should succeed: %v", err)
		}

		// Check INHERITED is populated
		inherited := helpers.GetEclassRegistry().GetInherited()
		if inherited != "test" {
			t.Errorf("INHERITED should be 'test', got '%s'", inherited)
		}
	})

	t.Run("inherit with nested inheritance", func(t *testing.T) {
		// Create a temporary directory for test eclasses
		tmpDir := t.TempDir()
		eclassDir := filepath.Join(tmpDir, "eclass")
		if err := os.MkdirAll(eclassDir, 0755); err != nil {
			t.Fatalf("failed to create eclass dir: %v", err)
		}

		// Create base eclass
		baseEclassContent := `# base.eclass - Base eclass
BASE_VAR="base"
base_func() {
	einfo "base_func"
}
`
		if err := os.WriteFile(filepath.Join(eclassDir, "base.eclass"), []byte(baseEclassContent), 0644); err != nil {
			t.Fatalf("failed to write base eclass: %v", err)
		}

		// Create child eclass that inherits base
		childEclassContent := `# child.eclass - Child eclass that inherits base
inherit base

CHILD_VAR="child"
child_func() {
	base_func
	einfo "child_func"
}
`
		if err := os.WriteFile(filepath.Join(eclassDir, "child.eclass"), []byte(childEclassContent), 0644); err != nil {
			t.Fatalf("failed to write child eclass: %v", err)
		}

		// Create environment
		pkg := &pkgdomain.Package{
			Name:    "app-misc/test",
			Version: "1.0",
		}

		env, err := NewEnvironment(pkg, "/tmp/portage", tmpDir, "/var/cache/distfiles")
		if err != nil {
			t.Fatalf("NewEnvironment failed: %v", err)
		}

		var stdout, stderr bytes.Buffer
		interp := NewInterpreter(env, &stdout, &stderr)
		helpers := interp.GetHelpers()

		// Inherit the child eclass (which should trigger inherit of base)
		err = helpers.Inherit([]string{"child"})
		if err != nil {
			t.Errorf("Inherit should succeed: %v", err)
		}

		// Check that both eclasses are in INHERITED
		inherited := helpers.GetEclassRegistry().GetInherited()
		if !bytes.Contains([]byte(inherited), []byte("base")) {
			t.Errorf("INHERITED should contain 'base', got '%s'", inherited)
		}
		if !bytes.Contains([]byte(inherited), []byte("child")) {
			t.Errorf("INHERITED should contain 'child', got '%s'", inherited)
		}
	})

	t.Run("inherit missing eclass fails gracefully", func(t *testing.T) {
		// Create a temporary directory with no eclasses
		tmpDir := t.TempDir()
		eclassDir := filepath.Join(tmpDir, "eclass")
		if err := os.MkdirAll(eclassDir, 0755); err != nil {
			t.Fatalf("failed to create eclass dir: %v", err)
		}

		pkg := &pkgdomain.Package{
			Name:    "app-misc/test",
			Version: "1.0",
		}

		env, err := NewEnvironment(pkg, "/tmp/portage", tmpDir, "/var/cache/distfiles")
		if err != nil {
			t.Fatalf("NewEnvironment failed: %v", err)
		}

		var stdout, stderr bytes.Buffer
		interp := NewInterpreter(env, &stdout, &stderr)
		helpers := interp.GetHelpers()

		// Try to inherit non-existent eclass
		err = helpers.Inherit([]string{"nonexistent"})
		if err == nil {
			t.Error("Inherit of non-existent eclass should fail")
		}

		// Error should mention inherit failed
		if err != nil && !bytes.Contains([]byte(err.Error()), []byte("inherit failed")) {
			t.Errorf("Error should mention 'inherit failed', got: %v", err)
		}
	})
}

// TestInheritThroughInterpreter tests inherit called from bash scripts.
func TestInheritThroughInterpreter(t *testing.T) {
	t.Run("inherit called from script", func(t *testing.T) {
		// Create test eclass directory
		tmpDir := t.TempDir()
		eclassDir := filepath.Join(tmpDir, "eclass")
		if err := os.MkdirAll(eclassDir, 0755); err != nil {
			t.Fatalf("failed to create eclass dir: %v", err)
		}

		// Create simple eclass
		eclassContent := `# simple.eclass
SIMPLE_VAR="set_by_eclass"
simple_helper() {
	einfo "Simple helper called"
}
`
		if err := os.WriteFile(filepath.Join(eclassDir, "simple.eclass"), []byte(eclassContent), 0644); err != nil {
			t.Fatalf("failed to write eclass: %v", err)
		}

		pkg := &pkgdomain.Package{
			Name:    "app-misc/test",
			Version: "1.0",
		}

		env, err := NewEnvironment(pkg, "/tmp/portage", tmpDir, "/var/cache/distfiles")
		if err != nil {
			t.Fatalf("NewEnvironment failed: %v", err)
		}

		var stdout, stderr bytes.Buffer
		interp := NewInterpreter(env, &stdout, &stderr)

		// Run script that calls inherit
		script := `inherit simple
einfo "After inherit"
`
		err = interp.Run(context.Background(), script)
		if err != nil {
			t.Errorf("Script execution failed: %v", err)
		}

		// Check INHERITED through helpers
		helpers := interp.GetHelpers()
		inherited := helpers.GetEclassRegistry().GetInherited()
		if inherited != "simple" {
			t.Errorf("INHERITED should be 'simple', got '%s'", inherited)
		}
	})

	t.Run("inherit builtin from script", func(t *testing.T) {
		pkg := &pkgdomain.Package{
			Name:    "app-misc/test",
			Version: "1.0",
		}

		env, err := NewEnvironment(pkg, "/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
		if err != nil {
			t.Fatalf("NewEnvironment failed: %v", err)
		}

		var stdout, stderr bytes.Buffer
		interp := NewInterpreter(env, &stdout, &stderr)

		// Run script that inherits builtin eclass and uses its function
		script := `inherit toolchain-funcs
CC=$(tc-getCC)
einfo "CC is $CC"
`
		err = interp.Run(context.Background(), script)
		if err != nil {
			t.Errorf("Script execution failed: %v", err)
		}

		// Check output contains gcc (default CC)
		output := stdout.String()
		if !bytes.Contains([]byte(output), []byte("gcc")) {
			t.Errorf("Output should contain 'gcc', got: %s", output)
		}
	})
}
