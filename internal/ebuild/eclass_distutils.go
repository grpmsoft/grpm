// Package ebuild implements ebuild execution engine.
//
// This file provides distutils-r1.eclass support for ebuild execution.
// The distutils-r1 eclass is the primary eclass for building Python packages
// using setuptools, flit, poetry, or other PEP 517 build systems.
//
// Reference: https://devmanual.gentoo.org/eclass-reference/distutils-r1.eclass/
package ebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ============================================================================
// Distutils Eclass Registration
// ============================================================================

// DistutilsEclass represents the distutils-r1.eclass implementation.
//
// This eclass provides:
//   - EXPORT_FUNCTIONS for all src_* phases
//   - Support for setuptools, flit, poetry build systems
//   - PEP 517/518 build backend support
//   - Automatic handling of multiple Python implementations
type DistutilsEclass struct{}

// Name returns the eclass name.
func (e *DistutilsEclass) Name() string {
	return "distutils-r1"
}

// ExportedFunctions returns the phase functions exported by this eclass.
func (e *DistutilsEclass) ExportedFunctions() []string {
	return []string{
		"src_prepare",
		"src_configure",
		"src_compile",
		"src_test",
		"src_install",
	}
}

// Variables returns the default variables set by this eclass.
func (e *DistutilsEclass) Variables() map[string]string {
	return map[string]string{
		"DISTUTILS_USE_PEP517":     "",        // Set to setuptools, flit, poetry, etc.
		"DISTUTILS_SINGLE_IMPL":    "",        // Set to force single implementation
		"DISTUTILS_USE_SETUPTOOLS": "bdepend", // How to handle setuptools dep
	}
}

// ============================================================================
// Distutils Phase Functions
// ============================================================================

// DistutilsR1SrcPrepare prepares sources for Python build.
//
// This is the src_prepare phase exported by distutils-r1.
// Applies patches and prepares the source tree.
func (h *Helpers) DistutilsR1SrcPrepare(args []string) error {
	// Call default src_prepare (applies patches)
	if err := h.DefaultSrcPrepare(args); err != nil {
		return err
	}

	// Additional distutils-specific preparation
	return h.distutilsPrepare()
}

// DistutilsR1SrcConfigure configures the Python package.
//
// For most Python packages, this is a no-op as configuration
// happens during the build phase.
func (h *Helpers) DistutilsR1SrcConfigure(args []string) error {
	// Most Python packages don't need separate configure
	return nil
}

// DistutilsR1SrcCompile builds the Python package.
//
// Uses python_foreach_impl to build for all enabled implementations
// or just builds for the single implementation.
func (h *Helpers) DistutilsR1SrcCompile(args []string) error {
	if h.isSingleImpl() {
		return h.distutilsCompile()
	}

	// Multi-implementation: use foreach
	return h.pythonForeachImplDo(h.distutilsCompile)
}

// DistutilsR1SrcTest runs tests for the Python package.
//
// Uses python_foreach_impl to test for all enabled implementations.
func (h *Helpers) DistutilsR1SrcTest(args []string) error {
	if h.isSingleImpl() {
		return h.distutilsTest()
	}

	return h.pythonForeachImplDo(h.distutilsTest)
}

// DistutilsR1SrcInstall installs the Python package.
//
// Uses python_foreach_impl to install for all enabled implementations,
// then calls python_install_all for shared files.
func (h *Helpers) DistutilsR1SrcInstall(args []string) error {
	if h.isSingleImpl() {
		if err := h.distutilsInstall(); err != nil {
			return err
		}
	} else {
		if err := h.pythonForeachImplDo(h.distutilsInstall); err != nil {
			return err
		}
	}

	// Install shared files (docs, etc.)
	return h.pythonInstallAll()
}

// ============================================================================
// Build Backend Functions
// ============================================================================

// distutilsPrepare prepares for distutils build.
func (h *Helpers) distutilsPrepare() error {
	// Create build directory structure
	workdir := h.getEnvOrDefault("WORKDIR", "")
	if workdir != "" {
		buildBase := filepath.Join(workdir, "build")
		if err := os.MkdirAll(buildBase, 0755); err != nil {
			return err
		}
	}

	return nil
}

// distutilsCompile compiles the Python package.
func (h *Helpers) distutilsCompile() error {
	impl := h.getPythonImpl()
	if impl == "" {
		return &DieError{Message: "distutils_compile: EPYTHON not set"}
	}

	info, err := ParsePythonImpl(impl)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("distutils_compile: %v", err)}
	}

	usePEP517 := h.getEnvOrDefault("DISTUTILS_USE_PEP517", "")

	if usePEP517 != "" {
		// PEP 517 build using pip
		return h.buildWithPEP517(info, usePEP517)
	}

	// Legacy setup.py build
	return h.buildWithSetupPy(info)
}

// buildWithPEP517 builds using PEP 517 backend.
func (h *Helpers) buildWithPEP517(info *PythonImplInfo, backend string) error {
	s := h.getEnvOrDefault("S", "")
	workdir := h.getEnvOrDefault("WORKDIR", "")
	buildDir := filepath.Join(workdir, fmt.Sprintf("build-%s", info.Name))

	// Create build directory
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("mkdir failed: %v", err)}
	}

	// Build wheel using pip
	// pip wheel --no-deps --no-build-isolation --wheel-dir=buildDir .
	buildArgs := []string{
		"-m", "pip", "wheel",
		"--no-deps",
		"--no-build-isolation",
		"--wheel-dir", buildDir,
		s,
	}

	h.writeStdout(fmt.Sprintf(">>> Building with PEP 517 (%s)\n", backend))
	return h.runCommand(info.Executable, buildArgs)
}

// buildWithSetupPy builds using legacy setup.py.
func (h *Helpers) buildWithSetupPy(info *PythonImplInfo) error {
	s := h.getEnvOrDefault("S", "")
	workdir := h.getEnvOrDefault("WORKDIR", "")
	buildDir := filepath.Join(workdir, fmt.Sprintf("build-%s", info.Name))

	// Change to source directory
	origDir, _ := os.Getwd()
	if err := os.Chdir(s); err != nil {
		return &DieError{Message: fmt.Sprintf("chdir failed: %v", err)}
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Build using setup.py
	buildArgs := []string{
		"setup.py", "build",
		"--build-base", buildDir,
	}

	h.writeStdout(">>> Building with setup.py\n")
	return h.runCommand(info.Executable, buildArgs)
}

// distutilsTest runs package tests.
func (h *Helpers) distutilsTest() error {
	impl := h.getPythonImpl()
	if impl == "" {
		return &DieError{Message: "distutils_test: EPYTHON not set"}
	}

	info, err := ParsePythonImpl(impl)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("distutils_test: %v", err)}
	}

	s := h.getEnvOrDefault("S", "")

	// Try pytest first
	testArgs := []string{"-m", "pytest", "-v"}
	h.writeStdout(fmt.Sprintf(">>> Running tests for %s\n", impl))

	// Check if pytest is available
	checkArgs := []string{"-c", "import pytest"}
	if err := h.runCommand(info.Executable, checkArgs); err == nil {
		return h.runCommandInDir(info.Executable, testArgs, s)
	}

	// Fall back to setup.py test
	testArgs = []string{"setup.py", "test"}
	return h.runCommandInDir(info.Executable, testArgs, s)
}

// distutilsInstall installs the Python package.
func (h *Helpers) distutilsInstall() error {
	impl := h.getPythonImpl()
	if impl == "" {
		return &DieError{Message: "distutils_install: EPYTHON not set"}
	}

	info, err := ParsePythonImpl(impl)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("distutils_install: %v", err)}
	}

	s := h.getEnvOrDefault("S", "")
	d := h.getEnvOrDefault("D", "")
	prefix := h.getEnvOrDefault("EPREFIX", "")
	workdir := h.getEnvOrDefault("WORKDIR", "")

	usePEP517 := h.getEnvOrDefault("DISTUTILS_USE_PEP517", "")

	if usePEP517 != "" {
		return h.installWithPEP517(info, workdir, d, prefix)
	}

	return h.installWithSetupPy(info, s, d, prefix)
}

// installWithPEP517 installs from wheel built with PEP 517.
func (h *Helpers) installWithPEP517(info *PythonImplInfo, workdir, d, prefix string) error {
	buildDir := filepath.Join(workdir, fmt.Sprintf("build-%s", info.Name))

	// Find wheel file
	wheelFile := ""
	entries, err := os.ReadDir(buildDir)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("read build dir: %v", err)}
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".whl") {
			wheelFile = filepath.Join(buildDir, entry.Name())
			break
		}
	}

	if wheelFile == "" {
		return &DieError{Message: "no wheel file found"}
	}

	// Install wheel
	// pip install --no-deps --prefix=/usr --root=${D} wheel.whl
	installArgs := []string{
		"-m", "pip", "install",
		"--no-deps",
		"--no-compile",
		"--prefix", filepath.Join(prefix, "usr"),
		"--root", d,
		wheelFile,
	}

	h.writeStdout(fmt.Sprintf(">>> Installing wheel for %s\n", info.Name))
	return h.runCommand(info.Executable, installArgs)
}

// installWithSetupPy installs using legacy setup.py.
func (h *Helpers) installWithSetupPy(info *PythonImplInfo, s, d, prefix string) error {
	// Change to source directory
	origDir, _ := os.Getwd()
	if err := os.Chdir(s); err != nil {
		return &DieError{Message: fmt.Sprintf("chdir failed: %v", err)}
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Install using setup.py
	installArgs := []string{
		"setup.py", "install",
		"--root", d,
		"--prefix", filepath.Join(prefix, "usr"),
		"--no-compile",
	}

	h.writeStdout(fmt.Sprintf(">>> Installing with setup.py for %s\n", info.Name))
	return h.runCommand(info.Executable, installArgs)
}

// pythonInstallAll installs documentation and other shared files.
func (h *Helpers) pythonInstallAll() error {
	// Install documentation
	if err := h.einstalldocs(); err != nil {
		// Non-fatal - documentation is optional
		h.writeStderr(fmt.Sprintf("Warning: einstalldocs failed: %v\n", err))
	}

	// Byte-compile Python files
	return h.PythonOptimize(nil)
}

// einstalldocs installs standard documentation files.
func (h *Helpers) einstalldocs() error {
	s := h.getEnvOrDefault("S", "")
	docFiles := []string{"README", "README.md", "README.rst", "README.txt",
		"CHANGELOG", "CHANGELOG.md", "CHANGES", "NEWS", "AUTHORS", "LICENSE"}

	for _, doc := range docFiles {
		docPath := filepath.Join(s, doc)
		if _, err := os.Stat(docPath); err == nil {
			if err := h.Dodoc([]string{docPath}); err != nil {
				return err
			}
		}
	}

	return nil
}

// ============================================================================
// Python Wrapper Functions for distutils
// ============================================================================

// PythonCompile is called for each implementation during compile.
//
// Ebuilds can override python_compile() to customize build.
func (h *Helpers) PythonCompile(args []string) error {
	return h.distutilsCompile()
}

// PythonTest is called for each implementation during testing.
//
// Ebuilds can override python_test() to customize testing.
func (h *Helpers) PythonTest(args []string) error {
	return h.distutilsTest()
}

// PythonInstall is called for each implementation during install.
//
// Ebuilds can override python_install() to customize installation.
func (h *Helpers) PythonInstall(args []string) error {
	return h.distutilsInstall()
}

// PythonInstallAll is called once for all implementations.
//
// Ebuilds can override python_install_all() for shared files.
func (h *Helpers) PythonInstallAll(args []string) error {
	return h.pythonInstallAll()
}

// ============================================================================
// Helper Functions
// ============================================================================

// isSingleImpl returns true if we're in single-implementation mode.
func (h *Helpers) isSingleImpl() bool {
	// Check DISTUTILS_SINGLE_IMPL
	if h.getEnvOrDefault("DISTUTILS_SINGLE_IMPL", "") != "" {
		return true
	}

	// Check if using python-single-r1
	return h.hasPythonSingleTarget()
}

// pythonForeachImplDo runs a function for each Python implementation.
func (h *Helpers) pythonForeachImplDo(fn func() error) error {
	targets := h.getPythonTargets()
	if len(targets) == 0 {
		// Fall back to single target
		target := h.getPythonSingleTarget()
		if target != "" {
			targets = []string{target}
		}
	}

	if len(targets) == 0 {
		return &DieError{Message: "No Python implementations enabled"}
	}

	for _, impl := range targets {
		// Export environment for this implementation
		if err := h.PythonExport([]string{impl}); err != nil {
			return err
		}

		// Run the function
		if err := fn(); err != nil {
			return fmt.Errorf("%s: %w", impl, err)
		}
	}

	return nil
}

// Note: runCommandInDir is defined in build_cmake.go
