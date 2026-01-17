package cli

import (
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/config"
	"github.com/grpmsoft/grpm/internal/pkg"
)

func TestFormatUSEFlags_NilPackage(t *testing.T) {
	result := FormatUSEFlags(nil, nil)
	if result != `USE=""` {
		t.Errorf("Expected USE=\"\" for nil package, got %q", result)
	}
}

func TestFormatUSEFlags_EmptyUseFlags(t *testing.T) {
	p := &pkg.Package{
		Name:     "app-misc/hello",
		Version:  "2.10",
		UseFlags: make(map[string]bool),
	}

	result := FormatUSEFlags(p, nil)
	if result != `USE=""` {
		t.Errorf("Expected USE=\"\" for empty flags, got %q", result)
	}
}

func TestFormatUSEFlags_SingleFlagDisabled(t *testing.T) {
	p := &pkg.Package{
		Name:    "app-misc/hello",
		Version: "2.10",
		UseFlags: map[string]bool{
			"nls": true,
		},
	}

	// With no config, flag should be disabled by default
	result := FormatUSEFlags(p, nil)
	if !strings.Contains(result, "-nls") {
		t.Errorf("Expected disabled flag '-nls' in result, got %q", result)
	}
}

func TestFormatUSEFlags_GlobalUSEEnabled(t *testing.T) {
	p := &pkg.Package{
		Name:    "app-misc/hello",
		Version: "2.10",
		UseFlags: map[string]bool{
			"nls": true,
			"doc": true,
		},
	}

	cfg := &config.Config{
		MakeConf: &config.MakeConf{
			USE: []string{"nls", "-doc"},
		},
	}

	result := FormatUSEFlags(p, cfg)

	// nls should be enabled (no prefix)
	if !strings.Contains(result, "nls") || strings.Contains(result, "-nls") {
		t.Errorf("Expected enabled flag 'nls' in result, got %q", result)
	}

	// doc should be disabled (with - prefix)
	if !strings.Contains(result, "-doc") {
		t.Errorf("Expected disabled flag '-doc' in result, got %q", result)
	}
}

func TestFormatUSEFlags_MultipleFlags(t *testing.T) {
	p := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		UseFlags: map[string]bool{
			"static-libs": true,
			"minizip":     true,
			"debug":       true,
		},
	}

	cfg := &config.Config{
		MakeConf: &config.MakeConf{
			USE: []string{"static-libs", "-debug"},
		},
	}

	result := FormatUSEFlags(p, cfg)

	// static-libs should be enabled
	if !strings.Contains(result, "static-libs") || strings.Contains(result, "-static-libs") {
		t.Errorf("Expected enabled 'static-libs', got %q", result)
	}

	// debug should be disabled
	if !strings.Contains(result, "-debug") {
		t.Errorf("Expected disabled '-debug', got %q", result)
	}

	// minizip should be disabled (not in global USE)
	if !strings.Contains(result, "-minizip") {
		t.Errorf("Expected disabled '-minizip', got %q", result)
	}
}

func TestFormatUSEFlags_USEExpandPythonTargets(t *testing.T) {
	p := &pkg.Package{
		Name:    "dev-python/foo",
		Version: "1.0",
		UseFlags: map[string]bool{
			"python_targets_python3_11": true,
			"python_targets_python3_12": true,
			"test":                      true,
		},
	}

	cfg := &config.Config{
		MakeConf: &config.MakeConf{
			USE: []string{"python_targets_python3_11", "python_targets_python3_12"},
		},
	}

	result := FormatUSEFlags(p, cfg)

	// Should contain separate PYTHON_TARGETS variable
	if !strings.Contains(result, "PYTHON_TARGETS=") {
		t.Errorf("Expected PYTHON_TARGETS= in result, got %q", result)
	}

	// Should contain python3_11 and python3_12 (without prefix)
	if !strings.Contains(result, "python3_11") {
		t.Errorf("Expected 'python3_11' in PYTHON_TARGETS, got %q", result)
	}

	if !strings.Contains(result, "python3_12") {
		t.Errorf("Expected 'python3_12' in PYTHON_TARGETS, got %q", result)
	}
}

func TestFormatUSEFlags_USEExpandCPUFlags(t *testing.T) {
	p := &pkg.Package{
		Name:    "media-libs/foo",
		Version: "1.0",
		UseFlags: map[string]bool{
			"cpu_flags_x86_avx2":   true,
			"cpu_flags_x86_sse4_2": true,
			"opengl":               true,
		},
	}

	cfg := &config.Config{
		MakeConf: &config.MakeConf{
			USE: []string{"cpu_flags_x86_avx2", "cpu_flags_x86_sse4_2", "opengl"},
		},
	}

	result := FormatUSEFlags(p, cfg)

	// Should contain separate CPU_FLAGS_X86 variable
	if !strings.Contains(result, "CPU_FLAGS_X86=") {
		t.Errorf("Expected CPU_FLAGS_X86= in result, got %q", result)
	}

	// opengl should be in USE=
	if !strings.Contains(result, `USE="`) || !strings.Contains(result, "opengl") {
		t.Errorf("Expected 'opengl' in USE, got %q", result)
	}
}

func TestFormatUSEFlags_OnlyDisabledFlags(t *testing.T) {
	p := &pkg.Package{
		Name:    "app-misc/test",
		Version: "1.0",
		UseFlags: map[string]bool{
			"debug": true,
			"test":  true,
		},
	}

	// No global USE - all flags disabled
	result := FormatUSEFlags(p, nil)

	// Both flags should be disabled
	if !strings.Contains(result, "-debug") {
		t.Errorf("Expected '-debug' in result, got %q", result)
	}
	if !strings.Contains(result, "-test") {
		t.Errorf("Expected '-test' in result, got %q", result)
	}
}

func TestResolvePackageUSE_BasicResolution(t *testing.T) {
	p := &pkg.Package{
		Name:    "app-misc/hello",
		Version: "2.10",
		UseFlags: map[string]bool{
			"nls":  true,
			"doc":  true,
			"test": true,
		},
	}

	cfg := &config.Config{
		MakeConf: &config.MakeConf{
			USE: []string{"nls", "-test"},
		},
	}

	enabled, disabled := resolvePackageUSE(p, cfg)

	// nls should be enabled
	found := false
	for _, f := range enabled {
		if f == "nls" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'nls' in enabled flags, got enabled=%v", enabled)
	}

	// test and doc should be disabled
	testDisabled := false
	docDisabled := false
	for _, f := range disabled {
		if f == "test" {
			testDisabled = true
		}
		if f == "doc" {
			docDisabled = true
		}
	}
	if !testDisabled {
		t.Errorf("Expected 'test' in disabled flags, got disabled=%v", disabled)
	}
	if !docDisabled {
		t.Errorf("Expected 'doc' in disabled flags, got disabled=%v", disabled)
	}
}

func TestResolvePackageUSE_NilPackage(t *testing.T) {
	enabled, disabled := resolvePackageUSE(nil, nil)

	if enabled != nil || disabled != nil {
		t.Errorf("Expected nil results for nil package, got enabled=%v, disabled=%v", enabled, disabled)
	}
}

func TestGetUSEExpandPrefix(t *testing.T) {
	tests := []struct {
		flag     string
		expected string
	}{
		{"python_targets_python3_11", "python_targets_"},
		{"cpu_flags_x86_avx2", "cpu_flags_x86_"},
		{"ruby_targets_ruby32", "ruby_targets_"},
		{"l10n_en", "l10n_"},
		{"nls", ""},           // Not a USE_EXPAND flag
		{"static-libs", ""},   // Not a USE_EXPAND flag
		{"python3_11", ""},    // Partial match is not valid
		{"python_target", ""}, // Missing trailing underscore in prefix
	}

	for _, tc := range tests {
		result := getUSEExpandPrefix(tc.flag)
		if result != tc.expected {
			t.Errorf("getUSEExpandPrefix(%q) = %q, expected %q", tc.flag, result, tc.expected)
		}
	}
}

func TestPrefixToVarName(t *testing.T) {
	tests := []struct {
		prefix   string
		expected string
	}{
		{"python_targets_", "PYTHON_TARGETS"},
		{"cpu_flags_x86_", "CPU_FLAGS_X86"},
		{"ruby_targets_", "RUBY_TARGETS"},
		{"l10n_", "L10N"},
	}

	for _, tc := range tests {
		result := prefixToVarName(tc.prefix)
		if result != tc.expected {
			t.Errorf("prefixToVarName(%q) = %q, expected %q", tc.prefix, result, tc.expected)
		}
	}
}

func TestFormatUSEFlags_FlagsAreSorted(t *testing.T) {
	p := &pkg.Package{
		Name:    "app-misc/test",
		Version: "1.0",
		UseFlags: map[string]bool{
			"zlib":    true,
			"alpha":   true,
			"beta":    true,
			"unicode": true,
		},
	}

	cfg := &config.Config{
		MakeConf: &config.MakeConf{
			USE: []string{"zlib", "alpha"},
		},
	}

	result := FormatUSEFlags(p, cfg)

	// The result should have enabled flags first, then disabled flags
	// Both groups should be sorted alphabetically
	// Expected order in USE: alpha zlib -beta -unicode

	// Check that enabled flags come before disabled
	alphaIdx := strings.Index(result, "alpha")
	zlibIdx := strings.Index(result, "zlib")
	betaIdx := strings.Index(result, "-beta")
	unicodeIdx := strings.Index(result, "-unicode")

	if alphaIdx > zlibIdx {
		t.Errorf("Expected 'alpha' before 'zlib' (alphabetical), got indices alpha=%d, zlib=%d", alphaIdx, zlibIdx)
	}

	if betaIdx > unicodeIdx {
		t.Errorf("Expected '-beta' before '-unicode' (alphabetical), got indices beta=%d, unicode=%d", betaIdx, unicodeIdx)
	}

	if zlibIdx > betaIdx {
		t.Errorf("Expected enabled flags before disabled, got zlib=%d, beta=%d", zlibIdx, betaIdx)
	}
}

func TestFormatUSEFlags_OnlyUSEExpandFlags(t *testing.T) {
	p := &pkg.Package{
		Name:    "dev-python/bar",
		Version: "1.0",
		UseFlags: map[string]bool{
			"python_targets_python3_11": true,
			"python_targets_python3_12": true,
		},
	}

	cfg := &config.Config{
		MakeConf: &config.MakeConf{
			USE: []string{"python_targets_python3_11"},
		},
	}

	result := FormatUSEFlags(p, cfg)

	// Should have empty USE= but populated PYTHON_TARGETS=
	if !strings.HasPrefix(result, `USE=""`) {
		t.Errorf("Expected result to start with USE=\"\", got %q", result)
	}

	if !strings.Contains(result, "PYTHON_TARGETS=") {
		t.Errorf("Expected PYTHON_TARGETS= in result, got %q", result)
	}

	// python3_11 enabled, python3_12 disabled
	if !strings.Contains(result, "python3_11") {
		t.Errorf("Expected 'python3_11' enabled in result, got %q", result)
	}

	if !strings.Contains(result, "-python3_12") {
		t.Errorf("Expected '-python3_12' disabled in result, got %q", result)
	}
}

func TestFormatUSEFlags_FlagOnlyInIUSE(t *testing.T) {
	// Test that flags not in IUSE are ignored even if in global USE
	p := &pkg.Package{
		Name:    "app-misc/test",
		Version: "1.0",
		UseFlags: map[string]bool{
			"nls": true,
		},
	}

	cfg := &config.Config{
		MakeConf: &config.MakeConf{
			USE: []string{"nls", "ssl", "zlib"}, // ssl and zlib not in IUSE
		},
	}

	result := FormatUSEFlags(p, cfg)

	// Only nls should appear (it's in IUSE)
	if !strings.Contains(result, "nls") {
		t.Errorf("Expected 'nls' in result, got %q", result)
	}

	// ssl and zlib should NOT appear (not in IUSE)
	if strings.Contains(result, "ssl") {
		t.Errorf("Did not expect 'ssl' in result (not in IUSE), got %q", result)
	}

	if strings.Contains(result, "zlib") {
		t.Errorf("Did not expect 'zlib' in result (not in IUSE), got %q", result)
	}
}
