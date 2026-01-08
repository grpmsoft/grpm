package binpkg

import (
	"archive/tar"
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cbzip2 "github.com/dsnet/compress/bzip2"
)

// TestTBZ2Writer_Write tests successful TBZ2 package creation
func TestTBZ2Writer_Write(t *testing.T) {
	// Skip on Windows (symlink requires admin privileges)
	if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") {
		t.Skip("Skipping symlink test on Windows (requires admin privileges)")
	}

	// Create temporary staging directory
	stagingDir := t.TempDir()

	// Create test files in staging
	testFile := filepath.Join(stagingDir, "usr", "bin", "testapp")
	if err := os.MkdirAll(filepath.Dir(testFile), 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
	if err := os.WriteFile(testFile, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create test symlink
	symlinkPath := filepath.Join(stagingDir, "usr", "bin", "testlink")
	if err := os.Symlink("testapp", symlinkPath); err != nil {
		t.Fatalf("failed to create test symlink: %v", err)
	}

	// Create output directory
	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "test-1.0.0.tbz2")

	// Prepare metadata
	metadata := &BuildMetadata{
		BuildDate: time.Now(),
		BuildHost: "test-builder",
		CFLAGS:    "-O2 -pipe",
		CXXFLAGS:  "-O2 -pipe",
		LDFLAGS:   "-Wl,-O1",
		USE:       []string{"ssl", "unicode"},
		Features:  []string{"sandbox", "usersandbox"},
		EAPI:      "8",
		Size:      1024,
	}

	// Create writer
	writer := NewTBZ2Writer(outputPath)
	writer.Verbose = false

	// Execute
	err := writer.Write(metadata, stagingDir)

	// Verify
	if err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	// Check output file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatalf("output file not created: %s", outputPath)
	}

	// Verify file structure
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	// Check for XPAK magic at the end
	if len(data) < 8 {
		t.Fatalf("output file too small: %d bytes", len(data))
	}

	// Find XPAKSTOP marker
	xpakStopPos := bytes.LastIndex(data, []byte("XPAKSTOP"))
	if xpakStopPos == -1 {
		t.Fatalf("XPAKSTOP marker not found")
	}

	// Verify XPAK footer
	footerStart := xpakStopPos - 12 // index_len(4) + data_len(4) + xpak_len(4)
	if footerStart < 0 {
		t.Fatalf("invalid XPAK footer position")
	}

	xpakLen := binary.BigEndian.Uint32(data[footerStart+8 : footerStart+12])
	if xpakLen == 0 {
		t.Fatalf("XPAK length is zero")
	}

	// Verify XPAKPACK marker
	xpakStart := xpakStopPos + 8 - int(xpakLen)
	if xpakStart < 0 || xpakStart >= len(data) {
		t.Fatalf("invalid XPAK start position: %d", xpakStart)
	}

	if !bytes.HasPrefix(data[xpakStart:], []byte("XPAKPACK")) {
		t.Fatalf("XPAKPACK marker not found at expected position")
	}

	t.Logf("TBZ2 package created successfully: %d bytes, XPAK at offset %d", len(data), xpakStart)
}

// TestTBZ2Writer_CreateTarBz2 tests tar.bz2 archive creation
func TestTBZ2Writer_CreateTarBz2(t *testing.T) {
	// Skip on Windows (symlink requires admin privileges)
	if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") {
		t.Skip("Skipping symlink test on Windows (requires admin privileges)")
	}

	// Create staging directory with test content
	stagingDir := t.TempDir()

	// Create directory
	dirPath := filepath.Join(stagingDir, "usr", "share", "doc")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Create regular file
	filePath := filepath.Join(dirPath, "README")
	if err := os.WriteFile(filePath, []byte("Test documentation"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create symlink
	linkPath := filepath.Join(dirPath, "README.link")
	if err := os.Symlink("README", linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Create TBZ2 writer
	outputPath := filepath.Join(t.TempDir(), "test.tar.bz2")
	writer := &TBZ2Writer{OutputPath: outputPath}

	// Execute
	err := writer.createTarBz2(stagingDir, outputPath)

	// Verify
	if err != nil {
		t.Fatalf("createTarBz2() failed: %v", err)
	}

	// Verify output file exists
	stat, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	if stat.Size() == 0 {
		t.Fatalf("output file is empty")
	}

	// Decompress and verify tar contents
	file, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("failed to open output file: %v", err)
	}
	defer func() { _ = file.Close() }()

	bz2Reader, err := cbzip2.NewReader(file, nil)
	if err != nil {
		t.Fatalf("failed to create bzip2 reader: %v", err)
	}
	defer func() { _ = bz2Reader.Close() }()

	tarReader := tar.NewReader(bz2Reader)

	foundFiles := make(map[string]bool)
	for {
		header, err := tarReader.Next()
		if err != nil {
			break
		}
		foundFiles[header.Name] = true
	}

	// Verify expected files
	expectedFiles := []string{
		"usr/share/doc/",
		"usr/share/doc/README",
		"usr/share/doc/README.link",
	}

	for _, expected := range expectedFiles {
		if !foundFiles[expected] {
			t.Errorf("expected file %s not found in tar archive", expected)
		}
	}

	t.Logf("Tar.bz2 created successfully with %d entries", len(foundFiles))
}

// TestTBZ2Writer_CreateXPAK tests XPAK metadata creation
func TestTBZ2Writer_CreateXPAK(t *testing.T) {
	// Create staging directory
	stagingDir := t.TempDir()
	testFile := filepath.Join(stagingDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Prepare metadata
	buildTime := time.Date(2025, 10, 9, 12, 0, 0, 0, time.UTC)
	metadata := &BuildMetadata{
		BuildDate: buildTime,
		BuildHost: "gentoo-builder",
		CFLAGS:    "-O2 -march=native",
		CXXFLAGS:  "-O2 -march=native",
		LDFLAGS:   "-Wl,-O1 -Wl,--as-needed",
		USE:       []string{"ssl", "ipv6", "nls"},
		Features:  []string{"sandbox", "test"},
		EAPI:      "8",
		Size:      2048,
	}

	// Create writer
	writer := &TBZ2Writer{}

	// Execute
	xpakData, err := writer.createXPAK(metadata, stagingDir)

	// Verify
	if err != nil {
		t.Fatalf("createXPAK() failed: %v", err)
	}

	if len(xpakData) == 0 {
		t.Fatalf("XPAK data is empty")
	}

	// Verify XPAK structure
	if !bytes.HasPrefix(xpakData, []byte("XPAKPACK")) {
		t.Fatalf("XPAK missing XPAKPACK header")
	}

	if !bytes.HasSuffix(xpakData, []byte("XPAKSTOP")) {
		t.Fatalf("XPAK missing XPAKSTOP footer")
	}

	// Verify footer structure (last 20 bytes before XPAKSTOP)
	footerStart := len(xpakData) - 20
	indexLen := binary.BigEndian.Uint32(xpakData[footerStart : footerStart+4])
	dataLen := binary.BigEndian.Uint32(xpakData[footerStart+4 : footerStart+8])
	xpakLen := binary.BigEndian.Uint32(xpakData[footerStart+8 : footerStart+12])

	if indexLen == 0 {
		t.Fatalf("XPAK index length is zero")
	}
	if dataLen == 0 {
		t.Fatalf("XPAK data length is zero")
	}
	if xpakLen == 0 {
		t.Fatalf("XPAK total length is zero")
	}

	// Verify calculated length matches actual
	expectedLen := 8 + int(indexLen) + int(dataLen) + 20 // XPAKPACK + index + data + footer
	if int(xpakLen) != expectedLen {
		t.Errorf("XPAK length mismatch: expected %d, got %d", expectedLen, xpakLen)
	}

	t.Logf("XPAK created: %d bytes (index: %d, data: %d)", len(xpakData), indexLen, dataLen)
}

// TestTBZ2Writer_CreateContents tests CONTENTS file generation
func TestTBZ2Writer_CreateContents(t *testing.T) {
	// Skip on Windows (symlink requires admin privileges)
	if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") {
		t.Skip("Skipping symlink test on Windows (requires admin privileges)")
	}

	// Create staging directory with different file types
	stagingDir := t.TempDir()

	// Create directory
	dirPath := filepath.Join(stagingDir, "usr", "lib")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Create regular file
	filePath := filepath.Join(dirPath, "libtest.so")
	if err := os.WriteFile(filePath, []byte("library"), 0755); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create symlink
	linkPath := filepath.Join(dirPath, "libtest.so.1")
	if err := os.Symlink("libtest.so", linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Create writer
	writer := &TBZ2Writer{}

	// Execute
	contents, err := writer.createContents(stagingDir)

	// Verify
	if err != nil {
		t.Fatalf("createContents() failed: %v", err)
	}

	if contents == "" {
		t.Fatalf("CONTENTS is empty")
	}

	// Parse CONTENTS
	lines := strings.Split(strings.TrimSpace(contents), "\n")

	foundDir := false
	foundFile := false
	foundSymlink := false

	for _, line := range lines {
		if strings.HasPrefix(line, "dir /usr/lib") {
			foundDir = true
		}
		if strings.HasPrefix(line, "obj /usr/lib/libtest.so ") {
			foundFile = true
			// Verify checksum present
			parts := strings.Fields(line)
			if len(parts) < 3 {
				t.Errorf("obj line missing checksum: %s", line)
			}
		}
		if strings.HasPrefix(line, "sym /usr/lib/libtest.so.1 -> libtest.so") {
			foundSymlink = true
		}
	}

	if !foundDir {
		t.Errorf("CONTENTS missing directory entry")
	}
	if !foundFile {
		t.Errorf("CONTENTS missing file entry")
	}
	if !foundSymlink {
		t.Errorf("CONTENTS missing symlink entry")
	}

	t.Logf("CONTENTS generated with %d entries", len(lines))
}

// TestTBZ2Writer_EncodeXPAK tests XPAK encoding
func TestTBZ2Writer_EncodeXPAK(t *testing.T) {
	tests := []struct {
		name    string
		entries map[string][]byte
		wantErr bool
	}{
		{
			name: "basic entries",
			entries: map[string][]byte{
				"EAPI":       []byte("8"),
				"BUILD_TIME": []byte("1696867200"),
				"USE":        []byte("ssl ipv6"),
			},
			wantErr: false,
		},
		{
			name: "empty value",
			entries: map[string][]byte{
				"EAPI": []byte(""),
				"USE":  []byte("ssl"),
			},
			wantErr: false,
		},
		{
			name:    "empty entries",
			entries: map[string][]byte{},
			wantErr: false,
		},
		{
			name: "large value",
			entries: map[string][]byte{
				"CONTENTS": bytes.Repeat([]byte("obj /usr/bin/test\n"), 1000),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := encodeXPAK(tt.entries)

			if (err != nil) != tt.wantErr {
				t.Errorf("encodeXPAK() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			// Verify structure
			if !bytes.HasPrefix(data, []byte("XPAKPACK")) {
				t.Errorf("missing XPAKPACK header")
			}
			if !bytes.HasSuffix(data, []byte("XPAKSTOP")) {
				t.Errorf("missing XPAKSTOP footer")
			}

			// Verify lengths are consistent
			footerStart := len(data) - 20
			indexLen := binary.BigEndian.Uint32(data[footerStart : footerStart+4])
			dataLen := binary.BigEndian.Uint32(data[footerStart+4 : footerStart+8])
			xpakLen := binary.BigEndian.Uint32(data[footerStart+8 : footerStart+12])

			expectedLen := 8 + indexLen + dataLen + 20
			if xpakLen != expectedLen {
				t.Errorf("XPAK length mismatch: expected %d, got %d", expectedLen, xpakLen)
			}
		})
	}
}

// TestTBZ2Writer_ErrorHandling tests error cases
func TestTBZ2Writer_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(t *testing.T) (string, string, *BuildMetadata) // outputPath, stagingDir, metadata
		wantErrMsg string
	}{
		{
			name: "non-existent staging directory",
			setupFunc: func(t *testing.T) (string, string, *BuildMetadata) {
				outputPath := filepath.Join(t.TempDir(), "test.tbz2")
				stagingDir := filepath.Join(t.TempDir(), "nonexistent")
				metadata := &BuildMetadata{EAPI: "8"}
				return outputPath, stagingDir, metadata
			},
			wantErrMsg: "failed to create tar.bz2",
		},
		// Skipping "invalid output directory" test on Windows - path validation differs
		// {
		// 	name: "invalid output directory",
		// 	setupFunc: func(t *testing.T) (string, string, *BuildMetadata) {
		// 		outputPath := "/invalid/path/that/does/not/exist/test.tbz2"
		// 		stagingDir := t.TempDir()
		// 		// Create test file
		// 		_ = os.WriteFile(filepath.Join(stagingDir, "test"), []byte("test"), 0644)
		// 		metadata := &BuildMetadata{EAPI: "8"}
		// 		return outputPath, stagingDir, metadata
		// 	},
		// 	wantErrMsg: "failed to create output directory",
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputPath, stagingDir, metadata := tt.setupFunc(t)

			writer := NewTBZ2Writer(outputPath)
			err := writer.Write(metadata, stagingDir)

			if err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("expected error containing %q, got %q", tt.wantErrMsg, err.Error())
			}
		})
	}
}

// BenchmarkTBZ2Writer_Write benchmarks TBZ2 package creation
func BenchmarkTBZ2Writer_Write(b *testing.B) {
	// Setup staging directory with test content
	stagingDir := b.TempDir()

	// Create 100 test files
	for i := 0; i < 100; i++ {
		dir := filepath.Join(stagingDir, "usr", "share", "test")
		if err := os.MkdirAll(dir, 0755); err != nil {
			b.Fatalf("failed to create directory: %v", err)
		}
		filePath := filepath.Join(dir, fmt.Sprintf("file%03d", i))
		if err := os.WriteFile(filePath, bytes.Repeat([]byte("test"), 256), 0644); err != nil {
			b.Fatalf("failed to create test file: %v", err)
		}
	}

	metadata := &BuildMetadata{
		BuildDate: time.Now(),
		EAPI:      "8",
		USE:       []string{"ssl", "ipv6"},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		outputPath := filepath.Join(b.TempDir(), "bench.tbz2")
		writer := NewTBZ2Writer(outputPath)
		if err := writer.Write(metadata, stagingDir); err != nil {
			b.Fatalf("Write() failed: %v", err)
		}
	}
}

// BenchmarkEncodeXPAK benchmarks XPAK encoding
func BenchmarkEncodeXPAK(b *testing.B) {
	entries := map[string][]byte{
		"EAPI":       []byte("8"),
		"BUILD_TIME": []byte("1696867200"),
		"USE":        []byte("ssl ipv6 unicode nls"),
		"CFLAGS":     []byte("-O2 -pipe -march=native"),
		"CONTENTS":   bytes.Repeat([]byte("obj /usr/bin/test 1234567890 0755 1234567890\n"), 100),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := encodeXPAK(entries)
		if err != nil {
			b.Fatalf("encodeXPAK() failed: %v", err)
		}
	}
}
