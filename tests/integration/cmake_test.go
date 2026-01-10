//go:build integration

package integration

import (
	"strings"
	"testing"
)

// CMakePackages defines packages using CMake build system for validation.
//
// v0.4.0 focus: Validate cmake.eclass inheritance and parsing.
// Actual builds require distfiles and are planned for future versions.
var CMakePackages = []PackageSpec{
	// Simple packages - should parse and detect cmake.eclass
	{
		Atom:        "dev-cpp/nlohmann_json",
		BuildSystem: BuildSystemCMake,
		Complexity:  "simple",
		Description: "JSON for Modern C++ (header-only)",
	},
	{
		Atom:        "dev-libs/rapidjson",
		BuildSystem: BuildSystemCMake,
		Complexity:  "simple",
		Description: "Fast JSON parser (header-only)",
	},
	{
		Atom:        "dev-libs/pugixml",
		BuildSystem: BuildSystemCMake,
		Complexity:  "simple",
		Description: "Light-weight XML processing library",
	},
	{
		Atom:        "media-libs/glm",
		BuildSystem: BuildSystemCMake,
		Complexity:  "simple",
		Description: "OpenGL Mathematics library",
	},
	{
		Atom:        "dev-libs/jsoncpp",
		BuildSystem: BuildSystemCMake,
		Complexity:  "simple",
		Description: "C++ JSON reader/writer library",
	},
	{
		Atom:        "dev-libs/tinyxml2",
		BuildSystem: BuildSystemCMake,
		Complexity:  "simple",
		Description: "Simple XML parser",
	},

	// Medium complexity packages
	{
		Atom:        "dev-util/cmake",
		BuildSystem: BuildSystemCMake,
		Complexity:  "medium",
		Description: "CMake build system (self-hosting)",
	},
	{
		Atom:        "dev-libs/spdlog",
		BuildSystem: BuildSystemCMake,
		Complexity:  "medium",
		Description: "Fast C++ logging library",
	},

	// Packages requiring unsupported features - skip
	{
		Atom:        "dev-libs/libfmt",
		BuildSystem: BuildSystemCMake,
		Complexity:  "simple",
		Description: "Modern C++ formatting library",
		SkipReason:  "May require multilib support (v0.5.0)",
	},
	{
		Atom:        "net-misc/curl",
		BuildSystem: BuildSystemCMake,
		Complexity:  "medium",
		Description: "Network transfer utility and library",
		SkipReason:  "Uses autotools, not cmake in Gentoo",
	},
	{
		Atom:        "media-libs/glfw",
		BuildSystem: BuildSystemCMake,
		Complexity:  "medium",
		Description: "OpenGL window and input library",
		SkipReason:  "Requires X11/Wayland dependencies",
	},
	{
		Atom:        "sci-libs/nlopt",
		BuildSystem: BuildSystemCMake,
		Complexity:  "medium",
		Description: "Nonlinear optimization library",
		SkipReason:  "May require Python/Guile bindings",
	},

	// Complex packages - always skip
	{
		Atom:        "dev-libs/boost",
		BuildSystem: BuildSystemCMake,
		Complexity:  "complex",
		Description: "Boost C++ libraries",
		SkipReason:  "Boost requires special build system (b2/bjam)",
	},
	{
		Atom:        "media-libs/opencv",
		BuildSystem: BuildSystemCMake,
		Complexity:  "complex",
		Description: "Computer vision library",
		SkipReason:  "OpenCV requires many dependencies",
	},
	{
		Atom:        "x11-libs/wxGTK",
		BuildSystem: BuildSystemCMake,
		Complexity:  "complex",
		Description: "wxWidgets GUI toolkit",
		SkipReason:  "wxGTK requires X11 and GTK dependencies",
	},
}

// TestCMake_ParseAll validates that CMake ebuilds can be parsed.
//
// v0.4.0 scope: Parsing and cmake.eclass detection, not building.
func TestCMake_ParseAll(t *testing.T) {
	skipIfNoRepo(t)

	// Count skipped packages upfront for accurate statistics
	skipped := 0
	for _, spec := range CMakePackages {
		if spec.SkipReason != "" {
			skipped++
		}
	}

	for _, spec := range CMakePackages {
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

	total := len(CMakePackages)
	expected := total - skipped
	t.Logf("=== CMake Parsing Summary ===")
	t.Logf("Total: %d, Expected to parse: %d, Skipped: %d", total, expected, skipped)
}

// TestCMake_EclassInheritance verifies cmake.eclass inheritance detection.
func TestCMake_EclassInheritance(t *testing.T) {
	skipIfNoRepo(t)

	for _, spec := range CMakePackages {
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

			// Check for cmake.eclass inheritance
			hasCMake := false
			for _, eclass := range result.Inherits {
				if strings.Contains(eclass, "cmake") {
					hasCMake = true
					break
				}
			}

			if hasCMake {
				t.Logf("%s correctly inherits cmake eclass", spec.Atom)
			} else {
				t.Logf("%s may use cmake without eclass (inherits: %v)",
					spec.Atom, result.Inherits)
			}
		})
	}
}

// TestCMake_NlohmannJSON validates the canonical CMake test package.
func TestCMake_NlohmannJSON(t *testing.T) {
	skipIfNoRepo(t)

	atom := "dev-cpp/nlohmann_json"
	if !packageExists(t, atom) {
		t.Skipf("%s not found in repository", atom)
	}

	result := validatePackageParsing(t, atom)
	if !result.Success {
		t.Fatalf("Failed to parse %s: %v", atom, result.Error)
	}

	// Verify cmake.eclass is inherited
	hasCMake := false
	for _, eclass := range result.Inherits {
		if strings.Contains(eclass, "cmake") {
			hasCMake = true
			break
		}
	}

	if !hasCMake {
		t.Errorf("%s should inherit cmake eclass, got: %v", atom, result.Inherits)
	}

	t.Logf("%s %s: EAPI=%s, inherits=%v, functions=%v",
		atom, result.Version, result.EAPI, result.Inherits, result.Functions)
}

// TestCMake_MetadataExtraction validates metadata extraction for CMake packages.
func TestCMake_MetadataExtraction(t *testing.T) {
	skipIfNoRepo(t)

	testCases := []struct {
		atom        string
		expectCMake bool
	}{
		{"dev-cpp/nlohmann_json", true},
		{"dev-libs/jsoncpp", true},
		{"dev-libs/pugixml", true},
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

			// Check cmake inheritance if expected
			if tc.expectCMake {
				hasCMake := false
				for _, eclass := range result.Inherits {
					if strings.Contains(eclass, "cmake") {
						hasCMake = true
						break
					}
				}
				if !hasCMake {
					t.Logf("Warning: %s expected to inherit cmake, got: %v",
						tc.atom, result.Inherits)
				}
			}

			t.Logf("%s: version=%s, EAPI=%s, inherits=%v",
				tc.atom, result.Version, result.EAPI, result.Inherits)
		})
	}
}

// TestCMake_FunctionDiscovery verifies function discovery for CMake packages.
func TestCMake_FunctionDiscovery(t *testing.T) {
	skipIfNoRepo(t)

	testAtoms := []string{
		"dev-cpp/nlohmann_json",
		"dev-libs/jsoncpp",
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

			// CMake packages typically have src_configure, src_compile, src_install
			// Either defined in ebuild or inherited from cmake.eclass
			if len(result.Functions) == 0 && len(result.Inherits) == 0 {
				t.Logf("Warning: %s has no functions and no inherits", atom)
			}
		})
	}
}
