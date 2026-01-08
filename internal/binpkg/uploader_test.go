package binpkg

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// TestHTTPUploader_Upload tests HTTP package upload
func TestHTTPUploader_Upload(t *testing.T) {
	// Create test package file
	pkgDir := t.TempDir()
	pkgPath := filepath.Join(pkgDir, "test-1.0.0.gpkg.tar")
	testContent := "test package content"
	if err := os.WriteFile(pkgPath, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to create test package: %v", err)
	}

	// Create test signature
	sigPath := pkgPath + ".sig"
	if err := os.WriteFile(sigPath, []byte("test signature"), 0644); err != nil {
		t.Fatalf("failed to create signature: %v", err)
	}

	// Create test HTTP server
	uploadedFiles := make(map[string][]byte)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT request, got %s", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Read uploaded content
		content, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		uploadedFiles[r.URL.Path] = content
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create test package
	testPkg := &BinaryPackage{
		Package:   pkg.NewPackage("app-test/testpkg", "1.0.0", "0"),
		Format:    FormatGPKG,
		Path:      pkgPath,
		Signature: &Signature{Type: SignatureGPG},
	}

	// Create uploader
	uploader := NewHTTPUploader(server.URL)
	uploader.Verbose = false

	// Execute
	err := uploader.Upload(testPkg)

	// Verify
	if err != nil {
		t.Fatalf("Upload() failed: %v", err)
	}

	// Check package uploaded
	expectedPath := "/app-test/testpkg-1.0.0.gpkg.tar"
	if content, ok := uploadedFiles[expectedPath]; !ok {
		t.Errorf("package not uploaded to expected path: %s", expectedPath)
	} else if string(content) != testContent {
		t.Errorf("uploaded content mismatch: expected %q, got %q", testContent, string(content))
	}

	// Check signature uploaded
	expectedSigPath := expectedPath + ".sig"
	if _, ok := uploadedFiles[expectedSigPath]; !ok {
		t.Errorf("signature not uploaded to expected path: %s", expectedSigPath)
	}

	t.Logf("Package uploaded successfully to %s", expectedPath)
}

// TestHTTPUploader_UploadWithAuth tests HTTP upload with authentication
func TestHTTPUploader_UploadWithAuth(t *testing.T) {
	expectedUsername := "testuser"
	expectedPassword := "testpass"

	// Create test server with auth check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			t.Error("no basic auth provided")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if username != expectedUsername || password != expectedPassword {
			t.Errorf("invalid credentials: got %s:%s", username, password)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create test file
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create uploader with auth
	uploader := NewHTTPUploader(server.URL)
	uploader.SetAuth(expectedUsername, expectedPassword)

	// Execute
	err := uploader.UploadFile(tmpFile, "/test/file.txt")

	// Verify
	if err != nil {
		t.Fatalf("UploadFile() with auth failed: %v", err)
	}

	t.Logf("Authenticated upload successful")
}

// TestHTTPUploader_UploadPOSTMultipart tests POST multipart upload
func TestHTTPUploader_UploadPOSTMultipart(t *testing.T) {
	// Create test server
	receivedFilename := ""
	receivedContent := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST request, got %s", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "failed to parse multipart", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "no file in form", http.StatusBadRequest)
			return
		}
		defer func() { _ = file.Close() }()

		receivedFilename = header.Filename

		content, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "failed to read file", http.StatusInternalServerError)
			return
		}
		receivedContent = string(content)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create test file
	tmpFile := filepath.Join(t.TempDir(), "upload.txt")
	testContent := "multipart test content"
	if err := os.WriteFile(tmpFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create uploader with POST multipart
	uploader := NewHTTPUploader(server.URL)
	uploader.Method = "POST"
	uploader.UseMultipart = true

	// Execute
	err := uploader.UploadFile(tmpFile, "/uploads/upload.txt")

	// Verify
	if err != nil {
		t.Fatalf("UploadFile() multipart failed: %v", err)
	}

	if receivedFilename != "upload.txt" {
		t.Errorf("expected filename %q, got %q", "upload.txt", receivedFilename)
	}

	if receivedContent != testContent {
		t.Errorf("expected content %q, got %q", testContent, receivedContent)
	}

	t.Logf("POST multipart upload successful")
}

// TestHTTPUploader_UploadDirectory tests directory upload
func TestHTTPUploader_UploadDirectory(t *testing.T) {
	// Create test directory with files
	testDir := t.TempDir()

	files := map[string]string{
		"file1.txt":        "content 1",
		"subdir/file2.txt": "content 2",
		"subdir/file3.txt": "content 3",
	}

	for path, content := range files {
		fullPath := filepath.Join(testDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", path, err)
		}
	}

	// Create test server
	uploadedFiles := make(map[string]string)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content, _ := io.ReadAll(r.Body)
		uploadedFiles[r.URL.Path] = string(content)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create uploader
	uploader := NewHTTPUploader(server.URL)

	// Execute
	err := uploader.UploadDirectory(testDir, "/remote/dir")

	// Verify
	if err != nil {
		t.Fatalf("UploadDirectory() failed: %v", err)
	}

	if len(uploadedFiles) != len(files) {
		t.Errorf("expected %d files uploaded, got %d", len(files), len(uploadedFiles))
	}

	t.Logf("Directory uploaded successfully (%d files)", len(uploadedFiles))
}

// TestHTTPUploader_ErrorHandling tests HTTP error cases
func TestHTTPUploader_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func() (*httptest.Server, string) // returns server and test file path
		wantErrMsg string
	}{
		{
			name: "404 not found",
			setupFunc: func() (*httptest.Server, string) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Error(w, "not found", http.StatusNotFound)
				}))
				tmpFile := filepath.Join(os.TempDir(), "test.txt")
				_ = os.WriteFile(tmpFile, []byte("test"), 0644)
				return server, tmpFile
			},
			wantErrMsg: "HTTP error 404",
		},
		{
			name: "500 internal server error",
			setupFunc: func() (*httptest.Server, string) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Error(w, "internal error", http.StatusInternalServerError)
				}))
				tmpFile := filepath.Join(os.TempDir(), "test.txt")
				_ = os.WriteFile(tmpFile, []byte("test"), 0644)
				return server, tmpFile
			},
			wantErrMsg: "HTTP error 500",
		},
		{
			name: "non-existent file",
			setupFunc: func() (*httptest.Server, string) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				return server, "/nonexistent/file.txt"
			},
			wantErrMsg: "failed to open file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, tmpFile := tt.setupFunc()
			defer server.Close()
			defer func() { _ = os.Remove(tmpFile) }()

			uploader := NewHTTPUploader(server.URL)
			err := uploader.UploadFile(tmpFile, "/test/upload.txt")

			if err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("expected error containing %q, got %q", tt.wantErrMsg, err.Error())
			}
		})
	}
}

// TestSSHUploader_NewSSHUploader tests SSH uploader creation
func TestSSHUploader_NewSSHUploader(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantErr  bool
		wantHost string
		wantPort int
		wantPath string
	}{
		{
			name:     "valid SSH URL",
			url:      "ssh://user@example.com/var/binhost",
			wantErr:  false,
			wantHost: "user@example.com",
			wantPort: 22,
			wantPath: "/var/binhost",
		},
		{
			name:     "SSH URL with port",
			url:      "ssh://user@example.com:2222/binhost",
			wantErr:  false,
			wantHost: "user@example.com",
			wantPort: 2222,
			wantPath: "/binhost",
		},
		{
			name:    "invalid scheme",
			url:     "http://example.com/path",
			wantErr: true,
		},
		{
			name:    "invalid URL",
			url:     "not-a-valid-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uploader, err := NewSSHUploader(tt.url)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewSSHUploader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if uploader.Host != tt.wantHost {
				t.Errorf("expected host %q, got %q", tt.wantHost, uploader.Host)
			}
			if uploader.Port != tt.wantPort {
				t.Errorf("expected port %d, got %d", tt.wantPort, uploader.Port)
			}
			if uploader.RemoteDir != tt.wantPath {
				t.Errorf("expected path %q, got %q", tt.wantPath, uploader.RemoteDir)
			}
		})
	}
}

// TestHTTPUploader_SetTimeout tests timeout configuration
func TestHTTPUploader_SetTimeout(t *testing.T) {
	uploader := NewHTTPUploader("http://example.com")

	// Test default timeout
	if uploader.HTTPClient.Timeout != 30*time.Minute {
		t.Errorf("expected default timeout 30min, got %v", uploader.HTTPClient.Timeout)
	}

	// Set custom timeout
	customTimeout := 10 * time.Second
	uploader.SetTimeout(customTimeout)

	if uploader.HTTPClient.Timeout != customTimeout {
		t.Errorf("expected timeout %v, got %v", customTimeout, uploader.HTTPClient.Timeout)
	}

	t.Logf("Timeout configuration successful")
}

// TestHTTPUploader_InvalidPackage tests upload with invalid package
func TestHTTPUploader_InvalidPackage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	uploader := NewHTTPUploader(server.URL)

	tests := []struct {
		name    string
		pkg     *BinaryPackage
		wantErr string
	}{
		{
			name:    "nil package",
			pkg:     nil,
			wantErr: "package cannot be nil",
		},
		{
			name: "invalid package name",
			pkg: &BinaryPackage{
				Package: pkg.NewPackage("invalid-no-category", "1.0.0", "0"),
			},
			wantErr: "invalid package name format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := uploader.Upload(tt.pkg)

			if err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

// BenchmarkHTTPUploader_Upload benchmarks HTTP upload
func BenchmarkHTTPUploader_Upload(b *testing.B) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create test package file
	pkgDir := b.TempDir()
	pkgPath := filepath.Join(pkgDir, "bench-1.0.0.gpkg.tar")
	if err := os.WriteFile(pkgPath, make([]byte, 1024*100), 0644); err != nil { // 100KB
		b.Fatalf("failed to create test package: %v", err)
	}

	testPkg := &BinaryPackage{
		Package: pkg.NewPackage("app-bench/test", "1.0.0", "0"),
		Format:  FormatGPKG,
		Path:    pkgPath,
	}

	uploader := NewHTTPUploader(server.URL)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := uploader.Upload(testPkg); err != nil {
			b.Fatalf("Upload() failed: %v", err)
		}
	}
}

// BenchmarkHTTPUploader_UploadFilePUT benchmarks PUT upload
func BenchmarkHTTPUploader_UploadFilePUT(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create test file (1MB)
	tmpFile := filepath.Join(b.TempDir(), "bench.dat")
	if err := os.WriteFile(tmpFile, make([]byte, 1024*1024), 0644); err != nil {
		b.Fatalf("failed to create test file: %v", err)
	}

	uploader := NewHTTPUploader(server.URL)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := uploader.UploadFile(tmpFile, fmt.Sprintf("/test/file%d.dat", i)); err != nil {
			b.Fatalf("UploadFile() failed: %v", err)
		}
	}
}
