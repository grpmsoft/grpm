package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// Phase represents installation phase.
type Phase int

const (
	// PhasePreInstall runs before package installation
	PhasePreInstall Phase = iota

	// PhasePostInstall runs after package installation
	PhasePostInstall

	// PhasePreRemove runs before package removal
	PhasePreRemove

	// PhasePostRemove runs after package removal
	PhasePostRemove
)

// String returns string representation of phase.
func (p Phase) String() string {
	switch p {
	case PhasePreInstall:
		return "pre-install"
	case PhasePostInstall:
		return "post-install"
	case PhasePreRemove:
		return "pre-remove"
	case PhasePostRemove:
		return "post-remove"
	default:
		return "unknown"
	}
}

// HookContext provides context for hook execution.
type HookContext struct {
	// Package being installed/removed
	Package *pkg.Package

	// Phase of installation
	Phase Phase

	// Root is the installation root
	Root string

	// Env contains environment variables
	Env map[string]string

	// Installer reference for progress reporting
	Installer *Installer
}

// Hook represents an installation hook.
//
// Hooks are executed at various phases of package installation/removal:
//   - Pre-install: before files are copied
//   - Post-install: after files are copied
//   - Pre-remove: before files are removed
//   - Post-remove: after files are removed
//
// Example hooks:
//   - ldconfig: update shared library cache
//   - update-desktop-database: update desktop entries
//   - update-mime-database: update MIME types
type Hook interface {
	// Name returns the hook name
	Name() string

	// ShouldRun checks if hook should run for this package/phase
	ShouldRun(ctx HookContext) bool

	// Run executes the hook
	Run(ctx HookContext) error
}

// LdconfigHook updates shared library cache after installation.
//
// This hook runs ldconfig to update /etc/ld.so.cache after installing
// libraries to /lib, /usr/lib, /lib64, /usr/lib64, etc.
type LdconfigHook struct{}

// Name returns hook name.
func (h *LdconfigHook) Name() string {
	return "ldconfig"
}

// ShouldRun checks if package installs any libraries.
func (h *LdconfigHook) ShouldRun(ctx HookContext) bool {
	// Only run after installation, not removal
	if ctx.Phase != PhasePostInstall {
		return false
	}

	// Check if package name suggests it's a library
	pkgName := ctx.Package.Name
	if filepath.Base(pkgName) == "lib" || filepath.Base(pkgName) == "glibc" {
		return true
	}

	// TODO: Check if package installed any .so files
	// For now, always run for safety
	return true
}

// Run executes ldconfig.
func (h *LdconfigHook) Run(ctx HookContext) error {
	ctx.Installer.progress("Running %s hook", h.Name())

	// Find ldconfig binary
	ldconfigPath, err := exec.LookPath("ldconfig")
	if err != nil {
		// ldconfig not found - skip silently
		return nil
	}

	// Run ldconfig with root parameter
	cmd := exec.Command(ldconfigPath)
	if ctx.Root != "/" {
		cmd.Args = append(cmd.Args, "-r", ctx.Root)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ldconfig failed: %w (output: %s)", err, output)
	}

	return nil
}

// UpdateDesktopDBHook updates desktop database after installation.
type UpdateDesktopDBHook struct{}

// Name returns hook name.
func (h *UpdateDesktopDBHook) Name() string {
	return "update-desktop-database"
}

// ShouldRun checks if package installs desktop files.
func (h *UpdateDesktopDBHook) ShouldRun(ctx HookContext) bool {
	// Only run after installation
	if ctx.Phase != PhasePostInstall && ctx.Phase != PhasePostRemove {
		return false
	}

	// TODO: Check if package installed .desktop files
	return false
}

// Run executes update-desktop-database.
func (h *UpdateDesktopDBHook) Run(ctx HookContext) error {
	ctx.Installer.progress("Running %s hook", h.Name())

	// Find update-desktop-database binary
	updateCmd, err := exec.LookPath("update-desktop-database")
	if err != nil {
		// Command not found - skip silently
		return nil
	}

	// Update desktop database
	desktopDir := filepath.Join(ctx.Root, "/usr/share/applications")
	if _, err := os.Stat(desktopDir); err != nil {
		// Directory doesn't exist - skip
		return nil
	}

	cmd := exec.Command(updateCmd, desktopDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("update-desktop-database failed: %w (output: %s)", err, output)
	}

	return nil
}

// UpdateMimeDBHook updates MIME database after installation.
type UpdateMimeDBHook struct{}

// Name returns hook name.
func (h *UpdateMimeDBHook) Name() string {
	return "update-mime-database"
}

// ShouldRun checks if package installs MIME files.
func (h *UpdateMimeDBHook) ShouldRun(ctx HookContext) bool {
	// Only run after installation
	if ctx.Phase != PhasePostInstall && ctx.Phase != PhasePostRemove {
		return false
	}

	// TODO: Check if package installed MIME files
	return false
}

// Run executes update-mime-database.
func (h *UpdateMimeDBHook) Run(ctx HookContext) error {
	ctx.Installer.progress("Running %s hook", h.Name())

	// Find update-mime-database binary
	updateCmd, err := exec.LookPath("update-mime-database")
	if err != nil {
		// Command not found - skip silently
		return nil
	}

	// Update MIME database
	mimeDir := filepath.Join(ctx.Root, "/usr/share/mime")
	if _, err := os.Stat(mimeDir); err != nil {
		// Directory doesn't exist - skip
		return nil
	}

	cmd := exec.Command(updateCmd, mimeDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("update-mime-database failed: %w (output: %s)", err, output)
	}

	return nil
}

// IconCacheHook updates icon cache after installation.
type IconCacheHook struct{}

// Name returns hook name.
func (h *IconCacheHook) Name() string {
	return "gtk-update-icon-cache"
}

// ShouldRun checks if package installs icons.
func (h *IconCacheHook) ShouldRun(ctx HookContext) bool {
	// Only run after installation
	if ctx.Phase != PhasePostInstall && ctx.Phase != PhasePostRemove {
		return false
	}

	// TODO: Check if package installed icon files
	return false
}

// Run executes gtk-update-icon-cache.
func (h *IconCacheHook) Run(ctx HookContext) error {
	ctx.Installer.progress("Running %s hook", h.Name())

	// Find gtk-update-icon-cache binary
	updateCmd, err := exec.LookPath("gtk-update-icon-cache")
	if err != nil {
		// Command not found - skip silently
		return nil
	}

	// Update icon cache for hicolor theme
	iconDir := filepath.Join(ctx.Root, "/usr/share/icons/hicolor")
	if _, err := os.Stat(iconDir); err != nil {
		// Directory doesn't exist - skip
		return nil
	}

	cmd := exec.Command(updateCmd, "-f", "-t", iconDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gtk-update-icon-cache failed: %w (output: %s)", err, output)
	}

	return nil
}
