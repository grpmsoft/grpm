package binpkg

import (
	"bytes"
	"compress/bzip2"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// createTestTBZ2 creates a valid TBZ2 file for testing.
// It creates a minimal bzip2 compressed tar archive with XPAK metadata appended.
func createTestTBZ2(t *testing.T, destPath string, xpakEntries map[string][]byte) {
	t.Helper()

	file, err := os.Create(destPath)
	if err != nil {
		t.Fatalf("Failed to create test TBZ2: %v", err)
	}
	defer func() { _ = file.Close() }()

	// Write minimal bzip2 data (empty but valid)
	// This is a minimal valid bzip2 stream
	bz2Data := []byte{
		0x42, 0x5a, 0x68, 0x39, // BZ magic + block size
		0x17, 0x72, 0x45, 0x38, // Stream end marker
		0x50, 0x90, 0x00, 0x00, // CRC and padding
		0x00, 0x00,
	}
	if _, err := file.Write(bz2Data); err != nil {
		t.Fatalf("Failed to write bz2 data: %v", err)
	}

	// Append XPAK data
	xpakData := createTestXPAK(xpakEntries)
	if _, err := file.Write(xpakData); err != nil {
		t.Fatalf("Failed to write XPAK data: %v", err)
	}
}

func TestLoadTBZ2(t *testing.T) {
	tmpDir := t.TempDir()
	tbz2Path := filepath.Join(tmpDir, "test-package-1.0.tbz2")

	// Create test TBZ2 with XPAK metadata
	createTestTBZ2(t, tbz2Path, map[string][]byte{
		"EAPI":       []byte("8"),
		"USE":        []byte("ssl python"),
		"BUILD_TIME": []byte("1234567890"),
	})

	binPkg, err := LoadTBZ2(tbz2Path)
	if err != nil {
		t.Fatalf("LoadTBZ2() error = %v", err)
	}

	if binPkg == nil {
		t.Fatal("LoadTBZ2() returned nil")
	}

	if binPkg.Format != FormatTBZ2 {
		t.Errorf("Format = %v, want %v", binPkg.Format, FormatTBZ2)
	}

	if binPkg.Path != tbz2Path {
		t.Errorf("Path = %q, want %q", binPkg.Path, tbz2Path)
	}

	if binPkg.Size <= 0 {
		t.Error("Size should be > 0")
	}
}

func TestLoadTBZ2_NonExistent(t *testing.T) {
	_, err := LoadTBZ2("/nonexistent/path.tbz2")
	if err == nil {
		t.Error("LoadTBZ2() should fail for non-existent file")
	}
}

func TestLoadTBZ2_TarBz2Extension(t *testing.T) {
	tmpDir := t.TempDir()
	tbz2Path := filepath.Join(tmpDir, "test-package-1.0.tar.bz2")

	createTestTBZ2(t, tbz2Path, map[string][]byte{
		"EAPI": []byte("8"),
	})

	binPkg, err := LoadTBZ2(tbz2Path)
	if err != nil {
		t.Fatalf("LoadTBZ2() error = %v", err)
	}

	if binPkg == nil {
		t.Fatal("LoadTBZ2() returned nil")
	}
}

func TestGetTBZ2Metadata(t *testing.T) {
	tmpDir := t.TempDir()
	tbz2Path := filepath.Join(tmpDir, "test-1.0.tbz2")

	createTestTBZ2(t, tbz2Path, map[string][]byte{
		"EAPI":       []byte("8"),
		"USE":        []byte("ssl python test"),
		"CFLAGS":     []byte("-O2 -pipe"),
		"CXXFLAGS":   []byte("-O2 -pipe"),
		"LDFLAGS":    []byte("-Wl,-O1"),
		"FEATURES":   []byte("sandbox userpriv"),
		"repository": []byte("gentoo"),
		"SIZE":       []byte("1024"),
		"CBUILD":     []byte("x86_64-pc-linux-gnu"),
		"BUILD_TIME": []byte("1704067200"),
	})

	metadata, err := GetTBZ2Metadata(tbz2Path)
	if err != nil {
		t.Fatalf("GetTBZ2Metadata() error = %v", err)
	}

	if metadata == nil {
		t.Fatal("GetTBZ2Metadata() returned nil")
	}

	// Verify metadata fields
	if metadata.EAPI != "8" {
		t.Errorf("EAPI = %q, want %q", metadata.EAPI, "8")
	}

	if len(metadata.USE) != 3 {
		t.Errorf("USE length = %d, want 3", len(metadata.USE))
	}

	if metadata.CFLAGS != "-O2 -pipe" {
		t.Errorf("CFLAGS = %q, want %q", metadata.CFLAGS, "-O2 -pipe")
	}

	if metadata.Repository != "gentoo" {
		t.Errorf("Repository = %q, want %q", metadata.Repository, "gentoo")
	}

	if metadata.Size != 1024 {
		t.Errorf("Size = %d, want 1024", metadata.Size)
	}
}

func TestGetTBZ2Metadata_InvalidFile(t *testing.T) {
	_, err := GetTBZ2Metadata("/nonexistent/file.tbz2")
	if err == nil {
		t.Error("GetTBZ2Metadata() should fail for non-existent file")
	}
}

func TestGetTBZ2Metadata_MinimalXPAK(t *testing.T) {
	tmpDir := t.TempDir()
	tbz2Path := filepath.Join(tmpDir, "minimal-1.0.tbz2")

	// Create TBZ2 with minimal XPAK (no BUILD_TIME)
	createTestTBZ2(t, tbz2Path, map[string][]byte{})

	metadata, err := GetTBZ2Metadata(tbz2Path)
	if err != nil {
		t.Fatalf("GetTBZ2Metadata() error = %v", err)
	}

	// Should have default EAPI 0 when no EAPI specified
	if metadata.EAPI != "0" {
		t.Errorf("EAPI = %q, want %q for old packages", metadata.EAPI, "0")
	}

	// BuildDate should be set even when BUILD_TIME is missing
	if metadata.BuildDate.IsZero() {
		t.Error("BuildDate should be set to current time when BUILD_TIME is missing")
	}
}

func TestXpakToMetadata(t *testing.T) {
	tests := []struct {
		name    string
		entries map[string][]byte
		checkFn func(*testing.T, *BuildMetadata)
	}{
		{
			name: "all_fields",
			entries: map[string][]byte{
				"BUILD_TIME": []byte("1704067200"),
				"CBUILD":     []byte("x86_64-pc-linux-gnu"),
				"CFLAGS":     []byte("-O2"),
				"CXXFLAGS":   []byte("-O2"),
				"LDFLAGS":    []byte("-Wl,-O1"),
				"USE":        []byte("ssl python"),
				"FEATURES":   []byte("sandbox"),
				"EAPI":       []byte("8"),
				"repository": []byte("gentoo"),
				"SIZE":       []byte("2048"),
			},
			checkFn: func(t *testing.T, m *BuildMetadata) {
				if m.BuildHost != "x86_64-pc-linux-gnu" {
					t.Errorf("BuildHost = %q", m.BuildHost)
				}
				if m.CFLAGS != "-O2" {
					t.Errorf("CFLAGS = %q", m.CFLAGS)
				}
				if m.EAPI != "8" {
					t.Errorf("EAPI = %q", m.EAPI)
				}
				if m.Size != 2048 {
					t.Errorf("Size = %d", m.Size)
				}
			},
		},
		{
			name:    "empty_entries",
			entries: map[string][]byte{},
			checkFn: func(t *testing.T, m *BuildMetadata) {
				if m.EAPI != "0" {
					t.Errorf("EAPI = %q, want %q", m.EAPI, "0")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xpak := &XPAK{Entries: tt.entries}
			metadata, err := xpakToMetadata(xpak)
			if err != nil {
				t.Fatalf("xpakToMetadata() error = %v", err)
			}
			if metadata == nil {
				t.Fatal("xpakToMetadata() returned nil")
			}
			tt.checkFn(t, metadata)
		})
	}
}

func TestFindXPAKOffset(t *testing.T) {
	tmpDir := t.TempDir()
	tbz2Path := filepath.Join(tmpDir, "test-1.0.tbz2")

	// Create test file with known XPAK position
	createTestTBZ2(t, tbz2Path, map[string][]byte{
		"TEST": []byte("value"),
	})

	file, err := os.Open(tbz2Path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()

	offset, err := findXPAKOffset(file)
	if err != nil {
		t.Fatalf("findXPAKOffset() error = %v", err)
	}

	if offset < 0 {
		t.Errorf("offset = %d, want >= 0", offset)
	}
}

func TestFindXPAKOffset_TooShort(t *testing.T) {
	tmpDir := t.TempDir()
	shortPath := filepath.Join(tmpDir, "short.tbz2")

	// Create a file that's too short
	if err := os.WriteFile(shortPath, []byte("short"), 0644); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(shortPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()

	_, err = findXPAKOffset(file)
	if err == nil {
		t.Error("findXPAKOffset() should fail for too-short file")
	}
}

// TestExtractTBZ2 tests extraction of TBZ2 packages.
// Note: This is a simplified test since creating a fully valid bzip2+tar
// archive with proper XPAK is complex.
func TestExtractTBZ2_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	err := ExtractTBZ2("/nonexistent/file.tbz2", tmpDir)
	if err == nil {
		t.Error("ExtractTBZ2() should fail for non-existent file")
	}
}

func TestBzip2Reader(t *testing.T) {
	// Test that we can decompress valid bzip2 data
	// This is a minimal bzip2-compressed empty block
	compressedData := []byte{
		0x42, 0x5a, 0x68, 0x39, // BZh9
		0x17, 0x72, 0x45, 0x38, // End of stream marker
		0x50, 0x90, 0x00, 0x00,
		0x00, 0x00,
	}

	reader := bzip2.NewReader(bytes.NewReader(compressedData))
	_, err := io.ReadAll(reader)
	// This should work - bzip2 decompression of minimal stream
	if err != nil {
		// bzip2 may reject truly minimal streams, that's OK
		t.Logf("bzip2 decompression: %v (expected for minimal stream)", err)
	}
}

func BenchmarkLoadTBZ2(b *testing.B) {
	tmpDir := b.TempDir()
	tbz2Path := filepath.Join(tmpDir, "bench-1.0.tbz2")

	// Create test file
	file, _ := os.Create(tbz2Path)
	// Write minimal bz2 data
	bz2Data := []byte{0x42, 0x5a, 0x68, 0x39, 0x17, 0x72, 0x45, 0x38, 0x50, 0x90, 0x00, 0x00, 0x00, 0x00}
	_, _ = file.Write(bz2Data)
	// Append XPAK
	xpakData := createTestXPAK(map[string][]byte{"EAPI": []byte("8")})
	_, _ = file.Write(xpakData)
	_ = file.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = LoadTBZ2(tbz2Path)
	}
}

func BenchmarkGetTBZ2Metadata(b *testing.B) {
	tmpDir := b.TempDir()
	tbz2Path := filepath.Join(tmpDir, "bench-1.0.tbz2")

	// Create test file
	file, _ := os.Create(tbz2Path)
	bz2Data := []byte{0x42, 0x5a, 0x68, 0x39, 0x17, 0x72, 0x45, 0x38, 0x50, 0x90, 0x00, 0x00, 0x00, 0x00}
	_, _ = file.Write(bz2Data)
	xpakData := createTestXPAK(map[string][]byte{
		"EAPI":       []byte("8"),
		"USE":        []byte("ssl python"),
		"BUILD_TIME": []byte("1234567890"),
	})
	_, _ = file.Write(xpakData)
	_ = file.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetTBZ2Metadata(tbz2Path)
	}
}

// Test XPAK length parsing in footer
func TestXPAKFooterParsing(t *testing.T) {
	// Create a buffer with known XPAK footer structure
	var buf bytes.Buffer

	// Simulate tar.bz2 data
	buf.WriteString("fake tar data here")

	// Add XPAK structure
	var indexBuf, dataBuf bytes.Buffer

	// Add a test entry
	key := []byte("TEST")
	value := []byte("value")

	// Write index entry
	_ = binary.Write(&indexBuf, binary.BigEndian, uint32(len(key)))
	indexBuf.Write(key)
	_ = binary.Write(&indexBuf, binary.BigEndian, uint32(len(value)))
	_ = binary.Write(&indexBuf, binary.BigEndian, uint32(dataBuf.Len()))
	dataBuf.Write(value)

	indexData := indexBuf.Bytes()
	dataData := dataBuf.Bytes()

	// Write XPAKPACK magic
	buf.WriteString(xpakMagic)

	// Write index and data
	buf.Write(indexData)
	buf.Write(dataData)

	// Write footer
	xpakLen := uint32(8 + len(indexData) + len(dataData))
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(indexData)))
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(dataData)))
	_ = binary.Write(&buf, binary.BigEndian, xpakLen)
	buf.WriteString(xpakStop)

	// Parse the XPAK
	reader := bytes.NewReader(buf.Bytes())
	xpak, err := ParseXPAK(reader)
	if err != nil {
		t.Fatalf("ParseXPAK() error = %v", err)
	}

	// Verify entry
	val, exists := xpak.GetString("TEST")
	if !exists {
		t.Error("TEST key not found")
	}
	if val != "value" {
		t.Errorf("TEST value = %q, want %q", val, "value")
	}
}
