// Package eclass provides dynamic eclass loading and execution.
//
// This file provides integration utilities for connecting the eclass
// package with the existing ebuild execution infrastructure.
package eclass

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/interp"
)

// HybridLoader provides hybrid eclass loading with dynamic execution
// and fallback to Go implementations.
//
// It tries dynamic loading first, then falls back to registered Go
// implementations for eclasses that cannot be dynamically executed.
type HybridLoader struct {
	// cache is the eclass cache for file lookup.
	cache *Cache

	// executor is the dynamic executor.
	executor *Executor

	// goFallbacks maps eclass name to Go implementation.
	goFallbacks map[string]GoEclassImpl

	// stdout and stderr for output.
	stdout io.Writer
	stderr io.Writer

	// verbose enables verbose logging.
	verbose bool
}

// GoEclassImpl is a Go implementation of an eclass.
//
// This allows keeping existing Go implementations as fallbacks
// when dynamic execution fails or for performance-critical eclasses.
type GoEclassImpl interface {
	// Name returns the eclass name (e.g., "cmake", "meson").
	Name() string

	// Execute runs the eclass setup with the given environment.
	// It should modify env to add IUSE, DEPEND, etc.
	Execute(ctx context.Context, env map[string]string) error

	// HasPhaseFunction checks if the eclass provides a phase function.
	HasPhaseFunction(phase string) bool

	// ExecutePhase runs a phase function.
	ExecutePhase(ctx context.Context, phase string, env map[string]string) error
}

// HybridLoaderOption configures a HybridLoader.
type HybridLoaderOption func(*HybridLoader)

// WithGoFallback registers a Go eclass implementation as fallback.
func WithGoFallback(impl GoEclassImpl) HybridLoaderOption {
	return func(h *HybridLoader) {
		h.goFallbacks[impl.Name()] = impl
	}
}

// WithVerbose enables verbose logging.
func WithVerbose(verbose bool) HybridLoaderOption {
	return func(h *HybridLoader) {
		h.verbose = verbose
	}
}

// WithHybridOutput sets stdout and stderr.
func WithHybridOutput(stdout, stderr io.Writer) HybridLoaderOption {
	return func(h *HybridLoader) {
		h.stdout = stdout
		h.stderr = stderr
	}
}

// NewHybridLoader creates a new hybrid loader.
func NewHybridLoader(cache *Cache, execHandler interp.ExecHandlerFunc, opts ...HybridLoaderOption) *HybridLoader {
	h := &HybridLoader{
		cache:       cache,
		goFallbacks: make(map[string]GoEclassImpl),
		stdout:      os.Stdout,
		stderr:      os.Stderr,
	}

	for _, opt := range opts {
		opt(h)
	}

	// Create executor with the exec handler
	execOpts := []ExecutorOption{
		WithOutput(h.stdout, h.stderr),
	}
	if execHandler != nil {
		execOpts = append(execOpts, WithExecHandler(execHandler))
	}
	h.executor = NewExecutor(cache, execOpts...)

	return h
}

// Inherit loads one or more eclasses using hybrid approach.
//
// For each eclass:
//  1. Try dynamic loading from cache
//  2. If dynamic loading fails, try Go fallback
//  3. If both fail, return error
func (h *HybridLoader) Inherit(ctx context.Context, eclasses []string) error {
	for _, name := range eclasses {
		if err := h.inheritSingle(ctx, name); err != nil {
			return fmt.Errorf("inheriting %s: %w", name, err)
		}
	}
	return nil
}

func (h *HybridLoader) inheritSingle(ctx context.Context, name string) error {
	// Check if already inherited
	if h.executor.GetInheritedString() != "" {
		for _, existing := range strings.Fields(h.executor.GetInheritedString()) {
			if existing == name {
				h.writeStdout(fmt.Sprintf(">>> Eclass %s already inherited (skipping)\n", name))
				return nil
			}
		}
	}

	// Try dynamic loading first
	if h.cache.Has(name) {
		err := h.executor.Inherit(ctx, []string{name})
		if err == nil {
			if h.verbose {
				h.writeStdout(fmt.Sprintf(">>> Eclass %s loaded dynamically\n", name))
			}
			return nil
		}

		// Dynamic loading failed - try fallback
		if h.verbose {
			h.writeStderr(fmt.Sprintf(">>> Dynamic loading failed for %s: %v\n", name, err))
		}
	}

	// Try Go fallback
	if impl, ok := h.goFallbacks[name]; ok {
		if h.verbose {
			h.writeStdout(fmt.Sprintf(">>> Using Go fallback for eclass %s\n", name))
		}
		env := h.executor.GetEnv()
		if err := impl.Execute(ctx, env); err != nil {
			return fmt.Errorf("go fallback for %s: %w", name, err)
		}
		// Update executor env from Go impl
		for k, v := range env {
			h.executor.SetVar(k, v)
		}
		return nil
	}

	// Both failed
	return &EclassNotFoundError{Name: name, Locations: h.cache.Locations()}
}

// GetExecutor returns the underlying executor.
func (h *HybridLoader) GetExecutor() *Executor {
	return h.executor
}

// GetCache returns the eclass cache.
func (h *HybridLoader) GetCache() *Cache {
	return h.cache
}

// HasGoFallback checks if a Go fallback exists for an eclass.
func (h *HybridLoader) HasGoFallback(name string) bool {
	_, ok := h.goFallbacks[name]
	return ok
}

func (h *HybridLoader) writeStdout(s string) {
	if h.stdout != nil {
		_, _ = io.WriteString(h.stdout, s)
	}
}

func (h *HybridLoader) writeStderr(s string) {
	if h.stderr != nil {
		_, _ = io.WriteString(h.stderr, s)
	}
}

// BuildCacheFromRepos builds an eclass cache from repository paths.
//
// Parameters:
//   - repos: List of repository root paths (e.g., /var/db/repos/gentoo)
//   - masters: Optional map of repo name to its master repos
//
// The first repo is treated as the master (lowest priority for deduplication).
func BuildCacheFromRepos(repos []string, masters map[string][]string) (*Cache, error) {
	cache := NewCache()

	for i, repoPath := range repos {
		eclassDir := filepath.Join(repoPath, "eclass")
		repoName := filepath.Base(repoPath)

		// Check if directory exists
		if _, err := os.Stat(eclassDir); os.IsNotExist(err) {
			continue
		}

		// First repo is master
		if i == 0 {
			cache.masterEclassDir = eclassDir
		}

		cache.repoNames[eclassDir] = repoName

		// Add masters for this repo if specified
		if m, ok := masters[repoName]; ok {
			for _, masterName := range m {
				for _, r := range repos {
					if filepath.Base(r) == masterName {
						masterEclass := filepath.Join(r, "eclass")
						if !cache.hasLocation(masterEclass) {
							cache.locations = append(cache.locations, masterEclass)
							if err := cache.scanDirectoryLocked(masterEclass); err != nil {
								continue
							}
						}
						break
					}
				}
			}
		}

		// Add this repo
		if !cache.hasLocation(eclassDir) {
			cache.locations = append(cache.locations, eclassDir)
			if err := cache.scanDirectoryLocked(eclassDir); err != nil {
				return nil, err
			}
		}
	}

	return cache, nil
}

// DefaultLocations returns the default eclass search locations.
func DefaultLocations() []string {
	return []string{
		"/var/db/repos/gentoo/eclass",
		"/usr/portage/eclass",
	}
}

// FindRepositoryEclass finds an eclass in repository eclass directories.
//
// This is a utility for locating eclass files when you have repository
// paths but not a full cache.
func FindRepositoryEclass(name string, repoPaths []string) (string, error) {
	eclassFile := name + ".eclass"

	for _, repoPath := range repoPaths {
		path := filepath.Join(repoPath, "eclass", eclassFile)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", &EclassNotFoundError{Name: name, Locations: repoPaths}
}
