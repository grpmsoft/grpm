// Package ebuild implements ebuild execution engine.
//
// This file provides filesystem utility functions (sed, cp, mv, rm, etc.).
package ebuild

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ============================================================================
// EAPI 8 Filesystem Utilities
// ============================================================================

// Sed runs sed on files in place.
//
// Usage: sed -i 's/old/new/g' file.txt
//
// Simple Go-based sed replacement for basic substitutions.
func (h *Helpers) Sed(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "sed: requires expression and file"}
	}

	inPlace := false
	exprIdx := 0

	// Parse -i flag
	if args[0] == "-i" {
		inPlace = true
		exprIdx = 1
	}

	if len(args) < exprIdx+2 {
		return &DieError{Message: "sed: requires expression and file"}
	}

	expression := args[exprIdx]
	files := args[exprIdx+1:]

	// Parse s/old/new/[flags] expression
	if !strings.HasPrefix(expression, "s/") {
		// Fall back to external sed for complex expressions
		cmd := exec.Command("sed", args...)
		if h.env != nil && h.env.S != "" {
			cmd.Dir = h.env.S
		}
		output, err := cmd.CombinedOutput()
		if len(output) > 0 {
			h.writeStdout(string(output))
		}
		if err != nil {
			return &DieError{Message: fmt.Sprintf("sed: %v", err)}
		}
		return nil
	}

	// Parse simple substitution
	parts := strings.Split(expression[2:], "/")
	if len(parts) < 2 {
		return &DieError{Message: fmt.Sprintf("sed: invalid expression: %s", expression)}
	}

	old := parts[0]
	newStr := parts[1]
	global := len(parts) >= 3 && strings.Contains(parts[2], "g")

	for _, file := range files {
		if err := h.sedFile(file, old, newStr, global, inPlace); err != nil {
			return err
		}
	}

	return nil
}

// sedFile performs sed substitution on a single file.
func (h *Helpers) sedFile(file, old, newStr string, global, inPlace bool) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("sed: read %s: %v", file, err)}
	}

	var result string
	if global {
		result = strings.ReplaceAll(string(content), old, newStr)
	} else {
		result = strings.Replace(string(content), old, newStr, 1)
	}

	if inPlace {
		if err := os.WriteFile(file, []byte(result), 0644); err != nil {
			return &DieError{Message: fmt.Sprintf("sed: write %s: %v", file, err)}
		}
	} else {
		h.writeStdout(result)
	}

	return nil
}

// PkgConfig returns pkg-config path.
//
// Usage: pkg-config --cflags zlib
//
// Wrapper for pkg-config command.
func (h *Helpers) PkgConfig(args []string) error {
	cmd := exec.Command("pkg-config", args...)
	if h.env != nil && h.env.S != "" {
		cmd.Dir = h.env.S
	}

	// Set PKG_CONFIG_PATH if needed
	if h.env != nil {
		cmd.Env = h.env.ToSlice()
	}

	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		h.writeStdout(string(output))
	}
	if err != nil {
		return &DieError{Message: fmt.Sprintf("pkg-config: %v", err)}
	}

	return nil
}

// Cat reads and outputs file contents (simple version).
func (h *Helpers) Cat(args []string) error {
	if len(args) < 1 {
		// Read from stdin - not supported in this context
		return &DieError{Message: "cat: no file specified"}
	}

	for _, file := range args {
		content, err := os.ReadFile(file)
		if err != nil {
			return &DieError{Message: fmt.Sprintf("cat: %s: %v", file, err)}
		}
		h.writeStdout(string(content))
	}

	return nil
}

// Mkdir creates directories.
func (h *Helpers) Mkdir(args []string) error {
	createParents := false
	mode := os.FileMode(0755)
	var dirs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-p" {
			createParents = true
		} else if arg == "-m" && i+1 < len(args) {
			i++
			m, err := strconv.ParseInt(args[i], 8, 32)
			if err != nil {
				return &DieError{Message: fmt.Sprintf("mkdir: invalid mode: %s", args[i])}
			}
			mode = os.FileMode(m)
		} else {
			dirs = append(dirs, arg)
		}
	}

	for _, dir := range dirs {
		var err error
		if createParents {
			err = os.MkdirAll(dir, mode)
		} else {
			err = os.Mkdir(dir, mode)
		}
		if err != nil {
			return &DieError{Message: fmt.Sprintf("mkdir: %s: %v", dir, err)}
		}
	}

	return nil
}

// Rm removes files and directories.
func (h *Helpers) Rm(args []string) error {
	recursive := false
	force := false
	var targets []string

	for _, arg := range args {
		switch arg {
		case "-r", "-R":
			recursive = true
		case "-rf", "-fr":
			recursive = true
			force = true
		case "-f":
			force = true
		default:
			targets = append(targets, arg)
		}
	}

	for _, target := range targets {
		var err error
		if recursive {
			err = os.RemoveAll(target)
		} else {
			err = os.Remove(target)
		}
		if err != nil && !force {
			return &DieError{Message: fmt.Sprintf("rm: %s: %v", target, err)}
		}
	}

	return nil
}

// Cp copies files.
func (h *Helpers) Cp(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "cp: requires source and destination"}
	}

	recursive := false
	preserve := false
	var sources []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-r", "-R":
			recursive = true
		case "-p", "-a":
			preserve = true
			recursive = true
		default:
			sources = append(sources, arg)
		}
	}

	if len(sources) < 2 {
		return &DieError{Message: "cp: requires source and destination"}
	}

	dest := sources[len(sources)-1]
	sources = sources[:len(sources)-1]

	// Unused - would be used for preserving timestamps
	_ = preserve

	for _, src := range sources {
		info, err := os.Stat(src)
		if err != nil {
			return &DieError{Message: fmt.Sprintf("cp: %s: %v", src, err)}
		}

		if info.IsDir() {
			if !recursive {
				return &DieError{Message: fmt.Sprintf("cp: %s is a directory (use -r)", src)}
			}
			if err := h.copyDir(src, filepath.Join(dest, filepath.Base(src))); err != nil {
				return &DieError{Message: fmt.Sprintf("cp: %v", err)}
			}
		} else {
			dstPath := dest
			if dstInfo, err := os.Stat(dest); err == nil && dstInfo.IsDir() {
				dstPath = filepath.Join(dest, filepath.Base(src))
			}
			if err := h.copyFile(src, dstPath); err != nil {
				return &DieError{Message: fmt.Sprintf("cp: %v", err)}
			}
		}
	}

	return nil
}

// copyFile copies a single file.
func (h *Helpers) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// copyDir recursively copies a directory.
func (h *Helpers) copyDir(src, dst string) error {
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

		return h.copyFile(path, dstPath)
	})
}

// Mv moves/renames files.
func (h *Helpers) Mv(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "mv: requires source and destination"}
	}

	src := args[len(args)-2]
	dst := args[len(args)-1]

	// If destination is a directory, move into it
	if info, err := os.Stat(dst); err == nil && info.IsDir() {
		dst = filepath.Join(dst, filepath.Base(src))
	}

	if err := os.Rename(src, dst); err != nil {
		return &DieError{Message: fmt.Sprintf("mv: %v", err)}
	}

	return nil
}

// Chmod changes file permissions.
func (h *Helpers) Chmod(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "chmod: requires mode and file"}
	}

	recursive := false
	modeIdx := 0

	if args[0] == "-R" {
		recursive = true
		modeIdx = 1
	}

	if len(args) < modeIdx+2 {
		return &DieError{Message: "chmod: requires mode and file"}
	}

	modeStr := args[modeIdx]
	mode, err := strconv.ParseInt(modeStr, 8, 32)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("chmod: invalid mode: %s", modeStr)}
	}

	for _, file := range args[modeIdx+1:] {
		if recursive {
			err := filepath.WalkDir(file, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				return os.Chmod(p, os.FileMode(mode))
			})
			if err != nil {
				return &DieError{Message: fmt.Sprintf("chmod: %v", err)}
			}
		} else {
			if err := os.Chmod(file, os.FileMode(mode)); err != nil {
				return &DieError{Message: fmt.Sprintf("chmod: %s: %v", file, err)}
			}
		}
	}

	return nil
}

// Ln creates links.
func (h *Helpers) Ln(args []string) error {
	symbolic := false
	force := false
	var sources []string

	for _, arg := range args {
		switch arg {
		case "-s":
			symbolic = true
		case "-f":
			force = true
		case "-sf", "-fs":
			symbolic = true
			force = true
		default:
			sources = append(sources, arg)
		}
	}

	if len(sources) < 2 {
		return &DieError{Message: "ln: requires target and link name"}
	}

	target := sources[0]
	linkName := sources[1]

	if force {
		_ = os.Remove(linkName)
	}

	if symbolic {
		if err := os.Symlink(target, linkName); err != nil {
			return &DieError{Message: fmt.Sprintf("ln: %v", err)}
		}
	} else {
		if err := os.Link(target, linkName); err != nil {
			return &DieError{Message: fmt.Sprintf("ln: %v", err)}
		}
	}

	return nil
}

// Find finds files (simple implementation).
func (h *Helpers) Find(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "find: requires path"}
	}

	path := args[0]
	namePattern := ""
	typeFilter := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-name":
			if i+1 < len(args) {
				i++
				namePattern = args[i]
			}
		case "-type":
			if i+1 < len(args) {
				i++
				typeFilter = args[i]
			}
		}
	}

	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Type filter
		if typeFilter != "" {
			switch typeFilter {
			case "f":
				if d.IsDir() {
					return nil
				}
			case "d":
				if !d.IsDir() {
					return nil
				}
			}
		}

		// Name pattern
		if namePattern != "" {
			matched, err := filepath.Match(namePattern, d.Name())
			if err != nil || !matched {
				return nil
			}
		}

		h.writeStdout(p + "\n")
		return nil
	})

	if err != nil {
		return &DieError{Message: fmt.Sprintf("find: %v", err)}
	}

	return nil
}

// Grep searches for patterns in files (simple implementation).
func (h *Helpers) Grep(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "grep: requires pattern and file"}
	}

	quiet := false
	invert := false
	pattern := ""
	var files []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-q":
			quiet = true
		case "-v":
			invert = true
		default:
			if pattern == "" {
				pattern = arg
			} else {
				files = append(files, arg)
			}
		}
	}

	found := false

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			matches := strings.Contains(line, pattern)
			if invert {
				matches = !matches
			}
			if matches {
				found = true
				if !quiet {
					h.writeStdout(line + "\n")
				}
			}
		}
		_ = f.Close()
	}

	if !found {
		return exitFalse()
	}

	return nil
}

// Xargs executes command with arguments from stdin (stub).
func (h *Helpers) Xargs(args []string) error {
	// Stub - would need to read from stdin and execute command
	h.writeStdout(">>> xargs: stub implementation\n")
	return nil
}

// Which finds commands in PATH.
func (h *Helpers) Which(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "which: requires command name"}
	}

	for _, cmd := range args {
		path, err := exec.LookPath(cmd)
		if err != nil {
			continue
		}
		h.writeStdout(path + "\n")
	}

	return nil
}

// Touch creates or updates file timestamps.
func (h *Helpers) Touch(args []string) error {
	for _, file := range args {
		// Check if file exists
		_, err := os.Stat(file)
		if os.IsNotExist(err) {
			// Create empty file
			f, err := os.Create(file)
			if err != nil {
				return &DieError{Message: fmt.Sprintf("touch: %s: %v", file, err)}
			}
			_ = f.Close()
		}
		// Update timestamps would use os.Chtimes but we skip for simplicity
	}

	return nil
}

// Install copies files with optional mode/owner.
func (h *Helpers) Install(args []string) error {
	mode := os.FileMode(0755)
	createDirs := false
	var sources []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-d":
			createDirs = true
		case "-m":
			if i+1 < len(args) {
				i++
				m, err := strconv.ParseInt(args[i], 8, 32)
				if err != nil {
					return &DieError{Message: fmt.Sprintf("install: invalid mode: %s", args[i])}
				}
				mode = os.FileMode(m)
			}
		default:
			sources = append(sources, arg)
		}
	}

	if createDirs {
		// Create directories
		for _, dir := range sources {
			if err := os.MkdirAll(dir, mode); err != nil {
				return &DieError{Message: fmt.Sprintf("install: %s: %v", dir, err)}
			}
		}
		return nil
	}

	if len(sources) < 2 {
		return &DieError{Message: "install: requires source and destination"}
	}

	dest := sources[len(sources)-1]
	sources = sources[:len(sources)-1]

	for _, src := range sources {
		dstPath := dest
		if info, err := os.Stat(dest); err == nil && info.IsDir() {
			dstPath = filepath.Join(dest, filepath.Base(src))
		}

		if err := h.installFile(src, dstPath, mode); err != nil {
			return &DieError{Message: fmt.Sprintf("install: %v", err)}
		}
	}

	return nil
}
