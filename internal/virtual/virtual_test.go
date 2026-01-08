package virtual

import (
	"os"
	"path/filepath"
	"testing"
)

// TestIsVirtual tests the IsVirtual function.
func TestIsVirtual(t *testing.T) {
	tests := []struct {
		name     string
		pkgName  string
		expected bool
	}{
		{"virtual jdk", "virtual/jdk", true},
		{"virtual editor", "virtual/editor", true},
		{"virtual mta", "virtual/mta", true},
		{"dev-java package", "dev-java/openjdk", false},
		{"sys-libs package", "sys-libs/zlib", false},
		{"app-misc package", "app-misc/hello", false},
		{"empty string", "", false},
		{"just virtual", "virtual", false},
		{"virtual no slash", "virtual-jdk", false},
		{"similar prefix", "virtual-test/pkg", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsVirtual(tt.pkgName)
			if result != tt.expected {
				t.Errorf("IsVirtual(%q) = %v, want %v", tt.pkgName, result, tt.expected)
			}
		})
	}
}

// TestExtractCategory tests category extraction from package names.
func TestExtractCategory(t *testing.T) {
	tests := []struct {
		pkgName  string
		expected string
	}{
		{"virtual/jdk", "virtual"},
		{"dev-java/openjdk", "dev-java"},
		{"sys-libs/zlib", "sys-libs"},
		{"package-only", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.pkgName, func(t *testing.T) {
			result := ExtractCategory(tt.pkgName)
			if result != tt.expected {
				t.Errorf("ExtractCategory(%q) = %q, want %q", tt.pkgName, result, tt.expected)
			}
		})
	}
}

// TestExtractPackageName tests package name extraction.
func TestExtractPackageName(t *testing.T) {
	tests := []struct {
		pkgName  string
		expected string
	}{
		{"virtual/jdk", "jdk"},
		{"dev-java/openjdk", "openjdk"},
		{"sys-libs/zlib", "zlib"},
		{"package-only", "package-only"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.pkgName, func(t *testing.T) {
			result := ExtractPackageName(tt.pkgName)
			if result != tt.expected {
				t.Errorf("ExtractPackageName(%q) = %q, want %q", tt.pkgName, result, tt.expected)
			}
		})
	}
}

// TestStripSlot tests slot stripping from atoms.
func TestStripSlot(t *testing.T) {
	tests := []struct {
		atom     string
		expected string
	}{
		{"dev-java/openjdk:17", "dev-java/openjdk"},
		{"sys-libs/zlib:0/1", "sys-libs/zlib"},
		{"dev-java/openjdk", "dev-java/openjdk"},
		{"pkg:=", "pkg"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.atom, func(t *testing.T) {
			result := StripSlot(tt.atom)
			if result != tt.expected {
				t.Errorf("StripSlot(%q) = %q, want %q", tt.atom, result, tt.expected)
			}
		})
	}
}

// TestExtractSlot tests slot extraction from atoms.
func TestExtractSlot(t *testing.T) {
	tests := []struct {
		atom     string
		expected string
	}{
		{"dev-java/openjdk:17", "17"},
		{"sys-libs/zlib:0/1", "0/1"},
		{"dev-java/openjdk", ""},
		{"pkg:=", "="},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.atom, func(t *testing.T) {
			result := ExtractSlot(tt.atom)
			if result != tt.expected {
				t.Errorf("ExtractSlot(%q) = %q, want %q", tt.atom, result, tt.expected)
			}
		})
	}
}

// TestNewVirtual tests Virtual constructor.
func TestNewVirtual(t *testing.T) {
	v := NewVirtual("virtual/jdk", "17")

	if v.Name != "virtual/jdk" {
		t.Errorf("expected Name = virtual/jdk, got %s", v.Name)
	}
	if v.Version != "17" {
		t.Errorf("expected Version = 17, got %s", v.Version)
	}
	if len(v.Providers) != 0 {
		t.Errorf("expected empty providers, got %d", len(v.Providers))
	}
}

// TestVirtualAddProvider tests adding providers.
func TestVirtualAddProvider(t *testing.T) {
	v := NewVirtual("virtual/jdk", "17")

	v.AddProvider("dev-java/openjdk:17")
	v.AddProvider("dev-java/oracle-jdk-bin:17")

	if len(v.Providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(v.Providers))
	}
	if !v.HasProviders() {
		t.Error("expected HasProviders() = true")
	}
}

// TestResolverSelectProvider tests provider selection logic.
func TestResolverSelectProvider(t *testing.T) {
	tests := []struct {
		name           string
		defaults       map[string]string
		installed      map[string]string
		virtual        string
		available      []string
		expectedResult string
		expectError    bool
	}{
		{
			name:           "first available (no preference)",
			defaults:       nil,
			installed:      nil,
			virtual:        "virtual/jdk",
			available:      []string{"dev-java/openjdk:17", "dev-java/oracle-jdk-bin:17"},
			expectedResult: "dev-java/openjdk:17",
			expectError:    false,
		},
		{
			name:           "prefer installed",
			defaults:       nil,
			installed:      map[string]string{"virtual/jdk": "dev-java/oracle-jdk-bin"},
			virtual:        "virtual/jdk",
			available:      []string{"dev-java/openjdk:17", "dev-java/oracle-jdk-bin:17"},
			expectedResult: "dev-java/oracle-jdk-bin:17",
			expectError:    false,
		},
		{
			name:           "prefer user default",
			defaults:       map[string]string{"virtual/jdk": "dev-java/oracle-jdk-bin"},
			installed:      nil,
			virtual:        "virtual/jdk",
			available:      []string{"dev-java/openjdk:17", "dev-java/oracle-jdk-bin:17"},
			expectedResult: "dev-java/oracle-jdk-bin:17",
			expectError:    false,
		},
		{
			name:           "installed overrides default",
			defaults:       map[string]string{"virtual/jdk": "dev-java/openjdk"},
			installed:      map[string]string{"virtual/jdk": "dev-java/oracle-jdk-bin"},
			virtual:        "virtual/jdk",
			available:      []string{"dev-java/openjdk:17", "dev-java/oracle-jdk-bin:17"},
			expectedResult: "dev-java/oracle-jdk-bin:17",
			expectError:    false,
		},
		{
			name:           "no providers",
			defaults:       nil,
			installed:      nil,
			virtual:        "virtual/jdk",
			available:      []string{},
			expectedResult: "",
			expectError:    true,
		},
		{
			name:           "preference not in available (fallback)",
			defaults:       map[string]string{"virtual/jdk": "dev-java/graalvm"},
			installed:      nil,
			virtual:        "virtual/jdk",
			available:      []string{"dev-java/openjdk:17", "dev-java/oracle-jdk-bin:17"},
			expectedResult: "dev-java/openjdk:17",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewResolver()

			for virtual, provider := range tt.defaults {
				r.SetDefault(virtual, provider)
			}
			for virtual, provider := range tt.installed {
				r.SetInstalled(virtual, provider)
			}

			result, err := r.SelectProvider(tt.virtual, tt.available)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expectedResult {
				t.Errorf("SelectProvider() = %q, want %q", result, tt.expectedResult)
			}
		})
	}
}

// TestResolverResolveAll tests batch resolution.
func TestResolverResolveAll(t *testing.T) {
	r := NewResolver()
	r.SetDefault("virtual/jdk", "dev-java/openjdk")
	r.SetDefault("virtual/editor", "app-editors/vim")

	virtuals := map[string][]string{
		"virtual/jdk":    {"dev-java/openjdk:17", "dev-java/oracle-jdk-bin:17"},
		"virtual/editor": {"app-editors/vim", "app-editors/emacs"},
	}

	result, err := r.ResolveAll(virtuals)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}

	if result["virtual/jdk"] != "dev-java/openjdk:17" {
		t.Errorf("virtual/jdk = %q, want dev-java/openjdk:17", result["virtual/jdk"])
	}
	if result["virtual/editor"] != "app-editors/vim" {
		t.Errorf("virtual/editor = %q, want app-editors/vim", result["virtual/editor"])
	}
}

// TestResolverStats tests statistics.
func TestResolverStats(t *testing.T) {
	r := NewResolver()
	r.SetDefault("virtual/jdk", "dev-java/openjdk")
	r.SetInstalled("virtual/editor", "app-editors/vim")

	stats := r.Stats()

	if stats.DefaultCount != 1 {
		t.Errorf("DefaultCount = %d, want 1", stats.DefaultCount)
	}
	if stats.InstalledCount != 1 {
		t.Errorf("InstalledCount = %d, want 1", stats.InstalledCount)
	}
}

// TestResolverClear tests clearing the resolver.
func TestResolverClear(t *testing.T) {
	r := NewResolver()
	r.SetDefault("virtual/jdk", "dev-java/openjdk")
	r.SetInstalled("virtual/editor", "app-editors/vim")

	r.Clear()

	stats := r.Stats()
	if stats.DefaultCount != 0 || stats.InstalledCount != 0 {
		t.Errorf("expected empty resolver after Clear()")
	}
}

// TestParserGetProviders tests OR-block provider extraction.
func TestParserGetProviders(t *testing.T) {
	tests := []struct {
		name     string
		rdepend  string
		expected []string
	}{
		{
			name: "simple OR block",
			rdepend: `|| (
				dev-java/openjdk:17
				dev-java/oracle-jdk-bin:17
			)`,
			expected: []string{"dev-java/openjdk:17", "dev-java/oracle-jdk-bin:17"},
		},
		{
			name:     "single line OR block",
			rdepend:  `|| ( dev-java/openjdk:17 dev-java/oracle-jdk-bin:17 )`,
			expected: []string{"dev-java/openjdk:17", "dev-java/oracle-jdk-bin:17"},
		},
		{
			name: "OR block with version operators",
			rdepend: `|| (
				>=dev-java/openjdk-17
				>=dev-java/oracle-jdk-bin-17
			)`,
			expected: []string{">=dev-java/openjdk-17", ">=dev-java/oracle-jdk-bin-17"},
		},
		{
			name:     "no OR block",
			rdepend:  `dev-java/openjdk:17`,
			expected: []string{},
		},
		{
			name:     "empty string",
			rdepend:  "",
			expected: []string{},
		},
		{
			name: "multiple OR blocks",
			rdepend: `
				|| ( dev-java/openjdk:17 dev-java/oracle-jdk-bin:17 )
				|| ( app-editors/vim app-editors/emacs )
			`,
			expected: []string{
				"dev-java/openjdk:17", "dev-java/oracle-jdk-bin:17",
				"app-editors/vim", "app-editors/emacs",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			result := p.GetProviders(tt.rdepend)

			if len(result) != len(tt.expected) {
				t.Errorf("GetProviders() returned %d items, want %d", len(result), len(tt.expected))
				t.Logf("Got: %v", result)
				t.Logf("Want: %v", tt.expected)
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("result[%d] = %q, want %q", i, result[i], expected)
				}
			}
		})
	}
}

// TestParserGetProvidersMap tests grouped provider extraction.
func TestParserGetProvidersMap(t *testing.T) {
	rdepend := `
		|| ( dev-java/openjdk:17 dev-java/oracle-jdk-bin:17 )
		|| ( app-editors/vim app-editors/emacs )
	`

	p := NewParser()
	result := p.GetProvidersMap(rdepend)

	if len(result) != 2 {
		t.Errorf("expected 2 OR groups, got %d", len(result))
	}

	// Check first group
	if group, ok := result[0]; ok {
		if len(group) != 2 {
			t.Errorf("group 0: expected 2 providers, got %d", len(group))
		}
	} else {
		t.Error("missing group 0")
	}

	// Check second group
	if group, ok := result[1]; ok {
		if len(group) != 2 {
			t.Errorf("group 1: expected 2 providers, got %d", len(group))
		}
	} else {
		t.Error("missing group 1")
	}
}

// TestParseVirtualProviders tests the convenience function.
func TestParseVirtualProviders(t *testing.T) {
	rdepend := `|| (
		>=dev-java/openjdk-17:17[ssl]
		>=dev-java/oracle-jdk-bin-17:17
	)`

	result := ParseVirtualProviders(rdepend)

	if len(result) != 2 {
		t.Errorf("expected 2 providers, got %d: %v", len(result), result)
	}
}

// TestNormalizeProvider tests provider normalization.
func TestNormalizeProvider(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"dev-java/openjdk", "dev-java/openjdk"},
		{">=dev-java/openjdk-17", "dev-java/openjdk"},
		{"dev-java/openjdk:17", "dev-java/openjdk:17"},
		{">=dev-java/openjdk-17:17", "dev-java/openjdk:17"},
		{"dev-java/openjdk[ssl]", "dev-java/openjdk"},
		{"!dev-java/openjdk", "dev-java/openjdk"},
		{"!!dev-java/openjdk", "dev-java/openjdk"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeProvider(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeProvider(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestConfigLoadDefaults tests loading configuration from file.
func TestConfigLoadDefaults(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "virtuals.conf")

	content := `# Virtual package provider configuration
virtual/jdk dev-java/openjdk
virtual/editor app-editors/vim
# Comment line
virtual/mta mail-mta/postfix

# Invalid entry (ignored)
not-virtual test

`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadDefaults(configPath)
	if err != nil {
		t.Fatalf("LoadDefaults() error: %v", err)
	}

	if cfg.Count() != 3 {
		t.Errorf("expected 3 entries, got %d", cfg.Count())
	}

	if provider, ok := cfg.Get("virtual/jdk"); !ok || provider != "dev-java/openjdk" {
		t.Errorf("virtual/jdk = %q, want dev-java/openjdk", provider)
	}
	if provider, ok := cfg.Get("virtual/editor"); !ok || provider != "app-editors/vim" {
		t.Errorf("virtual/editor = %q, want app-editors/vim", provider)
	}
	if provider, ok := cfg.Get("virtual/mta"); !ok || provider != "mail-mta/postfix" {
		t.Errorf("virtual/mta = %q, want mail-mta/postfix", provider)
	}
}

// TestConfigLoadDefaultsNotExist tests loading from non-existent file.
func TestConfigLoadDefaultsNotExist(t *testing.T) {
	cfg, err := LoadDefaults("/nonexistent/path/virtuals.conf")
	if err != nil {
		t.Fatalf("LoadDefaults() should not error for missing file: %v", err)
	}

	if cfg.Count() != 0 {
		t.Errorf("expected empty config for missing file")
	}
}

// TestConfigSave tests saving configuration.
func TestConfigSave(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "output.conf")

	cfg := NewConfig()
	cfg.Set("virtual/jdk", "dev-java/openjdk")
	cfg.Set("virtual/editor", "app-editors/vim")

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Read back and verify
	loaded, err := LoadDefaults(configPath)
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}

	if loaded.Count() != 2 {
		t.Errorf("expected 2 entries after reload, got %d", loaded.Count())
	}
}

// TestConfigApplyToResolver tests applying config to resolver.
func TestConfigApplyToResolver(t *testing.T) {
	cfg := NewConfig()
	cfg.Set("virtual/jdk", "dev-java/openjdk")
	cfg.Set("virtual/editor", "app-editors/vim")

	r := NewResolver()
	cfg.ApplyToResolver(r)

	stats := r.Stats()
	if stats.DefaultCount != 2 {
		t.Errorf("expected 2 defaults after apply, got %d", stats.DefaultCount)
	}

	if def, ok := r.GetDefault("virtual/jdk"); !ok || def != "dev-java/openjdk" {
		t.Errorf("virtual/jdk default = %q, want dev-java/openjdk", def)
	}
}

// TestIsPackageAtom tests package atom validation.
func TestIsPackageAtom(t *testing.T) {
	tests := []struct {
		token    string
		expected bool
	}{
		{"dev-java/openjdk", true},
		{">=dev-java/openjdk-17", true},
		{"dev-java/openjdk:17", true},
		{"dev-java/openjdk[ssl]", true},
		{"!dev-java/openjdk", true},
		{"!!dev-java/openjdk", true},
		{"~dev-java/openjdk-17", true},
		{"||", false},
		{"(", false},
		{")", false},
		{"", false},
		{"ssl?", false},
	}

	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			result := isPackageAtom(tt.token)
			if result != tt.expected {
				t.Errorf("isPackageAtom(%q) = %v, want %v", tt.token, result, tt.expected)
			}
		})
	}
}

// TestConfigRemove tests removing entries.
func TestConfigRemove(t *testing.T) {
	cfg := NewConfig()
	cfg.Set("virtual/jdk", "dev-java/openjdk")
	cfg.Set("virtual/editor", "app-editors/vim")

	cfg.Remove("virtual/jdk")

	if cfg.Count() != 1 {
		t.Errorf("expected 1 entry after remove, got %d", cfg.Count())
	}

	if _, ok := cfg.Get("virtual/jdk"); ok {
		t.Error("virtual/jdk should be removed")
	}
}

// TestResolverConcurrency tests thread safety.
func TestResolverConcurrency(t *testing.T) {
	r := NewResolver()

	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			r.SetDefault("virtual/jdk", "dev-java/openjdk")
			r.SetInstalled("virtual/editor", "app-editors/vim")
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_, _ = r.GetDefault("virtual/jdk")
			_, _ = r.GetInstalled("virtual/editor")
			_ = r.Stats()
		}
		done <- true
	}()

	// SelectProvider goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_, _ = r.SelectProvider("virtual/jdk", []string{"dev-java/openjdk", "dev-java/oracle"})
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done
}
