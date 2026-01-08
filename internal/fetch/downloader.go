package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrPartialContent indicates the server does not support resume.
var ErrPartialContent = errors.New("server does not support partial content")

// ProgressCallback is called during download with progress updates.
//
// Parameters:
//   - filename: the file being downloaded
//   - downloaded: bytes downloaded so far
//   - total: total file size (-1 if unknown)
type ProgressCallback func(filename string, downloaded, total int64)

// HTTPDownloader implements the Fetcher interface using HTTP.
//
// HTTPDownloader supports:
//   - Download resume via Range headers
//   - Mirror failover
//   - Checksum verification
//   - Progress reporting
//   - Configurable timeouts and retries
//
// HTTPDownloader is thread-safe for concurrent downloads.
type HTTPDownloader struct {
	client   *http.Client
	mirrors  *MirrorSelector
	config   Config
	progress ProgressCallback
	mu       sync.Mutex
}

// NewHTTPDownloader creates a new HTTPDownloader with the given configuration.
//
// If config.Mirrors is empty, DefaultMirrors will be used.
func NewHTTPDownloader(config Config) *HTTPDownloader {
	// Apply defaults
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.Timeout <= 0 {
		config.Timeout = 300
	}
	if config.Parallel <= 0 {
		config.Parallel = 1
	}

	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	return &HTTPDownloader{
		client:  client,
		mirrors: NewMirrorSelector(config.Mirrors),
		config:  config,
	}
}

// SetProgressCallback sets the progress callback function.
//
// The callback is called periodically during downloads with progress updates.
func (d *HTTPDownloader) SetProgressCallback(cb ProgressCallback) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.progress = cb
}

// Fetch downloads multiple distfiles to the destination directory.
//
// Downloads are performed sequentially when config.Parallel is 1,
// or in parallel otherwise. Checksum verification is performed
// after each successful download.
//
// Returns error if any distfile fails to download or verify.
func (d *HTTPDownloader) Fetch(ctx context.Context, distfiles []Distfile, destDir string) error {
	if len(distfiles) == 0 {
		return nil
	}

	if d.mirrors.Len() == 0 {
		return ErrNoMirrors
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	// Sequential download for now (parallel will be added later)
	for _, distfile := range distfiles {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := d.FetchOne(ctx, distfile, destDir); err != nil {
			return err
		}
	}

	return nil
}

// FetchOne downloads a single distfile to the destination directory.
//
// The download process:
//  1. Check if file already exists with correct checksum (skip if so)
//  2. Try each mirror in order of reliability
//  3. Resume partial downloads if supported
//  4. Verify checksum after download
//  5. Rename temporary file to final name on success
//
// Returns error if download fails from all mirrors or checksum verification fails.
func (d *HTTPDownloader) FetchOne(ctx context.Context, distfile Distfile, destDir string) error {
	if !distfile.IsValid() {
		return fmt.Errorf("invalid distfile: %s", distfile.Filename)
	}

	destPath := filepath.Join(destDir, distfile.Filename)

	// Check if file already exists with correct checksum
	if fileExists(destPath) {
		if err := Verify(destPath, distfile.Checksums); err == nil {
			log.Printf("  %s: already downloaded and verified", distfile.Filename)
			return nil
		}
		// File exists but checksum mismatch - will be re-downloaded
		log.Printf("  %s: checksum mismatch, re-downloading", distfile.Filename)
	}

	// Build list of URIs to try
	uris := d.buildURIList(distfile)
	if len(uris) == 0 {
		return fmt.Errorf("%w: no URIs available for %s", ErrDownloadFailed, distfile.Filename)
	}

	var lastErr error

	for _, uri := range uris {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		log.Printf("  %s: trying %s", distfile.Filename, uri)

		err := d.downloadWithRetry(ctx, uri, destPath, distfile)
		if err == nil {
			// Success - verify checksum
			if verifyErr := Verify(destPath, distfile.Checksums); verifyErr != nil {
				// Checksum failed - remove corrupt file and try next mirror
				_ = os.Remove(destPath)
				lastErr = verifyErr
				d.mirrors.ReportFailure(ExtractMirrorBase(uri))
				log.Printf("  %s: checksum verification failed: %v", distfile.Filename, verifyErr)
				continue
			}

			d.mirrors.ReportSuccess(ExtractMirrorBase(uri))
			log.Printf("  %s: downloaded and verified", distfile.Filename)
			return nil
		}

		lastErr = err
		d.mirrors.ReportFailure(ExtractMirrorBase(uri))
		log.Printf("  %s: download failed: %v", distfile.Filename, err)
	}

	return fmt.Errorf("%w: %s: %w", ErrDownloadFailed, distfile.Filename, lastErr)
}

// buildURIList builds the list of URIs to try for a distfile.
//
// Priority:
//  1. Explicit URIs from SRC_URI (if any)
//  2. Mirror URIs in order of reliability
func (d *HTTPDownloader) buildURIList(distfile Distfile) []string {
	var uris []string

	// First, add explicit URIs from the distfile
	uris = append(uris, distfile.URIs...)

	// Then add mirror URIs
	mirrorURIs := d.mirrors.GetURIs(distfile.Filename)
	uris = append(uris, mirrorURIs...)

	return uris
}

// downloadWithRetry attempts to download a file with retries.
func (d *HTTPDownloader) downloadWithRetry(ctx context.Context, uri, destPath string, distfile Distfile) error {
	var lastErr error

	for attempt := 0; attempt < d.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry with exponential backoff
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		err := d.download(ctx, uri, destPath, distfile)
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry on context cancellation
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
	}

	return lastErr
}

// download performs the actual HTTP download.
func (d *HTTPDownloader) download(ctx context.Context, uri, destPath string, distfile Distfile) error {
	partialPath := destPath + ".partial"

	existingSize := d.getExistingSize(partialPath)

	resp, existingSize, err := d.executeRequest(ctx, uri, existingSize)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	totalSize := d.calculateTotalSize(resp, existingSize, distfile.Size)

	downloaded, err := d.writeResponseToFile(ctx, resp, partialPath, existingSize, totalSize, distfile.Filename)
	if err != nil {
		return err
	}

	// Verify size if known
	if distfile.Size > 0 && downloaded != distfile.Size {
		return fmt.Errorf("size mismatch: expected %d, got %d", distfile.Size, downloaded)
	}

	// Rename partial file to final destination
	if err := os.Rename(partialPath, destPath); err != nil {
		return fmt.Errorf("renaming file: %w", err)
	}

	return nil
}

// getExistingSize returns the size of an existing partial download file.
func (d *HTTPDownloader) getExistingSize(partialPath string) int64 {
	if !d.config.Resume {
		return 0
	}
	if info, err := os.Stat(partialPath); err == nil {
		return info.Size()
	}
	return 0
}

// executeRequest creates and executes the HTTP request.
// Returns the response, adjusted existingSize, and any error.
func (d *HTTPDownloader) executeRequest(ctx context.Context, uri string, existingSize int64) (*http.Response, int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}

	// Add Range header for resume
	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}

	// Add User-Agent
	req.Header.Set("User-Agent", "GRPM/0.1.0 (Gentoo Package Manager)")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("HTTP request: %w", err)
	}

	// Handle response status and adjust existingSize
	existingSize, err = handleResponseStatus(resp, existingSize)
	if err != nil {
		_ = resp.Body.Close()
		return nil, 0, err
	}

	return resp, existingSize, nil
}

// handleResponseStatus processes HTTP status codes and returns adjusted existingSize.
func handleResponseStatus(resp *http.Response, existingSize int64) (int64, error) {
	switch resp.StatusCode {
	case http.StatusOK:
		// Full download - reset partial file
		return 0, nil
	case http.StatusPartialContent:
		// Resume supported - continue from existingSize
		return existingSize, nil
	case http.StatusRequestedRangeNotSatisfiable:
		// Resume not possible - start fresh
		return 0, nil
	case http.StatusNotFound:
		return 0, ErrFileNotFound
	default:
		return 0, fmt.Errorf("HTTP status %d: %s", resp.StatusCode, resp.Status)
	}
}

// calculateTotalSize determines the total expected file size.
func (d *HTTPDownloader) calculateTotalSize(resp *http.Response, existingSize, distfileSize int64) int64 {
	if resp.ContentLength > 0 {
		return existingSize + resp.ContentLength
	}
	if distfileSize > 0 {
		return distfileSize
	}
	return -1
}

// writeResponseToFile writes the response body to a file with progress reporting.
func (d *HTTPDownloader) writeResponseToFile(
	ctx context.Context,
	resp *http.Response,
	partialPath string,
	existingSize, totalSize int64,
	filename string,
) (int64, error) {
	file, err := d.openOutputFile(partialPath, existingSize, resp.StatusCode)
	if err != nil {
		return 0, err
	}

	downloaded, err := d.copyWithProgress(ctx, file, resp.Body, existingSize, totalSize, filename)
	if err != nil {
		_ = file.Close()
		return 0, err
	}

	if err := file.Close(); err != nil {
		return 0, fmt.Errorf("closing file: %w", err)
	}

	return downloaded, nil
}

// openOutputFile opens the output file for writing.
func (d *HTTPDownloader) openOutputFile(partialPath string, existingSize int64, statusCode int) (*os.File, error) {
	if existingSize > 0 && statusCode == http.StatusPartialContent {
		// Append to existing partial file
		file, err := os.OpenFile(partialPath, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("opening output file: %w", err)
		}
		return file, nil
	}
	// Create new file
	file, err := os.Create(partialPath)
	if err != nil {
		return nil, fmt.Errorf("creating output file: %w", err)
	}
	return file, nil
}

// copyWithProgress copies data from reader to file with progress reporting.
func (d *HTTPDownloader) copyWithProgress(
	ctx context.Context,
	file *os.File,
	body io.Reader,
	existingSize, totalSize int64,
	filename string,
) (int64, error) {
	downloaded := existingSize
	buf := make([]byte, 32*1024) // 32KB buffer

	d.mu.Lock()
	progressCb := d.progress
	d.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		n, readErr := body.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				return 0, fmt.Errorf("writing to file: %w", writeErr)
			}
			downloaded += int64(n)

			if progressCb != nil {
				progressCb(filename, downloaded, totalSize)
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return 0, fmt.Errorf("reading response: %w", readErr)
		}
	}

	return downloaded, nil
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
