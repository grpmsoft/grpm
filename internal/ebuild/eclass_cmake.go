// Package ebuild implements ebuild execution engine.
//
// This file provides cmake.eclass support for ebuild execution.
// The cmake eclass wraps CMake build system functions and provides
// EXPORT_FUNCTIONS support for standard phase implementations.
//
// Reference: https://devmanual.gentoo.org/eclass-reference/cmake.eclass/
package ebuild

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ============================================================================
// CMake Eclass Registration
// ============================================================================

// CmakeEclass represents the cmake.eclass implementation.
//
// This eclass provides:
//   - EXPORT_FUNCTIONS for src_prepare, src_configure, src_compile, src_test, src_install
//   - Default variables: CMAKE_MAKEFILE_GENERATOR, CMAKE_BUILD_TYPE, etc.
//   - Helper functions: cmake_use, cmake_use_find_package, cmake_comment_add_subdirectory
type CmakeEclass struct{}

// Name returns the eclass name.
func (e *CmakeEclass) Name() string {
	return "cmake"
}

// ExportedFunctions returns the phase functions exported by this eclass.
//
// Per cmake.eclass:
//
//	EXPORT_FUNCTIONS src_prepare src_configure src_compile src_test src_install
func (e *CmakeEclass) ExportedFunctions() []string {
	return []string{
		"src_prepare",
		"src_configure",
		"src_compile",
		"src_test",
		"src_install",
	}
}

// Variables returns the default variables set by this eclass.
//
// These match the defaults in cmake.eclass:
//   - CMAKE_MAKEFILE_GENERATOR: ninja (default build system)
//   - CMAKE_BUILD_TYPE: Release (optimized build)
//   - CMAKE_WARN_UNUSED_CLI: yes (warn about unused command line variables)
func (e *CmakeEclass) Variables() map[string]string {
	return map[string]string{
		"CMAKE_MAKEFILE_GENERATOR": "ninja",
		"CMAKE_BUILD_TYPE":         "Release",
		"CMAKE_WARN_UNUSED_CLI":    "yes",
	}
}

// ============================================================================
// CMake Eclass Helper Functions
// ============================================================================

// CmakeUse generates a CMake option based on a USE flag.
//
// Usage: cmake_use flag [option]
//
// Generates -D${option}=ON if USE flag is enabled, -D${option}=OFF otherwise.
// If option is not specified, the flag name is used (uppercased).
//
// Example:
//
//	cmake_use ssl        # -DSSL=ON or -DSSL=OFF
//	cmake_use ssl OPENSSL_SUPPORT  # -DOPENSSL_SUPPORT=ON or -DOPENSSL_SUPPORT=OFF
//
// Reference: cmake.eclass cmake_use().
func (h *Helpers) CmakeUse(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "cmake_use: requires USE flag argument"}
	}

	flag := args[0]
	optName := strings.ToUpper(flag)
	if len(args) >= 2 {
		optName = args[1]
	}

	value := "OFF"
	if h.isUseEnabled(flag) {
		value = "ON"
	}

	h.writeStdout(fmt.Sprintf("-D%s=%s", optName, value))
	return nil
}

// CmakeUseFindPackage generates CMAKE_DISABLE_FIND_PACKAGE option.
//
// Usage: cmake_use_find_package flag package
//
// Generates -DCMAKE_DISABLE_FIND_PACKAGE_${package}=OFF if USE flag is enabled,
// -DCMAKE_DISABLE_FIND_PACKAGE_${package}=ON if USE flag is disabled.
//
// Note: The logic is inverted - when USE is enabled, DISABLE is OFF.
//
// Example:
//
//	cmake_use_find_package ssl OpenSSL
//	# ssl enabled:  -DCMAKE_DISABLE_FIND_PACKAGE_OpenSSL=OFF
//	# ssl disabled: -DCMAKE_DISABLE_FIND_PACKAGE_OpenSSL=ON
//
// Reference: cmake.eclass cmake_use_find_package().
func (h *Helpers) CmakeUseFindPackage(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "cmake_use_find_package: requires USE flag and package name"}
	}

	flag := args[0]
	packageName := args[1]

	// Logic is inverted: USE enabled = DISABLE OFF
	value := "ON"
	if h.isUseEnabled(flag) {
		value = "OFF"
	}

	h.writeStdout(fmt.Sprintf("-DCMAKE_DISABLE_FIND_PACKAGE_%s=%s", packageName, value))
	return nil
}

// CmakeCommentAddSubdirectory comments out add_subdirectory() calls in CMakeLists.txt.
//
// Usage: cmake_comment_add_subdirectory subdir [path]
//
// Finds add_subdirectory(subdir) in CMakeLists.txt and comments it out.
// If path is specified, uses that file instead of CMAKE_USE_DIR/CMakeLists.txt.
//
// This is useful for disabling optional components that should not be built.
//
// Example:
//
//	cmake_comment_add_subdirectory tests      # Comments out add_subdirectory(tests)
//	cmake_comment_add_subdirectory docs src/  # Comments out in src/CMakeLists.txt
//
// Reference: cmake.eclass cmake_comment_add_subdirectory().
func (h *Helpers) CmakeCommentAddSubdirectory(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "cmake_comment_add_subdirectory: requires subdirectory argument"}
	}

	subdir := args[0]
	cmakeDir := h.getCmakeUseDir()

	// If path specified, use it as directory
	if len(args) >= 2 {
		cmakeDir = filepath.Join(cmakeDir, args[1])
	}

	cmakeLists := filepath.Join(cmakeDir, "CMakeLists.txt")

	// Read the file
	content, err := os.ReadFile(cmakeLists)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("cmake_comment_add_subdirectory: reading %s: %v", cmakeLists, err)}
	}

	// Build patterns to match add_subdirectory(subdir) with various spacing
	// Patterns: add_subdirectory(subdir), add_subdirectory( subdir ), etc.
	patterns := []string{
		fmt.Sprintf("add_subdirectory(%s)", subdir),
		fmt.Sprintf("add_subdirectory( %s )", subdir),
		fmt.Sprintf("add_subdirectory(%s )", subdir),
		fmt.Sprintf("add_subdirectory( %s)", subdir),
	}

	lines := strings.Split(string(content), "\n")
	modified := false

	for i, line := range lines {
		// Skip already commented lines
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check each pattern
		for _, pattern := range patterns {
			if strings.Contains(line, pattern) {
				// Comment out the line
				lines[i] = "# " + line + " # Commented by cmake_comment_add_subdirectory"
				modified = true
				h.writeStdout(fmt.Sprintf(">>> Commented out: %s\n", pattern))
				break
			}
		}
	}

	if !modified {
		h.writeStdout(fmt.Sprintf(">>> Warning: add_subdirectory(%s) not found in %s\n", subdir, cmakeLists))
		return nil
	}

	// Write back the modified content
	output := strings.Join(lines, "\n")
	if err := os.WriteFile(cmakeLists, []byte(output), 0644); err != nil {
		return &DieError{Message: fmt.Sprintf("cmake_comment_add_subdirectory: writing %s: %v", cmakeLists, err)}
	}

	return nil
}

// CmakeRunIn runs a command in the build directory.
//
// Usage: cmake_run_in dir cmd [args...]
//
// Changes to the specified directory and runs the command.
// Used for executing commands in out-of-source build directories.
//
// Example:
//
//	cmake_run_in "${BUILD_DIR}" make clean
//
// Reference: cmake.eclass cmake_run_in().
func (h *Helpers) CmakeRunIn(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "cmake_run_in: requires directory and command"}
	}

	dir := args[0]
	cmdName := args[1]
	cmdArgs := args[2:]

	h.writeStdout(fmt.Sprintf(">>> Running in %s: %s %s\n", dir, cmdName, strings.Join(cmdArgs, " ")))

	return h.runCommandInDir(cmdName, cmdArgs, dir)
}

// CmakeSrcPrepare implements the cmake src_prepare phase.
//
// Usage: cmake_src_prepare
//
// Calls the default src_prepare (applies patches via eapply_user), then
// optionally removes bundled CMake modules specified in CMAKE_REMOVE_MODULES_LIST.
//
// Reference: cmake.eclass cmake_src_prepare().
func (h *Helpers) CmakeSrcPrepare(args []string) error {
	h.writeStdout(">>> cmake_src_prepare\n")

	// Call default prepare first (applies patches)
	if err := h.DefaultSrcPrepare(nil); err != nil {
		return err
	}

	// Remove bundled CMake modules if CMAKE_REMOVE_MODULES_LIST is set
	modulesVar := h.getEnvVar("CMAKE_REMOVE_MODULES_LIST")
	if modulesVar == "" {
		// Use default list if not set
		modulesVar = "FindBLAS FindLAPACK"
	}

	modules := strings.Fields(modulesVar)
	if len(modules) == 0 {
		return nil
	}

	cmakeDir := h.getCmakeUseDir()

	// Look for cmake module directory
	moduleDirs := []string{
		filepath.Join(cmakeDir, "cmake"),
		filepath.Join(cmakeDir, "cmake", "Modules"),
		filepath.Join(cmakeDir, "CMake"),
		filepath.Join(cmakeDir, "CMake", "Modules"),
	}

	for _, moduleDir := range moduleDirs {
		if _, err := os.Stat(moduleDir); os.IsNotExist(err) {
			continue
		}

		for _, module := range modules {
			// Try both .cmake and Module.cmake patterns
			patterns := []string{
				filepath.Join(moduleDir, module+".cmake"),
				filepath.Join(moduleDir, "Find"+module+".cmake"),
			}

			for _, pattern := range patterns {
				if _, err := os.Stat(pattern); err == nil {
					h.writeStdout(fmt.Sprintf(">>> Removing bundled module: %s\n", pattern))
					if err := os.Remove(pattern); err != nil {
						h.writeStderr(fmt.Sprintf(">>> Warning: failed to remove %s: %v\n", pattern, err))
					}
				}
			}
		}
	}

	return nil
}

// CmakeBuildType returns the appropriate CMAKE_BUILD_TYPE value.
//
// Usage: cmake_build_type
//
// Returns the CMAKE_BUILD_TYPE value based on environment settings.
// Outputs "Debug", "Release", "RelWithDebInfo", or "MinSizeRel".
//
// Reference: cmake.eclass cmake_build_type().
func (h *Helpers) CmakeBuildType(args []string) error {
	buildType := h.getCmakeBuildType()
	h.writeStdout(buildType)
	return nil
}

// ============================================================================
// CMake Eclass Setup
// ============================================================================

// SetupCmakeEclass configures the cmake eclass defaults in the environment.
//
// Called when 'inherit cmake' is executed. Sets up:
//   - CMAKE_MAKEFILE_GENERATOR defaults
//   - CMAKE_BUILD_TYPE defaults
//   - BUILD_DIR defaults
func (h *Helpers) SetupCmakeEclass() error {
	if h.env == nil {
		return nil
	}

	eclass := &CmakeEclass{}
	defaults := eclass.Variables()

	for key, value := range defaults {
		// Only set if not already defined
		if h.getEnvVar(key) == "" {
			h.env.SetVar(key, value)
		}
	}

	// Set BUILD_DIR to default if not set
	if h.getEnvVar("BUILD_DIR") == "" && h.env.WORKDIR != "" && h.env.P != "" {
		buildDir := filepath.Join(h.env.WORKDIR, h.env.P+"_build")
		h.env.SetVar("BUILD_DIR", buildDir)
	}

	// Set CMAKE_USE_DIR to ${S} if not set
	if h.getEnvVar("CMAKE_USE_DIR") == "" && h.env.S != "" {
		h.env.SetVar("CMAKE_USE_DIR", h.env.S)
	}

	return nil
}

// ============================================================================
// CMake List Manipulation
// ============================================================================

// CmakeRemoveModulesFromList removes modules from a CMake list file.
//
// This is a helper for src_prepare to clean up bundled modules.
func (h *Helpers) CmakeRemoveModulesFromList(listFile string, modules []string) error {
	if _, err := os.Stat(listFile); os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to do
	}

	file, err := os.Open(listFile)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		skip := false
		for _, module := range modules {
			if strings.Contains(line, module) {
				skip = true
				break
			}
		}
		if !skip {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	_ = file.Close()

	return os.WriteFile(listFile, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

// ============================================================================
// CMake Multilib Support
// ============================================================================

// CmakeMultilibSrcConfigure handles multilib-aware CMake configuration.
//
// For multilib builds, this sets up the appropriate library directories
// and ABI-specific options.
func (h *Helpers) CmakeMultilibSrcConfigure(args []string) error {
	// Check if multilib is in effect
	abi := h.getEnvVar("ABI")
	if abi == "" {
		// Not multilib, use regular configure
		return h.CmakeSrcConfigure(args)
	}

	h.writeStdout(fmt.Sprintf(">>> cmake_multilib_src_configure (ABI=%s)\n", abi))

	// Build multilib-specific args
	multilibArgs := make([]string, 0, len(args)+2)

	// Add library directory for this ABI
	libdir := h.getEnvVar("LIBDIR_" + abi)
	if libdir != "" {
		eprefix := h.getEprefix()
		multilibArgs = append(multilibArgs, fmt.Sprintf("-DCMAKE_INSTALL_LIBDIR=%s/usr/%s", eprefix, libdir))
	}

	// Add user args
	multilibArgs = append(multilibArgs, args...)

	return h.CmakeSrcConfigure(multilibArgs)
}
