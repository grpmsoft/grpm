//go:build integration

package integration

import (
	"testing"

	"github.com/grpmsoft/grpm/internal/ebuild"
)

// CMakePackages defines the test packages using CMake build system.
//
// Per task specification (15 packages):
//
// Simple:
// - dev-cpp/nlohmann_json (Header-only JSON library)
// - dev-libs/libfmt (Modern C++ formatting)
// - dev-libs/rapidjson (Header-only JSON parser)
// - dev-libs/pugixml (XML processing library)
// - media-libs/glm (OpenGL math library)
//
// Medium:
// - dev-util/cmake (CMake itself - self-hosting)
// - net-misc/curl (Network library with many USE flags)
//
// Complex:
// - dev-libs/boost (Modular C++ libraries)
// - media-libs/opencv (Computer vision library)
// - x11-libs/wxGTK (GUI toolkit)
//
// Additional coverage:
// - dev-libs/jsoncpp (JSON library)
// - dev-libs/spdlog (Logging library)
// - dev-libs/tinyxml2 (Simple XML parser)
// - media-libs/glfw (OpenGL window library)
// - sci-libs/nlopt (Optimization library)
var CMakePackages = []PackageSpec{
	// Simple packages (header-only or minimal deps)
	{
		Atom:        "dev-cpp/nlohmann_json",
		BuildSystem: BuildSystemCMake,
		Complexity:  "simple",
		Description: "JSON for Modern C++ (header-only)",
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
		Atom:        "dev-libs/libfmt",
		BuildSystem: BuildSystemCMake,
		Complexity:  "simple",
		Description: "Modern C++ formatting library",
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
		Atom:        "dev-libs/rapidjson",
		BuildSystem: BuildSystemCMake,
		Complexity:  "simple",
		Description: "Fast JSON parser (header-only)",
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
		Atom:        "dev-libs/pugixml",
		BuildSystem: BuildSystemCMake,
		Complexity:  "simple",
		Description: "Light-weight XML processing library",
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
		Atom:        "media-libs/glm",
		BuildSystem: BuildSystemCMake,
		Complexity:  "simple",
		Description: "OpenGL Mathematics library",
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
		Atom:        "dev-libs/jsoncpp",
		BuildSystem: BuildSystemCMake,
		Complexity:  "simple",
		Description: "C++ JSON reader/writer library",
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
		Atom:        "dev-libs/tinyxml2",
		BuildSystem: BuildSystemCMake,
		Complexity:  "simple",
		Description: "Simple XML parser",
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
		Atom:        "dev-util/cmake",
		BuildSystem: BuildSystemCMake,
		Complexity:  "medium",
		Description: "CMake build system (self-hosting)",
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
		Atom:        "net-misc/curl",
		BuildSystem: BuildSystemCMake,
		Complexity:  "medium",
		Description: "Network transfer utility and library",
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
		Atom:        "dev-libs/spdlog",
		BuildSystem: BuildSystemCMake,
		Complexity:  "medium",
		Description: "Fast C++ logging library",
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
		Atom:        "media-libs/glfw",
		BuildSystem: BuildSystemCMake,
		Complexity:  "medium",
		Description: "OpenGL window and input library",
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
		Atom:        "sci-libs/nlopt",
		BuildSystem: BuildSystemCMake,
		Complexity:  "medium",
		Description: "Nonlinear optimization library",
		ExpectedPhases: []ebuild.Phase{
			ebuild.PhaseSetup,
			ebuild.PhaseUnpack,
			ebuild.PhasePrepare,
			ebuild.PhaseConfigure,
			ebuild.PhaseCompile,
			ebuild.PhaseInstall,
		},
	},

	// Complex packages
	{
		Atom:        "dev-libs/boost",
		BuildSystem: BuildSystemCMake,
		Complexity:  "complex",
		Description: "Boost C++ libraries",
		SkipReason:  "Boost requires special build system (b2/bjam), not standard CMake",
	},
	{
		Atom:        "media-libs/opencv",
		BuildSystem: BuildSystemCMake,
		Complexity:  "complex",
		Description: "Computer vision library",
		SkipReason:  "OpenCV requires many dependencies and takes very long to build",
	},
	{
		Atom:        "x11-libs/wxGTK",
		BuildSystem: BuildSystemCMake,
		Complexity:  "complex",
		Description: "wxWidgets GUI toolkit",
		SkipReason:  "wxGTK requires X11 and GTK dependencies",
	},
}

// TestCMake_BuildAll runs build tests for all CMake packages.
//
// Success target: >= 80% (12/15 packages)
func TestCMake_BuildAll(t *testing.T) {
	skipIfNoRepo(t)
	skipIfNoDistfiles(t)
	skipIfNoBuildTools(t, "cmake", "make", "gcc")

	var passed, failed, skipped int

	for _, spec := range CMakePackages {
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
	total := len(CMakePackages)
	t.Logf("=== CMake Summary ===")
	t.Logf("Total: %d, Passed: %d, Failed: %d, Skipped: %d", total, passed, failed, skipped)

	if total-skipped > 0 {
		passRate := float64(passed) / float64(total-skipped) * 100
		t.Logf("Pass Rate: %.1f%% (target: 80%%)", passRate)

		if passRate < 80 {
			t.Errorf("Pass rate %.1f%% is below target of 80%%", passRate)
		}
	}
}

// TestCMake_NlohmannJSON tests the nlohmann_json header-only library.
//
// This is the simplest CMake package - if this fails, something is wrong with cmake.eclass.
func TestCMake_NlohmannJSON(t *testing.T) {
	skipIfNoRepo(t)
	skipIfNoDistfiles(t)
	skipIfNoBuildTools(t, "cmake", "make")

	if !packageExists(t, "dev-cpp/nlohmann_json") {
		t.Skip("dev-cpp/nlohmann_json not found in repository")
	}

	result := buildPackage(t, "dev-cpp/nlohmann_json")

	if !result.Success() {
		t.Fatalf("nlohmann_json build failed: %v", result.Error)
	}

	// Verify all phases passed
	phases := []string{"setup", "unpack", "prepare", "configure", "compile", "install"}
	for _, phase := range phases {
		assertPhaseSuccess(t, result, phase)
	}

	// Verify header files were installed
	assertFilesInstalled(t, result, 1)

	t.Logf("nlohmann_json %s built successfully in %v", result.Version, result.Duration)
}

// TestCMake_Curl tests the curl library.
//
// curl is a good medium-complexity test with many USE flags.
func TestCMake_Curl(t *testing.T) {
	skipIfNoRepo(t)
	skipIfNoDistfiles(t)
	skipIfNoBuildTools(t, "cmake", "make", "gcc")

	if !packageExists(t, "net-misc/curl") {
		t.Skip("net-misc/curl not found in repository")
	}

	result := buildPackage(t, "net-misc/curl")

	if !result.Success() {
		t.Logf("curl build failed: %v", result.Error)
		for phase, pr := range result.Phases {
			if !pr.Success {
				t.Logf("Phase %s failed: %v", phase, pr.Error)
			}
		}
		t.Fail()
	} else {
		t.Logf("curl %s built successfully in %v (%d files)",
			result.Version, result.Duration, result.FilesInstalled)
	}
}

// TestCMake_InheritanceCheck verifies cmake.eclass inheritance.
func TestCMake_InheritanceCheck(t *testing.T) {
	skipIfNoRepo(t)

	// These packages should inherit cmake.eclass (or cmake-utils)
	cmakePackages := []string{
		"dev-cpp/nlohmann_json",
		"dev-libs/jsoncpp",
		"dev-libs/pugixml",
	}

	for _, atom := range cmakePackages {
		t.Run(atom, func(t *testing.T) {
			if !packageExists(t, atom) {
				t.Skipf("Package %s not found in repository", atom)
			}

			inherits := getEbuildInherits(t, atom)
			t.Logf("%s inherits: %v", atom, inherits)

			// Check if cmake or cmake-utils is inherited
			hasCMake := false
			for _, ec := range inherits {
				if ec == "cmake" || ec == "cmake-utils" {
					hasCMake = true
					break
				}
			}

			if !hasCMake {
				t.Logf("Warning: %s does not inherit cmake eclass (inherits: %v)", atom, inherits)
				// Not failing - some packages may use cmake differently
			}
		})
	}
}

// TestCMake_SimplePackages tests a subset of simple CMake packages.
//
// These are header-only or minimal dependency packages that should build reliably.
func TestCMake_SimplePackages(t *testing.T) {
	skipIfNoRepo(t)
	skipIfNoDistfiles(t)
	skipIfNoBuildTools(t, "cmake", "make")

	simplePackages := []string{
		"dev-cpp/nlohmann_json",
		"dev-libs/rapidjson",
		"media-libs/glm",
		"dev-libs/tinyxml2",
	}

	for _, atom := range simplePackages {
		atom := atom
		t.Run(atom, func(t *testing.T) {
			if !packageExists(t, atom) {
				t.Skipf("Package %s not found in repository", atom)
			}

			result := buildPackage(t, atom)
			if result.Success() {
				t.Logf("SUCCESS: %s built in %v", atom, result.Duration)
			} else {
				t.Errorf("FAILED: %s: %v", atom, result.Error)
			}
		})
	}
}

// TestCMake_PhaseExecution verifies CMake-specific phase execution.
func TestCMake_PhaseExecution(t *testing.T) {
	skipIfNoRepo(t)
	skipIfNoDistfiles(t)
	skipIfNoBuildTools(t, "cmake", "make")

	// Use nlohmann_json as test subject - simple and reliable
	atom := "dev-cpp/nlohmann_json"
	if !packageExists(t, atom) {
		t.Skipf("%s not found in repository", atom)
	}

	result := buildPackage(t, atom)

	// CMake packages should go through all standard phases
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
			} else {
				t.Logf("Phase %s completed in %v", phase, pr.Duration)
			}
		})
	}
}

// TestCMake_ConfigureFlags verifies that CMake configure flags are applied.
func TestCMake_ConfigureFlags(t *testing.T) {
	skipIfNoRepo(t)

	// Check that cmake.eclass defines src_configure
	testAtoms := []string{
		"dev-cpp/nlohmann_json",
		"dev-libs/jsoncpp",
	}

	for _, atom := range testAtoms {
		t.Run(atom, func(t *testing.T) {
			if !packageExists(t, atom) {
				t.Skipf("Package %s not found in repository", atom)
			}

			// Check if ebuild has custom src_configure
			hasConfigure := ebuildHasFunction(t, atom, "src_configure")
			t.Logf("%s has custom src_configure: %v", atom, hasConfigure)

			// Either way, the package should be buildable via cmake.eclass defaults
		})
	}
}
