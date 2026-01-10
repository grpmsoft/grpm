// Package eclass provides dynamic eclass loading and execution for GRPM.
//
// This package replaces hardcoded Go eclass implementations with dynamic
// loading from repository eclass/ directories. This approach ensures:
//
//   - Gentoo eclass updates propagate to GRPM automatically
//   - Custom overlay eclasses work correctly
//   - Different repositories can have different eclass versions
//
// # Architecture
//
// The package consists of three main components:
//
//   - Cache: Discovers and caches eclass files from repository directories
//   - Executor: Executes eclasses via mvdan.cc/sh bash interpreter
//   - HybridLoader: Provides fallback to Go implementations when needed
//
// # Usage
//
// Basic usage for dynamic eclass loading:
//
//	// Create cache from repository paths
//	cache, err := eclass.NewCacheWithLocations([]string{
//	    "/var/db/repos/gentoo/eclass",
//	    "/var/db/repos/guru/eclass",
//	})
//
//	// Create executor
//	exec := eclass.NewExecutor(cache,
//	    eclass.WithOutput(os.Stdout, os.Stderr),
//	)
//
//	// Inherit eclasses
//	ctx := context.Background()
//	if err := exec.Inherit(ctx, []string{"cmake", "multilib-minimal"}); err != nil {
//	    log.Fatalf("inherit failed: %v", err)
//	}
//
//	// Get accumulated metadata
//	metadata := exec.GetAccumulatedMetadata()
//	fmt.Println("DEPEND:", metadata["DEPEND"])
//
// # Hybrid Loading
//
// For production use, the HybridLoader provides fallback to Go implementations:
//
//	loader := eclass.NewHybridLoader(cache, execHandler,
//	    eclass.WithGoFallback(cmakeImpl),
//	    eclass.WithVerbose(true),
//	)
//
//	if err := loader.Inherit(ctx, []string{"cmake"}); err != nil {
//	    // Falls back to Go implementation if dynamic loading fails
//	}
//
// # Integration with ebuild Package
//
// Use the bridge in internal/ebuild to integrate with existing infrastructure:
//
//	cache, _ := ebuild.CreateDefaultEclassCache()
//	loader, err := ebuild.SetupDynamicEclassLoading(interp, cache)
//
// # Portage Compatibility
//
// This implementation follows Portage's eclass loading semantics:
//
//   - Eclass directories are searched in priority order (masters -> repo -> overlays)
//   - Metadata variables (DEPEND, IUSE, etc.) are accumulated across inherits
//   - EXPORT_FUNCTIONS declarations are supported
//   - Already-inherited eclasses are skipped
//
// Reference: Portage lib/portage/eclass_cache.py and bin/ebuild.sh
package eclass
