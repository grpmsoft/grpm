package pkg

import (
	"sort"
	"testing"
)

// mockUSEConfig implements USEResolverConfig for testing.
type mockUSEConfig struct {
	profileUSE []string
	globalUSE  []string
	packageUSE map[string][]string // "cat/name:version:slot" -> flags
	useExpand  map[string]string   // VAR_NAME -> "val1 val2"
	forcedUSE  []string
	maskedUSE  []string
}

func (m *mockUSEConfig) ProfileUSEFlags() []string { return m.profileUSE }
func (m *mockUSEConfig) GlobalUSEFlags() []string  { return m.globalUSE }
func (m *mockUSEConfig) PackageUSEFlags(category, name, version, slot string) []string {
	key := category + "/" + name + ":" + version + ":" + slot
	return m.packageUSE[key]
}
func (m *mockUSEConfig) USEExpandValue(varName string) string {
	return m.useExpand[varName]
}
func (m *mockUSEConfig) ForcedUSE() []string { return m.forcedUSE }
func (m *mockUSEConfig) MaskedUSE() []string { return m.maskedUSE }

func TestResolveEffectiveUSE_IUSEDefaultsOnly(t *testing.T) {
	// IUSE: +ssl -debug nls
	// Expected: ssl enabled, debug and nls disabled
	iuse := map[string]bool{
		"ssl":   true,  // +ssl
		"debug": false, // -debug
		"nls":   false, // nls (no prefix)
	}

	result := ResolveEffectiveUSE(iuse, "app-misc", "test", "1.0", "0", nil)

	if !result["ssl"] {
		t.Error("Expected ssl enabled from IUSE +ssl default")
	}
	if result["debug"] {
		t.Error("Expected debug disabled from IUSE -debug default")
	}
	if result["nls"] {
		t.Error("Expected nls disabled from IUSE bare default")
	}
}

func TestResolveEffectiveUSE_ProfileOverridesIUSE(t *testing.T) {
	// IUSE: +ssl -debug nls
	// Profile: -ssl nls
	// Expected: ssl disabled (profile overrides +default), nls enabled (profile), debug disabled
	iuse := map[string]bool{
		"ssl":   true,
		"debug": false,
		"nls":   false,
	}

	cfg := &mockUSEConfig{
		profileUSE: []string{"-ssl", "nls"},
	}

	result := ResolveEffectiveUSE(iuse, "app-misc", "test", "1.0", "0", cfg)

	if result["ssl"] {
		t.Error("Expected ssl disabled by profile override")
	}
	if !result["nls"] {
		t.Error("Expected nls enabled by profile")
	}
	if result["debug"] {
		t.Error("Expected debug still disabled")
	}
}

func TestResolveEffectiveUSE_MakeConfOverridesProfile(t *testing.T) {
	// IUSE: +ssl debug nls
	// Profile: nls -debug
	// make.conf: ssl debug -nls
	// Expected: ssl on (make.conf), debug on (make.conf overrides profile), nls off (make.conf)
	iuse := map[string]bool{
		"ssl":   true,
		"debug": false,
		"nls":   false,
	}

	cfg := &mockUSEConfig{
		profileUSE: []string{"nls", "-debug"},
		globalUSE:  []string{"ssl", "debug", "-nls"},
	}

	result := ResolveEffectiveUSE(iuse, "app-misc", "test", "1.0", "0", cfg)

	if !result["ssl"] {
		t.Error("Expected ssl enabled by make.conf")
	}
	if !result["debug"] {
		t.Error("Expected debug enabled by make.conf (overriding profile -debug)")
	}
	if result["nls"] {
		t.Error("Expected nls disabled by make.conf -nls")
	}
}

func TestResolveEffectiveUSE_PackageUSEOverridesAll(t *testing.T) {
	// IUSE: +ssl debug nls
	// Profile: nls
	// make.conf: ssl -nls
	// package.use for app-misc/test: -ssl nls
	// Expected: ssl off (package.use overrides make.conf), nls on (package.use), debug off
	iuse := map[string]bool{
		"ssl":   true,
		"debug": false,
		"nls":   false,
	}

	cfg := &mockUSEConfig{
		profileUSE: []string{"nls"},
		globalUSE:  []string{"ssl", "-nls"},
		packageUSE: map[string][]string{
			"app-misc/test:1.0:0": {"-ssl", "nls"},
		},
	}

	result := ResolveEffectiveUSE(iuse, "app-misc", "test", "1.0", "0", cfg)

	if result["ssl"] {
		t.Error("Expected ssl disabled by package.use override")
	}
	if !result["nls"] {
		t.Error("Expected nls enabled by package.use override")
	}
	if result["debug"] {
		t.Error("Expected debug still disabled")
	}
}

func TestResolveEffectiveUSE_USEExpand(t *testing.T) {
	iuse := map[string]bool{
		"python_targets_python3_12": false,
		"nls":                       false,
	}

	cfg := &mockUSEConfig{
		useExpand: map[string]string{
			"PYTHON_TARGETS": "python3_12 python3_13",
		},
	}

	result := ResolveEffectiveUSE(iuse, "dev-python", "foo", "1.0", "0", cfg)

	if !result["python_targets_python3_12"] {
		t.Error("Expected python_targets_python3_12 enabled via USE_EXPAND")
	}
	if !result["python_targets_python3_13"] {
		t.Error("Expected python_targets_python3_13 enabled via USE_EXPAND (even if not in IUSE)")
	}
	if result["nls"] {
		t.Error("Expected nls still disabled")
	}
}

func TestResolveEffectiveUSE_NilConfig(t *testing.T) {
	iuse := map[string]bool{
		"ssl":   true,
		"debug": false,
	}

	result := ResolveEffectiveUSE(iuse, "app-misc", "test", "1.0", "0", nil)

	if !result["ssl"] {
		t.Error("Expected ssl enabled from IUSE default with nil config")
	}
	if result["debug"] {
		t.Error("Expected debug disabled with nil config")
	}
}

func TestResolveEffectiveUSE_EmptyIUSE(t *testing.T) {
	iuse := map[string]bool{}
	cfg := &mockUSEConfig{
		globalUSE: []string{"ssl", "nls"},
	}

	result := ResolveEffectiveUSE(iuse, "app-misc", "test", "1.0", "0", cfg)

	// Flags from make.conf are still applied even if not in IUSE
	// This matches Portage behavior: global USE flags affect the effective set
	if !result["ssl"] {
		t.Error("Expected ssl enabled from make.conf even without IUSE")
	}
}

func TestIsUSEConditionalActive(t *testing.T) {
	effectiveUSE := map[string]bool{
		"ssl":   true,
		"nls":   true,
		"debug": false, // explicitly false (won't matter — only presence in map matters for true)
	}
	// Note: effectiveUSE["debug"] = false means debug was set but is false.
	// In our model, only flags with true value are "enabled".
	// Flags NOT in the map at all are also "disabled".

	// Remove debug to match our model (only true values in map)
	delete(effectiveUSE, "debug")

	tests := []struct {
		name     string
		cond     string
		expected bool
	}{
		{"empty condition", "", true},
		{"enabled flag", "ssl", true},
		{"enabled flag 2", "nls", true},
		{"disabled flag (not in map)", "debug", false},
		{"unknown flag", "mysql", false},
		{"negated enabled", "!ssl", false},
		{"negated disabled", "!debug", true},
		{"negated unknown", "!mysql", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsUSEConditionalActive(tt.cond, effectiveUSE)
			if result != tt.expected {
				t.Errorf("IsUSEConditionalActive(%q) = %v, want %v", tt.cond, result, tt.expected)
			}
		})
	}
}

func TestParseIUSEDefaults(t *testing.T) {
	tests := []struct {
		name         string
		iuseStr      string
		wantDefaults map[string]bool
		wantFlags    map[string]bool
	}{
		{
			name:    "mixed prefixes",
			iuseStr: "+ssl -debug nls test",
			wantDefaults: map[string]bool{
				"ssl":   true,
				"debug": false,
				"nls":   false,
				"test":  false,
			},
			wantFlags: map[string]bool{
				"ssl":   true,
				"debug": false,
				"nls":   false,
				"test":  false,
			},
		},
		{
			name:         "empty string",
			iuseStr:      "",
			wantDefaults: map[string]bool{},
			wantFlags:    map[string]bool{},
		},
		{
			name:    "all enabled defaults",
			iuseStr: "+ssl +nls +doc",
			wantDefaults: map[string]bool{
				"ssl": true,
				"nls": true,
				"doc": true,
			},
			wantFlags: map[string]bool{
				"ssl": true,
				"nls": true,
				"doc": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDefaults, gotFlags := ParseIUSEDefaults(tt.iuseStr)

			if len(gotDefaults) != len(tt.wantDefaults) {
				t.Errorf("defaults length: got %d, want %d", len(gotDefaults), len(tt.wantDefaults))
			}
			for k, v := range tt.wantDefaults {
				if gotDefaults[k] != v {
					t.Errorf("defaults[%q] = %v, want %v", k, gotDefaults[k], v)
				}
			}

			if len(gotFlags) != len(tt.wantFlags) {
				t.Errorf("flags length: got %d, want %d", len(gotFlags), len(tt.wantFlags))
			}
			for k, v := range tt.wantFlags {
				if gotFlags[k] != v {
					t.Errorf("flags[%q] = %v, want %v", k, gotFlags[k], v)
				}
			}
		})
	}
}

func TestSplitEnabledDisabled(t *testing.T) {
	effectiveUSE := map[string]bool{
		"ssl": true,
		"nls": true,
	}

	iuseFlags := map[string]bool{
		"ssl":   true,
		"nls":   false,
		"debug": false,
		"test":  false,
	}

	enabled, disabled := SplitEnabledDisabled(effectiveUSE, iuseFlags)

	sort.Strings(enabled)
	sort.Strings(disabled)

	if len(enabled) != 2 {
		t.Errorf("expected 2 enabled, got %d: %v", len(enabled), enabled)
	}
	if len(disabled) != 2 {
		t.Errorf("expected 2 disabled, got %d: %v", len(disabled), disabled)
	}

	// Verify exact content
	wantEnabled := []string{"nls", "ssl"}
	wantDisabled := []string{"debug", "test"}

	for i, w := range wantEnabled {
		if i >= len(enabled) || enabled[i] != w {
			t.Errorf("enabled[%d]: got %q, want %q", i, safeIndex(enabled, i), w)
		}
	}
	for i, w := range wantDisabled {
		if i >= len(disabled) || disabled[i] != w {
			t.Errorf("disabled[%d]: got %q, want %q", i, safeIndex(disabled, i), w)
		}
	}
}

func safeIndex(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return "<out of bounds>"
}

func TestResolveEffectiveUSE_FullPriorityChain(t *testing.T) {
	// This test validates the complete Portage priority chain:
	// IUSE defaults < profile < make.conf < USE_EXPAND < package.use
	//
	// flag "a": +a in IUSE, -a in profile -> disabled
	// flag "b": b in IUSE, b in profile, -b in make.conf -> disabled
	// flag "c": c in IUSE, c in make.conf, -c in package.use -> disabled
	// flag "d": d in IUSE, d in package.use -> enabled (package.use wins)
	// flag "e": +e in IUSE (no override) -> enabled
	iuse := map[string]bool{
		"a": true,  // +a
		"b": false, // b
		"c": false, // c
		"d": false, // d
		"e": true,  // +e
	}

	cfg := &mockUSEConfig{
		profileUSE: []string{"-a", "b"},
		globalUSE:  []string{"-b", "c"},
		packageUSE: map[string][]string{
			"app-misc/test:1.0:0": {"-c", "d"},
		},
	}

	result := ResolveEffectiveUSE(iuse, "app-misc", "test", "1.0", "0", cfg)

	expectations := map[string]bool{
		"a": false, // IUSE +a, profile -a -> disabled
		"b": false, // profile b, make.conf -b -> disabled
		"c": false, // make.conf c, package.use -c -> disabled
		"d": true,  // package.use d -> enabled
		"e": true,  // IUSE +e, no override -> enabled
	}

	for flag, want := range expectations {
		got := result[flag]
		if got != want {
			t.Errorf("flag %q: got %v, want %v", flag, got, want)
		}
	}
}

func TestResolveEffectiveUSE_ForcedOverridesPackageUse(t *testing.T) {
	iuse := map[string]bool{"forced_flag": false}
	cfg := &mockUSEConfig{
		packageUSE: map[string][]string{
			"app-misc/test:1.0:0": {"-forced_flag"},
		},
		forcedUSE: []string{"forced_flag"},
	}

	result := ResolveEffectiveUSE(iuse, "app-misc", "test", "1.0", "0", cfg)

	if !result["forced_flag"] {
		t.Error("use.force must override package.use -forced_flag: PMS says force is final")
	}
}

func TestResolveEffectiveUSE_MaskedOverridesPackageUse(t *testing.T) {
	iuse := map[string]bool{"masked_flag": false}
	cfg := &mockUSEConfig{
		packageUSE: map[string][]string{
			"app-misc/test:1.0:0": {"masked_flag"},
		},
		maskedUSE: []string{"masked_flag"},
	}

	result := ResolveEffectiveUSE(iuse, "app-misc", "test", "1.0", "0", cfg)

	if result["masked_flag"] {
		t.Error("use.mask must override package.use masked_flag: PMS says mask is final")
	}
}
