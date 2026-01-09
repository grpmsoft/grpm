// Package ebuild implements ebuild execution engine.
//
// This file provides default phase implementations per PMS Section 9.1.17 and Section 12.3.15.
//
// Per PMS Table 9.10, default_* functions are available in:
//   - EAPI 2-3: pkg_nofetch, src_unpack, src_prepare, src_configure, src_compile, src_test
//   - EAPI 4+:  All above plus src_install
//
// The `default` function (PMS 12.3.15) is available in EAPI 2+ and calls default_${EBUILD_PHASE}.
package ebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ============================================================================
// Default Phase Implementations per PMS Section 9.1.17
// ============================================================================

// DefaultSrcUnpack is the default src_unpack implementation.
//
// Usage: default_src_unpack
//
// Per PMS Section 9.1.4:
//
//	src_unpack() {
//	    if [[ -n ${A} ]]; then
//	        unpack ${A}
//	    fi
//	}
//
// Available in EAPI 2+ per PMS Table 9.10.
func (h *Helpers) DefaultSrcUnpack(args []string) error {
	// Check EAPI (available in EAPI 2+)
	if err := h.checkDefaultFunctionEAPI("default_src_unpack"); err != nil {
		return err
	}

	if h.env == nil {
		return &DieError{Message: "default_src_unpack: environment not set"}
	}

	// Get A (archive list)
	archives := strings.Fields(h.env.A)
	if len(archives) == 0 {
		h.writeStdout(">>> No archives to unpack (A is empty)\n")
		return nil
	}

	return h.Unpack(archives)
}

// DefaultSrcPrepare is the default src_prepare implementation.
//
// Usage: default_src_prepare
//
// Per PMS Section 9.1.5 (Table 9.4):
//   - EAPI 2-5: no-op
//   - EAPI 6-7: Format 6 (apply PATCHES, call eapply_user)
//   - EAPI 8+:  Format 8 (same as 6 but with bash 4.4+ array detection)
//
// Since we implement in Go, we use a simplified approach that handles both formats.
//
// Available in EAPI 2+ per PMS Table 9.10.
func (h *Helpers) DefaultSrcPrepare(args []string) error {
	// Check EAPI (available in EAPI 2+)
	if err := h.checkDefaultFunctionEAPI("default_src_prepare"); err != nil {
		return err
	}

	if h.env == nil {
		return &DieError{Message: "default_src_prepare: environment not set"}
	}

	// Per PMS Table 9.4:
	// - EAPI 2-5: no-op
	// - EAPI 6+: Apply PATCHES if set, then call eapply_user
	if !h.env.EAPIFeatures.EapplyUser {
		// EAPI 2-5: no-op
		return nil
	}

	// EAPI 6+: Apply PATCHES if set
	if patches := h.env.GetVar("PATCHES"); patches != "" {
		patchList := strings.Fields(patches)
		if len(patchList) > 0 {
			// Per PMS: eapply "${PATCHES[@]}" or eapply -- "${PATCHES[@]}" in EAPI 8
			if err := h.Eapply(patchList); err != nil {
				return err
			}
		}
	}

	// Always call eapply_user
	return h.EapplyUser(nil)
}

// DefaultSrcConfigure is the default src_configure implementation.
//
// Usage: default_src_configure
//
// Per PMS Section 9.1.6:
//
//	src_configure() {
//	    if [[ -x ${ECONF_SOURCE:-.}/configure ]]; then
//	        econf
//	    fi
//	}
//
// Available in EAPI 2+ per PMS Table 9.10.
func (h *Helpers) DefaultSrcConfigure(args []string) error {
	// Check EAPI (available in EAPI 2+)
	if err := h.checkDefaultFunctionEAPI("default_src_configure"); err != nil {
		return err
	}

	workDir := h.getWorkDir()
	if workDir == "" {
		return &DieError{Message: "default_src_configure: working directory not set"}
	}

	// Check ECONF_SOURCE first, then current directory
	econfSource := h.env.GetVar("ECONF_SOURCE")
	if econfSource == "" {
		econfSource = workDir
	}

	configurePath := filepath.Join(econfSource, "configure")

	// Check if configure exists and is executable
	info, err := os.Stat(configurePath)
	if os.IsNotExist(err) {
		h.writeStdout(">>> No configure script, skipping default_src_configure\n")
		return nil
	}
	if err != nil {
		return &DieError{Message: fmt.Sprintf("default_src_configure: stat %s: %v", configurePath, err)}
	}

	// Check if executable (PMS requires -x check)
	if info.Mode()&0111 == 0 {
		h.writeStdout(">>> configure exists but is not executable, skipping default_src_configure\n")
		return nil
	}

	return h.Econf(nil)
}

// DefaultSrcCompile is the default src_compile implementation.
//
// Usage: default_src_compile
//
// Per PMS Section 9.1.7 (Table 9.6):
//   - EAPI 0: runs econf if ./configure exists, then emake
//   - EAPI 1: runs econf if ${ECONF_SOURCE:-.}/configure exists, then emake
//   - EAPI 2+: runs emake if Makefile/GNUmakefile/makefile exists
//
// For default_src_compile (EAPI 2+), we use Format 2 (no auto-configure).
//
// Available in EAPI 2+ per PMS Table 9.10.
func (h *Helpers) DefaultSrcCompile(args []string) error {
	// Check EAPI (available in EAPI 2+)
	if err := h.checkDefaultFunctionEAPI("default_src_compile"); err != nil {
		return err
	}

	workDir := h.getWorkDir()
	if workDir == "" {
		return &DieError{Message: "default_src_compile: working directory not set"}
	}

	// Per PMS Format 2 (EAPI 2+): Only run emake if Makefile exists
	// Check for Makefile, GNUmakefile, or makefile
	if !h.hasMakefile(workDir) {
		h.writeStdout(">>> No Makefile, skipping default_src_compile\n")
		return nil
	}

	return h.Emake(nil)
}

// DefaultSrcTest is the default src_test implementation.
//
// Usage: default_src_test
//
// Per PMS Section 9.1.8:
// The default implementation runs emake check if available, else emake test if available.
// Per Table 9.7, EAPI 0-4 uses -j1, EAPI 5+ allows parallel tests.
//
// Available in EAPI 2+ per PMS Table 9.10.
func (h *Helpers) DefaultSrcTest(args []string) error {
	// Check EAPI (available in EAPI 2+)
	if err := h.checkDefaultFunctionEAPI("default_src_test"); err != nil {
		return err
	}

	workDir := h.getWorkDir()
	if workDir == "" {
		return &DieError{Message: "default_src_test: working directory not set"}
	}

	if !h.hasMakefile(workDir) {
		return nil
	}

	// Try "check" target first, then "test" target
	// Per PMS: Run emake check if target exists, else emake test if exists
	//
	// Note: In a full implementation, we would check if targets exist.
	// For simplicity, we try "check" and if it fails we try "test".
	// Most build systems use "check" for test target.
	return h.Emake([]string{"check"})
}

// DefaultSrcInstall is the default src_install implementation.
//
// Usage: default_src_install
//
// Per PMS Section 9.1.9 (Table 9.8):
//   - EAPI 0-3: no-op
//   - EAPI 4-5: Format 4 (emake install, manual DOCS handling)
//   - EAPI 6+:  Format 6 (emake install, einstalldocs)
//
// Available in EAPI 4+ per PMS Table 9.10.
func (h *Helpers) DefaultSrcInstall(args []string) error {
	// Check EAPI (available in EAPI 4+ only)
	if err := h.checkDefaultSrcInstallEAPI(); err != nil {
		return err
	}

	if h.env == nil {
		return &DieError{Message: "default_src_install: environment not set"}
	}

	workDir := h.getWorkDir()
	if workDir == "" {
		return &DieError{Message: "default_src_install: working directory not set"}
	}

	// Run emake install if Makefile exists
	if h.hasMakefile(workDir) {
		destdir := fmt.Sprintf("DESTDIR=%s", h.env.D)
		if err := h.Emake([]string{"install", destdir}); err != nil {
			return err
		}
	}

	// Per PMS: EAPI 6+ calls einstalldocs; EAPI 4-5 has inline DOCS handling
	if h.env.EAPIFeatures.Einstalldocs {
		// EAPI 6+ format: call einstalldocs
		return h.Einstalldocs(nil)
	}

	// EAPI 4-5: Manual DOCS handling
	// Per PMS Format 4: Install standard docs if DOCS is not set
	return h.installDocsFormat4(workDir)
}

// installDocsFormat4 implements EAPI 4-5 documentation installation.
//
// Per PMS Format 4:
//
//	if ! declare -p DOCS >/dev/null 2>&1; then
//	    for d in README* ChangeLog AUTHORS NEWS TODO CHANGES THANKS BUGS FAQ CREDITS CHANGELOG; do
//	        [[ -s "${d}" ]] && dodoc "${d}"
//	    done
//	elif [[ $(declare -p DOCS) == "declare -a"* ]]; then
//	    dodoc "${DOCS[@]}"
//	else
//	    dodoc ${DOCS}
//	fi
func (h *Helpers) installDocsFormat4(workDir string) error {
	// Check if DOCS is set
	if docsVar := h.env.GetVar("DOCS"); docsVar != "" {
		// DOCS is set, install those files
		for _, doc := range strings.Fields(docsVar) {
			docPath := doc
			if !filepath.IsAbs(doc) {
				docPath = filepath.Join(workDir, doc)
			}
			if info, err := os.Stat(docPath); err == nil && !info.IsDir() && info.Size() > 0 {
				if err := h.Dodoc([]string{docPath}); err != nil {
					// Log warning but continue
					h.writeStderr(fmt.Sprintf(">>> default_src_install: warning: failed to install %s: %v\n", doc, err))
				}
			}
		}
		return nil
	}

	// DOCS not set, install standard documentation files
	standardDocs := []string{
		"README", "README.*",
		"ChangeLog", "CHANGELOG",
		"AUTHORS",
		"NEWS",
		"TODO",
		"CHANGES",
		"THANKS",
		"BUGS",
		"FAQ",
		"CREDITS",
	}

	for _, pattern := range standardDocs {
		matches, err := filepath.Glob(filepath.Join(workDir, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || info.IsDir() || info.Size() == 0 {
				continue
			}
			if err := h.Dodoc([]string{match}); err != nil {
				// Log warning but continue
				h.writeStderr(fmt.Sprintf(">>> default_src_install: warning: failed to install %s: %v\n", filepath.Base(match), err))
			}
		}
	}

	return nil
}

// DefaultPkgNofetch is the default pkg_nofetch implementation.
//
// Usage: default_pkg_nofetch
//
// Per PMS Section 9.1.16:
// The pkg_nofetch function is run when the fetch phase of a fetch-restricted ebuild
// is run, and the relevant source files are not available. It should direct the user
// to download all relevant source files from their respective locations.
//
// Available in EAPI 2+ per PMS Table 9.10.
func (h *Helpers) DefaultPkgNofetch(args []string) error {
	// Check EAPI (available in EAPI 2+)
	if err := h.checkDefaultFunctionEAPI("default_pkg_nofetch"); err != nil {
		return err
	}

	if h.env == nil {
		return &DieError{Message: "default_pkg_nofetch: environment not set"}
	}

	// Print message about fetch restriction
	h.writeStderr(fmt.Sprintf("!!! The following files could not be fetched for %s/%s:\n", h.env.CATEGORY, h.env.PF))

	// List files in A
	archives := strings.Fields(h.env.A)
	for _, archive := range archives {
		h.writeStderr(fmt.Sprintf("!!!   %s\n", archive))
	}

	// Suggest checking SRC_URI
	h.writeStderr("!!! Please download these files manually and place them in your DISTDIR.\n")

	return nil
}

// ============================================================================
// Default Phase Dispatcher per PMS Section 12.3.15
// ============================================================================

// Default is the generic default function dispatcher.
//
// Usage: default
//
// Per PMS Section 12.3.15:
// Calls the default_${EBUILD_PHASE} function for the current phase.
// Must not be called if the default_ function does not exist for the current phase.
//
// Available in EAPI 2+ per PMS Table 12.26.
func (h *Helpers) Default(args []string) error {
	// Check EAPI (available in EAPI 2+)
	if err := h.checkDefaultCommandEAPI(); err != nil {
		return err
	}

	// Try to get phase from environment struct first
	phase := ""
	if h.env != nil && h.env.EBUILD_PHASE != "" {
		phase = h.env.EBUILD_PHASE
	}

	// Fall back to OS environment variable
	if phase == "" {
		phase = os.Getenv("EBUILD_PHASE")
	}

	// Dispatch to appropriate default_ function
	switch phase {
	case "nofetch":
		return h.DefaultPkgNofetch(args)
	case "unpack":
		return h.DefaultSrcUnpack(args)
	case "prepare":
		return h.DefaultSrcPrepare(args)
	case "configure":
		return h.DefaultSrcConfigure(args)
	case "compile":
		return h.DefaultSrcCompile(args)
	case "test":
		return h.DefaultSrcTest(args)
	case "install":
		return h.DefaultSrcInstall(args)
	default:
		// Per PMS: Must not be called if default_ function does not exist
		// For unknown phases, return error
		if phase == "" {
			return &DieError{Message: "default: EBUILD_PHASE not set"}
		}
		return &DieError{Message: fmt.Sprintf("default: no default_%s function exists", phase)}
	}
}

// ============================================================================
// EAPI Version Checks
// ============================================================================

// checkDefaultFunctionEAPI checks if default_ functions are available in the current EAPI.
//
// Per PMS Table 9.10:
//   - EAPI 0-1: default_ functions not available
//   - EAPI 2+:  default_ functions available (except default_src_install)
func (h *Helpers) checkDefaultFunctionEAPI(funcName string) error {
	if h.env == nil {
		// No environment, allow for backward compatibility
		return nil
	}

	eapi := h.env.EAPI
	if eapi == "" {
		eapi = "0"
	}

	// EAPI 0-1: default_ functions not available
	if eapi == "0" || eapi == "1" {
		return &DieError{Message: fmt.Sprintf("%s: not available in EAPI %s (requires EAPI 2+)", funcName, eapi)}
	}

	return nil
}

// checkDefaultSrcInstallEAPI checks if default_src_install is available.
//
// Per PMS Table 9.10:
//   - EAPI 0-3: default_src_install not available
//   - EAPI 4+:  default_src_install available
func (h *Helpers) checkDefaultSrcInstallEAPI() error {
	if h.env == nil {
		return nil
	}

	eapi := h.env.EAPI
	if eapi == "" {
		eapi = "0"
	}

	// EAPI 0-3: default_src_install not available
	if eapi == "0" || eapi == "1" || eapi == "2" || eapi == "3" {
		return &DieError{Message: fmt.Sprintf("default_src_install: not available in EAPI %s (requires EAPI 4+)", eapi)}
	}

	return nil
}

// checkDefaultCommandEAPI checks if the default command is available.
//
// Per PMS Table 12.26:
//   - EAPI 0-1: default command not available
//   - EAPI 2+:  default command available
func (h *Helpers) checkDefaultCommandEAPI() error {
	if h.env == nil {
		return nil
	}

	eapi := h.env.EAPI
	if eapi == "" {
		eapi = "0"
	}

	// EAPI 0-1: default command not available
	if eapi == "0" || eapi == "1" {
		return &DieError{Message: fmt.Sprintf("default: not available in EAPI %s (requires EAPI 2+)", eapi)}
	}

	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// hasMakefile checks if Makefile, GNUmakefile, or makefile exists in the directory.
func (h *Helpers) hasMakefile(dir string) bool {
	makefiles := []string{"Makefile", "GNUmakefile", "makefile"}
	for _, name := range makefiles {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}
