package eclass

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewExecutor(t *testing.T) {
	cache := NewCache()
	exec := NewExecutor(cache)

	if exec == nil {
		t.Fatal("NewExecutor returned nil")
	}

	if exec.cache != cache {
		t.Error("executor cache not set correctly")
	}
}

func TestExecutorInherit(t *testing.T) {
	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a simple eclass
	eclassContent := `
# test.eclass
IUSE="test-flag"
DEPEND="dev-libs/foo"

test_func() {
    echo "test function"
}
`
	if err := os.WriteFile(filepath.Join(eclassDir, "test.eclass"), []byte(eclassContent), 0644); err != nil {
		t.Fatal(err)
	}

	cache, err := NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exec := NewExecutor(cache, WithOutput(&stdout, &stderr))

	ctx := context.Background()
	if err := exec.Inherit(ctx, []string{"test"}); err != nil {
		t.Fatalf("Inherit failed: %v", err)
	}

	// Check inherited list
	inherited := exec.GetInherited()
	if len(inherited) != 1 || inherited[0] != "test" {
		t.Errorf("expected inherited=[test], got %v", inherited)
	}

	// Check INHERITED env var
	inheritedStr := exec.GetInheritedString()
	if inheritedStr != "test" {
		t.Errorf("expected INHERITED=test, got %s", inheritedStr)
	}
}

func TestExecutorMetadataAccumulation(t *testing.T) {
	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create first eclass
	eclass1 := `
# eclass1.eclass
IUSE="flag1"
DEPEND="dev-libs/lib1"
`
	if err := os.WriteFile(filepath.Join(eclassDir, "eclass1.eclass"), []byte(eclass1), 0644); err != nil {
		t.Fatal(err)
	}

	// Create second eclass
	eclass2 := `
# eclass2.eclass
IUSE="flag2"
DEPEND="dev-libs/lib2"
`
	if err := os.WriteFile(filepath.Join(eclassDir, "eclass2.eclass"), []byte(eclass2), 0644); err != nil {
		t.Fatal(err)
	}

	cache, err := NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exec := NewExecutor(cache, WithOutput(&stdout, &stderr))

	ctx := context.Background()

	// Inherit both eclasses
	if err := exec.Inherit(ctx, []string{"eclass1", "eclass2"}); err != nil {
		t.Fatalf("Inherit failed: %v", err)
	}

	// Check accumulated metadata
	accum := exec.GetAccumulatedMetadata()

	// IUSE should be accumulated
	if iuse, ok := accum["E_IUSE"]; !ok {
		t.Error("E_IUSE not found in accumulated metadata")
	} else if !strings.Contains(iuse, "flag1") || !strings.Contains(iuse, "flag2") {
		t.Errorf("expected IUSE to contain flag1 and flag2, got: %s", iuse)
	}

	// DEPEND should be accumulated
	if depend, ok := accum["E_DEPEND"]; !ok {
		t.Error("E_DEPEND not found in accumulated metadata")
	} else if !strings.Contains(depend, "lib1") || !strings.Contains(depend, "lib2") {
		t.Errorf("expected DEPEND to contain lib1 and lib2, got: %s", depend)
	}
}

func TestExecutorDoubleInherit(t *testing.T) {
	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create eclass
	eclassContent := `
# test.eclass
IUSE="test"
`
	if err := os.WriteFile(filepath.Join(eclassDir, "test.eclass"), []byte(eclassContent), 0644); err != nil {
		t.Fatal(err)
	}

	cache, err := NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exec := NewExecutor(cache, WithOutput(&stdout, &stderr))

	ctx := context.Background()

	// Inherit same eclass twice
	if err := exec.Inherit(ctx, []string{"test"}); err != nil {
		t.Fatalf("First inherit failed: %v", err)
	}
	if err := exec.Inherit(ctx, []string{"test"}); err != nil {
		t.Fatalf("Second inherit failed: %v", err)
	}

	// Should only be in list once
	inherited := exec.GetInherited()
	if len(inherited) != 1 {
		t.Errorf("expected 1 inherited eclass, got %d", len(inherited))
	}

	// Stdout should mention skipping
	if !strings.Contains(stdout.String(), "skipping") {
		t.Error("expected 'skipping' message for double inherit")
	}
}

func TestExecutorNestedInherit(t *testing.T) {
	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create base eclass
	baseEclass := `
# base.eclass
IUSE="base-flag"
base_func() {
    echo "base"
}
`
	if err := os.WriteFile(filepath.Join(eclassDir, "base.eclass"), []byte(baseEclass), 0644); err != nil {
		t.Fatal(err)
	}

	// Create child eclass that inherits base
	// Note: We simulate inherit by just setting variables - actual inherit
	// would require full interpreter integration
	childEclass := `
# child.eclass
IUSE="child-flag"
child_func() {
    echo "child"
}
`
	if err := os.WriteFile(filepath.Join(eclassDir, "child.eclass"), []byte(childEclass), 0644); err != nil {
		t.Fatal(err)
	}

	cache, err := NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exec := NewExecutor(cache, WithOutput(&stdout, &stderr))

	ctx := context.Background()

	// Inherit base first, then child
	if err := exec.Inherit(ctx, []string{"base", "child"}); err != nil {
		t.Fatalf("Inherit failed: %v", err)
	}

	// Both should be inherited
	inherited := exec.GetInherited()
	if len(inherited) != 2 {
		t.Errorf("expected 2 inherited eclasses, got %d", len(inherited))
	}
}

func TestExecutorExportFunctions(t *testing.T) {
	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatal(err)
	}

	cache, err := NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatal(err)
	}

	exec := NewExecutor(cache)

	// Export without ECLASS should fail
	err = exec.ExportFunctions([]string{"src_compile"})
	if err == nil {
		t.Error("expected error for EXPORT_FUNCTIONS without ECLASS")
	}

	// Set ECLASS and try again
	exec.mu.Lock()
	exec.currentEclass = "cmake"
	exec.mu.Unlock()

	err = exec.ExportFunctions([]string{"src_compile", "src_install"})
	if err != nil {
		t.Fatalf("ExportFunctions failed: %v", err)
	}

	// Check exported functions
	eclass, ok := exec.GetExportedFunction("src_compile")
	if !ok || eclass != "cmake" {
		t.Errorf("expected src_compile exported by cmake, got %s, %v", eclass, ok)
	}

	eclass, ok = exec.GetExportedFunction("src_install")
	if !ok || eclass != "cmake" {
		t.Errorf("expected src_install exported by cmake, got %s, %v", eclass, ok)
	}

	// Non-exported function should return false
	_, ok = exec.GetExportedFunction("src_test")
	if ok {
		t.Error("expected src_test to not be exported")
	}
}

func TestExecutorRun(t *testing.T) {
	cache := NewCache()

	var stdout, stderr bytes.Buffer
	exec := NewExecutor(cache, WithOutput(&stdout, &stderr))

	ctx := context.Background()

	// Run a simple script
	script := `
FOO="bar"
echo "Hello from script"
`
	if err := exec.Run(ctx, script); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !strings.Contains(stdout.String(), "Hello from script") {
		t.Errorf("expected output 'Hello from script', got: %s", stdout.String())
	}

	// Variable should be set
	foo := exec.GetVar("FOO")
	if foo != "bar" {
		t.Errorf("expected FOO=bar, got FOO=%s", foo)
	}
}

func TestExecutorSetGetVar(t *testing.T) {
	cache := NewCache()
	exec := NewExecutor(cache)

	exec.SetVar("TEST", "value")
	val := exec.GetVar("TEST")

	if val != "value" {
		t.Errorf("expected TEST=value, got TEST=%s", val)
	}
}

func TestExecutorReset(t *testing.T) {
	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatal(err)
	}

	eclassContent := `
# test.eclass
IUSE="test"
`
	if err := os.WriteFile(filepath.Join(eclassDir, "test.eclass"), []byte(eclassContent), 0644); err != nil {
		t.Fatal(err)
	}

	cache, err := NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exec := NewExecutor(cache, WithOutput(&stdout, &stderr))

	ctx := context.Background()

	// Inherit and accumulate state
	if err := exec.Inherit(ctx, []string{"test"}); err != nil {
		t.Fatalf("Inherit failed: %v", err)
	}

	// Set some variables
	exec.SetVar("CUSTOM", "value")

	// Reset
	exec.Reset()

	// Check state is cleared
	inherited := exec.GetInherited()
	if len(inherited) != 0 {
		t.Errorf("expected empty inherited after reset, got %v", inherited)
	}

	accum := exec.GetAccumulatedMetadata()
	if len(accum) != 0 {
		t.Errorf("expected empty accumulated metadata after reset, got %v", accum)
	}

	// CUSTOM should still be there (non-metadata vars preserved)
	if exec.GetVar("CUSTOM") != "value" {
		t.Error("expected CUSTOM to be preserved after reset")
	}
}

func TestExecutorFinalizeMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create eclass with DEPEND
	eclassContent := `
# test.eclass
DEPEND="eclass-dep"
`
	if err := os.WriteFile(filepath.Join(eclassDir, "test.eclass"), []byte(eclassContent), 0644); err != nil {
		t.Fatal(err)
	}

	cache, err := NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exec := NewExecutor(cache, WithOutput(&stdout, &stderr))

	ctx := context.Background()

	// Inherit eclass
	if err := exec.Inherit(ctx, []string{"test"}); err != nil {
		t.Fatalf("Inherit failed: %v", err)
	}

	// Simulate ebuild setting its own DEPEND
	exec.SetVar("DEPEND", "ebuild-dep")

	// Finalize
	exec.FinalizeMetadata()

	// DEPEND should be ebuild-dep + eclass-dep
	depend := exec.GetVar("DEPEND")
	if !strings.Contains(depend, "ebuild-dep") || !strings.Contains(depend, "eclass-dep") {
		t.Errorf("expected DEPEND to contain both ebuild-dep and eclass-dep, got: %s", depend)
	}
}

func TestExecutorWithEnvironment(t *testing.T) {
	cache := NewCache()

	initialEnv := map[string]string{
		"P":        "test-1.0",
		"PN":       "test",
		"PV":       "1.0",
		"CATEGORY": "app-misc",
	}

	exec := NewExecutor(cache, WithEnvironment(initialEnv))

	env := exec.GetEnv()
	for k, v := range initialEnv {
		if env[k] != v {
			t.Errorf("expected %s=%s, got %s=%s", k, v, k, env[k])
		}
	}
}

func TestExecutorEclassNotFound(t *testing.T) {
	cache := NewCache()

	var stdout, stderr bytes.Buffer
	exec := NewExecutor(cache, WithOutput(&stdout, &stderr))

	ctx := context.Background()

	err := exec.Inherit(ctx, []string{"nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent eclass")
	}

	if !IsEclassNotFound(err) && !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestExecutorGetEnv(t *testing.T) {
	cache := NewCache()
	exec := NewExecutor(cache)

	exec.SetVar("A", "1")
	exec.SetVar("B", "2")

	env := exec.GetEnv()

	// Verify it's a copy
	env["A"] = "modified"

	// Original should be unchanged
	if exec.GetVar("A") != "1" {
		t.Error("GetEnv should return a copy, not the original")
	}
}
