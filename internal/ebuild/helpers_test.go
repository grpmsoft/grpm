package ebuild

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// ============================================================================
// Shared Test Utilities
// ============================================================================

// createTestHelpers creates a Helpers instance for testing.
func createTestHelpers(t *testing.T) (*Helpers, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	testPkg := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		UseFlags: map[string]bool{
			"ssl":     true,
			"zlib":    true,
			"doc":     false,
			"static":  false,
			"minizip": true,
		},
	}

	env, err := NewEnvironment(testPkg, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(env, &stdout, &stderr)

	return helpers, &stdout, &stderr
}

// createTestHelpersWithEAPI creates a Helpers instance with a specific EAPI version.
func createTestHelpersWithEAPI(t *testing.T, eapi string) (*Helpers, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	testPkg := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		UseFlags: map[string]bool{
			"ssl":     true,
			"zlib":    true,
			"doc":     false,
			"static":  false,
			"minizip": true,
		},
	}

	env, err := NewEnvironmentWithEAPI(testPkg, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles", eapi)
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(env, &stdout, &stderr)

	return helpers, &stdout, &stderr
}

// createInstallTestHelpers creates a Helpers instance for installation tests.
func createInstallTestHelpers(t *testing.T) (*Helpers, string) {
	t.Helper()

	tmpDir := t.TempDir()
	imageDir := tmpDir + "/image"

	testPkg := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
		UseFlags: map[string]bool{
			"ssl":  true,
			"zlib": true,
		},
	}

	env, err := NewEnvironment(testPkg, tmpDir, "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}
	env.D = imageDir

	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(env, &stdout, &stderr)

	// Create image directory
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		t.Fatalf("failed to create image dir: %v", err)
	}

	return helpers, tmpDir
}

// createTestFile creates a test file in the given directory.
func createTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create %s: %v", path, err)
	}
	return path
}

// createBuildTestHelpers creates a Helpers instance for build command tests.
func createBuildTestHelpers(t *testing.T) (*Helpers, string, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	sourceDir := filepath.Join(workDir, "zlib-1.2.13")

	testPkg := &pkg.Package{
		Name:    "sys-libs/zlib",
		Version: "1.2.13",
		Slot:    pkg.Slot{Name: "0"},
		UseFlags: map[string]bool{
			"ssl":  true,
			"zlib": true,
		},
	}

	env, err := NewEnvironment(testPkg, tmpDir, "/var/db/repos/gentoo", tmpDir)
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}
	env.WORKDIR = workDir
	env.S = sourceDir
	env.DISTDIR = tmpDir
	env.MAKEOPTS = "-j4"
	env.A = "zlib-1.2.13.tar.gz"

	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(env, &stdout, &stderr)

	// Create directories
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	return helpers, tmpDir, &stdout, &stderr
}
