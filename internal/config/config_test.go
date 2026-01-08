package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultMakeConf(t *testing.T) {
	mc := DefaultMakeConf()

	if mc.CFLAGS != "-O2 -pipe" {
		t.Errorf("CFLAGS = %s, expected -O2 -pipe", mc.CFLAGS)
	}

	if mc.PORTDIR != "/var/db/repos/gentoo" {
		t.Errorf("PORTDIR = %s, expected /var/db/repos/gentoo", mc.PORTDIR)
	}

	if mc.DISTDIR != "/var/cache/distfiles" {
		t.Errorf("DISTDIR = %s, expected /var/cache/distfiles", mc.DISTDIR)
	}

	if len(mc.FEATURES) != 3 {
		t.Errorf("FEATURES length = %d, expected 3", len(mc.FEATURES))
	}
}

func TestLoadConfigNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Should use default values
	if cfg.MakeConf.CFLAGS != "-O2 -pipe" {
		t.Errorf("CFLAGS = %s, expected default", cfg.MakeConf.CFLAGS)
	}
}

func TestParseMakeConfLine(t *testing.T) {
	cfg := &Config{
		MakeConf: DefaultMakeConf(),
	}

	tests := []struct {
		line     string
		checkVar func() bool
	}{
		{`CFLAGS="-O3 -march=native"`, func() bool {
			return cfg.MakeConf.CFLAGS == "-O3 -march=native"
		}},
		{`MAKEOPTS="-j8"`, func() bool {
			return cfg.MakeConf.MAKEOPTS == "-j8"
		}},
		{`USE="ssl -debug"`, func() bool {
			return len(cfg.MakeConf.USE) == 2 && cfg.MakeConf.USE[0] == "ssl"
		}},
		{`PORTDIR="/usr/portage"`, func() bool {
			return cfg.MakeConf.PORTDIR == "/usr/portage"
		}},
	}

	for _, tt := range tests {
		cfg.parseMakeConfLine(tt.line)
		if !tt.checkVar() {
			t.Errorf("parseMakeConfLine(%s) failed", tt.line)
		}
	}
}

func TestLoadMakeConf(t *testing.T) {
	tmpDir := t.TempDir()

	// Create make.conf
	makeConfPath := filepath.Join(tmpDir, "make.conf")
	content := `# Test make.conf
CFLAGS="-O3 -pipe"
CXXFLAGS="${CFLAGS}"
MAKEOPTS="-j4"
USE="ssl -debug test"
PORTDIR="/usr/portage"
`

	if err := os.WriteFile(makeConfPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		Root:     tmpDir,
		MakeConf: DefaultMakeConf(),
	}

	if err := cfg.loadMakeConf(); err != nil {
		t.Fatalf("loadMakeConf() failed: %v", err)
	}

	if cfg.MakeConf.CFLAGS != "-O3 -pipe" {
		t.Errorf("CFLAGS = %s, expected -O3 -pipe", cfg.MakeConf.CFLAGS)
	}

	if cfg.MakeConf.MAKEOPTS != "-j4" {
		t.Errorf("MAKEOPTS = %s, expected -j4", cfg.MakeConf.MAKEOPTS)
	}

	if len(cfg.MakeConf.USE) != 3 {
		t.Errorf("USE flags count = %d, expected 3", len(cfg.MakeConf.USE))
	}
}

func TestGetPackageUSE(t *testing.T) {
	cfg := &Config{
		PackageUSE: map[string][]string{
			"sys-libs/zlib":   {"ssl", "-debug"},
			"app-editors/vim": {"python", "ruby"},
		},
	}

	// Test exact match
	useFlags := cfg.GetPackageUSE("sys-libs/zlib")
	if len(useFlags) != 2 {
		t.Errorf("GetPackageUSE() returned %d flags, expected 2", len(useFlags))
	}

	// Test non-existent package
	useFlags = cfg.GetPackageUSE("non/existent")
	if useFlags != nil {
		t.Error("GetPackageUSE() should return nil for non-existent package")
	}
}

func TestIsMasked(t *testing.T) {
	cfg := &Config{
		PackageMask: []string{
			">=sys-libs/glibc-2.35",
			"app-editors/vim-9.0",
		},
	}

	// Test exact match
	if !cfg.IsMasked("app-editors/vim-9.0") {
		t.Error("IsMasked() should return true for masked package")
	}

	// Test non-masked
	if cfg.IsMasked("sys-libs/zlib-1.2.13") {
		t.Error("IsMasked() should return false for non-masked package")
	}
}

func TestGetGlobalUSE(t *testing.T) {
	cfg := &Config{
		MakeConf: &MakeConf{
			USE: []string{"ssl", "python", "-debug"},
		},
	}

	globalUSE := cfg.GetGlobalUSE()
	if len(globalUSE) != 3 {
		t.Errorf("GetGlobalUSE() returned %d flags, expected 3", len(globalUSE))
	}

	if globalUSE[0] != "ssl" {
		t.Errorf("GetGlobalUSE()[0] = %s, expected ssl", globalUSE[0])
	}
}

func BenchmarkLoadConfig(b *testing.B) {
	tmpDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = LoadConfig(tmpDir)
	}
}

func BenchmarkGetPackageUSE(b *testing.B) {
	cfg := &Config{
		PackageUSE: map[string][]string{
			"sys-libs/zlib": {"ssl", "-debug"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cfg.GetPackageUSE("sys-libs/zlib")
	}
}

func TestGetGentooMirrors(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected []string
	}{
		{
			name:     "nil config returns nil",
			cfg:      &Config{MakeConf: nil},
			expected: nil,
		},
		{
			name:     "empty mirrors returns nil",
			cfg:      &Config{MakeConf: &MakeConf{GENTOO_MIRRORS: []string{}}},
			expected: nil,
		},
		{
			name: "configured mirrors are returned",
			cfg: &Config{
				MakeConf: &MakeConf{
					GENTOO_MIRRORS: []string{
						"https://gentoo.osuosl.org/",
						"https://mirrors.rit.edu/gentoo/",
					},
				},
			},
			expected: []string{
				"https://gentoo.osuosl.org/",
				"https://mirrors.rit.edu/gentoo/",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.GetGentooMirrors()
			if len(result) != len(tt.expected) {
				t.Errorf("GetGentooMirrors() returned %d mirrors, expected %d", len(result), len(tt.expected))
				return
			}
			for i, mirror := range result {
				if mirror != tt.expected[i] {
					t.Errorf("GetGentooMirrors()[%d] = %s, expected %s", i, mirror, tt.expected[i])
				}
			}
		})
	}
}

func TestGetGentooMirrorsImmutability(t *testing.T) {
	cfg := &Config{
		MakeConf: &MakeConf{
			GENTOO_MIRRORS: []string{"https://example.com/"},
		},
	}

	// Get mirrors and modify the returned slice
	mirrors := cfg.GetGentooMirrors()
	mirrors[0] = "https://modified.com/"

	// Original should be unchanged
	if cfg.MakeConf.GENTOO_MIRRORS[0] != "https://example.com/" {
		t.Error("GetGentooMirrors() should return a copy, not the original slice")
	}
}

func TestGetDistDir(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected string
	}{
		{
			name:     "nil MakeConf returns default",
			cfg:      &Config{MakeConf: nil},
			expected: "/var/cache/distfiles",
		},
		{
			name:     "empty DISTDIR returns default",
			cfg:      &Config{MakeConf: &MakeConf{DISTDIR: ""}},
			expected: "/var/cache/distfiles",
		},
		{
			name:     "configured DISTDIR is returned",
			cfg:      &Config{MakeConf: &MakeConf{DISTDIR: "/custom/distfiles"}},
			expected: "/custom/distfiles",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.GetDistDir()
			if result != tt.expected {
				t.Errorf("GetDistDir() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

func TestGetPortDir(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected string
	}{
		{
			name:     "nil MakeConf returns default",
			cfg:      &Config{MakeConf: nil},
			expected: "/var/db/repos/gentoo",
		},
		{
			name:     "empty PORTDIR returns default",
			cfg:      &Config{MakeConf: &MakeConf{PORTDIR: ""}},
			expected: "/var/db/repos/gentoo",
		},
		{
			name:     "configured PORTDIR is returned",
			cfg:      &Config{MakeConf: &MakeConf{PORTDIR: "/usr/portage"}},
			expected: "/usr/portage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.GetPortDir()
			if result != tt.expected {
				t.Errorf("GetPortDir() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

func TestGetMakeOpts(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected string
	}{
		{
			name:     "nil MakeConf returns default",
			cfg:      &Config{MakeConf: nil},
			expected: "-j1",
		},
		{
			name:     "empty MAKEOPTS returns default",
			cfg:      &Config{MakeConf: &MakeConf{MAKEOPTS: ""}},
			expected: "-j1",
		},
		{
			name:     "configured MAKEOPTS is returned",
			cfg:      &Config{MakeConf: &MakeConf{MAKEOPTS: "-j8 -l4"}},
			expected: "-j8 -l4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.GetMakeOpts()
			if result != tt.expected {
				t.Errorf("GetMakeOpts() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

func TestLoadMakeConfWithGentooMirrors(t *testing.T) {
	tmpDir := t.TempDir()

	// Create make.conf with GENTOO_MIRRORS
	makeConfPath := filepath.Join(tmpDir, "make.conf")
	content := `# Test make.conf with mirrors
GENTOO_MIRRORS="https://gentoo.osuosl.org/ https://mirrors.rit.edu/gentoo/"
DISTDIR="/custom/distfiles"
PORTDIR="/custom/portage"
MAKEOPTS="-j16"
`

	if err := os.WriteFile(makeConfPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Check GENTOO_MIRRORS
	mirrors := cfg.GetGentooMirrors()
	if len(mirrors) != 2 {
		t.Errorf("Expected 2 mirrors, got %d", len(mirrors))
	}
	if mirrors[0] != "https://gentoo.osuosl.org/" {
		t.Errorf("Mirror[0] = %s, expected https://gentoo.osuosl.org/", mirrors[0])
	}
	if mirrors[1] != "https://mirrors.rit.edu/gentoo/" {
		t.Errorf("Mirror[1] = %s, expected https://mirrors.rit.edu/gentoo/", mirrors[1])
	}

	// Check DISTDIR
	distDir := cfg.GetDistDir()
	if distDir != "/custom/distfiles" {
		t.Errorf("DISTDIR = %s, expected /custom/distfiles", distDir)
	}

	// Check PORTDIR
	portDir := cfg.GetPortDir()
	if portDir != "/custom/portage" {
		t.Errorf("PORTDIR = %s, expected /custom/portage", portDir)
	}

	// Check MAKEOPTS
	makeOpts := cfg.GetMakeOpts()
	if makeOpts != "-j16" {
		t.Errorf("MAKEOPTS = %s, expected -j16", makeOpts)
	}
}

// TestLoadPackageFile_SingleFile tests loading a single package.* file.
func TestLoadPackageFile_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package.use as a single file
	packageUsePath := filepath.Join(tmpDir, "package.use")
	content := `# Test package.use
sys-libs/zlib ssl -debug
app-editors/vim python ruby
dev-libs/openssl asm
`
	if err := os.WriteFile(packageUsePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Check sys-libs/zlib
	zlibFlags := cfg.GetPackageUSE("sys-libs/zlib")
	if len(zlibFlags) != 2 {
		t.Errorf("Expected 2 flags for zlib, got %d", len(zlibFlags))
	}

	// Check app-editors/vim
	vimFlags := cfg.GetPackageUSE("app-editors/vim")
	if len(vimFlags) != 2 {
		t.Errorf("Expected 2 flags for vim, got %d", len(vimFlags))
	}
}

// TestLoadPackageFile_Directory tests loading package.* from directory.
func TestLoadPackageFile_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package.use as a directory
	packageUseDir := filepath.Join(tmpDir, "package.use")
	if err := os.MkdirAll(packageUseDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create multiple files in the directory
	file1 := filepath.Join(packageUseDir, "libs")
	file1Content := `sys-libs/zlib ssl -debug
sys-libs/glibc hardened
`
	if err := os.WriteFile(file1, []byte(file1Content), 0644); err != nil {
		t.Fatal(err)
	}

	file2 := filepath.Join(packageUseDir, "editors")
	file2Content := `app-editors/vim python ruby
app-editors/emacs gtk
`
	if err := os.WriteFile(file2, []byte(file2Content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Check sys-libs/zlib
	zlibFlags := cfg.GetPackageUSE("sys-libs/zlib")
	if len(zlibFlags) != 2 {
		t.Errorf("Expected 2 flags for zlib, got %d", len(zlibFlags))
	}

	// Check app-editors/vim
	vimFlags := cfg.GetPackageUSE("app-editors/vim")
	if len(vimFlags) != 2 {
		t.Errorf("Expected 2 flags for vim, got %d", len(vimFlags))
	}

	// Check sys-libs/glibc
	glibcFlags := cfg.GetPackageUSE("sys-libs/glibc")
	if len(glibcFlags) != 1 {
		t.Errorf("Expected 1 flag for glibc, got %d", len(glibcFlags))
	}
}

// TestLoadPackageMask tests loading package.mask file.
func TestLoadPackageMask(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package.mask
	maskPath := filepath.Join(tmpDir, "package.mask")
	content := `# Test masks
>=sys-libs/glibc-2.35
app-editors/vim-9.0
# This is a comment
~sys-apps/systemd-250
`
	if err := os.WriteFile(maskPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Check masks are loaded
	if len(cfg.PackageMask) != 3 {
		t.Errorf("Expected 3 masks, got %d", len(cfg.PackageMask))
	}

	// Check specific mask
	if !cfg.IsMasked("app-editors/vim-9.0") {
		t.Error("app-editors/vim-9.0 should be masked")
	}
}

// TestLoadPackageAcceptKeywords tests loading package.accept_keywords.
func TestLoadPackageAcceptKeywords(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package.accept_keywords
	keywordsPath := filepath.Join(tmpDir, "package.accept_keywords")
	content := `sys-libs/zlib ~amd64
app-editors/vim ~amd64 ~x86
dev-libs/openssl **
`
	if err := os.WriteFile(keywordsPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Check keywords
	zlibKeywords := cfg.GetAcceptKeywords("sys-libs/zlib")
	if len(zlibKeywords) != 1 {
		t.Errorf("Expected 1 keyword for zlib, got %d", len(zlibKeywords))
	}

	vimKeywords := cfg.GetAcceptKeywords("app-editors/vim")
	if len(vimKeywords) != 2 {
		t.Errorf("Expected 2 keywords for vim, got %d", len(vimKeywords))
	}

	// Check non-existent package
	noKeywords := cfg.GetAcceptKeywords("non/existent")
	if noKeywords != nil {
		t.Error("Expected nil for non-existent package")
	}
}

// TestLoadPackageLicense tests loading package.license.
func TestLoadPackageLicense(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package.license
	licensePath := filepath.Join(tmpDir, "package.license")
	content := `app-misc/proprietary-app PROPRIETARY
dev-libs/oracle-java Oracle-BCLA-JavaSE
`
	if err := os.WriteFile(licensePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Check licenses
	if len(cfg.PackageLicense) != 2 {
		t.Errorf("Expected 2 license entries, got %d", len(cfg.PackageLicense))
	}
}

// TestParseMakeConfLine_AllVariables tests parsing all supported variables.
func TestParseMakeConfLine_AllVariables(t *testing.T) {
	cfg := &Config{
		MakeConf: DefaultMakeConf(),
	}

	tests := []struct {
		line  string
		check func() bool
		desc  string
	}{
		{`CFLAGS="-O3"`, func() bool { return cfg.MakeConf.CFLAGS == "-O3" }, "CFLAGS"},
		{`CXXFLAGS="-O2"`, func() bool { return cfg.MakeConf.CXXFLAGS == "-O2" }, "CXXFLAGS"},
		{`LDFLAGS="-Wl,-O1"`, func() bool { return cfg.MakeConf.LDFLAGS == "-Wl,-O1" }, "LDFLAGS"},
		{`MAKEOPTS="-j8"`, func() bool { return cfg.MakeConf.MAKEOPTS == "-j8" }, "MAKEOPTS"},
		{`USE="ssl"`, func() bool { return len(cfg.MakeConf.USE) == 1 && cfg.MakeConf.USE[0] == "ssl" }, "USE"},
		{`ACCEPT_KEYWORDS="~amd64"`, func() bool { return len(cfg.MakeConf.ACCEPT_KEYWORDS) == 1 }, "ACCEPT_KEYWORDS"},
		{`ACCEPT_LICENSE="*"`, func() bool { return len(cfg.MakeConf.ACCEPT_LICENSE) == 1 }, "ACCEPT_LICENSE"},
		{`FEATURES="sandbox"`, func() bool { return len(cfg.MakeConf.FEATURES) == 1 }, "FEATURES"},
		{`PORTDIR="/custom"`, func() bool { return cfg.MakeConf.PORTDIR == "/custom" }, "PORTDIR"},
		{`DISTDIR="/dist"`, func() bool { return cfg.MakeConf.DISTDIR == "/dist" }, "DISTDIR"},
		{`PKGDIR="/pkg"`, func() bool { return cfg.MakeConf.PKGDIR == "/pkg" }, "PKGDIR"},
		{`PORT_LOGDIR="/log"`, func() bool { return cfg.MakeConf.PORT_LOGDIR == "/log" }, "PORT_LOGDIR"},
		{`PORTAGE_TMPDIR="/tmp"`, func() bool { return cfg.MakeConf.PORTAGE_TMPDIR == "/tmp" }, "PORTAGE_TMPDIR"},
		{`GENTOO_MIRRORS="http://m1 http://m2"`, func() bool { return len(cfg.MakeConf.GENTOO_MIRRORS) == 2 }, "GENTOO_MIRRORS"},
	}

	for _, tt := range tests {
		// Reset MakeConf
		cfg.MakeConf = DefaultMakeConf()
		cfg.parseMakeConfLine(tt.line)

		if !tt.check() {
			t.Errorf("parseMakeConfLine(%s) failed for %s", tt.line, tt.desc)
		}
	}
}

// TestParseMakeConfLine_InvalidLine tests parsing invalid lines.
func TestParseMakeConfLine_InvalidLine(t *testing.T) {
	cfg := &Config{
		MakeConf: DefaultMakeConf(),
	}

	originalCFLAGS := cfg.MakeConf.CFLAGS

	// No equals sign
	cfg.parseMakeConfLine("INVALID_LINE")

	// CFLAGS should not change
	if cfg.MakeConf.CFLAGS != originalCFLAGS {
		t.Error("parseMakeConfLine should ignore invalid lines")
	}
}

// TestParseMakeConfLine_QuoteVariants tests various quote styles.
func TestParseMakeConfLine_QuoteVariants(t *testing.T) {
	tests := []struct {
		line     string
		expected string
	}{
		{`CFLAGS="-O2"`, "-O2"},
		{`CFLAGS='-O2'`, "-O2"},
		{`CFLAGS=-O2`, "-O2"},
		{`CFLAGS=" -O2 "`, " -O2 "}, // Whitespace inside quotes preserved after trim
	}

	for _, tt := range tests {
		cfg := &Config{MakeConf: DefaultMakeConf()}
		cfg.parseMakeConfLine(tt.line)

		// Trim quotes from expected
		expected := tt.expected
		if cfg.MakeConf.CFLAGS != expected {
			t.Errorf("parseMakeConfLine(%s): CFLAGS = %q, want %q", tt.line, cfg.MakeConf.CFLAGS, expected)
		}
	}
}

// TestConfig_Root tests Config.Root field.
func TestConfig_Root(t *testing.T) {
	tmpDir := t.TempDir()

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if cfg.Root != tmpDir {
		t.Errorf("Config.Root = %s, want %s", cfg.Root, tmpDir)
	}
}

// TestDefaultMakeConf_AllFields tests all default values.
func TestDefaultMakeConf_AllFields(t *testing.T) {
	mc := DefaultMakeConf()

	if mc.CFLAGS != "-O2 -pipe" {
		t.Errorf("CFLAGS = %s", mc.CFLAGS)
	}
	if mc.CXXFLAGS != "${CFLAGS}" {
		t.Errorf("CXXFLAGS = %s", mc.CXXFLAGS)
	}
	if mc.LDFLAGS != "" {
		t.Errorf("LDFLAGS = %s", mc.LDFLAGS)
	}
	if mc.MAKEOPTS != "-j1" {
		t.Errorf("MAKEOPTS = %s", mc.MAKEOPTS)
	}
	if mc.PORTDIR != "/var/db/repos/gentoo" {
		t.Errorf("PORTDIR = %s", mc.PORTDIR)
	}
	if mc.DISTDIR != "/var/cache/distfiles" {
		t.Errorf("DISTDIR = %s", mc.DISTDIR)
	}
	if mc.PKGDIR != "/var/cache/binpkgs" {
		t.Errorf("PKGDIR = %s", mc.PKGDIR)
	}
	if mc.PORT_LOGDIR != "/var/log/portage" {
		t.Errorf("PORT_LOGDIR = %s", mc.PORT_LOGDIR)
	}
	if mc.PORTAGE_TMPDIR != "/var/tmp/portage" {
		t.Errorf("PORTAGE_TMPDIR = %s", mc.PORTAGE_TMPDIR)
	}
}

// TestLoadConfig_EmptyMaps tests that maps are initialized.
func TestLoadConfig_EmptyMaps(t *testing.T) {
	tmpDir := t.TempDir()

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if cfg.PackageUSE == nil {
		t.Error("PackageUSE should not be nil")
	}
	if cfg.PackageAcceptKeywords == nil {
		t.Error("PackageAcceptKeywords should not be nil")
	}
	if cfg.PackageLicense == nil {
		t.Error("PackageLicense should not be nil")
	}
	if cfg.PackageMask == nil {
		t.Error("PackageMask should not be nil")
	}
}

// TestParsePackageFile_EmptyFile tests parsing empty file.
func TestParsePackageFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty package.use
	packageUsePath := filepath.Join(tmpDir, "package.use")
	if err := os.WriteFile(packageUsePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Should have empty PackageUSE map
	if len(cfg.PackageUSE) != 0 {
		t.Errorf("Expected empty PackageUSE, got %d entries", len(cfg.PackageUSE))
	}
}

// TestParsePackageFile_CommentsOnly tests parsing file with only comments.
func TestParsePackageFile_CommentsOnly(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package.use with only comments
	packageUsePath := filepath.Join(tmpDir, "package.use")
	content := `# This is a comment
# Another comment
   # Indented comment
`
	if err := os.WriteFile(packageUsePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if len(cfg.PackageUSE) != 0 {
		t.Errorf("Expected empty PackageUSE, got %d entries", len(cfg.PackageUSE))
	}
}

// TestParsePackageFile_SingleColumn tests lines with only atom (no flags).
func TestParsePackageFile_SingleColumn(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package.use with single column (atom only - should be ignored)
	packageUsePath := filepath.Join(tmpDir, "package.use")
	content := `sys-libs/zlib
`
	if err := os.WriteFile(packageUsePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Single column lines are ignored
	if len(cfg.PackageUSE) != 0 {
		t.Errorf("Expected empty PackageUSE for single column, got %d entries", len(cfg.PackageUSE))
	}
}

// TestPackageUSE_MultipleEntries tests appending USE flags from multiple entries.
func TestPackageUSE_MultipleEntries(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package.use with multiple entries for same package
	packageUsePath := filepath.Join(tmpDir, "package.use")
	content := `sys-libs/zlib ssl
sys-libs/zlib -debug
`
	if err := os.WriteFile(packageUsePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	flags := cfg.GetPackageUSE("sys-libs/zlib")
	if len(flags) != 2 {
		t.Errorf("Expected 2 flags, got %d", len(flags))
	}
}
