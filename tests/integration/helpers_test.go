// Package integration provides integration tests for GRPM.
package integration

import (
	"bytes"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/ebuild"
	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/state"
)

// TestHelpers_HasVersion tests has_version and best_version helper functions
// against a mock PackageDatabase containing known installed packages.
func TestHelpers_HasVersion(t *testing.T) {
	db := state.NewPackageDatabase(t.TempDir())

	testPackages := []struct {
		category string
		name     string
		version  string
	}{
		{"sys-libs", "zlib", "1.2.11"},
		{"sys-libs", "zlib", "1.2.13"},
		{"app-misc", "hello", "2.10"},
		{"dev-libs", "openssl", "1.1.1k"},
		{"dev-libs", "openssl", "3.0.8"},
	}

	for _, tp := range testPackages {
		installedPkg := &state.InstalledPackage{
			Package: &pkg.Package{
				Name:    tp.category + "/" + tp.name,
				Version: tp.version,
				Slot:    pkg.Slot{Name: "0"},
			},
			InstallTime: time.Now(),
		}
		if err := db.Add(installedPkg); err != nil {
			t.Fatalf("failed to add test package %s/%s-%s: %v",
				tp.category, tp.name, tp.version, err)
		}
	}

	testPkg := &pkg.Package{
		Name:    "test/pkg",
		Version: "1.0",
	}
	env, err := ebuild.NewEnvironment(testPkg, t.TempDir(), t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	var stdout, stderr bytes.Buffer
	helpers := ebuild.NewHelpers(env, &stdout, &stderr)
	helpers.SetPackageDatabase(db)

	hasVersionTests := []struct {
		name     string
		atom     string
		expected bool
	}{
		{"exact version installed", "=sys-libs/zlib-1.2.13", true},
		{"exact version not installed", "=sys-libs/zlib-1.2.12", false},
		{"any version of zlib", "sys-libs/zlib", true},
		{"nonexistent package", "dev-fake/notexist", false},
		{"zlib >= 1.2.0", ">=sys-libs/zlib-1.2.0", true},
		{"zlib >= 1.3.0 (not installed)", ">=sys-libs/zlib-1.3.0", false},
		{"zlib <= 2.0", "<=sys-libs/zlib-2.0", true},
		{"zlib > 1.2.12", ">sys-libs/zlib-1.2.12", true},
		{"zlib < 1.2.13", "<sys-libs/zlib-1.2.13", true},
		{"openssl any version", "dev-libs/openssl", true},
		{"openssl >= 3.0", ">=dev-libs/openssl-3.0", true},
	}

	for _, tc := range hasVersionTests {
		t.Run("has_version_"+tc.name, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()
			err := helpers.HasVersion([]string{tc.atom})
			found := err == nil
			if found != tc.expected {
				t.Errorf("has_version %q: got %v, expected %v", tc.atom, found, tc.expected)
			}
		})
	}

	bestVersionTests := []struct {
		name            string
		atom            string
		expectedVersion string
	}{
		{"zlib best version", "sys-libs/zlib", "1.2.13"},
		{"hello best version", "app-misc/hello", "2.10"},
		{"openssl best version", "dev-libs/openssl", "3.0.8"},
		{"nonexistent package", "dev-fake/notexist", ""},
	}

	for _, tc := range bestVersionTests {
		t.Run("best_version_"+tc.name, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()
			if err := helpers.BestVersion([]string{tc.atom}); err != nil {
				t.Fatalf("best_version failed: %v", err)
			}
			output := stdout.String()
			if tc.expectedVersion == "" {
				if output != "" {
					t.Errorf("expected empty output, got %q", output)
				}
			} else if !bytes.Contains([]byte(output), []byte(tc.expectedVersion)) {
				t.Errorf("expected version %s in output, got %q", tc.expectedVersion, output)
			}
		})
	}

	t.Run("has_version_no_database", func(t *testing.T) {
		helpersNoDB := ebuild.NewHelpers(env, &stdout, &stderr)
		if err := helpersNoDB.HasVersion([]string{"sys-libs/zlib"}); err == nil {
			t.Error("has_version without database should return not found")
		}
	})
}
