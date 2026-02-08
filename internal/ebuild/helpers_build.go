// Package ebuild implements ebuild execution engine.
//
// This file provides EAPI 8 build functions (emake, econf).
package ebuild

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
// getRuntimeDir returns the best working directory for command execution.
// Priority: runtimeDir (bash CWD from hc.Dir) > getWorkDir ($S/$WORKDIR).
func (h *Helpers) getRuntimeDir() string {
	if h.runtimeDir != "" {
		return h.runtimeDir
	}
	return h.getWorkDir()
}

// Checks the bash runtime environment first (runtimeEnv) for the latest $S,
// then falls back to the Go Environment struct.
func (h *Helpers) getWorkDir() string {
	// Check bash runtime env first — this has the authoritative $S value
	// set by the ebuild (e.g., S="${WORKDIR}/${MY_P}").
	if h.runtimeEnv != nil {
		if s := h.runtimeEnv.Get("S").String(); s != "" {
			return s
		}
		if w := h.runtimeEnv.Get("WORKDIR").String(); w != "" {
			return w
		}
	}
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
	workDir := h.getRuntimeDir()
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
// Per Portage phase-helpers.sh:514, uses ECONF_SOURCE (default ".")
// to locate the configure script. Automatically adds standard configure
// options like --prefix, --host, etc.
func (h *Helpers) Econf(args []string) error {
	// Per Portage: : ${ECONF_SOURCE:=.}
	econfSource := h.getEnvVar("ECONF_SOURCE")
	if econfSource == "" {
		econfSource = h.getRuntimeDir()
	}

	configurePath := filepath.Join(econfSource, "configure")

	// Check if configure script exists and is executable (per Portage line 545)
	info, err := os.Stat(configurePath)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("econf: %s does not exist in %s", "configure", econfSource)}
	}
	if info.Mode()&0111 == 0 {
		return &DieError{Message: "econf: configure is not executable"}
	}

	confArgs := h.buildConfArgs()
	confArgs = append(confArgs, args...)

	h.writeStdout(fmt.Sprintf(">>> Running: %s/configure %s\n", econfSource, strings.Join(confArgs, " ")))

	// Run configure from ECONF_SOURCE, not necessarily from $S
	cmd := h.createCommand(configurePath, confArgs, h.getRuntimeDir())
	output, cmdErr := cmd.CombinedOutput()
	if len(output) > 0 {
		h.writeStdout(string(output))
	}
	if cmdErr != nil {
		return &DieError{Message: "econf failed"}
	}
	return nil
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
	return h.getEnvVar("CHOST")
}

// getCbuild returns the build host triple.
func (h *Helpers) getCbuild() string {
	return h.getEnvVar("CBUILD")
}

// getLibDir returns the library directory name (wrapper for existing).
func (h *Helpers) getLibDir() string {
	return h.libDir
}
