// Package ebuild implements ebuild execution engine.
//
// This file provides multilib.eclass support for ebuild execution.
// The multilib eclass provides functions for handling multiple ABIs
// (Application Binary Interfaces) on multilib systems.
//
// Reference: https://devmanual.gentoo.org/eclass-reference/multilib.eclass/
package ebuild

import (
	"fmt"
	"strings"
)

// ============================================================================
// ABI Types and Constants
// ============================================================================

// ABI represents an Application Binary Interface configuration.
type ABI struct {
	// Name is the ABI identifier (e.g., "amd64", "x86")
	Name string

	// LibDir is the library directory (e.g., "lib64", "lib32", "lib")
	LibDir string

	// CHost is the target triplet (e.g., "x86_64-pc-linux-gnu")
	CHost string

	// CFlags are additional compiler flags for this ABI
	CFlags string

	// LDFlags are additional linker flags for this ABI
	LDFlags string
}

// CommonABIs defines standard ABI configurations for common architectures.
var CommonABIs = map[string][]ABI{
	"amd64": {
		{Name: "amd64", LibDir: "lib64", CHost: "x86_64-pc-linux-gnu"},
		{Name: "x86", LibDir: "lib32", CHost: "i686-pc-linux-gnu", CFlags: "-m32", LDFlags: "-m32"},
	},
	"x86": {
		{Name: "x86", LibDir: "lib", CHost: "i686-pc-linux-gnu"},
	},
	"arm64": {
		{Name: "arm64", LibDir: "lib64", CHost: "aarch64-unknown-linux-gnu"},
		{Name: "arm", LibDir: "lib", CHost: "armv7a-unknown-linux-gnueabihf"},
	},
	"arm": {
		{Name: "arm", LibDir: "lib", CHost: "armv7a-unknown-linux-gnueabihf"},
	},
	"ppc64": {
		{Name: "ppc64", LibDir: "lib64", CHost: "powerpc64-unknown-linux-gnu"},
		{Name: "ppc", LibDir: "lib32", CHost: "powerpc-unknown-linux-gnu", CFlags: "-m32"},
	},
}

// ============================================================================
// Multilib Eclass Registration
// ============================================================================

// MultilibEclass represents the multilib.eclass implementation.
type MultilibEclass struct{}

// Name returns the eclass name.
func (e *MultilibEclass) Name() string {
	return "multilib"
}

// ExportedFunctions returns the phase functions exported by this eclass.
func (e *MultilibEclass) ExportedFunctions() []string {
	return []string{} // multilib doesn't export phase functions
}

// Variables returns the default variables set by this eclass.
func (e *MultilibEclass) Variables() map[string]string {
	return map[string]string{}
}

// ============================================================================
// Core Multilib Functions
// ============================================================================

// Note: GetLibdir is defined in eclass_helpers.go

// computeLibdir determines the library directory.
func (h *Helpers) computeLibdir() string {
	// Check if ABI is explicitly set
	if abi := h.getEnvOrDefault("ABI", ""); abi != "" {
		return h.computeABILibdir(abi)
	}

	// Check LIBDIR_* variables
	if libdir := h.getEnvOrDefault("LIBDIR", ""); libdir != "" {
		return libdir
	}

	// Default based on CHOST
	chost := h.getEnvOrDefault("CHOST", "")
	if strings.Contains(chost, "x86_64") || strings.Contains(chost, "aarch64") ||
		strings.Contains(chost, "ppc64") || strings.Contains(chost, "s390x") {
		return "lib64"
	}

	return "lib"
}

// GetABILibdir returns the LIBDIR for a specific ABI.
//
// Usage: $(get_abi_LIBDIR amd64)
func (h *Helpers) GetABILibdir(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "get_abi_LIBDIR: requires ABI argument"}
	}

	libdir := h.computeABILibdir(args[0])
	h.writeStdout(libdir)
	return nil
}

// computeABILibdir returns the library directory for a specific ABI.
func (h *Helpers) computeABILibdir(abi string) string {
	// Check environment variable first
	varName := "LIBDIR_" + abi
	if libdir := h.getEnvOrDefault(varName, ""); libdir != "" {
		return libdir
	}

	// Look up in common ABIs
	for _, abis := range CommonABIs {
		for _, a := range abis {
			if a.Name == abi {
				return a.LibDir
			}
		}
	}

	// Default fallbacks
	switch abi {
	case "amd64", "arm64", "ppc64", "s390x":
		return "lib64"
	case "x86", "arm", "ppc":
		if h.isMultilib() {
			return "lib32"
		}
		return "lib"
	default:
		return "lib"
	}
}

// GetABIChost returns the CHOST for a specific ABI.
//
// Usage: $(get_abi_CHOST amd64)
func (h *Helpers) GetABIChost(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "get_abi_CHOST: requires ABI argument"}
	}

	abi := args[0]

	// Check environment variable first
	varName := "CHOST_" + abi
	if chost := h.getEnvOrDefault(varName, ""); chost != "" {
		h.writeStdout(chost)
		return nil
	}

	// Look up in common ABIs
	for _, abis := range CommonABIs {
		for _, a := range abis {
			if a.Name == abi {
				h.writeStdout(a.CHost)
				return nil
			}
		}
	}

	// Fall back to current CHOST
	h.writeStdout(h.getEnvOrDefault("CHOST", ""))
	return nil
}

// GetABICflags returns the CFLAGS for a specific ABI.
//
// Usage: $(get_abi_CFLAGS x86)
func (h *Helpers) GetABICflags(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "get_abi_CFLAGS: requires ABI argument"}
	}

	abi := args[0]

	// Check environment variable first
	varName := "CFLAGS_" + abi
	if cflags := h.getEnvOrDefault(varName, ""); cflags != "" {
		h.writeStdout(cflags)
		return nil
	}

	// Look up in common ABIs
	for _, abis := range CommonABIs {
		for _, a := range abis {
			if a.Name == abi {
				h.writeStdout(a.CFlags)
				return nil
			}
		}
	}

	h.writeStdout("")
	return nil
}

// GetABILdflags returns the LDFLAGS for a specific ABI.
//
// Usage: $(get_abi_LDFLAGS x86)
func (h *Helpers) GetABILdflags(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "get_abi_LDFLAGS: requires ABI argument"}
	}

	abi := args[0]

	// Check environment variable first
	varName := "LDFLAGS_" + abi
	if ldflags := h.getEnvOrDefault(varName, ""); ldflags != "" {
		h.writeStdout(ldflags)
		return nil
	}

	// Look up in common ABIs
	for _, abis := range CommonABIs {
		for _, a := range abis {
			if a.Name == abi {
				h.writeStdout(a.LDFlags)
				return nil
			}
		}
	}

	h.writeStdout("")
	return nil
}

// MultilibEnv sets up the environment for a specific ABI.
//
// Usage: multilib_env x86
//
// Sets: ABI, CHOST, CFLAGS, LDFLAGS, LIBDIR_*
func (h *Helpers) MultilibEnv(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "multilib_env: requires ABI argument"}
	}

	abi := args[0]
	return h.setupABIEnvironment(abi)
}

// setupABIEnvironment configures environment for a specific ABI.
func (h *Helpers) setupABIEnvironment(abiName string) error {
	// Find ABI definition
	var abi *ABI
	for _, abis := range CommonABIs {
		for i := range abis {
			if abis[i].Name == abiName {
				abi = &abis[i]
				break
			}
		}
	}

	if abi == nil {
		return &DieError{Message: fmt.Sprintf("unknown ABI: %s", abiName)}
	}

	// Set ABI variable
	h.setEnvVar("ABI", abi.Name)

	// Set LIBDIR_<abi>
	h.setEnvVar("LIBDIR_"+abi.Name, abi.LibDir)

	// Set CHOST_<abi>
	h.setEnvVar("CHOST_"+abi.Name, abi.CHost)

	// Append ABI-specific CFLAGS
	if abi.CFlags != "" {
		cflags := h.getEnvOrDefault("CFLAGS", "")
		if cflags != "" {
			cflags += " " + abi.CFlags
		} else {
			cflags = abi.CFlags
		}
		h.setEnvVar("CFLAGS", cflags)
	}

	// Append ABI-specific LDFLAGS
	if abi.LDFlags != "" {
		ldflags := h.getEnvOrDefault("LDFLAGS", "")
		if ldflags != "" {
			ldflags += " " + abi.LDFlags
		} else {
			ldflags = abi.LDFlags
		}
		h.setEnvVar("LDFLAGS", ldflags)
	}

	return nil
}

// ============================================================================
// Multilib Query Functions
// ============================================================================

// IsMultilib returns true if the system supports multilib.
//
// Usage: if multilib_is_native_abi; then ... fi
func (h *Helpers) MultilibIsNativeABI(args []string) error {
	currentABI := h.getEnvOrDefault("ABI", "")
	defaultABI := h.getDefaultABI()

	if currentABI == defaultABI || currentABI == "" {
		return nil // true
	}
	return exitFalse()
}

// getDefaultABI returns the default (native) ABI for the system.
func (h *Helpers) getDefaultABI() string {
	// Check DEFAULT_ABI
	if defaultABI := h.getEnvOrDefault("DEFAULT_ABI", ""); defaultABI != "" {
		return defaultABI
	}

	// Determine from CHOST
	chost := h.getEnvOrDefault("CHOST", "")
	switch {
	case strings.Contains(chost, "x86_64"):
		return "amd64"
	case strings.Contains(chost, "i686"), strings.Contains(chost, "i386"):
		return "x86"
	case strings.Contains(chost, "aarch64"):
		return "arm64"
	case strings.Contains(chost, "armv7"):
		return "arm"
	case strings.Contains(chost, "powerpc64"):
		return "ppc64"
	case strings.Contains(chost, "powerpc"):
		return "ppc"
	default:
		return "default"
	}
}

// isMultilib checks if the system is a multilib system.
func (h *Helpers) isMultilib() bool {
	// Check USE flag
	if h.isUseEnabled("multilib") {
		return true
	}

	// Check if multiple ABIs are defined
	multiABI := h.getEnvOrDefault("MULTILIB_ABIS", "")
	if multiABI != "" && strings.Contains(multiABI, " ") {
		return true
	}

	return false
}

// GetAllABIs returns all enabled ABIs for multilib.
//
// Usage: $(get_all_abis)
func (h *Helpers) GetAllABIs(args []string) error {
	abis := h.getEnabledABIs()
	var names []string
	for _, abi := range abis {
		names = append(names, abi.Name)
	}
	h.writeStdout(strings.Join(names, " "))
	return nil
}

// getEnabledABIs returns the list of enabled ABIs.
func (h *Helpers) getEnabledABIs() []ABI {
	var result []ABI

	// Check MULTILIB_ABIS
	multiABI := h.getEnvOrDefault("MULTILIB_ABIS", "")
	if multiABI != "" {
		for _, name := range strings.Fields(multiABI) {
			for _, abis := range CommonABIs {
				for _, abi := range abis {
					if abi.Name == name {
						result = append(result, abi)
					}
				}
			}
		}
		if len(result) > 0 {
			return result
		}
	}

	// Check abi_x86_* USE flags
	if h.isUseEnabled("abi_x86_64") {
		result = append(result, CommonABIs["amd64"][0])
	}
	if h.isUseEnabled("abi_x86_32") {
		if len(CommonABIs["amd64"]) > 1 {
			result = append(result, CommonABIs["amd64"][1])
		}
	}

	// Default to native ABI
	if len(result) == 0 {
		defaultABI := h.getDefaultABI()
		for _, abis := range CommonABIs {
			for _, abi := range abis {
				if abi.Name == defaultABI {
					result = append(result, abi)
					return result
				}
			}
		}
	}

	return result
}
