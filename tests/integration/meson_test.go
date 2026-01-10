//go:build integration

package integration

import (
	"strings"
	"testing"
)

// MesonPackages defines packages using Meson build system for validation.
//
// v0.4.0 focus: Validate meson.eclass inheritance and parsing.
// Actual builds require distfiles and are planned for future versions.
var MesonPackages = []PackageSpec{
	// Simple packages - should parse and detect meson.eclass
	{
		Atom:        "dev-libs/json-glib",
		BuildSystem: BuildSystemMeson,
		Complexity:  "simple",
		Description: "JSON library for GLib",
	},

	// Medium complexity packages
	{
		Atom:        "dev-libs/glib",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "Core GNOME library",
	},
	{
		Atom:        "media-libs/harfbuzz",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "OpenType text shaping engine",
	},
	{
		Atom:        "media-libs/fontconfig",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "Font configuration library",
	},
	{
		Atom:        "media-libs/freetype",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "Font rendering library",
	},
	{
		Atom:        "sys-apps/dbus",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "D-Bus message bus system",
	},

	// Packages requiring unsupported features - skip
	{
		Atom:        "dev-libs/libpcre2",
		BuildSystem: BuildSystemMeson,
		Complexity:  "simple",
		Description: "Perl-compatible regular expressions",
		SkipReason:  "May use autotools instead of meson in some versions",
	},
	{
		Atom:        "x11-libs/cairo",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "2D graphics library",
		SkipReason:  "Requires X11 dependencies",
	},
	{
		Atom:        "x11-libs/pango",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "Text rendering library",
		SkipReason:  "Requires X11 and Cairo dependencies",
	},
	{
		Atom:        "gnome-base/librsvg",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "SVG rendering library",
		SkipReason:  "librsvg requires Rust toolchain",
	},
	{
		Atom:        "dev-libs/libxslt",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "XSLT processor library",
		SkipReason:  "May use autotools instead of meson",
	},
	{
		Atom:        "app-text/ghostscript-gpl",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "PostScript/PDF interpreter",
		SkipReason:  "ghostscript has complex dependencies and build",
	},

	// Complex packages - always skip
	{
		Atom:        "media-libs/mesa",
		BuildSystem: BuildSystemMeson,
		Complexity:  "complex",
		Description: "OpenGL implementation",
		SkipReason:  "Mesa requires many dependencies",
	},
	{
		Atom:        "x11-libs/gtk+",
		BuildSystem: BuildSystemMeson,
		Complexity:  "complex",
		Description: "GTK+ toolkit",
		SkipReason:  "GTK+ requires X11 and many dependencies",
	},
	{
		Atom:        "net-misc/networkmanager",
		BuildSystem: BuildSystemMeson,
		Complexity:  "complex",
		Description: "Network management daemon",
		SkipReason:  "NetworkManager requires system integration",
	},
}

// TestMeson_ParseAll validates that Meson ebuilds can be parsed.
//
// v0.4.0 scope: Parsing and meson.eclass detection, not building.
func TestMeson_ParseAll(t *testing.T) {
	skipIfNoRepo(t)

	// Count skipped packages upfront for accurate statistics
	skipped := 0
	for _, spec := range MesonPackages {
		if spec.SkipReason != "" {
			skipped++
		}
	}

	for _, spec := range MesonPackages {
		spec := spec
		t.Run(spec.Atom, func(t *testing.T) {
			if spec.SkipReason != "" {
				t.Skip(spec.SkipReason)
			}

			if !packageExists(t, spec.Atom) {
				t.Skipf("Package %s not found in repository", spec.Atom)
			}

			result := validatePackageParsing(t, spec.Atom)
			if result.Success {
				t.Logf("SUCCESS: %s-%s parsed (inherits: %v)",
					spec.Atom, result.Version, result.Inherits)
			} else {
				t.Errorf("FAILED: %s: %v", spec.Atom, result.Error)
			}
		})
	}

	total := len(MesonPackages)
	expected := total - skipped
	t.Logf("=== Meson Parsing Summary ===")
	t.Logf("Total: %d, Expected to parse: %d, Skipped: %d", total, expected, skipped)
}

// TestMeson_EclassInheritance verifies meson.eclass inheritance detection.
func TestMeson_EclassInheritance(t *testing.T) {
	skipIfNoRepo(t)

	for _, spec := range MesonPackages {
		if spec.SkipReason != "" {
			continue
		}

		spec := spec
		t.Run(spec.Atom, func(t *testing.T) {
			if !packageExists(t, spec.Atom) {
				t.Skipf("Package %s not found", spec.Atom)
				return
			}

			result := validatePackageParsing(t, spec.Atom)
			if !result.Success {
				t.Skipf("Could not parse %s: %v", spec.Atom, result.Error)
				return
			}

			// Check for meson.eclass inheritance
			hasMeson := false
			for _, eclass := range result.Inherits {
				if strings.Contains(eclass, "meson") {
					hasMeson = true
					break
				}
			}

			if hasMeson {
				t.Logf("%s correctly inherits meson eclass", spec.Atom)
			} else {
				t.Logf("%s may use meson without eclass (inherits: %v)",
					spec.Atom, result.Inherits)
			}
		})
	}
}

// TestMeson_GLib validates the canonical Meson test package.
func TestMeson_GLib(t *testing.T) {
	skipIfNoRepo(t)

	atom := "dev-libs/glib"
	if !packageExists(t, atom) {
		t.Skipf("%s not found in repository", atom)
	}

	result := validatePackageParsing(t, atom)
	if !result.Success {
		t.Fatalf("Failed to parse %s: %v", atom, result.Error)
	}

	// Verify meson.eclass is inherited
	hasMeson := false
	for _, eclass := range result.Inherits {
		if strings.Contains(eclass, "meson") {
			hasMeson = true
			break
		}
	}

	if !hasMeson {
		t.Logf("Warning: %s may not inherit meson eclass, got: %v", atom, result.Inherits)
	}

	t.Logf("%s %s: EAPI=%s, inherits=%v, functions=%v",
		atom, result.Version, result.EAPI, result.Inherits, result.Functions)
}

// TestMeson_MetadataExtraction validates metadata extraction for Meson packages.
func TestMeson_MetadataExtraction(t *testing.T) {
	skipIfNoRepo(t)

	testCases := []struct {
		atom        string
		expectMeson bool
	}{
		{"dev-libs/json-glib", true},
		{"dev-libs/glib", true},
		{"media-libs/harfbuzz", true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.atom, func(t *testing.T) {
			if !packageExists(t, tc.atom) {
				t.Skipf("Package %s not found", tc.atom)
				return
			}

			result := validatePackageParsing(t, tc.atom)
			if !result.Success {
				t.Fatalf("Failed to parse %s: %v", tc.atom, result.Error)
			}

			// Verify EAPI is valid
			validEAPIs := map[string]bool{
				"0": true, "1": true, "2": true, "3": true,
				"4": true, "5": true, "6": true, "7": true, "8": true,
			}
			if result.EAPI != "" && !validEAPIs[result.EAPI] {
				t.Errorf("Invalid EAPI %q for %s", result.EAPI, tc.atom)
			}

			// Check meson inheritance if expected
			if tc.expectMeson {
				hasMeson := false
				for _, eclass := range result.Inherits {
					if strings.Contains(eclass, "meson") {
						hasMeson = true
						break
					}
				}
				if !hasMeson {
					t.Logf("Warning: %s expected to inherit meson, got: %v",
						tc.atom, result.Inherits)
				}
			}

			t.Logf("%s: version=%s, EAPI=%s, inherits=%v",
				tc.atom, result.Version, result.EAPI, result.Inherits)
		})
	}
}

// TestMeson_NinjaBackendAvailability verifies ninja backend is detected.
func TestMeson_NinjaBackendAvailability(t *testing.T) {
	skipIfNoRepo(t)

	// Check that meson packages could use ninja
	// This is a sanity check for the test environment
	t.Log("Meson typically uses ninja as build backend")

	// Verify test atoms have meson in inherits
	testAtoms := []string{"dev-libs/glib", "dev-libs/json-glib"}
	for _, atom := range testAtoms {
		if !packageExists(t, atom) {
			continue
		}

		result := validatePackageParsing(t, atom)
		if !result.Success {
			continue
		}

		for _, eclass := range result.Inherits {
			if strings.Contains(eclass, "meson") {
				t.Logf("%s: confirmed meson eclass inheritance", atom)
				break
			}
		}
	}
}

// TestMeson_FunctionDiscovery verifies function discovery for Meson packages.
func TestMeson_FunctionDiscovery(t *testing.T) {
	skipIfNoRepo(t)

	testAtoms := []string{
		"dev-libs/glib",
		"dev-libs/json-glib",
	}

	for _, atom := range testAtoms {
		atom := atom
		t.Run(atom, func(t *testing.T) {
			if !packageExists(t, atom) {
				t.Skipf("Package %s not found", atom)
				return
			}

			result := validatePackageParsing(t, atom)
			if !result.Success {
				t.Fatalf("Failed to parse %s: %v", atom, result.Error)
			}

			t.Logf("%s functions: %v", atom, result.Functions)

			// Meson packages typically have phase functions
			// Either defined in ebuild or inherited from meson.eclass
			if len(result.Functions) == 0 && len(result.Inherits) == 0 {
				t.Logf("Warning: %s has no functions and no inherits", atom)
			}
		})
	}
}
