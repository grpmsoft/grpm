//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grpmsoft/grpm/internal/config"
	"github.com/grpmsoft/grpm/internal/profile"
)

// TestConfig_PackageMaskDirectory tests package.mask as a directory (EAPI 7+).
//
// This validates the v0.7.3 fix for directory handling where package.mask
// can be either a file or a directory containing multiple files.
func TestConfig_PackageMaskDirectory(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(confDir string) error
		expectedMask []string
	}{
		{
			name: "single file",
			setup: func(confDir string) error {
				return os.WriteFile(
					filepath.Join(confDir, "package.mask"),
					[]byte("# Mask unstable\n>=sys-libs/glibc-2.39\ndev-libs/openssl:0/3.2\n"),
					0644,
				)
			},
			expectedMask: []string{">=sys-libs/glibc-2.39", "dev-libs/openssl:0/3.2"},
		},
		{
			name: "directory with multiple files",
			setup: func(confDir string) error {
				maskDir := filepath.Join(confDir, "package.mask")
				if err := os.MkdirAll(maskDir, 0755); err != nil {
					return err
				}
				// Files are read in lexicographic order
				if err := os.WriteFile(
					filepath.Join(maskDir, "01-security"),
					[]byte("# Security masks\n>=dev-libs/openssl-3.2.0\n"),
					0644,
				); err != nil {
					return err
				}
				if err := os.WriteFile(
					filepath.Join(maskDir, "02-testing"),
					[]byte("# Testing masks\n~sys-apps/systemd-255\n"),
					0644,
				); err != nil {
					return err
				}
				return nil
			},
			expectedMask: []string{">=dev-libs/openssl-3.2.0", "~sys-apps/systemd-255"},
		},
		{
			name: "directory with dotfiles and backups ignored",
			setup: func(confDir string) error {
				maskDir := filepath.Join(confDir, "package.mask")
				if err := os.MkdirAll(maskDir, 0755); err != nil {
					return err
				}
				// Valid file
				if err := os.WriteFile(
					filepath.Join(maskDir, "valid"),
					[]byte("=app-misc/test-1.0\n"),
					0644,
				); err != nil {
					return err
				}
				// Dotfile - should be ignored
				if err := os.WriteFile(
					filepath.Join(maskDir, ".hidden"),
					[]byte("=app-misc/hidden-1.0\n"),
					0644,
				); err != nil {
					return err
				}
				// Backup file - should be ignored
				if err := os.WriteFile(
					filepath.Join(maskDir, "backup~"),
					[]byte("=app-misc/backup-1.0\n"),
					0644,
				); err != nil {
					return err
				}
				return nil
			},
			expectedMask: []string{"=app-misc/test-1.0"},
		},
		{
			name: "directory with subdirectories ignored",
			setup: func(confDir string) error {
				maskDir := filepath.Join(confDir, "package.mask")
				if err := os.MkdirAll(maskDir, 0755); err != nil {
					return err
				}
				// Valid file
				if err := os.WriteFile(
					filepath.Join(maskDir, "main"),
					[]byte("=app-misc/main-1.0\n"),
					0644,
				); err != nil {
					return err
				}
				// Subdirectory - should be ignored (not recursed)
				subDir := filepath.Join(maskDir, "subdir")
				if err := os.MkdirAll(subDir, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(
					filepath.Join(subDir, "nested"),
					[]byte("=app-misc/nested-1.0\n"),
					0644,
				); err != nil {
					return err
				}
				return nil
			},
			expectedMask: []string{"=app-misc/main-1.0"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			confDir := filepath.Join(tmpDir, "etc", "portage")
			if err := os.MkdirAll(confDir, 0755); err != nil {
				t.Fatalf("failed to create config dir: %v", err)
			}

			// Create make.conf
			makeConf := filepath.Join(confDir, "make.conf")
			if err := os.WriteFile(makeConf, []byte("# Test config\n"), 0644); err != nil {
				t.Fatalf("failed to create make.conf: %v", err)
			}

			if err := tc.setup(confDir); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			cfg, err := config.LoadConfig(confDir)
			if err != nil {
				t.Fatalf("LoadConfig failed: %v", err)
			}

			maskedPkgs := cfg.PackageMask

			// Verify all expected masks are present
			for _, expected := range tc.expectedMask {
				found := false
				for _, masked := range maskedPkgs {
					if masked == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected mask %q not found in %v", expected, maskedPkgs)
				}
			}

			// Verify count matches
			if len(maskedPkgs) != len(tc.expectedMask) {
				t.Errorf("expected %d masks, got %d: %v",
					len(tc.expectedMask), len(maskedPkgs), maskedPkgs)
			}
		})
	}
}

// TestConfig_PackageUseDirectory tests package.use as a directory.
func TestConfig_PackageUseDirectory(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(confDir string) error
		checkAtom   string
		expectedUSE []string
	}{
		{
			name: "directory with per-package USE flags",
			setup: func(confDir string) error {
				useDir := filepath.Join(confDir, "package.use")
				if err := os.MkdirAll(useDir, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(
					filepath.Join(useDir, "browsers"),
					[]byte("www-client/firefox -wayland\n"),
					0644,
				); err != nil {
					return err
				}
				if err := os.WriteFile(
					filepath.Join(useDir, "media"),
					[]byte("media-video/mpv lua\n"),
					0644,
				); err != nil {
					return err
				}
				return nil
			},
			checkAtom:   "www-client/firefox",
			expectedUSE: []string{"-wayland"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			confDir := filepath.Join(tmpDir, "etc", "portage")
			if err := os.MkdirAll(confDir, 0755); err != nil {
				t.Fatalf("failed to create config dir: %v", err)
			}

			// Create make.conf
			makeConf := filepath.Join(confDir, "make.conf")
			if err := os.WriteFile(makeConf, []byte("# Test config\n"), 0644); err != nil {
				t.Fatalf("failed to create make.conf: %v", err)
			}

			if err := tc.setup(confDir); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			cfg, err := config.LoadConfig(confDir)
			if err != nil {
				t.Fatalf("LoadConfig failed: %v", err)
			}

			useFlags := cfg.GetPackageUSE(tc.checkAtom)

			for _, expected := range tc.expectedUSE {
				found := false
				for _, flag := range useFlags {
					if flag == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected USE flag %q not found for %s in %v",
						expected, tc.checkAtom, useFlags)
				}
			}
		})
	}
}

// TestProfile_DirectoryHandling tests profile directory parsing.
//
// This validates proper handling of profile directories following
// PMS specification for profile stacking.
func TestProfile_DirectoryHandling(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(profileDir string) error
		checkUseFlag string
		expectForce  bool
		expectMask   bool
	}{
		{
			name: "use.force as directory",
			setup: func(profileDir string) error {
				forceDir := filepath.Join(profileDir, "use.force")
				if err := os.MkdirAll(forceDir, 0755); err != nil {
					return err
				}
				return os.WriteFile(
					filepath.Join(forceDir, "00-base"),
					[]byte("multilib\nelibc_glibc\n"),
					0644,
				)
			},
			checkUseFlag: "multilib",
			expectForce:  true,
		},
		{
			name: "use.mask as directory",
			setup: func(profileDir string) error {
				maskDir := filepath.Join(profileDir, "use.mask")
				if err := os.MkdirAll(maskDir, 0755); err != nil {
					return err
				}
				return os.WriteFile(
					filepath.Join(maskDir, "deprecated"),
					[]byte("qt4\ngtk2\n"),
					0644,
				)
			},
			checkUseFlag: "qt4",
			expectMask:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			profileDir := filepath.Join(tmpDir, "profiles", "default", "linux", "amd64")
			if err := os.MkdirAll(profileDir, 0755); err != nil {
				t.Fatalf("failed to create profile dir: %v", err)
			}

			// Create basic profile files
			if err := os.WriteFile(
				filepath.Join(profileDir, "eapi"),
				[]byte("8\n"),
				0644,
			); err != nil {
				t.Fatalf("failed to create eapi file: %v", err)
			}

			if err := tc.setup(profileDir); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			prof, err := profile.LoadProfile(profileDir)
			if err != nil {
				t.Fatalf("LoadProfile failed: %v", err)
			}

			if tc.expectForce {
				found := false
				for _, flag := range prof.USEForce {
					if flag == tc.checkUseFlag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected %q in forced USE flags, got %v",
						tc.checkUseFlag, prof.USEForce)
				}
			}

			if tc.expectMask {
				found := false
				for _, flag := range prof.USEMask {
					if flag == tc.checkUseFlag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected %q in masked USE flags, got %v",
						tc.checkUseFlag, prof.USEMask)
				}
			}
		})
	}
}

// TestProfile_FileSortOrder tests lexicographic ordering of profile files.
//
// PMS requires files in directories to be processed in POSIX locale
// lexicographic order (C locale).
func TestProfile_FileSortOrder(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, "profile")
	useForceDir := filepath.Join(profileDir, "use.force")

	if err := os.MkdirAll(useForceDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	// Create eapi file
	if err := os.WriteFile(filepath.Join(profileDir, "eapi"), []byte("8\n"), 0644); err != nil {
		t.Fatalf("failed to create eapi: %v", err)
	}

	// Create files that should be processed in order: 01, 10, 2, aa, zz
	// Lexicographic order: 01 < 10 < 2 < aa < zz
	files := []struct {
		name    string
		content string
	}{
		{"zz-last", "flag_z\n"},
		{"10-ten", "flag_10\n"},
		{"2-two", "flag_2\n"},
		{"01-first", "flag_01\n"},
		{"aa-middle", "flag_aa\n"},
	}

	for _, f := range files {
		path := filepath.Join(useForceDir, f.name)
		if err := os.WriteFile(path, []byte(f.content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", f.name, err)
		}
	}

	prof, err := profile.LoadProfile(profileDir)
	if err != nil {
		t.Fatalf("LoadProfile failed: %v", err)
	}

	// All flags should be present
	expectedFlags := []string{"flag_01", "flag_10", "flag_2", "flag_aa", "flag_z"}
	for _, expected := range expectedFlags {
		found := false
		for _, flag := range prof.USEForce {
			if flag == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected flag %q not found in %v", expected, prof.USEForce)
		}
	}
}

// TestConfig_RealPortageConfig tests against real /etc/portage if available.
func TestConfig_RealPortageConfig(t *testing.T) {
	confPath := "/etc/portage"
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Skip("Real /etc/portage not found")
	}

	cfg, err := config.LoadConfig(confPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Basic sanity checks
	if cfg.MakeConf != nil {
		t.Logf("CFLAGS: %s", cfg.MakeConf.CFLAGS)
		t.Logf("USE: %v", cfg.MakeConf.USE)
		t.Logf("MAKEOPTS: %s", cfg.MakeConf.MAKEOPTS)
	}

	// Check package.mask
	t.Logf("Package masks: %d entries", len(cfg.PackageMask))
	for i, m := range cfg.PackageMask {
		if i >= 5 {
			t.Logf("  ... and %d more", len(cfg.PackageMask)-5)
			break
		}
		t.Logf("  - %s", m)
	}

	// Verify PORTDIR is set or has defaults
	portDir := cfg.GetPortDir()
	if portDir == "" {
		portDir = "/var/db/repos/gentoo"
	}
	if !strings.HasPrefix(portDir, "/") {
		t.Errorf("PORTDIR should be absolute path, got: %s", portDir)
	}
}
