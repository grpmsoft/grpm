package sync

import (
	"context"
	"log"
)

// AutoSyncer automatically selects the best available sync method
type AutoSyncer struct {
	gitSyncer   *GitSyncer
	rsyncSyncer *RsyncSyncer
	verbose     bool
}

// NewAutoSyncer creates a new auto syncer
func NewAutoSyncer() *AutoSyncer {
	return &AutoSyncer{
		gitSyncer:   NewGitSyncer(),
		rsyncSyncer: NewRsyncSyncer(),
	}
}

// Name returns the syncer name
func (a *AutoSyncer) Name() string {
	return "auto"
}

// IsAvailable always returns true (falls back to rsync if git unavailable)
func (a *AutoSyncer) IsAvailable() bool {
	return true
}

// Sync automatically selects and uses the best available sync method
func (a *AutoSyncer) Sync(ctx context.Context, config *SyncConfig) (*SyncResult, error) {
	a.verbose = config.Verbose

	// Strategy:
	// 1. If GPG verification required → prefer Git (rsync typically fails GPG)
	// 2. If Git available → prefer Git (better security)
	// 3. Fallback to rsync (faster, always available)

	var selectedSyncer Syncer
	var reason string

	if config.VerifyGPG && a.gitSyncer.IsAvailable() {
		// GPG required and Git available → use Git
		selectedSyncer = a.gitSyncer
		reason = "GPG verification required"
	} else if config.PreferGit && a.gitSyncer.IsAvailable() {
		// Git preferred and available → use Git
		selectedSyncer = a.gitSyncer
		reason = "Git preferred"
	} else if a.gitSyncer.IsAvailable() {
		// Git available → use Git for better security
		selectedSyncer = a.gitSyncer
		reason = "Git available (better security)"
	} else {
		// Fallback to rsync
		selectedSyncer = a.rsyncSyncer
		reason = "Git not available, using rsync fallback"
	}

	if a.verbose {
		log.Printf("🔍 Auto-selected sync method: %s (%s)", selectedSyncer.Name(), reason)
	}

	return selectedSyncer.Sync(ctx, config)
}

// VerifyGPG delegates to the selected syncer
func (a *AutoSyncer) VerifyGPG(repoPath string) error {
	// Try Git first (more reliable for GPG)
	if a.gitSyncer.IsAvailable() {
		return a.gitSyncer.VerifyGPG(repoPath)
	}

	// Fallback to rsync (typically fails)
	return a.rsyncSyncer.VerifyGPG(repoPath)
}
