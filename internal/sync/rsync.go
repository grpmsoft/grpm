package sync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/rsync"
)

// GentooMirrors is a list of official Gentoo rsync mirrors.
// Ordered by reliability and geographic distribution.
var GentooMirrors = []string{
	"rsync://rsync.gentoo.org/gentoo-portage",        // Round-robin (primary)
	"rsync://rsync.de.gentoo.org/gentoo-portage",     // Germany
	"rsync://rsync.us.gentoo.org/gentoo-portage",     // USA
	"rsync://rsync.jp.gentoo.org/gentoo-portage",     // Japan
	"rsync://rsync.uk.gentoo.org/gentoo-portage",     // UK
	"rsync://rsync.fr.gentoo.org/gentoo-portage",     // France
	"rsync://rsync.nl.gentoo.org/gentoo-portage",     // Netherlands
	"rsync://rsync1.us.gentoo.org/gentoo-portage",    // USA mirror 1
	"rsync://rsync.europe.gentoo.org/gentoo-portage", // Europe
}

// SyncStrategy defines retry behavior
type SyncStrategy struct {
	MaxRetries        int           // Max retries per mirror
	RetryDelay        time.Duration // Delay between retries
	MaxMirrors        int           // Max mirrors to try (0 = all)
	ConnectionTimeout time.Duration // Connection timeout per attempt
}

// DefaultStrategy returns sensible defaults for sync retries
func DefaultStrategy() SyncStrategy {
	return SyncStrategy{
		MaxRetries:        3,
		RetryDelay:        2 * time.Second,
		MaxMirrors:        5,
		ConnectionTimeout: 30 * time.Second,
	}
}

// RsyncSyncer implements repository synchronization using rsync
type RsyncSyncer struct {
	verbose  bool
	strategy SyncStrategy
	log      *logging.Logger
}

// NewRsyncSyncer creates a new rsync syncer
func NewRsyncSyncer() *RsyncSyncer {
	return &RsyncSyncer{
		strategy: DefaultStrategy(),
		log:      logging.New(),
	}
}

// WithStrategy sets custom retry strategy
func (r *RsyncSyncer) WithStrategy(s SyncStrategy) *RsyncSyncer {
	r.strategy = s
	return r
}

// Name returns the syncer name
func (r *RsyncSyncer) Name() string {
	return "rsync"
}

// IsAvailable checks if rsync is available
// We use native Go rsync implementation so this is always true
func (r *RsyncSyncer) IsAvailable() bool {
	return true // Native Go rsync is always available
}

// rsyncLogger adapts grpm logging to rsync.Logger interface
type rsyncLogger struct {
	log     *logging.Logger
	verbose bool
}

func (l rsyncLogger) Printf(format string, v ...interface{}) {
	if l.verbose {
		l.log.Debug("[rsync] "+format, v...)
	}
}

// Sync synchronizes the repository using rsync with smart retry and mirror fallback.
func (r *RsyncSyncer) Sync(ctx context.Context, config *SyncConfig) (*SyncResult, error) {
	r.verbose = config.Verbose
	start := time.Now()

	result := &SyncResult{
		Method: MethodRsync,
	}

	// Ensure repository directory exists
	if err := os.MkdirAll(config.RepoPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create repository directory: %w", err)
	}

	// Set logging level based on verbosity
	if r.verbose {
		r.log.SetLevel(logging.LevelVerbose)
	}

	// Build mirror list
	mirrors := r.buildMirrorList(config.SourceURL)

	r.log.Syncing("Gentoo Portage")
	if r.verbose {
		r.log.Verbose("Target: %s", config.RepoPath)
		r.log.Verbose("Strategy: %d retries per mirror, %d mirrors max",
			r.strategy.MaxRetries, len(mirrors))
	}

	// Try mirrors with smart retry
	var lastErr error
	for mirrorIdx, mirrorURL := range mirrors {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		r.log.Mirror(mirrorIdx+1, len(mirrors), r.extractHost(mirrorURL))

		// Try this mirror with retries
		err := r.syncWithRetry(ctx, mirrorURL, config.RepoPath)
		if err == nil {
			// Success!
			result.Duration = time.Since(start).String()
			result.MirrorUsed = mirrorURL

			// Verify GPG signature
			r.verifyGPGIfRequested(config, result)

			r.log.SyncComplete(time.Since(start), result.FilesChanged)
			r.log.Info("Mirror used: %s", r.extractHost(mirrorURL))
			return result, nil
		}

		lastErr = err
		r.log.MirrorFailed(r.extractHost(mirrorURL), err)

		// Check if it's a permanent error (not worth retrying other mirrors)
		if r.isPermanentError(err) {
			r.log.Warn("Permanent error detected, not trying other mirrors")
			break
		}
	}

	return nil, fmt.Errorf("all mirrors failed, last error: %w", lastErr)
}

// buildMirrorList creates an ordered list of mirrors to try.
func (r *RsyncSyncer) buildMirrorList(customURL string) []string {
	var mirrors []string

	// If custom URL provided, try it first
	if customURL != "" && customURL != "rsync://rsync.gentoo.org/gentoo-portage" {
		mirrors = append(mirrors, customURL)
	}

	// Add standard mirrors
	mirrors = append(mirrors, GentooMirrors...)

	// Limit to MaxMirrors
	if r.strategy.MaxMirrors > 0 && len(mirrors) > r.strategy.MaxMirrors {
		mirrors = mirrors[:r.strategy.MaxMirrors]
	}

	return mirrors
}

// syncWithRetry attempts to sync from a single mirror with retries.
func (r *RsyncSyncer) syncWithRetry(ctx context.Context, mirrorURL, destDir string) error {
	var lastErr error

	for attempt := 1; attempt <= r.strategy.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if attempt > 1 {
			r.log.Retry(attempt, r.strategy.MaxRetries, r.strategy.RetryDelay)
			time.Sleep(r.strategy.RetryDelay)
		}

		err := r.doSync(ctx, mirrorURL, destDir)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !r.isRetryableError(err) {
			r.log.Warn("Non-retryable error: %v", err)
			break
		}

		r.log.Verbose("Attempt %d failed: %v", attempt, err)
	}

	return lastErr
}

// doSync performs the actual rsync operation.
func (r *RsyncSyncer) doSync(ctx context.Context, mirrorURL, destDir string) error {
	client := rsync.NewClient()
	client.Compress = false // Many mirrors don't support compression
	client.Delete = true
	client.Timeout = r.strategy.ConnectionTimeout
	client.Logger = rsyncLogger{log: r.log, verbose: r.verbose}

	return client.Sync(ctx, mirrorURL, destDir)
}

// isRetryableError checks if an error is worth retrying.
func (r *RsyncSyncer) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Connection errors are retryable (all lowercase for comparison)
	retryablePatterns := []string{
		"connection reset",
		"connection refused",
		"connection timed out",
		"timeout",
		"eof",
		"broken pipe",
		"forcibly closed",
		"i/o timeout",
		"temporary failure",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}

	return false
}

// isPermanentError checks if an error means we should stop trying other mirrors.
func (r *RsyncSyncer) isPermanentError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// These errors affect all mirrors
	permanentPatterns := []string{
		"permission denied",
		"disk full",
		"no space left",
		"read-only file system",
	}

	for _, pattern := range permanentPatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}

	return false
}

// extractHost extracts hostname from rsync URL for logging.
func (r *RsyncSyncer) extractHost(url string) string {
	// rsync://hostname/module -> hostname
	url = strings.TrimPrefix(url, "rsync://")
	if idx := strings.Index(url, "/"); idx > 0 {
		return url[:idx]
	}
	return url
}

// verifyGPGIfRequested handles GPG verification with appropriate warnings.
func (r *RsyncSyncer) verifyGPGIfRequested(config *SyncConfig, result *SyncResult) {
	if !config.VerifyGPG {
		return
	}

	r.log.Info("Verifying GPG signature...")

	if err := r.VerifyGPG(config.RepoPath); err != nil {
		r.log.Warn("GPG verification failed (expected for rsync): %v", err)
		r.log.Warn("Consider using --method=git for GPG verification")
		result.GPGVerified = false
	} else {
		result.GPGVerified = true
		r.log.Success("GPG signature verified")
	}
}

// VerifyGPG verifies GPG signature of the repository
// Note: This typically fails for rsync because signature files are not synced
func (r *RsyncSyncer) VerifyGPG(repoPath string) error {
	// Check for timestamp files (standard Gentoo verification)
	timestampFile := filepath.Join(repoPath, "metadata", "timestamp.chk")
	signatureFile := timestampFile + ".asc"

	// Verify signature file exists
	if _, err := os.Stat(signatureFile); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("signature file not found: %s (rsync mirrors typically don't sync GPG signatures)", signatureFile)
		}
		return fmt.Errorf("failed to access signature file: %w", err)
	}

	// Verify timestamp file exists
	if _, err := os.Stat(timestampFile); err != nil {
		return fmt.Errorf("timestamp file not found: %s", timestampFile)
	}

	r.log.Verbose("Verifying GPG signature: %s", signatureFile)

	// Run gpg --verify
	cmd := exec.Command("gpg", "--verify", signatureFile, timestampFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gpg verification failed: ensure gpg is installed and Gentoo keys are imported: %w", err)
	}

	return nil
}
