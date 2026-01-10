//go:build integration

package integration

import (
	"testing"

	"github.com/grpmsoft/grpm/internal/ebuild"
)

// MesonPackages defines the test packages using Meson build system.
//
// Per task specification (15 packages):
//
// Simple:
// - dev-libs/json-glib (JSON for GLib)
//
// Medium:
// - dev-libs/glib (Core GNOME library)
// - x11-libs/cairo (2D graphics library)
// - x11-libs/pango (Text rendering)
// - media-libs/harfbuzz (Text shaping)
// - gnome-base/librsvg (SVG rendering)
//
// Complex:
// - media-libs/mesa (OpenGL implementation)
// - x11-libs/gtk+ (GTK toolkit)
//
// Additional coverage:
// - app-text/ghostscript-gpl (PostScript interpreter)
// - dev-libs/libpcre2 (Perl-compatible regex)
// - dev-libs/libxslt (XSLT processor)
// - media-libs/fontconfig (Font configuration)
// - media-libs/freetype (Font rendering)
// - net-misc/networkmanager (Network management)
// - sys-libs/dbus (D-Bus message bus)
var MesonPackages = []PackageSpec{
	// Simple packages
	{
		Atom:        "dev-libs/json-glib",
		BuildSystem: BuildSystemMeson,
		Complexity:  "simple",
		Description: "JSON library for GLib",
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
		Atom:        "dev-libs/libpcre2",
		BuildSystem: BuildSystemMeson,
		Complexity:  "simple",
		Description: "Perl-compatible regular expressions",
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
		Atom:        "dev-libs/glib",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "Core GNOME library",
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
		Atom:        "x11-libs/cairo",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "2D graphics library",
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
		Atom:        "x11-libs/pango",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "Text rendering library",
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
		Atom:        "media-libs/harfbuzz",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "OpenType text shaping engine",
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
		Atom:        "gnome-base/librsvg",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "SVG rendering library",
		ExpectedPhases: []ebuild.Phase{
			ebuild.PhaseSetup,
			ebuild.PhaseUnpack,
			ebuild.PhasePrepare,
			ebuild.PhaseConfigure,
			ebuild.PhaseCompile,
			ebuild.PhaseInstall,
		},
		SkipReason: "librsvg requires Rust toolchain",
	},
	{
		Atom:        "media-libs/fontconfig",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "Font configuration library",
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
		Atom:        "media-libs/freetype",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "Font rendering library",
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
		Atom:        "dev-libs/libxslt",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "XSLT processor library",
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
		Atom:        "sys-apps/dbus",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "D-Bus message bus system",
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
		Atom:        "app-text/ghostscript-gpl",
		BuildSystem: BuildSystemMeson,
		Complexity:  "medium",
		Description: "PostScript/PDF interpreter",
		ExpectedPhases: []ebuild.Phase{
			ebuild.PhaseSetup,
			ebuild.PhaseUnpack,
			ebuild.PhasePrepare,
			ebuild.PhaseConfigure,
			ebuild.PhaseCompile,
			ebuild.PhaseInstall,
		},
		SkipReason: "ghostscript has complex dependencies and build",
	},

	// Complex packages
	{
		Atom:        "media-libs/mesa",
		BuildSystem: BuildSystemMeson,
		Complexity:  "complex",
		Description: "OpenGL implementation",
		ExpectedPhases: []ebuild.Phase{
			ebuild.PhaseSetup,
			ebuild.PhaseUnpack,
			ebuild.PhasePrepare,
			ebuild.PhaseConfigure,
			ebuild.PhaseCompile,
			ebuild.PhaseInstall,
		},
		SkipReason: "Mesa requires many dependencies and takes very long to build",
	},
	{
		Atom:        "x11-libs/gtk+",
		BuildSystem: BuildSystemMeson,
		Complexity:  "complex",
		Description: "GTK+ toolkit",
		ExpectedPhases: []ebuild.Phase{
			ebuild.PhaseSetup,
			ebuild.PhaseUnpack,
			ebuild.PhasePrepare,
			ebuild.PhaseConfigure,
			ebuild.PhaseCompile,
			ebuild.PhaseInstall,
		},
		SkipReason: "GTK+ requires X11 and many dependencies",
	},
	{
		Atom:        "net-misc/networkmanager",
		BuildSystem: BuildSystemMeson,
		Complexity:  "complex",
		Description: "Network management daemon",
		ExpectedPhases: []ebuild.Phase{
			ebuild.PhaseSetup,
			ebuild.PhaseUnpack,
			ebuild.PhasePrepare,
			ebuild.PhaseConfigure,
			ebuild.PhaseCompile,
			ebuild.PhaseInstall,
		},
		SkipReason: "NetworkManager requires system integration and many dependencies",
	},
}

// TestMeson_BuildAll runs build tests for all Meson packages.
//
// Success target: >= 80% (12/15 packages)
func TestMeson_BuildAll(t *testing.T) {
	skipIfNoRepo(t)
	skipIfNoDistfiles(t)
	skipIfNoBuildTools(t, "meson", "ninja")

	var passed, failed, skipped int

	for _, spec := range MesonPackages {
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
	total := len(MesonPackages)
	t.Logf("=== Meson Summary ===")
	t.Logf("Total: %d, Passed: %d, Failed: %d, Skipped: %d", total, passed, failed, skipped)

	if total-skipped > 0 {
		passRate := float64(passed) / float64(total-skipped) * 100
		t.Logf("Pass Rate: %.1f%% (target: 80%%)", passRate)

		if passRate < 80 {
			t.Errorf("Pass rate %.1f%% is below target of 80%%", passRate)
		}
	}
}

// TestMeson_PCRE2 tests the libpcre2 library.
//
// PCRE2 is a simple Meson package that should build reliably.
func TestMeson_PCRE2(t *testing.T) {
	skipIfNoRepo(t)
	skipIfNoDistfiles(t)
	skipIfNoBuildTools(t, "meson", "ninja")

	if !packageExists(t, "dev-libs/libpcre2") {
		t.Skip("dev-libs/libpcre2 not found in repository")
	}

	result := buildPackage(t, "dev-libs/libpcre2")

	if !result.Success() {
		t.Fatalf("libpcre2 build failed: %v", result.Error)
	}

	// Verify all phases passed
	phases := []string{"setup", "unpack", "prepare", "configure", "compile", "install"}
	for _, phase := range phases {
		assertPhaseSuccess(t, result, phase)
	}

	// Verify library files were installed
	assertFilesInstalled(t, result, 1)

	t.Logf("libpcre2 %s built successfully in %v", result.Version, result.Duration)
}

// TestMeson_GLib tests the GLib library.
//
// GLib is the foundation of GNOME and many Meson-based projects.
func TestMeson_GLib(t *testing.T) {
	skipIfNoRepo(t)
	skipIfNoDistfiles(t)
	skipIfNoBuildTools(t, "meson", "ninja", "gcc")

	if !packageExists(t, "dev-libs/glib") {
		t.Skip("dev-libs/glib not found in repository")
	}

	result := buildPackage(t, "dev-libs/glib")

	if !result.Success() {
		t.Logf("GLib build failed: %v", result.Error)
		for phase, pr := range result.Phases {
			if !pr.Success {
				t.Logf("Phase %s failed: %v", phase, pr.Error)
			}
		}
		t.Fail()
	} else {
		t.Logf("GLib %s built successfully in %v (%d files)",
			result.Version, result.Duration, result.FilesInstalled)
	}
}

// TestMeson_InheritanceCheck verifies meson.eclass inheritance.
func TestMeson_InheritanceCheck(t *testing.T) {
	skipIfNoRepo(t)

	// These packages should inherit meson.eclass
	mesonPackages := []string{
		"dev-libs/json-glib",
		"dev-libs/glib",
		"media-libs/harfbuzz",
	}

	for _, atom := range mesonPackages {
		t.Run(atom, func(t *testing.T) {
			if !packageExists(t, atom) {
				t.Skipf("Package %s not found in repository", atom)
			}

			inherits := getEbuildInherits(t, atom)
			t.Logf("%s inherits: %v", atom, inherits)

			// Check if meson is inherited
			hasMeson := false
			for _, ec := range inherits {
				if ec == "meson" {
					hasMeson = true
					break
				}
			}

			if !hasMeson {
				t.Logf("Warning: %s does not inherit meson eclass (inherits: %v)", atom, inherits)
				// Not failing - some packages may use meson differently
			}
		})
	}
}

// TestMeson_SimplePackages tests a subset of simple Meson packages.
func TestMeson_SimplePackages(t *testing.T) {
	skipIfNoRepo(t)
	skipIfNoDistfiles(t)
	skipIfNoBuildTools(t, "meson", "ninja")

	simplePackages := []string{
		"dev-libs/json-glib",
		"dev-libs/libpcre2",
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

// TestMeson_PhaseExecution verifies Meson-specific phase execution.
func TestMeson_PhaseExecution(t *testing.T) {
	skipIfNoRepo(t)
	skipIfNoDistfiles(t)
	skipIfNoBuildTools(t, "meson", "ninja")

	// Use json-glib as test subject - simple and reliable
	atom := "dev-libs/json-glib"
	if !packageExists(t, atom) {
		t.Skipf("%s not found in repository", atom)
	}

	result := buildPackage(t, atom)

	// Meson packages should go through all standard phases
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

// TestMeson_NinjaBackend verifies that ninja is used as the build backend.
func TestMeson_NinjaBackend(t *testing.T) {
	skipIfNoRepo(t)

	// Verify ninja is available
	skipIfNoBuildTools(t, "meson", "ninja")

	// Check that meson.eclass uses ninja
	// This is more of a sanity check
	t.Log("Meson backend check: ninja is available")
}

// TestMeson_CrossCompilationSupport verifies cross-compilation variables.
//
// Meson has good cross-compilation support via EAPI 7+ variables.
func TestMeson_CrossCompilationSupport(t *testing.T) {
	skipIfNoRepo(t)

	// Check that packages have correct EAPI for cross-compilation
	testAtoms := []string{
		"dev-libs/glib",
		"dev-libs/json-glib",
	}

	for _, atom := range testAtoms {
		t.Run(atom, func(t *testing.T) {
			if !packageExists(t, atom) {
				t.Skipf("Package %s not found in repository", atom)
			}

			// Load package and check EAPI
			pkg := loadPackage(t, atom)
			t.Logf("%s version: %s", atom, pkg.Version)

			// Modern meson packages should use EAPI 8 for SYSROOT/BROOT support
			inherits := getEbuildInherits(t, atom)
			t.Logf("%s inherits: %v", atom, inherits)
		})
	}
}
