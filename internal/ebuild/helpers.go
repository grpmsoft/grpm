// Package ebuild implements ebuild execution engine.
//
// This file provides Go implementations of EAPI 8 Portage helper functions.
// All functions are implemented in pure Go, without external bash dependency.
//
// Helper functions are organized across multiple files:
//   - helpers.go (this file): Core types and initialization
//   - helpers_msg.go: Messaging functions (die, einfo, ewarn, etc.)
//   - helpers_use.go: USE flag functions (use, usev, usex, has, etc.)
//   - helpers_toolchain.go: Toolchain functions (tc-getCC, tc-arch, etc.)
//   - helpers_install.go: Installation functions (dobin, doins, dosym, etc.)
//   - helpers_doc.go: Documentation functions (dodoc, doman, etc.)
//   - helpers_build.go: Build functions (emake, econf)
//   - helpers_unpack.go: Archive extraction (unpack)
//   - helpers_patch.go: Patching functions (eapply, eapply_user)
//   - helpers_default.go: Default phase implementations
//   - helpers_version.go: Version manipulation (ver_cut, ver_rs)
//   - helpers_fs.go: Filesystem utilities (sed, cp, mv, etc.)
package ebuild

import (
	"fmt"
	"io"
	"runtime"

	"github.com/grpmsoft/grpm/internal/state"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
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
	eclassRegistry *EclassRegistry   // Eclass registry for inherit tracking
	eclassLoader   EclassLoaderIface // Eclass loader for inherit functionality (interface)
	eclassStack    *EclassStack      // Stack for eshopts/estack operations
	cflags         []string          // CFLAGS for flag-o-matic
	cxxflags       []string          // CXXFLAGS for flag-o-matic
	ldflags        []string          // LDFLAGS for flag-o-matic

	// Package database for has_version/best_version queries
	pkgDB *state.PackageDatabase

	// Strip control (EAPI 8)
	stripInclude []string // Paths to include in stripping
	stripExclude []string // Paths to exclude from stripping (-x flag)

	// Exit status tracking for assert command (PMS Section 12.3.6)
	lastExitStatus int   // Last command exit status ($?)
	pipeStatus     []int // Pipe status array (PIPESTATUS)

	// Nonfatal mode flag for nonfatal command (PMS Section 12.3.1)
	// When true, die() returns error instead of aborting the build.
	// EAPI 4+: nonfatal command sets this flag.
	nonfatalMode bool

	// Command dispatcher for nonfatal command execution.
	// Set by the interpreter to allow nonfatal to execute commands.
	commandDispatcher func(cmd string, args []string) error

	// runtimeEnv holds the current bash runner's environment during command dispatch.
	// Set by the exec handler before calling Go helpers, cleared after return.
	// This allows Go helpers to read variables set in the running bash script
	// (e.g., DISTUTILS_USE_PEP517) that aren't in the Environment struct.
	runtimeEnv expand.Environ

	// runtimeDir holds the current bash CWD during command dispatch.
	// Set by the exec handler from hc.Dir, cleared after return.
	// This is the actual working directory of the bash script (changed by cd).
	runtimeDir string
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
		libDir:         getLibDirDefault(), // Detect lib vs lib64
		eclassRegistry: NewEclassRegistry(portdir),
		eclassStack:    NewEclassStack(),
		cflags:         make([]string, 0),
		cxxflags:       make([]string, 0),
		ldflags:        make([]string, 0),
	}
}

// SetPackageDatabase sets the package database for has_version/best_version queries.
//
// This allows integration with the system's installed package database (VarDB).
// If not set, has_version and best_version will return "not installed" status.
func (h *Helpers) SetPackageDatabase(db *state.PackageDatabase) {
	h.pkgDB = db
}

// SetEclassLoader sets the eclass loader for inherit functionality.
//
// This must be called after creating both Helpers and Interpreter to resolve
// the circular dependency between them. The EclassLoader uses the Interpreter
// to execute eclass bash code.
//
// The loader can be either a *EclassLoader (legacy) or any type implementing
// EclassLoaderIface (for dynamic loading).
func (h *Helpers) SetEclassLoader(loader EclassLoaderIface) {
	h.eclassLoader = loader
}

// GetEclassLoader returns the eclass loader.
//
// Returns nil if no loader is set.
func (h *Helpers) GetEclassLoader() EclassLoaderIface {
	return h.eclassLoader
}

// getLibDirDefault returns the library directory name (lib or lib64).
// This is a package-level function to avoid conflicts with the method.
func getLibDirDefault() string {
	// On 64-bit systems, use lib64
	if runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64" || runtime.GOARCH == "ppc64" {
		return "lib64"
	}
	return "lib"
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

// SetLastExitStatus sets the last command exit status for assert command.
//
// This should be called by the interpreter after each command execution
// to track the exit status for subsequent assert calls.
func (h *Helpers) SetLastExitStatus(status int) {
	h.lastExitStatus = status
}

// GetLastExitStatus returns the last command exit status.
func (h *Helpers) GetLastExitStatus() int {
	return h.lastExitStatus
}

// SetPipeStatus sets the pipe status array for assert command.
//
// This should be called by the interpreter after pipeline execution
// to track the PIPESTATUS array for subsequent assert calls.
// Per PMS Section 12.3.6, assert checks PIPESTATUS for pipelines.
func (h *Helpers) SetPipeStatus(status []int) {
	h.pipeStatus = make([]int, len(status))
	copy(h.pipeStatus, status)
}

// GetPipeStatus returns the pipe status array.
func (h *Helpers) GetPipeStatus() []int {
	if h.pipeStatus == nil {
		return []int{h.lastExitStatus}
	}
	return h.pipeStatus
}

// SetNonfatalMode sets the nonfatal mode flag.
//
// When nonfatal mode is enabled, die() returns an error instead of
// aborting the build process. This is used by the nonfatal command.
// Per PMS Section 12.3.1: EAPI 4+ only.
func (h *Helpers) SetNonfatalMode(enabled bool) {
	h.nonfatalMode = enabled
}

// IsNonfatalMode returns true if nonfatal mode is currently active.
func (h *Helpers) IsNonfatalMode() bool {
	return h.nonfatalMode
}

// SetCommandDispatcher sets the function used to execute commands.
//
// This is called by the interpreter to wire up command execution for
// the nonfatal helper, which needs to execute commands through the
// interpreter's command dispatch mechanism.
func (h *Helpers) SetCommandDispatcher(dispatcher func(cmd string, args []string) error) {
	h.commandDispatcher = dispatcher
}
