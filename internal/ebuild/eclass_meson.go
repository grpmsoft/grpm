// Package ebuild implements ebuild execution engine.
//
// This file provides meson.eclass registration and support.
// The meson.eclass wraps the Meson build system helpers and provides
// EXPORT_FUNCTIONS for standard phase implementations.
//
// Reference: https://devmanual.gentoo.org/eclass-reference/meson.eclass/
//
// Key exports:
//   - src_configure -> meson_src_configure
//   - src_compile -> meson_src_compile
//   - src_test -> meson_src_test
//   - src_install -> meson_src_install
//
// Key functions:
//   - meson_use: Convert USE flag to -Dfoo=enabled/disabled
//   - meson_feature: Same as meson_use (feature option type)
//   - meson_use_bool: Convert USE flag to -Dfoo=true/false
package ebuild

// MesonEclass represents the meson.eclass implementation.
//
// The meson.eclass provides integration with the Meson build system,
// offering standard phase implementations and helper functions for
// converting USE flags to Meson options.
//
// When an ebuild inherits meson, the following phases are provided:
//   - src_configure: Runs meson setup with Gentoo-standard options
//   - src_compile: Builds using meson compile (ninja backend)
//   - src_test: Runs meson test
//   - src_install: Installs using meson install with DESTDIR
type MesonEclass struct{}

// Name returns the eclass name.
func (e *MesonEclass) Name() string {
	return "meson"
}

// ExportedFunctions returns the list of phase functions exported by this eclass.
//
// Per meson.eclass:
//
//	EXPORT_FUNCTIONS src_configure src_compile src_test src_install
//
// When an ebuild inherits meson, these phases are automatically provided
// unless the ebuild defines its own.
func (e *MesonEclass) ExportedFunctions() []string {
	return []string{
		"src_configure",
		"src_compile",
		"src_test",
		"src_install",
	}
}

// Variables returns the default variables set by meson.eclass.
//
// Key variables:
//   - EMESON_BUILDTYPE: Build type (default: "plain" for Gentoo)
//   - EMESON_WRAP_MODE: Wrap mode (default: "nodownload" to prevent bundling)
//
// The "plain" buildtype is used because Gentoo manages optimization flags
// through CFLAGS/CXXFLAGS, not through build system settings.
func (e *MesonEclass) Variables() map[string]string {
	return map[string]string{
		"EMESON_BUILDTYPE": MesonBuildTypePlain,
	}
}

// RegisterHelpers registers Go helper functions for meson.eclass.
//
// This binds the meson_* bash functions to their Go implementations:
//   - meson_src_configure -> Helpers.MesonSrcConfigure
//   - meson_src_compile -> Helpers.MesonSrcCompile
//   - meson_src_test -> Helpers.MesonSrcTest
//   - meson_src_install -> Helpers.MesonSrcInstall
//   - meson_use -> Helpers.MesonUse
//   - meson_feature -> Helpers.MesonFeature
//   - meson_use_bool -> Helpers.MesonUseBool
func (e *MesonEclass) RegisterHelpers(h *Helpers) {
	// Register the eclass functions through the function registry
	// These functions are defined in build_meson.go
	h.registerMesonFunctions()
}

// registerMesonFunctions registers meson eclass functions with the interpreter.
//
// This method sets up the function dispatch so that when bash code calls
// meson_src_configure, etc., the corresponding Go methods are invoked.
func (h *Helpers) registerMesonFunctions() {
	// Phase functions - these implement the standard meson.eclass phases
	h.eclassRegistry.RegisterFunction("meson", "meson_src_configure")
	h.eclassRegistry.RegisterFunction("meson", "meson_src_compile")
	h.eclassRegistry.RegisterFunction("meson", "meson_src_test")
	h.eclassRegistry.RegisterFunction("meson", "meson_src_install")

	// Helper functions - these convert USE flags to meson options
	h.eclassRegistry.RegisterFunction("meson", "meson_use")
	h.eclassRegistry.RegisterFunction("meson", "meson_feature")
	h.eclassRegistry.RegisterFunction("meson", "meson_use_bool")

	// Low-level meson wrapper (rarely used directly)
	h.eclassRegistry.RegisterFunction("meson", "meson")
}

// MesonEclassPhaseHandler returns the appropriate phase handler function name.
//
// This is used by the executor to map phase names to meson eclass functions.
// For example, when src_configure is called and meson is inherited,
// meson_src_configure should be invoked instead.
func MesonEclassPhaseHandler(phase string) string {
	switch phase {
	case "src_configure":
		return "meson_src_configure"
	case "src_compile":
		return "meson_src_compile"
	case "src_test":
		return "meson_src_test"
	case "src_install":
		return "meson_src_install"
	default:
		return ""
	}
}

// MesonEclassDefaultVariables returns the default environment variables
// that should be set when meson.eclass is inherited.
func MesonEclassDefaultVariables() map[string]string {
	return map[string]string{
		// Build type: "plain" means no build-system-level optimization.
		// Gentoo manages optimization through CFLAGS/CXXFLAGS instead.
		"EMESON_BUILDTYPE": MesonBuildTypePlain,

		// Wrap mode: "nodownload" prevents meson from downloading dependencies.
		// This is required for Gentoo's package management model where all
		// dependencies are explicitly declared and installed via the system.
		// Note: This is set via getMesonWrapMode() in build_meson.go, not here,
		// because the default is already "nodownload" in that function.
	}
}

// IsMesonEclassFunction checks if a function name is provided by meson.eclass.
func IsMesonEclassFunction(name string) bool {
	switch name {
	case "meson_src_configure",
		"meson_src_compile",
		"meson_src_test",
		"meson_src_install",
		"meson_use",
		"meson_feature",
		"meson_use_bool",
		"meson":
		return true
	default:
		return false
	}
}
