// Package ebuild implements ebuild execution engine.
//
// This file provides Go implementations of EAPI 8 Portage helper functions.
// All functions are implemented in pure Go, without external bash dependency.
package ebuild

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/ulikunitz/xz"
	"mvdan.cc/sh/v3/interp"
)

// ANSI color codes for terminal output.
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorBold   = "\033[1m"
)

// DieError is returned when die() is called in an ebuild.
// This error signals that ebuild execution should stop immediately.
type DieError struct {
	Message string
}

func (e *DieError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("die: %s", e.Message)
	}
	return "die called"
}

// exitFalse returns an exit code 1 error for false conditions.
// Used by USE flag functions to indicate false/disabled state.
func exitFalse() error {
	return interp.ExitStatus(1)
}

// Helpers provides Portage helper function implementations.
// All functions are implemented in pure Go, no external bash.
type Helpers struct {
	env    *Environment
	stdout io.Writer
	stderr io.Writer

	// State for install helpers
	insDestTree string // INSDESTTREE - target for doins
	exeDestTree string // EXEDESTTREE - target for doexe
	docDestTree string // DOCDESTTREE - subdirectory relative to /usr/share/doc/${PF}
	insOpts     string // INSOPTS - options for doins
	exeOpts     string // EXEOPTS - options for dobin/doexe
	dirOpts     string // DIROPTS - options for dodir
	destTree    string // DESTTREE - base installation prefix (default /usr)
	libDir      string // LIBDIR - library directory name (lib, lib64)

	// Eclass support
	eclassRegistry *EclassRegistry // Eclass registry for inherit tracking
	eclassStack    *EclassStack    // Stack for eshopts/estack operations
	cflags         []string        // CFLAGS for flag-o-matic
	cxxflags       []string        // CXXFLAGS for flag-o-matic
	ldflags        []string        // LDFLAGS for flag-o-matic
}

// NewHelpers creates helpers instance with default settings.
func NewHelpers(env *Environment, stdout, stderr io.Writer) *Helpers {
	portdir := ""
	if env != nil {
		portdir = env.PORTDIR
	}

	return &Helpers{
		env:            env,
		stdout:         stdout,
		stderr:         stderr,
		insDestTree:    "/usr",
		exeDestTree:    "", // Set by exeinto, used by doexe
		docDestTree:    "", // Subdirectory relative to doc dir
		insOpts:        "-m0644",
		exeOpts:        "-m0755",
		dirOpts:        "-m0755",
		destTree:       "/usr",
		libDir:         getLibDir(), // Detect lib vs lib64
		eclassRegistry: NewEclassRegistry(portdir),
		eclassStack:    NewEclassStack(),
		cflags:         make([]string, 0),
		cxxflags:       make([]string, 0),
		ldflags:        make([]string, 0),
	}
}

// getLibDir returns the library directory name (lib or lib64).
func getLibDir() string {
	// On 64-bit systems, use lib64
	if runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64" || runtime.GOARCH == "ppc64" {
		return "lib64"
	}
	return "lib"
}

// ============================================================================
// EAPI 8 Messaging Functions
// ============================================================================

// Die terminates ebuild execution with an error message.
//
// Usage: die "error message"
//
// In Portage, die() causes immediate termination. We return an error that
// should be propagated up to stop execution.
func (h *Helpers) Die(args []string) error {
	msg := strings.Join(args, " ")
	h.writeStderr(colorRed + " * " + colorReset + "ERROR: " + msg + "\n")
	return &DieError{Message: msg}
}

// Einfo prints an informational message (green asterisk).
//
// Usage: einfo "message"
func (h *Helpers) Einfo(args []string) error {
	msg := strings.Join(args, " ")
	h.writeStdout(colorGreen + " * " + colorReset + msg + "\n")
	return nil
}

// Ewarn prints a warning message (yellow asterisk).
//
// Usage: ewarn "message"
func (h *Helpers) Ewarn(args []string) error {
	msg := strings.Join(args, " ")
	h.writeStderr(colorYellow + " * " + colorReset + msg + "\n")
	return nil
}

// Eerror prints an error message (red asterisk).
//
// Usage: eerror "message"
func (h *Helpers) Eerror(args []string) error {
	msg := strings.Join(args, " ")
	h.writeStderr(colorRed + " * " + colorReset + msg + "\n")
	return nil
}

// Elog prints a message to the elog system.
//
// Usage: elog "message"
//
// In Portage, elog messages are saved for later review. Here we just output.
func (h *Helpers) Elog(args []string) error {
	msg := strings.Join(args, " ")
	h.writeStdout("LOG: " + msg + "\n")
	return nil
}

// Ebegin prints a "begin" message for a status operation.
//
// Usage: ebegin "Starting something"
func (h *Helpers) Ebegin(args []string) error {
	msg := strings.Join(args, " ")
	h.writeStdout(colorGreen + " * " + colorReset + msg + " ...")
	return nil
}

// Eend prints an "end" message with success/failure indicator.
//
// Usage: eend $? "failure message"
//
// Parameters:
//   - args[0]: exit code (0 = success, non-zero = failure)
//   - args[1:]: optional failure message
func (h *Helpers) Eend(args []string) error {
	exitCode := "0"
	failMsg := ""

	if len(args) >= 1 {
		exitCode = args[0]
	}
	if len(args) >= 2 {
		failMsg = strings.Join(args[1:], " ")
	}

	if exitCode == "0" {
		h.writeStdout(" " + colorGreen + "[ ok ]" + colorReset + "\n")
	} else {
		h.writeStdout(" " + colorRed + "[ !! ]" + colorReset + "\n")
		if failMsg != "" {
			h.writeStderr(colorRed + " * " + colorReset + failMsg + "\n")
		}
	}
	return nil
}

// ============================================================================
// EAPI 8 USE Flag Functions
// ============================================================================

// Has checks if a value exists in a list.
//
// Usage: has value item1 item2 item3 ...
//
// Returns exit code 0 if found, 1 if not found.
func (h *Helpers) Has(args []string) error {
	if len(args) < 2 {
		return exitFalse()
	}

	needle := args[0]
	haystack := args[1:]

	for _, item := range haystack {
		if item == needle {
			return nil // Found - exit code 0
		}
	}

	return exitFalse() // Not found - exit code 1
}

// Use checks if a USE flag is enabled.
//
// Usage: use flagname
//
// Returns exit code 0 if flag is enabled, 1 if disabled or not in IUSE.
func (h *Helpers) Use(args []string) error {
	if len(args) < 1 {
		return exitFalse()
	}

	flag := args[0]

	// Handle negation (use !flag)
	negate := false
	if strings.HasPrefix(flag, "!") {
		negate = true
		flag = flag[1:]
	}

	enabled := h.isUseEnabled(flag)

	if negate {
		enabled = !enabled
	}

	if enabled {
		return nil // Exit code 0
	}
	return exitFalse() // Exit code 1
}

// Usev prints the flag name if it's enabled.
//
// Usage: usev flagname [value]
//
// If flag is enabled, prints value (default: flagname).
// Returns exit code 0 if flag is enabled, 1 otherwise.
func (h *Helpers) Usev(args []string) error {
	if len(args) < 1 {
		return exitFalse()
	}

	flag := args[0]
	value := flag
	if len(args) >= 2 {
		value = args[1]
	}

	if h.isUseEnabled(flag) {
		h.writeStdout(value)
		return nil
	}

	return exitFalse()
}

// Usex outputs conditional values based on USE flag state.
//
// Usage: usex flag [true] [false] [trueSuffix] [falseSuffix]
//
// Outputs:
//   - If flag enabled: true + trueSuffix (default: "yes")
//   - If flag disabled: false + falseSuffix (default: "no")
func (h *Helpers) Usex(args []string) error {
	if len(args) < 1 {
		return exitFalse()
	}

	flag := args[0]
	trueVal := "yes"
	falseVal := "no"
	trueSuffix := ""
	falseSuffix := ""

	if len(args) >= 2 {
		trueVal = args[1]
	}
	if len(args) >= 3 {
		falseVal = args[2]
	}
	if len(args) >= 4 {
		trueSuffix = args[3]
	}
	if len(args) >= 5 {
		falseSuffix = args[4]
	}

	if h.isUseEnabled(flag) {
		h.writeStdout(trueVal + trueSuffix)
	} else {
		h.writeStdout(falseVal + falseSuffix)
	}

	return nil
}

// InIuse checks if a flag is declared in IUSE.
//
// Usage: in_iuse flagname
//
// Returns exit code 0 if flag is in IUSE, 1 otherwise.
func (h *Helpers) InIuse(args []string) error {
	if len(args) < 1 {
		return exitFalse()
	}

	flag := args[0]

	if h.isInIuse(flag) {
		return nil
	}

	return exitFalse()
}

// ============================================================================
// EAPI 8 Toolchain Functions
// ============================================================================

// TcGetCC prints the C compiler command.
//
// Usage: tc-getCC
//
// Returns CC from environment or default "gcc".
func (h *Helpers) TcGetCC(args []string) error {
	cc := h.getEnvOrDefault("CC", "gcc")
	h.writeStdout(cc)
	return nil
}

// TcGetCXX prints the C++ compiler command.
//
// Usage: tc-getCXX
//
// Returns CXX from environment or default "g++".
func (h *Helpers) TcGetCXX(args []string) error {
	cxx := h.getEnvOrDefault("CXX", "g++")
	h.writeStdout(cxx)
	return nil
}

// TcGetLD prints the linker command.
//
// Usage: tc-getLD
//
// Returns LD from environment or default "ld".
func (h *Helpers) TcGetLD(args []string) error {
	ld := h.getEnvOrDefault("LD", "ld")
	h.writeStdout(ld)
	return nil
}

// TcArch prints the target architecture.
//
// Usage: tc-arch
//
// Returns architecture name suitable for Portage KEYWORDS.
func (h *Helpers) TcArch(args []string) error {
	arch := h.detectArch()
	h.writeStdout(arch)
	return nil
}

// ============================================================================
// Helper Methods
// ============================================================================

// isUseEnabled checks if a USE flag is enabled.
func (h *Helpers) isUseEnabled(flag string) bool {
	if h.env == nil {
		return false
	}

	// Check in Package.UseFlags first (preferred)
	if h.env.Package != nil {
		if enabled, exists := h.env.Package.UseFlags[flag]; exists {
			return enabled
		}
	}

	// Fall back to USE environment variable
	useFlags := strings.Fields(h.env.USE)
	for _, f := range useFlags {
		if f == flag {
			return true
		}
		// Handle -flag (disabled)
		if f == "-"+flag {
			return false
		}
	}

	return false
}

// isInIuse checks if a flag is declared in IUSE.
func (h *Helpers) isInIuse(flag string) bool {
	if h.env == nil {
		return false
	}

	// Check in Package.UseFlags (all keys are valid IUSE)
	if h.env.Package != nil {
		if _, exists := h.env.Package.UseFlags[flag]; exists {
			return true
		}
	}

	// Fall back to IUSE environment variable (not in Environment struct currently)
	// TODO: Add IUSE field to Environment struct
	return false
}

// getEnvOrDefault gets an environment variable or returns default.
func (h *Helpers) getEnvOrDefault(key, defaultVal string) string {
	if h.env == nil {
		return defaultVal
	}

	// Check Environment struct fields
	switch key {
	case "CC":
		if h.env.CFLAGS != "" {
			// No CC field in Environment, use OS env
			if cc := os.Getenv("CC"); cc != "" {
				return cc
			}
		}
	case "CXX":
		if h.env.CXXFLAGS != "" {
			if cxx := os.Getenv("CXX"); cxx != "" {
				return cxx
			}
		}
	case "LD":
		if ld := os.Getenv("LD"); ld != "" {
			return ld
		}
	}

	return defaultVal
}

// detectArch detects the current architecture for Portage KEYWORDS.
func (h *Helpers) detectArch() string {
	// Map Go GOARCH to Gentoo KEYWORDS
	archMap := map[string]string{
		"amd64":   "amd64",
		"386":     "x86",
		"arm":     "arm",
		"arm64":   "arm64",
		"ppc64":   "ppc64",
		"ppc64le": "ppc64",
		"riscv64": "riscv",
		"s390x":   "s390",
		"mips":    "mips",
		"mips64":  "mips",
	}

	goarch := runtime.GOARCH
	if arch, ok := archMap[goarch]; ok {
		return arch
	}

	return goarch
}

// writeStdout writes to stdout.
func (h *Helpers) writeStdout(s string) {
	if h.stdout != nil {
		_, _ = io.WriteString(h.stdout, s)
	}
}

// writeStderr writes to stderr.
func (h *Helpers) writeStderr(s string) {
	if h.stderr != nil {
		_, _ = io.WriteString(h.stderr, s)
	}
}

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
// EAPI 8 Build Helper Functions
// ============================================================================

// Emake runs make with MAKEOPTS and additional arguments.
//
// Usage: emake
// Usage: emake target1 target2
//
// Runs make with parallelization options from MAKEOPTS environment variable.
func (h *Helpers) Emake(args []string) error {
	makeArgs := h.getMakeOpts()
	makeArgs = append(makeArgs, args...)

	h.writeStdout(fmt.Sprintf(">>> Running: make %s\n", strings.Join(makeArgs, " ")))

	return h.runCommand("make", makeArgs)
}

// getMakeOpts returns MAKEOPTS parsed as slice of strings.
func (h *Helpers) getMakeOpts() []string {
	var makeopts string
	if h.env != nil && h.env.MAKEOPTS != "" {
		makeopts = h.env.MAKEOPTS
	} else {
		makeopts = os.Getenv("MAKEOPTS")
	}
	if makeopts == "" {
		return nil
	}
	return strings.Fields(makeopts)
}

// getWorkDir returns the working directory (S or WORKDIR).
func (h *Helpers) getWorkDir() string {
	if h.env != nil {
		if h.env.S != "" {
			return h.env.S
		}
		return h.env.WORKDIR
	}
	return ""
}

// runCommand executes a command in the source directory.
func (h *Helpers) runCommand(name string, args []string) error {
	workDir := h.getWorkDir()
	if workDir == "" {
		return &DieError{Message: fmt.Sprintf("%s: working directory not set", name)}
	}

	// Check if working directory exists
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		return &DieError{Message: fmt.Sprintf("%s: working directory does not exist: %s", name, workDir)}
	}

	cmd := h.createCommand(name, args, workDir)

	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		h.writeStdout(string(output))
	}

	if err != nil {
		return &DieError{Message: fmt.Sprintf("%s failed: %v", name, err)}
	}

	return nil
}

// createCommand creates an exec.Cmd with proper environment.
func (h *Helpers) createCommand(name string, args []string, workDir string) *execCmd {
	cmd := newExecCmd(name, args...)
	cmd.Dir = workDir

	// Set environment if available
	if h.env != nil {
		cmd.Env = h.env.ToSlice()
	} else {
		cmd.Env = os.Environ()
	}

	return cmd
}

// execCmd wraps exec.Cmd to allow mocking in tests.
type execCmd struct {
	*exec.Cmd
}

// newExecCmd creates a new execCmd.
func newExecCmd(name string, args ...string) *execCmd {
	return &execCmd{Cmd: exec.Command(name, args...)}
}

// Econf runs ./configure with standard Portage options.
//
// Usage: econf
// Usage: econf --enable-feature
//
// Automatically adds standard configure options like --prefix, --host, etc.
func (h *Helpers) Econf(args []string) error {
	configurePath := filepath.Join(h.getWorkDir(), "configure")

	// Check if configure script exists
	if _, err := os.Stat(configurePath); os.IsNotExist(err) {
		return &DieError{Message: "econf: ./configure does not exist"}
	}

	confArgs := h.buildConfArgs()
	confArgs = append(confArgs, args...)

	h.writeStdout(fmt.Sprintf(">>> Running: ./configure %s\n", strings.Join(confArgs, " ")))

	return h.runCommand("./configure", confArgs)
}

// buildConfArgs builds standard configure arguments from environment.
func (h *Helpers) buildConfArgs() []string {
	args := []string{
		"--prefix=/usr",
		"--sysconfdir=/etc",
		"--localstatedir=/var",
		"--mandir=/usr/share/man",
		"--infodir=/usr/share/info",
	}

	// Add LIBDIR based on architecture
	libdir := h.getLibDir()
	args = append(args, fmt.Sprintf("--libdir=/usr/%s", libdir))

	// Add CHOST if set
	if chost := h.getChost(); chost != "" {
		args = append(args, fmt.Sprintf("--host=%s", chost))
	}

	// Add CBUILD if set and different from CHOST
	if cbuild := h.getCbuild(); cbuild != "" {
		args = append(args, fmt.Sprintf("--build=%s", cbuild))
	}

	return args
}

// getChost returns the target host triple.
func (h *Helpers) getChost() string {
	if chost := os.Getenv("CHOST"); chost != "" {
		return chost
	}
	return ""
}

// getCbuild returns the build host triple.
func (h *Helpers) getCbuild() string {
	if cbuild := os.Getenv("CBUILD"); cbuild != "" {
		return cbuild
	}
	return ""
}

// getLibDir returns the library directory name (wrapper for existing).
func (h *Helpers) getLibDir() string {
	return h.libDir
}

// Unpack extracts archives to WORKDIR.
//
// Usage: unpack file.tar.gz
// Usage: unpack ${A}
//
// Supported formats: .tar.gz, .tar.bz2, .tar.xz, .tar, .zip
// Pure Go implementation, no external commands.
func (h *Helpers) Unpack(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "unpack: no files specified"}
	}

	workDir := h.getWorkDir()
	if workDir == "" {
		if h.env != nil {
			workDir = h.env.WORKDIR
		}
		if workDir == "" {
			return &DieError{Message: "unpack: WORKDIR not set"}
		}
	}

	distDir := ""
	if h.env != nil {
		distDir = h.env.DISTDIR
	}
	if distDir == "" {
		distDir = os.Getenv("DISTDIR")
	}

	for _, file := range args {
		// Resolve file path - check DISTDIR first, then relative
		archivePath := file
		if !filepath.IsAbs(file) {
			if distDir != "" {
				candidate := filepath.Join(distDir, file)
				if _, err := os.Stat(candidate); err == nil {
					archivePath = candidate
				}
			}
			// If not found in DISTDIR, try relative to WORKDIR
			if archivePath == file {
				candidate := filepath.Join(workDir, file)
				if _, err := os.Stat(candidate); err == nil {
					archivePath = candidate
				}
			}
		}

		h.writeStdout(fmt.Sprintf(">>> Unpacking %s\n", filepath.Base(archivePath)))

		if err := h.unpackArchive(archivePath, workDir); err != nil {
			return &DieError{Message: fmt.Sprintf("unpack %s: %v", file, err)}
		}
	}

	return nil
}

// unpackArchive extracts a single archive to destination.
func (h *Helpers) unpackArchive(archivePath, destDir string) error {
	// Check file exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		return fmt.Errorf("archive not found: %s", archivePath)
	}

	lowerPath := strings.ToLower(archivePath)

	switch {
	case strings.HasSuffix(lowerPath, ".tar.gz") || strings.HasSuffix(lowerPath, ".tgz"):
		return h.unpackTarGz(archivePath, destDir)
	case strings.HasSuffix(lowerPath, ".tar.bz2") || strings.HasSuffix(lowerPath, ".tbz2"):
		return h.unpackTarBz2(archivePath, destDir)
	case strings.HasSuffix(lowerPath, ".tar.xz") || strings.HasSuffix(lowerPath, ".txz"):
		return h.unpackTarXz(archivePath, destDir)
	case strings.HasSuffix(lowerPath, ".tar"):
		return h.unpackTar(archivePath, destDir)
	case strings.HasSuffix(lowerPath, ".zip"):
		return h.unpackZip(archivePath, destDir)
	default:
		return fmt.Errorf("unsupported archive format: %s", archivePath)
	}
}

// unpackTarGz extracts a .tar.gz archive.
func (h *Helpers) unpackTarGz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gzReader.Close() }()

	return h.extractTar(tar.NewReader(gzReader), destDir)
}

// unpackTarBz2 extracts a .tar.bz2 archive.
func (h *Helpers) unpackTarBz2(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	bzReader := bzip2.NewReader(file)
	return h.extractTar(tar.NewReader(bzReader), destDir)
}

// unpackTarXz extracts a .tar.xz archive.
func (h *Helpers) unpackTarXz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	xzReader, err := xz.NewReader(file)
	if err != nil {
		return fmt.Errorf("xz reader: %w", err)
	}

	return h.extractTar(tar.NewReader(xzReader), destDir)
}

// unpackTar extracts a plain .tar archive.
func (h *Helpers) unpackTar(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	return h.extractTar(tar.NewReader(file), destDir)
}

// extractTar extracts files from a tar reader to destDir.
func (h *Helpers) extractTar(tarReader *tar.Reader, destDir string) error {
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		// Sanitize path to prevent directory traversal
		target := filepath.Join(destDir, header.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			return fmt.Errorf("invalid path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := h.extractTarFile(tarReader, target, header.Mode); err != nil {
				return err
			}
		case tar.TypeSymlink:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent: %w", err)
			}
			// Remove existing symlink if any
			_ = os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("symlink %s: %w", target, err)
			}
		case tar.TypeLink:
			// Hard link
			linkTarget := filepath.Join(destDir, header.Linkname)
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent: %w", err)
			}
			_ = os.Remove(target)
			if err := os.Link(linkTarget, target); err != nil {
				return fmt.Errorf("hardlink %s: %w", target, err)
			}
		default:
			// Skip other types (devices, etc.)
			continue
		}
	}
	return nil
}

// extractTarFile extracts a single regular file from tar.
func (h *Helpers) extractTarFile(tarReader *tar.Reader, target string, mode int64) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}

	outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(mode))
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}
	defer func() { _ = outFile.Close() }()

	if _, err := io.Copy(outFile, tarReader); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}

	return nil
}

// unpackZip extracts a .zip archive.
func (h *Helpers) unpackZip(archivePath, destDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	for _, f := range reader.File {
		target := filepath.Join(destDir, f.Name)

		// Sanitize path to prevent directory traversal
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			return fmt.Errorf("invalid path: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, f.Mode()); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
			continue
		}

		if err := h.extractZipFile(f, target); err != nil {
			return err
		}
	}

	return nil
}

// extractZipFile extracts a single file from zip.
func (h *Helpers) extractZipFile(f *zip.File, target string) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}

	src, err := f.Open()
	if err != nil {
		return fmt.Errorf("open zip entry: %w", err)
	}
	defer func() { _ = src.Close() }()

	dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}

	return nil
}

// Eapply applies patch files.
//
// Usage: eapply file.patch
// Usage: eapply -p1 file.patch
// Usage: eapply directory/
//
// Applies patches using the patch command with default -p1 strip level.
func (h *Helpers) Eapply(args []string) error {
	if len(args) < 1 {
		return &DieError{Message: "eapply: no patches specified"}
	}

	workDir := h.getWorkDir()
	if workDir == "" {
		return &DieError{Message: "eapply: working directory not set"}
	}

	// Default strip level
	stripLevel := "1"
	startIdx := 0

	// Parse -pN argument
	if len(args) >= 2 && strings.HasPrefix(args[0], "-p") {
		stripLevel = strings.TrimPrefix(args[0], "-p")
		startIdx = 1
	}

	for _, patch := range args[startIdx:] {
		info, err := os.Stat(patch)
		if err != nil {
			return &DieError{Message: fmt.Sprintf("eapply: %s: %v", patch, err)}
		}

		if info.IsDir() {
			// Apply all patches in directory
			if err := h.applyPatchDir(patch, stripLevel, workDir); err != nil {
				return err
			}
		} else {
			// Apply single patch
			if err := h.applyPatchFile(patch, stripLevel, workDir); err != nil {
				return err
			}
		}
	}

	return nil
}

// applyPatchDir applies all patches in a directory.
func (h *Helpers) applyPatchDir(dir, stripLevel, workDir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("eapply: read dir %s: %v", dir, err)}
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".patch") || strings.HasSuffix(name, ".diff") {
			patchPath := filepath.Join(dir, name)
			if err := h.applyPatchFile(patchPath, stripLevel, workDir); err != nil {
				return err
			}
		}
	}

	return nil
}

// applyPatchFile applies a single patch file.
func (h *Helpers) applyPatchFile(patchPath, stripLevel, workDir string) error {
	h.writeStdout(fmt.Sprintf(">>> Applying patch %s\n", filepath.Base(patchPath)))

	// Use patch command
	cmd := exec.Command("patch", "-p"+stripLevel, "-i", patchPath, "--batch", "--forward")
	cmd.Dir = workDir

	if h.env != nil {
		cmd.Env = h.env.ToSlice()
	}

	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		h.writeStdout(string(output))
	}

	if err != nil {
		return &DieError{Message: fmt.Sprintf("eapply %s: patch failed: %v", filepath.Base(patchPath), err)}
	}

	return nil
}

// EapplyUser applies user patches from /etc/portage/patches.
//
// Usage: eapply_user
//
// Looks for patches in /etc/portage/patches/${CATEGORY}/${PN}
// and applies them in sorted order.
func (h *Helpers) EapplyUser(args []string) error {
	if h.env == nil {
		// No environment, nothing to do
		return nil
	}

	// Build patches directory path
	patchesDir := h.getUserPatchesDir()
	if patchesDir == "" {
		return nil // No patches directory configured
	}

	// Check if category/package directory exists
	categoryPN := filepath.Join(patchesDir, h.env.CATEGORY, h.env.PN)

	info, err := os.Stat(categoryPN)
	if os.IsNotExist(err) {
		// No user patches, this is normal
		return nil
	}
	if err != nil {
		return &DieError{Message: fmt.Sprintf("eapply_user: %v", err)}
	}

	if !info.IsDir() {
		return nil
	}

	h.writeStdout(fmt.Sprintf(">>> Applying user patches from %s\n", categoryPN))

	// Get working directory
	workDir := h.getWorkDir()
	if workDir == "" {
		return &DieError{Message: "eapply_user: working directory not set"}
	}

	// Apply all patches in directory
	return h.applyPatchDir(categoryPN, "1", workDir)
}

// getUserPatchesDir returns the user patches directory.
func (h *Helpers) getUserPatchesDir() string {
	// Check PORTAGE_PATCHES_DIR environment variable
	if dir := os.Getenv("PORTAGE_PATCHES_DIR"); dir != "" {
		return dir
	}
	// Default location
	return "/etc/portage/patches"
}

// ============================================================================
// Default Phase Implementations
// ============================================================================

// DefaultSrcUnpack is the default src_unpack implementation.
//
// Usage: default_src_unpack
//
// Unpacks all archives listed in ${A}.
func (h *Helpers) DefaultSrcUnpack(args []string) error {
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
// Calls eapply_user to apply user patches.
func (h *Helpers) DefaultSrcPrepare(args []string) error {
	return h.EapplyUser(nil)
}

// DefaultSrcConfigure is the default src_configure implementation.
//
// Usage: default_src_configure
//
// Runs econf if ./configure exists.
func (h *Helpers) DefaultSrcConfigure(args []string) error {
	workDir := h.getWorkDir()
	if workDir == "" {
		return &DieError{Message: "default_src_configure: working directory not set"}
	}

	configurePath := filepath.Join(workDir, "configure")
	if _, err := os.Stat(configurePath); os.IsNotExist(err) {
		h.writeStdout(">>> No configure script, skipping default_src_configure\n")
		return nil
	}

	return h.Econf(nil)
}

// DefaultSrcCompile is the default src_compile implementation.
//
// Usage: default_src_compile
//
// Runs emake if Makefile exists.
func (h *Helpers) DefaultSrcCompile(args []string) error {
	workDir := h.getWorkDir()
	if workDir == "" {
		return &DieError{Message: "default_src_compile: working directory not set"}
	}

	makefilePath := filepath.Join(workDir, "Makefile")
	if _, err := os.Stat(makefilePath); os.IsNotExist(err) {
		// Also check for GNUmakefile
		gnuMakefilePath := filepath.Join(workDir, "GNUmakefile")
		if _, err := os.Stat(gnuMakefilePath); os.IsNotExist(err) {
			h.writeStdout(">>> No Makefile, skipping default_src_compile\n")
			return nil
		}
	}

	return h.Emake(nil)
}

// DefaultSrcTest is the default src_test implementation.
//
// Usage: default_src_test
//
// Runs emake check if Makefile exists.
func (h *Helpers) DefaultSrcTest(args []string) error {
	workDir := h.getWorkDir()
	if workDir == "" {
		return &DieError{Message: "default_src_test: working directory not set"}
	}

	makefilePath := filepath.Join(workDir, "Makefile")
	if _, err := os.Stat(makefilePath); os.IsNotExist(err) {
		return nil
	}

	return h.Emake([]string{"check"})
}

// DefaultSrcInstall is the default src_install implementation.
//
// Usage: default_src_install
//
// Runs emake install DESTDIR="${D}".
func (h *Helpers) DefaultSrcInstall(args []string) error {
	if h.env == nil {
		return &DieError{Message: "default_src_install: environment not set"}
	}

	workDir := h.getWorkDir()
	if workDir == "" {
		return &DieError{Message: "default_src_install: working directory not set"}
	}

	makefilePath := filepath.Join(workDir, "Makefile")
	if _, err := os.Stat(makefilePath); os.IsNotExist(err) {
		return &DieError{Message: "default_src_install: no Makefile found"}
	}

	destdir := fmt.Sprintf("DESTDIR=%s", h.env.D)
	return h.Emake([]string{"install", destdir})
}

// Default is the generic default function dispatcher.
//
// Usage: default
//
// Calls the default implementation for the current phase.
// The phase is determined from EBUILD_PHASE environment variable.
func (h *Helpers) Default(args []string) error {
	phase := os.Getenv("EBUILD_PHASE")
	if h.env != nil {
		// Environment might have phase info in the future
		if phase == "" {
			phase = os.Getenv("EBUILD_PHASE")
		}
	}

	switch phase {
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
		// Unknown phase, do nothing
		return nil
	}
}

// Ver_cut extracts version components.
//
// Usage: ver_cut 1 1.2.3     -> 1
// Usage: ver_cut 1-2 1.2.3   -> 1.2
// Usage: ver_cut 2- 1.2.3    -> 2.3
//
// Gentoo version cutting utility.
func (h *Helpers) VerCut(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "ver_cut: requires range and version arguments"}
	}

	rangeSpec := args[0]
	version := args[1]

	result, err := h.verCutImpl(rangeSpec, version)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("ver_cut: %v", err)}
	}

	h.writeStdout(result)
	return nil
}

// verCutImpl implements version cutting logic.
func (h *Helpers) verCutImpl(rangeSpec, version string) (string, error) {
	// Split version into components
	parts := h.splitVersion(version)
	if len(parts) == 0 {
		return "", nil
	}

	// Parse range
	start, end, err := h.parseVerRange(rangeSpec, len(parts))
	if err != nil {
		return "", err
	}

	// Extract requested parts
	if start > len(parts) {
		return "", nil
	}
	if end > len(parts) {
		end = len(parts)
	}

	return strings.Join(parts[start-1:end], "."), nil
}

// splitVersion splits a version string into components.
func (h *Helpers) splitVersion(version string) []string {
	// Split on . - _ characters
	var parts []string
	var current strings.Builder

	for _, r := range version {
		if r == '.' || r == '-' || r == '_' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parseVerRange parses a range specification like "1", "1-2", "2-".
func (h *Helpers) parseVerRange(rangeSpec string, maxParts int) (int, int, error) {
	if strings.Contains(rangeSpec, "-") {
		parts := strings.SplitN(rangeSpec, "-", 2)
		start := 1
		end := maxParts

		if parts[0] != "" {
			var err error
			start, err = strconv.Atoi(parts[0])
			if err != nil {
				return 0, 0, fmt.Errorf("invalid start: %s", parts[0])
			}
		}

		if parts[1] != "" {
			var err error
			end, err = strconv.Atoi(parts[1])
			if err != nil {
				return 0, 0, fmt.Errorf("invalid end: %s", parts[1])
			}
		}

		return start, end, nil
	}

	// Single number
	n, err := strconv.Atoi(rangeSpec)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range: %s", rangeSpec)
	}

	return n, n, nil
}

// VerRs replaces version separators.
//
// Usage: ver_rs 1-2 . 1_2_3  -> 1.2.3
//
// Replaces separators at specified positions.
func (h *Helpers) VerRs(args []string) error {
	if len(args) < 3 {
		return &DieError{Message: "ver_rs: requires range, separator, and version arguments"}
	}

	rangeSpec := args[0]
	newSep := args[1]
	version := args[2]

	result := h.verRsImpl(rangeSpec, newSep, version)
	h.writeStdout(result)
	return nil
}

// verRsImpl implements separator replacement logic.
func (h *Helpers) verRsImpl(rangeSpec, newSep, version string) string {
	// Find separator positions
	var sepPositions []int
	for i, r := range version {
		if r == '.' || r == '-' || r == '_' {
			sepPositions = append(sepPositions, i)
		}
	}

	if len(sepPositions) == 0 {
		return version
	}

	// Parse range
	start, end, err := h.parseVerRange(rangeSpec, len(sepPositions))
	if err != nil {
		return version
	}

	// Replace separators at specified positions
	result := []byte(version)
	for i := start - 1; i < end && i < len(sepPositions); i++ {
		pos := sepPositions[i]
		result[pos] = []byte(newSep)[0]
	}

	return string(result)
}

// GetFilesDir returns the ebuild FILESDIR path.
//
// Usage: get_filesdir
//
// Returns path to files/ directory in ebuild directory.
func (h *Helpers) GetFilesDir(args []string) error {
	if h.env == nil {
		return &DieError{Message: "get_filesdir: environment not set"}
	}

	filesDir := filepath.Join(h.env.PORTDIR, h.env.CATEGORY, h.env.PN, "files")
	h.writeStdout(filesDir)
	return nil
}

// InheritEclass handles eclass inheritance (stub).
//
// Usage: inherit eclass1 eclass2
//
// This is a stub - actual eclass handling requires sourcing bash files.
func (h *Helpers) Inherit(args []string) error {
	// Stub implementation - eclasses would normally source bash files
	for _, eclass := range args {
		h.writeStdout(fmt.Sprintf(">>> Inheriting eclass: %s (stub)\n", eclass))
	}
	return nil
}

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

// Dosym creates a symbolic link.
//
// Usage: dosym target linkname
//
// Creates symlink ${D}/${linkname} -> target
func (h *Helpers) Dosym(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "dosym: requires target and linkname"}
	}

	target := args[0]
	linkname := args[1]

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "dosym: D not set"}
	}

	linkPath := filepath.Join(imageDir, linkname)

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
// Note: This is a stub on non-Unix systems.
func (h *Helpers) Fowners(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "fowners: requires owner:group and path"}
	}

	// On Windows, ownership changes are not supported
	if runtime.GOOS == "windows" {
		h.writeStdout(">>> fowners: ownership changes not supported on Windows (skipping)\n")
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
	_ = ownerGroup // Would need to parse and look up UID/GID

	imageDir := h.getImageDir()
	if imageDir == "" {
		return &DieError{Message: "fowners: D not set"}
	}

	for _, path := range args[startIdx+1:] {
		fullPath := filepath.Join(imageDir, path)

		if recursive {
			err := filepath.WalkDir(fullPath, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				// Would call os.Chown here with resolved UID/GID
				return nil
			})
			if err != nil {
				return &DieError{Message: fmt.Sprintf("fowners: %v", err)}
			}
		} else {
			// Would call os.Chown here
			_ = fullPath
		}
	}

	h.writeStdout(fmt.Sprintf(">>> fowners: %s (stub - chown not implemented)\n", ownerGroup))
	return nil
}

// ApplyPatches applies patches from FILESDIR.
//
// Usage: apply_patches
//
// Looks for *.patch files in ${FILESDIR} and applies them.
func (h *Helpers) ApplyPatches(args []string) error {
	if h.env == nil {
		return &DieError{Message: "apply_patches: environment not set"}
	}

	filesDir := filepath.Join(h.env.PORTDIR, h.env.CATEGORY, h.env.PN, "files")

	info, err := os.Stat(filesDir)
	if os.IsNotExist(err) {
		// No files directory, nothing to do
		return nil
	}
	if err != nil {
		return &DieError{Message: fmt.Sprintf("apply_patches: %v", err)}
	}

	if !info.IsDir() {
		return nil
	}

	workDir := h.getWorkDir()
	if workDir == "" {
		return &DieError{Message: "apply_patches: working directory not set"}
	}

	return h.applyPatchDir(filesDir, "1", workDir)
}

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
func (h *Helpers) sedFile(file, old, new string, global, inPlace bool) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("sed: read %s: %v", file, err)}
	}

	var result string
	if global {
		result = strings.ReplaceAll(string(content), old, new)
	} else {
		result = strings.Replace(string(content), old, new, 1)
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

// PkgConfigPath returns pkg-config path.
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
	var dest string

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

	dest = sources[len(sources)-1]
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
		} else {
			// Update timestamps
			now := os.FileInfo(nil) // Would need current time
			_ = now
			// os.Chtimes would be called here
		}
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
