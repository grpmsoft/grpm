// Package eclass provides dynamic eclass loading and execution.
//
// This file implements the EclassExecutor which loads and executes
// eclasses dynamically using the mvdan.cc/sh bash interpreter.
package eclass

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// MetadataVars lists the metadata variables that are accumulated during inherit.
// These are backed up before sourcing an eclass and restored after.
var MetadataVars = []string{
	"IUSE",
	"REQUIRED_USE",
	"DEPEND",
	"RDEPEND",
	"PDEPEND",
	"BDEPEND",
	"IDEPEND",
	"PROPERTIES",
	"RESTRICT",
}

// Executor executes eclasses dynamically using the mvdan.cc/sh interpreter.
//
// It implements the Portage inherit() semantics:
//   - Sources eclass files in the shell environment
//   - Accumulates metadata (DEPEND, IUSE, etc.) incrementally
//   - Handles EXPORT_FUNCTIONS declarations
//   - Supports nested inheritance (eclass inheriting eclass)
//
// Thread-safe: Methods can be called concurrently.
type Executor struct {
	mu sync.RWMutex

	// cache is the eclass cache for file lookup.
	cache *Cache

	// env stores the current shell environment.
	env map[string]string

	// parser is reused for parsing eclass files.
	parser *syntax.Parser

	// stdout and stderr for output.
	stdout io.Writer
	stderr io.Writer

	// inherited tracks which eclasses have been loaded (INHERITED variable).
	inherited []string

	// exportedFunctions maps phase name to eclass that exports it.
	exportedFunctions map[string]string

	// currentEclass is the currently executing eclass (ECLASS variable).
	currentEclass string

	// eclassDepth tracks nested inherit depth.
	eclassDepth int

	// accumulatedMetadata stores accumulated values from eclasses.
	// Key format: "E_" + varname (e.g., E_DEPEND, E_IUSE).
	accumulatedMetadata map[string]string

	// execHandler is the optional exec handler for Go helper functions.
	execHandler interp.ExecHandlerFunc

	// openHandler is the optional open handler for file operations.
	openHandler interp.OpenHandlerFunc
}

// ExecutorOption configures an Executor.
type ExecutorOption func(*Executor)

// WithExecHandler sets a custom exec handler for intercepting commands.
//
// This allows integration with Go helper implementations (tc-getCC, use, etc.).
func WithExecHandler(handler interp.ExecHandlerFunc) ExecutorOption {
	return func(e *Executor) {
		e.execHandler = handler
	}
}

// WithOpenHandler sets a custom open handler for file operations.
func WithOpenHandler(handler interp.OpenHandlerFunc) ExecutorOption {
	return func(e *Executor) {
		e.openHandler = handler
	}
}

// WithOutput sets stdout and stderr for the executor.
func WithOutput(stdout, stderr io.Writer) ExecutorOption {
	return func(e *Executor) {
		e.stdout = stdout
		e.stderr = stderr
	}
}

// WithEnvironment sets initial environment variables.
func WithEnvironment(env map[string]string) ExecutorOption {
	return func(e *Executor) {
		for k, v := range env {
			e.env[k] = v
		}
	}
}

// NewExecutor creates a new eclass executor.
//
// Parameters:
//   - cache: The eclass cache for file lookup
//   - opts: Optional configuration options
func NewExecutor(cache *Cache, opts ...ExecutorOption) *Executor {
	e := &Executor{
		cache:               cache,
		env:                 make(map[string]string),
		parser:              syntax.NewParser(syntax.Variant(syntax.LangBash)),
		stdout:              os.Stdout,
		stderr:              os.Stderr,
		inherited:           make([]string, 0),
		exportedFunctions:   make(map[string]string),
		accumulatedMetadata: make(map[string]string),
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// Inherit loads one or more eclasses.
//
// This is the core function implementing inherit() from Portage ebuild.sh.
// It sources each eclass file, handles metadata accumulation, and tracks
// the INHERITED variable.
//
// Per Portage behavior:
//   - Already-inherited eclasses are skipped (no double-loading)
//   - Metadata variables are backed up, eclass is sourced, then metadata is accumulated
//   - ECLASS and ECLASS_DEPTH are set during execution
func (e *Executor) Inherit(ctx context.Context, eclasses []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.eclassDepth++
	defer func() { e.eclassDepth-- }()

	if e.eclassDepth > 1 {
		e.writeStderr(fmt.Sprintf(">>> Multiple Inheritance (Level: %d)\n", e.eclassDepth))
	}

	for _, name := range eclasses {
		if err := e.inheritSingleLocked(ctx, name); err != nil {
			return fmt.Errorf("inheriting %s: %w", name, err)
		}
	}

	return nil
}

// inheritSingleLocked loads a single eclass.
// Must be called with lock held.
func (e *Executor) inheritSingleLocked(ctx context.Context, name string) error {
	// Check if already inherited
	for _, existing := range e.inherited {
		if existing == name {
			e.writeStdout(fmt.Sprintf(">>> Eclass %s already inherited (skipping)\n", name))
			return nil
		}
	}

	e.writeStdout(fmt.Sprintf(">>> Inheriting eclass: %s\n", name))

	// Look up eclass in cache
	eclass, err := e.cache.Get(name)
	if err != nil {
		return err
	}

	// Set current eclass
	prevEclass := e.currentEclass
	e.currentEclass = name
	e.env["ECLASS"] = name
	defer func() {
		e.currentEclass = prevEclass
		if prevEclass != "" {
			e.env["ECLASS"] = prevEclass
		} else {
			delete(e.env, "ECLASS")
		}
	}()

	// Backup metadata variables
	backup := e.backupMetadata()

	// Unset metadata variables before sourcing
	for _, varName := range MetadataVars {
		delete(e.env, varName)
	}

	// Read and parse eclass content
	content, err := os.ReadFile(eclass.Path)
	if err != nil {
		return fmt.Errorf("reading eclass %s: %w", name, err)
	}

	// Execute eclass content
	if err := e.executeScript(ctx, string(content), eclass.Path); err != nil {
		return fmt.Errorf("executing eclass %s: %w", name, err)
	}

	// Accumulate metadata
	e.accumulateMetadata(backup)

	// Mark as inherited
	e.inherited = append(e.inherited, name)
	e.env["INHERITED"] = strings.Join(e.inherited, " ")

	return nil
}

// backupMetadata saves current values of metadata variables.
func (e *Executor) backupMetadata() map[string]string {
	backup := make(map[string]string)
	for _, varName := range MetadataVars {
		if val, ok := e.env[varName]; ok {
			backup[varName] = val
		}
	}
	return backup
}

// accumulateMetadata merges eclass-set metadata into accumulated values.
//
// Per Portage behavior:
//   - Eclass-set values are appended to E_* accumulator variables
//   - Original values (from backup) are restored to the original variables
func (e *Executor) accumulateMetadata(backup map[string]string) {
	for _, varName := range MetadataVars {
		// If eclass set this variable, append to accumulator
		if val, ok := e.env[varName]; ok && val != "" {
			accKey := "E_" + varName
			if existing := e.accumulatedMetadata[accKey]; existing != "" {
				e.accumulatedMetadata[accKey] = existing + " " + val
			} else {
				e.accumulatedMetadata[accKey] = val
			}
		}

		// Restore backed-up value (or unset if wasn't set)
		if backedUp, ok := backup[varName]; ok {
			e.env[varName] = backedUp
		} else {
			delete(e.env, varName)
		}
	}
}

// executeScript runs a bash script in the executor's environment.
func (e *Executor) executeScript(ctx context.Context, script, filename string) error {
	prog, err := e.parser.Parse(strings.NewReader(script), filename)
	if err != nil {
		return fmt.Errorf("parsing script: %w", err)
	}

	// Create runner with current environment
	runner, err := e.createRunner(ctx)
	if err != nil {
		return fmt.Errorf("creating runner: %w", err)
	}

	// Execute
	if err := runner.Run(ctx, prog); err != nil {
		return err
	}

	// Update environment from runner
	e.updateEnvFromRunner(runner)

	return nil
}

// createRunner creates a new shell interpreter runner.
func (e *Executor) createRunner(ctx context.Context) (*interp.Runner, error) {
	// Convert env map to slice
	envPairs := make([]string, 0, len(e.env))
	for k, v := range e.env {
		envPairs = append(envPairs, k+"="+v)
	}

	// Build options
	opts := []interp.RunnerOption{
		interp.StdIO(nil, e.stdout, e.stderr),
		interp.Env(expand.ListEnviron(envPairs...)),
	}

	// Add exec handler if provided
	if e.execHandler != nil {
		opts = append(opts, interp.ExecHandlers(func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
			return e.execHandler
		}))
	}

	// Add open handler if provided
	if e.openHandler != nil {
		opts = append(opts, interp.OpenHandler(e.openHandler))
	}

	return interp.New(opts...)
}

// updateEnvFromRunner syncs environment changes from the runner back to executor.
func (e *Executor) updateEnvFromRunner(runner *interp.Runner) {
	// Iterate over all variables in the runner's Vars map
	for name, vr := range runner.Vars {
		// Get the string value
		if str := vr.String(); str != "" {
			e.env[name] = str
		}
	}
}

// ExportFunctions registers phase functions from the current eclass.
//
// Usage: EXPORT_FUNCTIONS src_compile src_install
//
// After this call, when src_compile is invoked, it will call ${ECLASS}_src_compile.
func (e *Executor) ExportFunctions(phases []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.currentEclass == "" {
		return fmt.Errorf("EXPORT_FUNCTIONS called without a defined ECLASS")
	}

	for _, phase := range phases {
		e.exportedFunctions[phase] = e.currentEclass
	}

	return nil
}

// GetExportedFunction returns the eclass that exports a phase function.
//
// Returns the eclass name and true if found, empty string and false otherwise.
func (e *Executor) GetExportedFunction(phase string) (string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	eclass, ok := e.exportedFunctions[phase]
	return eclass, ok
}

// GetInherited returns the list of inherited eclasses.
func (e *Executor) GetInherited() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]string, len(e.inherited))
	copy(result, e.inherited)
	return result
}

// GetInheritedString returns the INHERITED variable value.
func (e *Executor) GetInheritedString() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return strings.Join(e.inherited, " ")
}

// GetAccumulatedMetadata returns accumulated metadata variables.
//
// These are the E_* variables containing merged values from all eclasses.
func (e *Executor) GetAccumulatedMetadata() map[string]string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make(map[string]string, len(e.accumulatedMetadata))
	for k, v := range e.accumulatedMetadata {
		result[k] = v
	}
	return result
}

// GetEnv returns the current environment.
func (e *Executor) GetEnv() map[string]string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make(map[string]string, len(e.env))
	for k, v := range e.env {
		result[k] = v
	}
	return result
}

// GetVar returns an environment variable value.
func (e *Executor) GetVar(name string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.env[name]
}

// SetVar sets an environment variable.
func (e *Executor) SetVar(name, value string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.env[name] = value
}

// FinalizeMetadata merges accumulated metadata into the environment.
//
// Call this after all inherit() calls are complete to get the final
// merged values. This combines ebuild-defined values with eclass-accumulated values.
//
// For each metadata variable:
//   - If ebuild defined it: ebuild_value + " " + accumulated_value
//   - If only eclass defined it: accumulated_value
func (e *Executor) FinalizeMetadata() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, varName := range MetadataVars {
		accKey := "E_" + varName
		accValue, hasAcc := e.accumulatedMetadata[accKey]
		ebuildValue, hasEbuild := e.env[varName]

		if hasAcc {
			if hasEbuild && ebuildValue != "" {
				// Ebuild value takes precedence, append accumulated
				e.env[varName] = ebuildValue + " " + accValue
			} else {
				// Only accumulated value
				e.env[varName] = accValue
			}
		}
	}
}

// Run executes a bash script in the executor's environment.
//
// This is useful for running ebuild scripts after eclasses are loaded.
func (e *Executor) Run(ctx context.Context, script string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.executeScript(ctx, script, "script")
}

// RunFile executes a bash script file.
func (e *Executor) RunFile(ctx context.Context, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading script %s: %w", path, err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	return e.executeScript(ctx, string(content), path)
}

// Reset clears the executor state for reuse.
func (e *Executor) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.inherited = make([]string, 0)
	e.exportedFunctions = make(map[string]string)
	e.accumulatedMetadata = make(map[string]string)
	e.currentEclass = ""
	e.eclassDepth = 0

	// Keep base environment but clear eclass-related vars
	delete(e.env, "ECLASS")
	delete(e.env, "INHERITED")
	for _, varName := range MetadataVars {
		delete(e.env, varName)
		delete(e.env, "E_"+varName)
	}
}

// writeStdout writes to stdout.
func (e *Executor) writeStdout(s string) {
	if e.stdout != nil {
		_, _ = io.WriteString(e.stdout, s)
	}
}

// writeStderr writes to stderr.
func (e *Executor) writeStderr(s string) {
	if e.stderr != nil {
		_, _ = io.WriteString(e.stderr, s)
	}
}
