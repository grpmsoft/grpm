// Package ebuild implements ebuild execution engine.
//
// This file provides python-any-r1.eclass support for ebuild execution.
// The python-any-r1 eclass is used for packages that need Python only at
// build time and don't install any Python modules.
//
// Reference: https://devmanual.gentoo.org/eclass-reference/python-any-r1.eclass/
package ebuild

import (
	"fmt"
	"strings"
)

// ============================================================================
// Python Any Eclass Registration
// ============================================================================

// PythonAnyEclass represents the python-any-r1.eclass implementation.
//
// This eclass provides:
//   - Simpler Python dependency handling for build-time only
//   - Any compatible Python implementation can be used
//   - No PYTHON_TARGETS, just any-of dependency
type PythonAnyEclass struct{}

// Name returns the eclass name.
func (e *PythonAnyEclass) Name() string {
	return "python-any-r1"
}

// ExportedFunctions returns the phase functions exported by this eclass.
func (e *PythonAnyEclass) ExportedFunctions() []string {
	return []string{
		"pkg_setup",
	}
}

// Variables returns the default variables set by this eclass.
func (e *PythonAnyEclass) Variables() map[string]string {
	return map[string]string{}
}

// ============================================================================
// Python Any Setup Functions
// ============================================================================

// PythonAnyR1PkgSetup sets up Python for build-time use.
//
// This is the pkg_setup phase function exported by python-any-r1.
// It finds any compatible Python implementation and sets up the environment.
func (h *Helpers) PythonAnyR1PkgSetup(args []string) error {
	impl := h.findAnyPythonImpl()
	if impl == "" {
		return &DieError{Message: "python-any-r1_pkg_setup: No compatible Python found"}
	}

	// Export Python environment
	return h.PythonExport([]string{impl})
}

// PythonAnyCheck checks if any compatible Python is available.
//
// Usage: python_check_deps || die "Missing Python dependency"
//
// Returns 0 if a compatible Python is found.
func (h *Helpers) PythonCheckDeps(args []string) error {
	impl := h.findAnyPythonImpl()
	if impl == "" {
		return exitFalse()
	}
	return nil
}

// PythonGenAnyReq generates any-of Python dependency.
//
// Usage: PYTHON_DEPS="$(python_gen_any_dep 'dev-python/foo[${PYTHON_USEDEP}]')"
//
// Generates:
//
//	|| (
//	    ( dev-lang/python:3.11 dev-python/foo[python_targets_python3_11] )
//	    ( dev-lang/python:3.10 dev-python/foo[python_targets_python3_10] )
//	)
func (h *Helpers) PythonAnyGenDep(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "python_gen_any_dep: requires dependency pattern"}
	}

	depPattern := strings.Join(args, " ")
	compat := strings.Fields(h.getEnvOrDefault("PYTHON_COMPAT", ""))

	if len(compat) == 0 {
		h.writeStdout("")
		return nil
	}

	var groups []string
	for _, impl := range compat {
		info, err := ParsePythonImpl(impl)
		if err != nil {
			continue
		}

		// Generate Python interpreter dependency
		var pythonDep string
		if info.Type == "pypy" {
			pythonDep = fmt.Sprintf("dev-python/pypy3:%d.%d", info.Major, info.Minor)
		} else {
			pythonDep = fmt.Sprintf("dev-lang/python:%d.%d", info.Major, info.Minor)
		}

		// Generate USE dep
		usedep := fmt.Sprintf("python_targets_%s(-)?", impl)
		dep := strings.ReplaceAll(depPattern, "${PYTHON_USEDEP}", usedep)

		groups = append(groups, fmt.Sprintf("( %s %s )", pythonDep, dep))
	}

	if len(groups) == 0 {
		h.writeStdout("")
	} else if len(groups) == 1 {
		h.writeStdout(groups[0])
	} else {
		h.writeStdout("|| ( " + strings.Join(groups, " ") + " )")
	}

	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// findAnyPythonImpl finds any installed Python from PYTHON_COMPAT.
func (h *Helpers) findAnyPythonImpl() string {
	compat := strings.Fields(h.getEnvOrDefault("PYTHON_COMPAT", ""))

	// Try each implementation in order
	for _, impl := range compat {
		if err := h.PythonIsInstalled([]string{impl}); err == nil {
			return impl
		}
	}

	// Fall back to system Python
	for _, impl := range []string{"python3_12", "python3_11", "python3_10"} {
		if err := h.PythonIsInstalled([]string{impl}); err == nil {
			return impl
		}
	}

	return ""
}
