//go:build integration

// Package fetch integration tests.
//
// These tests verify the fetch system works correctly in realistic scenarios.
// Some tests use mock HTTP servers for reliable CI execution, while others
// can optionally test against real Gentoo mirrors when network is available.
//
// Run with: go test -v -tags=integration ./internal/fetch/...
package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// Mock HTTP Server Tests (reliable for CI)
// =============================================================================

// TestIntegration_FullDownloadWorkflow tests the complete download workflow
// using a mock HTTP server.
func TestIntegration_FullDownloadWorkflow(t *testing.T) {
	// Simulate a Gentoo distfiles server with multiple packages
	packages := map[string][]byte{
		"hello-2.10.tar.gz":  []byte("fake hello tarball content - this would be real source code"),
		"zlib-1.2.13.tar.gz": []byte("fake zlib tarball content - compression library source"),
		"openssl-3.0.tar.gz": []byte("fake openssl tarball - cryptographic library"),
	}

	// Precompute checksums
	checksumMap := make(map[string]Checksums)
	for name, content := range packages {
		checksumMap[name] = computeTestChecksumsFromBytes(content)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract filename from path: /distfiles/filename
		filename := filepath.Base(r.URL.Path)
		content, ok := packages[filename]
		if !ok {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(content)
	}))
	defer server.Close()

	t.Run("download multiple distfiles", func(t *testing.T) {
		tmpDir := t.TempDir()

		config := Config{
			DistDir:    tmpDir,
			Mirrors:    []string{server.URL + "/"},
			MaxRetries: 2,
			Timeout:    30,
			Resume:     true,
		}

		downloader := NewHTTPDownloader(config)

		// Create distfile list
		var distfiles []Distfile
		for name, content := range packages {
			df := NewDistfile(name, int64(len(content)), checksumMap[name])
			distfiles = append(distfiles, df)
		}

		// Download all
		err := downloader.Fetch(context.Background(), distfiles, tmpDir)
		if err != nil {
			t.Fatalf("Fetch failed: %v", err)
		}

		// Verify all files exist with correct checksums
		for name := range packages {
			path := filepath.Join(tmpDir, name)
			if !fileExists(path) {
				t.Errorf("file %s was not downloaded", name)
				continue
			}

			// Verify checksum
			if err := Verify(path, checksumMap[name]); err != nil {
				t.Errorf("checksum verification failed for %s: %v", name, err)
			}
		}
	})

	t.Run("skip already downloaded files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Pre-create one file with correct content
		preExisting := "hello-2.10.tar.gz"
		content := packages[preExisting]
		destPath := filepath.Join(tmpDir, preExisting)
		if err := os.WriteFile(destPath, content, 0644); err != nil {
			t.Fatalf("creating pre-existing file: %v", err)
		}

		var requestCount int32
		countServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&requestCount, 1)
			filename := filepath.Base(r.URL.Path)
			if c, ok := packages[filename]; ok {
				_, _ = w.Write(c)
			} else {
				http.NotFound(w, r)
			}
		}))
		defer countServer.Close()

		downloader := NewHTTPDownloader(Config{
			Mirrors:    []string{countServer.URL + "/"},
			MaxRetries: 1,
			Timeout:    10,
		})

		// Request only the pre-existing file
		df := NewDistfile(preExisting, int64(len(content)), checksumMap[preExisting])
		err := downloader.FetchOne(context.Background(), df, tmpDir)
		if err != nil {
			t.Fatalf("FetchOne failed: %v", err)
		}

		// Should not have made any requests (file already valid)
		if atomic.LoadInt32(&requestCount) != 0 {
			t.Errorf("expected 0 requests for cached file, got %d", requestCount)
		}
	})
}

// TestIntegration_MirrorFailoverChain tests failover across multiple mirrors.
func TestIntegration_MirrorFailoverChain(t *testing.T) {
	testContent := []byte("success after failover chain")
	checksums := computeTestChecksumsFromBytes(testContent)

	var mirror1Hits, mirror2Hits, mirror3Hits int32

	// Mirror 1 - always 500 error
	mirror1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&mirror1Hits, 1)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer mirror1.Close()

	// Mirror 2 - always 404
	mirror2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&mirror2Hits, 1)
		http.NotFound(w, r)
	}))
	defer mirror2.Close()

	// Mirror 3 - success
	mirror3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&mirror3Hits, 1)
		_, _ = w.Write(testContent)
	}))
	defer mirror3.Close()

	tmpDir := t.TempDir()

	downloader := NewHTTPDownloader(Config{
		Mirrors:    []string{mirror1.URL + "/", mirror2.URL + "/", mirror3.URL + "/"},
		MaxRetries: 1,
		Timeout:    10,
	})

	distfile := NewDistfile("failover-test.tar.gz", int64(len(testContent)), checksums)
	err := downloader.FetchOne(context.Background(), distfile, tmpDir)
	if err != nil {
		t.Fatalf("FetchOne failed: %v", err)
	}

	// Verify all mirrors were tried in order
	if atomic.LoadInt32(&mirror1Hits) == 0 {
		t.Error("mirror1 should have been tried")
	}
	if atomic.LoadInt32(&mirror2Hits) == 0 {
		t.Error("mirror2 should have been tried")
	}
	if atomic.LoadInt32(&mirror3Hits) == 0 {
		t.Error("mirror3 should have been tried (and succeeded)")
	}

	// Verify file was downloaded
	destPath := filepath.Join(tmpDir, "failover-test.tar.gz")
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(content) != string(testContent) {
		t.Errorf("content mismatch after failover")
	}
}

// TestIntegration_MirrorReliabilityTracking tests that mirror reliability
// affects selection order and downloads use the prioritized mirrors.
func TestIntegration_MirrorReliabilityTracking(t *testing.T) {
	successContent := []byte("reliable mirror content")
	checksums := computeTestChecksumsFromBytes(successContent)

	// Track which mirror actually served the download
	var lastServingMirror string

	// Create 3 mirrors that all serve the same content
	mirror1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastServingMirror = "mirror1"
		_, _ = w.Write(successContent)
	}))
	defer mirror1.Close()

	mirror2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastServingMirror = "mirror2"
		_, _ = w.Write(successContent)
	}))
	defer mirror2.Close()

	mirror3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastServingMirror = "mirror3"
		_, _ = w.Write(successContent)
	}))
	defer mirror3.Close()

	// Create downloader with all three mirrors
	downloader := NewHTTPDownloader(Config{
		Mirrors:    []string{mirror1.URL + "/", mirror2.URL + "/", mirror3.URL + "/"},
		MaxRetries: 1,
		Timeout:    10,
	})

	// First download should use mirror1 (first in list)
	tmpDir := t.TempDir()
	distfile := NewDistfile("test1.tar.gz", int64(len(successContent)), checksums)
	err := downloader.FetchOne(context.Background(), distfile, tmpDir)
	if err != nil {
		t.Fatalf("first download failed: %v", err)
	}
	if lastServingMirror != "mirror1" {
		t.Errorf("expected mirror1 to serve first download, got %s", lastServingMirror)
	}

	// Now simulate reliability data: mirror2 is most reliable
	downloader.mirrors.ReportSuccess(mirror2.URL + "/")
	downloader.mirrors.ReportSuccess(mirror2.URL + "/")
	downloader.mirrors.ReportSuccess(mirror2.URL + "/")
	downloader.mirrors.ReportFailure(mirror1.URL + "/")
	downloader.mirrors.ReportFailure(mirror3.URL + "/")

	// Verify mirror2 is now first in the ordering
	ordered := downloader.mirrors.GetMirrors()
	if ordered[0] != mirror2.URL+"/" {
		t.Errorf("expected mirror2 first after reliability update, got %s", ordered[0])
	}

	// Second download should use mirror2 (now first due to reliability)
	tmpDir2 := t.TempDir()
	distfile2 := NewDistfile("test2.tar.gz", int64(len(successContent)), checksums)
	err = downloader.FetchOne(context.Background(), distfile2, tmpDir2)
	if err != nil {
		t.Fatalf("second download failed: %v", err)
	}
	if lastServingMirror != "mirror2" {
		t.Errorf("expected mirror2 to serve second download (most reliable), got %s", lastServingMirror)
	}
}

// TestIntegration_ManifestParsingRealistic tests parsing a realistic Manifest file.
func TestIntegration_ManifestParsingRealistic(t *testing.T) {
	// This is a realistic Manifest content from a Gentoo package
	manifestContent := `DIST hello-2.10.tar.gz 725946 BLAKE2B d60928e18a46e9afa2c84c265a2b9c53a6f8d91e45f7e3f9e3a9bae764de9c4f5b8e7a6d2c3b1a0e9f8d7c6b5a4f3e2d1c0b9a8e7f6d5c4b3a2e1f0d9c8b7a6e5 SHA512 defabc123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456
DIST hello-2.12.1.tar.gz 1033163 BLAKE2B a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456 SHA512 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
EBUILD hello-2.10.ebuild 745 BLAKE2B e1d2c3b4a5968778899001122334455667788990011223344556677889900112233445566778899001122334455667788990011223344556677889900112233445566 SHA512 fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210
EBUILD hello-2.12.1.ebuild 789 BLAKE2B f0e1d2c3b4a596877889900112233445566778899001122334455667788990011223344556677889900112233445566778899001122334455667788990011223344 SHA512 0fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba987654321
AUX fix-makefile.patch 234 BLAKE2B 1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef SHA512 abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890
MISC metadata.xml 567 BLAKE2B 9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba SHA512 fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321
`

	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "Manifest")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("creating test Manifest: %v", err)
	}

	t.Run("parse full manifest", func(t *testing.T) {
		manifest, err := ParseManifest(manifestPath)
		if err != nil {
			t.Fatalf("ParseManifest failed: %v", err)
		}

		// Verify counts by type
		if len(manifest.Entries) != 6 {
			t.Errorf("expected 6 total entries, got %d", len(manifest.Entries))
		}
		if len(manifest.DistFiles) != 2 {
			t.Errorf("expected 2 DIST entries, got %d", len(manifest.DistFiles))
		}
	})

	t.Run("get distfiles for download", func(t *testing.T) {
		manifest, err := ParseManifest(manifestPath)
		if err != nil {
			t.Fatalf("ParseManifest failed: %v", err)
		}

		distfiles := manifest.GetDistfiles()
		if len(distfiles) != 2 {
			t.Errorf("expected 2 distfiles, got %d", len(distfiles))
		}

		// Verify all distfiles are valid
		for _, df := range distfiles {
			if !df.IsValid() {
				t.Errorf("distfile %q is not valid", df.Filename)
			}
			if df.Checksums.BLAKE2B == "" {
				t.Errorf("distfile %q missing BLAKE2B checksum", df.Filename)
			}
			if df.Checksums.SHA512 == "" {
				t.Errorf("distfile %q missing SHA512 checksum", df.Filename)
			}
		}
	})

	t.Run("verify entry details", func(t *testing.T) {
		manifest, err := ParseManifest(manifestPath)
		if err != nil {
			t.Fatalf("ParseManifest failed: %v", err)
		}

		entry, ok := manifest.GetEntry("hello-2.10.tar.gz")
		if !ok {
			t.Fatal("entry hello-2.10.tar.gz not found")
		}

		if entry.Type != EntryTypeDist {
			t.Errorf("expected DIST type, got %s", entry.Type)
		}
		if entry.Size != 725946 {
			t.Errorf("expected size 725946, got %d", entry.Size)
		}
	})
}

// TestIntegration_ChecksumVerificationWorkflow tests the checksum workflow.
func TestIntegration_ChecksumVerificationWorkflow(t *testing.T) {
	// Create a test file with known content
	testContent := []byte("Integration test content for checksum verification")
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test-checksum.txt")

	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	t.Run("compute and verify all checksums", func(t *testing.T) {
		// Compute checksums
		checksums, err := ComputeChecksums(testFile)
		if err != nil {
			t.Fatalf("ComputeChecksums failed: %v", err)
		}

		// Verify all algorithms are populated
		if checksums.SHA256 == "" {
			t.Error("SHA256 should not be empty")
		}
		if checksums.SHA512 == "" {
			t.Error("SHA512 should not be empty")
		}
		if checksums.BLAKE2B == "" {
			t.Error("BLAKE2B should not be empty")
		}

		// Verify the file with computed checksums
		if err := Verify(testFile, checksums); err != nil {
			t.Errorf("Verify failed with computed checksums: %v", err)
		}

		// Verify all checksums match
		results, err := VerifyAll(testFile, checksums)
		if err != nil {
			t.Errorf("VerifyAll failed: %v", err)
		}
		if len(results) != 3 {
			t.Errorf("expected 3 results, got %d", len(results))
		}
		for _, r := range results {
			if !r.Match {
				t.Errorf("%s checksum mismatch", r.Algorithm)
			}
		}
	})

	t.Run("detect corrupted file", func(t *testing.T) {
		// Compute original checksums
		originalChecksums, err := ComputeChecksums(testFile)
		if err != nil {
			t.Fatalf("ComputeChecksums failed: %v", err)
		}

		// Create corrupted copy
		corruptedFile := filepath.Join(tmpDir, "corrupted.txt")
		corruptedContent := append(testContent, []byte(" CORRUPTED")...)
		if err := os.WriteFile(corruptedFile, corruptedContent, 0644); err != nil {
			t.Fatalf("creating corrupted file: %v", err)
		}

		// Verify should fail
		err = Verify(corruptedFile, originalChecksums)
		if err == nil {
			t.Error("expected error for corrupted file")
		}
		if !errors.Is(err, ErrChecksumMismatch) {
			t.Errorf("expected ErrChecksumMismatch, got: %v", err)
		}
	})
}

// TestIntegration_DownloadWithProgressReporting tests progress callback.
func TestIntegration_DownloadWithProgressReporting(t *testing.T) {
	// Create larger content to ensure multiple progress callbacks
	testContent := make([]byte, 100*1024) // 100KB
	for i := range testContent {
		testContent[i] = byte(i % 256)
	}
	checksums := computeTestChecksumsFromBytes(testContent)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testContent)))
		// Write in chunks to trigger progress callbacks
		chunkSize := 8192
		for i := 0; i < len(testContent); i += chunkSize {
			end := i + chunkSize
			if end > len(testContent) {
				end = len(testContent)
			}
			_, _ = w.Write(testContent[i:end])
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()

	downloader := NewHTTPDownloader(Config{
		Mirrors:    []string{server.URL + "/"},
		MaxRetries: 1,
		Timeout:    30,
	})

	var progressCalls int32
	var seenFilename string
	var maxDownloaded, reportedTotal int64

	downloader.SetProgressCallback(func(filename string, downloaded, total int64) {
		atomic.AddInt32(&progressCalls, 1)
		seenFilename = filename
		if downloaded > maxDownloaded {
			maxDownloaded = downloaded
		}
		reportedTotal = total
	})

	distfile := NewDistfile("large-file.bin", int64(len(testContent)), checksums)
	err := downloader.FetchOne(context.Background(), distfile, tmpDir)
	if err != nil {
		t.Fatalf("FetchOne failed: %v", err)
	}

	// Verify progress was reported
	if atomic.LoadInt32(&progressCalls) == 0 {
		t.Error("progress callback was never called")
	}
	if seenFilename != "large-file.bin" {
		t.Errorf("expected filename 'large-file.bin', got %q", seenFilename)
	}
	if maxDownloaded != int64(len(testContent)) {
		t.Errorf("expected final downloaded %d, got %d", len(testContent), maxDownloaded)
	}
	if reportedTotal != int64(len(testContent)) {
		t.Errorf("expected total %d, got %d", len(testContent), reportedTotal)
	}
}

// TestIntegration_ResumePartialDownload tests resuming an interrupted download.
func TestIntegration_ResumePartialDownload(t *testing.T) {
	// Create content that can be split
	fullContent := []byte("PART1_CONTENT|PART2_CONTENT|PART3_CONTENT")
	splitPoint := 15 // After "PART1_CONTENT|"
	checksums := computeTestChecksumsFromBytes(fullContent)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "" {
			// Parse Range header: bytes=N-
			var start int64
			if _, err := fmt.Sscanf(rangeHeader, "bytes=%d-", &start); err == nil {
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write(fullContent[start:])
				return
			}
		}
		// Full content
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fullContent)))
		_, _ = w.Write(fullContent)
	}))
	defer server.Close()

	tmpDir := t.TempDir()

	// Create partial download file
	partialPath := filepath.Join(tmpDir, "resume-test.tar.gz.partial")
	if err := os.WriteFile(partialPath, fullContent[:splitPoint], 0644); err != nil {
		t.Fatalf("creating partial file: %v", err)
	}

	downloader := NewHTTPDownloader(Config{
		Mirrors:    []string{server.URL + "/"},
		MaxRetries: 1,
		Timeout:    10,
		Resume:     true,
	})

	distfile := NewDistfile("resume-test.tar.gz", int64(len(fullContent)), checksums)
	err := downloader.FetchOne(context.Background(), distfile, tmpDir)
	if err != nil {
		t.Fatalf("FetchOne failed: %v", err)
	}

	// Verify complete file
	destPath := filepath.Join(tmpDir, "resume-test.tar.gz")
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(content) != string(fullContent) {
		t.Errorf("content mismatch: expected %q, got %q", fullContent, content)
	}

	// Verify checksum of final file
	if err := Verify(destPath, checksums); err != nil {
		t.Errorf("checksum verification failed: %v", err)
	}
}

// TestIntegration_ContextCancellation tests that downloads respect context.
func TestIntegration_ContextCancellation(t *testing.T) {
	// Server that streams slowly
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		// Write one byte at a time with delay
		for i := 0; i < 1000; i++ {
			select {
			case <-r.Context().Done():
				return
			default:
				_, _ = w.Write([]byte{'X'})
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
	}))
	defer slowServer.Close()

	tmpDir := t.TempDir()

	downloader := NewHTTPDownloader(Config{
		Mirrors:    []string{slowServer.URL + "/"},
		MaxRetries: 1,
		Timeout:    60, // Long timeout, but context will cancel first
	})

	distfile := NewDistfile("slow-file.tar.gz", 1000, NewChecksums("abc", "", ""))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := downloader.FetchOne(ctx, distfile, tmpDir)
	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}

	// Should be context-related error
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		// Also accept wrapped errors
		if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "deadline") {
			t.Errorf("expected context-related error, got: %v", err)
		}
	}
}

// =============================================================================
// Real Network Tests (optional - skip if network unavailable)
// =============================================================================

// networkAvailable checks if network is available by trying to connect to a known host.
func networkAvailable() bool {
	conn, err := net.DialTimeout("tcp", "distfiles.gentoo.org:443", 5*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// TestIntegration_RealMirrorConnection tests connection to real Gentoo mirrors.
// This test is skipped if network is unavailable.
func TestIntegration_RealMirrorConnection(t *testing.T) {
	if !networkAvailable() {
		t.Skip("network unavailable, skipping real mirror test")
	}

	// Just test that we can make a HEAD request to a real mirror
	client := &http.Client{Timeout: 30 * time.Second}

	mirrors := DefaultMirrors
	var connected bool

	for _, mirror := range mirrors {
		url := mirror + "distfiles/"
		resp, err := client.Head(url)
		if err != nil {
			t.Logf("mirror %s not reachable: %v", mirror, err)
			continue
		}
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusMovedPermanently ||
			resp.StatusCode == http.StatusFound {
			t.Logf("successfully connected to mirror: %s (status: %d)", mirror, resp.StatusCode)
			connected = true
			break
		}
	}

	if !connected {
		t.Skip("no mirrors reachable, skipping")
	}
}

// TestIntegration_RealSmallFileDownload tests downloading a small real file.
// This test downloads a tiny file from Gentoo mirrors to verify the full workflow.
// Skipped if network is unavailable.
func TestIntegration_RealSmallFileDownload(t *testing.T) {
	if !networkAvailable() {
		t.Skip("network unavailable, skipping real download test")
	}

	// We'll test by downloading just the robots.txt or a similar small file
	// from the mirror. This is not a distfile but confirms network/HTTP works.
	client := &http.Client{Timeout: 30 * time.Second}

	// Try to get a small file from the first reachable mirror
	var body []byte
	for _, mirror := range DefaultMirrors {
		// Try to get robots.txt which most web servers have
		url := strings.TrimSuffix(mirror, "/") + "/robots.txt"
		resp, err := client.Get(url)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusOK {
			body, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if len(body) > 0 {
				t.Logf("successfully downloaded %d bytes from %s", len(body), url)
				break
			}
		}
		_ = resp.Body.Close()
	}

	if len(body) == 0 {
		t.Skip("could not download test file from any mirror")
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

// computeTestChecksumsFromBytes computes checksums for test content.
func computeTestChecksumsFromBytes(content []byte) Checksums {
	tmpFile, err := os.CreateTemp("", "checksum-test-*")
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.Write(content); err != nil {
		panic(err)
	}
	if err := tmpFile.Close(); err != nil {
		panic(err)
	}

	checksums, err := ComputeChecksums(tmpFile.Name())
	if err != nil {
		panic(err)
	}
	return checksums
}
