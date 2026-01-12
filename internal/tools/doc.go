// Package tools provides external tool detection and management for GRPM.
//
// This package implements detection of external tools (compilers, build systems,
// utilities) that ebuilds may require during package building. It provides:
//
//   - Tool registry with metadata (binary name, Gentoo package, description)
//   - Tool availability detection via PATH lookup
//   - Integration with eclass requirements
//   - Clear error messages with installation suggestions
//
// # Architecture
//
// The package follows a registry pattern with three main components:
//
//   - Tool: Value object representing an external tool
//   - Registry: Collection of known tools with lookup methods
//   - Detector: Checks tool availability on the current system
//
// # Usage
//
//	// Create registry with default tools
//	registry := tools.NewDefaultRegistry()
//
//	// Create detector
//	detector := tools.NewDetector(registry)
//
//	// Check if cmake is available
//	if !detector.IsAvailable("cmake") {
//	    tool := registry.Get("cmake")
//	    fmt.Printf("Missing: %s\n", tool.Name)
//	    fmt.Printf("Install: grpm install %s\n", tool.Package)
//	}
//
//	// Check all tools needed for cmake.eclass
//	missing := detector.MissingForEclass("cmake")
//	for _, tool := range missing {
//	    fmt.Printf("- %s (%s)\n", tool.Name, tool.Package)
//	}
//
// # Tool Categories
//
// Tools are organized into categories for easier management:
//
//   - compilers: gcc, clang, rustc, go
//   - build-systems: make, ninja, cmake, meson, autoconf, automake
//   - languages: python, perl, ruby
//   - utilities: pkg-config, git, wget, curl, patch
//   - compression: gzip, bzip2, xz, zstd, tar
//   - documentation: doxygen, sphinx-build, asciidoc
//
// # Integration Points
//
//   - Pre-build check in grpm emerge
//   - grpm tools CLI command
//   - Analyzer integration for coverage reports
package tools
