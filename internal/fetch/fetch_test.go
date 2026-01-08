package fetch

import (
	"os"
	"path/filepath"
	"testing"
)

// Test data - realistic Manifest content
const testManifestContent = `DIST hello-2.10.tar.gz 725946 BLAKE2B abc123def456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890 SHA512 def789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012
DIST hello-2.12.2.tar.gz 1168515 BLAKE2B 111222333444555666777888999000aaabbbcccdddeeefff000111222333444555666777888999000aaabbbcccdddeeefff000111222333444555666777888999 SHA512 999888777666555444333222111000fffeeeddccccbbbaaa000999888777666555444333222111000fffeeeddccccbbbaaa000999888777666555444333222111
EBUILD hello-2.10.ebuild 1234 BLAKE2B ebuild123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456 SHA512 ebuild789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012
AUX fix-build.patch 567 BLAKE2B patch1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567 SHA512 patch7890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123
`

// TestDistfile tests Distfile Value Object
func TestDistfile(t *testing.T) {
	t.Run("NewDistfile", func(t *testing.T) {
		checksums := NewChecksums("sha256hash", "sha512hash", "blake2bhash")
		df := NewDistfile("hello-2.10.tar.gz", 725946, checksums)

		if df.Filename != "hello-2.10.tar.gz" {
			t.Errorf("expected filename 'hello-2.10.tar.gz', got %q", df.Filename)
		}
		if df.Size != 725946 {
			t.Errorf("expected size 725946, got %d", df.Size)
		}
		if len(df.URIs) != 0 {
			t.Errorf("expected empty URIs, got %v", df.URIs)
		}
	})

	t.Run("WithURIs immutability", func(t *testing.T) {
		checksums := NewChecksums("sha256", "", "")
		original := NewDistfile("test.tar.gz", 100, checksums)
		uris := []string{"http://example.com/test.tar.gz"}

		withURIs := original.WithURIs(uris)

		// Original should be unchanged
		if len(original.URIs) != 0 {
			t.Errorf("original URIs modified, expected 0, got %d", len(original.URIs))
		}

		// New copy should have URIs
		if len(withURIs.URIs) != 1 {
			t.Errorf("expected 1 URI, got %d", len(withURIs.URIs))
		}

		// Modifying source slice should not affect copy
		uris[0] = "modified"
		if withURIs.URIs[0] == "modified" {
			t.Error("URIs should be copied, not referenced")
		}
	})

	t.Run("IsValid", func(t *testing.T) {
		tests := []struct {
			name     string
			distfile Distfile
			valid    bool
		}{
			{
				name:     "valid with BLAKE2B",
				distfile: NewDistfile("test.tar.gz", 100, NewChecksums("", "", "blake2b")),
				valid:    true,
			},
			{
				name:     "valid with SHA512",
				distfile: NewDistfile("test.tar.gz", 100, NewChecksums("", "sha512", "")),
				valid:    true,
			},
			{
				name:     "valid with SHA256",
				distfile: NewDistfile("test.tar.gz", 100, NewChecksums("sha256", "", "")),
				valid:    true,
			},
			{
				name:     "invalid - no filename",
				distfile: NewDistfile("", 100, NewChecksums("sha256", "", "")),
				valid:    false,
			},
			{
				name:     "invalid - no checksums",
				distfile: NewDistfile("test.tar.gz", 100, Checksums{}),
				valid:    false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := tt.distfile.IsValid(); got != tt.valid {
					t.Errorf("IsValid() = %v, want %v", got, tt.valid)
				}
			})
		}
	})
}

// TestChecksums tests Checksums Value Object
func TestChecksums(t *testing.T) {
	t.Run("NewChecksums", func(t *testing.T) {
		c := NewChecksums("sha256", "sha512", "blake2b")

		if c.SHA256 != "sha256" || c.SHA512 != "sha512" || c.BLAKE2B != "blake2b" {
			t.Error("NewChecksums did not set all fields correctly")
		}
	})

	t.Run("HasAny", func(t *testing.T) {
		tests := []struct {
			name      string
			checksums Checksums
			hasAny    bool
		}{
			{"all set", NewChecksums("a", "b", "c"), true},
			{"only sha256", NewChecksums("a", "", ""), true},
			{"only sha512", NewChecksums("", "b", ""), true},
			{"only blake2b", NewChecksums("", "", "c"), true},
			{"none set", NewChecksums("", "", ""), false},
			{"zero value", Checksums{}, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := tt.checksums.HasAny(); got != tt.hasAny {
					t.Errorf("HasAny() = %v, want %v", got, tt.hasAny)
				}
			})
		}
	})

	t.Run("Preferred algorithm priority", func(t *testing.T) {
		tests := []struct {
			name          string
			checksums     Checksums
			wantAlgorithm string
			wantHash      string
		}{
			{
				name:          "BLAKE2B preferred over others",
				checksums:     NewChecksums("sha256", "sha512", "blake2b"),
				wantAlgorithm: "BLAKE2B",
				wantHash:      "blake2b",
			},
			{
				name:          "SHA512 when no BLAKE2B",
				checksums:     NewChecksums("sha256", "sha512", ""),
				wantAlgorithm: "SHA512",
				wantHash:      "sha512",
			},
			{
				name:          "SHA256 when only option",
				checksums:     NewChecksums("sha256", "", ""),
				wantAlgorithm: "SHA256",
				wantHash:      "sha256",
			},
			{
				name:          "empty when none set",
				checksums:     Checksums{},
				wantAlgorithm: "",
				wantHash:      "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				algo, hash := tt.checksums.Preferred()
				if algo != tt.wantAlgorithm {
					t.Errorf("Preferred() algorithm = %q, want %q", algo, tt.wantAlgorithm)
				}
				if hash != tt.wantHash {
					t.Errorf("Preferred() hash = %q, want %q", hash, tt.wantHash)
				}
			})
		}
	})

	t.Run("Equals", func(t *testing.T) {
		c1 := NewChecksums("a", "b", "c")
		c2 := NewChecksums("a", "b", "c")
		c3 := NewChecksums("x", "b", "c")

		if !c1.Equals(c2) {
			t.Error("identical checksums should be equal")
		}
		if c1.Equals(c3) {
			t.Error("different checksums should not be equal")
		}
	})
}

// TestConfig tests Config
func TestConfig(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		cfg := DefaultConfig()

		if cfg.DistDir != "/var/cache/distfiles" {
			t.Errorf("expected default DistDir '/var/cache/distfiles', got %q", cfg.DistDir)
		}
		if cfg.MaxRetries != 3 {
			t.Errorf("expected default MaxRetries 3, got %d", cfg.MaxRetries)
		}
		if cfg.Timeout != 300 {
			t.Errorf("expected default Timeout 300, got %d", cfg.Timeout)
		}
		if !cfg.Resume {
			t.Error("expected default Resume true")
		}
		if cfg.Parallel != 1 {
			t.Errorf("expected default Parallel 1, got %d", cfg.Parallel)
		}
	})

	t.Run("WithDistDir", func(t *testing.T) {
		cfg := DefaultConfig().WithDistDir("/custom/distfiles")

		if cfg.DistDir != "/custom/distfiles" {
			t.Errorf("expected DistDir '/custom/distfiles', got %q", cfg.DistDir)
		}
	})

	t.Run("WithMirrors immutability", func(t *testing.T) {
		mirrors := []string{"http://mirror1.com/", "http://mirror2.com/"}
		cfg := DefaultConfig().WithMirrors(mirrors)

		// Modify original slice
		mirrors[0] = "modified"

		// Config should not be affected
		if cfg.Mirrors[0] == "modified" {
			t.Error("WithMirrors should copy slice, not reference it")
		}
	})
}

// TestParseManifest tests Manifest parsing
func TestParseManifest(t *testing.T) {
	// Create temporary Manifest file
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "Manifest")

	if err := os.WriteFile(manifestPath, []byte(testManifestContent), 0644); err != nil {
		t.Fatalf("failed to create test Manifest: %v", err)
	}

	t.Run("parse valid Manifest", func(t *testing.T) {
		manifest, err := ParseManifest(manifestPath)
		if err != nil {
			t.Fatalf("ParseManifest failed: %v", err)
		}

		// Check total entries
		if len(manifest.Entries) != 4 {
			t.Errorf("expected 4 entries, got %d", len(manifest.Entries))
		}

		// Check DIST entries
		if len(manifest.DistFiles) != 2 {
			t.Errorf("expected 2 DIST entries, got %d", len(manifest.DistFiles))
		}

		// Verify specific entry
		entry, ok := manifest.GetEntry("hello-2.10.tar.gz")
		if !ok {
			t.Fatal("expected entry 'hello-2.10.tar.gz' to exist")
		}

		if entry.Type != EntryTypeDist {
			t.Errorf("expected DIST type, got %s", entry.Type)
		}
		if entry.Size != 725946 {
			t.Errorf("expected size 725946, got %d", entry.Size)
		}
		if entry.Checksums.BLAKE2B == "" {
			t.Error("expected BLAKE2B checksum to be set")
		}
		if entry.Checksums.SHA512 == "" {
			t.Error("expected SHA512 checksum to be set")
		}
	})

	t.Run("GetDistfiles conversion", func(t *testing.T) {
		manifest, err := ParseManifest(manifestPath)
		if err != nil {
			t.Fatalf("ParseManifest failed: %v", err)
		}

		distfiles := manifest.GetDistfiles()

		if len(distfiles) != 2 {
			t.Errorf("expected 2 distfiles, got %d", len(distfiles))
		}

		// Verify conversion
		for _, df := range distfiles {
			if !df.IsValid() {
				t.Errorf("distfile %q should be valid", df.Filename)
			}
		}
	})

	t.Run("HasDistfile", func(t *testing.T) {
		manifest, err := ParseManifest(manifestPath)
		if err != nil {
			t.Fatalf("ParseManifest failed: %v", err)
		}

		if !manifest.HasDistfile("hello-2.10.tar.gz") {
			t.Error("expected HasDistfile to return true for existing DIST entry")
		}

		if manifest.HasDistfile("hello-2.10.ebuild") {
			t.Error("expected HasDistfile to return false for EBUILD entry")
		}

		if manifest.HasDistfile("nonexistent.tar.gz") {
			t.Error("expected HasDistfile to return false for nonexistent entry")
		}
	})

	t.Run("parse nonexistent file", func(t *testing.T) {
		_, err := ParseManifest("/nonexistent/Manifest")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("parse empty lines and comments", func(t *testing.T) {
		content := `# Comment line
DIST test.tar.gz 100 SHA256 abc123def456789012345678901234567890123456789012345678901234

# Another comment

DIST other.tar.gz 200 SHA256 def456789012345678901234567890123456789012345678901234567890
`
		emptyPath := filepath.Join(tmpDir, "ManifestWithComments")
		if err := os.WriteFile(emptyPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		manifest, err := ParseManifest(emptyPath)
		if err != nil {
			t.Fatalf("ParseManifest failed: %v", err)
		}

		if len(manifest.DistFiles) != 2 {
			t.Errorf("expected 2 DIST entries, got %d", len(manifest.DistFiles))
		}
	})

	t.Run("parse invalid format", func(t *testing.T) {
		invalidContent := "INVALID LINE WITH NOT ENOUGH FIELDS"
		invalidPath := filepath.Join(tmpDir, "InvalidManifest")
		if err := os.WriteFile(invalidPath, []byte(invalidContent), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		_, err := ParseManifest(invalidPath)
		if err == nil {
			t.Error("expected error for invalid format")
		}
	})
}

// TestManifestPath tests path construction
func TestManifestPath(t *testing.T) {
	tests := []struct {
		repo     string
		pkg      string
		expected string
	}{
		{
			repo:     "/var/db/repos/gentoo",
			pkg:      "app-misc/hello",
			expected: filepath.Join("/var/db/repos/gentoo", "app-misc/hello", "Manifest"),
		},
		{
			repo:     "/usr/portage",
			pkg:      "sys-libs/zlib",
			expected: filepath.Join("/usr/portage", "sys-libs/zlib", "Manifest"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.pkg, func(t *testing.T) {
			got := ManifestPath(tt.repo, tt.pkg)
			if got != tt.expected {
				t.Errorf("ManifestPath(%q, %q) = %q, want %q", tt.repo, tt.pkg, got, tt.expected)
			}
		})
	}
}

// TestParseManifestLine tests individual line parsing
func TestParseManifestLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantType  EntryType
		wantFile  string
		wantSize  int64
		wantError bool
	}{
		{
			name:     "valid DIST entry",
			line:     "DIST hello.tar.gz 12345 SHA256 abc123",
			wantType: EntryTypeDist,
			wantFile: "hello.tar.gz",
			wantSize: 12345,
		},
		{
			name:     "valid EBUILD entry",
			line:     "EBUILD hello-2.10.ebuild 567 BLAKE2B xyz789 SHA512 def456",
			wantType: EntryTypeEbuild,
			wantFile: "hello-2.10.ebuild",
			wantSize: 567,
		},
		{
			name:     "valid AUX entry",
			line:     "AUX patch.patch 100 SHA512 hashvalue",
			wantType: EntryTypeAux,
			wantFile: "patch.patch",
			wantSize: 100,
		},
		{
			name:     "valid MISC entry",
			line:     "MISC metadata.xml 200 BLAKE2B hashvalue",
			wantType: EntryTypeMisc,
			wantFile: "metadata.xml",
			wantSize: 200,
		},
		{
			name:      "insufficient fields",
			line:      "DIST hello.tar.gz 100",
			wantError: true,
		},
		{
			name:      "invalid entry type",
			line:      "UNKNOWN hello.tar.gz 100 SHA256 abc",
			wantError: true,
		},
		{
			name:      "invalid size",
			line:      "DIST hello.tar.gz notanumber SHA256 abc",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parseManifestLine(tt.line)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if entry.Type != tt.wantType {
				t.Errorf("Type = %s, want %s", entry.Type, tt.wantType)
			}
			if entry.Filename != tt.wantFile {
				t.Errorf("Filename = %q, want %q", entry.Filename, tt.wantFile)
			}
			if entry.Size != tt.wantSize {
				t.Errorf("Size = %d, want %d", entry.Size, tt.wantSize)
			}
		})
	}
}
