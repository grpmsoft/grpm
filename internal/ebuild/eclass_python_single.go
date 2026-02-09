// Package ebuild implements ebuild execution engine.
//
// This file provides python-single-r1.eclass support for ebuild execution.
// The python-single-r1 eclass is used for packages that support a single
// Python implementation at a time (selected via PYTHON_SINGLE_TARGET).
//
// Reference: https://devmanual.gentoo.org/eclass-reference/python-single-r1.eclass/
package ebuild

import (
	"fmt"
	"os/exec"
	"strings"
)

// ============================================================================
// Python Single Eclass Registration
// ============================================================================

// PythonSingleEclass represents the python-single-r1.eclass implementation.
//
// This eclass provides:
//   - EXPORT_FUNCTIONS for pkg_setup
//   - PYTHON_SINGLE_TARGET variable handling
//   - Single implementation setup
type PythonSingleEclass struct{}

// Name returns the eclass name.
func (e *PythonSingleEclass) Name() string {
	return "python-single-r1"
}

// ExportedFunctions returns the phase functions exported by this eclass.
func (e *PythonSingleEclass) ExportedFunctions() []string {
	return []string{
		"pkg_setup",
	}
}

// Variables returns the default variables set by this eclass.
func (e *PythonSingleEclass) Variables() map[string]string {
	return map[string]string{}
}

// ============================================================================
// Python Single Setup Functions
// ============================================================================

// PythonSingleR1PkgSetup sets up the Python environment for single target.
//
// This is the pkg_setup phase function exported by python-single-r1.
// It selects the single Python implementation from PYTHON_SINGLE_TARGET
// and exports the necessary environment variables.
//
// Usage (in ebuild): inherit python-single-r1
// Then pkg_setup automatically calls this function.
func (h *Helpers) PythonSingleR1PkgSetup(_ []string) error {
	// Get the single target from USE flags
	target := h.getPythonSingleTarget()
	if target == "" {
		// Auto-detect: pick the best available Python from PYTHON_COMPAT
		compat := strings.Fields(h.getEnvOrDefault("PYTHON_COMPAT", ""))
		for _, candidate := range []string{
			"python3_12", "python3_13", "python3_11", "python3_14",
		} {
			for _, c := range compat {
				if c == candidate {
					info, err := ParsePythonImpl(candidate)
					if err == nil {
						if _, lookErr := exec.LookPath(info.Executable); lookErr == nil {
							target = candidate
							break
						}
					}
				}
			}
			if target != "" {
				break
			}
		}
		if target == "" && len(compat) > 0 {
			target = compat[len(compat)-1] // Last entry is usually newest
		}
		if target == "" {
			target = "python3_12" // Last resort
		}
	}

	// Export Python environment
	return h.PythonExport([]string{target})
}

// PythonSetup is a wrapper for python_setup that works with both eclasses.
//
// For python-single-r1, it sets up the single target.
// For python-r1, it would set up the current implementation in a foreach loop.
func (h *Helpers) PythonSetup(args []string) error {
	// Check which eclass mode we're in
	if h.hasPythonSingleTarget() {
		// python-single-r1 mode
		return h.PythonSingleR1PkgSetup(args)
	}

	// Try to get current implementation from env
	impl := h.getPythonImpl()
	if impl == "" {
		// python-any-r1 mode or no impl set: auto-detect available Python.
		// Try implementations in preference order.
		for _, candidate := range []string{
			"python3_12", "python3_13", "python3_11", "python3_14",
		} {
			info, err := ParsePythonImpl(candidate)
			if err != nil {
				continue
			}
			if _, lookErr := exec.LookPath(info.Executable); lookErr == nil {
				impl = candidate
				break
			}
		}
	}

	if impl == "" {
		impl = "python3_12" // Last resort fallback
	}

	return h.PythonExport([]string{impl})
}

// PythonGenCondDep generates conditional dependencies based on Python target.
//
// Usage: RDEPEND="$(python_gen_cond_dep 'dev-python/foo[${PYTHON_USEDEP}]')"
//
// Generates dependencies for the selected Python implementation.
func (h *Helpers) PythonGenCondDep(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "python_gen_cond_dep: requires dependency pattern"}
	}

	depPattern := strings.Join(args, " ")
	target := h.getPythonSingleTarget()
	if target == "" {
		// Fall back to checking enabled targets
		target = h.getFirstEnabledPythonTarget()
	}

	if target == "" {
		h.writeStdout("")
		return nil
	}

	// Generate PYTHON_USEDEP for the target
	usedep := fmt.Sprintf("python_single_target_%s(-)?", target)

	// Replace ${PYTHON_USEDEP} in pattern
	result := strings.ReplaceAll(depPattern, "${PYTHON_USEDEP}", usedep)
	result = strings.ReplaceAll(result, "${PYTHON_SINGLE_USEDEP}", usedep)

	h.writeStdout(result)
	return nil
}

// PythonGenUseDep generates USE dependency string for Python target.
//
// Usage: RDEPEND="dev-python/foo[$(python_gen_usedep)]"
//
// Returns the USE dependency string for the current Python target.
func (h *Helpers) PythonGenUseDep(args []string) error {
	target := h.getPythonSingleTarget()
	if target == "" {
		target = h.getPythonImpl()
	}

	if target == "" {
		h.writeStdout("")
		return nil
	}

	h.writeStdout(fmt.Sprintf("python_single_target_%s(-)?", target))
	return nil
}

// PythonGenImplDep generates implementation-specific dependency.
//
// Usage: RDEPEND="$(python_gen_impl_dep 'dev-lang/python:*')"
//
// Generates the dependency for the Python interpreter itself.
func (h *Helpers) PythonGenImplDep(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "python_gen_impl_dep: requires dependency pattern"}
	}

	depPattern := strings.Join(args, " ")
	target := h.getPythonSingleTarget()
	if target == "" {
		target = h.getFirstEnabledPythonTarget()
	}

	if target == "" {
		h.writeStdout("")
		return nil
	}

	info, err := ParsePythonImpl(target)
	if err != nil {
		h.writeStdout("")
		return nil
	}

	// Generate slot for the implementation
	slot := fmt.Sprintf("%d.%d", info.Major, info.Minor)

	// Replace patterns
	result := strings.ReplaceAll(depPattern, ":*", ":"+slot)
	result = strings.ReplaceAll(result, "${PYTHON_SINGLE_TARGET}", target)

	h.writeStdout(result)
	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// getPythonSingleTarget returns the selected single Python target.
func (h *Helpers) getPythonSingleTarget() string {
	// Check USE flags for python_single_target_* flags
	useFlags := strings.Fields(h.getEnvOrDefault("USE", ""))
	useSet := make(map[string]bool)
	for _, f := range useFlags {
		useSet[f] = true
	}

	compat := strings.Fields(h.getEnvOrDefault("PYTHON_COMPAT", ""))
	for _, impl := range compat {
		flag := "python_single_target_" + impl
		if useSet[flag] {
			return impl
		}
	}

	// Fall back to PYTHON_SINGLE_TARGET if explicitly set
	return h.getEnvOrDefault("PYTHON_SINGLE_TARGET", "")
}

// hasPythonSingleTarget checks if we're in python-single-r1 mode.
func (h *Helpers) hasPythonSingleTarget() bool {
	// Check if any python_single_target_* USE flag is enabled
	useFlags := strings.Fields(h.getEnvOrDefault("USE", ""))
	for _, f := range useFlags {
		if strings.HasPrefix(f, "python_single_target_") {
			return true
		}
	}
	return false
}

// isPythonCompatible checks if an implementation is in PYTHON_COMPAT.
func (h *Helpers) isPythonCompatible(impl string) bool {
	compat := strings.Fields(h.getEnvOrDefault("PYTHON_COMPAT", ""))
	for _, c := range compat {
		if c == impl {
			return true
		}
	}
	return false
}

// getFirstEnabledPythonTarget returns the first enabled Python target.
func (h *Helpers) getFirstEnabledPythonTarget() string {
	use := h.getEnvOrDefault("USE", "")
	compat := strings.Fields(h.getEnvOrDefault("PYTHON_COMPAT", ""))

	for _, impl := range compat {
		// Check both single and multi target flags
		if strings.Contains(use, "python_single_target_"+impl) ||
			strings.Contains(use, "python_targets_"+impl) {
			return impl
		}
	}

	// Return first compatible if nothing enabled
	if len(compat) > 0 {
		return compat[0]
	}

	return ""
}
