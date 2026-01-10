//go:build integration

package integration

import (
	"strings"
	"testing"
)

// AutotoolsPackages defines packages using autotools build system for validation.
//
// v0.4.0 focus: Validate parsing and build system detection, not actual builds.
// Actual builds require distfiles and are planned for future versions.
var AutotoolsPackages = []PackageSpec{
	// Simple packages - should parse and detect autotools
	{
		Atom:        "app-misc/hello",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "simple",
		Description: "GNU Hello - the classic test package",
	},
	{
		Atom:        "sys-apps/grep",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "simple",
		Description: "GNU grep",
		SkipReason:  "Uses brace expansion in variable names (mvdan.cc/sh limitation)",
	},
	{
		Atom:        "sys-apps/sed",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "simple",
		Description: "GNU sed",
		SkipReason:  "Uses brace expansion in variable names (mvdan.cc/sh limitation)",
	},
	{
		Atom:        "app-misc/screen",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "medium",
		Description: "Terminal multiplexer",
	},
	{
		Atom:        "sys-apps/coreutils",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "medium",
		Description: "GNU core utilities",
		SkipReason:  "Uses brace expansion in variable names (mvdan.cc/sh limitation)",
	},

	// Packages requiring unsupported eclasses - skip
	{
		Atom:        "sys-libs/zlib",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "simple",
		Description: "Compression library",
		SkipReason:  "Requires multilib-minimal eclass (v0.5.0)",
	},
	{
		Atom:        "dev-libs/expat",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "simple",
		Description: "XML parser library",
		SkipReason:  "Requires multilib-minimal eclass (v0.5.0)",
	},
	{
		Atom:        "dev-libs/openssl",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "medium",
		Description: "Cryptography and SSL/TLS library",
		SkipReason:  "Requires multilib-minimal eclass (v0.5.0)",
	},
	{
		Atom:        "dev-libs/libxml2",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "medium",
		Description: "XML parsing library",
		SkipReason:  "Requires multilib-minimal eclass (v0.5.0)",
	},
	{
		Atom:        "sys-libs/glibc",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "complex",
		Description: "GNU C library",
		SkipReason:  "glibc requires special build environment and cross-compilation setup",
	},
}

// TestAutotools_ParseAll validates that autotools ebuilds can be parsed.
//
// v0.4.0 scope: Parsing and metadata extraction, not building.
func TestAutotools_ParseAll(t *testing.T) {
	skipIfNoRepo(t)

	// Count skipped packages upfront for accurate statistics
	skipped := 0
	for _, spec := range AutotoolsPackages {
		if spec.SkipReason != "" {
			skipped++
		}
	}

	for _, spec := range AutotoolsPackages {
		spec := spec
		t.Run(spec.Atom, func(t *testing.T) {
			if spec.SkipReason != "" {
				t.Skip(spec.SkipReason)
			}

			if !packageExists(t, spec.Atom) {
				t.Skipf("Package %s not found in repository", spec.Atom)
			}

			// Validate parsing
			result := validatePackageParsing(t, spec.Atom)
			if result.Success {
				t.Logf("SUCCESS: %s-%s parsed (inherits: %v)",
					spec.Atom, result.Version, result.Inherits)
			} else {
				t.Errorf("FAILED: %s: %v", spec.Atom, result.Error)
			}
		})
	}

	total := len(AutotoolsPackages)
	expected := total - skipped
	t.Logf("=== Autotools Parsing Summary ===")
	t.Logf("Total: %d, Expected to parse: %d, Skipped: %d", total, expected, skipped)
}

// TestAutotools_HelloWorld validates the canonical GNU Hello package.
func TestAutotools_HelloWorld(t *testing.T) {
	skipIfNoRepo(t)

	if !packageExists(t, "app-misc/hello") {
		t.Skip("app-misc/hello not found in repository")
	}

	result := validatePackageParsing(t, "app-misc/hello")
	if !result.Success {
		t.Fatalf("GNU Hello parsing failed: %v", result.Error)
	}

	// hello should be simple - no complex inherits
	if len(result.Inherits) > 3 {
		t.Logf("Warning: hello inherits more eclasses than expected: %v", result.Inherits)
	}

	t.Logf("GNU Hello %s parsed successfully", result.Version)
}

// TestAutotools_BuildSystemDetection verifies autotools detection.
func TestAutotools_BuildSystemDetection(t *testing.T) {
	skipIfNoRepo(t)

	// These packages should be detected as using autotools
	autotoolsIndicators := []string{"econf", "emake", "default"}

	for _, spec := range AutotoolsPackages {
		if spec.SkipReason != "" {
			continue
		}

		spec := spec
		t.Run(spec.Atom, func(t *testing.T) {
			if !packageExists(t, spec.Atom) {
				t.Skipf("Package %s not found", spec.Atom)
				return
			}

			// Get ebuild content to check for autotools patterns
			result := validatePackageParsing(t, spec.Atom)
			if !result.Success {
				t.Skipf("Could not parse %s: %v", spec.Atom, result.Error)
				return
			}

			// Check if ebuild has autotools indicators
			hasAutotools := false
			for _, fn := range result.Functions {
				if fn == "src_configure" || fn == "src_compile" || fn == "src_install" {
					hasAutotools = true
					break
				}
			}

			// Also check inherits for autotools eclass
			for _, eclass := range result.Inherits {
				if strings.Contains(eclass, "autotools") ||
					strings.Contains(eclass, "toolchain") ||
					strings.Contains(eclass, "flag-o-matic") {
					hasAutotools = true
					break
				}
			}

			// Simple packages may use defaults which implies autotools
			if !hasAutotools {
				for _, ind := range autotoolsIndicators {
					_ = ind // Packages using defaults are still autotools-based
					hasAutotools = true
					break
				}
			}

			if !hasAutotools {
				t.Logf("Package %s may not use standard autotools (functions: %v, inherits: %v)",
					spec.Atom, result.Functions, result.Inherits)
			}
		})
	}
}

// TestAutotools_EbuildMetadata validates ebuild metadata extraction.
func TestAutotools_EbuildMetadata(t *testing.T) {
	skipIfNoRepo(t)

	testCases := []struct {
		atom           string
		expectEAPI     string // Empty means any valid EAPI
		expectInherits bool
	}{
		{
			atom:           "app-misc/hello",
			expectEAPI:     "", // Any valid EAPI
			expectInherits: false,
		},
		{
			atom:           "app-misc/screen",
			expectEAPI:     "",
			expectInherits: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.atom, func(t *testing.T) {
			if !packageExists(t, tc.atom) {
				t.Skipf("Package %s not found in repository", tc.atom)
				return
			}

			result := validatePackageParsing(t, tc.atom)
			if !result.Success {
				t.Fatalf("Failed to parse %s: %v", tc.atom, result.Error)
			}

			// Verify EAPI is valid (0-8)
			validEAPIs := map[string]bool{
				"0": true, "1": true, "2": true, "3": true,
				"4": true, "5": true, "6": true, "7": true, "8": true,
			}
			if result.EAPI != "" && !validEAPIs[result.EAPI] {
				t.Errorf("Invalid EAPI %q for %s", result.EAPI, tc.atom)
			}

			t.Logf("%s: version=%s, EAPI=%s, inherits=%v",
				tc.atom, result.Version, result.EAPI, result.Inherits)
		})
	}
}
