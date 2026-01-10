// Package ebuild implements ebuild execution engine.
//
// This file provides Meson build system support for ebuild execution.
// Implements the meson.eclass helper functions for building Meson-based packages.
//
// Reference: https://devmanual.gentoo.org/eclass-reference/meson.eclass/
//
// Core functions implemented:
//   - Meson: Run meson with Gentoo-standard options
//   - MesonSrcConfigure: Configure phase using meson setup
//   - MesonSrcCompile: Compile phase using meson compile
//   - MesonSrcInstall: Install phase using meson install
//   - MesonSrcTest: Test phase using meson test
//   - MesonUse: Generate -Doption=enabled/disabled based on USE flag
//   - MesonFeature: Generate -Doption=enabled/disabled/auto
package ebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ============================================================================
// Meson Configuration Constants
// ============================================================================

// Meson build types per meson.eclass.
const (
	// MesonBuildTypePlain is the default build type for Gentoo (no optimization flags).
	MesonBuildTypePlain = "plain"

	// MesonBuildTypeDebug includes debug symbols, no optimization.
	MesonBuildTypeDebug = "debug"

	// MesonBuildTypeDebugOptimized has debug symbols with optimization.
	MesonBuildTypeDebugOptimized = "debugoptimized"

	// MesonBuildTypeRelease is optimized release build.
	MesonBuildTypeRelease = "release"

	// MesonBuildTypeMinSize is optimized for size.
	MesonBuildTypeMinSize = "minsize"
)

// Meson wrap modes per meson.eclass.
const (
	// MesonWrapModeNoDownload prevents downloading dependencies (Gentoo default).
	MesonWrapModeNoDownload = "nodownload"

	// MesonWrapModeNone doesn't use subprojects at all.
	MesonWrapModeNone = "nofallback"
)

// Meson feature option values.
const (
	// MesonFeatureEnabled forces feature on.
	MesonFeatureEnabled = "enabled"

	// MesonFeatureDisabled forces feature off.
	MesonFeatureDisabled = "disabled"

	// MesonFeatureAuto lets meson decide.
	MesonFeatureAuto = "auto"
)

// ============================================================================
// Meson Helper Functions
// ============================================================================

// Meson runs meson with standard Gentoo options.
//
// Usage: meson [options] [source] [builddir]
//
// This is the low-level meson wrapper. For the standard configure phase,
// use MesonSrcConfigure instead.
//
// Options are passed directly to meson. If no paths are specified,
// EMESON_SOURCE (defaults to ${S}) and BUILD_DIR are used.
//
// Reference: meson.eclass meson function.
func (h *Helpers) Meson(args []string) error {
	buildDir := h.getMesonBuildDir()
	sourceDir := h.getMesonSource()

	// Create build directory if it doesn't exist
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("meson: failed to create build directory: %v", err)}
	}

	// Build meson setup arguments
	mesonArgs := []string{"setup"}
	mesonArgs = append(mesonArgs, h.buildMesonArgs()...)
	mesonArgs = append(mesonArgs, args...)

	// Add source and build directories as the last arguments
	// Meson expects: meson setup [options] sourcedir builddir
	hasSourceDir := false
	hasBuildDir := false
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			if !hasSourceDir {
				hasSourceDir = true
			} else {
				hasBuildDir = true
			}
		}
	}

	if !hasSourceDir {
		mesonArgs = append(mesonArgs, sourceDir)
	}
	if !hasBuildDir {
		mesonArgs = append(mesonArgs, buildDir)
	}

	h.writeStdout(fmt.Sprintf(">>> Running: meson %s\n", strings.Join(mesonArgs, " ")))

	return h.runCommand("meson", mesonArgs)
}

// MesonSrcConfigure implements the default meson src_configure phase.
//
// Usage: meson_src_configure
//
// Sets standard meson options including:
//   - --prefix: ${EPREFIX}/usr
//   - --libdir: $(get_libdir)
//   - --localstatedir: ${EPREFIX}/var/lib
//   - --sysconfdir: ${EPREFIX}/etc
//   - --buildtype: ${EMESON_BUILDTYPE:-plain}
//   - --wrap-mode: nodownload
//
// Additional arguments can be passed via emesonargs or MYMESONARGS environment variable.
//
// Reference: meson.eclass meson_src_configure().
func (h *Helpers) MesonSrcConfigure(args []string) error {
	h.writeStdout(">>> meson_src_configure\n")

	// Collect all arguments
	allArgs := make([]string, 0)

	// Add user-specified arguments from MYMESONARGS
	if mymesonargs := h.getEnvVar("MYMESONARGS"); mymesonargs != "" {
		allArgs = append(allArgs, strings.Fields(mymesonargs)...)
	}

	// Add emesonargs (ebuild-set variable)
	if emesonargs := h.getEnvVar("emesonargs"); emesonargs != "" {
		allArgs = append(allArgs, strings.Fields(emesonargs)...)
	}

	// Add any direct arguments
	allArgs = append(allArgs, args...)

	return h.Meson(allArgs)
}

// MesonSrcCompile implements the default meson src_compile phase.
//
// Usage: meson_src_compile
//
// Calls meson compile (or ninja) in BUILD_DIR.
// Uses ninja backend with parallelization from MAKEOPTS.
//
// Reference: meson.eclass meson_src_compile().
func (h *Helpers) MesonSrcCompile(args []string) error {
	h.writeStdout(">>> meson_src_compile\n")

	buildDir := h.getMesonBuildDir()

	// Check if build directory exists
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		return &DieError{Message: "meson_src_compile: build directory does not exist (run meson_src_configure first)"}
	}

	// Use meson compile for backend-agnostic building
	compileArgs := []string{"compile", "-C", buildDir}

	// Add parallel jobs from MAKEOPTS
	if jobs := h.extractParallelJobs(); jobs > 0 {
		compileArgs = append(compileArgs, "-j", fmt.Sprintf("%d", jobs))
	}

	// Add verbose flag if requested
	if h.getEnvVar("MESON_VERBOSE") == "1" {
		compileArgs = append(compileArgs, "-v")
	}

	// Add any additional arguments
	compileArgs = append(compileArgs, args...)

	h.writeStdout(fmt.Sprintf(">>> Running: meson %s\n", strings.Join(compileArgs, " ")))

	return h.runCommand("meson", compileArgs)
}

// MesonSrcInstall implements the default meson src_install phase.
//
// Usage: meson_src_install
//
// Runs meson install with DESTDIR set to ${D}.
//
// Reference: meson.eclass meson_src_install().
func (h *Helpers) MesonSrcInstall(args []string) error {
	h.writeStdout(">>> meson_src_install\n")

	buildDir := h.getMesonBuildDir()

	// Check if build directory exists
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		return &DieError{Message: "meson_src_install: build directory does not exist"}
	}

	// Get DESTDIR (D environment variable)
	destdir := h.getDestDir()
	if destdir == "" {
		return &DieError{Message: "meson_src_install: D (destination directory) not set"}
	}

	// Create destination directory
	if err := os.MkdirAll(destdir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("meson_src_install: failed to create destination directory: %v", err)}
	}

	// Use meson install
	installArgs := []string{"install", "-C", buildDir, "--destdir", destdir}

	// Add --no-rebuild flag to avoid rebuilding during install
	installArgs = append(installArgs, "--no-rebuild")

	// Add any additional arguments
	installArgs = append(installArgs, args...)

	h.writeStdout(fmt.Sprintf(">>> Running: meson %s\n", strings.Join(installArgs, " ")))

	return h.runCommand("meson", installArgs)
}

// MesonSrcTest implements the default meson src_test phase.
//
// Usage: meson_src_test
//
// Runs meson test in the build directory with appropriate options.
//
// Reference: meson.eclass meson_src_test().
func (h *Helpers) MesonSrcTest(args []string) error {
	h.writeStdout(">>> meson_src_test\n")

	buildDir := h.getMesonBuildDir()

	// Check if build directory exists
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		return &DieError{Message: "meson_src_test: build directory does not exist"}
	}

	// Build meson test arguments
	testArgs := []string{"test", "-C", buildDir}

	// Add --no-rebuild to avoid rebuilding during test
	testArgs = append(testArgs, "--no-rebuild")

	// Add --print-errorlogs for verbose test output on failure
	testArgs = append(testArgs, "--print-errorlogs")

	// Add parallel jobs from MAKEOPTS
	if jobs := h.extractParallelJobs(); jobs > 0 {
		testArgs = append(testArgs, "--num-processes", fmt.Sprintf("%d", jobs))
	}

	// Add any additional arguments
	testArgs = append(testArgs, args...)

	h.writeStdout(fmt.Sprintf(">>> Running: meson %s\n", strings.Join(testArgs, " ")))

	return h.runCommand("meson", testArgs)
}

// MesonUse returns a meson option based on USE flag state.
//
// Usage: meson_use <use-flag> [option-name]
//
// If the USE flag is enabled, returns "-Doption=enabled".
// If the USE flag is disabled, returns "-Doption=disabled".
//
// If option-name is not provided, uses the use-flag name.
//
// Example:
//
//	meson_use ssl       -> -Dssl=enabled (if ssl is in USE)
//	meson_use ssl tls   -> -Dtls=enabled (if ssl is in USE)
//
// Reference: meson.eclass meson_use().
func (h *Helpers) MesonUse(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "meson_use: missing USE flag argument"}
	}

	useFlag := args[0]
	optionName := useFlag
	if len(args) >= 2 {
		optionName = args[1]
	}

	enabled := h.hasUseFlag(useFlag)
	value := MesonFeatureDisabled
	if enabled {
		value = MesonFeatureEnabled
	}

	h.writeStdout(fmt.Sprintf("-D%s=%s", optionName, value))
	return nil
}

// MesonFeature returns a meson feature option based on USE flag state.
//
// Usage: meson_feature <use-flag> [option-name]
//
// This is similar to MesonUse but explicitly uses the "feature" option type.
// Returns "-Doption=enabled" or "-Doption=disabled".
//
// Reference: meson.eclass meson_feature().
func (h *Helpers) MesonFeature(args []string) error {
	// MesonFeature is identical to MesonUse for enabled/disabled output
	return h.MesonUse(args)
}

// MesonUseBool returns a meson boolean option based on USE flag state.
//
// Usage: meson_use_bool <use-flag> [option-name]
//
// If the USE flag is enabled, returns "-Doption=true".
// If the USE flag is disabled, returns "-Doption=false".
//
// Reference: Custom helper for boolean options.
func (h *Helpers) MesonUseBool(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "meson_use_bool: missing USE flag argument"}
	}

	useFlag := args[0]
	optionName := useFlag
	if len(args) >= 2 {
		optionName = args[1]
	}

	enabled := h.hasUseFlag(useFlag)
	value := "false"
	if enabled {
		value = "true"
	}

	h.writeStdout(fmt.Sprintf("-D%s=%s", optionName, value))
	return nil
}

// ============================================================================
// Meson Internal Helper Functions
// ============================================================================

// getMesonSource returns the Meson source directory.
//
// Uses EMESON_SOURCE if set, otherwise defaults to ${S}.
func (h *Helpers) getMesonSource() string {
	if sourceDir := h.getEnvVar("EMESON_SOURCE"); sourceDir != "" {
		return sourceDir
	}
	return h.getWorkDir()
}

// getMesonBuildDir returns the Meson build directory.
//
// Uses BUILD_DIR if set, otherwise defaults to ${WORKDIR}/${P}-build.
// Note: meson.eclass uses "-build" suffix, not "_build" like cmake.
func (h *Helpers) getMesonBuildDir() string {
	if buildDir := h.getEnvVar("BUILD_DIR"); buildDir != "" {
		return buildDir
	}

	// Default: ${WORKDIR}/${P}-build (meson convention)
	if h.env != nil {
		return filepath.Join(h.env.WORKDIR, h.env.P+"-build")
	}

	return ""
}

// getMesonBuildType returns the Meson build type.
//
// Uses EMESON_BUILDTYPE if set, otherwise defaults to "plain".
// "plain" is the Gentoo default as we manage optimization via CFLAGS.
func (h *Helpers) getMesonBuildType() string {
	if buildType := h.getEnvVar("EMESON_BUILDTYPE"); buildType != "" {
		return buildType
	}
	return MesonBuildTypePlain
}

// getMesonWrapMode returns the Meson wrap mode.
//
// Uses EMESON_WRAP_MODE if set, otherwise defaults to "nodownload".
// This prevents meson from downloading dependencies (Gentoo policy).
func (h *Helpers) getMesonWrapMode() string {
	if wrapMode := h.getEnvVar("EMESON_WRAP_MODE"); wrapMode != "" {
		return wrapMode
	}
	return MesonWrapModeNoDownload
}

// buildMesonArgs builds the standard Meson arguments for Gentoo.
//
// This includes all the standard meson.eclass options:
//   - --prefix
//   - --libdir
//   - --localstatedir
//   - --sysconfdir
//   - --buildtype
//   - --wrap-mode
//   - Backend selection (ninja)
func (h *Helpers) buildMesonArgs() []string {
	eprefix := h.getEprefix()
	libdir := h.getLibDir()

	args := []string{
		// Installation directories
		fmt.Sprintf("--prefix=%s/usr", eprefix),
		fmt.Sprintf("--libdir=%s", libdir),
		fmt.Sprintf("--localstatedir=%s/var/lib", eprefix),
		fmt.Sprintf("--sysconfdir=%s/etc", eprefix),

		// Build configuration
		fmt.Sprintf("--buildtype=%s", h.getMesonBuildType()),
		fmt.Sprintf("--wrap-mode=%s", h.getMesonWrapMode()),

		// Use ninja backend (default and recommended)
		"--backend=ninja",
	}

	// Add mandir, infodir, datadir for FHS compliance
	args = append(args,
		fmt.Sprintf("--mandir=%s/usr/share/man", eprefix),
		fmt.Sprintf("--infodir=%s/usr/share/info", eprefix),
		fmt.Sprintf("--datadir=%s/usr/share", eprefix),
	)

	// Add bindir, sbindir
	args = append(args,
		fmt.Sprintf("--bindir=%s/usr/bin", eprefix),
		fmt.Sprintf("--sbindir=%s/usr/sbin", eprefix),
	)

	// Add includedir
	args = append(args,
		fmt.Sprintf("--includedir=%s/usr/include", eprefix),
	)

	// Set libexecdir per meson.eclass
	args = append(args,
		fmt.Sprintf("--libexecdir=%s/usr/libexec", eprefix),
	)

	// Cross-compilation support
	if h.isCrossCompiling() {
		crossFile := h.generateMesonCrossFile()
		if crossFile != "" {
			args = append(args, "--cross-file="+crossFile)
		}
	}

	// Native file for build tools
	if h.getEnvVar("EMESON_NATIVE_FILE") != "" {
		args = append(args, "--native-file="+h.getEnvVar("EMESON_NATIVE_FILE"))
	}

	return args
}

// isCrossCompiling checks if we are cross-compiling.
//
// Returns true if CHOST != CBUILD.
func (h *Helpers) isCrossCompiling() bool {
	chost := h.getEnvVar("CHOST")
	cbuild := h.getEnvVar("CBUILD")

	// If CBUILD is not set, assume native compilation
	if cbuild == "" {
		return false
	}

	return chost != cbuild
}

// generateMesonCrossFile generates a meson cross-file for cross-compilation.
//
// Returns the path to the generated cross-file, or empty string if not needed.
func (h *Helpers) generateMesonCrossFile() string {
	if !h.isCrossCompiling() {
		return ""
	}

	// Check if a custom cross-file was provided
	if crossFile := h.getEnvVar("EMESON_CROSS_FILE"); crossFile != "" {
		return crossFile
	}

	// Generate a cross-file
	buildDir := h.getMesonBuildDir()
	crossFilePath := filepath.Join(buildDir, "meson.cross")

	// Get toolchain information
	chost := h.getEnvVar("CHOST")
	cc := h.getTool("CC", "gcc")
	cxx := h.getTool("CXX", "g++")
	ar := h.getTool("AR", "ar")
	strip := h.getTool("STRIP", "strip")
	pkgConfig := h.getTool("PKG_CONFIG", "pkg-config")

	// Determine system and CPU
	system := "linux"
	cpuFamily := "x86_64"
	cpu := "x86_64"

	// Parse CHOST for architecture info
	if parts := strings.Split(chost, "-"); len(parts) >= 1 {
		cpuPart := parts[0]
		cpu = cpuPart

		// Map CPU to CPU family
		switch {
		case strings.HasPrefix(cpuPart, "x86_64"):
			cpuFamily = "x86_64"
		case strings.HasPrefix(cpuPart, "i686"), strings.HasPrefix(cpuPart, "i386"):
			cpuFamily = "x86"
		case strings.HasPrefix(cpuPart, "aarch64"):
			cpuFamily = "aarch64"
		case strings.HasPrefix(cpuPart, "arm"):
			cpuFamily = "arm"
		case strings.HasPrefix(cpuPart, "powerpc64"):
			cpuFamily = "ppc64"
		case strings.HasPrefix(cpuPart, "powerpc"):
			cpuFamily = "ppc"
		case strings.HasPrefix(cpuPart, "riscv64"):
			cpuFamily = "riscv64"
		case strings.HasPrefix(cpuPart, "riscv32"):
			cpuFamily = "riscv32"
		}
	}

	// Detect endianness
	endian := h.detectEndian()

	// Generate cross-file content
	content := fmt.Sprintf(`[binaries]
c = '%s'
cpp = '%s'
ar = '%s'
strip = '%s'
pkgconfig = '%s'

[host_machine]
system = '%s'
cpu_family = '%s'
cpu = '%s'
endian = '%s'
`, cc, cxx, ar, strip, pkgConfig, system, cpuFamily, cpu, endian)

	// Ensure build directory exists
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		h.writeStderr(fmt.Sprintf("warning: failed to create build directory for cross-file: %v\n", err))
		return ""
	}

	// Write cross-file
	if err := os.WriteFile(crossFilePath, []byte(content), 0644); err != nil {
		h.writeStderr(fmt.Sprintf("warning: failed to write meson cross-file: %v\n", err))
		return ""
	}

	return crossFilePath
}

// hasUseFlag checks if a USE flag is enabled.
//
// Checks the environment USE variable (space-separated list of enabled flags).
func (h *Helpers) hasUseFlag(flag string) bool {
	// Check environment's USE string first (contains enabled flags)
	if h.env != nil && h.env.USE != "" {
		for _, f := range strings.Fields(h.env.USE) {
			if f == flag {
				return true
			}
			if f == "-"+flag {
				return false
			}
		}
	}

	// Check USE environment variable
	useVar := h.getEnvVar("USE")
	if useVar != "" {
		for _, f := range strings.Fields(useVar) {
			if f == flag {
				return true
			}
			if f == "-"+flag {
				return false
			}
		}
	}

	return false
}
