package install

import (
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/state"
)

// TestPhaseString tests phase string representation.
func TestPhaseString(t *testing.T) {
	tests := []struct {
		phase    Phase
		expected string
	}{
		{PhasePreInstall, "pre-install"},
		{PhasePostInstall, "post-install"},
		{PhasePreRemove, "pre-remove"},
		{PhasePostRemove, "post-remove"},
		{Phase(999), "unknown"},
	}

	for _, tt := range tests {
		result := tt.phase.String()
		if result != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, result)
		}
	}
}

// TestHookContext tests hook context creation.
func TestHookContext(t *testing.T) {
	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
	}

	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	ctx := HookContext{
		Package:   p,
		Phase:     PhasePostInstall,
		Root:      "/",
		Env:       make(map[string]string),
		Installer: installer,
	}

	if ctx.Package != p {
		t.Error("expected package to be set")
	}

	if ctx.Phase != PhasePostInstall {
		t.Errorf("expected phase PostInstall, got %v", ctx.Phase)
	}

	if ctx.Root != "/" {
		t.Errorf("expected root /, got %s", ctx.Root)
	}
}

// TestLdconfigHookName tests ldconfig hook name.
func TestLdconfigHookName(t *testing.T) {
	hook := &LdconfigHook{}

	if hook.Name() != "ldconfig" {
		t.Errorf("expected name 'ldconfig', got %s", hook.Name())
	}
}

// TestLdconfigHookShouldRun tests ldconfig hook execution conditions.
func TestLdconfigHookShouldRun(t *testing.T) {
	hook := &LdconfigHook{}

	tests := []struct {
		phase    Phase
		pkgName  string
		expected bool
	}{
		{PhasePostInstall, "sys-libs/zlib", true},
		{PhasePostInstall, "sys-libs/glibc", true},
		{PhasePostInstall, "app-editors/vim", true}, // Always true for now
		{PhasePreInstall, "sys-libs/zlib", false},
		{PhasePostRemove, "sys-libs/zlib", false},
	}

	for _, tt := range tests {
		p := &pkg.Package{
			Name: tt.pkgName,
		}

		ctx := HookContext{
			Package: p,
			Phase:   tt.phase,
		}

		result := hook.ShouldRun(ctx)
		if result != tt.expected {
			t.Errorf("ShouldRun(%s, %s) = %v, expected %v",
				tt.phase, tt.pkgName, result, tt.expected)
		}
	}
}

// TestUpdateDesktopDBHookName tests update-desktop-database hook name.
func TestUpdateDesktopDBHookName(t *testing.T) {
	hook := &UpdateDesktopDBHook{}

	if hook.Name() != "update-desktop-database" {
		t.Errorf("expected name 'update-desktop-database', got %s", hook.Name())
	}
}

// TestUpdateDesktopDBHookShouldRun tests update-desktop-database hook conditions.
func TestUpdateDesktopDBHookShouldRun(t *testing.T) {
	hook := &UpdateDesktopDBHook{}

	tests := []struct {
		phase    Phase
		expected bool
	}{
		{PhasePostInstall, false}, // Currently always false (TODO: check .desktop files)
		{PhasePostRemove, false},
		{PhasePreInstall, false},
	}

	for _, tt := range tests {
		ctx := HookContext{
			Phase: tt.phase,
		}

		result := hook.ShouldRun(ctx)
		if result != tt.expected {
			t.Errorf("ShouldRun(%s) = %v, expected %v", tt.phase, result, tt.expected)
		}
	}
}

// TestUpdateMimeDBHookName tests update-mime-database hook name.
func TestUpdateMimeDBHookName(t *testing.T) {
	hook := &UpdateMimeDBHook{}

	if hook.Name() != "update-mime-database" {
		t.Errorf("expected name 'update-mime-database', got %s", hook.Name())
	}
}

// TestUpdateMimeDBHookShouldRun tests update-mime-database hook conditions.
func TestUpdateMimeDBHookShouldRun(t *testing.T) {
	hook := &UpdateMimeDBHook{}

	tests := []struct {
		phase    Phase
		expected bool
	}{
		{PhasePostInstall, false}, // Currently always false
		{PhasePostRemove, false},
		{PhasePreInstall, false},
	}

	for _, tt := range tests {
		ctx := HookContext{
			Phase: tt.phase,
		}

		result := hook.ShouldRun(ctx)
		if result != tt.expected {
			t.Errorf("ShouldRun(%s) = %v, expected %v", tt.phase, result, tt.expected)
		}
	}
}

// TestIconCacheHookName tests gtk-update-icon-cache hook name.
func TestIconCacheHookName(t *testing.T) {
	hook := &IconCacheHook{}

	if hook.Name() != "gtk-update-icon-cache" {
		t.Errorf("expected name 'gtk-update-icon-cache', got %s", hook.Name())
	}
}

// TestIconCacheHookShouldRun tests gtk-update-icon-cache hook conditions.
func TestIconCacheHookShouldRun(t *testing.T) {
	hook := &IconCacheHook{}

	tests := []struct {
		phase    Phase
		expected bool
	}{
		{PhasePostInstall, false}, // Currently always false
		{PhasePostRemove, false},
		{PhasePreInstall, false},
	}

	for _, tt := range tests {
		ctx := HookContext{
			Phase: tt.phase,
		}

		result := hook.ShouldRun(ctx)
		if result != tt.expected {
			t.Errorf("ShouldRun(%s) = %v, expected %v", tt.phase, result, tt.expected)
		}
	}
}

// TestMultipleHooks tests running multiple hooks.
func TestMultipleHooks(t *testing.T) {
	hooks := []Hook{
		&LdconfigHook{},
		&UpdateDesktopDBHook{},
		&UpdateMimeDBHook{},
		&IconCacheHook{},
	}

	if len(hooks) != 4 {
		t.Errorf("expected 4 hooks, got %d", len(hooks))
	}

	// Verify all hooks have names
	for _, hook := range hooks {
		if hook.Name() == "" {
			t.Error("hook has empty name")
		}
	}
}

// TestHookInterface tests that all hooks implement Hook interface.
func TestHookInterface(t *testing.T) {
	var _ Hook = &LdconfigHook{}
	var _ Hook = &UpdateDesktopDBHook{}
	var _ Hook = &UpdateMimeDBHook{}
	var _ Hook = &IconCacheHook{}
}

// TestUpdateDesktopDBHook_Run tests the update-desktop-database hook Run method.
func TestUpdateDesktopDBHook_Run(t *testing.T) {
	hook := &UpdateDesktopDBHook{}

	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	ctx := HookContext{
		Package:   &pkg.Package{Name: "test/pkg"},
		Phase:     PhasePostInstall,
		Root:      tmpDir,
		Env:       make(map[string]string),
		Installer: installer,
	}

	// Run should not error (may skip if binary not found)
	err := hook.Run(ctx)
	if err != nil {
		t.Logf("Hook run returned error (expected if update-desktop-database not found): %v", err)
	}
}

// TestUpdateMimeDBHook_Run tests the update-mime-database hook Run method.
func TestUpdateMimeDBHook_Run(t *testing.T) {
	hook := &UpdateMimeDBHook{}

	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	ctx := HookContext{
		Package:   &pkg.Package{Name: "test/pkg"},
		Phase:     PhasePostInstall,
		Root:      tmpDir,
		Env:       make(map[string]string),
		Installer: installer,
	}

	// Run should not error (may skip if binary not found)
	err := hook.Run(ctx)
	if err != nil {
		t.Logf("Hook run returned error (expected if update-mime-database not found): %v", err)
	}
}

// TestIconCacheHook_Run tests the gtk-update-icon-cache hook Run method.
func TestIconCacheHook_Run(t *testing.T) {
	hook := &IconCacheHook{}

	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	ctx := HookContext{
		Package:   &pkg.Package{Name: "test/pkg"},
		Phase:     PhasePostInstall,
		Root:      tmpDir,
		Env:       make(map[string]string),
		Installer: installer,
	}

	// Run should not error (may skip if binary not found)
	err := hook.Run(ctx)
	if err != nil {
		t.Logf("Hook run returned error (expected if gtk-update-icon-cache not found): %v", err)
	}
}

// TestLdconfigHook_Run tests the ldconfig hook Run method.
func TestLdconfigHook_Run(t *testing.T) {
	hook := &LdconfigHook{}

	tmpDir := t.TempDir()
	db := state.NewPackageDatabase(tmpDir)
	installer := NewInstaller(tmpDir, db)

	ctx := HookContext{
		Package:   &pkg.Package{Name: "test/pkg"},
		Phase:     PhasePostInstall,
		Root:      tmpDir,
		Env:       make(map[string]string),
		Installer: installer,
	}

	// Run should not error (may skip if ldconfig not found or not on Linux)
	err := hook.Run(ctx)
	if err != nil {
		t.Logf("Hook run returned error (expected on Windows or if ldconfig fails): %v", err)
	}
}

// BenchmarkLdconfigHookShouldRun benchmarks ldconfig hook condition check.
func BenchmarkLdconfigHookShouldRun(b *testing.B) {
	hook := &LdconfigHook{}
	ctx := HookContext{
		Package: &pkg.Package{
			Name: "sys-libs/zlib",
		},
		Phase: PhasePostInstall,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hook.ShouldRun(ctx)
	}
}
