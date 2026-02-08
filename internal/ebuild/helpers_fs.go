// Package ebuild implements ebuild execution engine.
//
// This file provides filesystem utility functions (sed, cp, mv, rm, etc.).
package ebuild

import (
	"bufio"
	"bytes"
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
	if len(args) < 1 {
		return &DieError{Message: "sed: requires arguments"}
	}

	// Check for simple single-expression case: sed -i 's/old/new/g' file
	// For anything with -e, -n, -E, or complex expressions, delegate to external sed.
	hasComplexFlags := false
	for _, arg := range args {
		if arg == "-e" || arg == "-n" || arg == "-E" || arg == "-r" ||
			arg == "--regexp-extended" || strings.HasPrefix(arg, "--") {
			hasComplexFlags = true
			break
		}
	}

	if hasComplexFlags {
		return h.sedExternal(args)
	}

	// Try to parse simple case: [-i] 's/old/new/[g]' file...
	inPlace := false
	exprIdx := 0

	// Parse -i flag (with optional suffix like -i.bak)
	if len(args) > 0 && (args[0] == "-i" || strings.HasPrefix(args[0], "-i")) {
		inPlace = true
		if args[0] == "-i" {
			exprIdx = 1
		} else {
			// -i.bak style — ignore backup suffix
			exprIdx = 1
		}
	}

	if len(args) < exprIdx+2 {
		return h.sedExternal(args)
	}

	expression := args[exprIdx]
	files := args[exprIdx+1:]

	// Parse s/old/new/[flags] or s|old|new|[flags] expression
	if len(expression) < 2 || expression[0] != 's' {
		return h.sedExternal(args)
	}

	delim := expression[1]
	delimStr := string(delim)
	rest := expression[2:]

	parts := strings.SplitN(rest, delimStr, 3)
	if len(parts) < 2 {
		return h.sedExternal(args)
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

// sedExternal delegates sed to the system's sed command.
func (h *Helpers) sedExternal(args []string) error {
	cmd := exec.Command("sed", args...)
	if h.env != nil && h.env.S != "" {
		cmd.Dir = h.env.S
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if stdout.Len() > 0 {
		h.writeStdout(stdout.String())
	}
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return &DieError{Message: fmt.Sprintf("sed: %s", errMsg)}
		}
		return &DieError{Message: fmt.Sprintf("sed: %v", err)}
	}
	return nil
}

// sedFile performs sed substitution on a single file.
func (h *Helpers) sedFile(file, old, newStr string, global, inPlace bool) error {
	// Resolve relative paths against source directory
	if !filepath.IsAbs(file) {
		if h.env != nil && h.env.S != "" {
			file = filepath.Join(h.env.S, file)
		}
	}
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
		// No args = read from stdin. Since we don't have stdin access in
		// the command map handler, return success silently. The caller
		// may pipe through bash-level redirection which mvdan.cc/sh handles.
		return nil
	}

	for _, file := range args {
		if file == "-" {
			// "-" means stdin, skip silently
			continue
		}
		content, err := os.ReadFile(file)
		if err != nil {
			return &DieError{Message: fmt.Sprintf("cat: %s: %v", file, err)}
		}
		h.writeStdout(string(content))
	}

	return nil
}

// CatWithStdin reads from stdin when cat is called without file arguments.
func (h *Helpers) CatWithStdin(stdin io.Reader, args []string) error {
	if len(args) == 0 || (len(args) == 1 && args[0] == "-") {
		// Read from stdin
		if stdin != nil {
			data, err := io.ReadAll(stdin)
			if err != nil {
				return &DieError{Message: fmt.Sprintf("cat: reading stdin: %v", err)}
			}
			h.writeStdout(string(data))
		}
		return nil
	}

	for _, file := range args {
		if file == "-" {
			if stdin != nil {
				data, err := io.ReadAll(stdin)
				if err != nil {
					return &DieError{Message: fmt.Sprintf("cat: reading stdin: %v", err)}
				}
				h.writeStdout(string(data))
			}
			continue
		}
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
	endOfFlags := false

	for _, arg := range args {
		if endOfFlags {
			targets = append(targets, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-' {
			// Parse combined short flags like -rf, -Rf, -rfv
			for _, ch := range arg[1:] {
				switch ch {
				case 'r', 'R':
					recursive = true
				case 'f':
					force = true
				case 'v':
					// verbose - ignore
				}
			}
			continue
		}
		targets = append(targets, arg)
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
	force := false
	var targets []string
	endOfFlags := false

	for _, arg := range args {
		if endOfFlags {
			targets = append(targets, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			for _, ch := range arg[1:] {
				switch ch {
				case 'f':
					force = true
				case 'v', 'n', 'T':
					// ignore
				}
			}
			continue
		}
		targets = append(targets, arg)
	}
	_ = force

	if len(targets) < 2 {
		return &DieError{Message: "mv: requires source and destination"}
	}

	dst := targets[len(targets)-1]
	sources := targets[:len(targets)-1]

	for _, src := range sources {
		dest := dst
		// If destination is a directory, move into it
		if info, err := os.Stat(dest); err == nil && info.IsDir() {
			dest = filepath.Join(dest, filepath.Base(src))
		}

		if err := os.Rename(src, dest); err != nil {
			// Handle cross-device link: fall back to copy + remove
			if strings.Contains(err.Error(), "cross-device link") ||
				strings.Contains(err.Error(), "invalid cross-device link") {
				if cpErr := h.copyRecursive(src, dest); cpErr != nil {
					return &DieError{Message: fmt.Sprintf("mv: %v", cpErr)}
				}
				if rmErr := os.RemoveAll(src); rmErr != nil {
					return &DieError{Message: fmt.Sprintf("mv: remove source: %v", rmErr)}
				}
			} else {
				return &DieError{Message: fmt.Sprintf("mv: rename %s: %v", src, err)}
			}
		}
	}

	return nil
}

// copyRecursive copies a file or directory recursively.
func (h *Helpers) copyRecursive(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := h.copyRecursive(
				filepath.Join(src, entry.Name()),
				filepath.Join(dst, entry.Name()),
			); err != nil {
				return err
			}
		}
		return nil
	}

	// Copy regular file
	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, content, info.Mode())
}

// Chmod changes file permissions. Supports both octal and symbolic modes.
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

	for _, file := range args[modeIdx+1:] {
		if err := applyMode(file, modeStr, recursive); err != nil {
			return &DieError{Message: fmt.Sprintf("chmod: %s: %v", file, err)}
		}
	}

	return nil
}

// Ln creates links.
func (h *Helpers) Ln(args []string) error {
	symbolic := false
	force := false
	noDereference := false
	var targets []string
	endOfFlags := false

	for _, arg := range args {
		if endOfFlags {
			targets = append(targets, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-' {
			// Parse combined short flags like -snf, -sf, -fs
			for _, ch := range arg[1:] {
				switch ch {
				case 's':
					symbolic = true
				case 'f':
					force = true
				case 'n':
					noDereference = true
				case 'v':
					// verbose - ignore
				case 'r':
					// relative - ignore
				}
			}
			continue
		}
		targets = append(targets, arg)
	}
	_ = noDereference // Used for symlink behavior, not critical for our implementation

	if len(targets) < 1 {
		return &DieError{Message: "ln: requires at least a target"}
	}

	// Single target: ln -s /path/to/file → creates ./basename -> target
	if len(targets) == 1 {
		target := targets[0]
		linkName := filepath.Base(target)
		if force {
			_ = os.Remove(linkName)
		}
		if symbolic {
			return os.Symlink(target, linkName)
		}
		return os.Link(target, linkName)
	}

	// Multiple targets with directory: ln -s file1 file2 dir/
	last := targets[len(targets)-1]
	if info, err := os.Stat(last); err == nil && info.IsDir() && len(targets) > 2 {
		for _, target := range targets[:len(targets)-1] {
			linkName := filepath.Join(last, filepath.Base(target))
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
		}
		return nil
	}

	// Two targets: ln -s target linkname
	target := targets[0]
	linkName := targets[1]

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

// XargsOptions configures xargs behavior.
type XargsOptions struct {
	NullDelimiter bool   // -0: Use null byte as delimiter
	MaxArgs       int    // -n: Maximum arguments per command invocation
	NoRunIfEmpty  bool   // -r: Don't run command if stdin is empty
	Delimiter     string // -d: Custom delimiter
	Verbose       bool   // -t: Print commands before executing
}

// Xargs executes command with arguments from stdin.
//
// Usage: xargs [OPTIONS] COMMAND [INITIAL-ARGS]
//
// Reads arguments from stdin (typically piped from another command)
// and executes the specified command with those arguments.
//
// Options:
//
//	-0, --null           Items are separated by null byte, not whitespace
//	-n N, --max-args=N   Use at most N arguments per command line
//	-r, --no-run-if-empty  Don't run if stdin is empty
//	-d CHAR              Use CHAR as delimiter instead of whitespace
//	-t, --verbose        Print commands before executing
//
// This is called from XargsWithStdin which provides the stdin reader.
func (h *Helpers) Xargs(args []string) error {
	// This is a fallback when called without stdin context.
	// Real implementation is XargsWithStdin which gets stdin from interpreter.
	h.writeStderr(">>> xargs: warning: no stdin available (use in pipeline)\n")
	return nil
}

// XargsWithStdin executes xargs with provided stdin reader.
//
// This is the real implementation called by the interpreter when it
// can provide the stdin reader from the shell context.
func (h *Helpers) XargsWithStdin(stdin io.Reader, args []string) error {
	opts, cmdIdx := h.parseXargsOptions(args)

	// Get command and initial args
	if cmdIdx >= len(args) {
		// No command specified - default to echo
		return h.xargsExecute(stdin, opts, "echo", nil)
	}

	cmd := args[cmdIdx]
	var initialArgs []string
	if cmdIdx+1 < len(args) {
		initialArgs = args[cmdIdx+1:]
	}

	return h.xargsExecute(stdin, opts, cmd, initialArgs)
}

// parseXargsOptions parses xargs command line options.
// Returns options and index of first non-option argument (command).
func (h *Helpers) parseXargsOptions(args []string) (XargsOptions, int) {
	opts := XargsOptions{
		MaxArgs:   0,  // 0 means unlimited
		Delimiter: "", // empty means whitespace/newline
	}

	i := 0
	for i < len(args) {
		arg := args[i]
		advance, done := h.parseXargsSingleOption(&opts, args, i)
		i += advance
		if done {
			break
		}
		// If advance is 0, we hit a non-option (command)
		if advance == 0 && !strings.HasPrefix(arg, "-") {
			break
		}
	}

	return opts, i
}

// parseXargsSingleOption parses a single xargs option.
// Returns (advance count, should stop parsing).
func (h *Helpers) parseXargsSingleOption(opts *XargsOptions, args []string, i int) (int, bool) {
	if i >= len(args) {
		return 0, true
	}

	arg := args[i]

	// Simple boolean flags
	if arg == "-0" || arg == "--null" {
		opts.NullDelimiter = true
		return 1, false
	}
	if arg == "-r" || arg == "--no-run-if-empty" {
		opts.NoRunIfEmpty = true
		return 1, false
	}
	if arg == "-t" || arg == "--verbose" {
		opts.Verbose = true
		return 1, false
	}

	// Max args option
	if advance := h.parseXargsMaxArgs(opts, args, i); advance > 0 {
		return advance, false
	}

	// Delimiter option
	if advance := h.parseXargsDelimiter(opts, args, i); advance > 0 {
		return advance, false
	}

	// End of options marker
	if arg == "--" {
		return 1, true
	}

	// Unknown option - skip
	if strings.HasPrefix(arg, "-") {
		return 1, false
	}

	// Non-option found (command)
	return 0, true
}

// parseXargsMaxArgs parses -n / --max-args option.
// Returns advance count (0 if not a max-args option).
func (h *Helpers) parseXargsMaxArgs(opts *XargsOptions, args []string, i int) int {
	arg := args[i]

	if arg == "-n" || arg == "--max-args" {
		if i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err == nil && n > 0 {
				opts.MaxArgs = n
			}
			return 2
		}
		return 1
	}

	if strings.HasPrefix(arg, "--max-args=") {
		val := strings.TrimPrefix(arg, "--max-args=")
		n, err := strconv.Atoi(val)
		if err == nil && n > 0 {
			opts.MaxArgs = n
		}
		return 1
	}

	return 0
}

// parseXargsDelimiter parses -d / --delimiter option.
// Returns advance count (0 if not a delimiter option).
func (h *Helpers) parseXargsDelimiter(opts *XargsOptions, args []string, i int) int {
	arg := args[i]

	if arg == "-d" || arg == "--delimiter" {
		if i+1 < len(args) {
			opts.Delimiter = args[i+1]
			return 2
		}
		return 1
	}

	if strings.HasPrefix(arg, "-d") && len(arg) > 2 {
		// -dX format (delimiter immediately follows)
		opts.Delimiter = strings.TrimPrefix(arg, "-d")
		return 1
	}

	if strings.HasPrefix(arg, "--delimiter=") {
		opts.Delimiter = strings.TrimPrefix(arg, "--delimiter=")
		return 1
	}

	return 0
}

// xargsExecute reads input and executes command with batched arguments.
func (h *Helpers) xargsExecute(stdin io.Reader, opts XargsOptions, cmd string, initialArgs []string) error {
	// Read all input arguments
	inputArgs, err := h.xargsReadInput(stdin, opts)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("xargs: reading input: %v", err)}
	}

	// Check if we should run at all
	if len(inputArgs) == 0 && opts.NoRunIfEmpty {
		return nil
	}

	// Batch arguments and execute
	if opts.MaxArgs > 0 && len(inputArgs) > opts.MaxArgs {
		// Execute in batches
		for i := 0; i < len(inputArgs); i += opts.MaxArgs {
			end := i + opts.MaxArgs
			if end > len(inputArgs) {
				end = len(inputArgs)
			}
			batch := inputArgs[i:end]
			if err := h.xargsRunCommand(cmd, initialArgs, batch, opts.Verbose); err != nil {
				return err
			}
		}
	} else {
		// Execute all at once
		if err := h.xargsRunCommand(cmd, initialArgs, inputArgs, opts.Verbose); err != nil {
			return err
		}
	}

	return nil
}

// xargsReadInput reads arguments from stdin based on options.
func (h *Helpers) xargsReadInput(stdin io.Reader, opts XargsOptions) ([]string, error) {
	if stdin == nil {
		return nil, nil
	}

	var args []string

	if opts.NullDelimiter {
		// Read null-delimited input
		args = h.xargsReadNullDelimited(stdin)
	} else if opts.Delimiter != "" {
		// Read with custom delimiter
		args = h.xargsReadDelimited(stdin, opts.Delimiter)
	} else {
		// Read whitespace/newline delimited (default)
		args = h.xargsReadWhitespaceDelimited(stdin)
	}

	return args, nil
}

// xargsReadNullDelimited reads null-byte delimited input.
func (h *Helpers) xargsReadNullDelimited(stdin io.Reader) []string {
	var args []string
	scanner := bufio.NewScanner(stdin)
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		// Split on null byte
		for i := 0; i < len(data); i++ {
			if data[i] == 0 {
				return i + 1, data[:i], nil
			}
		}
		if atEOF && len(data) > 0 {
			return len(data), data, nil
		}
		return 0, nil, nil
	})

	for scanner.Scan() {
		text := scanner.Text()
		if text != "" {
			args = append(args, text)
		}
	}

	return args
}

// xargsReadDelimited reads input with custom delimiter.
func (h *Helpers) xargsReadDelimited(stdin io.Reader, delimiter string) []string {
	var args []string

	content, err := io.ReadAll(stdin)
	if err != nil {
		return nil
	}

	// Split by delimiter
	parts := strings.Split(string(content), delimiter)
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			args = append(args, trimmed)
		}
	}

	return args
}

// xargsReadWhitespaceDelimited reads whitespace/newline delimited input (default).
func (h *Helpers) xargsReadWhitespaceDelimited(stdin io.Reader) []string {
	var args []string
	scanner := bufio.NewScanner(stdin)

	for scanner.Scan() {
		line := scanner.Text()
		// Split line on whitespace
		fields := strings.Fields(line)
		args = append(args, fields...)
	}

	return args
}

// xargsRunCommand executes the command with given arguments.
func (h *Helpers) xargsRunCommand(cmd string, initialArgs, inputArgs []string, verbose bool) error {
	// Build full argument list
	fullArgs := make([]string, 0, len(initialArgs)+len(inputArgs))
	fullArgs = append(fullArgs, initialArgs...)
	fullArgs = append(fullArgs, inputArgs...)

	if verbose {
		// Print command before executing
		h.writeStderr(cmd + " " + strings.Join(fullArgs, " ") + "\n")
	}

	// Execute command
	execCmd := exec.Command(cmd, fullArgs...)
	if h.env != nil && h.env.S != "" {
		execCmd.Dir = h.env.S
	}

	// Set environment if available
	if h.env != nil {
		execCmd.Env = h.env.ToSlice()
	}

	output, err := execCmd.CombinedOutput()
	if len(output) > 0 {
		h.writeStdout(string(output))
	}

	if err != nil {
		return &DieError{Message: fmt.Sprintf("xargs: %s: %v", cmd, err)}
	}

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
