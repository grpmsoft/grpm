// Package ebuild implements ebuild execution engine.
//
// This file provides python-r1.eclass support for ebuild execution.
// The python-r1 eclass is used for packages that support multiple Python
// implementations simultaneously (selected via PYTHON_TARGETS).
//
// Reference: https://devmanual.gentoo.org/eclass-reference/python-r1.eclass/
package ebuild

import (
	"fmt"
	"strings"
)

// ============================================================================
// Python R1 Eclass Registration
// ============================================================================

// PythonR1Eclass represents the python-r1.eclass implementation.
//
// This eclass provides:
//   - EXPORT_FUNCTIONS for pkg_setup
//   - PYTHON_TARGETS variable handling
//   - python_foreach_impl for iterating over implementations
type PythonR1Eclass struct{}

// Name returns the eclass name.
func (e *PythonR1Eclass) Name() string {
	return "python-r1"
}

// ExportedFunctions returns the phase functions exported by this eclass.
func (e *PythonR1Eclass) ExportedFunctions() []string {
	return []string{
		"pkg_setup",
	}
}

// Variables returns the default variables set by this eclass.
func (e *PythonR1Eclass) Variables() map[string]string {
	return map[string]string{}
}

// ============================================================================
// Python R1 Setup Functions
// ============================================================================

// PythonR1PkgSetup sets up the Python environment for multiple targets.
//
// This is the pkg_setup phase function exported by python-r1.
// It validates that at least one Python target is enabled.
func (h *Helpers) PythonR1PkgSetup(args []string) error {
	targets := h.getPythonTargets()
	if len(targets) == 0 {
		return &DieError{Message: "python-r1_pkg_setup: No PYTHON_TARGETS enabled"}
	}

	// Validate all targets are in PYTHON_COMPAT
	for _, target := range targets {
		if !h.isPythonCompatible(target) {
			return &DieError{Message: fmt.Sprintf(
				"python-r1_pkg_setup: %s not in PYTHON_COMPAT", target)}
		}
	}

	return nil
}

// ============================================================================
// Python Foreach Implementation
// ============================================================================

// PythonForeachImpl runs a command for each enabled Python implementation.
//
// Usage: python_foreach_impl emake
// Usage: python_foreach_impl python_compile
//
// This function:
//  1. Gets the list of enabled PYTHON_TARGETS
//  2. For each target, exports EPYTHON and related variables
//  3. Changes BUILD_DIR to implementation-specific directory
//  4. Runs the specified command
//
// The function sets:
//   - EPYTHON: Current implementation (e.g., python3_11)
//   - PYTHON: Path to interpreter
//   - BUILD_DIR: Implementation-specific build directory
func (h *Helpers) PythonForeachImpl(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "python_foreach_impl: requires command argument"}
	}

	targets := h.getPythonTargets()
	if len(targets) == 0 {
		return &DieError{Message: "python_foreach_impl: No PYTHON_TARGETS enabled"}
	}

	command := args[0]
	cmdArgs := args[1:]

	// Save original BUILD_DIR
	origBuildDir := h.getEnvOrDefault("BUILD_DIR", "")

	for _, impl := range targets {
		// Export Python environment for this implementation
		if err := h.PythonExport([]string{impl}); err != nil {
			return err
		}

		// Set implementation-specific BUILD_DIR
		if origBuildDir != "" {
			h.setEnvVar("BUILD_DIR", fmt.Sprintf("%s-%s", origBuildDir, impl))
		} else {
			workdir := h.getEnvOrDefault("WORKDIR", "")
			h.setEnvVar("BUILD_DIR", fmt.Sprintf("%s/build-%s", workdir, impl))
		}

		// Execute the command
		if err := h.executeCommand(command, cmdArgs); err != nil {
			return fmt.Errorf("python_foreach_impl: %s failed for %s: %w",
				command, impl, err)
		}
	}

	// Restore original BUILD_DIR
	if origBuildDir != "" {
		h.setEnvVar("BUILD_DIR", origBuildDir)
	}

	return nil
}

// PythonCopySource copies source to implementation-specific directories.
//
// Usage: python_copy_sources
//
// Creates a copy of the source tree for each enabled Python implementation.
// Each copy is placed in ${WORKDIR}/${P}-${impl}.
func (h *Helpers) PythonCopySources(args []string) error {
	targets := h.getPythonTargets()
	if len(targets) == 0 {
		return &DieError{Message: "python_copy_sources: No PYTHON_TARGETS enabled"}
	}

	workdir := h.getEnvOrDefault("WORKDIR", "")
	s := h.getEnvOrDefault("S", "")
	p := h.getEnvOrDefault("P", "")

	if s == "" {
		return &DieError{Message: "python_copy_sources: S not set"}
	}

	for _, impl := range targets {
		destDir := fmt.Sprintf("%s/%s-%s", workdir, p, impl)

		// Copy source directory
		if err := h.copyDirRecursive(s, destDir); err != nil {
			return &DieError{Message: fmt.Sprintf(
				"python_copy_sources: failed to copy for %s: %v", impl, err)}
		}
	}

	return nil
}

// PythonOptimizeAll byte-compiles Python files.
//
// Usage: python_optimize [directory]
//
// Compiles .py files to .pyc for all enabled implementations.
func (h *Helpers) PythonOptimize(args []string) error {
	targets := h.getPythonTargets()
	if len(targets) == 0 {
		// Fall back to single target mode
		target := h.getPythonSingleTarget()
		if target != "" {
			targets = []string{target}
		} else {
			impl := h.getPythonImpl()
			if impl != "" {
				targets = []string{impl}
			}
		}
	}

	if len(targets) == 0 {
		return &DieError{Message: "python_optimize: No Python implementation set"}
	}

	// Determine directories to optimize
	var dirs []string
	if len(args) > 0 {
		dirs = args
	} else {
		// Default: optimize site-packages
		d := h.getEnvOrDefault("D", "")
		for _, impl := range targets {
			sitedir := h.computePythonSitedir(impl)
			dirs = append(dirs, d+sitedir)
		}
	}

	for _, impl := range targets {
		info, err := ParsePythonImpl(impl)
		if err != nil {
			continue
		}

		for _, dir := range dirs {
			// Run Python to compile modules
			// python -m compileall -q -f -d destdir srcdir
			compileArgs := []string{"-m", "compileall", "-q", "-f", dir}
			if err := h.runCommand(info.Executable, compileArgs); err != nil {
				// Non-fatal - just log warning
				h.writeStderr(fmt.Sprintf("Warning: python_optimize failed for %s: %v\n",
					dir, err))
			}
		}
	}

	return nil
}

// ============================================================================
// Python Dependency Generation
// ============================================================================

// PythonGenAnyDep generates dependencies matching any Python target.
//
// Usage: DEPEND="$(python_gen_any_dep 'dev-python/foo[${PYTHON_USEDEP}]')"
//
// Generates || ( dep[python_targets_X] dep[python_targets_Y] ... )
func (h *Helpers) PythonGenAnyDep(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "python_gen_any_dep: requires dependency pattern"}
	}

	depPattern := strings.Join(args, " ")
	compat := strings.Fields(h.getEnvOrDefault("PYTHON_COMPAT", ""))

	if len(compat) == 0 {
		h.writeStdout("")
		return nil
	}

	var deps []string
	for _, impl := range compat {
		usedep := fmt.Sprintf("python_targets_%s(-)?", impl)
		dep := strings.ReplaceAll(depPattern, "${PYTHON_USEDEP}", usedep)
		dep = strings.ReplaceAll(dep, "${PYTHON_MULTI_USEDEP}", usedep)
		deps = append(deps, dep)
	}

	if len(deps) == 1 {
		h.writeStdout(deps[0])
	} else {
		h.writeStdout("|| ( " + strings.Join(deps, " ") + " )")
	}

	return nil
}

// PythonSetActiveVersion sets the active Python for the current scope.
//
// Usage: python_set_active_version python3_11
//
// Sets EPYTHON and PYTHON to the specified implementation.
func (h *Helpers) PythonSetActiveVersion(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "python_set_active_version: requires implementation"}
	}

	return h.PythonExport(args)
}

// ============================================================================
// Helper Functions
// ============================================================================

// getPythonTargets returns the list of enabled Python targets.
func (h *Helpers) getPythonTargets() []string {
	useFlags := strings.Fields(h.getEnvOrDefault("USE", ""))
	useSet := make(map[string]bool)
	for _, f := range useFlags {
		useSet[f] = true
	}

	compat := strings.Fields(h.getEnvOrDefault("PYTHON_COMPAT", ""))

	var targets []string
	for _, impl := range compat {
		flag := "python_targets_" + impl
		if useSet[flag] {
			targets = append(targets, impl)
		}
	}

	// Fall back to PYTHON_TARGETS if explicitly set
	if len(targets) == 0 {
		explicit := h.getEnvOrDefault("PYTHON_TARGETS", "")
		targets = strings.Fields(explicit)
	}

	return targets
}

// executeCommand executes a helper command by name.
// Uses the command dispatcher if available, otherwise runs as external command.
func (h *Helpers) executeCommand(cmd string, args []string) error {
	// Check if it's a registered helper function
	dispatcher := h.getCommandDispatcher()
	if dispatcher != nil {
		return dispatcher(cmd, args)
	}

	// Fall back to running as external command
	return h.runCommand(cmd, args)
}

// getCommandDispatcher returns the command dispatcher.
// This is a helper to access the field from helpers.go.
func (h *Helpers) getCommandDispatcher() func(string, []string) error {
	return h.commandDispatcher
}
