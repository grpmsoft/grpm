// Package ebuild implements ebuild execution engine.
//
// This file provides a bridge between the ebuild package and the new
// eclass package, enabling dynamic eclass loading while maintaining
// backward compatibility with existing Go implementations.
package ebuild

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/grpmsoft/grpm/internal/eclass"
	mvdaninterp "mvdan.cc/sh/v3/interp"
)

// DynamicEclassLoader provides dynamic eclass loading using the eclass package.
//
// It wraps the eclass.HybridLoader and adapts it to the existing
// EclassLoader interface used by the Helpers type.
type DynamicEclassLoader struct {
	// hybridLoader is the underlying hybrid loader.
	hybridLoader *eclass.HybridLoader

	// registry is the eclass registry for compatibility.
	registry *EclassRegistry

	// stdout and stderr for output.
	stdout io.Writer
	stderr io.Writer

	// goImpls stores registered Go eclass implementations.
	goImpls map[string]*GoEclassAdapter
}

// GoEclassAdapter adapts existing Go eclass implementations to the
// eclass.GoEclassImpl interface.
type GoEclassAdapter struct {
	name           string
	executeFunc    func(ctx context.Context, env map[string]string) error
	hasPhaseFunc   func(phase string) bool
	executePhase   func(ctx context.Context, phase string, env map[string]string) error
	phaseFunctions map[string]func(ctx context.Context, env map[string]string) error
}

// Name returns the eclass name.
func (a *GoEclassAdapter) Name() string {
	return a.name
}

// Execute runs the eclass setup.
func (a *GoEclassAdapter) Execute(ctx context.Context, env map[string]string) error {
	if a.executeFunc != nil {
		return a.executeFunc(ctx, env)
	}
	return nil
}

// HasPhaseFunction checks if the eclass provides a phase function.
func (a *GoEclassAdapter) HasPhaseFunction(phase string) bool {
	if a.hasPhaseFunc != nil {
		return a.hasPhaseFunc(phase)
	}
	_, ok := a.phaseFunctions[phase]
	return ok
}

// ExecutePhase runs a phase function.
func (a *GoEclassAdapter) ExecutePhase(ctx context.Context, phase string, env map[string]string) error {
	if a.executePhase != nil {
		return a.executePhase(ctx, phase, env)
	}
	if fn, ok := a.phaseFunctions[phase]; ok {
		return fn(ctx, env)
	}
	return fmt.Errorf("phase %s not provided by %s", phase, a.name)
}

// NewDynamicEclassLoader creates a new dynamic eclass loader.
//
// Parameters:
//   - cache: The eclass cache for file lookup
//   - execHandler: The exec handler for Go helper functions
//   - stdout, stderr: Output writers
func NewDynamicEclassLoader(
	cache *eclass.Cache,
	execHandler mvdaninterp.ExecHandlerFunc,
	stdout, stderr io.Writer,
) *DynamicEclassLoader {
	loader := &DynamicEclassLoader{
		stdout:  stdout,
		stderr:  stderr,
		goImpls: make(map[string]*GoEclassAdapter),
	}

	// Create hybrid loader with the exec handler
	loaderOpts := []eclass.HybridLoaderOption{
		eclass.WithHybridOutput(stdout, stderr),
		eclass.WithVerbose(true), // Enable verbose to see actual errors
	}

	loader.hybridLoader = eclass.NewHybridLoader(cache, execHandler, loaderOpts...)

	// Create registry from cache locations
	// Note: cache.Locations() returns eclass directories (e.g., /var/db/repos/gentoo/eclass)
	// but NewEclassRegistry expects repository root (e.g., /var/db/repos/gentoo)
	// So we need to get the parent directory
	portdir := ""
	if locs := cache.Locations(); len(locs) > 0 {
		// Get parent of eclass dir (e.g., /var/db/repos/gentoo/eclass -> /var/db/repos/gentoo)
		portdir = filepath.Dir(locs[0])
	}
	loader.registry = NewEclassRegistry(portdir)

	return loader
}

// SetEnv sets environment variables in the executor.
//
// This should be called before Inherit() to ensure variables like EAPI
// are available to eclasses.
func (l *DynamicEclassLoader) SetEnv(vars map[string]string) {
	for k, v := range vars {
		l.hybridLoader.GetExecutor().SetVar(k, v)
	}
}

// Inherit loads one or more eclasses.
//
// This implements the same interface as EclassLoader.Inherit for compatibility.
func (l *DynamicEclassLoader) Inherit(ctx context.Context, eclasses []string) error {
	for _, name := range eclasses {
		// Check if already loaded
		if l.registry.IsLoaded(name) {
			l.writeStdout(fmt.Sprintf(">>> Eclass %s already inherited (skipping)\n", name))
			continue
		}

		// Mark as loaded BEFORE sourcing, per Portage behavior.
		// Portage's inherit() adds to INHERITED before sourcing the eclass
		// (ebuild.sh line 384-385). This prevents infinite recursion when
		// eclass A inherits B which inherits A again.
		eclassPath := ""
		if ec, err := l.hybridLoader.GetCache().Get(name); err == nil {
			eclassPath = ec.Path
		}
		l.registry.MarkLoaded(name, eclassPath)

		// Try dynamic loading via hybrid loader
		if err := l.hybridLoader.Inherit(ctx, []string{name}); err != nil {
			return fmt.Errorf("inheriting %s: %w", name, err)
		}
	}

	return nil
}

// RegisterGoEclass registers a Go eclass implementation as fallback.
//
// This allows existing Go implementations (cmake, meson, etc.) to be
// used as fallbacks when dynamic execution fails.
func (l *DynamicEclassLoader) RegisterGoEclass(adapter *GoEclassAdapter) {
	l.goImpls[adapter.name] = adapter

	// Also register with hybrid loader
	l.hybridLoader = eclass.NewHybridLoader(
		l.hybridLoader.GetCache(),
		nil,
		eclass.WithGoFallback(adapter),
		eclass.WithHybridOutput(l.stdout, l.stderr),
	)
}

// GetExecutor returns the underlying eclass executor.
func (l *DynamicEclassLoader) GetExecutor() *eclass.Executor {
	return l.hybridLoader.GetExecutor()
}

// GetCache returns the eclass cache.
func (l *DynamicEclassLoader) GetCache() *eclass.Cache {
	return l.hybridLoader.GetCache()
}

// GetRegistry returns the eclass registry for compatibility.
func (l *DynamicEclassLoader) GetRegistry() *EclassRegistry {
	return l.registry
}

// GetInherited returns the INHERITED variable value.
func (l *DynamicEclassLoader) GetInherited() string {
	return l.registry.GetInherited()
}

// GetExportedFunction returns the eclass that exports a phase function.
func (l *DynamicEclassLoader) GetExportedFunction(phase string) (string, bool) {
	return l.hybridLoader.GetExecutor().GetExportedFunction(phase)
}

// GetAccumulatedMetadata returns accumulated metadata from eclasses.
func (l *DynamicEclassLoader) GetAccumulatedMetadata() map[string]string {
	return l.hybridLoader.GetExecutor().GetAccumulatedMetadata()
}

// FinalizeMetadata merges accumulated metadata into the environment.
func (l *DynamicEclassLoader) FinalizeMetadata() {
	l.hybridLoader.GetExecutor().FinalizeMetadata()
}

func (l *DynamicEclassLoader) writeStdout(s string) {
	if l.stdout != nil {
		_, _ = io.WriteString(l.stdout, s)
	}
}

// CreateEclassCache creates an eclass cache for the given repository paths.
//
// This is a convenience function for setting up dynamic eclass loading.
func CreateEclassCache(repoPaths []string) (*eclass.Cache, error) {
	locations := make([]string, 0, len(repoPaths))
	for _, path := range repoPaths {
		locations = append(locations, path+"/eclass")
	}
	return eclass.NewCacheWithLocations(locations)
}

// CreateDefaultEclassCache creates an eclass cache with default Gentoo locations.
func CreateDefaultEclassCache() (*eclass.Cache, error) {
	return eclass.NewCacheWithLocations(eclass.DefaultLocations())
}

// SetupDynamicEclassLoading configures the interpreter for dynamic eclass loading.
//
// This replaces the default EclassLoader with a DynamicEclassLoader.
//
// Parameters:
//   - interp: The interpreter to configure
//   - cache: The eclass cache (or nil to use defaults)
//
// Returns the configured DynamicEclassLoader for further customization.
func SetupDynamicEclassLoading(interp *Interpreter, cache *eclass.Cache) (*DynamicEclassLoader, error) {
	if cache == nil {
		var err error
		cache, err = CreateDefaultEclassCache()
		if err != nil {
			return nil, fmt.Errorf("creating default cache: %w", err)
		}
	}

	// loader is captured by closure and assigned after creation
	var loader *DynamicEclassLoader

	// Create exec handler that uses interpreter's helpers
	// This allows eclass code to call Go helper functions
	// Note: Some commands need to use Executor state directly because
	// the interpreter's helpers don't track state during dynamic execution.
	execHandler := func(ctx context.Context, args []string) error {
		if len(args) == 0 {
			return nil
		}

		cmd := args[0]
		cmdArgs := args[1:]
		hc := mvdaninterp.HandlerCtx(ctx)

		// EXPORT_FUNCTIONS must use the Executor's state directly
		// because it needs access to currentEclass which is only
		// tracked by the Executor during dynamic eclass execution
		if cmd == "EXPORT_FUNCTIONS" && loader != nil {
			if err := loader.GetExecutor().ExportFunctions(cmdArgs); err != nil {
				return &DieError{Message: fmt.Sprintf("EXPORT_FUNCTIONS: %v", err)}
			}
			return nil
		}

		// inherit from within eclasses must go through the Executor
		// to properly handle nested inheritance and EAPI requirements
		if cmd == "inherit" && loader != nil {
			if err := loader.Inherit(ctx, cmdArgs); err != nil {
				return &DieError{Message: fmt.Sprintf("inherit: %v", err)}
			}
			return nil
		}

		// Version functions need to write to context stdout for command substitution
		switch cmd {
		case "ver_cut":
			if len(cmdArgs) >= 2 {
				result, err := interp.helpers.verCutImpl(cmdArgs[0], cmdArgs[1])
				if err != nil {
					return &DieError{Message: fmt.Sprintf("ver_cut: %v", err)}
				}
				_, _ = io.WriteString(hc.Stdout, result)
				return nil
			}
			return &DieError{Message: "ver_cut: requires range and version arguments"}
		case "ver_rs":
			if len(cmdArgs) >= 3 {
				result := interp.helpers.verRsImpl(cmdArgs[0], cmdArgs[1], cmdArgs[2])
				_, _ = io.WriteString(hc.Stdout, result)
				return nil
			}
			return &DieError{Message: "ver_rs: requires range, separator, and version arguments"}
		}

		// Dispatch to interpreter's command map
		return interp.dispatchCommand(cmd, cmdArgs)
	}

	// Create dynamic loader (now the closure can access it)
	loader = NewDynamicEclassLoader(cache, execHandler, interp.stdout, interp.stderr)

	// Replace the default eclass loader
	// DynamicEclassLoader implements EclassLoaderIface directly
	interp.helpers.SetEclassLoader(loader)

	return loader, nil
}
