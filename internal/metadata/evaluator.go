// Package metadata provides ebuild metadata extraction with eclass support.
//
// This package provides lightweight metadata extraction from ebuilds by
// sourcing them with the mvdan.cc/sh bash interpreter and eclass support.
// It is designed to have minimal dependencies to avoid import cycles.
//
// The main use case is extracting DEPEND/RDEPEND/BDEPEND from packages
// that define their dependencies dynamically in eclasses (e.g., gcc via
// toolchain.eclass).
package metadata

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/grpmsoft/grpm/internal/eclass"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// PackageInfo contains the minimal package information needed for metadata extraction.
// This is a copy of essential fields to avoid importing pkg which would create cycles.
type PackageInfo struct {
	Name     string          // category/package (e.g., "sys-devel/gcc")
	Version  string          // version (e.g., "13.4.1_p20250807")
	Slot     string          // slot (e.g., "0")
	UseFlags map[string]bool // USE flags
}

// Evaluator evaluates ebuild metadata by sourcing the ebuild
// with eclass support via the mvdan.cc/sh bash interpreter.
//
// This is necessary for packages that dynamically generate dependencies
// via eclass functions (e.g., toolchain.eclass for gcc).
type Evaluator struct {
	// RepoPath is the path to the Portage repository.
	RepoPath string

	// EclassCache is the eclass file cache.
	EclassCache *eclass.Cache

	// Verbose enables debug output.
	Verbose bool
}

// NewEvaluator creates a new metadata evaluator.
//
// Parameters:
//   - repoPath: Path to the Portage repository (e.g., /var/db/repos/gentoo)
func NewEvaluator(repoPath string) (*Evaluator, error) {
	// Create eclass cache from repository
	eclassDir := filepath.Join(repoPath, "eclass")
	cache, err := eclass.NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		return nil, fmt.Errorf("creating eclass cache: %w", err)
	}

	return &Evaluator{
		RepoPath:    repoPath,
		EclassCache: cache,
	}, nil
}

// ExtractMetadata sources an ebuild and extracts multiple metadata variables.
//
// This is useful for extracting DEPEND, RDEPEND, BDEPEND, IUSE, etc. from
// packages that define these variables in eclasses.
//
// Parameters:
//   - ctx: Context for cancellation
//   - ebuildPath: Path to the ebuild file
//   - pkgInfo: Package metadata for variable expansion
//   - varNames: List of variable names to extract
//
// Returns a map of variable name to value.
func (e *Evaluator) ExtractMetadata(
	ctx context.Context,
	ebuildPath string,
	pkgInfo *PackageInfo,
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
	env := e.createEnvironment(pkgInfo, ebuildPath)

	// Create interpreter with output capture
	var stdout bytes.Buffer
	runner, err := e.createRunner(ctx, env, &stdout, io.Discard)
	if err != nil {
		return nil, fmt.Errorf("creating interpreter: %w", err)
	}

	// Build script that sources ebuild and outputs requested variables
	script := buildExtractionScript(string(content), varNames)

	// Parse and execute
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	prog, err := parser.Parse(strings.NewReader(script), "metadata")
	if err != nil {
		return nil, fmt.Errorf("parsing script: %w", err)
	}

	if err := runner.Run(ctx, prog); err != nil {
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

// createEnvironment creates bash environment variables for ebuild sourcing.
func (e *Evaluator) createEnvironment(pkgInfo *PackageInfo, ebuildPath string) map[string]string {
	// Parse package name
	parts := strings.Split(pkgInfo.Name, "/")
	category := ""
	packageName := pkgInfo.Name
	if len(parts) == 2 {
		category = parts[0]
		packageName = parts[1]
	}

	// Extract revision from version
	version := pkgInfo.Version
	revision := "r0"
	if idx := strings.LastIndex(version, "-r"); idx != -1 {
		revision = version[idx+1:]
		version = version[:idx]
	}

	// Build PVR and PF
	pvr := version
	if revision != "r0" {
		pvr = version + "-" + revision
	}
	pf := packageName + "-" + pvr
	pVar := packageName + "-" + version

	// Build USE string
	var useFlags []string
	for flag, enabled := range pkgInfo.UseFlags {
		if enabled {
			useFlags = append(useFlags, flag)
		}
	}

	env := map[string]string{
		"P":        pVar,
		"PN":       packageName,
		"PV":       version,
		"PR":       revision,
		"PVR":      pvr,
		"PF":       pf,
		"CATEGORY": category,
		"SLOT":     pkgInfo.Slot,
		"USE":      strings.Join(useFlags, " "),

		"PORTDIR":        e.RepoPath,
		"DISTDIR":        "/var/cache/distfiles",
		"FILESDIR":       filepath.Join(e.RepoPath, category, packageName, "files"),
		"EBUILD":         ebuildPath,
		"PORTAGE_TMPDIR": "/var/tmp/portage",
		"WORKDIR":        fmt.Sprintf("/var/tmp/portage/%s/%s/work", category, pf),
		"T":              fmt.Sprintf("/var/tmp/portage/%s/%s/temp", category, pf),
		"HOME":           fmt.Sprintf("/var/tmp/portage/%s/%s/temp/homedir", category, pf),
		"S":              fmt.Sprintf("/var/tmp/portage/%s/%s/work/%s", category, pf, pVar),
		"D":              fmt.Sprintf("/var/tmp/portage/%s/%s/image", category, pf),
		"ROOT":           "/",
		"EROOT":          "/",
		"EPREFIX":        "",
		"EAPI":           "8",

		// ExtraVars: Disable unnecessary eclass features for metadata extraction.
		// These prevent eclasses from inheriting problematic dependencies
		// that aren't needed for DEPEND/RDEPEND evaluation.
		// See: https://github.com/grpmsoft/grpm/issues/50

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
	}

	return env
}

// createRunner creates an mvdan.cc/sh runner with eclass support.
func (e *Evaluator) createRunner(_ context.Context, env map[string]string, stdout, stderr io.Writer) (*interp.Runner, error) {
	// Convert map to slice format for expand.ListEnviron
	var envSlice []string
	for k, v := range env {
		envSlice = append(envSlice, k+"="+v)
	}

	// Create exec handler for inherit() and helper functions
	// ExecHandlers takes a middleware function that wraps the next handler
	execHandler := func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return next(ctx, args)
			}

			cmd := args[0]
			switch cmd {
			case "inherit":
				// Handle eclass inheritance - just pass through (stub)
				// The actual sourcing is done via the script's inherit() function
				return nil
			default:
				// Pass through to next handler
				return next(ctx, args)
			}
		}
	}

	runner, err := interp.New(
		interp.Env(expand.ListEnviron(envSlice...)),
		interp.StdIO(nil, stdout, stderr),
		interp.ExecHandlers(execHandler),
	)
	if err != nil {
		return nil, err
	}

	return runner, nil
}

// buildExtractionScript creates a bash script that sources an ebuild and extracts variables.
func buildExtractionScript(ebuildContent string, varNames []string) string {
	var script strings.Builder

	script.WriteString("#!/bin/bash\n\n")

	// Write the eclass infrastructure
	script.WriteString(eclassInfrastructure)
	script.WriteString("\n")

	// Stub phase functions so they don't error when sourced
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

	// Source the ebuild
	script.WriteString("# --- BEGIN EBUILD ---\n")
	script.WriteString(ebuildContent)
	script.WriteString("\n# --- END EBUILD ---\n\n")

	// Output each variable
	for _, name := range varNames {
		script.WriteString(fmt.Sprintf("echo \"%s=$%s\"\n", name, name))
	}

	return script.String()
}

// eclassInfrastructure contains bash functions needed for eclass support.
// This is a copy of the infrastructure from internal/ebuild/metadata.go
// to avoid import cycles.
const eclassInfrastructure = `
# === ECLASS INFRASTRUCTURE ===
# These functions implement Portage's eclass system for metadata extraction.

# INHERITED tracks which eclasses have been loaded
INHERITED=""

# inherit() - Load one or more eclasses by sourcing them directly.
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
            continue
        fi

        # Save and set ECLASS
        local saved_eclass="${ECLASS:-}"
        ECLASS="${eclass}"

        # Source the eclass
        . "${eclass_file}"

        # Update INHERITED
        INHERITED="${INHERITED:+${INHERITED} }${eclass}"

        # Restore ECLASS
        ECLASS="${saved_eclass}"
    done
}

# EXPORT_FUNCTIONS - Create wrapper functions for phase functions.
EXPORT_FUNCTIONS() {
    local phase
    local current_eclass="${ECLASS}"
    if [[ -z "${current_eclass}" ]]; then
        return 1
    fi
    for phase in "$@"; do
        eval "${phase}() { ${current_eclass}_${phase} \"\$@\"; }"
    done
}

# die() - Fatal error handler (non-fatal during metadata extraction)
die() { return 1; }

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
has() {
    local needle="$1"
    shift
    local item
    for item in "$@"; do
        [[ "${item}" == "${needle}" ]] && return 0
    done
    return 1
}

has_version() { return 1; }
best_version() { echo ""; }

usev() {
    local flag="$1"
    local default="${2:-${flag}}"
    if has "${flag}" ${USE}; then
        echo "${default}"
        return 0
    fi
    return 1
}

use() { has "$1" ${USE}; }

usex() {
    local flag="$1"
    local iftrue="${2:-yes}"
    local iffalse="${3:-no}"
    if use "${flag}"; then
        echo "${iftrue}"
    else
        echo "${iffalse}"
    fi
}

use_with() {
    local flag="$1"
    local opt="${2:-${flag}}"
    if use "${flag}"; then
        echo "--with-${opt}"
    else
        echo "--without-${opt}"
    fi
}

use_enable() {
    local flag="$1"
    local opt="${2:-${flag}}"
    if use "${flag}"; then
        echo "--enable-${opt}"
    else
        echo "--disable-${opt}"
    fi
}

in_iuse() { has "$1" ${IUSE}; }

# === TOOLCHAIN FUNCTIONS ===
tc-is-cross-compiler() { return 1; }
tc-is-gcc() { return 0; }
tc-is-clang() { return 1; }
tc-getCC() { echo "${CC:-gcc}"; }
tc-getCXX() { echo "${CXX:-g++}"; }
tc-export() { :; }

tc-arch() {
    case "${CHOST:-x86_64-pc-linux-gnu}" in
        x86_64-*) echo "amd64" ;;
        i?86-*) echo "x86" ;;
        aarch64-*) echo "arm64" ;;
        arm-*) echo "arm" ;;
        *) echo "amd64" ;;
    esac
}

# === VERSION FUNCTIONS ===
# PMS-compliant ver_cut implementation.
#
# WORKAROUND: This implementation uses indexed variables (comp_0, comp_1, ...)
# instead of bash arrays with slicing (${arr[@]:start:len}) due to bugs in
# mvdan.cc/sh (https://github.com/mvdan/sh):
#
# 1. "local -a arr" without assignment doesn't set expand.Indexed kind
#    (runner.go:713-718 sets KeepValue instead of Indexed for -a flag)
#
# 2. Array slicing ${arr[@]:start:len} doesn't work correctly in command
#    substitution (subshells) - returns full array instead of slice
#
# TODO: Create issue and PR at https://github.com/mvdan/sh to fix these bugs,
# then simplify this implementation to use standard Portage ver_cut from
# bin/version-functions.sh

ver_cut() {
    local range="${1}"
    local v="${2:-${PV}}"

    # Parse range: "1" -> start=1,end=1; "1-3" -> start=1,end=3
    local start end
    if [[ "${range}" == *-* ]]; then
        start="${range%-*}"
        end="${range#*-}"
        [[ -z "${end}" ]] && end=999
    else
        start="${range}"
        end="${range}"
    fi

    # Build components using indexed variables (comp_0, comp_1, ...)
    # Pattern: comp_0=sep, comp_1=val, comp_2=sep, comp_3=val, ...
    local idx=0
    local char prev_is_num=-1  # -1=none, 0=alpha, 1=num
    local current=""
    local len=${#v}
    local i

    # First element is always empty (separator before first component)
    eval "comp_0=''"
    idx=1

    i=0
    while (( i < len )); do
        char="${v:i:1}"

        if [[ "${char}" =~ [0-9] ]]; then
            if (( prev_is_num == 1 )); then
                current="${current}${char}"
            else
                if (( prev_is_num == 0 )); then
                    eval "comp_${idx}='${current}'"
                    ((idx++))
                    eval "comp_${idx}=''"
                    ((idx++))
                fi
                current="${char}"
                prev_is_num=1
            fi
        elif [[ "${char}" =~ [a-zA-Z] ]]; then
            if (( prev_is_num == 0 )); then
                current="${current}${char}"
            else
                if (( prev_is_num == 1 )); then
                    eval "comp_${idx}='${current}'"
                    ((idx++))
                    eval "comp_${idx}=''"
                    ((idx++))
                fi
                current="${char}"
                prev_is_num=0
            fi
        else
            if [[ -n "${current}" ]]; then
                eval "comp_${idx}='${current}'"
                ((idx++))
            fi
            eval "comp_${idx}='${char}'"
            ((idx++))
            current=""
            prev_is_num=-1
        fi
        ((i++))
    done

    if [[ -n "${current}" ]]; then
        eval "comp_${idx}='${current}'"
        ((idx++))
    fi

    local max=$(( (idx) / 2 ))
    [[ ${end} -gt ${max} ]] && end=${max}

    # Build result - component n is at comp_(2n-1), separator after n is at comp_(2n)
    local result=""
    local n
    for (( n=start; n<=end; n++ )); do
        local comp_idx=$(( n*2 - 1 ))
        eval "result=\"\${result}\${comp_${comp_idx}}\""
        if (( n < end )); then
            local sep_idx=$(( n*2 ))
            eval "result=\"\${result}\${comp_${sep_idx}}\""
        fi
    done

    echo "${result}"
}

ver_rs() {
    # Simplified ver_rs - returns version as-is
    # Full implementation (separator replacement) not needed for metadata extraction
    # and would require same workaround as ver_cut due to mvdan.cc/sh bugs
    local v="${2:-${PV}}"
    echo "${v}"
}

ver_test() {
    # Simplified ver_test - always returns true for metadata extraction
    # Full implementation would compare versions per PMS algorithm
    return 0
}

# === MULTILIB FUNCTIONS ===
get_libdir() { echo "lib64"; }
multilib_native_use_with() { use_with "$@"; }
multilib_native_use_enable() { use_enable "$@"; }
multilib_native_usex() { usex "$@"; }

# === PYTHON ECLASS STUBS ===
_python_check_PYTHON_COMPAT() { :; }
_python_set_impls() { :; }
python_check_deps() { return 0; }
python_gen_any_dep() { :; }
python_gen_cond_dep() { echo ""; }
python_setup() { :; }
EPYTHON="${EPYTHON:-python3}"
PYTHON="${PYTHON:-/usr/bin/python3}"

# === MISC STUBS ===
check_license() { :; }
optfeature() { :; }
estack_push() { :; }
estack_pop() { return 0; }
eshopts_push() { :; }
eshopts_pop() { :; }

# === END ECLASS INFRASTRUCTURE ===
`
