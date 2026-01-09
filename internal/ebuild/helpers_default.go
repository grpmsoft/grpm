// Package ebuild implements ebuild execution engine.
//
// This file provides default phase implementations (default_src_unpack, etc.).
package ebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
// This allows ebuilds to call `default` in custom phase functions to
// invoke the default behavior (e.g., in src_configure: default calls econf).
func (h *Helpers) Default(args []string) error {
	// Try to get phase from environment struct first
	phase := ""
	if h.env != nil && h.env.EBUILD_PHASE != "" {
		phase = h.env.EBUILD_PHASE
	}

	// Fall back to OS environment variable
	if phase == "" {
		phase = os.Getenv("EBUILD_PHASE")
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
