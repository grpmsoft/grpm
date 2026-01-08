package commands

import (
	"log"
	"path/filepath"

	"github.com/grpmsoft/grpm/internal/repo"
	"github.com/grpmsoft/grpm/internal/solver"
	"github.com/grpmsoft/grpm/internal/state"
	"github.com/spf13/cobra"
)

var (
	snapshotDir string
	fsType      string
)

var installCmd = &cobra.Command{
	Use:   "install [package...]",
	Short: "Install packages with transaction safety",
	Long: `Install packages with full transaction safety using filesystem snapshots.

The installation process:
  1. Create filesystem snapshot (rollback point)
  2. Resolve dependencies
  3. Download sources
  4. Build packages
  5. Merge files to system
  6. Update package database

If any step fails, the system can be rolled back to the snapshot.

Example:
  grpm install sys-libs/zlib
  grpm install --repo=/usr/portage app-editors/vim`,
	Args: cobra.MinimumNArgs(1),
	Run:  runInstall,
}

func init() {
	installCmd.Flags().StringVar(&repoPath, "repo", "/var/db/repos/gentoo", "Path to Portage repository")
	installCmd.Flags().StringVar(&snapshotDir, "snapshot-dir", "/.snapshots", "Snapshot directory")
	installCmd.Flags().StringVar(&fsType, "fs-type", "btrfs", "Filesystem type (btrfs or zfs)")
	installCmd.Flags().BoolVar(&useMockRepo, "mock", false, "Use mock repository for testing")
}

func runInstall(cmd *cobra.Command, args []string) {
	// Convert path to absolute
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		log.Fatalf("Invalid repository path: %v", err)
	}

	// Initialize snapshot manager
	sm := state.NewSnapshotManager(snapshotDir, fsType)

	// Create snapshot before changes
	snapshotID, err := sm.CreateSnapshot("/")
	if err != nil {
		log.Fatalf("Failed to create snapshot: %v", err)
	}
	log.Printf("Created system snapshot: %s", snapshotID)

	// Resolve dependencies
	var r repo.Repository
	if !useMockRepo {
		r, err = repo.NewPortageRepository(absRepoPath)
		if err != nil {
			log.Fatalf("Repository error: %v", err)
		}
	} else {
		r = repo.NewMockRepository()
	}

	resolver := solver.NewResolver(r)
	solution, err := resolver.Resolve(args)
	if err != nil {
		log.Fatalf("Dependency resolution failed: %v", err)
	}

	// Installation process (stub - will be implemented in Phase 4)
	log.Println("Installing packages:")
	for name, pkg := range solution {
		log.Printf("- %s-%s (slot: %s)", name, pkg.Version, pkg.Slot)
		// TODO Phase 4: Actual installation
		// - Download sources
		// - Execute ebuild phases (setup, unpack, compile, install)
		// - Merge files using internal/install
		// - Update package database
	}

	// If installation succeeded
	log.Println("Installation completed successfully")

	// TODO: Cleanup old snapshots, update configuration, run hooks
}
