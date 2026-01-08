package binpkg

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

func TestBinhostType_String(t *testing.T) {
	tests := []struct {
		binhostType BinhostType
		expected    string
	}{
		{BinhostLocal, "local"},
		{BinhostHTTP, "http"},
		{BinhostRsync, "rsync"},
		{BinhostSSH, "ssh"},
		{BinhostType(99), "unknown"},
	}

	for _, tt := range tests {
		result := tt.binhostType.String()
		if result != tt.expected {
			t.Errorf("BinhostType(%d).String() = %q, want %q", tt.binhostType, result, tt.expected)
		}
	}
}

func TestNewBinhost(t *testing.T) {
	tests := []struct {
		uri          string
		expectedType BinhostType
	}{
		{"file:///var/cache/binpkgs", BinhostLocal},
		{"/var/cache/binpkgs", BinhostLocal},
		{"http://example.com/binpkgs", BinhostHTTP},
		{"https://example.com/binpkgs", BinhostHTTP},
		{"rsync://rsync.gentoo.org/portage", BinhostRsync},
		{"ssh://user@host/path", BinhostSSH},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			binhost, err := NewBinhost(tt.uri)
			if err != nil {
				t.Fatalf("NewBinhost() error = %v", err)
			}

			if binhost.URI != tt.uri {
				t.Errorf("URI = %q, want %q", binhost.URI, tt.uri)
			}

			if binhost.Type != tt.expectedType {
				t.Errorf("Type = %v, want %v", binhost.Type, tt.expectedType)
			}

			if binhost.Packages == nil {
				t.Error("Packages should not be nil")
			}
		})
	}
}

func TestDetectBinhostType(t *testing.T) {
	tests := []struct {
		uri      string
		expected BinhostType
	}{
		{"http://example.com", BinhostHTTP},
		{"https://example.com", BinhostHTTP},
		{"rsync://rsync.gentoo.org", BinhostRsync},
		{"ssh://user@host", BinhostSSH},
		{"/var/cache/binpkgs", BinhostLocal},
		{"file:///var/cache/binpkgs", BinhostLocal},
		{"unknown://something", BinhostLocal}, // Falls back to local
	}

	for _, tt := range tests {
		result := detectBinhostType(tt.uri)
		if result != tt.expected {
			t.Errorf("detectBinhostType(%q) = %v, want %v", tt.uri, result, tt.expected)
		}
	}
}

func TestBinhost_SyncLocal(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some test packages
	gpkgPath := filepath.Join(tmpDir, "test-1.0.gpkg.tar")
	createTestGPKG(t, gpkgPath, map[string][]byte{
		"metadata.tar": []byte("test"),
	})

	binhost, _ := NewBinhost(tmpDir)

	err := binhost.Sync()
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if len(binhost.Packages) != 1 {
		t.Errorf("Packages length = %d, want 1", len(binhost.Packages))
	}

	if binhost.LastSync.IsZero() {
		t.Error("LastSync should be set")
	}
}

func TestBinhost_SyncLocal_RelativePath(t *testing.T) {
	binhost, _ := NewBinhost("relative/path")

	err := binhost.Sync()
	if err == nil {
		t.Error("Sync() should fail for relative path")
	}
}

func TestBinhost_SyncLocal_NotDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	binhost, _ := NewBinhost(filePath)

	err := binhost.Sync()
	if err == nil {
		t.Error("Sync() should fail when path is not a directory")
	}
}

func TestBinhost_SyncLocal_NonExistent(t *testing.T) {
	binhost, _ := NewBinhost("/nonexistent/path")

	err := binhost.Sync()
	if err == nil {
		t.Error("Sync() should fail for non-existent directory")
	}
}

func TestBinhost_SyncHTTP(t *testing.T) {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Packages" {
			content := `CPV: sys-libs/zlib-1.2.13
PATH: sys-libs/zlib-1.2.13.gpkg.tar
SIZE: 1024
SHA256: abc123
USE: ssl
EAPI: 8

CPV: app-misc/hello-2.10
PATH: app-misc/hello-2.10.gpkg.tar
SIZE: 512
`
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(content))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	binhost, _ := NewBinhost(server.URL)

	err := binhost.Sync()
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if len(binhost.Packages) != 2 {
		t.Errorf("Packages length = %d, want 2", len(binhost.Packages))
	}
}

func TestBinhost_SyncHTTP_Error(t *testing.T) {
	// Create mock HTTP server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	binhost, _ := NewBinhost(server.URL)

	err := binhost.Sync()
	if err == nil {
		t.Error("Sync() should fail on HTTP error")
	}
}

func TestBinhost_SyncRsync(t *testing.T) {
	binhost, _ := NewBinhost("rsync://rsync.gentoo.org/portage")

	err := binhost.Sync()
	if err == nil {
		t.Error("Sync() should fail for rsync (not implemented)")
	}
}

func TestBinhost_SyncSSH(t *testing.T) {
	binhost, _ := NewBinhost("ssh://user@host/path")

	err := binhost.Sync()
	if err == nil {
		t.Error("Sync() should fail for ssh (not implemented)")
	}
}

func TestBinhost_Find(t *testing.T) {
	binhost := &Binhost{
		Packages: []*BinaryPackage{
			{Package: &pkg.Package{Name: "sys-libs/zlib"}},
			{Package: &pkg.Package{Name: "sys-libs/glibc"}},
			{Package: &pkg.Package{Name: "app-misc/hello"}},
		},
	}

	tests := []struct {
		atom     string
		expected int
	}{
		{"zlib", 1},
		{"sys-libs", 2},
		{"hello", 1},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		result := binhost.Find(tt.atom)
		if len(result) != tt.expected {
			t.Errorf("Find(%q) returned %d results, want %d", tt.atom, len(result), tt.expected)
		}
	}
}

func TestBinhost_Find_NilPackage(t *testing.T) {
	binhost := &Binhost{
		Packages: []*BinaryPackage{
			{Package: nil}, // Nil package
			{Package: &pkg.Package{Name: "sys-libs/zlib"}},
		},
	}

	result := binhost.Find("zlib")
	if len(result) != 1 {
		t.Errorf("Find() returned %d results, want 1 (should skip nil packages)", len(result))
	}
}

func TestBinhost_Download_Local(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.gpkg.tar")
	srcContent := []byte("package content")
	if err := os.WriteFile(srcPath, srcContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Create destination path
	destPath := filepath.Join(tmpDir, "dest.gpkg.tar")

	binhost := &Binhost{Type: BinhostLocal}
	binPkg := &BinaryPackage{Path: srcPath}

	err := binhost.Download(binPkg, destPath)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	// Verify content
	destContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if !bytes.Equal(destContent, srcContent) {
		t.Error("Downloaded content doesn't match source")
	}
}

func TestBinhost_Download_HTTP(t *testing.T) {
	// Create mock HTTP server
	fileContent := []byte("binary package content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fileContent)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "downloaded.gpkg.tar")

	binhost := &Binhost{Type: BinhostHTTP}
	binPkg := &BinaryPackage{Path: server.URL + "/test.gpkg.tar"}

	err := binhost.Download(binPkg, destPath)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	// Verify content
	destContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if !bytes.Equal(destContent, fileContent) {
		t.Error("Downloaded content doesn't match")
	}
}

func TestBinhost_Download_HTTP_Error(t *testing.T) {
	// Create mock HTTP server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "downloaded.gpkg.tar")

	binhost := &Binhost{Type: BinhostHTTP}
	binPkg := &BinaryPackage{Path: server.URL + "/notfound.gpkg.tar"}

	err := binhost.Download(binPkg, destPath)
	if err == nil {
		t.Error("Download() should fail on HTTP error")
	}
}

func TestBinhost_Download_Unsupported(t *testing.T) {
	binhost := &Binhost{Type: BinhostRsync}
	binPkg := &BinaryPackage{Path: "rsync://test/pkg.gpkg.tar"}

	err := binhost.Download(binPkg, "/tmp/dest.gpkg.tar")
	if err == nil {
		t.Error("Download() should fail for unsupported binhost type")
	}
}

func TestParsePackagesIndex(t *testing.T) {
	content := `CPV: sys-libs/zlib-1.2.13
PATH: sys-libs/zlib-1.2.13.gpkg.tar
SIZE: 1024
SHA256: abc123
USE: ssl python
EAPI: 8

CPV: app-misc/hello-2.10
PATH: app-misc/hello-2.10.gpkg.tar
`

	packages, err := parsePackagesIndex(bytes.NewReader([]byte(content)), "http://example.com")
	if err != nil {
		t.Fatalf("parsePackagesIndex() error = %v", err)
	}

	if len(packages) != 2 {
		t.Errorf("parsePackagesIndex() returned %d packages, want 2", len(packages))
	}

	// Check first package
	if packages[0].Checksum != "abc123" {
		t.Errorf("First package checksum = %q, want %q", packages[0].Checksum, "abc123")
	}

	if packages[0].BuildInfo.EAPI != "8" {
		t.Errorf("First package EAPI = %q, want %q", packages[0].BuildInfo.EAPI, "8")
	}
}

func TestParsePackagesIndex_Empty(t *testing.T) {
	packages, err := parsePackagesIndex(bytes.NewReader([]byte("")), "http://example.com")
	if err != nil {
		t.Fatalf("parsePackagesIndex() error = %v", err)
	}

	if len(packages) != 0 {
		t.Errorf("parsePackagesIndex() returned %d packages for empty input, want 0", len(packages))
	}
}

func TestParsePackagesIndex_MissingCPV(t *testing.T) {
	content := `PATH: sys-libs/zlib-1.2.13.gpkg.tar
SIZE: 1024
`

	packages, err := parsePackagesIndex(bytes.NewReader([]byte(content)), "http://example.com")
	if err != nil {
		t.Fatalf("parsePackagesIndex() error = %v", err)
	}

	// Should skip entries without CPV
	if len(packages) != 0 {
		t.Errorf("parsePackagesIndex() returned %d packages, want 0 (missing CPV)", len(packages))
	}
}

func TestPackageFromIndexEntry(t *testing.T) {
	entry := map[string]string{
		"CPV":    "sys-libs/zlib-1.2.13",
		"PATH":   "sys-libs/zlib-1.2.13.gpkg.tar",
		"SIZE":   "1024",
		"SHA256": "abc123",
		"USE":    "ssl python",
		"EAPI":   "8",
	}

	pkg, err := packageFromIndexEntry(entry, "http://example.com")
	if err != nil {
		t.Fatalf("packageFromIndexEntry() error = %v", err)
	}

	if pkg.Checksum != "abc123" {
		t.Errorf("Checksum = %q, want %q", pkg.Checksum, "abc123")
	}

	if pkg.Format != FormatGPKG {
		t.Errorf("Format = %v, want %v", pkg.Format, FormatGPKG)
	}

	if len(pkg.BuildInfo.USE) != 2 {
		t.Errorf("USE flags count = %d, want 2", len(pkg.BuildInfo.USE))
	}
}

func TestPackageFromIndexEntry_MissingFields(t *testing.T) {
	tests := []struct {
		name  string
		entry map[string]string
	}{
		{
			name:  "missing_cpv",
			entry: map[string]string{"PATH": "test.gpkg.tar"},
		},
		{
			name:  "missing_path",
			entry: map[string]string{"CPV": "sys-libs/zlib-1.2.13"},
		},
		{
			name:  "invalid_cpv",
			entry: map[string]string{"CPV": "invalid", "PATH": "test.gpkg.tar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := packageFromIndexEntry(tt.entry, "http://example.com")
			if err == nil {
				t.Error("packageFromIndexEntry() should fail")
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	srcContent := []byte("test content")
	if err := os.WriteFile(srcPath, srcContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Copy file
	destPath := filepath.Join(tmpDir, "dest.txt")
	if err := copyFile(srcPath, destPath); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	// Verify content
	destContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(destContent, srcContent) {
		t.Error("Copied content doesn't match source")
	}
}

func TestCopyFile_SourceNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	err := copyFile("/nonexistent/source", filepath.Join(tmpDir, "dest"))
	if err == nil {
		t.Error("copyFile() should fail for non-existent source")
	}
}

func TestDownloadFile(t *testing.T) {
	fileContent := []byte("downloaded content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fileContent)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "downloaded.txt")

	if err := downloadFile(server.URL, destPath); err != nil {
		t.Fatalf("downloadFile() error = %v", err)
	}

	destContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(destContent, fileContent) {
		t.Error("Downloaded content doesn't match")
	}
}

func TestDownloadFile_InvalidURL(t *testing.T) {
	tmpDir := t.TempDir()
	err := downloadFile("://invalid-url", filepath.Join(tmpDir, "dest"))
	if err == nil {
		t.Error("downloadFile() should fail for invalid URL")
	}
}

func BenchmarkBinhost_Find(b *testing.B) {
	// Create binhost with many packages
	packages := make([]*BinaryPackage, 1000)
	for i := 0; i < 1000; i++ {
		packages[i] = &BinaryPackage{
			Package: &pkg.Package{Name: "category/pkg" + string(rune('a'+i%26))},
		}
	}

	binhost := &Binhost{Packages: packages}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = binhost.Find("pkgm")
	}
}

func BenchmarkParsePackagesIndex(b *testing.B) {
	// Create large Packages index
	var content bytes.Buffer
	for i := 0; i < 100; i++ {
		content.WriteString("CPV: sys-libs/pkg")
		content.WriteString(string(rune('0' + i%10)))
		content.WriteString("-1.0\n")
		content.WriteString("PATH: sys-libs/pkg")
		content.WriteString(string(rune('0' + i%10)))
		content.WriteString("-1.0.gpkg.tar\n")
		content.WriteString("SIZE: 1024\n")
		content.WriteString("USE: ssl python\n\n")
	}

	data := content.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parsePackagesIndex(bytes.NewReader(data), "http://example.com")
	}
}
