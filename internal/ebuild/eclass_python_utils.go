// Package ebuild implements ebuild execution engine.
//
// This file provides python-utils-r1.eclass support for ebuild execution.
// The python-utils-r1 eclass provides utility functions for Python package
// building that are used by python-single-r1, python-r1, and distutils-r1.
//
// Reference: https://devmanual.gentoo.org/eclass-reference/python-utils-r1.eclass/
package ebuild

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ============================================================================
// Python Utils Eclass Registration
// ============================================================================

// PythonUtilsEclass represents the python-utils-r1.eclass implementation.
//
// This eclass provides utility functions for working with Python:
//   - Python implementation detection and validation
//   - Path discovery (site-packages, include, library)
//   - Environment variable export (PYTHON, EPYTHON, PYTHON_SITEDIR)
//   - Python script installation helpers
type PythonUtilsEclass struct{}

// Name returns the eclass name.
func (e *PythonUtilsEclass) Name() string {
	return "python-utils-r1"
}

// ExportedFunctions returns empty list as this eclass doesn't export phases.
func (e *PythonUtilsEclass) ExportedFunctions() []string {
	return []string{}
}

// Variables returns empty map as variables are set dynamically.
func (e *PythonUtilsEclass) Variables() map[string]string {
	return map[string]string{}
}

// ============================================================================
// Python Implementation Detection
// ============================================================================

// pythonImplPattern matches Python implementation names.
// Examples: python3_10, python3_11, python3_12, pypy3
var pythonImplPattern = regexp.MustCompile(`^(python|pypy)(\d+)_(\d+)$`)

// PythonImplInfo contains information about a Python implementation.
type PythonImplInfo struct {
	// Name is the implementation name (e.g., "python3_11")
	Name string
	// Type is "cpython" or "pypy"
	Type string
	// Major version (e.g., 3)
	Major int
	// Minor version (e.g., 11)
	Minor int
	// Executable path
	Executable string
	// ABI suffix for extensions (e.g., "cpython-311-x86_64-linux-gnu")
	ABISuffix string
}

// ParsePythonImpl parses a Python implementation name.
//
// Examples:
//   - python3_10 -> cpython 3.10
//   - python3_11 -> cpython 3.11
//   - pypy3_10 -> pypy 3.10
func ParsePythonImpl(impl string) (*PythonImplInfo, error) {
	matches := pythonImplPattern.FindStringSubmatch(impl)
	if matches == nil {
		return nil, fmt.Errorf("invalid Python implementation: %s", impl)
	}

	var major, minor int
	_, _ = fmt.Sscanf(matches[2], "%d", &major)
	_, _ = fmt.Sscanf(matches[3], "%d", &minor)

	info := &PythonImplInfo{
		Name:  impl,
		Major: major,
		Minor: minor,
	}

	if matches[1] == "pypy" {
		info.Type = "pypy"
		info.Executable = fmt.Sprintf("pypy%d.%d", major, minor)
	} else {
		info.Type = "cpython"
		info.Executable = fmt.Sprintf("python%d.%d", major, minor)
	}

	return info, nil
}

// ============================================================================
// Python Path Discovery Functions
// ============================================================================

// PythonGetSitedir returns the site-packages directory for an implementation.
//
// Usage: sitedir=$(python_get_sitedir)
//
// Returns the path to site-packages, e.g.:
//   - /usr/lib/python3.11/site-packages (CPython)
//   - /usr/lib/pypy3.10/site-packages (PyPy)
func (h *Helpers) PythonGetSitedir(args []string) error {
	impl := h.getPythonImpl()
	if impl == "" {
		return &DieError{Message: "python_get_sitedir: EPYTHON not set"}
	}

	sitedir := h.computePythonSitedir(impl)
	h.writeStdout(sitedir)
	return nil
}

// computePythonSitedir computes site-packages path for implementation.
func (h *Helpers) computePythonSitedir(impl string) string {
	info, err := ParsePythonImpl(impl)
	if err != nil {
		// Fallback for non-standard names
		return fmt.Sprintf("/usr/lib/%s/site-packages", impl)
	}

	prefix := h.getEnvOrDefault("EPREFIX", "")
	if info.Type == "pypy" {
		return filepath.Join(prefix, "usr", "lib",
			fmt.Sprintf("pypy%d.%d", info.Major, info.Minor), "site-packages")
	}

	return filepath.Join(prefix, "usr", "lib",
		fmt.Sprintf("python%d.%d", info.Major, info.Minor), "site-packages")
}

// PythonGetIncludedir returns the include directory for an implementation.
//
// Usage: includedir=$(python_get_includedir)
//
// Returns the path to Python includes, e.g., /usr/include/python3.11
func (h *Helpers) PythonGetIncludedir(args []string) error {
	impl := h.getPythonImpl()
	if impl == "" {
		return &DieError{Message: "python_get_includedir: EPYTHON not set"}
	}

	includedir := h.computePythonIncludedir(impl)
	h.writeStdout(includedir)
	return nil
}

// computePythonIncludedir computes include path for implementation.
func (h *Helpers) computePythonIncludedir(impl string) string {
	info, err := ParsePythonImpl(impl)
	if err != nil {
		return fmt.Sprintf("/usr/include/%s", impl)
	}

	prefix := h.getEnvOrDefault("EPREFIX", "")
	if info.Type == "pypy" {
		return filepath.Join(prefix, "usr", "include",
			fmt.Sprintf("pypy%d.%d", info.Major, info.Minor))
	}

	return filepath.Join(prefix, "usr", "include",
		fmt.Sprintf("python%d.%d", info.Major, info.Minor))
}

// PythonGetLibrary returns the libpython shared library path.
//
// Usage: library=$(python_get_library)
//
// Returns the path to libpython, e.g., /usr/lib64/libpython3.11.so
func (h *Helpers) PythonGetLibrary(args []string) error {
	impl := h.getPythonImpl()
	if impl == "" {
		return &DieError{Message: "python_get_library: EPYTHON not set"}
	}

	library := h.computePythonLibrary(impl)
	h.writeStdout(library)
	return nil
}

// computePythonLibrary computes library path for implementation.
func (h *Helpers) computePythonLibrary(impl string) string {
	info, err := ParsePythonImpl(impl)
	if err != nil {
		return ""
	}

	prefix := h.getEnvOrDefault("EPREFIX", "")
	libdir := h.libDir

	if info.Type == "pypy" {
		return filepath.Join(prefix, "usr", libdir,
			fmt.Sprintf("libpypy%d.%d-c.so", info.Major, info.Minor))
	}

	return filepath.Join(prefix, "usr", libdir,
		fmt.Sprintf("libpython%d.%d.so", info.Major, info.Minor))
}

// PythonGetScriptdir returns the script installation directory.
//
// Usage: scriptdir=$(python_get_scriptdir)
//
// Returns the path where Python scripts should be installed.
func (h *Helpers) PythonGetScriptdir(args []string) error {
	prefix := h.getEnvOrDefault("EPREFIX", "")
	h.writeStdout(filepath.Join(prefix, "usr", "bin"))
	return nil
}

// ============================================================================
// Python Implementation Validation
// ============================================================================

// PythonIsInstalled checks if a Python implementation is installed.
//
// Usage: python_is_installed python3_11 && echo "installed"
//
// Returns exit code 0 if installed, 1 otherwise.
func (h *Helpers) PythonIsInstalled(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "python_is_installed: requires implementation argument"}
	}

	impl := args[0]
	info, err := ParsePythonImpl(impl)
	if err != nil {
		return exitFalse()
	}

	// Check if executable exists
	_, err = exec.LookPath(info.Executable)
	if err != nil {
		return exitFalse()
	}

	return nil
}

// PythonIsCompatible checks if implementation is in PYTHON_COMPAT.
//
// Usage: python_is_compatible python3_11 && use it
//
// Returns 0 if compatible, 1 otherwise.
func (h *Helpers) PythonIsCompatible(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "python_is_compatible: requires implementation argument"}
	}

	impl := args[0]
	compat := h.getEnvOrDefault("PYTHON_COMPAT", "")

	// PYTHON_COMPAT is an array in bash, stored as space-separated string
	for _, c := range strings.Fields(compat) {
		if c == impl {
			return nil
		}
	}

	return exitFalse()
}

// ============================================================================
// Python Environment Export
// ============================================================================

// PythonExport exports Python-related environment variables.
//
// Usage: python_export python3_11
//
// Exports:
//   - EPYTHON: Implementation name (python3_11)
//   - PYTHON: Path to interpreter (/usr/bin/python3.11)
//   - PYTHON_SITEDIR: Site-packages directory
//   - PYTHON_INCLUDEDIR: Include directory
//   - PYTHON_LIBPATH: Library path
func (h *Helpers) PythonExport(args []string) error {
	var impl string
	if len(args) >= 1 {
		impl = args[0]
	} else {
		impl = h.getPythonImpl()
	}

	if impl == "" {
		return &DieError{Message: "python_export: no implementation specified"}
	}

	info, err := ParsePythonImpl(impl)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("python_export: %v", err)}
	}

	// Find the Python executable
	pythonPath, err := exec.LookPath(info.Executable)
	if err != nil {
		// Try with prefix
		prefix := h.getEnvOrDefault("EPREFIX", "")
		pythonPath = filepath.Join(prefix, "usr", "bin", info.Executable)
	}

	// Set environment variables
	h.setEnvVar("EPYTHON", impl)
	h.setEnvVar("PYTHON", pythonPath)
	h.setEnvVar("PYTHON_SITEDIR", h.computePythonSitedir(impl))
	h.setEnvVar("PYTHON_INCLUDEDIR", h.computePythonIncludedir(impl))
	h.setEnvVar("PYTHON_LIBPATH", h.computePythonLibrary(impl))

	return nil
}

// PythonWrapper installs a wrapper script for a Python script.
//
// Usage: python_wrapper script.py /usr/bin/script
//
// Creates a wrapper that calls the script with the correct Python.
func (h *Helpers) PythonWrapper(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "python_wrapper: requires script and destination"}
	}

	script := args[0]
	dest := args[1]

	impl := h.getPythonImpl()
	if impl == "" {
		return &DieError{Message: "python_wrapper: EPYTHON not set"}
	}

	info, err := ParsePythonImpl(impl)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("python_wrapper: %v", err)}
	}

	// Create wrapper content
	wrapper := fmt.Sprintf(`#!/usr/bin/env %s
# Wrapper generated by GRPM
import sys
exec(open('%s').read())
`, info.Executable, script)

	// Get destination path
	d := h.getEnvOrDefault("D", "")
	destPath := filepath.Join(d, dest)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("python_wrapper: mkdir failed: %v", err)}
	}

	// Write wrapper
	if err := os.WriteFile(destPath, []byte(wrapper), 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("python_wrapper: write failed: %v", err)}
	}

	return nil
}

// ============================================================================
// Python Script Installation
// ============================================================================

// PythonDoexe installs an executable Python script.
//
// Usage: python_doexe script.py
//
// Installs the script with proper shebang to PYTHON_SCRIPTDIR.
func (h *Helpers) PythonDoexe(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "python_doexe: requires file argument"}
	}

	src := args[0]
	impl := h.getPythonImpl()
	if impl == "" {
		return &DieError{Message: "python_doexe: EPYTHON not set"}
	}

	info, err := ParsePythonImpl(impl)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("python_doexe: %v", err)}
	}

	// Read source file
	content, err := os.ReadFile(src)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("python_doexe: cannot read %s: %v", src, err)}
	}

	// Replace shebang
	text := string(content)
	if strings.HasPrefix(text, "#!") {
		lines := strings.SplitN(text, "\n", 2)
		if len(lines) > 1 {
			text = fmt.Sprintf("#!/usr/bin/env %s\n%s", info.Executable, lines[1])
		}
	}

	// Determine destination
	d := h.getEnvOrDefault("D", "")
	prefix := h.getEnvOrDefault("EPREFIX", "")
	destDir := filepath.Join(d, prefix, "usr", "bin")
	destFile := filepath.Join(destDir, filepath.Base(src))

	// Ensure directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("python_doexe: mkdir failed: %v", err)}
	}

	// Write file
	if err := os.WriteFile(destFile, []byte(text), 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("python_doexe: write failed: %v", err)}
	}

	return nil
}

// PythonNewexe installs an executable with a new name.
//
// Usage: python_newexe script.py newname
//
// Installs the script as newname with proper shebang.
func (h *Helpers) PythonNewexe(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "python_newexe: requires source and destination"}
	}

	src := args[0]
	newName := args[1]

	impl := h.getPythonImpl()
	if impl == "" {
		return &DieError{Message: "python_newexe: EPYTHON not set"}
	}

	info, err := ParsePythonImpl(impl)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("python_newexe: %v", err)}
	}

	// Read source file
	content, err := os.ReadFile(src)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("python_newexe: cannot read %s: %v", src, err)}
	}

	// Replace shebang
	text := string(content)
	if strings.HasPrefix(text, "#!") {
		lines := strings.SplitN(text, "\n", 2)
		if len(lines) > 1 {
			text = fmt.Sprintf("#!/usr/bin/env %s\n%s", info.Executable, lines[1])
		}
	}

	// Determine destination
	d := h.getEnvOrDefault("D", "")
	prefix := h.getEnvOrDefault("EPREFIX", "")
	destDir := filepath.Join(d, prefix, "usr", "bin")
	destFile := filepath.Join(destDir, newName)

	// Ensure directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("python_newexe: mkdir failed: %v", err)}
	}

	// Write file
	if err := os.WriteFile(destFile, []byte(text), 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("python_newexe: write failed: %v", err)}
	}

	return nil
}

// PythonDomodule installs a Python module to site-packages.
//
// Usage: python_domodule module.py
// Usage: python_domodule mypackage/
//
// Installs Python modules/packages to PYTHON_SITEDIR.
func (h *Helpers) PythonDomodule(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "python_domodule: requires file argument"}
	}

	impl := h.getPythonImpl()
	if impl == "" {
		return &DieError{Message: "python_domodule: EPYTHON not set"}
	}

	d := h.getEnvOrDefault("D", "")
	sitedir := h.computePythonSitedir(impl)
	destDir := filepath.Join(d, sitedir)

	// Ensure destination exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("python_domodule: mkdir failed: %v", err)}
	}

	for _, src := range args {
		info, err := os.Stat(src)
		if err != nil {
			return &DieError{Message: fmt.Sprintf("python_domodule: cannot stat %s: %v", src, err)}
		}

		if info.IsDir() {
			// Copy directory recursively
			if err := h.copyDirRecursive(src, filepath.Join(destDir, filepath.Base(src))); err != nil {
				return &DieError{Message: fmt.Sprintf("python_domodule: copy failed: %v", err)}
			}
		} else {
			// Copy single file
			dest := filepath.Join(destDir, filepath.Base(src))
			if err := h.copyFileWithMode(src, dest, 0644); err != nil {
				return &DieError{Message: fmt.Sprintf("python_domodule: copy failed: %v", err)}
			}
		}
	}

	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// getPythonImpl returns the current Python implementation from EPYTHON.
func (h *Helpers) getPythonImpl() string {
	return h.getEnvOrDefault("EPYTHON", "")
}

// setEnvVar sets an environment variable in the execution environment.
func (h *Helpers) setEnvVar(name, value string) {
	if h.env != nil {
		// Use the appropriate method based on Environment struct
		switch name {
		case "EPYTHON":
			h.env.SetVar(name, value)
		case "PYTHON":
			h.env.SetVar(name, value)
		case "PYTHON_SITEDIR":
			h.env.SetVar(name, value)
		case "PYTHON_INCLUDEDIR":
			h.env.SetVar(name, value)
		case "PYTHON_LIBPATH":
			h.env.SetVar(name, value)
		default:
			h.env.SetVar(name, value)
		}
	}
}

// copyDirRecursive copies a directory recursively.
func (h *Helpers) copyDirRecursive(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return h.copyFileWithMode(path, dstPath, info.Mode())
	})
}

// copyFileWithMode copies a file with specified permissions.
func (h *Helpers) copyFileWithMode(src, dst string, mode os.FileMode) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	return os.WriteFile(dst, content, mode)
}
