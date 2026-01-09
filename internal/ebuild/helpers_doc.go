// Package ebuild implements ebuild execution engine.
//
// This file provides EAPI 8 documentation functions (dodoc, doman, newdoc, newman).
package ebuild

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
