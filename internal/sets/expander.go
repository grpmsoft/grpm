// Package sets implements Portage package sets.
//
// This file implements the SetExpander service for unified set expansion
// across all CLI commands.
package sets

import (
	"fmt"
	"strings"

	"github.com/grpmsoft/grpm/internal/config"
	"github.com/grpmsoft/grpm/internal/profile"
)

// SetExpander expands package set references (@world, @selected, @system)
// to their constituent package atoms.
//
// This provides a unified interface for all CLI commands that accept
// package arguments. Instead of each command implementing its own set
// expansion logic, they can use SetExpander to consistently handle sets.
//
// Example:
//
//	expander := sets.NewExpander("/", nil, nil)
//	atoms, err := expander.Expand([]string{"@world", "app-misc/hello"})
//	// Returns: ["sys-apps/portage", "app-misc/hello", ...]
type SetExpander struct {
	registry *Registry
	rootDir  string
}

// ExpanderConfig holds configuration for the SetExpander.
type ExpanderConfig struct {
	// RootDir is the system root directory (usually "/").
	RootDir string

	// PortageDir is the Portage state directory (usually "/var/lib/portage").
	// Used for finding the world file.
	PortageDir string

	// ProfilePath is the path to the active profile.
	// Used for finding system packages.
	ProfilePath string

	// Config is the Portage configuration (make.conf).
	// Optional - used for loading additional settings.
	Config *config.Config
}

// DefaultExpanderConfig returns a default expander configuration.
func DefaultExpanderConfig() *ExpanderConfig {
	return &ExpanderConfig{
		RootDir:     "/",
		PortageDir:  "/var/lib/portage",
		ProfilePath: "/etc/portage/make.profile",
	}
}

// NewExpander creates a new SetExpander with the given configuration.
//
// Parameters:
//   - rootDir: System root directory (usually "/")
//   - prof: Loaded profile (can be nil, will be loaded from profilePath)
//   - cfg: Portage configuration (can be nil)
//
// Example:
//
//	expander := sets.NewExpander("/", nil, nil)
func NewExpander(rootDir string, prof *profile.Profile, cfg *config.Config) *SetExpander {
	// Load profile if not provided
	if prof == nil {
		var err error
		prof, err = profile.LoadProfile("/etc/portage/make.profile")
		if err != nil {
			// Profile loading is optional - sets will work without it
			// @system will just be empty
			prof = nil
		}
	}

	// Create registry with profile
	registry := NewRegistry(rootDir, prof)

	return &SetExpander{
		registry: registry,
		rootDir:  rootDir,
	}
}

// NewExpanderWithConfig creates a new SetExpander with detailed configuration.
func NewExpanderWithConfig(cfg *ExpanderConfig) *SetExpander {
	if cfg == nil {
		cfg = DefaultExpanderConfig()
	}

	// Load profile if path is provided
	var prof *profile.Profile
	if cfg.ProfilePath != "" {
		var err error
		prof, err = profile.LoadProfile(cfg.ProfilePath)
		if err != nil {
			// Profile loading is optional
			prof = nil
		}
	}

	// Create registry
	registry := NewRegistry(cfg.RootDir, prof)

	return &SetExpander{
		registry: registry,
		rootDir:  cfg.RootDir,
	}
}

// Expand expands package arguments, converting set references to atoms.
//
// Arguments can be:
//   - Package atoms (e.g., "app-misc/hello", ">=sys-libs/zlib-1.2")
//   - Set references (e.g., "@world", "@selected", "@system")
//
// Set references are expanded to their constituent package atoms.
// Regular package atoms are passed through unchanged.
//
// Returns an error if a set reference is unknown or cannot be expanded.
//
// Example:
//
//	atoms, err := expander.Expand([]string{"@world", "app-misc/hello"})
//	// Returns: ["sys-apps/portage", "sys-libs/glibc", "app-misc/hello", ...]
func (e *SetExpander) Expand(args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, nil
	}

	var result []string

	for _, arg := range args {
		if IsSetReference(arg) {
			// Expand set to atoms
			atoms, err := e.expandSet(arg)
			if err != nil {
				return nil, fmt.Errorf("failed to expand set %s: %w", arg, err)
			}
			result = append(result, atoms...)
		} else {
			// Pass through regular package atoms
			result = append(result, arg)
		}
	}

	// Deduplicate results while preserving order
	return deduplicateArgs(result), nil
}

// ExpandSet expands a single set reference to package atoms.
//
// The setName can be with or without the @ prefix:
//   - "@world" or "world" -> expands @world
//   - "@selected" or "selected" -> expands @selected
//   - "@system" or "system" -> expands @system
//
// Returns an error if the set is unknown.
func (e *SetExpander) ExpandSet(setName string) ([]string, error) {
	return e.expandSet(setName)
}

// expandSet expands a set reference to package atoms.
func (e *SetExpander) expandSet(setName string) ([]string, error) {
	// Normalize set name (ensure @ prefix)
	normalizedName := setName
	if !strings.HasPrefix(normalizedName, "@") {
		normalizedName = "@" + normalizedName
	}

	// Get set from registry
	set, err := e.registry.GetSet(normalizedName)
	if err != nil {
		return nil, err
	}

	// Get packages from set
	atoms, err := set.Packages()
	if err != nil {
		return nil, fmt.Errorf("failed to get packages from %s: %w", normalizedName, err)
	}

	// Convert atoms to strings
	result := make([]string, 0, len(atoms))
	for _, atom := range atoms {
		// Use category/package format (CP)
		atomStr := atom.Category + "/" + atom.Package
		result = append(result, atomStr)
	}

	return result, nil
}

// HasSets checks if any argument is a set reference.
//
// This is useful for commands that may want to handle sets differently
// or display additional information when sets are involved.
func (e *SetExpander) HasSets(args []string) bool {
	for _, arg := range args {
		if IsSetReference(arg) {
			return true
		}
	}
	return false
}

// ListSets returns all known set names.
func (e *SetExpander) ListSets() []string {
	return e.registry.ListSets()
}

// GetRegistry returns the underlying set registry.
// This allows advanced use cases that need direct registry access.
func (e *SetExpander) GetRegistry() *Registry {
	return e.registry
}

// deduplicateArgs removes duplicate strings while preserving order.
func deduplicateArgs(args []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(args))

	for _, arg := range args {
		if !seen[arg] {
			seen[arg] = true
			result = append(result, arg)
		}
	}

	return result
}
