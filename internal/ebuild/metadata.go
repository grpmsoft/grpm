// Package ebuild implements ebuild execution engine.
//
// This file provides metadata extraction from ebuilds via bash interpreter
// execution with eclass support. This enables proper SRC_URI evaluation
// for packages that generate distfile URLs dynamically (e.g., toolchain.eclass).
package ebuild

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/grpmsoft/grpm/internal/eclass"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// MetadataEvaluator evaluates ebuild metadata by sourcing the ebuild
// with eclass support via the mvdan.cc/sh bash interpreter.
//
// This is necessary for packages that dynamically generate SRC_URI
// via eclass functions (e.g., toolchain.eclass's get_gcc_src_uri()).
type MetadataEvaluator struct {
	// RepoPath is the path to the Portage repository.
	RepoPath string

	// EclassCache is the eclass file cache.
	EclassCache *eclass.Cache

	// Verbose enables debug output.
	Verbose bool
}

// NewMetadataEvaluator creates a new metadata evaluator.
//
// Parameters:
//   - repoPath: Path to the Portage repository (e.g., /var/db/repos/gentoo)
func NewMetadataEvaluator(repoPath string) (*MetadataEvaluator, error) {
	// Create eclass cache from repository
	eclassDir := filepath.Join(repoPath, "eclass")
	cache, err := eclass.NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		return nil, fmt.Errorf("creating eclass cache: %w", err)
	}

	return &MetadataEvaluator{
		RepoPath:    repoPath,
		EclassCache: cache,
	}, nil
}

// EvaluateSrcURI sources an ebuild with eclasses and returns the evaluated SRC_URI.
//
// This function:
//  1. Creates a minimal environment with package variables (P, PV, PN, etc.)
//  2. Sets up the bash interpreter with dynamic eclass loading
//  3. Sources the ebuild (which will inherit eclasses and define variables)
//  4. Extracts the SRC_URI variable from the environment after sourcing
//
// For packages like gcc that use toolchain.eclass, this enables proper
// evaluation of dynamic SRC_URI generation functions like get_gcc_src_uri().
//
// Parameters:
//   - ebuildPath: Path to the ebuild file
//   - pkgInfo: Package metadata for variable expansion
//
// Returns the evaluated SRC_URI string, or error if sourcing fails.
func (m *MetadataEvaluator) EvaluateSrcURI(ctx context.Context, ebuildPath string, pkgInfo *pkg.Package) (string, error) {
	if ebuildPath == "" {
		return "", fmt.Errorf("ebuild path is empty")
	}

	// Read ebuild content
	content, err := os.ReadFile(ebuildPath)
	if err != nil {
		return "", fmt.Errorf("reading ebuild %s: %w", ebuildPath, err)
	}

	// Create minimal environment for ebuild sourcing
	env, err := m.createMetadataEnvironment(pkgInfo, ebuildPath)
	if err != nil {
		return "", fmt.Errorf("creating environment: %w", err)
	}

	// Create interpreter with output capture (suppress to avoid noise)
	var stdout, stderr bytes.Buffer
	interp := NewInterpreter(env, &stdout, &stderr)

	// Setup dynamic eclass loading
	if m.EclassCache != nil {
		_, err := SetupDynamicEclassLoading(interp, m.EclassCache)
		if err != nil {
			// Non-fatal: continue without dynamic loading
			if m.Verbose {
				fmt.Fprintf(os.Stderr, "Warning: dynamic eclass loading unavailable: %v\n", err)
			}
		}
	}

	// Build script that sources ebuild and outputs SRC_URI
	// We need to source the ebuild to execute inherit() and any
	// variable assignments that depend on eclass functions.
	script := buildMetadataExtractionScript(string(content))

	// Execute the script
	if err := interp.Run(ctx, script); err != nil {
		return "", fmt.Errorf("sourcing ebuild: %w", err)
	}

	// Extract SRC_URI from stdout (the script echoes it)
	srcURI := strings.TrimSpace(stdout.String())

	// If empty, try to get from environment directly via a second pass
	if srcURI == "" {
		srcURI = extractSrcURIFromContent(string(content), env.ToMap())
	}

	return srcURI, nil
}

// createMetadataEnvironment creates a minimal environment for metadata extraction.
//
// This sets up the standard Portage package variables (P, PN, PV, etc.)
// without creating build directories, since we only need metadata.
func (m *MetadataEvaluator) createMetadataEnvironment(pkgInfo *pkg.Package, ebuildPath string) (*Environment, error) {
	if pkgInfo == nil {
		return nil, fmt.Errorf("package info is nil")
	}

	// Parse package name: "category/package-name"
	parts := strings.Split(pkgInfo.Name, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid package name format: %s", pkgInfo.Name)
	}

	category := parts[0]
	packageName := parts[1]

	// Extract revision from version
	version := pkgInfo.Version
	revision := "r0"
	if idx := strings.LastIndex(version, "-r"); idx != -1 {
		revision = version[idx+1:]
		version = version[:idx]
	}

	// Build variables
	pVar := fmt.Sprintf("%s-%s", packageName, version)
	pvr := computePVR(version, revision)
	pf := fmt.Sprintf("%s-%s", packageName, pvr)

	// Create a minimal environment (no directory creation needed)
	env := &Environment{
		Package:  pkgInfo,
		P:        pVar,
		PN:       packageName,
		PV:       version,
		PR:       revision,
		PVR:      pvr,
		PF:       pf,
		CATEGORY: category,

		PORTDIR:  m.RepoPath,
		DISTDIR:  "/var/cache/distfiles",
		FILESDIR: filepath.Join(m.RepoPath, category, packageName, "files"),
		EBUILD:   ebuildPath,

		// Temporary directories (not created, just set for variable expansion)
		PORTAGE_TMPDIR: "/var/tmp/portage",
		WORKDIR:        fmt.Sprintf("/var/tmp/portage/%s/%s/work", category, pf),
		T:              fmt.Sprintf("/var/tmp/portage/%s/%s/temp", category, pf),
		HOME:           fmt.Sprintf("/var/tmp/portage/%s/%s/temp/homedir", category, pf),

		// Default S (will be recalculated if ebuild overrides)
		S: fmt.Sprintf("/var/tmp/portage/%s/%s/work/%s", category, pf, pVar),

		// Image directory
		D: fmt.Sprintf("/var/tmp/portage/%s/%s/image", category, pf),

		// Root settings
		ROOT:    "/",
		EROOT:   "/",
		EPREFIX: "",

		// Build flags from environment
		CFLAGS:   os.Getenv("CFLAGS"),
		CXXFLAGS: os.Getenv("CXXFLAGS"),
		LDFLAGS:  os.Getenv("LDFLAGS"),
		MAKEOPTS: os.Getenv("MAKEOPTS"),

		// Default EAPI
		EAPI: "8",
		SLOT: pkgInfo.Slot.String(),

		// USE flags
		USE: buildUSEString(pkgInfo.UseFlags),
	}

	return env, nil
}

// buildUSEString creates a space-separated USE flag string.
func buildUSEString(flags map[string]bool) string {
	var enabled []string
	for flag, on := range flags {
		if on {
			enabled = append(enabled, flag)
		}
	}
	return strings.Join(enabled, " ")
}

// buildMetadataExtractionScript creates a bash script that sources
// an ebuild and extracts the SRC_URI variable.
//
// The script:
//  1. Defines stub functions for phase functions (so they don't error)
//  2. Sources the ebuild content (which will call inherit)
//  3. Echoes the final SRC_URI value
func buildMetadataExtractionScript(ebuildContent string) string {
	var script strings.Builder

	// Header
	script.WriteString("#!/bin/bash\n\n")

	// Define stub phase functions (they're called during source but we don't need them)
	phaseFuncs := []string{
		"pkg_pretend", "pkg_setup", "pkg_nofetch",
		"src_unpack", "src_prepare", "src_configure",
		"src_compile", "src_test", "src_install",
		"pkg_preinst", "pkg_postinst", "pkg_prerm", "pkg_postrm",
		"pkg_config", "pkg_info",
	}
	for _, fn := range phaseFuncs {
		script.WriteString(fmt.Sprintf("%s() { :; }\n", fn))
	}
	script.WriteString("\n")

	// Source the ebuild content inline
	// This ensures inherit() calls are executed and SRC_URI is evaluated
	script.WriteString("# --- BEGIN EBUILD ---\n")
	script.WriteString(ebuildContent)
	script.WriteString("\n# --- END EBUILD ---\n\n")

	// Output the evaluated SRC_URI
	script.WriteString("echo \"$SRC_URI\"\n")

	return script.String()
}

// extractSrcURIFromContent extracts SRC_URI using simple variable expansion.
// This is a fallback when the interpreter fails to capture the value.
func extractSrcURIFromContent(content string, vars map[string]string) string {
	// Find SRC_URI assignment
	srcURI := extractEbuildVariable(content, "SRC_URI")
	if srcURI == "" {
		return ""
	}

	// Expand variables
	return expandVariables(srcURI, vars)
}

// EvaluateSrcURI is a convenience function for evaluating SRC_URI.
//
// Parameters:
//   - ebuildPath: Path to the ebuild file
//   - repoPath: Path to the Portage repository
//   - pkgInfo: Package metadata
//
// Returns the evaluated SRC_URI string.
func EvaluateSrcURI(ctx context.Context, ebuildPath, repoPath string, pkgInfo *pkg.Package) (string, error) {
	evaluator, err := NewMetadataEvaluator(repoPath)
	if err != nil {
		return "", err
	}
	return evaluator.EvaluateSrcURI(ctx, ebuildPath, pkgInfo)
}

// ExtractEbuildMetadata sources an ebuild and extracts multiple metadata variables.
//
// This is useful when you need more than just SRC_URI (e.g., DEPEND, RDEPEND, IUSE).
//
// Parameters:
//   - ebuildPath: Path to the ebuild file
//   - pkgInfo: Package metadata
//   - varNames: List of variable names to extract
//
// Returns a map of variable name to value.
func (m *MetadataEvaluator) ExtractEbuildMetadata(
	ctx context.Context,
	ebuildPath string,
	pkgInfo *pkg.Package,
	varNames []string,
) (map[string]string, error) {
	if ebuildPath == "" {
		return nil, fmt.Errorf("ebuild path is empty")
	}

	// Read ebuild content
	content, err := os.ReadFile(ebuildPath)
	if err != nil {
		return nil, fmt.Errorf("reading ebuild %s: %w", ebuildPath, err)
	}

	// Create environment
	env, err := m.createMetadataEnvironment(pkgInfo, ebuildPath)
	if err != nil {
		return nil, fmt.Errorf("creating environment: %w", err)
	}

	// Create interpreter
	var stdout bytes.Buffer
	interp := NewInterpreter(env, &stdout, io.Discard)

	// Setup dynamic eclass loading
	if m.EclassCache != nil {
		_, _ = SetupDynamicEclassLoading(interp, m.EclassCache)
	}

	// Build script that sources ebuild and outputs requested variables
	script := buildMultiVarExtractionScript(string(content), varNames)

	// Execute
	if err := interp.Run(ctx, script); err != nil {
		return nil, fmt.Errorf("sourcing ebuild: %w", err)
	}

	// Parse output (format: "VAR=value\n")
	result := make(map[string]string)
	lines := strings.Split(stdout.String(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, "="); idx > 0 {
			name := line[:idx]
			value := line[idx+1:]
			result[name] = value
		}
	}

	return result, nil
}

// buildMultiVarExtractionScript creates a script that extracts multiple variables.
func buildMultiVarExtractionScript(ebuildContent string, varNames []string) string {
	var script strings.Builder

	script.WriteString("#!/bin/bash\n\n")

	// Stub phase functions
	phaseFuncs := []string{
		"pkg_pretend", "pkg_setup", "pkg_nofetch",
		"src_unpack", "src_prepare", "src_configure",
		"src_compile", "src_test", "src_install",
		"pkg_preinst", "pkg_postinst", "pkg_prerm", "pkg_postrm",
		"pkg_config", "pkg_info",
	}
	for _, fn := range phaseFuncs {
		script.WriteString(fmt.Sprintf("%s() { :; }\n", fn))
	}
	script.WriteString("\n")

	// Source ebuild
	script.WriteString("# --- BEGIN EBUILD ---\n")
	script.WriteString(ebuildContent)
	script.WriteString("\n# --- END EBUILD ---\n\n")

	// Output each variable
	for _, name := range varNames {
		script.WriteString(fmt.Sprintf("echo \"%s=$%s\"\n", name, name))
	}

	return script.String()
}
