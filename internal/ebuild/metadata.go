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
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/grpmsoft/grpm/internal/eclass"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// EvalMode specifies the bash evaluation backend.
type EvalMode int

const (
	// EvalModeGo uses pure Go mvdan.cc/sh interpreter (default).
	// Cross-platform, no external dependencies.
	// Limitation: Some advanced bash features not supported.
	EvalModeGo EvalMode = iota

	// EvalModeNativeBash uses system's /bin/bash.
	// Full bash compatibility, but requires bash installed.
	// Recommended for complex eclasses on Gentoo systems.
	EvalModeNativeBash
)

// MetadataEvaluator evaluates ebuild metadata by sourcing the ebuild
// with eclass support.
//
// Supports two evaluation modes:
//   - EvalModeGo (default): Pure Go via mvdan.cc/sh - cross-platform
//   - EvalModeNativeBash: System bash - full compatibility
//
// This is necessary for packages that dynamically generate SRC_URI
// via eclass functions (e.g., toolchain.eclass's get_gcc_src_uri()).
type MetadataEvaluator struct {
	// RepoPath is the path to the Portage repository.
	RepoPath string

	// EclassCache is the eclass file cache.
	EclassCache *eclass.Cache

	// Mode specifies evaluation backend (default: EvalModeGo).
	Mode EvalMode

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

	// Dispatch to appropriate backend
	switch m.Mode {
	case EvalModeNativeBash:
		return m.evaluateWithNativeBash(ctx, string(content), env)
	default: // EvalModeGo
		return m.evaluateWithGo(ctx, string(content), env)
	}
}

// evaluateWithGo uses pure Go mvdan.cc/sh interpreter.
// This is the default, cross-platform approach.
func (m *MetadataEvaluator) evaluateWithGo(ctx context.Context, ebuildContent string, env *Environment) (string, error) {
	var stdout, stderr bytes.Buffer
	interp := NewInterpreter(env, &stdout, &stderr)

	// Setup dynamic eclass loading
	if m.EclassCache != nil {
		_, err := SetupDynamicEclassLoading(interp, m.EclassCache)
		if err != nil && m.Verbose {
			fmt.Fprintf(os.Stderr, "Warning: dynamic eclass loading unavailable: %v\n", err)
		}
	}

	// Build script with eclass infrastructure
	script := buildMetadataExtractionScript(ebuildContent)

	// Execute the script
	if err := interp.Run(ctx, script); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("sourcing ebuild: %w\nstderr: %s", err, stderr.String())
		}
		return "", fmt.Errorf("sourcing ebuild: %w", err)
	}

	// Extract SRC_URI from stdout
	srcURI := strings.TrimSpace(stdout.String())

	// If empty, try simple variable extraction as fallback
	if srcURI == "" {
		srcURI = extractSrcURIFromContent(ebuildContent, env.ToMap())
	}

	return srcURI, nil
}

// evaluateWithNativeBash uses the system's bash to evaluate the ebuild.
// This is the most reliable method for complex eclasses that use advanced bash features.
func (m *MetadataEvaluator) evaluateWithNativeBash(ctx context.Context, ebuildContent string, env *Environment) (string, error) {
	// Check if bash is available
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		return "", fmt.Errorf("bash not found: %w", err)
	}

	// Build the extraction script (minimal, bash-native)
	script := buildNativeBashScript(ebuildContent)

	// Create the command
	cmd := exec.CommandContext(ctx, bashPath, "-c", script)

	// Set up environment variables
	envMap := env.ToMap()
	envSlice := make([]string, 0, len(envMap)+10)
	for k, v := range envMap {
		envSlice = append(envSlice, k+"="+v)
	}
	// Add PORTDIR explicitly for inherit() to find eclasses
	envSlice = append(envSlice, "PORTDIR="+m.RepoPath)
	cmd.Env = append(os.Environ(), envSlice...)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the script
	if err := cmd.Run(); err != nil {
		// Include stderr in error message for debugging
		errMsg := err.Error()
		if stderr.Len() > 0 {
			errMsg += "\nstderr: " + stderr.String()
		}
		return "", fmt.Errorf("%s", errMsg)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// buildNativeBashScript creates a minimal script for native bash execution.
// This script is simpler because real bash handles all features natively.
func buildNativeBashScript(ebuildContent string) string {
	var script strings.Builder

	script.WriteString("#!/bin/bash\n")
	script.WriteString("set -e\n\n")

	// Define minimal stubs for phase functions (not needed for metadata)
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

	// Define inherit function that uses bash's source
	script.WriteString(`
# inherit() - Load eclasses
inherit() {
    local eclass
    for eclass in "$@"; do
        local eclass_file="${PORTDIR}/eclass/${eclass}.eclass"
        if [[ -f "${eclass_file}" ]]; then
            local saved_eclass="${ECLASS:-}"
            ECLASS="${eclass}"
            source "${eclass_file}"
            INHERITED="${INHERITED:+${INHERITED} }${eclass}"
            ECLASS="${saved_eclass}"
        fi
    done
}

# EXPORT_FUNCTIONS
EXPORT_FUNCTIONS() {
    local phase
    local current_eclass="${ECLASS}"
    for phase in "$@"; do
        eval "${phase}() { ${current_eclass}_${phase} \"\$@\"; }"
    done
}

# Minimal stubs for functions not needed during metadata extraction
die() { echo "ERROR: $*" >&2; exit 1; }
einfo() { :; }
ewarn() { :; }
eerror() { :; }
ebegin() { :; }
eend() { return 0; }
debug-print() { :; }
debug-print-function() { :; }
debug-print-section() { :; }
eqawarn() { :; }
`)

	script.WriteString("\n# === Source ebuild ===\n")
	script.WriteString(ebuildContent)
	script.WriteString("\n\n# === Output SRC_URI ===\n")
	script.WriteString("echo \"${SRC_URI}\"\n")

	return script.String()
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

		// ExtraVars: Disable unnecessary eclass features for metadata extraction.
		// These prevent eclasses from inheriting problematic dependencies
		// that aren't needed for SRC_URI evaluation.
		ExtraVars: map[string]string{
			// Disable Python-related eclass inheritance in toolchain.eclass
			// (line 30: [[ -n ${TOOLCHAIN_HAS_TESTS} ]] && inherit python-any-r1)
			"TOOLCHAIN_HAS_TESTS": "",

			// Disable git-based live builds in toolchain.eclass
			// (line 50-52: live builds inherit git-r3)
			"TOOLCHAIN_USE_GIT_PATCHES": "",

			// Provide CHOST for tc-arch and similar functions
			"CHOST": "x86_64-pc-linux-gnu",

			// CBUILD for cross-compilation checks
			"CBUILD": "x86_64-pc-linux-gnu",
		},
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
//  1. Defines inherit() as a bash function that uses `. ` to source eclasses
//  2. Defines EXPORT_FUNCTIONS() to create phase wrapper functions
//  3. Defines stub functions for phase functions (so they don't error)
//  4. Sources the ebuild content (which will call inherit)
//  5. Echoes the final SRC_URI value
//
// CRITICAL: inherit() MUST be a bash function that sources eclasses in the same
// shell context. Using a separate Go executor loses function definitions.
func buildMetadataExtractionScript(ebuildContent string) string {
	var script strings.Builder

	// Header
	script.WriteString("#!/bin/bash\n\n")

	// Write the eclass/inherit infrastructure
	script.WriteString(eclassInfrastructure)
	script.WriteString("\n")

	// Define stub phase functions (they're called during source but we don't need them)
	// These will be overwritten by EXPORT_FUNCTIONS from eclasses
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

	// Source the ebuild content inline (with phase function bodies stripped).
	// Phase functions (src_compile, src_test, etc.) often contain advanced bash
	// constructs unsupported by mvdan.cc/sh (e.g., brace expansion in variable
	// names like RUN_{VERY_,}EXPENSIVE_TESTS). Since metadata extraction only
	// needs global variable values (SRC_URI, DEPEND, etc.), stripping function
	// bodies prevents parse errors while preserving all metadata assignments.
	script.WriteString("# --- BEGIN EBUILD ---\n")
	script.WriteString(stripFunctionBodies(ebuildContent))
	script.WriteString("\n# --- END EBUILD ---\n\n")

	// Output the evaluated SRC_URI
	script.WriteString("echo \"$SRC_URI\"\n")

	return script.String()
}

// stripFunctionBodies removes function bodies from ebuild content.
//
// Phase functions (src_compile, src_test, src_install, etc.) often contain
// advanced bash features that mvdan.cc/sh cannot parse (e.g., brace expansion
// in variable names: RUN_{VERY_,}EXPENSIVE_TESTS). Since metadata extraction
// only needs global-scope code (variable assignments, inherit calls), we
// replace function bodies with no-ops.
//
// Handles both formats:
//
//	func_name() {
//	    body
//	}
//
//	func_name() { one-liner; }
func stripFunctionBodies(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	depth := 0
	inFunction := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if !inFunction {
			// Detect function definition: "name() {" or "name () {"
			if isFunctionDefinition(trimmed) {
				// Check if it's a one-liner: "name() { body; }"
				if strings.Count(trimmed, "{") == strings.Count(trimmed, "}") && strings.Contains(trimmed, "}") {
					// One-liner function — replace with stub
					funcName := extractFunctionName(trimmed)
					result = append(result, funcName+"() { :; }")
					continue
				}
				// Multi-line function — replace opening and skip body
				funcName := extractFunctionName(trimmed)
				result = append(result, funcName+"() { :; }")
				depth = strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
				inFunction = true
				continue
			}
			result = append(result, line)
		} else {
			// Inside function body — count braces to find the end
			depth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
			if depth <= 0 {
				inFunction = false
				depth = 0
			}
			// Skip the line (function body)
		}
	}

	return strings.Join(result, "\n")
}

// isFunctionDefinition checks if a line starts a bash function definition.
// Matches: "name() {", "name ()" with optional leading whitespace.
func isFunctionDefinition(trimmed string) bool {
	// Skip comments and empty lines
	if trimmed == "" || trimmed[0] == '#' {
		return false
	}

	// Look for "() {" or "()" pattern
	parenIdx := strings.Index(trimmed, "()")
	if parenIdx <= 0 {
		return false
	}

	// Extract potential function name (before the parentheses)
	name := strings.TrimSpace(trimmed[:parenIdx])

	// Validate function name: must be a valid bash identifier
	// (letters, digits, underscores, hyphens; not starting with digit)
	if name == "" {
		return false
	}
	for i, ch := range name {
		if i == 0 && ch >= '0' && ch <= '9' {
			return false
		}
		isAlpha := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
		isDigit := ch >= '0' && ch <= '9'
		if !isAlpha && !isDigit && ch != '_' && ch != '-' {
			return false
		}
	}

	// Must have opening brace somewhere on this line or be just "name()"
	rest := strings.TrimSpace(trimmed[parenIdx+2:])
	return rest == "" || rest == "{" || strings.HasPrefix(rest, "{")
}

// extractFunctionName extracts the function name from a function definition line.
func extractFunctionName(trimmed string) string {
	parenIdx := strings.Index(trimmed, "()")
	if parenIdx <= 0 {
		return ""
	}
	return strings.TrimSpace(trimmed[:parenIdx])
}

// eclassInfrastructure contains bash functions needed for eclass support.
// These run WITHIN the same bash context, preserving function definitions.
//
// NOTE: This is designed for mvdan.cc/sh which has limitations:
// - No negative array indices (${arr[-1]})
// - No "declare -p" option
// - Limited extglob support
const eclassInfrastructure = `
# === ECLASS INFRASTRUCTURE ===
# These functions implement Portage's eclass system for metadata extraction.
# Designed for compatibility with mvdan.cc/sh interpreter.

# Force bash 4 mode: eclasses check BASH_VERSINFO to decide between
# ${var@a} (bash 5+, unsupported by mvdan.cc/sh) and declare -p (bash 4).
BASH_VERSINFO=(4 4 0 0 release x86_64-pc-linux-gnu)

# INHERITED tracks which eclasses have been loaded
INHERITED=""

# Eclass depth counter (simple integer, avoids array limitations)
__ECLASS_DEPTH=0

# inherit() - Load one or more eclasses by sourcing them directly.
# This MUST use bash's '. ' (source) command to keep functions in the same context.
inherit() {
    local eclass
    for eclass in "$@"; do
        # Skip if already inherited
        if [[ " ${INHERITED} " == *" ${eclass} "* ]]; then
            continue
        fi

        # Find eclass file
        local eclass_file="${PORTDIR}/eclass/${eclass}.eclass"
        if [[ ! -f "${eclass_file}" ]]; then
            echo "ERROR: eclass not found: ${eclass}" >&2
            return 1
        fi

        # Save and set ECLASS
        local saved_eclass="${ECLASS:-}"
        ECLASS="${eclass}"
        __ECLASS_DEPTH=$((__ECLASS_DEPTH + 1))

        # Source the eclass (CRITICAL: keeps functions in same context).
        # Redirect stdout to /dev/null: eclass top-level code may call
        # usev/usex/echo which pollutes the stdout used to capture SRC_URI.
        # Command substitutions ($(...)) capture their own stdout separately.
        . "${eclass_file}" >/dev/null

        # Update INHERITED
        INHERITED="${INHERITED:+${INHERITED} }${eclass}"

        # Restore ECLASS
        __ECLASS_DEPTH=$((__ECLASS_DEPTH - 1))
        ECLASS="${saved_eclass}"
    done
}

# EXPORT_FUNCTIONS - Create wrapper functions for phase functions.
# When an eclass exports a function, it creates a wrapper that calls ${ECLASS}_${phase}.
EXPORT_FUNCTIONS() {
    local phase
    local current_eclass="${ECLASS}"

    if [[ -z "${current_eclass}" ]]; then
        echo "ERROR: EXPORT_FUNCTIONS called without ECLASS set" >&2
        return 1
    fi

    for phase in "$@"; do
        # Create wrapper function: phase() calls ${eclass}_${phase}
        eval "${phase}() { ${current_eclass}_${phase} \"\$@\"; }"
    done
}

# die() - Fatal error handler (non-fatal during metadata extraction)
die() {
    echo "ERROR: $*" >&2
    # Don't exit during metadata extraction - allow script to continue
    return 1
}

# === DEBUG FUNCTIONS ===
debug-print-function() { :; }
debug-print-section() { :; }
debug-print() { :; }

# === MESSAGE FUNCTIONS ===
einfo() { :; }
einfon() { :; }
ewarn() { :; }
eerror() { :; }
ebegin() { :; }
eend() { return 0; }
eqawarn() { :; }

# === UTILITY FUNCTIONS ===

# has - Check if element is in list
has() {
    local needle="$1"
    shift
    local item
    for item in "$@"; do
        [[ "${item}" == "${needle}" ]] && return 0
    done
    return 1
}

# has_version - Check if package is installed (stub: always false)
has_version() { return 1; }

# best_version - Get best installed version (stub: empty)
best_version() { echo ""; }

# usev - Output USE flag value if set
usev() {
    local flag="$1"
    local default="${2:-${flag}}"
    if has "${flag}" ${USE}; then
        echo "${default}"
        return 0
    fi
    return 1
}

# use - Check if USE flag is enabled
use() {
    has "$1" ${USE}
}

# usex - USE flag expansion (EAPI 5+)
usex() {
    local flag="$1"
    local iftrue="${2:-yes}"
    local iffalse="${3:-no}"
    local trueval="${4:-}"
    local falseval="${5:-}"
    if use "${flag}"; then
        echo "${iftrue}${trueval}"
    else
        echo "${iffalse}${falseval}"
    fi
}

# use_with/use_enable - Configure options based on USE flags
use_with() {
    local flag="$1"
    local opt="${2:-${flag}}"
    local val="${3:-}"
    if use "${flag}"; then
        if [[ -n "${val}" ]]; then
            echo "--with-${opt}=${val}"
        else
            echo "--with-${opt}"
        fi
    else
        echo "--without-${opt}"
    fi
}

use_enable() {
    local flag="$1"
    local opt="${2:-${flag}}"
    local val="${3:-}"
    if use "${flag}"; then
        if [[ -n "${val}" ]]; then
            echo "--enable-${opt}=${val}"
        else
            echo "--enable-${opt}"
        fi
    else
        echo "--disable-${opt}"
    fi
}

# in_iuse - Check if flag is in IUSE
in_iuse() {
    has "$1" ${IUSE}
}

# === TOOLCHAIN FUNCTIONS ===
tc-is-cross-compiler() { return 1; }
tc-is-gcc() { return 0; }
tc-is-clang() { return 1; }
tc-getCC() { echo "${CC:-gcc}"; }
tc-getCXX() { echo "${CXX:-g++}"; }
tc-getLD() { echo "${LD:-ld}"; }
tc-getAR() { echo "${AR:-ar}"; }
tc-getNM() { echo "${NM:-nm}"; }
tc-getRANLIB() { echo "${RANLIB:-ranlib}"; }
tc-getOBJCOPY() { echo "${OBJCOPY:-objcopy}"; }
tc-getOBJDUMP() { echo "${OBJDUMP:-objdump}"; }
tc-getSTRIP() { echo "${STRIP:-strip}"; }
tc-getPKG_CONFIG() { echo "${PKG_CONFIG:-pkg-config}"; }
tc-getBUILD_CC() { echo "${BUILD_CC:-${CC:-gcc}}"; }
tc-getBUILD_CXX() { echo "${BUILD_CXX:-${CXX:-g++}}"; }
tc-export() { :; }  # Stub - actual export not needed for metadata

tc-arch() {
    case "${CHOST:-x86_64-pc-linux-gnu}" in
        x86_64-*) echo "amd64" ;;
        i?86-*) echo "x86" ;;
        aarch64-*) echo "arm64" ;;
        arm-*) echo "arm" ;;
        powerpc64le-*|ppc64le-*) echo "ppc64" ;;
        powerpc64-*|ppc64-*) echo "ppc64" ;;
        powerpc-*|ppc-*) echo "ppc" ;;
        riscv64-*) echo "riscv" ;;
        s390x-*) echo "s390" ;;
        *) echo "amd64" ;;
    esac
}

tc-endian() {
    case "$(tc-arch)" in
        ppc64|s390) echo "big" ;;
        *) echo "little" ;;
    esac
}

# === MULTILIB FUNCTIONS ===
get_libdir() { echo "lib64"; }
multilib_native_use_with() { use_with "$@"; }
multilib_native_use_enable() { use_enable "$@"; }
multilib_native_usex() { usex "$@"; }

# === PYTHON ECLASS STUBS ===
# These stub out python-r1.eclass, python-single-r1.eclass, python-any-r1.eclass
# since mvdan.cc/sh doesn't support all bash features they use.

# Stub: Pretend python is available
_python_check_PYTHON_COMPAT() { :; }
_python_set_impls() { :; }
python_check_deps() { return 0; }
python_gen_any_dep() { :; }
python_gen_cond_dep() { echo ""; }
python_gen_impl_dep() { echo ""; }
python_gen_useflags() { echo ""; }
python_get_PYTHON() { echo "/usr/bin/python3"; }
python_get_PYTHON_SITEDIR() { echo "/usr/lib/python3/site-packages"; }
python_get_implementation() { echo "cpython"; }
python_get_includedir() { echo "/usr/include/python3"; }
python_get_library_path() { echo "/usr/lib/libpython3.so"; }
python_get_scriptdir() { echo "/usr/bin"; }
python_has_version() { return 0; }
python_is_installed() { return 0; }
python_is_python3() { return 0; }
python_foreach_impl() { :; }
python_domodule() { :; }
python_doheader() { :; }
python_newexe() { :; }
python_newscript() { :; }
python_doscript() { :; }
python_scriptinto() { :; }
python_moduleinto() { :; }
python_optimize() { :; }
python_setup() { :; }
python_fix_shebang() { :; }
python_export() { :; }
python_wrapper_setup() { :; }

# EPYTHON stub
EPYTHON="${EPYTHON:-python3}"
PYTHON="${PYTHON:-/usr/bin/python3}"

# === MISC STUBS ===
check_license() { :; }
optfeature() { :; }
estack_push() { :; }
estack_pop() { return 0; }
eshopts_push() { :; }
eshopts_pop() { :; }
# NOTE: ver_cut, ver_rs, ver_test are NOT defined here - they are
# handled by the Go exec handler which properly writes to stdout
# for command substitution to work.

# === END ECLASS INFRASTRUCTURE ===
`

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

	// Write the eclass/inherit infrastructure
	script.WriteString(eclassInfrastructure)
	script.WriteString("\n")

	// Stub phase functions (will be overwritten by EXPORT_FUNCTIONS from eclasses)
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

	// Source ebuild (with function bodies stripped — same as buildMetadataExtractionScript)
	script.WriteString("# --- BEGIN EBUILD ---\n")
	script.WriteString(stripFunctionBodies(ebuildContent))
	script.WriteString("\n# --- END EBUILD ---\n\n")

	// Output each variable
	for _, name := range varNames {
		script.WriteString(fmt.Sprintf("echo \"%s=$%s\"\n", name, name))
	}

	return script.String()
}
