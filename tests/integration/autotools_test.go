//go:build integration

package integration

import (
	"testing"

	"github.com/grpmsoft/grpm/internal/ebuild"
)

// AutotoolsPackages defines the test packages using autotools build system.
//
// Per task specification:
// - app-misc/hello (Simple, GNU Hello)
// - sys-libs/zlib (Simple, Compression)
// - dev-libs/openssl (Medium, Crypto)
// - sys-libs/glibc (Complex, C library)
// - dev-libs/libxml2 (Medium, XML)
//
// Additional packages for better coverage:
// - app-misc/screen (Medium, Terminal multiplexer)
// - sys-apps/coreutils (Medium, Core utilities)
// - dev-libs/expat (Simple, XML parser)
// - sys-apps/findutils (Simple, Find utilities)
// - sys-apps/grep (Simple, Pattern matching)
var AutotoolsPackages = []PackageSpec{
	// Simple packages (expected to pass)
	{
		Atom:        "app-misc/hello",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "simple",
		Description: "GNU Hello - the classic test package",
		ExpectedPhases: []ebuild.Phase{
			ebuild.PhaseSetup,
			ebuild.PhaseUnpack,
			ebuild.PhasePrepare,
			ebuild.PhaseConfigure,
			ebuild.PhaseCompile,
			ebuild.PhaseInstall,
		},
	},
	{
		Atom:        "sys-libs/zlib",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "simple",
		Description: "Compression library",
		ExpectedPhases: []ebuild.Phase{
			ebuild.PhaseSetup,
			ebuild.PhaseUnpack,
			ebuild.PhasePrepare,
			ebuild.PhaseConfigure,
			ebuild.PhaseCompile,
			ebuild.PhaseInstall,
		},
	},
	{
		Atom:        "dev-libs/expat",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "simple",
		Description: "XML parser library",
		ExpectedPhases: []ebuild.Phase{
			ebuild.PhaseSetup,
			ebuild.PhaseUnpack,
			ebuild.PhasePrepare,
			ebuild.PhaseConfigure,
			ebuild.PhaseCompile,
			ebuild.PhaseInstall,
		},
	},
	{
		Atom:        "sys-apps/findutils",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "simple",
		Description: "GNU find utilities",
		ExpectedPhases: []ebuild.Phase{
			ebuild.PhaseSetup,
			ebuild.PhaseUnpack,
			ebuild.PhasePrepare,
			ebuild.PhaseConfigure,
			ebuild.PhaseCompile,
			ebuild.PhaseInstall,
		},
	},
	{
		Atom:        "sys-apps/grep",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "simple",
		Description: "GNU grep",
		ExpectedPhases: []ebuild.Phase{
			ebuild.PhaseSetup,
			ebuild.PhaseUnpack,
			ebuild.PhasePrepare,
			ebuild.PhaseConfigure,
			ebuild.PhaseCompile,
			ebuild.PhaseInstall,
		},
	},

	// Medium complexity packages
	{
		Atom:        "dev-libs/openssl",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "medium",
		Description: "Cryptography and SSL/TLS library",
		ExpectedPhases: []ebuild.Phase{
			ebuild.PhaseSetup,
			ebuild.PhaseUnpack,
			ebuild.PhasePrepare,
			ebuild.PhaseConfigure,
			ebuild.PhaseCompile,
			ebuild.PhaseInstall,
		},
	},
	{
		Atom:        "dev-libs/libxml2",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "medium",
		Description: "XML parsing library",
		ExpectedPhases: []ebuild.Phase{
			ebuild.PhaseSetup,
			ebuild.PhaseUnpack,
			ebuild.PhasePrepare,
			ebuild.PhaseConfigure,
			ebuild.PhaseCompile,
			ebuild.PhaseInstall,
		},
	},
	{
		Atom:        "app-misc/screen",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "medium",
		Description: "Terminal multiplexer",
		ExpectedPhases: []ebuild.Phase{
			ebuild.PhaseSetup,
			ebuild.PhaseUnpack,
			ebuild.PhasePrepare,
			ebuild.PhaseConfigure,
			ebuild.PhaseCompile,
			ebuild.PhaseInstall,
		},
	},
	{
		Atom:        "sys-apps/coreutils",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "medium",
		Description: "GNU core utilities",
		ExpectedPhases: []ebuild.Phase{
			ebuild.PhaseSetup,
			ebuild.PhaseUnpack,
			ebuild.PhasePrepare,
			ebuild.PhaseConfigure,
			ebuild.PhaseCompile,
			ebuild.PhaseInstall,
		},
	},

	// Complex packages (may require special handling)
	{
		Atom:        "sys-libs/glibc",
		BuildSystem: BuildSystemAutotools,
		Complexity:  "complex",
		Description: "GNU C library",
		SkipReason:  "glibc requires special build environment and cross-compilation setup",
	},
}

// TestAutotools_BuildAll runs build tests for all autotools packages.
//
// Success target: >= 90% (9/10 packages)
func TestAutotools_BuildAll(t *testing.T) {
	skipIfNoRepo(t)
	skipIfNoDistfiles(t)
	skipIfNoBuildTools(t, "make", "gcc")

	var passed, failed, skipped int

	for _, spec := range AutotoolsPackages {
		spec := spec // Capture range variable
		t.Run(spec.Atom, func(t *testing.T) {
			if spec.SkipReason != "" {
				t.Skip(spec.SkipReason)
				skipped++
				return
			}

			// Check if package exists
			if !packageExists(t, spec.Atom) {
				t.Skipf("Package %s not found in repository", spec.Atom)
				skipped++
				return
			}

			result := buildPackage(t, spec.Atom)
			if result.Success() {
				t.Logf("SUCCESS: %s-%s built in %v (%d files)",
					spec.Atom, result.Version, result.Duration, result.FilesInstalled)
				passed++

				// Verify expected phases
				for _, phase := range spec.ExpectedPhases {
					assertPhaseSuccess(t, result, string(phase))
				}

				// Verify files were installed
				assertFilesInstalled(t, result, 1)
			} else {
				t.Errorf("FAILED: %s: %v", spec.Atom, result.Error)
				failed++
			}
		})
	}

	// Log summary
	total := len(AutotoolsPackages)
	t.Logf("=== Autotools Summary ===")
	t.Logf("Total: %d, Passed: %d, Failed: %d, Skipped: %d", total, passed, failed, skipped)

	if total-skipped > 0 {
		passRate := float64(passed) / float64(total-skipped) * 100
		t.Logf("Pass Rate: %.1f%% (target: 90%%)", passRate)

		if passRate < 90 {
			t.Errorf("Pass rate %.1f%% is below target of 90%%", passRate)
		}
	}
}

// TestAutotools_HelloWorld specifically tests the GNU Hello package.
//
// This is the canonical test case - if this fails, something is fundamentally wrong.
func TestAutotools_HelloWorld(t *testing.T) {
	skipIfNoRepo(t)
	skipIfNoDistfiles(t)
	skipIfNoBuildTools(t, "make", "gcc")

	if !packageExists(t, "app-misc/hello") {
		t.Skip("app-misc/hello not found in repository")
	}

	result := buildPackage(t, "app-misc/hello")

	if !result.Success() {
		t.Fatalf("GNU Hello build failed: %v", result.Error)
	}

	// Verify all phases passed
	phases := []string{"setup", "unpack", "prepare", "configure", "compile", "install"}
	for _, phase := range phases {
		assertPhaseSuccess(t, result, phase)
	}

	// Verify hello binary was installed
	assertFilesInstalled(t, result, 1)

	t.Logf("GNU Hello %s built successfully in %v", result.Version, result.Duration)
}

// TestAutotools_Zlib tests the zlib compression library.
//
// zlib is a fundamental dependency for many packages.
func TestAutotools_Zlib(t *testing.T) {
	skipIfNoRepo(t)
	skipIfNoDistfiles(t)
	skipIfNoBuildTools(t, "make", "gcc")

	if !packageExists(t, "sys-libs/zlib") {
		t.Skip("sys-libs/zlib not found in repository")
	}

	result := buildPackage(t, "sys-libs/zlib")

	if !result.Success() {
		t.Fatalf("zlib build failed: %v", result.Error)
	}

	// zlib should install library files
	assertFilesInstalled(t, result, 3) // libz.so, libz.a, zlib.h at minimum

	t.Logf("zlib %s built successfully in %v (%d files)",
		result.Version, result.Duration, result.FilesInstalled)
}

// TestAutotools_OpenSSL tests the OpenSSL library.
//
// OpenSSL is a medium-complexity package with many configure options.
func TestAutotools_OpenSSL(t *testing.T) {
	skipIfNoRepo(t)
	skipIfNoDistfiles(t)
	skipIfNoBuildTools(t, "make", "gcc", "perl")

	if !packageExists(t, "dev-libs/openssl") {
		t.Skip("dev-libs/openssl not found in repository")
	}

	result := buildPackage(t, "dev-libs/openssl")

	if !result.Success() {
		// OpenSSL is complex - log details for debugging
		t.Logf("OpenSSL build failed: %v", result.Error)
		for phase, pr := range result.Phases {
			if !pr.Success {
				t.Logf("Phase %s failed: %v", phase, pr.Error)
			}
		}
		t.Fail()
	} else {
		t.Logf("OpenSSL %s built successfully in %v (%d files)",
			result.Version, result.Duration, result.FilesInstalled)
	}
}

// TestAutotools_PhaseExecution verifies that all phases execute correctly.
func TestAutotools_PhaseExecution(t *testing.T) {
	skipIfNoRepo(t)
	skipIfNoDistfiles(t)
	skipIfNoBuildTools(t, "make", "gcc")

	// Use hello as the test subject - it's simple and reliable
	if !packageExists(t, "app-misc/hello") {
		t.Skip("app-misc/hello not found in repository")
	}

	result := buildPackage(t, "app-misc/hello")

	expectedPhases := map[string]bool{
		"setup":     true,
		"unpack":    true,
		"prepare":   true,
		"configure": true,
		"compile":   true,
		"install":   true,
	}

	for phase := range expectedPhases {
		t.Run(phase, func(t *testing.T) {
			pr, ok := result.Phases[phase]
			if !ok {
				t.Errorf("Phase %s was not executed", phase)
				return
			}
			if !pr.Success {
				t.Errorf("Phase %s failed: %v", phase, pr.Error)
			}
		})
	}
}

// TestAutotools_EbuildParsing verifies that autotools ebuilds are parsed correctly.
func TestAutotools_EbuildParsing(t *testing.T) {
	skipIfNoRepo(t)

	testCases := []struct {
		atom            string
		expectedInherit []string
	}{
		{
			atom:            "app-misc/hello",
			expectedInherit: nil, // hello typically doesn't inherit any eclasses
		},
		{
			atom:            "sys-libs/zlib",
			expectedInherit: nil, // zlib may or may not inherit eclasses
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.atom, func(t *testing.T) {
			if !packageExists(t, tc.atom) {
				t.Skipf("Package %s not found in repository", tc.atom)
			}

			inherits := getEbuildInherits(t, tc.atom)
			t.Logf("%s inherits: %v", tc.atom, inherits)

			// Just verify we can parse the ebuild
			version := getPackageVersion(t, tc.atom)
			if version == "" {
				t.Errorf("Could not get version for %s", tc.atom)
			}
		})
	}
}
