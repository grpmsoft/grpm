package cli

import (
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/config"
)

func TestGatherSystemInfo_ReturnsValidInfo(t *testing.T) {
	cfg := &config.Config{
		Root: "/etc/portage",
		MakeConf: &config.MakeConf{
			CFLAGS:          "-O2 -pipe",
			CXXFLAGS:        "-O2 -pipe",
			LDFLAGS:         "-Wl,-O1",
			MAKEOPTS:        "-j4",
			USE:             []string{"X", "alsa", "pulseaudio"},
			ACCEPT_KEYWORDS: []string{"amd64"},
			ACCEPT_LICENSE:  []string{"*"},
			FEATURES:        []string{"sandbox", "userpriv"},
		},
	}

	info := GatherSystemInfo("0.8.2", cfg, "/var/db/repos/gentoo")

	// Verify GRPM version
	if info.GRPMVersion != "0.8.2" {
		t.Errorf("GRPMVersion = %q, want %q", info.GRPMVersion, "0.8.2")
	}

	// Verify Go version is not empty
	if info.GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}

	// Verify Platform is not empty
	if info.Platform == "" {
		t.Error("Platform should not be empty")
	}

	// Verify ConfigVars are populated
	if info.ConfigVars["CFLAGS"] != "-O2 -pipe" {
		t.Errorf("CFLAGS = %q, want %q", info.ConfigVars["CFLAGS"], "-O2 -pipe")
	}
	if info.ConfigVars["USE"] != "X alsa pulseaudio" {
		t.Errorf("USE = %q, want %q", info.ConfigVars["USE"], "X alsa pulseaudio")
	}
	if info.ConfigVars["ACCEPT_KEYWORDS"] != "amd64" {
		t.Errorf("ACCEPT_KEYWORDS = %q, want %q", info.ConfigVars["ACCEPT_KEYWORDS"], "amd64")
	}
}

func TestFormatSystemInfo_MatchesPortageFormat(t *testing.T) {
	info := &SystemInfo{
		GRPMVersion: "0.8.2",
		GoVersion:   "go1.25.0",
		Platform:    "linux-amd64",
		Uname:       "Linux-6.6.87-x86_64",
		Memory: MemoryInfo{
			Total: 8133872 * 1024, // Convert to bytes
			Free:  6836952 * 1024,
		},
		InstalledPkgs: []InstalledPkg{
			{CP: "sys-devel/gcc", Version: "14.3.0"},
			{CP: "sys-libs/glibc", Version: "2.40-r8"},
		},
		Repositories: []RepoInfo{
			{Name: "gentoo", Location: "/var/db/repos/gentoo", SyncType: "rsync"},
		},
		ConfigVars: map[string]string{
			"CFLAGS":          "-O2 -pipe",
			"CXXFLAGS":        "-O2 -pipe",
			"USE":             "X alsa pulseaudio",
			"ACCEPT_KEYWORDS": "amd64",
		},
	}

	output := FormatSystemInfo(info)

	// Check header line
	if !strings.HasPrefix(output, "GRPM 0.8.2 (go1.25.0, linux-amd64)") {
		t.Errorf("Output should start with version header, got: %s", strings.Split(output, "\n")[0])
	}

	// Check separator line
	if !strings.Contains(output, strings.Repeat("=", 65)) {
		t.Error("Output should contain separator line")
	}

	// Check uname
	if !strings.Contains(output, "System uname: Linux-6.6.87-x86_64") {
		t.Error("Output should contain System uname")
	}

	// Check memory info
	if !strings.Contains(output, "KiB Mem:") {
		t.Error("Output should contain memory info")
	}

	// Check installed packages
	if !strings.Contains(output, "sys-devel/gcc:") {
		t.Error("Output should contain gcc")
	}
	if !strings.Contains(output, "14.3.0") {
		t.Error("Output should contain gcc version")
	}

	// Check repositories
	if !strings.Contains(output, "Repositories:") {
		t.Error("Output should contain Repositories section")
	}
	if !strings.Contains(output, "gentoo") {
		t.Error("Output should contain repository name")
	}

	// Check config vars
	if !strings.Contains(output, `CFLAGS="-O2 -pipe"`) {
		t.Error("Output should contain CFLAGS")
	}
	if !strings.Contains(output, `USE="X alsa pulseaudio"`) {
		t.Error("Output should contain USE")
	}
}

func TestFormatSystemInfo_NoMemoryOnNonLinux(t *testing.T) {
	info := &SystemInfo{
		GRPMVersion: "0.8.2",
		GoVersion:   "go1.25.0",
		Platform:    "darwin-amd64",
		Memory:      MemoryInfo{}, // Empty memory info
		ConfigVars:  map[string]string{},
	}

	output := FormatSystemInfo(info)

	// Should not contain memory section
	if strings.Contains(output, "KiB Mem:") {
		t.Error("Non-Linux output should not contain memory info")
	}
}

func TestFormatSystemInfo_NotInstalledPackages(t *testing.T) {
	info := &SystemInfo{
		GRPMVersion: "0.8.2",
		GoVersion:   "go1.25.0",
		Platform:    "linux-amd64",
		InstalledPkgs: []InstalledPkg{
			{CP: "sys-devel/gcc", Version: "14.3.0"},
			{CP: "dev-lang/rust", Version: ""}, // Not installed
		},
		ConfigVars: map[string]string{},
	}

	output := FormatSystemInfo(info)

	// Installed package should show version
	if !strings.Contains(output, "14.3.0") {
		t.Error("Output should contain gcc version")
	}

	// Not installed package should show [Not installed]
	if !strings.Contains(output, "[Not installed]") {
		t.Error("Output should show [Not installed] for missing packages")
	}
}

func TestParseInt64(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"123", 123},
		{"0", 0},
		{"8133872", 8133872},
		{"abc", 0},
		{"123abc", 123},
		{"", 0},
	}

	for _, tc := range tests {
		got := parseInt64(tc.input)
		if got != tc.want {
			t.Errorf("parseInt64(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}
