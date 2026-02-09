// Package ebuild implements ebuild execution engine.
//
// This file provides EAPI 8 patching functions (eapply, eapply_user).
package ebuild

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
		// Expand glob patterns (bash may pass unexpanded globs when nullglob is off)
		var paths []string
		if strings.ContainsAny(patch, "*?[") {
			matches, err := filepath.Glob(patch)
			if err != nil || len(matches) == 0 {
				return &DieError{Message: fmt.Sprintf("eapply: %s: no matching files", patch)}
			}
			paths = matches
		} else {
			paths = []string{patch}
		}

		for _, p := range paths {
			info, err := os.Stat(p)
			if err != nil {
				return &DieError{Message: fmt.Sprintf("eapply: %s: %v", p, err)}
			}

			if info.IsDir() {
				// Apply all patches in directory
				if err := h.applyPatchDir(p, stripLevel, workDir); err != nil {
					return err
				}
			} else {
				// Apply single patch
				if err := h.applyPatchFile(p, stripLevel, workDir); err != nil {
					return err
				}
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
