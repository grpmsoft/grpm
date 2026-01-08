// Package install implements package installation engine.

package install

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/grpmsoft/grpm/internal/config"
)

// ConfigProtect handles configuration file protection.
//
// CONFIG_PROTECT prevents package updates from overwriting user-modified
// configuration files. When a protected file would be overwritten, the new
// version is installed with a ._cfg*_ prefix instead.
//
// CONFIG_PROTECT_MASK allows specific paths within protected areas to be
// updated normally (e.g., /etc/env.d within /etc).
//
// Example:
//
//	cp := NewConfigProtect()
//	if cp.ShouldProtect("/etc/foo.conf") {
//	    newPath := cp.GenerateProtectedName("/etc/foo.conf")
//	    // Install to newPath instead of /etc/foo.conf
//	}
type ConfigProtect struct {
	// Protected contains paths that should be protected (CONFIG_PROTECT).
	// Files in these directories won't be overwritten if modified.
	Protected []string

	// Masked contains paths within Protected that should NOT be protected
	// (CONFIG_PROTECT_MASK). These paths are updated normally.
	Masked []string

	// mu protects concurrent access to protected name generation.
	mu sync.Mutex
}

// ProtectedFile represents a configuration file conflict.
// When a protected file would be overwritten, the new version is installed
// with a ._cfg*_ prefix, creating a conflict that needs manual resolution.
type ProtectedFile struct {
	// Original is the target path (e.g., /etc/foo.conf).
	Original string

	// Protected is the actual installed path (e.g., /etc/._cfg0000_foo.conf).
	Protected string

	// Package is the package that owns this file (e.g., sys-apps/package-1.0).
	Package string
}

// NewConfigProtect creates a ConfigProtect with default Gentoo paths.
//
// Default protected paths:
//   - /etc (main configuration directory)
//   - /usr/share/config (KDE configuration)
//
// Default masked paths:
//   - /etc/env.d (environment configuration, auto-generated)
//   - /etc/gconf (GNOME configuration, auto-generated)
func NewConfigProtect() *ConfigProtect {
	return &ConfigProtect{
		Protected: []string{"/etc", "/usr/share/config"},
		Masked:    []string{"/etc/env.d", "/etc/gconf"},
	}
}

// LoadFromConfig loads CONFIG_PROTECT and CONFIG_PROTECT_MASK from make.conf.
//
// If the config values are empty, the default paths are preserved.
// Values from config completely replace defaults when non-empty.
//
// Example make.conf:
//
//	CONFIG_PROTECT="/etc /usr/share/config /opt/myapp/config"
//	CONFIG_PROTECT_MASK="/etc/env.d /etc/gconf /etc/sandbox.d"
func (cp *ConfigProtect) LoadFromConfig(cfg *config.Config) {
	if cfg == nil || cfg.MakeConf == nil {
		return
	}

	// Load CONFIG_PROTECT from FEATURES or dedicated field
	// For now, we check if there's a generic way to get values
	// Since Config doesn't expose raw values, we keep defaults
	// TODO: Add CONFIG_PROTECT/CONFIG_PROTECT_MASK to Config struct
}

// IsProtected checks if a path is within a CONFIG_PROTECT directory.
//
// A path is protected if it starts with any path in the Protected list.
// This uses prefix matching, so /etc/foo.conf matches /etc.
func (cp *ConfigProtect) IsProtected(path string) bool {
	// Normalize path for consistent comparison
	cleanPath := filepath.Clean(path)

	for _, protectedPath := range cp.Protected {
		cleanProtected := filepath.Clean(protectedPath)

		// Check if path starts with protected path
		// Either exact match or path is under protected directory
		if cleanPath == cleanProtected {
			return true
		}

		// Check if path is under protected directory
		if strings.HasPrefix(cleanPath, cleanProtected+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

// IsMasked checks if a path is in CONFIG_PROTECT_MASK.
//
// A path is masked if it starts with any path in the Masked list.
// Masked paths are NOT protected even if they're within a protected directory.
func (cp *ConfigProtect) IsMasked(path string) bool {
	cleanPath := filepath.Clean(path)

	for _, maskedPath := range cp.Masked {
		cleanMasked := filepath.Clean(maskedPath)

		if cleanPath == cleanMasked {
			return true
		}

		if strings.HasPrefix(cleanPath, cleanMasked+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

// ShouldProtect returns true if a file needs CONFIG_PROTECT handling.
//
// A file should be protected if:
//   - It is within a CONFIG_PROTECT path (IsProtected returns true)
//   - It is NOT within a CONFIG_PROTECT_MASK path (IsMasked returns false)
//
// Example:
//
//	/etc/foo.conf        -> true  (protected, not masked)
//	/etc/env.d/99local   -> false (protected but masked)
//	/usr/bin/foo         -> false (not protected)
func (cp *ConfigProtect) ShouldProtect(path string) bool {
	return cp.IsProtected(path) && !cp.IsMasked(path)
}

// GenerateProtectedName generates a ._cfg*_ filename for a protected file.
//
// Portage uses the format ._cfgNNNN_filename where NNNN is a 4-digit
// sequential number starting from 0000.
//
// Examples:
//
//	/etc/foo.conf -> /etc/._cfg0000_foo.conf (first conflict)
//	/etc/foo.conf -> /etc/._cfg0001_foo.conf (second conflict)
//
// The function finds the next available number by checking existing files.
// It is thread-safe for concurrent merge operations - the file is created
// atomically within the critical section to prevent race conditions.
//
// Returns empty string if more than 10000 conflicts exist (unlikely scenario).
func (cp *ConfigProtect) GenerateProtectedName(path string) string {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	dir := filepath.Dir(path)
	base := filepath.Base(path)

	// Find next available number
	for i := 0; i < 10000; i++ {
		name := fmt.Sprintf("._cfg%04d_%s", i, base)
		candidate := filepath.Join(dir, name)

		// Use O_EXCL to atomically check and create - prevents race conditions
		f, err := os.OpenFile(candidate, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			_ = f.Close()
			return candidate
		}
		// File exists or other error, try next number
	}

	// Too many conflicts - should never happen in practice
	return ""
}

// FindExistingConfigs finds all ._cfg*_ files for a given path.
//
// This is useful for tools like etc-update that need to find all
// pending configuration updates.
//
// Returns paths sorted by number (._cfg0000_* before ._cfg0001_*).
// Returns empty slice if no conflicts exist.
//
// Example:
//
//	configs, err := cp.FindExistingConfigs("/etc/foo.conf")
//	// Returns: ["/etc/._cfg0000_foo.conf", "/etc/._cfg0001_foo.conf"]
func (cp *ConfigProtect) FindExistingConfigs(path string) ([]string, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	var configs []string
	prefix := "._cfg"
	suffix := "_" + base

	for _, entry := range entries {
		name := entry.Name()

		// Check if matches pattern: ._cfgNNNN_basename
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
			// Verify the middle part is a 4-digit number
			middle := name[len(prefix) : len(name)-len(suffix)]
			if len(middle) == 4 && isDigits(middle) {
				configs = append(configs, filepath.Join(dir, name))
			}
		}
	}

	// Sort by filename (which sorts by number due to zero-padding)
	sort.Strings(configs)

	return configs, nil
}

// isDigits checks if a string contains only digits.
func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// CompareFiles compares two files and returns true if they are identical.
//
// Uses SHA256 checksums for comparison. Returns false if either file
// cannot be read or if their contents differ.
func CompareFiles(path1, path2 string) (bool, error) {
	hash1, err := calculateFileHash(path1)
	if err != nil {
		return false, fmt.Errorf("hashing %s: %w", path1, err)
	}

	hash2, err := calculateFileHash(path2)
	if err != nil {
		return false, fmt.Errorf("hashing %s: %w", path2, err)
	}

	return hash1 == hash2, nil
}

// calculateFileHash computes SHA256 hash of a file.
func calculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// AddProtected adds a path to the protected list.
//
// This is useful for adding custom protected paths from configuration.
func (cp *ConfigProtect) AddProtected(path string) {
	cp.Protected = append(cp.Protected, filepath.Clean(path))
}

// AddMasked adds a path to the masked list.
//
// This is useful for adding custom masked paths from configuration.
func (cp *ConfigProtect) AddMasked(path string) {
	cp.Masked = append(cp.Masked, filepath.Clean(path))
}

// SetProtected replaces the protected paths list.
//
// Paths are cleaned before being stored.
func (cp *ConfigProtect) SetProtected(paths []string) {
	cp.Protected = make([]string, len(paths))
	for i, p := range paths {
		cp.Protected[i] = filepath.Clean(p)
	}
}

// SetMasked replaces the masked paths list.
//
// Paths are cleaned before being stored.
func (cp *ConfigProtect) SetMasked(paths []string) {
	cp.Masked = make([]string, len(paths))
	for i, p := range paths {
		cp.Masked[i] = filepath.Clean(p)
	}
}

// GetProtectedCount returns the number of protected paths.
func (cp *ConfigProtect) GetProtectedCount() int {
	return len(cp.Protected)
}

// GetMaskedCount returns the number of masked paths.
func (cp *ConfigProtect) GetMaskedCount() int {
	return len(cp.Masked)
}
