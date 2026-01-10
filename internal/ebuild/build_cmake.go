// Package ebuild implements ebuild execution engine.
//
// This file provides CMake build system support for ebuild execution.
// Implements the cmake.eclass helper functions for building CMake-based packages.
//
// Reference: https://devmanual.gentoo.org/eclass-reference/cmake.eclass/
//
// Core functions implemented:
//   - Cmake: Run cmake with Gentoo-standard options
//   - CmakeSrcConfigure: Configure phase using cmake
//   - CmakeSrcCompile: Compile phase using cmake --build
//   - CmakeSrcInstall: Install phase using cmake --install
//   - CmakeSrcTest: Test phase using ctest
//   - Eninja: Run ninja build system
package ebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ============================================================================
// CMake Configuration Constants
// ============================================================================

// CMake generator types per cmake.eclass.
const (
	// CMakeGeneratorNinja uses the Ninja build system (default in modern Gentoo).
	CMakeGeneratorNinja = "Ninja"

	// CMakeGeneratorUnixMakefiles uses traditional Unix Makefiles.
	CMakeGeneratorUnixMakefiles = "Unix Makefiles"
)

// CMake build types per cmake.eclass.
const (
	// CMakeBuildTypeRelease is optimized release build (default).
	CMakeBuildTypeRelease = "Release"

	// CMakeBuildTypeDebug includes debug symbols, no optimization.
	CMakeBuildTypeDebug = "Debug"

	// CMakeBuildTypeRelWithDebInfo is release with debug info.
	CMakeBuildTypeRelWithDebInfo = "RelWithDebInfo"

	// CMakeBuildTypeMinSizeRel is optimized for size.
	CMakeBuildTypeMinSizeRel = "MinSizeRel"
)

// ============================================================================
// CMake Helper Functions
// ============================================================================

// Cmake runs cmake with standard Gentoo options.
//
// Usage: cmake [options] [path]
//
// This is the low-level cmake wrapper. For the standard configure phase,
// use CmakeSrcConfigure instead.
//
// Options are passed directly to cmake. If no path is specified,
// CMAKE_USE_DIR (defaults to ${S}) is used as the source directory.
//
// Reference: cmake.eclass cmake function.
func (h *Helpers) Cmake(args []string) error {
	buildDir := h.getCmakeBuildDir()
	sourceDir := h.getCmakeUseDir()

	// Create build directory if it doesn't exist
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("cmake: failed to create build directory: %v", err)}
	}

	// Build cmake arguments
	cmakeArgs := h.buildCmakeArgs()
	cmakeArgs = append(cmakeArgs, args...)

	// Add source directory as the last argument if not already specified
	hasSourceDir := false
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			hasSourceDir = true
			break
		}
	}
	if !hasSourceDir {
		cmakeArgs = append(cmakeArgs, sourceDir)
	}

	h.writeStdout(fmt.Sprintf(">>> Running: cmake %s\n", strings.Join(cmakeArgs, " ")))

	return h.runCommandInDir("cmake", cmakeArgs, buildDir)
}

// CmakeSrcConfigure implements the default cmake src_configure phase.
//
// Usage: cmake_src_configure
//
// Sets standard CMAKE_* variables including:
//   - CMAKE_INSTALL_PREFIX: ${EPREFIX}/usr
//   - CMAKE_BUILD_TYPE: Release (or from CMAKE_BUILD_TYPE env)
//   - CMAKE_INSTALL_LIBDIR: ${EPREFIX}/usr/${libdir}
//   - CMAKE_C_COMPILER: from tc-getCC
//   - CMAKE_CXX_COMPILER: from tc-getCXX
//
// Additional arguments can be passed via MYCMAKEARGS environment variable.
//
// Reference: cmake.eclass cmake_src_configure().
func (h *Helpers) CmakeSrcConfigure(args []string) error {
	h.writeStdout(">>> cmake_src_configure\n")

	// Run cmake with additional user args
	allArgs := make([]string, 0)

	// Add user-specified arguments from MYCMAKEARGS
	if mycmakeargs := h.getEnvVar("MYCMAKEARGS"); mycmakeargs != "" {
		allArgs = append(allArgs, strings.Fields(mycmakeargs)...)
	}

	// Add any direct arguments
	allArgs = append(allArgs, args...)

	return h.Cmake(allArgs)
}

// CmakeSrcCompile implements the default cmake src_compile phase.
//
// Usage: cmake_src_compile
//
// Calls cmake --build with the appropriate generator-specific options.
// Uses Ninja if available and configured, otherwise Make.
//
// Honors MAKEOPTS for parallel builds.
//
// Reference: cmake.eclass cmake_src_compile().
func (h *Helpers) CmakeSrcCompile(args []string) error {
	h.writeStdout(">>> cmake_src_compile\n")

	buildDir := h.getCmakeBuildDir()

	// Check if build directory exists
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		return &DieError{Message: "cmake_src_compile: build directory does not exist (run cmake_src_configure first)"}
	}

	// Use cmake --build for generator-agnostic building
	buildArgs := []string{"--build", buildDir}

	// Add parallel jobs from MAKEOPTS
	if jobs := h.extractParallelJobs(); jobs > 0 {
		buildArgs = append(buildArgs, "--parallel", fmt.Sprintf("%d", jobs))
	}

	// Add any additional arguments
	buildArgs = append(buildArgs, args...)

	h.writeStdout(fmt.Sprintf(">>> Running: cmake %s\n", strings.Join(buildArgs, " ")))

	return h.runCommand("cmake", buildArgs)
}

// CmakeSrcInstall implements the default cmake src_install phase.
//
// Usage: cmake_src_install
//
// Runs cmake --install with DESTDIR set to ${D}.
//
// Reference: cmake.eclass cmake_src_install().
func (h *Helpers) CmakeSrcInstall(args []string) error {
	h.writeStdout(">>> cmake_src_install\n")

	buildDir := h.getCmakeBuildDir()

	// Check if build directory exists
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		return &DieError{Message: "cmake_src_install: build directory does not exist"}
	}

	// Get DESTDIR (D environment variable)
	destdir := h.getDestDir()
	if destdir == "" {
		return &DieError{Message: "cmake_src_install: D (destination directory) not set"}
	}

	// Create destination directory
	if err := os.MkdirAll(destdir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("cmake_src_install: failed to create destination directory: %v", err)}
	}

	// Use cmake --install
	installArgs := []string{"--install", buildDir}
	installArgs = append(installArgs, args...)

	h.writeStdout(fmt.Sprintf(">>> Running: DESTDIR=%s cmake %s\n", destdir, strings.Join(installArgs, " ")))

	// Set DESTDIR in environment and run
	return h.runCommandWithEnv("cmake", installArgs, buildDir, map[string]string{
		"DESTDIR": destdir,
	})
}

// CmakeSrcTest implements the default cmake src_test phase.
//
// Usage: cmake_src_test
//
// Runs ctest in the build directory with appropriate options.
//
// Reference: cmake.eclass cmake_src_test().
func (h *Helpers) CmakeSrcTest(args []string) error {
	h.writeStdout(">>> cmake_src_test\n")

	buildDir := h.getCmakeBuildDir()

	// Check if build directory exists
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		return &DieError{Message: "cmake_src_test: build directory does not exist"}
	}

	// Build ctest arguments
	ctestArgs := []string{"--test-dir", buildDir, "--output-on-failure"}

	// Add parallel jobs from MAKEOPTS
	if jobs := h.extractParallelJobs(); jobs > 0 {
		ctestArgs = append(ctestArgs, "--parallel", fmt.Sprintf("%d", jobs))
	}

	// Add any additional arguments
	ctestArgs = append(ctestArgs, args...)

	h.writeStdout(fmt.Sprintf(">>> Running: ctest %s\n", strings.Join(ctestArgs, " ")))

	return h.runCommand("ctest", ctestArgs)
}

// Eninja runs the Ninja build system.
//
// Usage: eninja [target...]
//
// Runs ninja in the build directory with parallelization from MAKEOPTS.
//
// Reference: cmake.eclass eninja().
func (h *Helpers) Eninja(args []string) error {
	buildDir := h.getCmakeBuildDir()

	// Check if build directory exists
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		return &DieError{Message: "eninja: build directory does not exist"}
	}

	// Build ninja arguments
	ninjaArgs := make([]string, 0)

	// Add parallel jobs from MAKEOPTS
	if jobs := h.extractParallelJobs(); jobs > 0 {
		ninjaArgs = append(ninjaArgs, "-j", fmt.Sprintf("%d", jobs))
	}

	// Add targets
	ninjaArgs = append(ninjaArgs, args...)

	h.writeStdout(fmt.Sprintf(">>> Running: ninja %s\n", strings.Join(ninjaArgs, " ")))

	return h.runCommandInDir("ninja", ninjaArgs, buildDir)
}

// ============================================================================
// CMake Internal Helper Functions
// ============================================================================

// getCmakeUseDir returns the CMake source directory.
//
// Uses CMAKE_USE_DIR if set, otherwise defaults to ${S}.
func (h *Helpers) getCmakeUseDir() string {
	if useDir := h.getEnvVar("CMAKE_USE_DIR"); useDir != "" {
		return useDir
	}
	return h.getWorkDir()
}

// getCmakeBuildDir returns the CMake build directory.
//
// Uses BUILD_DIR if set, otherwise defaults to ${WORKDIR}/${P}_build.
func (h *Helpers) getCmakeBuildDir() string {
	if buildDir := h.getEnvVar("BUILD_DIR"); buildDir != "" {
		return buildDir
	}

	// Default: ${WORKDIR}/${P}_build
	if h.env != nil {
		return filepath.Join(h.env.WORKDIR, h.env.P+"_build")
	}

	return ""
}

// getCmakeGenerator returns the CMake generator to use.
//
// Uses CMAKE_MAKEFILE_GENERATOR if set, otherwise defaults to Ninja.
func (h *Helpers) getCmakeGenerator() string {
	if gen := h.getEnvVar("CMAKE_MAKEFILE_GENERATOR"); gen != "" {
		// Normalize common names
		switch strings.ToLower(gen) {
		case "ninja":
			return CMakeGeneratorNinja
		case "emake", "make", "unix makefiles":
			return CMakeGeneratorUnixMakefiles
		default:
			return gen
		}
	}
	return CMakeGeneratorNinja
}

// getCmakeBuildType returns the CMake build type.
//
// Uses CMAKE_BUILD_TYPE if set, otherwise defaults to Release.
func (h *Helpers) getCmakeBuildType() string {
	if buildType := h.getEnvVar("CMAKE_BUILD_TYPE"); buildType != "" {
		return buildType
	}
	return CMakeBuildTypeRelease
}

// getEprefix returns the EPREFIX value.
//
// Used for Gentoo Prefix installations. Usually empty.
func (h *Helpers) getEprefix() string {
	if h.env != nil {
		return h.env.EPREFIX
	}
	return h.getEnvVar("EPREFIX")
}

// getDestDir returns the D (destination) directory.
func (h *Helpers) getDestDir() string {
	if h.env != nil {
		return h.env.D
	}
	return h.getEnvVar("D")
}

// buildCmakeArgs builds the standard CMake arguments for Gentoo.
//
// This includes all the standard cmake.eclass options:
//   - Generator (-G)
//   - Install prefix
//   - Build type
//   - Library directory
//   - Documentation directories
//   - Compilers (CC, CXX)
func (h *Helpers) buildCmakeArgs() []string {
	eprefix := h.getEprefix()
	libdir := h.getLibDir()

	args := []string{
		"-G", h.getCmakeGenerator(),
		fmt.Sprintf("-DCMAKE_INSTALL_PREFIX=%s/usr", eprefix),
		fmt.Sprintf("-DCMAKE_BUILD_TYPE=%s", h.getCmakeBuildType()),
		fmt.Sprintf("-DCMAKE_INSTALL_LIBDIR=%s/usr/%s", eprefix, libdir),
	}

	// Documentation directories
	if h.env != nil {
		args = append(args,
			fmt.Sprintf("-DCMAKE_INSTALL_DOCDIR=%s/usr/share/doc/%s", eprefix, h.env.PF),
		)
	}
	args = append(args,
		fmt.Sprintf("-DCMAKE_INSTALL_MANDIR=%s/usr/share/man", eprefix),
		fmt.Sprintf("-DCMAKE_INSTALL_INFODIR=%s/usr/share/info", eprefix),
	)

	// Compiler settings from toolchain functions
	cc := h.getTool("CC", "gcc")
	cxx := h.getTool("CXX", "g++")
	args = append(args,
		fmt.Sprintf("-DCMAKE_C_COMPILER=%s", cc),
		fmt.Sprintf("-DCMAKE_CXX_COMPILER=%s", cxx),
	)

	// Verbose makefile for debugging (optional, per cmake.eclass)
	if h.getEnvVar("CMAKE_VERBOSE") == "1" || h.getEnvVar("CMAKE_VERBOSE") == "ON" {
		args = append(args, "-DCMAKE_VERBOSE_MAKEFILE=ON")
	}

	return args
}

// extractParallelJobs extracts the -j value from MAKEOPTS.
//
// Returns 0 if no parallel jobs specified.
func (h *Helpers) extractParallelJobs() int {
	makeopts := h.getMakeOpts()

	for i, opt := range makeopts {
		if opt == "-j" && i+1 < len(makeopts) {
			var jobs int
			if _, err := fmt.Sscanf(makeopts[i+1], "%d", &jobs); err == nil {
				return jobs
			}
		}
		if strings.HasPrefix(opt, "-j") {
			var jobs int
			if _, err := fmt.Sscanf(opt[2:], "%d", &jobs); err == nil {
				return jobs
			}
		}
	}

	// Default to number of CPUs if no -j specified but MAKEOPTS is set
	if len(makeopts) > 0 {
		return runtime.NumCPU()
	}

	return 0
}

// runCommandInDir executes a command in a specific directory.
func (h *Helpers) runCommandInDir(name string, args []string, dir string) error {
	if dir == "" {
		return &DieError{Message: fmt.Sprintf("%s: working directory not set", name)}
	}

	// Check if working directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return &DieError{Message: fmt.Sprintf("%s: working directory does not exist: %s", name, dir)}
	}

	cmd := h.createCommand(name, args, dir)

	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		h.writeStdout(string(output))
	}

	if err != nil {
		return &DieError{Message: fmt.Sprintf("%s failed: %v", name, err)}
	}

	return nil
}

// runCommandWithEnv executes a command with additional environment variables.
func (h *Helpers) runCommandWithEnv(name string, args []string, dir string, extraEnv map[string]string) error {
	if dir == "" {
		dir = h.getWorkDir()
	}

	// Check if working directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return &DieError{Message: fmt.Sprintf("%s: working directory does not exist: %s", name, dir)}
	}

	cmd := h.createCommand(name, args, dir)

	// Add extra environment variables
	for k, v := range extraEnv {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		h.writeStdout(string(output))
	}

	if err != nil {
		return &DieError{Message: fmt.Sprintf("%s failed: %v", name, err)}
	}

	return nil
}
