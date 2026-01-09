package fetch

import (
	"testing"

	"github.com/grpmsoft/grpm/internal/repo"
)

func TestParseAndListDistfiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		srcURI    string
		flags     map[string]bool
		vars      map[string]string
		wantLen   int
		wantFiles []string
		wantErr   bool
	}{
		{
			name:      "simple URL",
			srcURI:    "https://example.com/file.tar.gz",
			wantLen:   1,
			wantFiles: []string{"file.tar.gz"},
		},
		{
			name:      "arrow syntax",
			srcURI:    "https://example.com/src.tar.gz -> custom.tar.gz",
			wantLen:   1,
			wantFiles: []string{"custom.tar.gz"},
		},
		{
			name:      "conditional enabled",
			srcURI:    "doc? ( https://example.com/doc.pdf )",
			flags:     map[string]bool{"doc": true},
			wantLen:   1,
			wantFiles: []string{"doc.pdf"},
		},
		{
			name:    "conditional disabled",
			srcURI:  "doc? ( https://example.com/doc.pdf )",
			flags:   map[string]bool{"doc": false},
			wantLen: 0,
		},
		{
			name:      "variable expansion",
			srcURI:    "https://example.com/src.tar.gz -> ${P}.tar.gz",
			vars:      map[string]string{"P": "hello-1.0"},
			wantLen:   1,
			wantFiles: []string{"hello-1.0.tar.gz"},
		},
		{
			name:      "mixed content",
			srcURI:    "https://example.com/main.tar.gz doc? ( https://example.com/doc.pdf )",
			flags:     map[string]bool{"doc": true},
			wantLen:   2,
			wantFiles: []string{"main.tar.gz", "doc.pdf"},
		},
		{
			name:    "empty input",
			srcURI:  "",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entries, err := ParseAndListDistfiles(tt.srcURI, tt.flags, tt.vars)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseAndListDistfiles() error = %v, wantErr %v", err, tt.wantErr)
			}

			if len(entries) != tt.wantLen {
				t.Fatalf("got %d entries, want %d", len(entries), tt.wantLen)
			}

			if tt.wantFiles != nil {
				for i, wantFile := range tt.wantFiles {
					if i >= len(entries) {
						t.Errorf("missing expected file: %s", wantFile)
						continue
					}
					if entries[i].Filename != wantFile {
						t.Errorf("entry[%d].Filename = %q, want %q", i, entries[i].Filename, wantFile)
					}
				}
			}
		})
	}
}

func TestSrcURIFetcher_entriesToDistfiles(t *testing.T) {
	t.Parallel()

	// Create a mock manifest
	manifest := &Manifest{
		Entries: map[string]ManifestEntry{
			"file1.tar.gz": {
				Type:     EntryTypeDist,
				Filename: "file1.tar.gz",
				Size:     1000,
				Checksums: Checksums{
					SHA256: "abc123",
					SHA512: "def456",
				},
			},
			"file2.tar.gz": {
				Type:     EntryTypeDist,
				Filename: "file2.tar.gz",
				Size:     2000,
				Checksums: Checksums{
					SHA256: "ghi789",
				},
			},
		},
	}

	tests := []struct {
		name    string
		entries []repo.SrcURIEntry
		wantLen int
		wantErr bool
	}{
		{
			name: "single entry",
			entries: []repo.SrcURIEntry{
				{URL: "https://example.com/file1.tar.gz", Filename: "file1.tar.gz"},
			},
			wantLen: 1,
		},
		{
			name: "multiple entries",
			entries: []repo.SrcURIEntry{
				{URL: "https://example.com/file1.tar.gz", Filename: "file1.tar.gz"},
				{URL: "https://example.com/file2.tar.gz", Filename: "file2.tar.gz"},
			},
			wantLen: 2,
		},
		{
			name: "duplicate filenames deduplicated",
			entries: []repo.SrcURIEntry{
				{URL: "https://example.com/file1.tar.gz", Filename: "file1.tar.gz"},
				{URL: "https://mirror.com/file1.tar.gz", Filename: "file1.tar.gz"},
			},
			wantLen: 1,
		},
		{
			name: "missing from manifest",
			entries: []repo.SrcURIEntry{
				{URL: "https://example.com/unknown.tar.gz", Filename: "unknown.tar.gz"},
			},
			wantErr: true,
		},
		{
			name:    "empty entries",
			entries: []repo.SrcURIEntry{},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fetcher := NewSrcURIFetcher(DefaultConfig())

			distfiles, err := fetcher.entriesToDistfiles(tt.entries, manifest)

			if (err != nil) != tt.wantErr {
				t.Fatalf("entriesToDistfiles() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && len(distfiles) != tt.wantLen {
				t.Fatalf("got %d distfiles, want %d", len(distfiles), tt.wantLen)
			}
		})
	}
}

func TestSrcURIFetcher_entriesToDistfiles_PreservesURIs(t *testing.T) {
	t.Parallel()

	manifest := &Manifest{
		Entries: map[string]ManifestEntry{
			"file.tar.gz": {
				Type:     EntryTypeDist,
				Filename: "file.tar.gz",
				Size:     1000,
				Checksums: Checksums{
					SHA256: "abc123",
				},
			},
		},
	}

	entries := []repo.SrcURIEntry{
		{
			URL:      "https://example.com/custom/path/file.tar.gz",
			Filename: "file.tar.gz",
		},
	}

	fetcher := NewSrcURIFetcher(DefaultConfig())
	distfiles, err := fetcher.entriesToDistfiles(entries, manifest)

	if err != nil {
		t.Fatalf("entriesToDistfiles() error = %v", err)
	}

	if len(distfiles) != 1 {
		t.Fatalf("got %d distfiles, want 1", len(distfiles))
	}

	if len(distfiles[0].URIs) != 1 {
		t.Fatalf("got %d URIs, want 1", len(distfiles[0].URIs))
	}

	if distfiles[0].URIs[0] != "https://example.com/custom/path/file.tar.gz" {
		t.Errorf("URI = %q, want %q", distfiles[0].URIs[0], "https://example.com/custom/path/file.tar.gz")
	}
}

func TestSrcURIFetcher_entriesToDistfiles_ChecksumPreserved(t *testing.T) {
	t.Parallel()

	manifest := &Manifest{
		Entries: map[string]ManifestEntry{
			"file.tar.gz": {
				Type:     EntryTypeDist,
				Filename: "file.tar.gz",
				Size:     12345,
				Checksums: Checksums{
					SHA256:  "sha256hash",
					SHA512:  "sha512hash",
					BLAKE2B: "blake2bhash",
				},
			},
		},
	}

	entries := []repo.SrcURIEntry{
		{URL: "https://example.com/file.tar.gz", Filename: "file.tar.gz"},
	}

	fetcher := NewSrcURIFetcher(DefaultConfig())
	distfiles, err := fetcher.entriesToDistfiles(entries, manifest)

	if err != nil {
		t.Fatalf("entriesToDistfiles() error = %v", err)
	}

	if len(distfiles) != 1 {
		t.Fatalf("got %d distfiles, want 1", len(distfiles))
	}

	d := distfiles[0]

	if d.Filename != "file.tar.gz" {
		t.Errorf("Filename = %q, want %q", d.Filename, "file.tar.gz")
	}

	if d.Size != 12345 {
		t.Errorf("Size = %d, want %d", d.Size, 12345)
	}

	if d.Checksums.SHA256 != "sha256hash" {
		t.Errorf("SHA256 = %q, want %q", d.Checksums.SHA256, "sha256hash")
	}

	if d.Checksums.SHA512 != "sha512hash" {
		t.Errorf("SHA512 = %q, want %q", d.Checksums.SHA512, "sha512hash")
	}

	if d.Checksums.BLAKE2B != "blake2bhash" {
		t.Errorf("BLAKE2B = %q, want %q", d.Checksums.BLAKE2B, "blake2bhash")
	}
}

func TestNewSrcURIFetcher(t *testing.T) {
	t.Parallel()

	config := DefaultConfig().WithMirrors([]string{"https://mirror.example.com/"})

	fetcher := NewSrcURIFetcher(config)

	if fetcher == nil {
		t.Fatal("NewSrcURIFetcher() returned nil")
	}

	if fetcher.downloader == nil {
		t.Error("downloader is nil")
	}

	if fetcher.config.DistDir != config.DistDir {
		t.Errorf("DistDir = %q, want %q", fetcher.config.DistDir, config.DistDir)
	}
}
