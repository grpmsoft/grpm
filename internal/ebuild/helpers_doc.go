// Package ebuild implements ebuild execution engine.
//
// This file provides EAPI 8 documentation functions (dodoc, doman, newdoc, newman, einstalldocs).
package ebuild

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ============================================================================
// EAPI 8 Documentation Functions
// ============================================================================

// getDocDir returns the documentation directory for the package.
func (h *Helpers) getDocDir() string {
	imageDir := h.getImageDir()
	pf := h.getPF()
	docDir := filepath.Join(imageDir, "usr", "share", "doc", pf)
	if h.docDestTree != "" {
		docDir = filepath.Join(docDir, h.docDestTree)
	}
	return docDir
}

// Dodoc installs documentation to ${D}/usr/share/doc/${PF}${DOCDESTTREE}.
//
// Usage: dodoc README CHANGELOG
// Usage: dodoc -r docs (recursive)
//
// Installs documentation files with mode 0644.
func (h *Helpers) Dodoc(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "dodoc: no files specified"}
	}

	docDir := h.getDocDir()
	if err := os.MkdirAll(docDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("dodoc: mkdir %s: %v", docDir, err)}
	}

	mode := fs.FileMode(0644)
	recursive := false
	files := args

	// Check for -r flag
	if len(args) > 0 && args[0] == "-r" {
		recursive = true
		files = args[1:]
	}

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			return &DieError{Message: fmt.Sprintf("dodoc: %s: %v", file, err)}
		}

		if info.IsDir() {
			if !recursive {
				return &DieError{Message: fmt.Sprintf("dodoc: %s is a directory (use -r)", file)}
			}
			dst := filepath.Join(docDir, filepath.Base(file))
			if err := h.installDir(file, dst, mode); err != nil {
				return &DieError{Message: fmt.Sprintf("dodoc: %v", err)}
			}
		} else {
			dst := filepath.Join(docDir, filepath.Base(file))
			if err := h.installFile(file, dst, mode); err != nil {
				return &DieError{Message: fmt.Sprintf("dodoc: %v", err)}
			}
		}
	}

	return nil
}

// Newdoc installs a doc file with a new name.
//
// Usage: newdoc README.md README
//
// Installs README.md as README in ${D}/usr/share/doc/${PF}.
func (h *Helpers) Newdoc(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "newdoc: requires source and destination name"}
	}

	src := args[0]
	destName := args[1]

	info, err := os.Stat(src)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("newdoc: %s: %v", src, err)}
	}
	if info.IsDir() {
		return &DieError{Message: fmt.Sprintf("newdoc: %s is a directory", src)}
	}

	docDir := h.getDocDir()
	if err := os.MkdirAll(docDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("newdoc: mkdir %s: %v", docDir, err)}
	}

	dst := filepath.Join(docDir, destName)
	if err := h.installFile(src, dst, 0644); err != nil {
		return &DieError{Message: fmt.Sprintf("newdoc: %v", err)}
	}

	return nil
}

// Doman installs man pages to ${D}/usr/share/man/manN.
//
// Usage: doman foo.1 bar.8
//
// Automatically determines section from file extension.
func (h *Helpers) Doman(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "doman: no files specified"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "doman: D not set"}
	}

	for _, file := range args {
		info, err := os.Stat(file)
		if err != nil {
			return &DieError{Message: fmt.Sprintf("doman: %s: %v", file, err)}
		}
		if info.IsDir() {
			return &DieError{Message: fmt.Sprintf("doman: %s is a directory", file)}
		}

		// Determine man section from extension
		ext := filepath.Ext(file)
		if ext == "" || len(ext) < 2 {
			return &DieError{Message: fmt.Sprintf("doman: %s has no valid section extension", file)}
		}
		section := ext[1:] // Remove the dot

		destDir := filepath.Join(imageDir, "usr", "share", "man", "man"+section)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return &DieError{Message: fmt.Sprintf("doman: mkdir %s: %v", destDir, err)}
		}

		dst := filepath.Join(destDir, filepath.Base(file))
		if err := h.installFile(file, dst, 0644); err != nil {
			return &DieError{Message: fmt.Sprintf("doman: %v", err)}
		}
	}

	return nil
}

// Newman installs a man page with a new name.
//
// Usage: newman foo.man foo.1
//
// Installs foo.man as foo.1 in the appropriate man section.
func (h *Helpers) Newman(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "newman: requires source and destination name"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "newman: D not set"}
	}

	src := args[0]
	destName := args[1]

	info, err := os.Stat(src)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("newman: %s: %v", src, err)}
	}
	if info.IsDir() {
		return &DieError{Message: fmt.Sprintf("newman: %s is a directory", src)}
	}

	// Determine man section from destination name extension
	ext := filepath.Ext(destName)
	if ext == "" || len(ext) < 2 {
		return &DieError{Message: fmt.Sprintf("newman: %s has no valid section extension", destName)}
	}
	section := ext[1:]

	destDir := filepath.Join(imageDir, "usr", "share", "man", "man"+section)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("newman: mkdir %s: %v", destDir, err)}
	}

	dst := filepath.Join(destDir, destName)
	if err := h.installFile(src, dst, 0644); err != nil {
		return &DieError{Message: fmt.Sprintf("newman: %v", err)}
	}

	return nil
}

// Doinfo installs GNU Info files to ${D}/usr/share/info.
//
// Usage: doinfo <file>...
//
// Per PMS Section 12.3.9:
//   - Installs GNU Info files into /usr/share/info
//   - Files are installed with mode 0644
//   - Failure behavior is EAPI dependent
//
// Example:
//
//	doinfo doc/myapp.info
//	doinfo doc/*.info
func (h *Helpers) Doinfo(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "doinfo: no files specified"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "doinfo: D not set"}
	}

	destDir := filepath.Join(imageDir, "usr", "share", "info")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("doinfo: mkdir %s: %v", destDir, err)}
	}

	for _, file := range args {
		info, err := os.Stat(file)
		if err != nil {
			return &DieError{Message: fmt.Sprintf("doinfo: %s: %v", file, err)}
		}
		if info.IsDir() {
			return &DieError{Message: fmt.Sprintf("doinfo: %s is a directory", file)}
		}

		dst := filepath.Join(destDir, filepath.Base(file))
		if err := h.installFile(file, dst, 0644); err != nil {
			return &DieError{Message: fmt.Sprintf("doinfo: %v", err)}
		}
	}

	return nil
}

// ============================================================================
// EAPI 8 Standard Documentation Installation
// ============================================================================

// standardDocPatterns contains the standard documentation file patterns
// that einstalldocs looks for. Per PMS Section 11.3.3.20.
var standardDocPatterns = []string{
	"README",
	"README.*",
	"README.md",
	"README.rst",
	"README.txt",
	"CHANGELOG",
	"CHANGELOG.*",
	"ChangeLog",
	"ChangeLog.*",
	"CHANGES",
	"CHANGES.*",
	"AUTHORS",
	"AUTHORS.*",
	"NEWS",
	"NEWS.*",
	"TODO",
	"TODO.*",
	"COPYING",
	"COPYING.*",
	"LICENSE",
	"LICENSE.*",
	"LICENSE-*",
	"HACKING",
	"HACKING.*",
	"MAINTAINERS",
}

// installDocFiles installs a space-separated list of doc files from a variable.
// workdir is the base directory for relative paths.
func (h *Helpers) installDocFiles(docsVar, workdir string) error {
	for _, f := range strings.Fields(docsVar) {
		// Handle paths that might be relative or absolute
		var fullPath string
		if filepath.IsAbs(f) {
			fullPath = f
		} else {
			fullPath = filepath.Join(workdir, f)
		}

		// Check if file/directory exists
		info, err := os.Stat(fullPath)
		if err != nil {
			// Skip missing files silently (per Portage behavior)
			continue
		}

		if info.IsDir() {
			// Install directory recursively
			if err := h.Dodoc([]string{"-r", fullPath}); err != nil {
				return err
			}
		} else {
			if err := h.Dodoc([]string{fullPath}); err != nil {
				return err
			}
		}
	}
	return nil
}

// installStandardDocs installs documentation files matching standard patterns.
func (h *Helpers) installStandardDocs(workdir string) {
	for _, pattern := range standardDocPatterns {
		matches, err := filepath.Glob(filepath.Join(workdir, pattern))
		if err != nil {
			// Invalid pattern - skip
			continue
		}

		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				continue
			}

			// Skip directories for simple patterns
			if info.IsDir() {
				continue
			}

			// Install the file
			if err := h.Dodoc([]string{match}); err != nil {
				// Log warning but continue with other files
				h.writeStderr(fmt.Sprintf(">>> einstalldocs: warning: failed to install %s: %v\n",
					filepath.Base(match), err))
			}
		}
	}
}

// Einstalldocs installs standard documentation files.
//
// Per PMS Section 11.3.3.20 (EAPI 8):
//   - Looks for standard documentation files in ${S} (source directory)
//   - Installs found files to ${ED}/usr/share/doc/${PF}/
//   - Respects the DOCS environment variable if set
//   - Respects the HTML_DOCS environment variable for HTML documentation
//
// Standard files searched:
//
//	README*, CHANGELOG*, ChangeLog*, AUTHORS*, NEWS*, TODO*,
//	COPYING*, LICENSE*, HACKING*, MAINTAINERS
//
// Usage: einstalldocs (no arguments)
//
// Returns nil on success. Missing files are silently skipped.
func (h *Helpers) Einstalldocs(args []string) error {
	// Get the source directory (S)
	workdir := h.getSourceDir()
	if workdir == "" {
		return &DieError{Message: "einstalldocs: S (source directory) not set"}
	}

	// Install files from DOCS variable if set
	if h.env != nil {
		if docsVar := h.env.GetVar("DOCS"); docsVar != "" {
			if err := h.installDocFiles(docsVar, workdir); err != nil {
				return err
			}
		}
	}

	// Install standard documentation files
	h.installStandardDocs(workdir)

	// Handle HTML_DOCS
	if h.env != nil {
		if htmlDocsVar := h.env.GetVar("HTML_DOCS"); htmlDocsVar != "" {
			// Save current docDestTree and set to html/
			oldDocDestTree := h.docDestTree
			h.docDestTree = "html"
			err := h.installDocFiles(htmlDocsVar, workdir)
			h.docDestTree = oldDocDestTree
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// getSourceDir returns the source directory (S or WORKDIR).
func (h *Helpers) getSourceDir() string {
	if h.env == nil {
		return ""
	}
	if h.env.S != "" {
		return h.env.S
	}
	return h.env.WORKDIR
}

// ============================================================================
// EAPI 8 Locale/Gettext Functions
// ============================================================================

// Domo installs gettext .mo (message object) files to the locale directory.
//
// Usage: domo <file>...
//
// Per PMS Section 12.3.9:
//   - Installs .mo files with mode 0644
//   - Language is extracted from filename (e.g., de.mo -> de, fr_FR.mo -> fr_FR)
//   - Destination filename is ${PN}.mo
//   - Failure behavior is EAPI dependent
//
// Per PMS Table 12.15, destination path is EAPI dependent:
//   - EAPI 0-6: ${DESTTREE}/share/locale
//   - EAPI 7-8: /usr/share/locale
//
// Per PMS Table 12.3, domo is banned in EAPI 9.
//
// Example:
//
//	domo po/de.mo            # -> /usr/share/locale/de/LC_MESSAGES/${PN}.mo
//	domo po/fr_FR.mo         # -> /usr/share/locale/fr_FR/LC_MESSAGES/${PN}.mo
//	domo po/*.mo             # install all
func (h *Helpers) Domo(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "domo: no files specified"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "domo: D not set"}
	}

	// Get PN for the installed filename
	pn := h.getPN()
	if pn == "" {
		return &DieError{Message: "domo: PN not set"}
	}

	// Determine locale base directory based on EAPI
	// Per PMS Table 12.15:
	// - EAPI 0-6: ${DESTTREE}/share/locale
	// - EAPI 7+: /usr/share/locale
	localeBase := h.getLocaleBaseDir()

	for _, file := range args {
		info, err := os.Stat(file)
		if err != nil {
			return &DieError{Message: fmt.Sprintf("domo: %s: %v", file, err)}
		}
		if info.IsDir() {
			return &DieError{Message: fmt.Sprintf("domo: %s is a directory", file)}
		}

		// Extract locale from filename
		// Per PMS: "generated by taking the basename of the file, removing the .* suffix"
		// e.g., de.mo -> de, fr_FR.mo -> fr_FR
		base := filepath.Base(file)
		locale := strings.TrimSuffix(base, filepath.Ext(base))
		if locale == "" {
			return &DieError{Message: fmt.Sprintf("domo: %s has no valid locale in filename", file)}
		}

		// Build destination path: <locale_base>/<locale>/LC_MESSAGES/${PN}.mo
		destDir := filepath.Join(imageDir, localeBase, locale, "LC_MESSAGES")
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return &DieError{Message: fmt.Sprintf("domo: mkdir %s: %v", destDir, err)}
		}

		// Install as ${PN}.mo (not original filename)
		destFile := filepath.Join(destDir, pn+".mo")
		if err := h.installFile(file, destFile, 0644); err != nil {
			return &DieError{Message: fmt.Sprintf("domo: %v", err)}
		}
	}

	return nil
}

// getPN returns the package name (PN).
func (h *Helpers) getPN() string {
	if h.env != nil {
		return h.env.PN
	}
	return ""
}

// getLocaleBaseDir returns the locale directory base path.
//
// Per PMS Table 12.15:
//   - EAPI 0-6: ${DESTTREE}/share/locale (respects into command)
//   - EAPI 7+: /usr/share/locale (fixed path)
func (h *Helpers) getLocaleBaseDir() string {
	// Check EAPI to determine behavior
	if h.env != nil && h.isEAPI7OrLater() {
		// EAPI 7+: Fixed /usr/share/locale
		return "/usr/share/locale"
	}
	// EAPI 0-6: ${DESTTREE}/share/locale
	return filepath.Join(h.destTree, "share", "locale")
}

// isEAPI7OrLater returns true if the current EAPI is 7 or later.
func (h *Helpers) isEAPI7OrLater() bool {
	if h.env == nil {
		return false
	}
	eapi := h.env.EAPI
	if eapi == "" {
		return false
	}
	// EAPI 7, 8, 9 use fixed /usr/share/locale
	return eapi == "7" || eapi == "8" || eapi == "9"
}
