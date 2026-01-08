package sync

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// GitSyncer implements repository synchronization using Git
type GitSyncer struct {
	verbose bool
}

// NewGitSyncer creates a new Git syncer
func NewGitSyncer() *GitSyncer {
	return &GitSyncer{}
}

// Name returns the syncer name
func (g *GitSyncer) Name() string {
	return "git"
}

// IsAvailable checks if git is installed and available
func (g *GitSyncer) IsAvailable() bool {
	cmd := exec.Command("git", "--version")
	err := cmd.Run()
	return err == nil
}

// Sync synchronizes the repository using Git
func (g *GitSyncer) Sync(ctx context.Context, config *SyncConfig) (*SyncResult, error) {
	g.verbose = config.Verbose
	start := time.Now()

	result := &SyncResult{
		Method: MethodGit,
	}

	// Default to official Gentoo git mirror if URL not specified
	gitURL := config.SourceURL
	if gitURL == "" {
		gitURL = "https://anongit.gentoo.org/git/repo/gentoo.git"
	}

	// Check if repository already exists
	gitDir := filepath.Join(config.RepoPath, ".git")
	isCloned := false
	if _, err := os.Stat(gitDir); err == nil {
		isCloned = true
	}

	if isCloned {
		// Repository exists - do incremental pull
		if g.verbose {
			log.Printf("🔄 Updating existing Git repository...")
			log.Printf("   Path: %s", config.RepoPath)
		}

		if err := g.pull(ctx, config.RepoPath); err != nil {
			return nil, fmt.Errorf("git pull failed: %w", err)
		}
	} else {
		// New clone needed
		if g.verbose {
			log.Printf("🔄 Cloning Gentoo repository...")
			log.Printf("   Source: %s", gitURL)
			log.Printf("   Target: %s", config.RepoPath)
		}

		if err := g.clone(ctx, gitURL, config.RepoPath); err != nil {
			return nil, fmt.Errorf("git clone failed: %w", err)
		}
	}

	result.Duration = time.Since(start).String()

	// Verify GPG signature
	if config.VerifyGPG {
		if g.verbose {
			log.Printf("🔐 Verifying GPG signature...")
		}

		if err := g.VerifyGPG(config.RepoPath); err != nil {
			return nil, fmt.Errorf("GPG verification failed: %w", err)
		}

		result.GPGVerified = true
		if g.verbose {
			log.Println("✅ GPG signature verified")
		}
	}

	log.Printf("✅ Git sync completed in %s", result.Duration)
	return result, nil
}

// clone performs git clone
func (g *GitSyncer) clone(ctx context.Context, url, repoPath string) error {
	// Use --depth=1 for shallow clone to save bandwidth/time
	// Use --single-branch to clone only main branch
	cmd := exec.CommandContext(ctx,
		"git", "clone",
		"--depth=1",
		"--single-branch",
		url,
		repoPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// pull performs git pull to update existing repository
func (g *GitSyncer) pull(ctx context.Context, repoPath string) error {
	// Change to repository directory
	cmd := exec.CommandContext(ctx,
		"git", "-C", repoPath,
		"pull", "--ff-only",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// VerifyGPG verifies the GPG signature of the latest commit
func (g *GitSyncer) VerifyGPG(repoPath string) error {
	// Verify latest commit signature
	cmd := exec.Command("git", "-C", repoPath, "verify-commit", "HEAD")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("commit signature verification failed: %w\n"+
			"Ensure Gentoo GPG keys are imported:\n"+
			"  gpg --import /usr/share/openpgp-keys/gentoo-release.asc\n"+
			"Or download from: https://www.gentoo.org/downloads/signatures/", err)
	}

	return nil
}
