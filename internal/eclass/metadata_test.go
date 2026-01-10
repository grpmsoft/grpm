package eclass

import (
	"testing"
)

func TestNewMetadataManager(t *testing.T) {
	m := NewMetadataManager()
	if m == nil {
		t.Fatal("NewMetadataManager returned nil")
	}

	if len(m.accumulated) != 0 {
		t.Error("expected empty accumulated")
	}

	if len(m.exported) != 0 {
		t.Error("expected empty exported")
	}

	if len(m.inherited) != 0 {
		t.Error("expected empty inherited")
	}
}

func TestMetadataManagerInheritFlow(t *testing.T) {
	m := NewMetadataManager()

	// Simulate ebuild environment
	env := map[string]string{
		"P":      "test-1.0",
		"DEPEND": "ebuild-dep",
	}

	// Begin inherit
	backup := m.BeginInherit("test-eclass", env)

	if m.GetCurrentEclass() != "test-eclass" {
		t.Errorf("expected current eclass 'test-eclass', got '%s'", m.GetCurrentEclass())
	}

	if m.GetDepth() != 1 {
		t.Errorf("expected depth 1, got %d", m.GetDepth())
	}

	// Simulate eclass setting variables
	eclassEnv := map[string]string{
		"P":      "test-1.0",
		"DEPEND": "eclass-dep",
		"IUSE":   "eclass-flag",
	}

	// End inherit
	result := m.EndInherit("test-eclass", eclassEnv, backup)

	// Check inherited
	if !m.IsInherited("test-eclass") {
		t.Error("expected test-eclass to be inherited")
	}

	// Check accumulated
	accumulated := m.GetAccumulated()
	if accumulated["DEPEND"] != "eclass-dep" {
		t.Errorf("expected accumulated DEPEND='eclass-dep', got '%s'", accumulated["DEPEND"])
	}
	if accumulated["IUSE"] != "eclass-flag" {
		t.Errorf("expected accumulated IUSE='eclass-flag', got '%s'", accumulated["IUSE"])
	}

	// Check restored values
	if result["DEPEND"] != "ebuild-dep" {
		t.Errorf("expected restored DEPEND='ebuild-dep', got '%s'", result["DEPEND"])
	}
}

func TestMetadataManagerExportFunctions(t *testing.T) {
	m := NewMetadataManager()

	// Export without ECLASS should fail
	err := m.ExportFunctions([]string{"src_compile"})
	if err == nil {
		t.Error("expected error for EXPORT_FUNCTIONS without ECLASS")
	}

	// Set current eclass
	m.mu.Lock()
	m.currentEclass = "cmake"
	m.mu.Unlock()

	// Export should succeed
	err = m.ExportFunctions([]string{"src_compile", "src_install"})
	if err != nil {
		t.Fatalf("ExportFunctions failed: %v", err)
	}

	// Check exported
	eclass, ok := m.GetExportedFunction("src_compile")
	if !ok || eclass != "cmake" {
		t.Errorf("expected src_compile exported by cmake, got %s, %v", eclass, ok)
	}

	eclass, ok = m.GetExportedFunction("src_install")
	if !ok || eclass != "cmake" {
		t.Errorf("expected src_install exported by cmake, got %s, %v", eclass, ok)
	}

	// Non-exported should return false
	_, ok = m.GetExportedFunction("src_test")
	if ok {
		t.Error("expected src_test to not be exported")
	}
}

func TestMetadataManagerMultipleInherits(t *testing.T) {
	m := NewMetadataManager()

	// First inherit
	backup1 := m.BeginInherit("eclass1", map[string]string{})
	env1 := map[string]string{"DEPEND": "dep1", "IUSE": "flag1"}
	m.EndInherit("eclass1", env1, backup1)

	// Second inherit
	backup2 := m.BeginInherit("eclass2", map[string]string{})
	env2 := map[string]string{"DEPEND": "dep2", "IUSE": "flag2"}
	m.EndInherit("eclass2", env2, backup2)

	// Check both inherited
	inherited := m.GetInherited()
	if len(inherited) != 2 {
		t.Errorf("expected 2 inherited eclasses, got %d", len(inherited))
	}

	// Check accumulated (should be concatenated)
	accumulated := m.GetAccumulated()
	if accumulated["DEPEND"] != "dep1 dep2" {
		t.Errorf("expected DEPEND='dep1 dep2', got '%s'", accumulated["DEPEND"])
	}
	if accumulated["IUSE"] != "flag1 flag2" {
		t.Errorf("expected IUSE='flag1 flag2', got '%s'", accumulated["IUSE"])
	}
}

func TestMetadataManagerFinalizeMetadata(t *testing.T) {
	m := NewMetadataManager()

	// Simulate inherits
	backup := m.BeginInherit("test", map[string]string{})
	m.EndInherit("test", map[string]string{"DEPEND": "eclass-dep"}, backup)

	// Ebuild environment
	env := map[string]string{
		"DEPEND": "ebuild-dep",
		"IUSE":   "ebuild-flag",
	}

	// Finalize
	result := m.FinalizeMetadata(env)

	// DEPEND should be ebuild + eclass
	if result["DEPEND"] != "ebuild-dep eclass-dep" {
		t.Errorf("expected DEPEND='ebuild-dep eclass-dep', got '%s'", result["DEPEND"])
	}

	// IUSE should be just ebuild (no eclass value)
	if result["IUSE"] != "ebuild-flag" {
		t.Errorf("expected IUSE='ebuild-flag', got '%s'", result["IUSE"])
	}
}

func TestMetadataManagerReset(t *testing.T) {
	m := NewMetadataManager()

	// Add some state
	m.mu.Lock()
	m.currentEclass = "test"
	m.depth = 2
	m.inherited = append(m.inherited, "test")
	m.accumulated["DEPEND"] = "dep"
	m.exported["src_compile"] = "test"
	m.mu.Unlock()

	// Reset
	m.Reset()

	// Verify cleared
	if m.GetCurrentEclass() != "" {
		t.Error("expected empty current eclass after reset")
	}
	if m.GetDepth() != 0 {
		t.Error("expected depth 0 after reset")
	}
	if len(m.GetInherited()) != 0 {
		t.Error("expected empty inherited after reset")
	}
	if len(m.GetAccumulated()) != 0 {
		t.Error("expected empty accumulated after reset")
	}
}

func TestPhaseFunctionName(t *testing.T) {
	tests := []struct {
		eclass   string
		phase    string
		expected string
	}{
		{"cmake", "src_compile", "cmake_src_compile"},
		{"meson", "src_install", "meson_src_install"},
		{"python-r1", "pkg_setup", "python-r1_pkg_setup"},
	}

	for _, tt := range tests {
		result := PhaseFunctionName(tt.eclass, tt.phase)
		if result != tt.expected {
			t.Errorf("PhaseFunctionName(%s, %s) = %s, want %s",
				tt.eclass, tt.phase, result, tt.expected)
		}
	}
}

func TestIsPhaseFunction(t *testing.T) {
	tests := []struct {
		funcName      string
		expectEclass  string
		expectPhase   string
		expectMatched bool
	}{
		{"cmake_src_compile", "cmake", "src_compile", true},
		{"meson_src_install", "meson", "src_install", true},
		{"python-r1_pkg_setup", "python-r1", "pkg_setup", true},
		{"random_function", "", "", false},
		{"src_compile", "", "", false}, // No eclass prefix
	}

	for _, tt := range tests {
		eclass, phase, matched := IsPhaseFunction(tt.funcName)
		if matched != tt.expectMatched {
			t.Errorf("IsPhaseFunction(%s): matched = %v, want %v",
				tt.funcName, matched, tt.expectMatched)
		}
		if matched {
			if eclass != tt.expectEclass {
				t.Errorf("IsPhaseFunction(%s): eclass = %s, want %s",
					tt.funcName, eclass, tt.expectEclass)
			}
			if phase != tt.expectPhase {
				t.Errorf("IsPhaseFunction(%s): phase = %s, want %s",
					tt.funcName, phase, tt.expectPhase)
			}
		}
	}
}

func TestMetadataManagerGetInheritedString(t *testing.T) {
	m := NewMetadataManager()

	// Empty
	if m.GetInheritedString() != "" {
		t.Error("expected empty string for no inherited")
	}

	// Add some
	m.mu.Lock()
	m.inherited = []string{"foo", "bar", "baz"}
	m.mu.Unlock()

	result := m.GetInheritedString()
	if result != "foo bar baz" {
		t.Errorf("expected 'foo bar baz', got '%s'", result)
	}
}

func TestMetadataManagerGetAccumulatedVar(t *testing.T) {
	m := NewMetadataManager()

	// Empty
	if m.GetAccumulatedVar("DEPEND") != "" {
		t.Error("expected empty for unset var")
	}

	// Set
	m.mu.Lock()
	m.accumulated["DEPEND"] = "test-dep"
	m.mu.Unlock()

	if m.GetAccumulatedVar("DEPEND") != "test-dep" {
		t.Errorf("expected 'test-dep', got '%s'", m.GetAccumulatedVar("DEPEND"))
	}
}
