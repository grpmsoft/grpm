// Package ebuild implements ebuild execution engine.
//
// This module handles execution of ebuild phases in a sandboxed environment,
// similar to Portage's ebuild.sh.
//
// Example:
//
//	executor := ebuild.NewExecutor(pkg, "/var/tmp/portage")
//	results, err := executor.ExecutePhases(ebuild.StandardPhases())
package ebuild

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/grpmsoft/grpm/internal/distfile"
	"github.com/grpmsoft/grpm/internal/eclass"
	"github.com/grpmsoft/grpm/internal/fetch"
	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/sandbox"
)

// Executor executes ebuild phases.
type Executor struct {
	// Package to build
	Package *pkg.Package

	// Environment for execution
	Env *Environment

	// EbuildPath is the path to the ebuild file
	EbuildPath string

	// EnableSandbox enables sandbox isolation
	EnableSandbox bool

	// EnableTests enables test phase execution
	EnableTests bool

	// KeepWork preserves work directory after build
	KeepWork bool

	// OnProgress is called with status updates
	OnProgress func(phase Phase, status string)

	// Fetcher handles distfile downloads (optional)
	// If nil, fetch phase is skipped
	Fetcher fetch.Fetcher

	// RepoPath is the path to the Portage repository
	// Used to locate Manifest files for distfile verification
	RepoPath string

	// Sandbox provides build isolation (nil if sandbox disabled)
	Sandbox sandbox.Sandbox

	// SandboxConfig is the sandbox configuration
	SandboxConfig *sandbox.Config

	// DenyNetwork blocks network access during build
	DenyNetwork bool

	// ParsedEbuild contains the parsed ebuild script with function definitions
	// Populated by ParseEbuild() for phase dispatch decisions
	ParsedEbuild *EbuildScript

	// EAPIFeatures contains the EAPI feature set for the ebuild
	// Populated by ParseEbuild() based on EAPI version
	EAPIFeatures pkg.EAPIFeatures

	// interpreter is the bash interpreter for executing ebuild functions
	interpreter *Interpreter

	// dynamicLoader provides dynamic eclass loading from repository
	// Uses HybridLoader with Go fallbacks for complex eclasses
	dynamicLoader *DynamicEclassLoader

	// eclassCache stores the eclass file cache for dynamic loading
	eclassCache *eclass.Cache

	// enableDynamicEclass tracks if dynamic loading is enabled
	enableDynamicEclass bool

	// currentPhase tracks the currently executing phase for EBUILD_PHASE
	currentPhase Phase
}

// ExecutorOptions configures ebuild execution.
type ExecutorOptions struct {
	// TmpDir is the temporary build directory
	TmpDir string

	// PortDir is the Portage tree directory
	PortDir string

	// DistDir is the distfiles directory
	DistDir string

	// EbuildPath is the path to the ebuild file
	EbuildPath string

	// EnableSandbox enables sandbox isolation
	EnableSandbox bool

	// EnableTests enables test phase execution
	EnableTests bool

	// KeepWork preserves work directory after build
	KeepWork bool

	// Fetcher handles distfile downloads (optional)
	// If nil, fetch phase is skipped and sources must be pre-downloaded
	Fetcher fetch.Fetcher

	// SandboxConfig overrides default sandbox configuration.
	// If nil and EnableSandbox is true, default config is used.
	SandboxConfig *sandbox.Config

	// DenyNetwork blocks network access during build.
	// Implements Portage's network-sandbox feature.
	DenyNetwork bool

	// EnableDynamicEclass enables dynamic eclass loading from repository.
	// When true (default), eclasses are loaded directly from eclass/ directories
	// using mvdan.cc/sh interpreter, with fallback to Go implementations.
	// This addresses the community feedback about hardcoded eclasses.
	EnableDynamicEclass bool

	// EclassLocations specifies custom eclass search paths.
	// If empty, defaults to [PortDir/eclass, /var/db/repos/gentoo/eclass].
	EclassLocations []string
}

// DefaultOptions returns default executor options.
func DefaultOptions() ExecutorOptions {
	return ExecutorOptions{
		TmpDir:              "/var/tmp/portage",
		PortDir:             "/var/db/repos/gentoo",
		DistDir:             "/var/cache/distfiles",
		EnableSandbox:       true,
		EnableTests:         false,
		KeepWork:            false,
		DenyNetwork:         true, // network-sandbox by default
		EnableDynamicEclass: true, // dynamic eclass loading by default
	}
}

// NewExecutor creates a new ebuild executor.
func NewExecutor(pkg *pkg.Package, opts ExecutorOptions) (*Executor, error) {
	if pkg == nil {
		return nil, fmt.Errorf("package is nil")
	}

	// Create environment
	env, err := NewEnvironment(pkg, opts.TmpDir, opts.PortDir, opts.DistDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create environment: %w", err)
	}

	executor := &Executor{
		Package:       pkg,
		Env:           env,
		EbuildPath:    opts.EbuildPath,
		EnableSandbox: opts.EnableSandbox,
		EnableTests:   opts.EnableTests,
		KeepWork:      opts.KeepWork,
		Fetcher:       opts.Fetcher,
		RepoPath:      opts.PortDir,
		DenyNetwork:   opts.DenyNetwork,
	}

	// Initialize sandbox if enabled
	if opts.EnableSandbox {
		sbConfig := opts.SandboxConfig
		if sbConfig == nil {
			sbConfig = sandbox.DefaultConfig()
		}

		// Apply executor settings to sandbox config
		sbConfig.DenyNetwork = opts.DenyNetwork

		// Add build directories as writable
		sbConfig = sbConfig.WithWorkdir(
			env.WORKDIR,
			env.D,
			env.T,
			env.HOME,
		).WithDistDir(env.DISTDIR).WithRepoPath(opts.PortDir)

		sb, err := sandbox.New(sbConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create sandbox: %w", err)
		}

		executor.Sandbox = sb
		executor.SandboxConfig = sbConfig
	}

	// Initialize dynamic eclass loading if enabled
	if opts.EnableDynamicEclass {
		executor.enableDynamicEclass = true

		locations := opts.EclassLocations
		if len(locations) == 0 {
			// Default locations: PortDir/eclass and standard Gentoo path
			locations = []string{
				filepath.Join(opts.PortDir, "eclass"),
			}
		}

		cache, err := eclass.NewCacheWithLocations(locations)
		if err != nil {
			// Non-fatal: fall back to Go implementations
			logging.Debug("[ebuild] warning: failed to create eclass cache: %v (using Go fallbacks)", err)
		} else {
			executor.eclassCache = cache
		}
	}

	return executor, nil
}

// ExecutePhases executes a list of ebuild phases.
//
// Returns results for each phase. If a phase fails, execution stops
// and remaining phases are skipped.
//
// If a Fetcher is configured, distfiles are automatically downloaded
// before the unpack phase.
//
// If sandbox is enabled, commands are executed within the sandbox environment.
func (e *Executor) ExecutePhases(phases []Phase) ([]PhaseResult, error) {
	// Create build directories
	if err := e.Env.CreateDirectories(); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	// Cleanup unless KeepWork is set
	if !e.KeepWork {
		defer func() {
			_ = e.Env.Cleanup()
		}()
	}

	// Cleanup sandbox when done
	if e.Sandbox != nil {
		defer func() {
			if err := e.Sandbox.Close(); err != nil {
				logging.Debug("[sandbox] cleanup error: %v", err)
			}
			// Report any violations
			violations := e.Sandbox.Violations()
			if len(violations) > 0 {
				logging.Debug("[sandbox] %d violation(s) detected:", len(violations))
				for _, v := range violations {
					logging.Debug("[sandbox]   %s", v.String())
				}
			}
		}()
	}

	results := make([]PhaseResult, 0, len(phases))
	fetchDone := false

	for _, phase := range phases {
		// Skip test phase if not enabled
		if phase == PhaseTest && !e.EnableTests {
			e.progress(phase, "skipping tests (not enabled)")
			continue
		}

		// Fetch distfiles before unpack phase (only once)
		if phase == PhaseUnpack && !fetchDone && e.Fetcher != nil {
			e.progress(PhaseFetch, "starting")
			if err := e.fetchDistfiles(context.Background()); err != nil {
				result := PhaseResult{
					Phase:   PhaseFetch,
					Success: false,
					Error:   err,
				}
				results = append(results, result)
				e.progress(PhaseFetch, fmt.Sprintf("failed: %v", err))
				return results, fmt.Errorf("fetch phase failed: %w", err)
			}
			e.progress(PhaseFetch, "completed")
			fetchDone = true
		}

		e.progress(phase, "starting")

		// Use real phase implementation from phases_impl.go
		result := e.ExecutePhaseReal(phase)
		results = append(results, result)

		if !result.Success {
			e.progress(phase, fmt.Sprintf("failed: %v", result.Error))
			return results, fmt.Errorf("phase %s failed: %w", phase, result.Error)
		}

		e.progress(phase, fmt.Sprintf("completed in %dms", result.Duration))
	}

	return results, nil
}

// fetchDistfiles downloads source tarballs for the package.
//
// Process:
//  1. Parse Manifest file from repository
//  2. Extract DIST entries (distfiles to download)
//  3. Parse SRC_URI from ebuild to get explicit download URLs
//  4. Expand mirror:// URLs to real HTTP(S) URLs
//  5. Download using configured Fetcher with expanded URLs
//  6. Verify checksums (handled by Fetcher)
//
// Mirror URL expansion (v0.7.4+):
//   - mirror://gnu/... → https://ftp.gnu.org/gnu/..., https://mirrors.kernel.org/gnu/...
//   - mirror://sourceforge/... → https://downloads.sourceforge.net/...
//
// Returns nil if no distfiles are needed or fetch succeeds.
func (e *Executor) fetchDistfiles(ctx context.Context) error {
	if e.Fetcher == nil {
		return nil // No fetcher configured, skip
	}

	// Build Manifest path: repo/category/package/Manifest
	manifestPath := fetch.ManifestPath(e.RepoPath, e.Package.Name)

	// Parse Manifest file
	manifest, err := fetch.ParseManifest(manifestPath)
	if err != nil {
		// Manifest not found is not an error - package may not need distfiles
		if isManifestNotFound(err) {
			e.progress(PhaseFetch, "no Manifest file found, skipping fetch")
			return nil
		}
		return fmt.Errorf("parsing Manifest: %w", err)
	}

	// Get distfiles with expanded URIs from SRC_URI
	distfiles, err := e.getDistfilesWithURIs(manifest)
	if err != nil {
		// Fallback to manifest-only distfiles if SRC_URI parsing fails
		logging.Debug("[ebuild] Warning: SRC_URI parsing failed: %v, using manifest-only", err)
		distfiles = manifest.GetDistfiles()
	}

	if len(distfiles) == 0 {
		e.progress(PhaseFetch, "no distfiles required")
		return nil
	}

	e.progress(PhaseFetch, fmt.Sprintf("downloading %d distfile(s)", len(distfiles)))

	// Update environment with archive names (A variable)
	e.updateArchiveList(distfiles)

	// Download all distfiles
	if err := e.Fetcher.Fetch(ctx, distfiles, e.Env.DISTDIR); err != nil {
		return fmt.Errorf("downloading distfiles: %w", err)
	}

	return nil
}

// getDistfilesWithURIs resolves distfiles using the unified distfile.Service.
//
// This is the single source of truth for SRC_URI resolution, handling:
//   - Custom variables (MY_P, MY_PN, etc.)
//   - Eclass-generated SRC_URI
//   - Version-specific distfile filtering
//   - mirror:// URL expansion
func (e *Executor) getDistfilesWithURIs(manifest *fetch.Manifest) ([]fetch.Distfile, error) {
	if e.EbuildPath == "" {
		return manifest.GetDistfiles(), nil
	}

	// Use unified distfile service (single source of truth)
	evaluator := NewSrcURIEvaluator()
	svc := distfile.NewService(e.RepoPath, evaluator)
	ctx := context.Background()

	distfiles, err := svc.ResolveDistfiles(ctx, e.Package, e.EbuildPath, manifest)
	if err != nil {
		logging.Warn("distfile resolution failed: %v, using manifest", err)
		return manifest.GetDistfiles(), nil
	}

	return distfiles, nil
}

// extractEbuildVariable extracts a variable value from ebuild content.
//
// Handles both single-line and multi-line assignments:
//   - VAR="value"
//   - VAR='value'
//   - VAR="line1
//     line2"
func extractEbuildVariable(content, varName string) string {
	// Find variable assignment
	patterns := []string{
		varName + `="`,
		varName + `='`,
		varName + `=`,
	}

	for _, pattern := range patterns {
		// Search for pattern at word boundary (start of line or after whitespace)
		idx := findVariableAssignment(content, pattern)
		if idx == -1 {
			continue
		}

		start := idx + len(pattern)
		quote := byte(0)
		if pattern[len(pattern)-1] == '"' || pattern[len(pattern)-1] == '\'' {
			quote = pattern[len(pattern)-1]
		}

		// Find end of value
		var value []byte
		escaped := false
		for i := start; i < len(content); i++ {
			c := content[i]

			if escaped {
				value = append(value, c)
				escaped = false
				continue
			}

			if c == '\\' {
				escaped = true
				continue
			}

			if quote != 0 && c == quote {
				return string(value)
			}

			if quote == 0 && (c == ' ' || c == '\t' || c == '\n') {
				return string(value)
			}

			value = append(value, c)
		}

		return string(value)
	}

	return ""
}

// expandVariables expands ${VAR} and $VAR references in string.
func expandVariables(s string, vars map[string]string) string {
	result := s

	// Expand ${VAR} syntax
	for name, value := range vars {
		result = replaceAll(result, "${"+name+"}", value)
	}

	// Expand $VAR syntax (only followed by non-alphanumeric)
	for name, value := range vars {
		result = replaceAll(result, "$"+name+"/", value+"/")
		result = replaceAll(result, "$"+name+".", value+".")
		result = replaceAll(result, "$"+name+"-", value+"-")
		result = replaceAll(result, "$"+name+" ", value+" ")
		result = replaceAll(result, "$"+name+"\t", value+"\t")
		result = replaceAll(result, "$"+name+"\n", value+"\n")
	}

	return result
}

// findVariableAssignment finds a variable assignment pattern at word boundary.
// Returns -1 if not found.
// A valid word boundary is: start of string, newline, or tab/space.
// This prevents matching "S=" inside "KEYWORDS=" for example.
func findVariableAssignment(s, pattern string) int {
	for i := 0; i <= len(s)-len(pattern); i++ {
		if s[i:i+len(pattern)] == pattern {
			// Check if at word boundary
			if i == 0 {
				return i // Start of string
			}
			prevChar := s[i-1]
			if prevChar == '\n' || prevChar == '\t' || prevChar == ' ' {
				return i // After whitespace/newline
			}
			// Not at word boundary, continue searching
		}
	}
	return -1
}

// replaceAll replaces all occurrences of old with new in s.
func replaceAll(s, old, new string) string {
	if old == "" {
		return s
	}
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		if i <= len(s)-len(old) && s[i:i+len(old)] == old {
			result = append(result, new...)
			i += len(old)
		} else {
			result = append(result, s[i])
			i++
		}
	}
	return string(result)
}

// updateArchiveList sets the A environment variable with archive filenames.
//
// A is a space-separated list of all source archives for the package.
// Used by ebuilds to know which files to unpack.
func (e *Executor) updateArchiveList(distfiles []fetch.Distfile) {
	names := make([]string, 0, len(distfiles))
	for _, df := range distfiles {
		names = append(names, df.Filename)
	}
	e.Env.A = joinStrings(names, " ")
}

// isManifestNotFound checks if error indicates Manifest file not found.
func isManifestNotFound(err error) bool {
	if err == nil {
		return false
	}
	// Check for our specific error type using errors.Is
	return errors.Is(err, fetch.ErrManifestNotFound)
}

// joinStrings joins strings with a separator.
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for _, s := range strs[1:] {
		result += sep + s
	}
	return result
}

// executeBashScript executes a bash script in the ebuild environment.
//
// This is a helper for executing ebuild phase functions.
// TODO: Use this method when implementing custom ebuild phase execution.
func (e *Executor) executeBashScript(script string) (string, error) {
	// Prepare bash command
	cmd := exec.Command("bash", "-c", script)

	// Set environment
	cmd.Env = e.Env.ToSlice()

	// Set working directory
	cmd.Dir = e.Env.S

	// Execute and capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("script execution failed: %w", err)
	}

	return string(output), nil
}

// Suppress unused warning - this will be used in future implementation
var _ = (*Executor).executeBashScript

// GetImageDirectory returns the installation image directory (D).
//
// This is the directory where files are installed during the install phase,
// and will be merged to the system by the package installer.
func (e *Executor) GetImageDirectory() string {
	return e.Env.D
}

// GetWorkDirectory returns the work directory.
func (e *Executor) GetWorkDirectory() string {
	return e.Env.WORKDIR
}

// progress reports phase execution progress.
func (e *Executor) progress(phase Phase, status string) {
	if e.OnProgress != nil {
		e.OnProgress(phase, status)
	}
}

// ParseEbuild parses an ebuild file and extracts metadata.
//
// Parses the ebuild to extract:
//   - Function definitions (src_configure, src_compile, etc.)
//   - Inherited eclasses
//   - EAPI version
//
// This information is used for phase dispatch decisions.
func (e *Executor) ParseEbuild() error {
	// Construct ebuild path if not set
	if e.EbuildPath == "" {
		e.EbuildPath = filepath.Join(
			e.Env.PORTDIR,
			e.Env.CATEGORY,
			e.Env.PN,
			e.Env.PF+".ebuild",
		)
	}

	// Check if ebuild file exists
	if _, err := os.Stat(e.EbuildPath); err != nil {
		if os.IsNotExist(err) {
			// No ebuild file - this is OK, use defaults
			logging.Debug("[ebuild] no ebuild file found at %s, using defaults", e.EbuildPath)
			return nil
		}
		return fmt.Errorf("checking ebuild file: %w", err)
	}

	// Parse the ebuild script
	parsed, err := ParseEbuildScript(e.EbuildPath)
	if err != nil {
		return fmt.Errorf("parsing ebuild: %w", err)
	}

	e.ParsedEbuild = parsed

	// Update EAPI in environment if specified in ebuild
	if parsed.EAPI != "" && parsed.EAPI != "0" {
		e.Env.EAPI = parsed.EAPI
	}

	// Validate EAPI per PMS Chapter 2
	// This must happen before any other operations on the package
	eapi := pkg.NormalizeEAPI(e.Env.EAPI)
	if err := pkg.ValidateEAPI(eapi); err != nil {
		return fmt.Errorf("ebuild %s: %w", e.EbuildPath, err)
	}
	e.Env.EAPI = eapi

	// Get EAPI features (this won't fail now since ValidateEAPI passed)
	eapiFeatures, err := pkg.GetEAPIFeatures(eapi)
	if err != nil {
		// This should not happen since ValidateEAPI already checked
		return fmt.Errorf("ebuild %s has %w", e.EbuildPath, err)
	}
	e.EAPIFeatures = eapiFeatures

	logging.Debug("[ebuild] EAPI %s: bash %s, features: BDEPEND=%v, IDEPEND=%v, SlotOperators=%v",
		eapiFeatures.Version, eapiFeatures.BashVersion,
		eapiFeatures.BDEPEND, eapiFeatures.IDEPEND, eapiFeatures.SlotOperators)

	// Log discovered phase functions
	if len(parsed.DefinedFunctions) > 0 {
		logging.Debug("[ebuild] discovered %d functions in %s", len(parsed.DefinedFunctions), filepath.Base(e.EbuildPath))
		for name := range parsed.DefinedFunctions {
			logging.Debug("[ebuild]   - %s", name)
		}
	}

	// Log inherited eclasses
	if len(parsed.InheritedEclasses) > 0 {
		logging.Debug("[ebuild] inherits: %v", parsed.InheritedEclasses)
	}

	// S variable handling: Do NOT parse S from ebuild with regex.
	// Ebuilds often define S conditionally (if [[ ${PV} == *_p* ]]; then S=...),
	// which regex-based extraction cannot handle correctly. Instead, we rely on:
	// 1. Default S = WORKDIR/P (set by NewEnvironment)
	// 2. __grpm_sync_env in the combined script reads $S from bash AFTER
	//    the ebuild's conditionals have been properly evaluated.
	logging.Debug("[ebuild] S value (default): %s", e.Env.S)

	return nil
}

// parseSVariable parses and evaluates the S variable from ebuild content.
//
// Many packages define custom S (source directory) using bash parameter expansion:
//   - S=${WORKDIR}/${PN/f/F}-${PV} (e.g., screenfetch -> screenFetch)
//   - S=${WORKDIR}/${MY_P}
//   - S=${WORKDIR}/source
//
// This method reads the ebuild, extracts S, expands variables and parameter
// substitutions, and updates e.Env.S if different from default.
func (e *Executor) parseSVariable() error {
	if e.EbuildPath == "" {
		return nil
	}

	// Read ebuild content
	content, err := os.ReadFile(e.EbuildPath)
	if err != nil {
		return fmt.Errorf("reading ebuild: %w", err)
	}

	// Build variable map for expansion
	vars := e.Env.ToMap()

	// Parse S variable with bash parameter expansion
	sValue := ParseSVariable(string(content), vars)
	if sValue == "" {
		return nil // S not defined, use default
	}

	// Update S if different from default
	if sValue != e.Env.S {
		logging.Debug("[ebuild] custom S detected: %s (default was: %s)", sValue, e.Env.S)
		e.Env.S = sValue
	}

	return nil
}

// RunCommand executes a command, optionally within the sandbox.
//
// If sandbox is enabled, the command runs with filesystem isolation.
// The command's environment is set to the ebuild environment variables.
//
// Parameters:
//   - ctx: Context for cancellation
//   - cmd: Command to execute (Path and Args must be set)
//
// Returns combined stdout/stderr output and any error.
func (e *Executor) RunCommand(ctx context.Context, cmd *exec.Cmd) (string, error) {
	// Set environment if not already set
	if len(cmd.Env) == 0 {
		cmd.Env = e.Env.ToSlice()
	}

	// Set working directory if not set
	if cmd.Dir == "" {
		cmd.Dir = e.Env.S
	}

	// Run through sandbox if enabled
	if e.Sandbox != nil && e.EnableSandbox {
		if err := e.Sandbox.Run(ctx, cmd); err != nil {
			return "", fmt.Errorf("sandboxed command failed: %w", err)
		}
		return "", nil
	}

	// Run directly without sandbox
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}
	return string(output), nil
}

// RunSandboxedCommand is a convenience wrapper for running commands in sandbox.
//
// Creates an exec.Cmd and runs it through RunCommand.
//
// Parameters:
//   - ctx: Context for cancellation
//   - name: Command name (path to executable)
//   - args: Command arguments
//
// Returns combined stdout/stderr output and any error.
func (e *Executor) RunSandboxedCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return e.RunCommand(ctx, cmd)
}

// SandboxViolations returns any sandbox violations detected during execution.
//
// Returns nil if sandbox is disabled or no violations occurred.
func (e *Executor) SandboxViolations() []sandbox.Violation {
	if e.Sandbox == nil {
		return nil
	}
	return e.Sandbox.Violations()
}

// IsSandboxEnabled returns true if sandbox isolation is active.
func (e *Executor) IsSandboxEnabled() bool {
	return e.EnableSandbox && e.Sandbox != nil
}

// ============================================================================
// Phase Dispatch Methods
// ============================================================================

// HasPhaseFunction checks if the ebuild defines a custom phase function.
//
// Per PMS Section 8, ebuilds can override default phase implementations by
// defining functions like src_configure(), src_compile(), etc.
//
// This method checks:
//  1. If ebuild defines the function directly
//  2. If an inherited eclass exports the function via EXPORT_FUNCTIONS
//
// Returns true if custom function should be called instead of default.
func (e *Executor) HasPhaseFunction(phase Phase) bool {
	funcName := phaseFunctionName(phase)
	if funcName == "" {
		return false
	}

	// Check parsed ebuild first
	if e.ParsedEbuild != nil {
		if e.ParsedEbuild.HasFunction(funcName) {
			return true
		}
	}

	// Check if an eclass exported this phase function
	if e.interpreter != nil {
		helpers := e.interpreter.GetHelpers()
		if helpers != nil && helpers.eclassRegistry != nil {
			if _, ok := helpers.eclassRegistry.GetExportedFunction(funcName); ok {
				return true
			}
		}
	}

	// Also check the dynamic eclass loader's executor (separate EXPORT_FUNCTIONS state)
	if e.dynamicLoader != nil {
		if _, ok := e.dynamicLoader.GetExportedFunction(funcName); ok {
			return true
		}
	}

	return false
}

// RunPhaseFunction executes the ebuild's custom phase function.
//
// This method:
//  1. Sets up the EBUILD_PHASE environment variable
//  2. Sources the ebuild file
//  3. Calls the phase function (e.g., src_configure)
//  4. Returns output and any error
//
// Per PMS Section 8, the execution follows this pattern:
//   - If ebuild defines the function directly -> call it
//   - If eclass exported the function via EXPORT_FUNCTIONS -> call eclass version
//   - The function can call `default` to invoke default_src_* implementation
//
// The function is executed through the embedded bash interpreter using a
// combined script that sources the ebuild and calls the function in one pass.
// This is necessary because mvdan.cc/sh doesn't persist function definitions
// between runs.
func (e *Executor) RunPhaseFunction(phase Phase) (string, error) {
	funcName := phaseFunctionName(phase)
	if funcName == "" {
		return "", fmt.Errorf("unknown phase: %s", phase)
	}

	// Update current phase for EBUILD_PHASE
	e.currentPhase = phase
	e.Env.EBUILD_PHASE = string(phase)

	// Set EBUILD_PHASE environment variable for subprocesses
	if err := os.Setenv("EBUILD_PHASE", string(phase)); err != nil {
		logging.Debug("[ebuild] warning: failed to set EBUILD_PHASE: %v", err)
	}
	defer func() { _ = os.Unsetenv("EBUILD_PHASE") }()

	// Initialize interpreter if needed
	if e.interpreter == nil {
		if err := e.initInterpreter(); err != nil {
			return "", fmt.Errorf("initializing interpreter: %w", err)
		}
	}

	// Check if we need to call an eclass function instead of ebuild's own
	eclassName := ""
	helpers := e.interpreter.GetHelpers()
	if helpers != nil && helpers.eclassRegistry != nil {
		if ec, ok := helpers.eclassRegistry.GetExportedFunction(funcName); ok {
			eclassName = ec
		}
	}
	// Also check dynamic loader (separate EXPORT_FUNCTIONS state)
	if eclassName == "" && e.dynamicLoader != nil {
		if ec, ok := e.dynamicLoader.GetExportedFunction(funcName); ok {
			eclassName = ec
		}
	}

	// Build a combined script that:
	// 1. Sets EBUILD_PHASE
	// 2. Sources the ebuild (to define functions)
	// 3. Calls the phase function
	//
	// This must be done in a single Run() call because mvdan.cc/sh
	// doesn't persist function definitions between runs.
	var combinedScript bytes.Buffer
	combinedScript.WriteString("#!/bin/bash\n")
	combinedScript.WriteString(fmt.Sprintf("EBUILD_PHASE=%s\n", phase))
	combinedScript.WriteString(fmt.Sprintf("EBUILD_PHASE_FUNC=%s\n", funcName))

	// Source the ebuild to define all functions
	if e.EbuildPath != "" {
		if _, err := os.Stat(e.EbuildPath); err == nil {
			content, err := os.ReadFile(e.EbuildPath)
			if err != nil {
				return "", fmt.Errorf("reading ebuild: %w", err)
			}
			// Embed ebuild content directly in the script
			// This ensures function definitions persist for the function call
			combinedScript.WriteString("\n# --- BEGIN EBUILD SOURCE ---\n")
			combinedScript.Write(content)
			combinedScript.WriteString("\n# --- END EBUILD SOURCE ---\n\n")
		}
	}

	// Sync critical variables from bash env to Go struct.
	// Ebuilds may override S, WORKDIR, etc. in top-level code (e.g., S="${WORKDIR}/${MY_P}").
	// This command is intercepted by the interpreter to update env.S, env.WORKDIR.
	combinedScript.WriteString("__grpm_sync_env\n")

	// Set CWD per Portage's phase-functions.sh rules.
	// src_unpack runs in $WORKDIR; src_prepare through src_install run in $S
	// (with $WORKDIR fallback if $S doesn't exist, per EAPI 7+).
	// pkg_* phases don't change directory.
	switch phase {
	case PhaseUnpack:
		combinedScript.WriteString("cd \"${WORKDIR}\" || true\n")
	case PhasePrepare, PhaseConfigure, PhaseCompile, PhaseTest, PhaseInstall:
		combinedScript.WriteString("if [[ -d \"${S}\" ]]; then cd \"${S}\"; else cd \"${WORKDIR}\"; fi\n")
	}

	// Determine which function to call
	targetFunc := funcName
	if eclassName != "" {
		// Eclass exported this phase, call eclass's prefixed version
		// e.g., cmake_src_configure instead of src_configure
		targetFunc = fmt.Sprintf("%s_%s", eclassName, funcName)
		logging.Debug("[ebuild] calling eclass function: %s (from %s.eclass)", targetFunc, eclassName)
	} else {
		logging.Debug("[ebuild] calling ebuild function: %s", funcName)
	}

	// Call the phase function
	combinedScript.WriteString(fmt.Sprintf("%s\n", targetFunc))

	// Ensure exit status 0 if function completed successfully.
	// This is needed because mvdan.cc/sh may propagate the exit status
	// from the last command in the function (e.g., `if use flag; then...; fi`
	// where `use flag` returns 1). In standard bash, an if statement with
	// a false condition and no else clause returns 0, but mvdan.cc/sh
	// may return the condition's exit status when using ExecHandlers.
	// Adding `true` ensures the script returns 0 unless there's a real error.
	combinedScript.WriteString("true\n")

	// Execute through interpreter with output capture
	var output bytes.Buffer
	ctx := context.Background()

	// Create interpreter with output capture.
	// Share the eclass loader from the executor's main interpreter so that
	// dynamically loaded eclasses (meson, cmake, etc.) and their
	// EXPORT_FUNCTIONS are available when the ebuild is sourced.
	logging.Debug("[ebuild] RunPhaseFunction: creating interpreter with S=%s", e.Env.S)
	interp := NewInterpreter(e.Env, &output, &output)
	if e.interpreter != nil {
		mainHelpers := e.interpreter.GetHelpers()
		if mainHelpers != nil {
			// Share the eclass loader so inherit() can load eclasses from repository
			if loader := mainHelpers.GetEclassLoader(); loader != nil {
				interp.GetHelpers().SetEclassLoader(loader)
			}
			// Share the eclass registry so EXPORT_FUNCTIONS state persists
			if mainHelpers.eclassRegistry != nil {
				interp.GetHelpers().eclassRegistry = mainHelpers.eclassRegistry
			}
		}
	}

	// Execute the combined script
	if err := interp.Run(ctx, combinedScript.String()); err != nil {
		return output.String(), fmt.Errorf("executing %s: %w", funcName, err)
	}

	return output.String(), nil
}

// initInterpreter initializes the bash interpreter for the executor.
func (e *Executor) initInterpreter() error {
	e.interpreter = NewInterpreter(e.Env, os.Stdout, os.Stderr)

	// Setup dynamic eclass loading if cache is available
	if e.enableDynamicEclass && e.eclassCache != nil {
		loader, err := SetupDynamicEclassLoading(e.interpreter, e.eclassCache)
		if err != nil {
			// Non-fatal: fall back to Go implementations
			logging.Debug("[ebuild] warning: failed to setup dynamic eclass loading: %v", err)
		} else {
			e.dynamicLoader = loader
			logging.Debug("[ebuild] dynamic eclass loading enabled (%d locations)",
				len(e.eclassCache.Locations()))
		}
	}

	return nil
}

// GetCurrentPhase returns the currently executing phase.
func (e *Executor) GetCurrentPhase() Phase {
	return e.currentPhase
}

// SetCurrentPhase sets the current phase (for EBUILD_PHASE).
func (e *Executor) SetCurrentPhase(phase Phase) {
	e.currentPhase = phase

	// Update environment struct
	if e.Env != nil {
		e.Env.EBUILD_PHASE = string(phase)
	}

	// Also set in OS environment for subprocesses
	if err := os.Setenv("EBUILD_PHASE", string(phase)); err != nil {
		logging.Debug("[ebuild] warning: failed to set EBUILD_PHASE: %v", err)
	}
}

// GetInterpreter returns the bash interpreter, creating one if needed.
func (e *Executor) GetInterpreter() *Interpreter {
	if e.interpreter == nil {
		_ = e.initInterpreter()
	}
	return e.interpreter
}

// GetEAPIFeatures returns the EAPI feature set for this ebuild.
//
// If ParseEbuild() hasn't been called, this returns features for the default EAPI
// (set in the environment, typically EAPI 8).
func (e *Executor) GetEAPIFeatures() pkg.EAPIFeatures {
	// Return cached features if available
	if e.EAPIFeatures.IsValid() {
		return e.EAPIFeatures
	}

	// Otherwise look up from current EAPI
	features, err := pkg.GetEAPIFeatures(e.Env.EAPI)
	if err != nil {
		// Return default EAPI features on error
		return pkg.MustGetEAPIFeatures(pkg.DefaultEAPI())
	}
	return features
}

// SupportsFeature checks if the ebuild's EAPI supports a specific feature.
//
// Available feature checks:
//   - "bdepend": BDEPEND variable (EAPI 7+)
//   - "idepend": IDEPEND variable (EAPI 8+)
//   - "slot_operators": := and :* operators (EAPI 5+)
//   - "subslots": Subslot support (EAPI 5+)
//   - "required_use": REQUIRED_USE validation (EAPI 4+)
//   - "dosym_relative": dosym -r flag (EAPI 8+)
//   - "eapply": eapply helper (EAPI 6+)
//   - "src_uri_arrows": SRC_URI -> rename syntax (EAPI 2+)
//
// Returns false for unknown feature names.
func (e *Executor) SupportsFeature(feature string) bool {
	f := e.GetEAPIFeatures()

	switch feature {
	case "bdepend":
		return f.BDEPEND
	case "idepend":
		return f.IDEPEND
	case "slot_operators":
		return f.SlotOperators
	case "subslots":
		return f.SubSlots
	case "required_use":
		return f.RequiredUse
	case "dosym_relative":
		return f.DosymRelative
	case "eapply":
		return f.Eapply
	case "src_uri_arrows":
		return f.SrcURIArrows
	case "failglob":
		return f.Failglob
	case "use_deps":
		return f.UseDeps
	case "iuse_defaults":
		return f.IUSEDefaults
	default:
		return false
	}
}
