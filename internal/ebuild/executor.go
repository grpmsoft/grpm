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
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"

	"github.com/grpmsoft/grpm/internal/fetch"
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
}

// DefaultOptions returns default executor options.
func DefaultOptions() ExecutorOptions {
	return ExecutorOptions{
		TmpDir:        "/var/tmp/portage",
		PortDir:       "/var/db/repos/gentoo",
		DistDir:       "/var/cache/distfiles",
		EnableSandbox: true,
		EnableTests:   false,
		KeepWork:      false,
		DenyNetwork:   true, // network-sandbox by default
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
				log.Printf("[sandbox] cleanup error: %v", err)
			}
			// Report any violations
			violations := e.Sandbox.Violations()
			if len(violations) > 0 {
				log.Printf("[sandbox] %d violation(s) detected:", len(violations))
				for _, v := range violations {
					log.Printf("[sandbox]   %s", v.String())
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
//  3. Download using configured Fetcher
//  4. Verify checksums (handled by Fetcher)
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

	// Get distfiles to download
	distfiles := manifest.GetDistfiles()
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
// This is a stub - actual implementation would parse bash ebuild syntax.
func (e *Executor) ParseEbuild() error {
	// TODO: Parse ebuild file
	// - Extract DESCRIPTION, HOMEPAGE, SRC_URI, LICENSE
	// - Parse DEPEND, RDEPEND, BDEPEND
	// - Extract USE flags
	// - Find phase functions (src_configure, src_compile, etc)

	// Check if ebuild exists
	if e.EbuildPath == "" {
		// Try to construct default path
		e.EbuildPath = filepath.Join(
			e.Env.PORTDIR,
			e.Env.CATEGORY,
			e.Env.PN,
			e.Env.PF+".ebuild",
		)
	}

	// For now, just return success
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
