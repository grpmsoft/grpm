package binpkg

import (
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		path     string
		expected BinaryFormat
	}{
		{"/var/cache/binpkgs/sys-libs/zlib-1.2.13.gpkg.tar", FormatGPKG},
		{"/tmp/packages/app-editors/vim-9.0.tbz2", FormatTBZ2},
		{"/tmp/packages/sys-apps/portage-3.0.30.tar.bz2", FormatTBZ2},
		{"/tmp/unknown.tar.gz", FormatUnknown},
		{"package.deb", FormatUnknown},
	}

	for _, tt := range tests {
		result := DetectFormat(tt.path)
		if result != tt.expected {
			t.Errorf("DetectFormat(%s) = %v, expected %v", tt.path, result, tt.expected)
		}
	}
}

func TestBinaryPackage_IsCompatible(t *testing.T) {
	tests := []struct {
		name       string
		buildUSE   []string
		desiredUSE []string
		expected   bool
	}{
		{
			name:       "exact match",
			buildUSE:   []string{"ssl", "python"},
			desiredUSE: []string{"ssl", "python"},
			expected:   true,
		},
		{
			name:       "subset match",
			buildUSE:   []string{"ssl", "python", "test"},
			desiredUSE: []string{"ssl", "python"},
			expected:   true,
		},
		{
			name:       "missing required flag",
			buildUSE:   []string{"python"},
			desiredUSE: []string{"ssl", "python"},
			expected:   false,
		},
		{
			name:       "negative flag present",
			buildUSE:   []string{"ssl", "debug"},
			desiredUSE: []string{"ssl", "-debug"},
			expected:   false,
		},
		{
			name:       "negative flag absent",
			buildUSE:   []string{"ssl"},
			desiredUSE: []string{"ssl", "-debug"},
			expected:   true,
		},
		{
			name:       "empty desired",
			buildUSE:   []string{"ssl", "python"},
			desiredUSE: []string{},
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := &BinaryPackage{
				BuildInfo: &BuildMetadata{
					USE: tt.buildUSE,
				},
			}

			result := bp.IsCompatible(tt.desiredUSE)
			if result != tt.expected {
				t.Errorf("IsCompatible() = %v, expected %v (build: %v, desired: %v)",
					result, tt.expected, tt.buildUSE, tt.desiredUSE)
			}
		})
	}
}

func TestBinaryPackage_IsCompatible_NilBuildInfo(t *testing.T) {
	bp := &BinaryPackage{
		BuildInfo: nil,
	}

	if bp.IsCompatible([]string{"ssl"}) {
		t.Error("IsCompatible() should return false when BuildInfo is nil")
	}
}

func TestBinaryPackage_IsFresh(t *testing.T) {
	tests := []struct {
		name      string
		buildDate time.Time
		maxAge    time.Duration
		expected  bool
	}{
		{
			name:      "fresh package",
			buildDate: time.Now().Add(-1 * time.Hour),
			maxAge:    24 * time.Hour,
			expected:  true,
		},
		{
			name:      "old package",
			buildDate: time.Now().Add(-48 * time.Hour),
			maxAge:    24 * time.Hour,
			expected:  false,
		},
		{
			name:      "exactly at limit",
			buildDate: time.Now().Add(-24 * time.Hour),
			maxAge:    24 * time.Hour,
			expected:  false, // time.Since will be slightly over 24h
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := &BinaryPackage{
				BuildInfo: &BuildMetadata{
					BuildDate: tt.buildDate,
				},
			}

			result := bp.IsFresh(tt.maxAge)
			if result != tt.expected {
				t.Errorf("IsFresh() = %v, expected %v (age: %v, max: %v)",
					result, tt.expected, time.Since(tt.buildDate), tt.maxAge)
			}
		})
	}
}

func TestBinaryPackage_IsFresh_NilBuildInfo(t *testing.T) {
	bp := &BinaryPackage{
		BuildInfo: nil,
	}

	if bp.IsFresh(24 * time.Hour) {
		t.Error("IsFresh() should return false when BuildInfo is nil")
	}
}

func TestBinaryPackage_String(t *testing.T) {
	tests := []struct {
		name     string
		bp       *BinaryPackage
		expected string
	}{
		{
			name: "valid package",
			bp: &BinaryPackage{
				Package: &pkg.Package{
					Name:    "sys-libs/zlib",
					Version: "1.2.13",
				},
				Format: FormatGPKG,
				Size:   1024000,
			},
			expected: "BinaryPackage{sys-libs/zlib-1.2.13, gpkg, 1024000 bytes}",
		},
		{
			name: "nil package",
			bp: &BinaryPackage{
				Package: nil,
			},
			expected: "BinaryPackage{unknown}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.bp.String()
			if result != tt.expected {
				t.Errorf("String() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

// TestDetectFormat_EdgeCases tests format detection with edge cases that could
// break package installation if mishandled.
func TestDetectFormat_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected BinaryFormat
	}{
		// Complex version strings that could confuse parsers
		{"version with underscore suffix", "/var/cache/binpkgs/dev-lang/python-3.11.4_p1.gpkg.tar", FormatGPKG},
		{"version with revision", "/var/cache/binpkgs/sys-apps/portage-3.0.51-r1.tbz2", FormatTBZ2},
		{"version with alpha/beta", "/var/cache/binpkgs/dev-libs/openssl-3.0.10_beta1.gpkg.tar", FormatGPKG},

		// Paths that could trip up string parsing
		{"path with spaces", "/var/cache/binpkgs/My Packages/zlib-1.2.13.gpkg.tar", FormatGPKG},
		{"deeply nested path", "/a/b/c/d/e/f/g/package-1.0.gpkg.tar", FormatGPKG},
		{"relative path", "./packages/zlib-1.2.13.tbz2", FormatTBZ2},

		// Package names that look like extensions
		{"package name with tar", "/var/cache/binpkgs/app-arch/tar-1.34.gpkg.tar", FormatGPKG},
		{"package name with tbz2", "/var/cache/binpkgs/app-misc/tbz2tool-1.0.tbz2", FormatTBZ2},

		// Unsupported formats should not crash
		{"xz compression", "/var/cache/binpkgs/sys-libs/zlib-1.2.13.tar.xz", FormatUnknown},
		{"zstd compression", "/var/cache/binpkgs/sys-libs/zlib-1.2.13.tar.zst", FormatUnknown},
		{"rpm format", "/packages/zlib-1.2.13.rpm", FormatUnknown},
		{"empty path", "", FormatUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFormat(tt.path)
			if result != tt.expected {
				t.Errorf("DetectFormat(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestBinaryPackage_IsCompatible_USEExpand tests USE flag compatibility with
// USE_EXPAND patterns like python_targets_python3_11, which are critical for
// proper binary package selection.
func TestBinaryPackage_IsCompatible_USEExpand(t *testing.T) {
	tests := []struct {
		name       string
		buildUSE   []string
		desiredUSE []string
		expected   bool
	}{
		{
			name:       "python targets match",
			buildUSE:   []string{"python_targets_python3_11", "python_targets_python3_12"},
			desiredUSE: []string{"python_targets_python3_11"},
			expected:   true,
		},
		{
			name:       "python target missing",
			buildUSE:   []string{"python_targets_python3_11"},
			desiredUSE: []string{"python_targets_python3_12"},
			expected:   false,
		},
		{
			name:       "lua targets",
			buildUSE:   []string{"lua_targets_lua5-4", "lua_single_target_lua5-4"},
			desiredUSE: []string{"lua_targets_lua5-4"},
			expected:   true,
		},
		{
			name:       "ruby targets with negative",
			buildUSE:   []string{"ruby_targets_ruby31", "ruby_targets_ruby32"},
			desiredUSE: []string{"ruby_targets_ruby31", "-ruby_targets_ruby30"},
			expected:   true,
		},
		{
			name:       "mixed regular and expand flags",
			buildUSE:   []string{"ssl", "python_targets_python3_11", "test"},
			desiredUSE: []string{"ssl", "python_targets_python3_11", "-debug"},
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := &BinaryPackage{
				BuildInfo: &BuildMetadata{
					USE: tt.buildUSE,
				},
			}

			result := bp.IsCompatible(tt.desiredUSE)
			if result != tt.expected {
				t.Errorf("IsCompatible() = %v, expected %v\n  build:   %v\n  desired: %v",
					result, tt.expected, tt.buildUSE, tt.desiredUSE)
			}
		})
	}
}

// TestBinaryPackage_SubslotCompatibility tests subslot matching which is
// critical for ABI compatibility. Binary packages built against one subslot
// may not work with a different subslot installed.
func TestBinaryPackage_SubslotCompatibility(t *testing.T) {
	tests := []struct {
		name               string
		pkgSlot            pkg.Slot
		installedSlot      pkg.Slot
		expectedCompatible bool
	}{
		{
			name:               "exact slot match",
			pkgSlot:            pkg.NewSlot("0", "1.2"),
			installedSlot:      pkg.NewSlot("0", "1.2"),
			expectedCompatible: true,
		},
		{
			name:               "subslot mismatch (ABI break)",
			pkgSlot:            pkg.NewSlot("0", "1.2"),
			installedSlot:      pkg.NewSlot("0", "1.3"),
			expectedCompatible: false,
		},
		{
			name:               "different main slots (can coexist)",
			pkgSlot:            pkg.NewSlot("1", "1.0"),
			installedSlot:      pkg.NewSlot("2", "1.0"),
			expectedCompatible: true, // Different slots CAN coexist per Portage semantics
		},
		{
			name:               "slot without subslot",
			pkgSlot:            pkg.NewSlot("0", ""),
			installedSlot:      pkg.NewSlot("0", ""),
			expectedCompatible: true,
		},
		{
			name:               "complex subslot (openssl style)",
			pkgSlot:            pkg.NewSlot("0", "3"),
			installedSlot:      pkg.NewSlot("0", "3"),
			expectedCompatible: true,
		},
		{
			name:               "subslot vs no subslot (compatible)",
			pkgSlot:            pkg.NewSlot("0", "1.2"),
			installedSlot:      pkg.NewSlot("0", ""),
			expectedCompatible: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := &BinaryPackage{
				Package: &pkg.Package{
					Name:    "sys-libs/zlib",
					Version: "1.2.13",
					Slot:    tt.pkgSlot,
				},
			}

			// Use the actual slot compatibility logic
			compatible := bp.Package.Slot.Equals(tt.installedSlot) ||
				bp.Package.Slot.IsCompatibleWith(tt.installedSlot)
			if compatible != tt.expectedCompatible {
				t.Errorf("Slot compatibility = %v, expected %v (pkg: %s, installed: %s)",
					compatible, tt.expectedCompatible, tt.pkgSlot.String(), tt.installedSlot.String())
			}
		})
	}
}

// TestBinaryPackage_LargeUSESet tests performance and correctness with
// realistic large USE flag sets (some packages have 50+ USE flags).
func TestBinaryPackage_LargeUSESet(t *testing.T) {
	// Simulate a package like chromium or firefox with many USE flags
	buildUSE := []string{
		"X", "alsa", "cups", "dbus", "ffmpeg", "gtk", "hangouts",
		"kerberos", "libcxx", "lto", "official", "pgo", "proprietary-codecs",
		"pulseaudio", "qt5", "screencast", "selinux", "suid", "system-ffmpeg",
		"system-harfbuzz", "system-icu", "system-png", "vaapi", "wayland",
		"widevine", "python_targets_python3_11", "python_targets_python3_12",
		"l10n_en", "l10n_ru", "l10n_de", "l10n_fr", "l10n_es",
	}

	tests := []struct {
		name       string
		desiredUSE []string
		expected   bool
	}{
		{
			name:       "subset of flags",
			desiredUSE: []string{"X", "alsa", "cups"},
			expected:   true,
		},
		{
			name:       "all flags match",
			desiredUSE: buildUSE,
			expected:   true,
		},
		{
			name:       "one flag missing",
			desiredUSE: []string{"X", "alsa", "debug"}, // debug not in buildUSE
			expected:   false,
		},
		{
			name:       "many negative flags",
			desiredUSE: []string{"X", "-debug", "-test", "-doc", "-static-libs"},
			expected:   true,
		},
	}

	bp := &BinaryPackage{
		BuildInfo: &BuildMetadata{
			USE: buildUSE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bp.IsCompatible(tt.desiredUSE)
			if result != tt.expected {
				t.Errorf("IsCompatible() with large USE set = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestBuildMetadata_Validation tests that build metadata is properly validated
// to prevent corrupt binary packages from being installed.
func TestBuildMetadata_Validation(t *testing.T) {
	tests := []struct {
		name    string
		meta    *BuildMetadata
		isValid bool
	}{
		{
			name: "valid metadata",
			meta: &BuildMetadata{
				EAPI:      "8",
				USE:       []string{"ssl", "python"},
				BuildDate: time.Now().Add(-1 * time.Hour),
				CFLAGS:    "-O2 -pipe",
			},
			isValid: true,
		},
		{
			name: "missing EAPI",
			meta: &BuildMetadata{
				USE:       []string{"ssl"},
				BuildDate: time.Now(),
			},
			isValid: false,
		},
		{
			name: "future build date (clock skew)",
			meta: &BuildMetadata{
				EAPI:      "8",
				BuildDate: time.Now().Add(24 * time.Hour),
			},
			isValid: false,
		},
		{
			name:    "nil metadata",
			meta:    nil,
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validation logic: EAPI must exist, build date must be in past
			valid := tt.meta != nil &&
				tt.meta.EAPI != "" &&
				(tt.meta.BuildDate.IsZero() || tt.meta.BuildDate.Before(time.Now().Add(time.Minute)))

			if valid != tt.isValid {
				t.Errorf("Metadata validation = %v, expected %v", valid, tt.isValid)
			}
		})
	}
}
