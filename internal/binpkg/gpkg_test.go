package binpkg

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// createTestGPKG creates a valid GPKG archive for testing.
func createTestGPKG(t *testing.T, destPath string, contents map[string][]byte) {
	t.Helper()

	file, err := os.Create(destPath)
	if err != nil {
		t.Fatalf("Failed to create test GPKG: %v", err)
	}
	defer func() { _ = file.Close() }()

	tw := tar.NewWriter(file)
	defer func() { _ = tw.Close() }()

	// Add gpkg-1 marker
	if err := tw.WriteHeader(&tar.Header{
		Name: "gpkg-1",
		Mode: 0644,
		Size: 0,
	}); err != nil {
		t.Fatalf("Failed to write gpkg-1 marker: %v", err)
	}

	// Add contents
	for name, data := range contents {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(data)),
		}); err != nil {
			t.Fatalf("Failed to write header for %s: %v", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("Failed to write data for %s: %v", name, err)
		}
	}
}

// createTestGPKGWithImage creates a GPKG archive with an embedded image.tar.
func createTestGPKGWithImage(t *testing.T, destPath string, files map[string][]byte) {
	t.Helper()

	file, err := os.Create(destPath)
	if err != nil {
		t.Fatalf("Failed to create test GPKG: %v", err)
	}
	defer func() { _ = file.Close() }()

	tw := tar.NewWriter(file)
	defer func() { _ = tw.Close() }()

	// Add gpkg-1 marker
	if err := tw.WriteHeader(&tar.Header{
		Name: "gpkg-1",
		Mode: 0644,
		Size: 0,
	}); err != nil {
		t.Fatalf("Failed to write gpkg-1 marker: %v", err)
	}

	// Create image.tar in memory
	var imageBuf bytes.Buffer
	imageTw := tar.NewWriter(&imageBuf)

	for name, data := range files {
		if err := imageTw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(data)),
		}); err != nil {
			t.Fatalf("Failed to write image header for %s: %v", name, err)
		}
		if _, err := imageTw.Write(data); err != nil {
			t.Fatalf("Failed to write image data for %s: %v", name, err)
		}
	}
	_ = imageTw.Close()

	// Add image.tar to outer archive
	imageData := imageBuf.Bytes()
	if err := tw.WriteHeader(&tar.Header{
		Name: "image.tar",
		Mode: 0644,
		Size: int64(len(imageData)),
	}); err != nil {
		t.Fatalf("Failed to write image.tar header: %v", err)
	}
	if _, err := tw.Write(imageData); err != nil {
		t.Fatalf("Failed to write image.tar data: %v", err)
	}
}

func TestLoadGPKG(t *testing.T) {
	tmpDir := t.TempDir()
	gpkgPath := filepath.Join(tmpDir, "test-package-1.0.gpkg.tar")

	// Create test GPKG
	createTestGPKG(t, gpkgPath, map[string][]byte{
		"metadata.tar": []byte("test metadata"),
	})

	// Test loading
	binPkg, err := LoadGPKG(gpkgPath)
	if err != nil {
		t.Fatalf("LoadGPKG() error = %v", err)
	}

	if binPkg == nil {
		t.Fatal("LoadGPKG() returned nil")
	}

	if binPkg.Format != FormatGPKG {
		t.Errorf("Format = %v, want %v", binPkg.Format, FormatGPKG)
	}

	if binPkg.Path != gpkgPath {
		t.Errorf("Path = %q, want %q", binPkg.Path, gpkgPath)
	}

	if binPkg.Size <= 0 {
		t.Error("Size should be > 0")
	}
}

func TestLoadGPKG_NonExistent(t *testing.T) {
	_, err := LoadGPKG("/nonexistent/path.gpkg.tar")
	if err == nil {
		t.Error("LoadGPKG() should fail for non-existent file")
	}
}

func TestGetGPKGMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	gpkgPath := filepath.Join(tmpDir, "test-1.0.gpkg.tar")

	// Create test GPKG with metadata
	createTestGPKG(t, gpkgPath, map[string][]byte{
		"metadata.tar": []byte("test metadata content"),
	})

	metadata, err := GetGPKGMetadata(gpkgPath)
	if err != nil {
		t.Fatalf("GetGPKGMetadata() error = %v", err)
	}

	if metadata == nil {
		t.Fatal("GetGPKGMetadata() returned nil")
	}

	// Should have default metadata
	if metadata.EAPI == "" {
		t.Error("EAPI should not be empty")
	}
}

func TestGetGPKGMetadata_NoMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	gpkgPath := filepath.Join(tmpDir, "test-1.0.gpkg.tar")

	// Create test GPKG without metadata.tar
	createTestGPKG(t, gpkgPath, map[string][]byte{
		"gpkg-1": []byte(""),
	})

	// Should return default metadata when no metadata.tar found
	metadata, err := GetGPKGMetadata(gpkgPath)
	if err != nil {
		t.Fatalf("GetGPKGMetadata() error = %v", err)
	}

	if metadata == nil {
		t.Fatal("GetGPKGMetadata() should return default metadata")
	}
}

func TestGetGPKGMetadata_InvalidFile(t *testing.T) {
	_, err := GetGPKGMetadata("/nonexistent/file.gpkg.tar")
	if err == nil {
		t.Error("GetGPKGMetadata() should fail for non-existent file")
	}
}

func TestExtractGPKG(t *testing.T) {
	tmpDir := t.TempDir()
	gpkgPath := filepath.Join(tmpDir, "test-1.0.gpkg.tar")
	destDir := filepath.Join(tmpDir, "extracted")

	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test GPKG with image
	createTestGPKGWithImage(t, gpkgPath, map[string][]byte{
		"usr/bin/hello":        []byte("#!/bin/bash\necho hello"),
		"usr/share/doc/README": []byte("Documentation"),
	})

	err := ExtractGPKG(gpkgPath, destDir)
	if err != nil {
		t.Fatalf("ExtractGPKG() error = %v", err)
	}

	// Verify files were extracted
	expectedFiles := []string{
		filepath.Join(destDir, "usr", "bin", "hello"),
		filepath.Join(destDir, "usr", "share", "doc", "README"),
	}

	for _, f := range expectedFiles {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("Expected file %s not found: %v", f, err)
		}
	}
}

func TestExtractGPKG_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	err := ExtractGPKG("/nonexistent/file.gpkg.tar", tmpDir)
	if err == nil {
		t.Error("ExtractGPKG() should fail for non-existent file")
	}
}

func TestExtractGPKG_NoImageTar(t *testing.T) {
	tmpDir := t.TempDir()
	gpkgPath := filepath.Join(tmpDir, "test-1.0.gpkg.tar")
	destDir := filepath.Join(tmpDir, "extracted")

	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test GPKG without image.tar
	createTestGPKG(t, gpkgPath, map[string][]byte{
		"metadata.tar": []byte("metadata only"),
	})

	err := ExtractGPKG(gpkgPath, destDir)
	if err == nil {
		t.Error("ExtractGPKG() should fail when no image.tar found")
	}
}

func TestExtractFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a tar with a file
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	content := []byte("test file content")
	header := &tar.Header{
		Name: "test.txt",
		Mode: 0644,
		Size: int64(len(content)),
	}
	_ = tw.WriteHeader(header)
	_, _ = tw.Write(content)
	_ = tw.Close()

	// Extract using tar reader
	tr := tar.NewReader(&buf)
	hdr, _ := tr.Next()

	targetPath := filepath.Join(tmpDir, "test.txt")
	err := extractFile(tr, targetPath, hdr)
	if err != nil {
		t.Fatalf("extractFile() error = %v", err)
	}

	// Verify content
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("Failed to read extracted file: %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("Extracted content = %q, want %q", data, content)
	}
}

func BenchmarkLoadGPKG(b *testing.B) {
	tmpDir := b.TempDir()
	gpkgPath := filepath.Join(tmpDir, "bench-1.0.gpkg.tar")

	// Create test GPKG
	file, _ := os.Create(gpkgPath)
	tw := tar.NewWriter(file)
	_ = tw.WriteHeader(&tar.Header{Name: "gpkg-1", Mode: 0644, Size: 0})
	_ = tw.WriteHeader(&tar.Header{Name: "metadata.tar", Mode: 0644, Size: 4})
	_, _ = tw.Write([]byte("test"))
	_ = tw.Close()
	_ = file.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = LoadGPKG(gpkgPath)
	}
}

func BenchmarkGetGPKGMetadata(b *testing.B) {
	tmpDir := b.TempDir()
	gpkgPath := filepath.Join(tmpDir, "bench-1.0.gpkg.tar")

	// Create test GPKG
	file, _ := os.Create(gpkgPath)
	tw := tar.NewWriter(file)
	_ = tw.WriteHeader(&tar.Header{Name: "gpkg-1", Mode: 0644, Size: 0})
	_ = tw.WriteHeader(&tar.Header{Name: "metadata.tar", Mode: 0644, Size: 4})
	_, _ = tw.Write([]byte("test"))
	_ = tw.Close()
	_ = file.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetGPKGMetadata(gpkgPath)
	}
}
