// Package ebuild implements ebuild execution engine.
//
// This file provides multilib-build.eclass support for ebuild execution.
// The multilib-build eclass provides functions for building packages
// for multiple ABIs in a single merge.
//
// Reference: https://devmanual.gentoo.org/eclass-reference/multilib-build.eclass/
package ebuild

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ============================================================================
// Multilib Build Eclass Registration
// ============================================================================

// MultilibBuildEclass represents the multilib-build.eclass implementation.
type MultilibBuildEclass struct{}

// Name returns the eclass name.
func (e *MultilibBuildEclass) Name() string {
	return "multilib-build"
}

// ExportedFunctions returns the phase functions exported by this eclass.
func (e *MultilibBuildEclass) ExportedFunctions() []string {
	return []string{
		"src_configure",
		"src_compile",
		"src_test",
		"src_install",
	}
}

// Variables returns the default variables set by this eclass.
func (e *MultilibBuildEclass) Variables() map[string]string {
	return map[string]string{
		"MULTILIB_COMPAT": "",
	}
}

// ============================================================================
// Multilib Build Phase Functions
// ============================================================================

// MultilibBuildSrcConfigure configures for each enabled ABI.
//
// This is the src_configure phase exported by multilib-build.
func (h *Helpers) MultilibBuildSrcConfigure(args []string) error {
	return h.multilibForeachABIDo(func(abi ABI) error {
		h.writeStdout(fmt.Sprintf(">>> Configuring for ABI: %s\n", abi.Name))
		return h.multilibSrcConfigure(abi)
	})
}

// MultilibBuildSrcCompile compiles for each enabled ABI.
func (h *Helpers) MultilibBuildSrcCompile(args []string) error {
	return h.multilibForeachABIDo(func(abi ABI) error {
		h.writeStdout(fmt.Sprintf(">>> Compiling for ABI: %s\n", abi.Name))
		return h.multilibSrcCompile(abi)
	})
}

// MultilibBuildSrcTest tests for each enabled ABI.
func (h *Helpers) MultilibBuildSrcTest(args []string) error {
	return h.multilibForeachABIDo(func(abi ABI) error {
		h.writeStdout(fmt.Sprintf(">>> Testing for ABI: %s\n", abi.Name))
		return h.multilibSrcTest(abi)
	})
}

// MultilibBuildSrcInstall installs for each enabled ABI.
func (h *Helpers) MultilibBuildSrcInstall(args []string) error {
	return h.multilibForeachABIDo(func(abi ABI) error {
		h.writeStdout(fmt.Sprintf(">>> Installing for ABI: %s\n", abi.Name))
		return h.multilibSrcInstall(abi)
	})
}

// ============================================================================
// Multilib Foreach Implementation
// ============================================================================

// MultilibForeachABI runs a command for each enabled ABI.
//
// Usage: multilib_foreach_abi emake
func (h *Helpers) MultilibForeachABI(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "multilib_foreach_abi: requires command argument"}
	}

	command := args[0]
	cmdArgs := args[1:]

	return h.multilibForeachABIDo(func(abi ABI) error {
		return h.executeCommand(command, cmdArgs)
	})
}

// multilibForeachABIDo runs a function for each enabled ABI.
func (h *Helpers) multilibForeachABIDo(fn func(abi ABI) error) error {
	abis := h.getEnabledABIs()
	if len(abis) == 0 {
		return &DieError{Message: "multilib_foreach_abi: no ABIs enabled"}
	}

	// Save original environment
	origABI := h.getEnvOrDefault("ABI", "")
	origCFLAGS := h.getEnvOrDefault("CFLAGS", "")
	origLDFLAGS := h.getEnvOrDefault("LDFLAGS", "")
	origBuildDir := h.getEnvOrDefault("BUILD_DIR", "")

	for _, abi := range abis {
		// Setup ABI environment
		if err := h.setupABIEnvironment(abi.Name); err != nil {
			return err
		}

		// Set ABI-specific BUILD_DIR
		workdir := h.getEnvOrDefault("WORKDIR", "")
		if workdir != "" {
			buildDir := filepath.Join(workdir, fmt.Sprintf("build-%s", abi.Name))
			h.setEnvVar("BUILD_DIR", buildDir)
		}

		// Run the function
		if err := fn(abi); err != nil {
			return fmt.Errorf("multilib_foreach_abi: failed for %s: %w", abi.Name, err)
		}
	}

	// Restore original environment
	h.setEnvVar("ABI", origABI)
	h.setEnvVar("CFLAGS", origCFLAGS)
	h.setEnvVar("LDFLAGS", origLDFLAGS)
	if origBuildDir != "" {
		h.setEnvVar("BUILD_DIR", origBuildDir)
	}

	return nil
}

// ============================================================================
// Multilib Phase Implementations
// ============================================================================

// multilibSrcConfigure runs configure for a specific ABI.
func (h *Helpers) multilibSrcConfigure(abi ABI) error {
	// Default: run econf
	return h.Econf(nil)
}

// multilibSrcCompile runs compilation for a specific ABI.
func (h *Helpers) multilibSrcCompile(abi ABI) error {
	// Default: run emake
	return h.Emake(nil)
}

// multilibSrcTest runs tests for a specific ABI.
func (h *Helpers) multilibSrcTest(_ ABI) error {
	// Default: no test
	return nil
}

// multilibSrcInstall runs installation for a specific ABI.
func (h *Helpers) multilibSrcInstall(_ ABI) error {
	// Default: run emake install
	return h.Emake([]string{"install", "DESTDIR=" + h.getEnvOrDefault("D", "")})
}

// ============================================================================
// Multilib Dependency Generation
// ============================================================================

// MultilibUsedep generates the USEDEP string for multilib.
//
// Usage: RDEPEND="dev-libs/foo[${MULTILIB_USEDEP}]"
//
// Returns: "abi_x86_32(-)?,abi_x86_64(-)?"
func (h *Helpers) MultilibUsedep(args []string) error {
	usedep := h.computeMultilibUsedep()
	h.writeStdout(usedep)
	return nil
}

// computeMultilibUsedep generates the USEDEP string.
func (h *Helpers) computeMultilibUsedep() string {
	// Check MULTILIB_USEDEP if set
	if usedep := h.getEnvOrDefault("MULTILIB_USEDEP", ""); usedep != "" {
		return usedep
	}

	// Generate based on MULTILIB_COMPAT or default ABIs
	compat := h.getEnvOrDefault("MULTILIB_COMPAT", "")
	var parts []string

	if compat != "" {
		// Parse MULTILIB_COMPAT array
		for _, abi := range strings.Fields(compat) {
			parts = append(parts, fmt.Sprintf("%s(-)?,", abi))
		}
	} else {
		// Default for amd64 multilib
		parts = []string{
			"abi_x86_32(-)?,",
			"abi_x86_64(-)?,",
		}
	}

	// Remove trailing comma from last element
	if len(parts) > 0 {
		last := len(parts) - 1
		parts[last] = strings.TrimSuffix(parts[last], ",")
	}

	return strings.Join(parts, "")
}

// MultilibMinimalAllABIs checks if only native ABI should be used.
//
// Usage: if multilib_build_binaries; then ... fi
func (h *Helpers) MultilibMinimalAllABIs(args []string) error {
	if h.isUseEnabled("multilib-minimal") {
		return nil // true
	}
	return exitFalse()
}

// ============================================================================
// Multilib Installation Helpers
// ============================================================================

// MultilibDolibs installs libraries to the correct libdir.
//
// Usage: multilib_do_libs libfoo.so.1
func (h *Helpers) MultilibDolibs(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "multilib_dolibs: requires file argument"}
	}

	// Set libdir based on current ABI before calling dolib
	h.libDir = h.computeLibdir()
	return h.Dolib(args)
}

// MultilibNativeUseBuild runs command only for native ABI.
//
// Usage: multilib_native_use_build command args...
func (h *Helpers) MultilibNativeUseBuild(args []string) error {
	// Check if we're building native ABI
	currentABI := h.getEnvOrDefault("ABI", "")
	defaultABI := h.getDefaultABI()

	if currentABI != defaultABI && currentABI != "" {
		return nil // Skip for non-native ABIs
	}

	if len(args) < 1 {
		return nil
	}

	return h.executeCommand(args[0], args[1:])
}

// Note: MultilibNativeUseEnable is defined in eclass_helpers.go
// Note: Dolib is defined in helpers_install.go
