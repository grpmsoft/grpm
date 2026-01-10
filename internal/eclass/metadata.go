// Package eclass provides dynamic eclass loading and execution.
//
// This file provides metadata handling for eclasses, including
// EXPORT_FUNCTIONS support and metadata accumulation.
package eclass

import (
	"strings"
	"sync"
)

// MetadataManager manages metadata accumulation during inherit.
//
// It tracks:
//   - Accumulated metadata (E_DEPEND, E_IUSE, etc.)
//   - Exported functions from eclasses
//   - INHERITED list
//
// Thread-safe: All methods can be called concurrently.
type MetadataManager struct {
	mu sync.RWMutex

	// accumulated stores accumulated metadata from eclasses.
	// Key format: varname (e.g., DEPEND, IUSE).
	// These are the E_* variables from Portage.
	accumulated map[string]string

	// exported maps phase name to the eclass that exports it.
	exported map[string]string

	// inherited is the list of inherited eclasses.
	inherited []string

	// currentEclass is the currently executing eclass.
	currentEclass string

	// depth tracks nesting level of inherit calls.
	depth int
}

// NewMetadataManager creates a new metadata manager.
func NewMetadataManager() *MetadataManager {
	return &MetadataManager{
		accumulated: make(map[string]string),
		exported:    make(map[string]string),
		inherited:   make([]string, 0),
	}
}

// BeginInherit prepares for inheriting an eclass.
//
// Returns the backup of current metadata values that should be restored
// after the eclass is sourced.
func (m *MetadataManager) BeginInherit(eclassName string, currentEnv map[string]string) map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.depth++
	m.currentEclass = eclassName

	// Backup current metadata values
	backup := make(map[string]string)
	for _, varName := range MetadataVars {
		if val, ok := currentEnv[varName]; ok {
			backup[varName] = val
		}
	}

	return backup
}

// EndInherit completes the inherit process.
//
// It accumulates metadata set by the eclass and restores the backed-up values.
// Returns the updated environment with restored values.
func (m *MetadataManager) EndInherit(eclassName string, newEnv, backup map[string]string) map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.depth--

	// Accumulate metadata set by eclass
	for _, varName := range MetadataVars {
		if val, ok := newEnv[varName]; ok && val != "" {
			if existing := m.accumulated[varName]; existing != "" {
				m.accumulated[varName] = existing + " " + val
			} else {
				m.accumulated[varName] = val
			}
		}
	}

	// Restore backed-up values
	result := make(map[string]string, len(newEnv))
	for k, v := range newEnv {
		result[k] = v
	}

	for _, varName := range MetadataVars {
		if backedUp, ok := backup[varName]; ok {
			result[varName] = backedUp
		} else {
			delete(result, varName)
		}
	}

	// Record as inherited
	m.addInherited(eclassName)

	// Clear current eclass
	if m.depth == 0 {
		m.currentEclass = ""
	}

	return result
}

// addInherited adds an eclass to the inherited list.
// Must be called with lock held.
func (m *MetadataManager) addInherited(name string) {
	for _, existing := range m.inherited {
		if existing == name {
			return // Already inherited
		}
	}
	m.inherited = append(m.inherited, name)
}

// ExportFunctions registers phase functions from the current eclass.
//
// Usage: EXPORT_FUNCTIONS src_compile src_install
func (m *MetadataManager) ExportFunctions(phases []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentEclass == "" {
		return &ExportFunctionsError{Message: "EXPORT_FUNCTIONS called without a defined ECLASS"}
	}

	for _, phase := range phases {
		m.exported[phase] = m.currentEclass
	}

	return nil
}

// GetExportedFunction returns the eclass that exports a phase function.
func (m *MetadataManager) GetExportedFunction(phase string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	eclass, ok := m.exported[phase]
	return eclass, ok
}

// GetInherited returns the list of inherited eclasses.
func (m *MetadataManager) GetInherited() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]string, len(m.inherited))
	copy(result, m.inherited)
	return result
}

// GetInheritedString returns INHERITED as a space-separated string.
func (m *MetadataManager) GetInheritedString() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return strings.Join(m.inherited, " ")
}

// IsInherited checks if an eclass has been inherited.
func (m *MetadataManager) IsInherited(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, existing := range m.inherited {
		if existing == name {
			return true
		}
	}
	return false
}

// GetAccumulated returns accumulated metadata.
func (m *MetadataManager) GetAccumulated() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]string, len(m.accumulated))
	for k, v := range m.accumulated {
		result[k] = v
	}
	return result
}

// GetAccumulatedVar returns a specific accumulated variable.
func (m *MetadataManager) GetAccumulatedVar(name string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.accumulated[name]
}

// FinalizeMetadata merges accumulated metadata into the environment.
//
// For each metadata variable:
//   - If ebuild defined it: ebuild_value + " " + accumulated_value
//   - If only eclass defined it: accumulated_value
func (m *MetadataManager) FinalizeMetadata(env map[string]string) map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string, len(env))
	for k, v := range env {
		result[k] = v
	}

	for _, varName := range MetadataVars {
		accValue := m.accumulated[varName]
		ebuildValue := env[varName]

		if accValue != "" {
			if ebuildValue != "" {
				result[varName] = ebuildValue + " " + accValue
			} else {
				result[varName] = accValue
			}
		}
	}

	return result
}

// GetCurrentEclass returns the currently executing eclass.
func (m *MetadataManager) GetCurrentEclass() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentEclass
}

// GetDepth returns the current nesting depth.
func (m *MetadataManager) GetDepth() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.depth
}

// Reset clears all state.
func (m *MetadataManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.accumulated = make(map[string]string)
	m.exported = make(map[string]string)
	m.inherited = make([]string, 0)
	m.currentEclass = ""
	m.depth = 0
}

// ExportFunctionsError is returned when EXPORT_FUNCTIONS fails.
type ExportFunctionsError struct {
	Message string
}

func (e *ExportFunctionsError) Error() string {
	return e.Message
}

// PhaseFunctionName returns the function name for an exported phase.
//
// For phase "src_compile" exported by eclass "cmake", returns "cmake_src_compile".
func PhaseFunctionName(eclass, phase string) string {
	return eclass + "_" + phase
}

// IsPhaseFunction checks if a function name matches the pattern for an exported phase.
//
// Returns (eclass, phase, true) if it matches "{eclass}_{phase}" pattern.
func IsPhaseFunction(funcName string) (eclass, phase string, ok bool) {
	// Valid phases
	phases := []string{
		"pkg_pretend", "pkg_setup", "pkg_nofetch",
		"src_unpack", "src_prepare", "src_configure",
		"src_compile", "src_test", "src_install",
		"pkg_preinst", "pkg_postinst", "pkg_prerm", "pkg_postrm",
		"pkg_config", "pkg_info",
	}

	for _, p := range phases {
		suffix := "_" + p
		if strings.HasSuffix(funcName, suffix) {
			return funcName[:len(funcName)-len(suffix)], p, true
		}
	}

	return "", "", false
}
