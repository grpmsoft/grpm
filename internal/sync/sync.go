package sync

import (
	"context"
)

// SyncMethod represents the type of sync method
type SyncMethod string

const (
	MethodRsync SyncMethod = "rsync"
	MethodGit   SyncMethod = "git"
	MethodAuto  SyncMethod = "auto"
)

// SyncConfig holds configuration for repository synchronization
type SyncConfig struct {
	Method    SyncMethod // rsync, git, or auto
	RepoPath  string     // Local repository path
	SourceURL string     // Source repository URL
	VerifyGPG bool       // Enable GPG signature verification
	Verbose   bool       // Enable verbose logging
	PreferGit bool       // Prefer git over rsync when auto-detecting
}

// SyncResult contains synchronization results
type SyncResult struct {
	Method           SyncMethod // Method used for sync
	FilesChanged     int        // Number of files changed
	BytesTransferred int64      // Total bytes transferred
	Duration         string     // Duration of sync operation
	GPGVerified      bool       // Whether GPG signature was verified
}

// Syncer defines the interface for repository synchronization
type Syncer interface {
	// Sync synchronizes the repository
	Sync(ctx context.Context, config *SyncConfig) (*SyncResult, error)

	// VerifyGPG verifies GPG signature of the repository
	VerifyGPG(repoPath string) error

	// Name returns the name of the syncer implementation
	Name() string

	// IsAvailable checks if this syncer can be used on the system
	IsAvailable() bool
}

// NewSyncer creates a new syncer based on the specified method
func NewSyncer(method SyncMethod) (Syncer, error) {
	switch method {
	case MethodRsync:
		return NewRsyncSyncer(), nil
	case MethodGit:
		return NewGitSyncer(), nil
	case MethodAuto:
		return NewAutoSyncer(), nil
	default:
		return NewAutoSyncer(), nil
	}
}
