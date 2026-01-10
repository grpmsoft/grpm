// Package ebuild implements ebuild execution engine.
//
// This file provides EAPI 8 toolchain functions based on toolchain-funcs.eclass.
// Reference: https://devmanual.gentoo.org/eclass-reference/toolchain-funcs.eclass/
//
// Core functions implemented:
//   - tc-getCC: Get C compiler
//   - tc-getCXX: Get C++ compiler
//   - tc-getAR: Get archiver
//   - tc-getRANLIB: Get archive indexer
//   - tc-getNM: Get symbol lister
//   - tc-getOBJCOPY: Get objcopy
//   - tc-getSTRIP: Get strip
//   - tc-getLD: Get linker
//   - tc-getPKG_CONFIG: Get pkg-config tool
//   - tc-getBUILD_CC: Get build host C compiler
//   - tc-getBUILD_CXX: Get build host C++ compiler
//   - tc-is-cross-compiler: Check if cross-compiling
//   - tc-export: Export multiple toolchain variables
//   - tc-arch: Get target architecture
package ebuild

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ============================================================================
// EAPI 8 Toolchain Functions (toolchain-funcs.eclass)
// ============================================================================

// TcGetCC prints the C compiler command.
//
// Usage: tc-getCC
//
// Detection order per toolchain-funcs.eclass:
//  1. CC environment variable if set
//  2. ${CHOST}-gcc if CHOST is set and command exists
//  3. "gcc" as fallback
func (h *Helpers) TcGetCC(args []string) error {
	cc := h.getTool("CC", "gcc")
	h.writeStdout(cc)
	return nil
}

// TcGetCXX prints the C++ compiler command.
//
// Usage: tc-getCXX
//
// Detection order per toolchain-funcs.eclass:
//  1. CXX environment variable if set
//  2. ${CHOST}-g++ if CHOST is set and command exists
//  3. "g++" as fallback
func (h *Helpers) TcGetCXX(args []string) error {
	cxx := h.getTool("CXX", "g++")
	h.writeStdout(cxx)
	return nil
}

// TcGetLD prints the linker command.
//
// Usage: tc-getLD
//
// Detection order per toolchain-funcs.eclass:
//  1. LD environment variable if set
//  2. ${CHOST}-ld if CHOST is set and command exists
//  3. "ld" as fallback
func (h *Helpers) TcGetLD(args []string) error {
	ld := h.getTool("LD", "ld")
	h.writeStdout(ld)
	return nil
}

// TcGetAR prints the archiver command.
//
// Usage: tc-getAR
//
// Detection order per toolchain-funcs.eclass:
//  1. AR environment variable if set
//  2. ${CHOST}-ar if CHOST is set and command exists
//  3. "ar" as fallback
func (h *Helpers) TcGetAR(args []string) error {
	ar := h.getTool("AR", "ar")
	h.writeStdout(ar)
	return nil
}

// TcGetRANLIB prints the archive indexer command.
//
// Usage: tc-getRANLIB
//
// Detection order per toolchain-funcs.eclass:
//  1. RANLIB environment variable if set
//  2. ${CHOST}-ranlib if CHOST is set and command exists
//  3. "ranlib" as fallback
func (h *Helpers) TcGetRANLIB(args []string) error {
	ranlib := h.getTool("RANLIB", "ranlib")
	h.writeStdout(ranlib)
	return nil
}

// TcGetNM prints the symbol lister command.
//
// Usage: tc-getNM
//
// Detection order per toolchain-funcs.eclass:
//  1. NM environment variable if set
//  2. ${CHOST}-nm if CHOST is set and command exists
//  3. "nm" as fallback
func (h *Helpers) TcGetNM(args []string) error {
	nm := h.getTool("NM", "nm")
	h.writeStdout(nm)
	return nil
}

// TcGetOBJCOPY prints the objcopy command.
//
// Usage: tc-getOBJCOPY
//
// Detection order per toolchain-funcs.eclass:
//  1. OBJCOPY environment variable if set
//  2. ${CHOST}-objcopy if CHOST is set and command exists
//  3. "objcopy" as fallback
func (h *Helpers) TcGetOBJCOPY(args []string) error {
	objcopy := h.getTool("OBJCOPY", "objcopy")
	h.writeStdout(objcopy)
	return nil
}

// TcGetSTRIP prints the strip command.
//
// Usage: tc-getSTRIP
//
// Detection order per toolchain-funcs.eclass:
//  1. STRIP environment variable if set
//  2. ${CHOST}-strip if CHOST is set and command exists
//  3. "strip" as fallback
func (h *Helpers) TcGetSTRIP(args []string) error {
	strip := h.getTool("STRIP", "strip")
	h.writeStdout(strip)
	return nil
}

// TcGetPKG_CONFIG prints the pkg-config command.
//
// Usage: tc-getPKG_CONFIG
//
// Detection order per toolchain-funcs.eclass:
//  1. PKG_CONFIG environment variable if set
//  2. ${CHOST}-pkg-config if CHOST is set and command exists
//  3. "pkg-config" as fallback
func (h *Helpers) TcGetPKG_CONFIG(args []string) error {
	pkgConfig := h.getTool("PKG_CONFIG", "pkg-config")
	h.writeStdout(pkgConfig)
	return nil
}

// TcGetBUILD_CC prints the build host C compiler command.
//
// Usage: tc-getBUILD_CC
//
// This returns the compiler for the build host (CBUILD), not the target (CHOST).
// Used in cross-compilation scenarios where we need to compile tools that run
// on the build machine.
//
// Detection order:
//  1. BUILD_CC environment variable if set
//  2. ${CBUILD}-gcc if CBUILD is set and command exists
//  3. "gcc" as fallback
func (h *Helpers) TcGetBUILD_CC(args []string) error {
	buildCC := h.getBuildTool("BUILD_CC", "gcc")
	h.writeStdout(buildCC)
	return nil
}

// TcGetBUILD_CXX prints the build host C++ compiler command.
//
// Usage: tc-getBUILD_CXX
//
// This returns the C++ compiler for the build host (CBUILD), not the target (CHOST).
//
// Detection order:
//  1. BUILD_CXX environment variable if set
//  2. ${CBUILD}-g++ if CBUILD is set and command exists
//  3. "g++" as fallback
func (h *Helpers) TcGetBUILD_CXX(args []string) error {
	buildCXX := h.getBuildTool("BUILD_CXX", "g++")
	h.writeStdout(buildCXX)
	return nil
}

// TcIsCrossCompiler checks if we are cross-compiling.
//
// Usage: tc-is-cross-compiler && echo "cross-compiling"
//
// Returns exit code 0 (true) if CHOST != CBUILD, 1 (false) otherwise.
// Per toolchain-funcs.eclass, cross-compilation is detected by comparing
// CHOST (target host) with CBUILD (build host).
func (h *Helpers) TcIsCrossCompiler(args []string) error {
	chost := h.getEnvVar("CHOST")
	cbuild := h.getEnvVar("CBUILD")

	// If CBUILD is not set, assume native compilation
	if cbuild == "" {
		return exitFalse()
	}

	// Cross-compiling if CHOST != CBUILD
	if chost != cbuild {
		return nil // exit 0 = true
	}

	return exitFalse() // exit 1 = false
}

// TcExport exports multiple toolchain variables to the environment.
//
// Usage: tc-export CC CXX AR
//
// This function sets the specified toolchain variables in the environment
// by detecting the appropriate tools for the current CHOST.
// Variables are exported as environment variables for child processes.
//
// Output: Prints "export VAR=value" for each variable (shell-compatible).
func (h *Helpers) TcExport(args []string) error {
	if len(args) == 0 {
		// Default variables to export
		args = []string{"CC", "CXX", "LD", "AR", "RANLIB", "NM", "OBJCOPY", "STRIP", "PKG_CONFIG"}
	}

	for _, varName := range args {
		value := h.getToolForExport(varName)
		if value != "" {
			// Set in ExtraVars for ebuild access
			if h.env != nil {
				h.env.SetVar(varName, value)
			}
			// Also set in process environment for exec'd commands
			if err := os.Setenv(varName, value); err != nil {
				h.writeStderr(fmt.Sprintf("tc-export: warning: failed to export %s: %v\n", varName, err))
			}
			// Print export command for shell compatibility
			h.writeStdout(fmt.Sprintf("export %s=%q\n", varName, value))
		}
	}

	return nil
}

// TcArch prints the target architecture.
//
// Usage: tc-arch
//
// Returns architecture name suitable for Portage KEYWORDS.
// Detects from CHOST if set, otherwise uses runtime.GOARCH.
func (h *Helpers) TcArch(args []string) error {
	arch := h.detectArch()
	h.writeStdout(arch)
	return nil
}

// TcArchKernel prints the kernel architecture name.
//
// Usage: tc-arch-kernel
//
// Returns the architecture name as used by the Linux kernel.
// This differs from tc-arch in some cases (e.g., amd64 -> x86_64).
func (h *Helpers) TcArchKernel(args []string) error {
	arch := h.detectKernelArch()
	h.writeStdout(arch)
	return nil
}

// TcEndian prints the target endianness.
//
// Usage: tc-endian
//
// Returns "big" or "little" based on target architecture.
func (h *Helpers) TcEndian(args []string) error {
	endian := h.detectEndian()
	h.writeStdout(endian)
	return nil
}

// ============================================================================
// Internal Helper Functions
// ============================================================================

// getTool returns the appropriate toolchain command for target (CHOST).
//
// Detection order:
//  1. Environment variable (e.g., CC)
//  2. ${CHOST}-tool if CHOST is set and command exists
//  3. Default fallback (e.g., "gcc")
func (h *Helpers) getTool(envVar, defaultTool string) string {
	// Check environment variable first
	if val := h.getEnvVar(envVar); val != "" {
		return val
	}

	// Check CHOST prefix
	if chost := h.getEnvVar("CHOST"); chost != "" {
		crossTool := chost + "-" + defaultTool
		if h.commandExists(crossTool) {
			return crossTool
		}
	}

	return defaultTool
}

// getBuildTool returns the appropriate toolchain command for build host (CBUILD).
//
// Detection order:
//  1. BUILD_* environment variable (e.g., BUILD_CC)
//  2. ${CBUILD}-tool if CBUILD is set and command exists
//  3. Default fallback (e.g., "gcc")
func (h *Helpers) getBuildTool(envVar, defaultTool string) string {
	// Check BUILD_* environment variable first
	if val := h.getEnvVar(envVar); val != "" {
		return val
	}

	// Check CBUILD prefix
	if cbuild := h.getEnvVar("CBUILD"); cbuild != "" {
		crossTool := cbuild + "-" + defaultTool
		if h.commandExists(crossTool) {
			return crossTool
		}
	}

	return defaultTool
}

// getToolForExport returns the tool value for a given variable name.
// Maps variable names to their detection functions.
func (h *Helpers) getToolForExport(varName string) string {
	switch varName {
	case "CC":
		return h.getTool("CC", "gcc")
	case "CXX":
		return h.getTool("CXX", "g++")
	case "LD":
		return h.getTool("LD", "ld")
	case "AR":
		return h.getTool("AR", "ar")
	case "RANLIB":
		return h.getTool("RANLIB", "ranlib")
	case "NM":
		return h.getTool("NM", "nm")
	case "OBJCOPY":
		return h.getTool("OBJCOPY", "objcopy")
	case "STRIP":
		return h.getTool("STRIP", "strip")
	case "PKG_CONFIG":
		return h.getTool("PKG_CONFIG", "pkg-config")
	case "BUILD_CC":
		return h.getBuildTool("BUILD_CC", "gcc")
	case "BUILD_CXX":
		return h.getBuildTool("BUILD_CXX", "g++")
	default:
		return ""
	}
}

// getEnvVar gets an environment variable, checking multiple sources.
// Priority:
//  1. Environment ExtraVars (ebuild-set variables)
//  2. OS environment
func (h *Helpers) getEnvVar(key string) string {
	// Check Environment ExtraVars first
	if h.env != nil {
		if val := h.env.GetVar(key); val != "" {
			return val
		}
	}

	// Fall back to OS environment
	return os.Getenv(key)
}

// getEnvOrDefault gets an environment variable or returns default value.
// Used by eclass_helpers.go for backward compatibility.
func (h *Helpers) getEnvOrDefault(key, defaultVal string) string {
	if val := h.getEnvVar(key); val != "" {
		return val
	}
	return defaultVal
}

// commandExists checks if a command exists in PATH.
func (h *Helpers) commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// detectArch detects the target architecture for Portage KEYWORDS.
//
// First checks CHOST for architecture hint, then falls back to runtime.GOARCH.
func (h *Helpers) detectArch() string {
	// Try to extract arch from CHOST
	if chost := h.getEnvVar("CHOST"); chost != "" {
		if arch := h.archFromCHOST(chost); arch != "" {
			return arch
		}
	}

	// Fall back to Go runtime arch
	return h.goarchToGentooArch(runtime.GOARCH)
}

// detectKernelArch returns the Linux kernel architecture name.
func (h *Helpers) detectKernelArch() string {
	arch := h.detectArch()

	// Map Gentoo arch to kernel arch
	kernelArchMap := map[string]string{
		"amd64": "x86_64",
		"x86":   "x86",
		"arm":   "arm",
		"arm64": "arm64",
		"ppc64": "powerpc",
		"ppc":   "powerpc",
		"riscv": "riscv",
		"s390":  "s390",
		"mips":  "mips",
		"sparc": "sparc",
		"alpha": "alpha",
		"hppa":  "parisc",
		"ia64":  "ia64",
		"loong": "loongarch",
		"m68k":  "m68k",
	}

	if kernelArch, ok := kernelArchMap[arch]; ok {
		return kernelArch
	}

	return arch
}

// detectEndian returns the endianness of the target architecture.
func (h *Helpers) detectEndian() string {
	// Check CHOST for explicit endianness markers first
	if chost := h.getEnvVar("CHOST"); chost != "" {
		// Extract the CPU part (first component)
		parts := strings.Split(chost, "-")
		if len(parts) > 0 {
			cpu := parts[0]

			// Check for explicit little-endian marker (e.g., mipsel, ppc64le, armv7l)
			if strings.HasSuffix(cpu, "el") || strings.HasSuffix(cpu, "le") {
				return "little"
			}

			// Check for explicit big-endian marker (e.g., mipseb, armeb)
			if strings.HasSuffix(cpu, "eb") {
				return "big"
			}

			// MIPS without suffix is big-endian
			// mips, mips64 = big-endian; mipsel, mips64el = little-endian
			if cpu == "mips" || cpu == "mips64" {
				return "big"
			}
		}
	}

	arch := h.detectArch()

	// Big-endian architectures (when CHOST doesn't specify)
	bigEndianArchs := map[string]bool{
		"ppc":   true,
		"ppc64": true,
		"s390":  true,
		"sparc": true,
		"m68k":  true,
		"hppa":  true,
	}

	if bigEndianArchs[arch] {
		return "big"
	}

	return "little"
}

// archFromCHOST extracts the Gentoo architecture from a CHOST triplet.
// Example: "x86_64-pc-linux-gnu" -> "amd64"
func (h *Helpers) archFromCHOST(chost string) string {
	// Extract first component (CPU part)
	parts := strings.Split(chost, "-")
	if len(parts) == 0 {
		return ""
	}

	cpu := parts[0]

	// Map CPU to Gentoo arch
	cpuToArch := map[string]string{
		"x86_64":      "amd64",
		"i686":        "x86",
		"i586":        "x86",
		"i486":        "x86",
		"i386":        "x86",
		"aarch64":     "arm64",
		"arm":         "arm",
		"armv7a":      "arm",
		"armv6j":      "arm",
		"powerpc":     "ppc",
		"powerpc64":   "ppc64",
		"powerpc64le": "ppc64",
		"riscv64":     "riscv",
		"riscv32":     "riscv",
		"s390x":       "s390",
		"s390":        "s390",
		"mips":        "mips",
		"mipsel":      "mips",
		"mips64":      "mips",
		"mips64el":    "mips",
		"sparc":       "sparc",
		"sparc64":     "sparc",
		"alpha":       "alpha",
		"hppa":        "hppa",
		"hppa2.0":     "hppa",
		"ia64":        "ia64",
		"loongarch64": "loong",
		"m68k":        "m68k",
	}

	if arch, ok := cpuToArch[cpu]; ok {
		return arch
	}

	// Try prefix matching for variants (e.g., armv7l -> arm)
	for prefix, arch := range map[string]string{
		"arm":       "arm",
		"mips":      "mips",
		"powerpc":   "ppc",
		"sparc":     "sparc",
		"riscv":     "riscv",
		"loongarch": "loong",
	} {
		if strings.HasPrefix(cpu, prefix) {
			return arch
		}
	}

	return ""
}

// goarchToGentooArch maps Go GOARCH to Gentoo KEYWORDS.
func (h *Helpers) goarchToGentooArch(goarch string) string {
	archMap := map[string]string{
		"amd64":    "amd64",
		"386":      "x86",
		"arm":      "arm",
		"arm64":    "arm64",
		"ppc64":    "ppc64",
		"ppc64le":  "ppc64",
		"riscv64":  "riscv",
		"s390x":    "s390",
		"mips":     "mips",
		"mipsle":   "mips",
		"mips64":   "mips",
		"mips64le": "mips",
		"loong64":  "loong",
	}

	if arch, ok := archMap[goarch]; ok {
		return arch
	}

	return goarch
}
