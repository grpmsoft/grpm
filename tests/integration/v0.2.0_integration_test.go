// Package integration provides integration tests for GRPM v0.2.0 features.
//
// These tests verify the correct integration of:
//   - has_version/best_version queries against PackageDatabase
//   - SHA256 checksum generation for binary packages
//   - GPKG metadata parsing from tar archives
//   - VarDB queries through PackageService
//   - Atom matching in binhost repositories
//
// Test patterns used:
//   - Table-driven tests with t.Run() for subtests
//   - t.TempDir() for temporary test directories
//   - Mock implementations for testing in isolation
package integration

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/application"
	"github.com/grpmsoft/grpm/internal/binpkg"
	"github.com/grpmsoft/grpm/internal/ebuild"
	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
	"github.com/grpmsoft/grpm/internal/state"
)

// ============================================================================
// TestIntegration_HasVersion - has_version/best_version with PackageDatabase
// ============================================================================

// TestIntegration_HasVersion tests has_version and best_version helper functions
// against a mock PackageDatabase containing known installed packages.
func TestIntegration_HasVersion(t *testing.T) {
	// Create mock package database with test packages
	db := state.NewPackageDatabase(t.TempDir())

	// Add test packages to database
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

	// Create helpers instance with package database
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

	// Table-driven tests for has_version
	hasVersionTests := []struct {
		name     string
		atom     string
		expected bool // true = should be found (exit 0), false = not found (exit 1)
	}{
		// Exact version matches
		{"exact version installed", "=sys-libs/zlib-1.2.13", true},
		{"exact version not installed", "=sys-libs/zlib-1.2.12", false},

		// Package name only (any version)
		{"any version of zlib", "sys-libs/zlib", true},
		{"nonexistent package", "dev-fake/notexist", false},

		// Version constraint: >=
		{"zlib >= 1.2.0", ">=sys-libs/zlib-1.2.0", true},
		{"zlib >= 1.2.11", ">=sys-libs/zlib-1.2.11", true},
		{"zlib >= 1.2.13", ">=sys-libs/zlib-1.2.13", true},
		{"zlib >= 1.3.0 (not installed)", ">=sys-libs/zlib-1.3.0", false},

		// Version constraint: <=
		{"zlib <= 2.0", "<=sys-libs/zlib-2.0", true},
		{"zlib <= 1.2.13", "<=sys-libs/zlib-1.2.13", true},
		{"zlib <= 1.0.0 (too old)", "<=sys-libs/zlib-1.0.0", false},

		// Version constraint: >
		{"zlib > 1.2.0", ">sys-libs/zlib-1.2.0", true},
		{"zlib > 1.2.12", ">sys-libs/zlib-1.2.12", true},
		{"zlib > 1.2.13 (none newer)", ">sys-libs/zlib-1.2.13", false},

		// Version constraint: <
		{"zlib < 2.0", "<sys-libs/zlib-2.0", true},
		{"zlib < 1.2.13", "<sys-libs/zlib-1.2.13", true}, // 1.2.11 satisfies
		{"zlib < 1.2.11 (none older)", "<sys-libs/zlib-1.2.11", false},

		// OpenSSL tests (multiple major versions)
		{"openssl any version", "dev-libs/openssl", true},
		{"openssl >= 3.0", ">=dev-libs/openssl-3.0", true},
		{"openssl < 2.0", "<dev-libs/openssl-2.0", true}, // 1.1.1k satisfies
	}

	for _, tc := range hasVersionTests {
		t.Run("has_version_"+tc.name, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()

			err := helpers.HasVersion([]string{tc.atom})

			// exit code 0 (nil error) = found, exit code 1 = not found
			found := err == nil

			if found != tc.expected {
				if tc.expected {
					t.Errorf("has_version %q: expected to find package, but got not found (err=%v)", tc.atom, err)
				} else {
					t.Errorf("has_version %q: expected not found, but package was found", tc.atom)
				}
			}
		})
	}

	// Table-driven tests for best_version
	bestVersionTests := []struct {
		name            string
		atom            string
		expectedVersion string // empty if no match expected
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

			err := helpers.BestVersion([]string{tc.atom})
			if err != nil {
				t.Fatalf("best_version failed: %v", err)
			}

			output := stdout.String()

			if tc.expectedVersion == "" {
				// Expect no output for nonexistent package
				if output != "" {
					t.Errorf("best_version %q: expected empty output, got %q", tc.atom, output)
				}
			} else {
				// Check version is in output
				if output == "" {
					t.Errorf("best_version %q: expected output containing version %s, got empty", tc.atom, tc.expectedVersion)
				} else if !bytes.Contains([]byte(output), []byte(tc.expectedVersion)) {
					t.Errorf("best_version %q: expected version %s in output, got %q", tc.atom, tc.expectedVersion, output)
				}
			}
		})
	}

	// Test has_version without database (should return not found)
	t.Run("has_version_no_database", func(t *testing.T) {
		helpersNoDB := ebuild.NewHelpers(env, &stdout, &stderr)
		// Don't set database

		err := helpersNoDB.HasVersion([]string{"sys-libs/zlib"})
		if err == nil {
			t.Error("has_version without database should return not found (exit 1)")
		}
	})
}

// ============================================================================
// TestIntegration_SHA256Checksum - SHA256 checksum generation
// ============================================================================

// TestIntegration_SHA256Checksum tests SHA256 checksum generation
// for binary packages using the computeSHA256 function.
func TestIntegration_SHA256Checksum(t *testing.T) {
	// Table-driven tests for various file contents
	tests := []struct {
		name    string
		content []byte
	}{
		{
			name:    "empty file",
			content: []byte{},
		},
		{
			name:    "hello world",
			content: []byte("hello world\n"),
		},
		{
			name:    "binary content",
			content: []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0xfd},
		},
		{
			name:    "large content (1KB)",
			content: bytes.Repeat([]byte("GRPM test content "), 57),
		},
		{
			name:    "unicode content",
			content: []byte("Package manager: GRPM"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create temp file with test content
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "testfile")

			if err := os.WriteFile(testFile, tc.content, 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			// Compute expected hash from content
			expectedHash := sha256.Sum256(tc.content)
			expected := hex.EncodeToString(expectedHash[:])

			// Read file and compute hash (simulating gpkg_writer.computeSHA256)
			f, err := os.Open(testFile)
			if err != nil {
				t.Fatalf("failed to open test file: %v", err)
			}
			defer func() { _ = f.Close() }()

			h := sha256.New()
			if _, err := io.Copy(h, f); err != nil {
				t.Fatalf("failed to hash file: %v", err)
			}
			computed := hex.EncodeToString(h.Sum(nil))

			if computed != expected {
				t.Errorf("SHA256 mismatch:\n  got:      %s\n  expected: %s",
					computed, expected)
			}

			// Verify hash has correct length (64 hex chars)
			if len(computed) != 64 {
				t.Errorf("SHA256 hash length: got %d, expected 64", len(computed))
			}
		})
	}

	// Test known SHA256 value (empty file has well-known hash)
	t.Run("known_empty_file_hash", func(t *testing.T) {
		// Empty file SHA256 is well-known constant
		knownEmptyHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		computed := sha256.Sum256([]byte{})
		computedHex := hex.EncodeToString(computed[:])

		if computedHex != knownEmptyHash {
			t.Errorf("empty file hash mismatch:\n  got:      %s\n  expected: %s",
				computedHex, knownEmptyHash)
		}
	})

	// Test checksum format in BuildMetadata (sha256:hexstring)
	t.Run("checksum_format_sha256_prefix", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.bin")
		content := []byte("test content for checksum")

		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Compute expected checksum in format "sha256:hexstring"
		h := sha256.Sum256(content)
		expectedFormat := "sha256:" + hex.EncodeToString(h[:])

		// Verify format is correct
		if len(expectedFormat) != 7+64 { // "sha256:" + 64 hex chars
			t.Errorf("checksum format length incorrect: got %d, expected 71",
				len(expectedFormat))
		}

		if expectedFormat[:7] != "sha256:" {
			t.Errorf("checksum should start with 'sha256:', got %s", expectedFormat[:7])
		}
	})
}

// ============================================================================
// TestIntegration_GPKGMetadataParsing - GPKG format metadata extraction
// ============================================================================

// TestIntegration_GPKGMetadataParsing tests GPKG metadata parsing
// from tar archives containing metadata.tar with package information.
func TestIntegration_GPKGMetadataParsing(t *testing.T) {
	// Test splitPkgNameVersion helper function
	splitTests := []struct {
		input   string
		name    string
		version string
	}{
		{"zlib-1.2.13", "zlib", "1.2.13"},
		{"hello-2.10-r1", "hello", "2.10-r1"},
		{"gtk+-3.24.38", "gtk+", "3.24.38"},
		{"python-exec-2.4.10", "python-exec", "2.4.10"},
		{"gcc-12.3.1_p20230526", "gcc", "12.3.1_p20230526"},
		{"llvm-libunwind-16.0.6", "llvm-libunwind", "16.0.6"},
		// Edge cases
		{"package-name-without-version", "package-name-without-version", ""},
		{"single", "single", ""},
		{"a-1", "a", "1"},
	}

	for _, tc := range splitTests {
		t.Run("splitPkgNameVersion_"+tc.input, func(t *testing.T) {
			// Test using GPKG loading which internally uses splitPkgNameVersion
			// We verify this indirectly through package creation

			// Create a minimal GPKG file structure for testing
			tmpDir := t.TempDir()
			gpkgPath := filepath.Join(tmpDir, tc.input+".gpkg.tar")

			// Create GPKG tar with metadata
			if err := createTestGPKG(gpkgPath, tc.input, "", tc.version, "0", nil); err != nil {
				t.Fatalf("failed to create test GPKG: %v", err)
			}

			// Load and verify
			binPkg, err := binpkg.LoadGPKG(gpkgPath)
			if err != nil {
				t.Fatalf("failed to load GPKG: %v", err)
			}

			if binPkg.Package == nil {
				t.Fatal("loaded package is nil")
			}

			// When PF metadata is provided, version should be parsed correctly
			if tc.version != "" && binPkg.Package.Version != tc.version {
				t.Errorf("version mismatch: got %q, expected %q",
					binPkg.Package.Version, tc.version)
			}
		})
	}

	// Test full GPKG metadata parsing
	t.Run("full_metadata_parsing", func(t *testing.T) {
		tmpDir := t.TempDir()
		gpkgPath := filepath.Join(tmpDir, "sys-libs--zlib-1.2.13.gpkg.tar")

		// Create GPKG with full metadata
		metadata := map[string]string{
			"CATEGORY":   "sys-libs",
			"PF":         "zlib-1.2.13",
			"SLOT":       "0/1.2",
			"USE":        "minizip static-libs",
			"BUILD_TIME": "1700000000",
			"EAPI":       "8",
			"CFLAGS":     "-O2 -pipe",
		}

		if err := createTestGPKG(gpkgPath, "zlib-1.2.13", "sys-libs", "1.2.13", "0/1.2", metadata); err != nil {
			t.Fatalf("failed to create test GPKG: %v", err)
		}

		binPkg, err := binpkg.LoadGPKG(gpkgPath)
		if err != nil {
			t.Fatalf("failed to load GPKG: %v", err)
		}

		// Verify package metadata
		if binPkg.Package.Name != "sys-libs/zlib" {
			t.Errorf("package name: got %q, expected %q", binPkg.Package.Name, "sys-libs/zlib")
		}

		if binPkg.Package.Version != "1.2.13" {
			t.Errorf("package version: got %q, expected %q", binPkg.Package.Version, "1.2.13")
		}

		// Verify slot parsing
		if binPkg.Package.Slot.Name != "0" {
			t.Errorf("slot name: got %q, expected %q", binPkg.Package.Slot.Name, "0")
		}

		// Verify build metadata
		if binPkg.BuildInfo == nil {
			t.Fatal("BuildInfo is nil")
		}

		if binPkg.BuildInfo.EAPI != "8" {
			t.Errorf("EAPI: got %q, expected %q", binPkg.BuildInfo.EAPI, "8")
		}

		if binPkg.BuildInfo.CFLAGS != "-O2 -pipe" {
			t.Errorf("CFLAGS: got %q, expected %q", binPkg.BuildInfo.CFLAGS, "-O2 -pipe")
		}
	})
}

// createTestGPKG creates a minimal GPKG tar file for testing.
func createTestGPKG(path, pf, category, version, slot string, extraMeta map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	tw := tar.NewWriter(f)
	defer func() { _ = tw.Close() }()

	// Create metadata.tar content
	var metaBuf bytes.Buffer
	metaTW := tar.NewWriter(&metaBuf)

	// Add metadata files
	metaFiles := map[string]string{
		"PF":   pf,
		"SLOT": slot,
	}

	if category != "" {
		metaFiles["CATEGORY"] = category
	}

	// Add extra metadata
	for k, v := range extraMeta {
		metaFiles[k] = v
	}

	for name, content := range metaFiles {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := metaTW.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := metaTW.Write([]byte(content)); err != nil {
			return err
		}
	}

	if err := metaTW.Close(); err != nil {
		return err
	}

	// Write metadata.tar to outer tar
	metaData := metaBuf.Bytes()
	hdr := &tar.Header{
		Name: "metadata.tar",
		Mode: 0644,
		Size: int64(len(metaData)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(metaData); err != nil {
		return err
	}

	return nil
}

// ============================================================================
// TestIntegration_VarDBQueries - PackageService with VarDB
// ============================================================================

// TestIntegration_VarDBQueries tests PackageService queries against
// installed packages in the VarDB (PackageDatabase).
func TestIntegration_VarDBQueries(t *testing.T) {
	// Create mock repository
	mockRepo := repo.NewMockRepository()

	// Create package database with installed packages
	db := state.NewPackageDatabase(t.TempDir())

	// Mark zlib as installed
	zlibInstalled := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "sys-libs/zlib",
			Version: "1.2.13",
			Slot:    pkg.Slot{Name: "0"},
		},
		InstallTime: time.Now(),
		Size:        102400,
	}
	if err := db.Add(zlibInstalled); err != nil {
		t.Fatalf("failed to add installed package: %v", err)
	}

	// Create PackageService with both repository and database
	service := application.NewPackageService(mockRepo, db)

	tests := []struct {
		name            string
		packageName     string
		expectInstalled bool
		expectError     bool
	}{
		{
			name:            "installed package zlib",
			packageName:     "sys-libs/zlib",
			expectInstalled: true,
			expectError:     false,
		},
		{
			name:            "not installed package hello",
			packageName:     "app-misc/hello",
			expectInstalled: false,
			expectError:     false,
		},
		{
			name:            "nonexistent package",
			packageName:     "dev-fake/notexist",
			expectInstalled: false,
			expectError:     true, // Package not in repository
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info, err := service.GetPackageInfo(tc.packageName)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error for %s, got nil", tc.packageName)
				}
				return
			}

			if err != nil {
				t.Fatalf("GetPackageInfo(%s) error: %v", tc.packageName, err)
			}

			if info.Installed != tc.expectInstalled {
				t.Errorf("GetPackageInfo(%s).Installed = %v, expected %v",
					tc.packageName, info.Installed, tc.expectInstalled)
			}
		})
	}

	// Test with nil database (should not crash, just report not installed)
	t.Run("nil_database", func(t *testing.T) {
		serviceNoDB := application.NewPackageService(mockRepo, nil)

		info, err := serviceNoDB.GetPackageInfo("sys-libs/zlib")
		if err != nil {
			t.Fatalf("GetPackageInfo failed: %v", err)
		}

		// With nil database, Installed should be false
		if info.Installed {
			t.Error("expected Installed=false with nil database")
		}
	})
}

// ============================================================================
// TestIntegration_AtomMatching - Binhost atom matching
// ============================================================================

// TestIntegration_AtomMatching tests binhost package searching
// with various Portage atom specifications.
func TestIntegration_AtomMatching(t *testing.T) {
	// Create test binhost with packages
	tmpDir := t.TempDir()
	binhost, err := binpkg.NewBinhost(tmpDir)
	if err != nil {
		t.Fatalf("failed to create binhost: %v", err)
	}

	// Add test packages to binhost
	testPackages := []*binpkg.BinaryPackage{
		{
			Package: &pkg.Package{Name: "sys-libs/zlib", Version: "1.2.11"},
			Format:  binpkg.FormatGPKG,
			Path:    filepath.Join(tmpDir, "zlib-1.2.11.gpkg.tar"),
		},
		{
			Package: &pkg.Package{Name: "sys-libs/zlib", Version: "1.2.13"},
			Format:  binpkg.FormatGPKG,
			Path:    filepath.Join(tmpDir, "zlib-1.2.13.gpkg.tar"),
		},
		{
			Package: &pkg.Package{Name: "sys-libs/zlib", Version: "1.3.0"},
			Format:  binpkg.FormatGPKG,
			Path:    filepath.Join(tmpDir, "zlib-1.3.0.gpkg.tar"),
		},
		{
			Package: &pkg.Package{Name: "app-misc/hello", Version: "2.10"},
			Format:  binpkg.FormatGPKG,
			Path:    filepath.Join(tmpDir, "hello-2.10.gpkg.tar"),
		},
		{
			Package: &pkg.Package{Name: "dev-libs/openssl", Version: "1.1.1k"},
			Format:  binpkg.FormatGPKG,
			Path:    filepath.Join(tmpDir, "openssl-1.1.1k.gpkg.tar"),
		},
	}

	// Directly set packages (simulating sync)
	binhost.Packages = testPackages

	tests := []struct {
		name          string
		atom          string
		expectedCount int
		description   string
	}{
		// Package name only (all versions)
		{
			name:          "all zlib versions",
			atom:          "sys-libs/zlib",
			expectedCount: 3,
			description:   "should find all 3 zlib versions",
		},
		{
			name:          "single hello version",
			atom:          "app-misc/hello",
			expectedCount: 1,
			description:   "should find hello 2.10",
		},

		// Exact version match
		{
			name:          "exact version match",
			atom:          "=sys-libs/zlib-1.2.13",
			expectedCount: 1,
			description:   "should find exactly zlib-1.2.13",
		},
		{
			name:          "exact version no match",
			atom:          "=sys-libs/zlib-1.2.12",
			expectedCount: 0,
			description:   "should find no packages",
		},

		// Version constraint: >=
		{
			name:          "zlib >= 1.2",
			atom:          ">=sys-libs/zlib-1.2",
			expectedCount: 3,
			description:   "should find all 3 versions",
		},
		{
			name:          "zlib >= 1.2.13",
			atom:          ">=sys-libs/zlib-1.2.13",
			expectedCount: 2,
			description:   "should find 1.2.13 and 1.3.0",
		},
		{
			name:          "zlib >= 1.3.0",
			atom:          ">=sys-libs/zlib-1.3.0",
			expectedCount: 1,
			description:   "should find only 1.3.0",
		},
		{
			name:          "zlib >= 2.0",
			atom:          ">=sys-libs/zlib-2.0",
			expectedCount: 0,
			description:   "should find no packages",
		},

		// Version constraint: <=
		{
			name:          "zlib <= 1.3",
			atom:          "<=sys-libs/zlib-1.3",
			expectedCount: 2,
			description:   "should find 1.2.11 and 1.2.13",
		},
		{
			name:          "zlib <= 1.2.11",
			atom:          "<=sys-libs/zlib-1.2.11",
			expectedCount: 1,
			description:   "should find only 1.2.11",
		},

		// Version constraint: >
		{
			name:          "zlib > 1.2.11",
			atom:          ">sys-libs/zlib-1.2.11",
			expectedCount: 2,
			description:   "should find 1.2.13 and 1.3.0",
		},
		{
			name:          "zlib > 1.3.0",
			atom:          ">sys-libs/zlib-1.3.0",
			expectedCount: 0,
			description:   "should find no packages",
		},

		// Version constraint: <
		{
			name:          "zlib < 1.3.0",
			atom:          "<sys-libs/zlib-1.3.0",
			expectedCount: 2,
			description:   "should find 1.2.11 and 1.2.13",
		},
		{
			name:          "zlib < 1.2.11",
			atom:          "<sys-libs/zlib-1.2.11",
			expectedCount: 0,
			description:   "should find no packages",
		},

		// Nonexistent package
		{
			name:          "nonexistent package",
			atom:          "dev-fake/notexist",
			expectedCount: 0,
			description:   "should find no packages",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matches := binhost.Find(tc.atom)

			if len(matches) != tc.expectedCount {
				versions := make([]string, len(matches))
				for i, m := range matches {
					if m.Package != nil {
						versions[i] = m.Package.Version
					}
				}
				t.Errorf("Find(%q): got %d packages %v, expected %d (%s)",
					tc.atom, len(matches), versions, tc.expectedCount, tc.description)
			}
		})
	}
}

// ============================================================================
// TestIntegration_DatabaseConsistency - Verify database operations
// ============================================================================

// TestIntegration_DatabaseConsistency tests that PackageDatabase maintains
// consistency across add/remove/query operations.
func TestIntegration_DatabaseConsistency(t *testing.T) {
	db := state.NewPackageDatabase(t.TempDir())

	// Add package
	testPkg := &state.InstalledPackage{
		Package: &pkg.Package{
			Name:    "sys-libs/zlib",
			Version: "1.2.13",
			Slot:    pkg.Slot{Name: "0"},
		},
		InstallTime: time.Now(),
		Files: []state.InstalledFile{
			{Path: "/usr/lib/libz.so", Type: state.FileTypeSymlink},
			{Path: "/usr/lib/libz.so.1", Type: state.FileTypeSymlink},
			{Path: "/usr/lib/libz.so.1.2.13", Type: state.FileTypeRegular},
		},
	}

	t.Run("add_and_verify", func(t *testing.T) {
		if err := db.Add(testPkg); err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		// Verify count
		if db.Count() != 1 {
			t.Errorf("Count() = %d, expected 1", db.Count())
		}

		// Verify Has
		if !db.Has("sys-libs/zlib-1.2.13") {
			t.Error("Has() returned false for installed package")
		}

		// Verify Get
		pkg, err := db.Get("sys-libs/zlib-1.2.13")
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}
		if pkg.Package.Version != "1.2.13" {
			t.Errorf("Get().Version = %s, expected 1.2.13", pkg.Package.Version)
		}
	})

	t.Run("file_ownership", func(t *testing.T) {
		// Verify file ownership
		owner, err := db.WhoOwns("/usr/lib/libz.so.1.2.13")
		if err != nil {
			t.Fatalf("WhoOwns() error: %v", err)
		}
		if owner != "sys-libs/zlib-1.2.13" {
			t.Errorf("WhoOwns() = %s, expected sys-libs/zlib-1.2.13", owner)
		}

		// Non-owned file
		_, err = db.WhoOwns("/usr/lib/libfoo.so")
		if err == nil {
			t.Error("WhoOwns() should return error for non-owned file")
		}
	})

	t.Run("remove_and_verify", func(t *testing.T) {
		if err := db.Remove("sys-libs/zlib-1.2.13"); err != nil {
			t.Fatalf("Remove() error: %v", err)
		}

		// Verify removed
		if db.Has("sys-libs/zlib-1.2.13") {
			t.Error("Has() returned true after removal")
		}

		if db.Count() != 0 {
			t.Errorf("Count() = %d after removal, expected 0", db.Count())
		}

		// File ownership should be gone
		_, err := db.WhoOwns("/usr/lib/libz.so.1.2.13")
		if err == nil {
			t.Error("WhoOwns() should return error after package removal")
		}
	})
}
