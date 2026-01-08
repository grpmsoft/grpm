package fetch

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestMirrorSelector tests the MirrorSelector functionality.
func TestMirrorSelector(t *testing.T) {
	t.Run("NewMirrorSelector with empty mirrors uses defaults", func(t *testing.T) {
		selector := NewMirrorSelector(nil)
		if selector.Len() != len(DefaultMirrors) {
			t.Errorf("expected %d mirrors, got %d", len(DefaultMirrors), selector.Len())
		}
	})

	t.Run("NewMirrorSelector normalizes URLs", func(t *testing.T) {
		mirrors := []string{
			"https://mirror1.example.com",
			"https://mirror2.example.com/",
			"  https://mirror3.example.com  ",
		}
		selector := NewMirrorSelector(mirrors)

		if selector.Len() != 3 {
			t.Errorf("expected 3 mirrors, got %d", selector.Len())
		}

		// All should have trailing slash
		for _, m := range selector.GetMirrors() {
			if !strings.HasSuffix(m, "/") {
				t.Errorf("mirror %q should have trailing slash", m)
			}
		}
	})

	t.Run("NewMirrorSelector skips empty mirrors", func(t *testing.T) {
		mirrors := []string{"https://mirror1.example.com", "", "  ", "https://mirror2.example.com"}
		selector := NewMirrorSelector(mirrors)

		if selector.Len() != 2 {
			t.Errorf("expected 2 mirrors, got %d", selector.Len())
		}
	})
}

// TestMirrorSelectorGetURIs tests URI generation.
func TestMirrorSelectorGetURIs(t *testing.T) {
	t.Run("GetURIs generates correct URIs", func(t *testing.T) {
		mirrors := []string{"https://example.com/"}
		selector := NewMirrorSelector(mirrors)

		uris := selector.GetURIs("hello-2.10.tar.gz")

		if len(uris) != 1 {
			t.Fatalf("expected 1 URI, got %d", len(uris))
		}

		expected := "https://example.com/distfiles/hello-2.10.tar.gz"
		if uris[0] != expected {
			t.Errorf("expected %q, got %q", expected, uris[0])
		}
	})

	t.Run("GetURIs handles mirrors with distfiles path", func(t *testing.T) {
		mirrors := []string{"https://example.com/distfiles/"}
		selector := NewMirrorSelector(mirrors)

		uris := selector.GetURIs("hello-2.10.tar.gz")

		if len(uris) != 1 {
			t.Fatalf("expected 1 URI, got %d", len(uris))
		}

		// Should not double the distfiles path
		if strings.Contains(uris[0], "distfiles/distfiles") {
			t.Errorf("URI should not have double distfiles path: %s", uris[0])
		}
	})
}

// TestMirrorSelectorReliability tests success/failure tracking.
func TestMirrorSelectorReliability(t *testing.T) {
	t.Run("ReportSuccess increases score", func(t *testing.T) {
		mirrors := []string{"https://mirror1.example.com/", "https://mirror2.example.com/"}
		selector := NewMirrorSelector(mirrors)

		selector.ReportSuccess("https://mirror2.example.com/")
		selector.ReportSuccess("https://mirror2.example.com/")

		// mirror2 should now be first
		ordered := selector.GetMirrors()
		if ordered[0] != "https://mirror2.example.com/" {
			t.Errorf("expected mirror2 first, got %s", ordered[0])
		}
	})

	t.Run("ReportFailure decreases score", func(t *testing.T) {
		mirrors := []string{"https://mirror1.example.com/", "https://mirror2.example.com/"}
		selector := NewMirrorSelector(mirrors)

		selector.ReportFailure("https://mirror1.example.com/")

		// mirror1 should now be last (failures penalized more)
		ordered := selector.GetMirrors()
		if ordered[0] != "https://mirror2.example.com/" {
			t.Errorf("expected mirror2 first after mirror1 failure, got %s", ordered[0])
		}
	})

	t.Run("GetStats returns copy", func(t *testing.T) {
		selector := NewMirrorSelector([]string{"https://example.com/"})
		selector.ReportSuccess("https://example.com/")
		selector.ReportFailure("https://example.com/")

		stats := selector.GetStats("https://example.com/")
		if stats == nil {
			t.Fatal("expected stats, got nil")
		}
		if stats.Successes != 1 {
			t.Errorf("expected 1 success, got %d", stats.Successes)
		}
		if stats.Failures != 1 {
			t.Errorf("expected 1 failure, got %d", stats.Failures)
		}
	})

	t.Run("GetStats returns nil for unknown mirror", func(t *testing.T) {
		selector := NewMirrorSelector([]string{"https://example.com/"})

		stats := selector.GetStats("https://unknown.example.com/")
		if stats != nil {
			t.Errorf("expected nil for unknown mirror, got %+v", stats)
		}
	})
}

// TestMirrorSelectorAddMirror tests adding mirrors.
func TestMirrorSelectorAddMirror(t *testing.T) {
	t.Run("AddMirror adds new mirror", func(t *testing.T) {
		selector := NewMirrorSelector([]string{"https://example1.com/"})
		selector.AddMirror("https://example2.com")

		if selector.Len() != 2 {
			t.Errorf("expected 2 mirrors, got %d", selector.Len())
		}
	})

	t.Run("AddMirror is idempotent", func(t *testing.T) {
		selector := NewMirrorSelector([]string{"https://example.com/"})
		selector.AddMirror("https://example.com/")
		selector.AddMirror("https://example.com")

		if selector.Len() != 1 {
			t.Errorf("expected 1 mirror, got %d", selector.Len())
		}
	})

	t.Run("AddMirror ignores empty string", func(t *testing.T) {
		selector := NewMirrorSelector([]string{"https://example.com/"})
		selector.AddMirror("")
		selector.AddMirror("  ")

		if selector.Len() != 1 {
			t.Errorf("expected 1 mirror, got %d", selector.Len())
		}
	})
}

// TestExtractMirrorBase tests mirror base extraction.
func TestExtractMirrorBase(t *testing.T) {
	tests := []struct {
		uri      string
		expected string
	}{
		{
			uri:      "https://distfiles.gentoo.org/distfiles/hello.tar.gz",
			expected: "https://distfiles.gentoo.org/",
		},
		{
			uri:      "https://mirrors.example.com/gentoo/distfiles/pkg.tar.gz",
			expected: "https://mirrors.example.com/",
		},
		{
			uri:      "http://example.com:8080/path/to/file.tar.gz",
			expected: "http://example.com:8080/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			result := ExtractMirrorBase(tt.uri)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestHTTPDownloader tests the HTTP downloader.
func TestHTTPDownloader(t *testing.T) {
	t.Run("NewHTTPDownloader applies defaults", func(t *testing.T) {
		downloader := NewHTTPDownloader(Config{})

		if downloader.config.MaxRetries != 3 {
			t.Errorf("expected MaxRetries 3, got %d", downloader.config.MaxRetries)
		}
		if downloader.config.Timeout != 300 {
			t.Errorf("expected Timeout 300, got %d", downloader.config.Timeout)
		}
		if downloader.config.Parallel != 1 {
			t.Errorf("expected Parallel 1, got %d", downloader.config.Parallel)
		}
	})
}

// TestHTTPDownloaderFetchOne tests single file download.
func TestHTTPDownloaderFetchOne(t *testing.T) {
	// Create test server
	testContent := []byte("Hello, World! This is test content for download.")
	checksums := computeTestChecksums(testContent)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/distfiles/test-file.txt" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testContent)))
			_, _ = w.Write(testContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	t.Run("FetchOne downloads and verifies file", func(t *testing.T) {
		tmpDir := t.TempDir()

		downloader := NewHTTPDownloader(Config{
			Mirrors:    []string{server.URL + "/"},
			MaxRetries: 1,
			Timeout:    10,
		})

		distfile := Distfile{
			Filename:  "test-file.txt",
			Size:      int64(len(testContent)),
			Checksums: checksums,
		}

		err := downloader.FetchOne(context.Background(), distfile, tmpDir)
		if err != nil {
			t.Fatalf("FetchOne failed: %v", err)
		}

		// Verify file exists
		destPath := filepath.Join(tmpDir, "test-file.txt")
		if !fileExists(destPath) {
			t.Fatal("downloaded file does not exist")
		}

		// Verify content
		content, err := os.ReadFile(destPath)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if string(content) != string(testContent) {
			t.Errorf("content mismatch: expected %q, got %q", testContent, content)
		}
	})

	t.Run("FetchOne skips already downloaded file", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Pre-create the file with correct content
		destPath := filepath.Join(tmpDir, "test-file.txt")
		if err := os.WriteFile(destPath, testContent, 0644); err != nil {
			t.Fatalf("creating test file: %v", err)
		}

		var requestCount int32
		countServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&requestCount, 1)
			_, _ = w.Write(testContent)
		}))
		defer countServer.Close()

		downloader := NewHTTPDownloader(Config{
			Mirrors:    []string{countServer.URL + "/"},
			MaxRetries: 1,
			Timeout:    10,
		})

		distfile := Distfile{
			Filename:  "test-file.txt",
			Size:      int64(len(testContent)),
			Checksums: checksums,
		}

		err := downloader.FetchOne(context.Background(), distfile, tmpDir)
		if err != nil {
			t.Fatalf("FetchOne failed: %v", err)
		}

		// Should not have made any requests
		if atomic.LoadInt32(&requestCount) != 0 {
			t.Errorf("expected 0 requests for cached file, got %d", requestCount)
		}
	})

	t.Run("FetchOne re-downloads on checksum mismatch", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Pre-create the file with wrong content
		destPath := filepath.Join(tmpDir, "test-file.txt")
		if err := os.WriteFile(destPath, []byte("wrong content"), 0644); err != nil {
			t.Fatalf("creating test file: %v", err)
		}

		downloader := NewHTTPDownloader(Config{
			Mirrors:    []string{server.URL + "/"},
			MaxRetries: 1,
			Timeout:    10,
		})

		distfile := Distfile{
			Filename:  "test-file.txt",
			Size:      int64(len(testContent)),
			Checksums: checksums,
		}

		err := downloader.FetchOne(context.Background(), distfile, tmpDir)
		if err != nil {
			t.Fatalf("FetchOne failed: %v", err)
		}

		// Verify content was replaced
		content, err := os.ReadFile(destPath)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if string(content) != string(testContent) {
			t.Errorf("content mismatch: expected %q, got %q", testContent, content)
		}
	})

	t.Run("FetchOne returns error for 404", func(t *testing.T) {
		tmpDir := t.TempDir()

		downloader := NewHTTPDownloader(Config{
			Mirrors:    []string{server.URL + "/"},
			MaxRetries: 1,
			Timeout:    10,
		})

		distfile := Distfile{
			Filename:  "nonexistent.txt",
			Size:      100,
			Checksums: NewChecksums("abc", "", ""),
		}

		err := downloader.FetchOne(context.Background(), distfile, tmpDir)
		if err == nil {
			t.Fatal("expected error for 404, got nil")
		}
	})

	t.Run("FetchOne respects context cancellation", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Server that delays response
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(5 * time.Second)
			_, _ = w.Write(testContent)
		}))
		defer slowServer.Close()

		downloader := NewHTTPDownloader(Config{
			Mirrors:    []string{slowServer.URL + "/"},
			MaxRetries: 1,
			Timeout:    30,
		})

		distfile := Distfile{
			Filename:  "test-file.txt",
			Size:      int64(len(testContent)),
			Checksums: checksums,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := downloader.FetchOne(ctx, distfile, tmpDir)
		if err == nil {
			t.Fatal("expected context cancellation error, got nil")
		}
	})
}

// TestHTTPDownloaderResume tests download resume functionality.
func TestHTTPDownloaderResume(t *testing.T) {
	testContent := []byte("AAAAAAAAAA" + "BBBBBBBBBB" + "CCCCCCCCCC") // 30 bytes
	checksums := computeTestChecksums(testContent)

	t.Run("Resume continues partial download", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create partial file with first 10 bytes
		partialPath := filepath.Join(tmpDir, "resume-test.txt.partial")
		if err := os.WriteFile(partialPath, testContent[:10], 0644); err != nil {
			t.Fatalf("creating partial file: %v", err)
		}

		var rangeHeader string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rangeHeader = r.Header.Get("Range")

			if rangeHeader != "" {
				// Resume request - return remaining content
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write(testContent[10:])
				return
			}

			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testContent)))
			_, _ = w.Write(testContent)
		}))
		defer server.Close()

		downloader := NewHTTPDownloader(Config{
			Mirrors:    []string{server.URL + "/"},
			MaxRetries: 1,
			Timeout:    10,
			Resume:     true,
		})

		distfile := Distfile{
			Filename:  "resume-test.txt",
			Size:      int64(len(testContent)),
			Checksums: checksums,
		}

		err := downloader.FetchOne(context.Background(), distfile, tmpDir)
		if err != nil {
			t.Fatalf("FetchOne failed: %v", err)
		}

		// Verify Range header was sent
		if rangeHeader != "bytes=10-" {
			t.Errorf("expected Range header 'bytes=10-', got %q", rangeHeader)
		}

		// Verify final content
		destPath := filepath.Join(tmpDir, "resume-test.txt")
		content, err := os.ReadFile(destPath)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if string(content) != string(testContent) {
			t.Errorf("content mismatch: expected %q, got %q", testContent, content)
		}
	})

	t.Run("Resume disabled does not send Range header", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create partial file
		partialPath := filepath.Join(tmpDir, "resume-test.txt.partial")
		if err := os.WriteFile(partialPath, testContent[:10], 0644); err != nil {
			t.Fatalf("creating partial file: %v", err)
		}

		var rangeHeader string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rangeHeader = r.Header.Get("Range")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testContent)))
			_, _ = w.Write(testContent)
		}))
		defer server.Close()

		downloader := NewHTTPDownloader(Config{
			Mirrors:    []string{server.URL + "/"},
			MaxRetries: 1,
			Timeout:    10,
			Resume:     false,
		})

		distfile := Distfile{
			Filename:  "resume-test.txt",
			Size:      int64(len(testContent)),
			Checksums: checksums,
		}

		err := downloader.FetchOne(context.Background(), distfile, tmpDir)
		if err != nil {
			t.Fatalf("FetchOne failed: %v", err)
		}

		// Verify Range header was NOT sent
		if rangeHeader != "" {
			t.Errorf("expected no Range header, got %q", rangeHeader)
		}
	})
}

// TestHTTPDownloaderFetch tests batch download.
func TestHTTPDownloaderFetch(t *testing.T) {
	content1 := []byte("file1 content")
	content2 := []byte("file2 content")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/distfiles/file1.txt":
			_, _ = w.Write(content1)
		case "/distfiles/file2.txt":
			_, _ = w.Write(content2)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Run("Fetch downloads multiple files", func(t *testing.T) {
		tmpDir := t.TempDir()

		downloader := NewHTTPDownloader(Config{
			Mirrors:    []string{server.URL + "/"},
			MaxRetries: 1,
			Timeout:    10,
		})

		distfiles := []Distfile{
			{
				Filename:  "file1.txt",
				Size:      int64(len(content1)),
				Checksums: computeTestChecksums(content1),
			},
			{
				Filename:  "file2.txt",
				Size:      int64(len(content2)),
				Checksums: computeTestChecksums(content2),
			},
		}

		err := downloader.Fetch(context.Background(), distfiles, tmpDir)
		if err != nil {
			t.Fatalf("Fetch failed: %v", err)
		}

		// Verify both files exist
		for _, name := range []string{"file1.txt", "file2.txt"} {
			if !fileExists(filepath.Join(tmpDir, name)) {
				t.Errorf("file %s does not exist", name)
			}
		}
	})

	t.Run("Fetch returns error on first failure", func(t *testing.T) {
		tmpDir := t.TempDir()

		downloader := NewHTTPDownloader(Config{
			Mirrors:    []string{server.URL + "/"},
			MaxRetries: 1,
			Timeout:    10,
		})

		distfiles := []Distfile{
			{
				Filename:  "nonexistent.txt",
				Size:      100,
				Checksums: NewChecksums("abc", "", ""),
			},
			{
				Filename:  "file1.txt",
				Size:      int64(len(content1)),
				Checksums: computeTestChecksums(content1),
			},
		}

		err := downloader.Fetch(context.Background(), distfiles, tmpDir)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("Fetch with empty distfiles succeeds", func(t *testing.T) {
		tmpDir := t.TempDir()

		downloader := NewHTTPDownloader(Config{
			Mirrors:    []string{server.URL + "/"},
			MaxRetries: 1,
			Timeout:    10,
		})

		err := downloader.Fetch(context.Background(), nil, tmpDir)
		if err != nil {
			t.Fatalf("Fetch with empty distfiles should succeed, got: %v", err)
		}
	})
}

// TestHTTPDownloaderMirrorFailover tests mirror failover behavior.
func TestHTTPDownloaderMirrorFailover(t *testing.T) {
	testContent := []byte("success content")
	checksums := computeTestChecksums(testContent)

	var mirror1Requests, mirror2Requests int32

	// Mirror 1 - always fails
	mirror1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&mirror1Requests, 1)
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer mirror1.Close()

	// Mirror 2 - always succeeds
	mirror2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&mirror2Requests, 1)
		_, _ = w.Write(testContent)
	}))
	defer mirror2.Close()

	t.Run("Failover to second mirror on first failure", func(t *testing.T) {
		tmpDir := t.TempDir()
		atomic.StoreInt32(&mirror1Requests, 0)
		atomic.StoreInt32(&mirror2Requests, 0)

		downloader := NewHTTPDownloader(Config{
			Mirrors:    []string{mirror1.URL + "/", mirror2.URL + "/"},
			MaxRetries: 1,
			Timeout:    10,
		})

		distfile := Distfile{
			Filename:  "test.txt",
			Size:      int64(len(testContent)),
			Checksums: checksums,
		}

		err := downloader.FetchOne(context.Background(), distfile, tmpDir)
		if err != nil {
			t.Fatalf("FetchOne failed: %v", err)
		}

		// Both mirrors should have received requests
		if atomic.LoadInt32(&mirror1Requests) == 0 {
			t.Error("mirror1 should have received requests")
		}
		if atomic.LoadInt32(&mirror2Requests) == 0 {
			t.Error("mirror2 should have received requests")
		}

		// File should exist with correct content
		content, err := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if string(content) != string(testContent) {
			t.Errorf("content mismatch: expected %q, got %q", testContent, content)
		}
	})
}

// TestHTTPDownloaderProgress tests progress callback.
func TestHTTPDownloaderProgress(t *testing.T) {
	testContent := []byte("content for progress test - needs to be reasonably sized")
	checksums := computeTestChecksums(testContent)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testContent)))
		_, _ = w.Write(testContent)
	}))
	defer server.Close()

	t.Run("Progress callback is called", func(t *testing.T) {
		tmpDir := t.TempDir()

		downloader := NewHTTPDownloader(Config{
			Mirrors:    []string{server.URL + "/"},
			MaxRetries: 1,
			Timeout:    10,
		})

		var progressCalls int32
		var lastDownloaded int64
		downloader.SetProgressCallback(func(filename string, downloaded, total int64) {
			atomic.AddInt32(&progressCalls, 1)
			lastDownloaded = downloaded
		})

		distfile := Distfile{
			Filename:  "progress-test.txt",
			Size:      int64(len(testContent)),
			Checksums: checksums,
		}

		err := downloader.FetchOne(context.Background(), distfile, tmpDir)
		if err != nil {
			t.Fatalf("FetchOne failed: %v", err)
		}

		// Progress should have been called at least once
		if atomic.LoadInt32(&progressCalls) == 0 {
			t.Error("progress callback was never called")
		}

		// Last downloaded value should equal file size
		if lastDownloaded != int64(len(testContent)) {
			t.Errorf("expected last downloaded %d, got %d", len(testContent), lastDownloaded)
		}
	})
}

// TestMirrorStats tests MirrorStats score calculation.
func TestMirrorStats(t *testing.T) {
	tests := []struct {
		name      string
		successes int
		failures  int
		expected  int
	}{
		{name: "zero stats", successes: 0, failures: 0, expected: 0},
		{name: "only successes", successes: 5, failures: 0, expected: 5},
		{name: "only failures", successes: 0, failures: 3, expected: -6},
		{name: "mixed positive", successes: 10, failures: 2, expected: 6},
		{name: "mixed negative", successes: 2, failures: 5, expected: -8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := MirrorStats{
				Successes: tt.successes,
				Failures:  tt.failures,
			}
			if score := stats.Score(); score != tt.expected {
				t.Errorf("expected score %d, got %d", tt.expected, score)
			}
		})
	}
}

// computeTestChecksums computes checksums for test content.
func computeTestChecksums(content []byte) Checksums {
	// Create temp file
	f, err := os.CreateTemp("", "checksum-test-*")
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()

	if _, err := f.Write(content); err != nil {
		panic(err)
	}
	_ = f.Close()

	checksums, err := ComputeChecksums(f.Name())
	if err != nil {
		panic(err)
	}
	return checksums
}
