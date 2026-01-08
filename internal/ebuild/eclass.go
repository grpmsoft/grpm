// Package ebuild implements ebuild execution engine.
//
// This file provides eclass support for ebuild execution.
// Eclasses are bash libraries that provide common functionality
// to ebuilds (e.g., eutils, toolchain-funcs, multilib).
package ebuild

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"mvdan.cc/sh/v3/syntax"
)

// EclassRegistry manages loaded eclasses and their functions.
//
// The registry tracks which eclasses have been inherited to prevent
// double-loading and stores parsed ASTs for reuse.
type EclassRegistry struct {
	mu sync.RWMutex

	// loaded tracks eclasses that have been loaded (name -> path)
	loaded map[string]string

	// functions stores eclass-defined function names (eclass -> []function)
	functions map[string][]string

	// parsedASTs caches parsed eclass ASTs for reuse
	parsedASTs map[string]*syntax.File

	// inherited is the space-separated list of inherited eclasses (INHERITED)
	inherited []string

	// eclassLocations are paths to search for eclasses (e.g., /var/db/repos/gentoo/eclass)
	eclassLocations []string

	// currentEclass is the currently executing eclass (for EXPORT_FUNCTIONS)
	currentEclass string

	// eclassDepth tracks nesting level of inherit calls
	eclassDepth int

	// exportedFunctions maps phase name -> eclass that provides it
	exportedFunctions map[string]string
}

// NewEclassRegistry creates a new eclass registry with default locations.
func NewEclassRegistry(portdir string) *EclassRegistry {
	locations := []string{}

	// Primary location: PORTDIR/eclass
	if portdir != "" {
		locations = append(locations, filepath.Join(portdir, "eclass"))
	}

	// Fallback locations
	defaultLocations := []string{
		"/var/db/repos/gentoo/eclass",
		"/usr/portage/eclass",
	}

	for _, loc := range defaultLocations {
		if loc != filepath.Join(portdir, "eclass") {
			locations = append(locations, loc)
		}
	}

	return &EclassRegistry{
		loaded:            make(map[string]string),
		functions:         make(map[string][]string),
		parsedASTs:        make(map[string]*syntax.File),
		inherited:         make([]string, 0),
		eclassLocations:   locations,
		exportedFunctions: make(map[string]string),
	}
}

// IsLoaded checks if an eclass has been loaded.
func (r *EclassRegistry) IsLoaded(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.loaded[name]
	return ok
}

// MarkLoaded marks an eclass as loaded.
func (r *EclassRegistry) MarkLoaded(name, path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loaded[name] = path

	// Add to INHERITED if not already present
	for _, existing := range r.inherited {
		if existing == name {
			return
		}
	}
	r.inherited = append(r.inherited, name)
}

// GetInherited returns the INHERITED variable value.
func (r *EclassRegistry) GetInherited() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return strings.Join(r.inherited, " ")
}

// FindEclass locates an eclass file in the search paths.
//
// Returns the full path to the eclass file or error if not found.
func (r *EclassRegistry) FindEclass(name string) (string, error) {
	eclassFile := name + ".eclass"

	for _, location := range r.eclassLocations {
		path := filepath.Join(location, eclassFile)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("eclass %s not found in: %v", name, r.eclassLocations)
}

// RegisterFunction records that an eclass defines a function.
func (r *EclassRegistry) RegisterFunction(eclass, funcName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.functions[eclass]; !ok {
		r.functions[eclass] = make([]string, 0)
	}
	r.functions[eclass] = append(r.functions[eclass], funcName)
}

// GetFunctions returns functions defined by an eclass.
func (r *EclassRegistry) GetFunctions(eclass string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.functions[eclass]
}

// CacheAST stores a parsed eclass AST for reuse.
func (r *EclassRegistry) CacheAST(name string, ast *syntax.File) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.parsedASTs[name] = ast
}

// GetCachedAST retrieves a cached eclass AST.
func (r *EclassRegistry) GetCachedAST(name string) (*syntax.File, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ast, ok := r.parsedASTs[name]
	return ast, ok
}

// SetCurrentEclass sets the currently executing eclass (for EXPORT_FUNCTIONS).
func (r *EclassRegistry) SetCurrentEclass(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.currentEclass = name
}

// GetCurrentEclass returns the currently executing eclass.
func (r *EclassRegistry) GetCurrentEclass() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentEclass
}

// IncrementDepth increments the eclass inheritance depth.
func (r *EclassRegistry) IncrementDepth() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.eclassDepth++
	return r.eclassDepth
}

// DecrementDepth decrements the eclass inheritance depth.
func (r *EclassRegistry) DecrementDepth() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.eclassDepth--
	return r.eclassDepth
}

// ExportFunction registers a function export from an eclass.
//
// EXPORT_FUNCTIONS causes eclass functions to become default phase implementations.
// For example, EXPORT_FUNCTIONS src_compile causes eclass_src_compile to be called
// when src_compile is invoked.
func (r *EclassRegistry) ExportFunction(phase string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.currentEclass == "" {
		return fmt.Errorf("EXPORT_FUNCTIONS called without a defined ECLASS")
	}

	r.exportedFunctions[phase] = r.currentEclass
	return nil
}

// GetExportedFunction returns the eclass that provides a phase function.
func (r *EclassRegistry) GetExportedFunction(phase string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	eclass, ok := r.exportedFunctions[phase]
	return eclass, ok
}

// EclassLoader loads and executes eclasses.
type EclassLoader struct {
	registry    *EclassRegistry
	interpreter *Interpreter
	stdout      io.Writer
	stderr      io.Writer
}

// NewEclassLoader creates a new eclass loader.
func NewEclassLoader(registry *EclassRegistry, interp *Interpreter) *EclassLoader {
	return &EclassLoader{
		registry:    registry,
		interpreter: interp,
		stdout:      interp.stdout,
		stderr:      interp.stderr,
	}
}

// Inherit loads one or more eclasses.
//
// This is the implementation of the inherit() function from Portage.
// It sources the eclass bash code using the interpreter.
func (l *EclassLoader) Inherit(ctx context.Context, eclasses []string) error {
	depth := l.registry.IncrementDepth()
	defer l.registry.DecrementDepth()

	if depth > 1 {
		// Multiple inheritance - log for debugging
		l.writeStderr(fmt.Sprintf(">>> Multiple Inheritance (Level: %d)\n", depth))
	}

	for _, eclass := range eclasses {
		if err := l.loadEclass(ctx, eclass); err != nil {
			return fmt.Errorf("inheriting %s: %w", eclass, err)
		}
	}

	return nil
}

// loadEclass loads a single eclass.
func (l *EclassLoader) loadEclass(ctx context.Context, name string) error {
	// Check if already loaded
	if l.registry.IsLoaded(name) {
		l.writeStdout(fmt.Sprintf(">>> Eclass %s already inherited (skipping)\n", name))
		return nil
	}

	// Find eclass file
	path, err := l.registry.FindEclass(name)
	if err != nil {
		return err
	}

	l.writeStdout(fmt.Sprintf(">>> Inheriting eclass: %s\n", name))

	// Set current eclass for EXPORT_FUNCTIONS
	l.registry.SetCurrentEclass(name)
	defer l.registry.SetCurrentEclass("")

	// Check for built-in eclass implementations
	if handled := l.handleBuiltinEclass(name); handled {
		l.registry.MarkLoaded(name, path)
		return nil
	}

	// Read and execute eclass file
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading eclass %s: %w", name, err)
	}

	// Execute eclass content through interpreter
	if err := l.interpreter.Run(ctx, string(content)); err != nil {
		return fmt.Errorf("executing eclass %s: %w", name, err)
	}

	// Mark as loaded
	l.registry.MarkLoaded(name, path)

	return nil
}

// handleBuiltinEclass checks if an eclass has a built-in Go implementation.
//
// Returns true if the eclass is handled by Go code and doesn't need
// bash execution.
func (l *EclassLoader) handleBuiltinEclass(name string) bool {
	switch name {
	case "toolchain-funcs":
		// Already implemented in helpers.go (tc-getCC, tc-getCXX, etc.)
		return true
	case "eutils":
		// Implemented via helpers.go (epatch uses eapply, etc.)
		return true
	case "multilib":
		// Basic multilib functions implemented in helpers.go
		return true
	case "flag-o-matic":
		// Flag manipulation functions
		return true
	case "linux-info":
		// Linux kernel info functions
		return true
	default:
		return false
	}
}

func (l *EclassLoader) writeStdout(s string) {
	if l.stdout != nil {
		_, _ = io.WriteString(l.stdout, s)
	}
}

func (l *EclassLoader) writeStderr(s string) {
	if l.stderr != nil {
		_, _ = io.WriteString(l.stderr, s)
	}
}

// EclassStack provides push/pop stack operations for shell options and values.
//
// Used by eshopts_push/eshopts_pop and estack_push/estack_pop.
type EclassStack struct {
	mu     sync.Mutex
	stacks map[string][]string
}

// NewEclassStack creates a new eclass stack.
func NewEclassStack() *EclassStack {
	return &EclassStack{
		stacks: make(map[string][]string),
	}
}

// Push pushes a value onto a named stack.
func (s *EclassStack) Push(name, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.stacks[name]; !ok {
		s.stacks[name] = make([]string, 0)
	}
	s.stacks[name] = append(s.stacks[name], value)
}

// Pop pops a value from a named stack.
func (s *EclassStack) Pop(name string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stack, ok := s.stacks[name]
	if !ok || len(stack) == 0 {
		return "", false
	}

	value := stack[len(stack)-1]
	s.stacks[name] = stack[:len(stack)-1]
	return value, true
}

// Len returns the length of a named stack.
func (s *EclassStack) Len(name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if stack, ok := s.stacks[name]; ok {
		return len(stack)
	}
	return 0
}
