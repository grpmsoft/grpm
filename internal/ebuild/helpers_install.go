// Package ebuild implements ebuild execution engine.
//
// This file provides EAPI 8 installation functions (dobin, doins, dosym, etc.).
package ebuild

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ============================================================================
// EAPI 8 Directory Setting Functions
// ============================================================================

// Insinto sets the installation directory for doins.
//
// Usage: insinto /usr/share/myapp
//
// Sets INSDESTTREE which is used by doins to determine the target directory.
func (h *Helpers) Insinto(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "insinto: no directory specified"}
	}
	h.insDestTree = args[0]
	return nil
}

// Exeinto sets the installation directory for doexe.
//
// Usage: exeinto /usr/libexec
//
// Sets EXEDESTTREE which is used by doexe to determine the target directory.
func (h *Helpers) Exeinto(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "exeinto: no directory specified"}
	}
	h.exeDestTree = args[0]
	return nil
}

// Docinto sets the subdirectory for dodoc.
//
// Usage: docinto examples
//
// Sets a subdirectory relative to /usr/share/doc/${PF} for dodoc.
// Use empty string or "/" to reset to default.
func (h *Helpers) Docinto(args []string) error {
	if len(args) < 1 {
		h.docDestTree = ""
		return nil
	}
	subdir := args[0]
	if subdir == "/" {
		subdir = ""
	}
	h.docDestTree = subdir
	return nil
}

// ============================================================================
// EAPI 8 Option Setting Functions
// ============================================================================

// parseMode parses a mode string like "-m0644" or "0755" and returns the mode.
func parseMode(opts string) (fs.FileMode, error) {
	// Find -m flag
	parts := strings.Fields(opts)
	for _, part := range parts {
		if strings.HasPrefix(part, "-m") {
			modeStr := strings.TrimPrefix(part, "-m")
			mode, err := strconv.ParseInt(modeStr, 8, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid mode: %s", modeStr)
			}
			return fs.FileMode(mode), nil
		}
	}
	return 0644, nil // Default mode
}

// Insopts sets install options for doins.
//
// Usage: insopts -m0600
//
// Default: -m0644
func (h *Helpers) Insopts(args []string) error {
	if len(args) < 1 {
		h.insOpts = "-m0644" // Reset to default
		return nil
	}
	h.insOpts = strings.Join(args, " ")
	return nil
}

// Exeopts sets install options for doexe/dobin.
//
// Usage: exeopts -m0700
//
// Default: -m0755
func (h *Helpers) Exeopts(args []string) error {
	if len(args) < 1 {
		h.exeOpts = "-m0755" // Reset to default
		return nil
	}
	h.exeOpts = strings.Join(args, " ")
	return nil
}

// Diropts sets options for dodir.
//
// Usage: diropts -m0700
//
// Default: -m0755
func (h *Helpers) Diropts(args []string) error {
	if len(args) < 1 {
		h.dirOpts = "-m0755" // Reset to default
		return nil
	}
	h.dirOpts = strings.Join(args, " ")
	return nil
}

// ============================================================================
// EAPI 8 Binary Installation Functions
// ============================================================================

// getImageDir returns the image directory ${D}.
func (h *Helpers) getImageDir() string {
	if h.env != nil {
		return h.env.D
	}
	return ""
}

// getPF returns the package full name (${PF}).
func (h *Helpers) getPF() string {
	if h.env != nil {
		return h.env.PF
	}
	return "unknown"
}

// installFile copies a file with specified permissions.
func (h *Helpers) installFile(src, dst string, mode fs.FileMode) error {
	// Handle symlinks - preserve them
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		// Copy symlink
		target, err := os.Readlink(src)
		if err != nil {
			return fmt.Errorf("readlink %s: %w", src, err)
		}
		if err := os.Symlink(target, dst); err != nil {
			return fmt.Errorf("symlink %s -> %s: %w", dst, target, err)
		}
		return nil
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}

	// Copy regular file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy %s to %s: %w", src, dst, err)
	}

	return nil
}

// installDir recursively copies a directory.
func (h *Helpers) installDir(src, dst string, mode fs.FileMode) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		return h.installFile(path, dstPath, mode)
	})
}

// Dobin installs executables to ${D}${DESTTREE}/bin.
//
// Usage: dobin myapp
//
// Installs files to /usr/bin by default with mode 0755.
func (h *Helpers) Dobin(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "dobin: no files specified"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "dobin: D not set"}
	}

	mode, err := parseMode(h.exeOpts)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("dobin: %v", err)}
	}

	destDir := filepath.Join(imageDir, h.destTree, "bin")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("dobin: mkdir %s: %v", destDir, err)}
	}

	for _, file := range args {
		info, err := os.Stat(file)
		if err != nil {
			return &DieError{Message: fmt.Sprintf("dobin: %s: %v", file, err)}
		}
		if info.IsDir() {
			return &DieError{Message: fmt.Sprintf("dobin: %s is a directory", file)}
		}

		dst := filepath.Join(destDir, filepath.Base(file))
		if err := h.installFile(file, dst, mode); err != nil {
			return &DieError{Message: fmt.Sprintf("dobin: %v", err)}
		}
	}

	return nil
}

// Dosbin installs executables to ${D}${DESTTREE}/sbin.
//
// Usage: dosbin mydaemon
//
// Installs files to /usr/sbin by default with mode 0755.
func (h *Helpers) Dosbin(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "dosbin: no files specified"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "dosbin: D not set"}
	}

	mode, err := parseMode(h.exeOpts)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("dosbin: %v", err)}
	}

	destDir := filepath.Join(imageDir, h.destTree, "sbin")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("dosbin: mkdir %s: %v", destDir, err)}
	}

	for _, file := range args {
		info, err := os.Stat(file)
		if err != nil {
			return &DieError{Message: fmt.Sprintf("dosbin: %s: %v", file, err)}
		}
		if info.IsDir() {
			return &DieError{Message: fmt.Sprintf("dosbin: %s is a directory", file)}
		}

		dst := filepath.Join(destDir, filepath.Base(file))
		if err := h.installFile(file, dst, mode); err != nil {
			return &DieError{Message: fmt.Sprintf("dosbin: %v", err)}
		}
	}

	return nil
}

// Newbin installs a file as an executable with a new name.
//
// Usage: newbin src.sh dest
//
// Installs src.sh as dest in ${D}${DESTTREE}/bin.
func (h *Helpers) Newbin(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "newbin: requires source and destination name"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "newbin: D not set"}
	}

	mode, err := parseMode(h.exeOpts)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("newbin: %v", err)}
	}

	src := args[0]
	destName := args[1]

	info, err := os.Stat(src)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("newbin: %s: %v", src, err)}
	}
	if info.IsDir() {
		return &DieError{Message: fmt.Sprintf("newbin: %s is a directory", src)}
	}

	destDir := filepath.Join(imageDir, h.destTree, "bin")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("newbin: mkdir %s: %v", destDir, err)}
	}

	dst := filepath.Join(destDir, destName)
	if err := h.installFile(src, dst, mode); err != nil {
		return &DieError{Message: fmt.Sprintf("newbin: %v", err)}
	}

	return nil
}

// Newsbin installs a file as sbin executable with a new name.
//
// Usage: newsbin src.sh dest
//
// Installs src.sh as dest in ${D}${DESTTREE}/sbin.
func (h *Helpers) Newsbin(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "newsbin: requires source and destination name"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "newsbin: D not set"}
	}

	mode, err := parseMode(h.exeOpts)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("newsbin: %v", err)}
	}

	src := args[0]
	destName := args[1]

	info, err := os.Stat(src)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("newsbin: %s: %v", src, err)}
	}
	if info.IsDir() {
		return &DieError{Message: fmt.Sprintf("newsbin: %s is a directory", src)}
	}

	destDir := filepath.Join(imageDir, h.destTree, "sbin")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("newsbin: mkdir %s: %v", destDir, err)}
	}

	dst := filepath.Join(destDir, destName)
	if err := h.installFile(src, dst, mode); err != nil {
		return &DieError{Message: fmt.Sprintf("newsbin: %v", err)}
	}

	return nil
}

// Doexe installs executables to ${D}${EXEDESTTREE}.
//
// Usage: doexe script.sh
//
// Installs files to EXEDESTTREE (set by exeinto) with mode 0755.
func (h *Helpers) Doexe(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "doexe: no files specified"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "doexe: D not set"}
	}

	if h.exeDestTree == "" {
		return &DieError{Message: "doexe: EXEDESTTREE not set (use exeinto first)"}
	}

	mode, err := parseMode(h.exeOpts)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("doexe: %v", err)}
	}

	destDir := filepath.Join(imageDir, h.exeDestTree)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("doexe: mkdir %s: %v", destDir, err)}
	}

	for _, file := range args {
		info, err := os.Stat(file)
		if err != nil {
			return &DieError{Message: fmt.Sprintf("doexe: %s: %v", file, err)}
		}
		if info.IsDir() {
			return &DieError{Message: fmt.Sprintf("doexe: %s is a directory", file)}
		}

		dst := filepath.Join(destDir, filepath.Base(file))
		if err := h.installFile(file, dst, mode); err != nil {
			return &DieError{Message: fmt.Sprintf("doexe: %v", err)}
		}
	}

	return nil
}

// ============================================================================
// EAPI 8 File Installation Functions
// ============================================================================

// Doins installs files to ${D}${INSDESTTREE}.
//
// Usage: doins file1 file2
// Usage: doins -r directory (recursive)
//
// Installs files to INSDESTTREE (set by insinto) with INSOPTS permissions.
func (h *Helpers) Doins(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "doins: no files specified"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "doins: D not set"}
	}

	mode, err := parseMode(h.insOpts)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("doins: %v", err)}
	}

	destDir := filepath.Join(imageDir, h.insDestTree)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("doins: mkdir %s: %v", destDir, err)}
	}

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
			return &DieError{Message: fmt.Sprintf("doins: %s: %v", file, err)}
		}

		if info.IsDir() {
			if !recursive {
				return &DieError{Message: fmt.Sprintf("doins: %s is a directory (use -r)", file)}
			}
			dst := filepath.Join(destDir, filepath.Base(file))
			if err := h.installDir(file, dst, mode); err != nil {
				return &DieError{Message: fmt.Sprintf("doins: %v", err)}
			}
		} else {
			dst := filepath.Join(destDir, filepath.Base(file))
			if err := h.installFile(file, dst, mode); err != nil {
				return &DieError{Message: fmt.Sprintf("doins: %v", err)}
			}
		}
	}

	return nil
}

// Newins installs a file with a new name.
//
// Usage: newins source.conf dest.conf
//
// Installs source.conf as dest.conf in ${D}${INSDESTTREE}.
func (h *Helpers) Newins(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "newins: requires source and destination name"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "newins: D not set"}
	}

	mode, err := parseMode(h.insOpts)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("newins: %v", err)}
	}

	src := args[0]
	destName := args[1]

	info, err := os.Stat(src)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("newins: %s: %v", src, err)}
	}
	if info.IsDir() {
		return &DieError{Message: fmt.Sprintf("newins: %s is a directory", src)}
	}

	destDir := filepath.Join(imageDir, h.insDestTree)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("newins: mkdir %s: %v", destDir, err)}
	}

	dst := filepath.Join(destDir, destName)
	if err := h.installFile(src, dst, mode); err != nil {
		return &DieError{Message: fmt.Sprintf("newins: %v", err)}
	}

	return nil
}

// ============================================================================
// EAPI 8 Library/Header Installation Functions
// ============================================================================

// Dolib installs libraries to ${D}${DESTTREE}/$(get_libdir).
//
// Deprecated in EAPI 7+, use dolib.so or dolib.a instead.
//
// Usage: dolib libfoo.so libbar.a
func (h *Helpers) Dolib(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "dolib: no files specified"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "dolib: D not set"}
	}

	destDir := filepath.Join(imageDir, h.destTree, h.libDir)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("dolib: mkdir %s: %v", destDir, err)}
	}

	for _, file := range args {
		info, err := os.Stat(file)
		if err != nil {
			return &DieError{Message: fmt.Sprintf("dolib: %s: %v", file, err)}
		}
		if info.IsDir() {
			return &DieError{Message: fmt.Sprintf("dolib: %s is a directory", file)}
		}

		// Use 0755 for shared libs, 0644 for static
		mode := fs.FileMode(0644)
		if strings.HasSuffix(file, ".so") || strings.Contains(file, ".so.") {
			mode = 0755
		}

		dst := filepath.Join(destDir, filepath.Base(file))
		if err := h.installFile(file, dst, mode); err != nil {
			return &DieError{Message: fmt.Sprintf("dolib: %v", err)}
		}
	}

	return nil
}

// DolibSo installs shared libraries.
//
// Usage: dolib.so libfoo.so.1.0
//
// Installs shared libraries with mode 0755.
func (h *Helpers) DolibSo(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "dolib.so: no files specified"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "dolib.so: D not set"}
	}

	destDir := filepath.Join(imageDir, h.destTree, h.libDir)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("dolib.so: mkdir %s: %v", destDir, err)}
	}

	for _, file := range args {
		info, err := os.Lstat(file)
		if err != nil {
			return &DieError{Message: fmt.Sprintf("dolib.so: %s: %v", file, err)}
		}
		if info.IsDir() {
			return &DieError{Message: fmt.Sprintf("dolib.so: %s is a directory", file)}
		}

		dst := filepath.Join(destDir, filepath.Base(file))
		if err := h.installFile(file, dst, 0755); err != nil {
			return &DieError{Message: fmt.Sprintf("dolib.so: %v", err)}
		}
	}

	return nil
}

// DolibA installs static libraries.
//
// Usage: dolib.a libfoo.a
//
// Installs static libraries with mode 0644.
func (h *Helpers) DolibA(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "dolib.a: no files specified"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "dolib.a: D not set"}
	}

	destDir := filepath.Join(imageDir, h.destTree, h.libDir)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("dolib.a: mkdir %s: %v", destDir, err)}
	}

	for _, file := range args {
		info, err := os.Stat(file)
		if err != nil {
			return &DieError{Message: fmt.Sprintf("dolib.a: %s: %v", file, err)}
		}
		if info.IsDir() {
			return &DieError{Message: fmt.Sprintf("dolib.a: %s is a directory", file)}
		}

		dst := filepath.Join(destDir, filepath.Base(file))
		if err := h.installFile(file, dst, 0644); err != nil {
			return &DieError{Message: fmt.Sprintf("dolib.a: %v", err)}
		}
	}

	return nil
}

// Doheader installs headers to ${D}/usr/include${INSDESTTREE}.
//
// Usage: doheader foo.h bar.h
// Usage: doheader -r include (recursive)
//
// Installs header files with mode 0644.
func (h *Helpers) Doheader(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "doheader: no files specified"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "doheader: D not set"}
	}

	// If INSDESTTREE is set to something other than /usr, use it as subdir
	subDir := ""
	if h.insDestTree != "/usr" && h.insDestTree != "" {
		subDir = h.insDestTree
	}

	destDir := filepath.Join(imageDir, "usr", "include")
	if subDir != "" {
		// Strip leading /usr/include if present
		subDir = strings.TrimPrefix(subDir, "/usr/include")
		subDir = strings.TrimPrefix(subDir, "/")
		if subDir != "" {
			destDir = filepath.Join(destDir, subDir)
		}
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("doheader: mkdir %s: %v", destDir, err)}
	}

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
			return &DieError{Message: fmt.Sprintf("doheader: %s: %v", file, err)}
		}

		if info.IsDir() {
			if !recursive {
				return &DieError{Message: fmt.Sprintf("doheader: %s is a directory (use -r)", file)}
			}
			dst := filepath.Join(destDir, filepath.Base(file))
			if err := h.installDir(file, dst, 0644); err != nil {
				return &DieError{Message: fmt.Sprintf("doheader: %v", err)}
			}
		} else {
			dst := filepath.Join(destDir, filepath.Base(file))
			if err := h.installFile(file, dst, 0644); err != nil {
				return &DieError{Message: fmt.Sprintf("doheader: %v", err)}
			}
		}
	}

	return nil
}

// ============================================================================
// EAPI 8 Directory Creation Functions
// ============================================================================

// Dodir creates directories in ${D} with DIROPTS permissions.
//
// Usage: dodir /usr/share/myapp /etc/myapp
//
// Creates directories with mode from diropts (default 0755).
func (h *Helpers) Dodir(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "dodir: no directories specified"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "dodir: D not set"}
	}

	mode, err := parseMode(h.dirOpts)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("dodir: %v", err)}
	}

	for _, dir := range args {
		destDir := filepath.Join(imageDir, dir)
		if err := os.MkdirAll(destDir, mode); err != nil {
			return &DieError{Message: fmt.Sprintf("dodir: mkdir %s: %v", destDir, err)}
		}
	}

	return nil
}

// Keepdir creates directories with a .keep file to preserve empty dirs.
//
// Usage: keepdir /var/lib/myapp
//
// Creates the directory and adds a .keep_${CATEGORY}_${PN}-${SLOT} file.
func (h *Helpers) Keepdir(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "keepdir: no directories specified"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "keepdir: D not set"}
	}

	mode, err := parseMode(h.dirOpts)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("keepdir: %v", err)}
	}

	// Build keepfile name
	keepName := ".keep"
	if h.env != nil {
		category := h.env.CATEGORY
		pn := h.env.PN
		slot := h.env.SLOT
		if category != "" && pn != "" {
			keepName = fmt.Sprintf(".keep_%s_%s-%s", category, pn, slot)
		}
	}

	for _, dir := range args {
		destDir := filepath.Join(imageDir, dir)
		if err := os.MkdirAll(destDir, mode); err != nil {
			return &DieError{Message: fmt.Sprintf("keepdir: mkdir %s: %v", destDir, err)}
		}

		// Create .keep file
		keepFile := filepath.Join(destDir, keepName)
		f, err := os.Create(keepFile)
		if err != nil {
			return &DieError{Message: fmt.Sprintf("keepdir: create %s: %v", keepFile, err)}
		}
		_ = f.Close()
	}

	return nil
}

// ============================================================================
// EAPI 8 Config/Init Script Functions
// ============================================================================

// Doconfd installs files to /etc/conf.d.
//
// Usage: doconfd myapp.conf
func (h *Helpers) Doconfd(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "doconfd: no files specified"}
	}

	// Save current insDestTree, set to /etc/conf.d
	oldInsDestTree := h.insDestTree
	h.insDestTree = "/etc/conf.d"
	defer func() { h.insDestTree = oldInsDestTree }()

	return h.Doins(args)
}

// Doinitd installs files to /etc/init.d.
//
// Usage: doinitd myapp
func (h *Helpers) Doinitd(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "doinitd: no files specified"}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "doinitd: D not set"}
	}

	destDir := filepath.Join(imageDir, "etc", "init.d")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("doinitd: mkdir %s: %v", destDir, err)}
	}

	for _, file := range args {
		info, err := os.Stat(file)
		if err != nil {
			return &DieError{Message: fmt.Sprintf("doinitd: %s: %v", file, err)}
		}
		if info.IsDir() {
			return &DieError{Message: fmt.Sprintf("doinitd: %s is a directory", file)}
		}

		dst := filepath.Join(destDir, filepath.Base(file))
		if err := h.installFile(file, dst, 0755); err != nil {
			return &DieError{Message: fmt.Sprintf("doinitd: %v", err)}
		}
	}

	return nil
}

// Doenvd installs files to /etc/env.d.
//
// Usage: doenvd 99myapp
func (h *Helpers) Doenvd(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "doenvd: no files specified"}
	}

	// Save current insDestTree, set to /etc/env.d
	oldInsDestTree := h.insDestTree
	h.insDestTree = "/etc/env.d"
	defer func() { h.insDestTree = oldInsDestTree }()

	return h.Doins(args)
}

// ============================================================================
// EAPI 8 Symlink/Permission Functions
// ============================================================================

// Dosym creates a symbolic link.
//
// Usage: dosym target linkname
// Usage: dosym -r target linkname (EAPI 8: relative symlink)
//
// With -r flag (EAPI 8), calculates the relative path from linkname to target
// automatically. Both paths should be absolute paths within the image.
//
// Creates symlink ${D}/${linkname} -> target
func (h *Helpers) Dosym(args []string) error {
	relative := false
	argIdx := 0

	// Parse -r flag (EAPI 8 feature)
	if len(args) > 0 && args[0] == "-r" {
		relative = true
		argIdx = 1
	}

	if len(args) < argIdx+2 {
		return &DieError{Message: "dosym: requires target and linkname"}
	}

	target := args[argIdx]
	linkname := args[argIdx+1]

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "dosym: D not set"}
	}

	linkPath := filepath.Join(imageDir, linkname)

	// For relative symlinks, calculate the relative path from link to target
	if relative {
		target = calculateRelativePath(linkname, target)
	}

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
		return &DieError{Message: fmt.Sprintf("dosym: mkdir: %v", err)}
	}

	// Remove existing link/file
	_ = os.Remove(linkPath)

	// Create symlink
	if err := os.Symlink(target, linkPath); err != nil {
		return &DieError{Message: fmt.Sprintf("dosym: %v", err)}
	}

	return nil
}

// calculateRelativePath computes the relative path from linkPath to targetPath.
//
// Both paths should be absolute paths (starting with /).
// Returns a relative path that, when resolved from the link's directory,
// points to the target.
//
// Examples:
//   - linkPath="/usr/lib/libfoo.so", targetPath="/usr/lib/libfoo.so.1"
//     returns "libfoo.so.1"
//   - linkPath="/usr/lib/libfoo.so", targetPath="/usr/lib64/libfoo.so.1"
//     returns "../lib64/libfoo.so.1"
//   - linkPath="/usr/bin/python", targetPath="/usr/bin/python3.11"
//     returns "python3.11"
func calculateRelativePath(linkPath, targetPath string) string {
	// Clean both paths
	linkPath = filepath.Clean(linkPath)
	targetPath = filepath.Clean(targetPath)

	// Get the directory containing the link
	linkDir := filepath.Dir(linkPath)

	// Calculate relative path from link directory to target
	relPath, err := filepath.Rel(linkDir, targetPath)
	if err != nil {
		// If we can't compute relative path, return target as-is
		return targetPath
	}

	// On Windows, convert backslashes to forward slashes for consistency
	// (though GRPM targets Linux, this ensures tests work cross-platform)
	relPath = filepath.ToSlash(relPath)

	return relPath
}

// Fperms changes file permissions in ${D}.
//
// Usage: fperms 0755 /usr/bin/myapp
func (h *Helpers) Fperms(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "fperms: requires mode and path"}
	}

	modeStr := args[0]
	mode, err := strconv.ParseInt(modeStr, 8, 32)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("fperms: invalid mode: %s", modeStr)}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "fperms: D not set"}
	}

	for _, path := range args[1:] {
		fullPath := filepath.Join(imageDir, path)
		if err := os.Chmod(fullPath, os.FileMode(mode)); err != nil {
			return &DieError{Message: fmt.Sprintf("fperms: chmod %s: %v", path, err)}
		}
	}

	return nil
}

// Fowners changes file ownership in ${D}.
//
// Usage: fowners root:root /usr/bin/myapp
// Usage: fowners -R root:root /usr/share/myapp
//
// Per PMS Section 11.3.3.13:
//   - Changes ownership of files in ${D}
//   - Supports -R for recursive operation
//   - Owner:group can be names or numeric UID:GID
//
// Note: Skip with warning on non-Unix systems.
func (h *Helpers) Fowners(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "fowners: requires owner:group and path"}
	}

	// Check platform support (build-time detection via build tags)
	if !chownSupported() {
		h.writeStdout(">>> fowners: ownership changes not supported on this platform (skipping)\n")
		return nil
	}

	recursive := false
	startIdx := 0

	if args[0] == "-R" {
		recursive = true
		startIdx = 1
	}

	if len(args) < startIdx+2 {
		return &DieError{Message: "fowners: requires owner:group and path"}
	}

	ownerGroup := args[startIdx]
	paths := args[startIdx+1:]

	// Parse owner:group into UID/GID
	uid, gid, err := parseOwnerGroup(ownerGroup)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("fowners: %v", err)}
	}

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "fowners: D not set"}
	}

	for _, path := range paths {
		// Prefix path with ${D}, per Portage behavior
		fullPath := filepath.Join(imageDir, path)

		if err := chownPath(fullPath, uid, gid, recursive); err != nil {
			return &DieError{Message: fmt.Sprintf("fowners: chown %s: %v", path, err)}
		}
	}

	h.writeStdout(fmt.Sprintf(">>> fowners: %s\n", ownerGroup))
	return nil
}

// ============================================================================
// EAPI 8 Strip Control Functions
// ============================================================================

// Dostrip controls which files get stripped during installation.
//
// Usage: dostrip <path>...         - Include paths in stripping
// Usage: dostrip -x <path>...      - Exclude paths from stripping
//
// Per PMS Section 11.3.3.17 (EAPI 8):
//   - Without -x, adds paths to the list of files to strip
//   - With -x, adds paths to the exclusion list (files that should NOT be stripped)
//   - Paths are relative to ${ED} (e.g., /usr/bin, /usr/lib/debug)
//
// The strip lists are tracked in the Helpers struct and can be queried by
// the installation system to determine which files should be stripped.
func (h *Helpers) Dostrip(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "dostrip: no paths specified"}
	}

	exclude := false
	paths := args

	// Check for -x flag
	if args[0] == "-x" {
		exclude = true
		paths = args[1:]
		if len(paths) < 1 {
			return &DieError{Message: "dostrip: no paths specified after -x"}
		}
	}

	for _, p := range paths {
		// Normalize path (ensure it starts with /)
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}

		if exclude {
			h.stripExclude = append(h.stripExclude, p)
		} else {
			h.stripInclude = append(h.stripInclude, p)
		}
	}

	return nil
}

// GetStripInclude returns the list of paths to include in stripping.
func (h *Helpers) GetStripInclude() []string {
	return h.stripInclude
}

// GetStripExclude returns the list of paths to exclude from stripping.
func (h *Helpers) GetStripExclude() []string {
	return h.stripExclude
}

// ShouldStrip determines whether a file at the given path should be stripped.
//
// Logic:
//   - If stripInclude is empty, default behavior applies (strip most binaries)
//   - If stripInclude is set, only files under those paths are stripped
//   - Files matching stripExclude are never stripped
//
// The path should be relative to ${ED} (e.g., /usr/bin/myapp).
func (h *Helpers) ShouldStrip(path string) bool {
	// Normalize path - use forward slashes for consistent comparison
	// (GRPM targets Linux, but tests may run on Windows)
	path = filepath.ToSlash(filepath.Clean(path))
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Check exclusion list first - exclusions always win
	for _, exclude := range h.stripExclude {
		exclude = filepath.ToSlash(filepath.Clean(exclude))
		if strings.HasPrefix(path, exclude) {
			return false
		}
	}

	// If include list is empty, default is to strip (unless excluded above)
	if len(h.stripInclude) == 0 {
		return true
	}

	// Check if path is under any include path
	for _, include := range h.stripInclude {
		include = filepath.ToSlash(filepath.Clean(include))
		if strings.HasPrefix(path, include) {
			return true
		}
	}

	// Not in include list
	return false
}
