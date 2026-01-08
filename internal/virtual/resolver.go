package virtual

import (
	"fmt"
	"strings"
	"sync"
)

// Resolver handles virtual package resolution.
//
// It selects the best provider for a virtual package based on:
// 1. Currently installed provider (prefer consistency)
// 2. User-configured default provider preference
// 3. First available provider (fallback)
//
// Thread-safe: all operations can be called concurrently.
type Resolver struct {
	// defaults maps virtual package names to preferred providers
	// e.g., "virtual/jdk" -> "dev-java/openjdk"
	defaults map[string]string

	// installed maps virtual package names to currently installed providers
	// e.g., "virtual/jdk" -> "dev-java/openjdk-17"
	installed map[string]string

	// mu protects concurrent access
	mu sync.RWMutex
}

// NewResolver creates a new virtual package resolver.
func NewResolver() *Resolver {
	return &Resolver{
		defaults:  make(map[string]string),
		installed: make(map[string]string),
	}
}

// SetDefault sets the preferred provider for a virtual package.
//
// The provider can be a partial match - e.g., "dev-java/openjdk" will
// match "dev-java/openjdk:17" or "dev-java/openjdk-bin".
//
// Example:
//
//	r.SetDefault("virtual/jdk", "dev-java/openjdk")
func (r *Resolver) SetDefault(virtual, provider string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.defaults[virtual] = provider
}

// GetDefault returns the preferred provider for a virtual, if set.
func (r *Resolver) GetDefault(virtual string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.defaults[virtual]
	return provider, ok
}

// SetInstalled marks a provider as currently installed for a virtual.
//
// This is used to prefer the currently installed provider when
// re-resolving dependencies.
//
// Example:
//
//	r.SetInstalled("virtual/jdk", "dev-java/openjdk-17.0.2")
func (r *Resolver) SetInstalled(virtual, provider string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.installed[virtual] = provider
}

// GetInstalled returns the installed provider for a virtual, if any.
func (r *Resolver) GetInstalled(virtual string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.installed[virtual]
	return provider, ok
}

// SelectProvider chooses the best provider for a virtual package.
//
// Selection priority:
// 1. Currently installed provider (if in available list)
// 2. User-configured default provider (prefix match)
// 3. First available provider
//
// Returns an error if no providers are available.
//
// Example:
//
//	available := []string{"dev-java/openjdk:17", "dev-java/oracle-jdk-bin:17"}
//	provider, err := r.SelectProvider("virtual/jdk", available)
func (r *Resolver) SelectProvider(virtual string, available []string) (string, error) {
	if len(available) == 0 {
		return "", fmt.Errorf("no providers available for virtual package %s", virtual)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. Check installed provider first (prefer consistency)
	if installed, ok := r.installed[virtual]; ok {
		for _, p := range available {
			if r.matchProvider(p, installed) {
				return p, nil
			}
		}
	}

	// 2. Check user preference
	if preferred, ok := r.defaults[virtual]; ok {
		for _, p := range available {
			if r.matchProvider(p, preferred) {
				return p, nil
			}
		}
	}

	// 3. Return first available
	return available[0], nil
}

// matchProvider checks if a provider matches the pattern.
//
// Supports:
// - Exact match: "dev-java/openjdk:17" == "dev-java/openjdk:17"
// - Prefix match: "dev-java/openjdk:17" matches "dev-java/openjdk"
// - Name-only match: "dev-java/openjdk:17" matches "openjdk"
//
// Slot variations are handled by stripping slots before comparison.
func (r *Resolver) matchProvider(provider, pattern string) bool {
	// Exact match
	if provider == pattern {
		return true
	}

	// Strip slots for comparison
	providerBase := StripSlot(provider)
	patternBase := StripSlot(pattern)

	// Base name exact match
	if providerBase == patternBase {
		return true
	}

	// Prefix match (pattern is prefix of provider)
	if strings.HasPrefix(providerBase, patternBase) {
		return true
	}

	// Name-only match (category/pkg matches pkg)
	providerName := ExtractPackageName(providerBase)
	patternName := ExtractPackageName(patternBase)

	return providerName == patternName
}

// ResolveAll resolves all virtuals in a dependency list.
//
// For each virtual package, selects the best provider and returns
// a mapping from virtual to selected provider.
//
// virtuals maps virtual package names to their available providers.
//
// Returns map[virtual]selectedProvider and any error.
func (r *Resolver) ResolveAll(virtuals map[string][]string) (map[string]string, error) {
	result := make(map[string]string, len(virtuals))

	for virtual, providers := range virtuals {
		selected, err := r.SelectProvider(virtual, providers)
		if err != nil {
			return nil, fmt.Errorf("resolving %s: %w", virtual, err)
		}
		result[virtual] = selected
	}

	return result, nil
}

// Clear removes all configured defaults and installed providers.
func (r *Resolver) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.defaults = make(map[string]string)
	r.installed = make(map[string]string)
}

// Stats returns resolver statistics.
func (r *Resolver) Stats() ResolverStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return ResolverStats{
		DefaultCount:   len(r.defaults),
		InstalledCount: len(r.installed),
	}
}

// ResolverStats contains resolver statistics.
type ResolverStats struct {
	// DefaultCount is the number of configured default providers.
	DefaultCount int

	// InstalledCount is the number of known installed providers.
	InstalledCount int
}
