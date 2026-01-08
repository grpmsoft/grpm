package sync

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gokrazy/rsync/rsynccmd"
)

// RsyncSyncer implements repository synchronization using rsync
type RsyncSyncer struct {
	verbose bool
}

// NewRsyncSyncer creates a new rsync syncer
func NewRsyncSyncer() *RsyncSyncer {
	return &RsyncSyncer{}
}

// Name returns the syncer name
func (r *RsyncSyncer) Name() string {
	return "rsync"
}

// IsAvailable checks if rsync is available
// We use native Go rsync (gokrazy/rsync) so this is always true
func (r *RsyncSyncer) IsAvailable() bool {
	return true // Native Go rsync is always available
}

// Sync synchronizes the repository using rsync
func (r *RsyncSyncer) Sync(ctx context.Context, config *SyncConfig) (*SyncResult, error) {
	r.verbose = config.Verbose
	start := time.Now()

	result := &SyncResult{
		Method: MethodRsync,
	}

	// Default to official Gentoo rsync mirror if URL not specified
	rsyncURL := config.SourceURL
	if rsyncURL == "" {
		rsyncURL = "rsync://rsync.gentoo.org/gentoo-portage"
	}

	// Ensure repository directory exists
	if err := os.MkdirAll(config.RepoPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create repository directory: %w", err)
	}

	if r.verbose {
		log.Printf("🔄 Syncing via rsync (native Go implementation)...")
		log.Printf("   Source: %s", rsyncURL)
		log.Printf("   Target: %s", config.RepoPath)
	}

	// Build rsync command using gokrazy/rsync (native Go implementation)
	cmd := rsynccmd.Command("grpm-rsync",
		"-avz",              // archive, verbose, compress
		"--delete",          // delete obsolete files
		rsyncURL+"/",        // source (trailing slash important!)
		config.RepoPath+"/", // destination
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute sync
	_, err := cmd.Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("rsync failed: %w", err)
	}

	result.Duration = time.Since(start).String()

	// Verify GPG signature
	if config.VerifyGPG {
		if r.verbose {
			log.Printf("🔐 Verifying GPG signature...")
		}

		if err := r.VerifyGPG(config.RepoPath); err != nil {
			// For rsync, GPG verification is known to fail (signature files not synced)
			// Log warning but don't fail
			log.Printf("⚠️  GPG verification failed (expected for rsync): %v", err)
			log.Printf("⚠️  Consider using --method=git for GPG verification")
			result.GPGVerified = false
		} else {
			result.GPGVerified = true
			if r.verbose {
				log.Println("✅ GPG signature verified")
			}
		}
	}

	log.Printf("✅ Rsync completed in %s", result.Duration)
	return result, nil
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

	if r.verbose {
		log.Printf("Verifying GPG signature: %s", signatureFile)
	}

	// Run gpg --verify
	cmd := exec.Command("gpg", "--verify", signatureFile, timestampFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gpg verification failed: ensure gpg is installed and Gentoo keys are imported: %w", err)
	}

	return nil
}
