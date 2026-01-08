package state

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

type SnapshotManager struct {
	snapshotDir string
	fsType      string
}

func NewSnapshotManager(snapshotDir, fsType string) *SnapshotManager {
	return &SnapshotManager{
		snapshotDir: snapshotDir,
		fsType:      fsType,
	}
}

func (sm *SnapshotManager) CreateSnapshot(targetPath string) (string, error) {
	snapshotID := fmt.Sprintf("snapshot-%d", time.Now().UnixNano())
	snapshotPath := filepath.Join(sm.snapshotDir, snapshotID)

	var cmd *exec.Cmd
	switch sm.fsType {
	case "btrfs":
		cmd = exec.Command("btrfs", "subvolume", "snapshot", targetPath, snapshotPath)
	case "zfs":
		dataset := sm.findDataset(targetPath)
		if dataset == "" {
			return "", fmt.Errorf("ZFS dataset not found for %s", targetPath)
		}
		cmd = exec.Command("zfs", "snapshot", fmt.Sprintf("%s@%s", dataset, snapshotID))
	default:
		return "", fmt.Errorf("unsupported filesystem type: %s", sm.fsType)
	}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create snapshot: %w", err)
	}

	return snapshotID, nil
}

func (sm *SnapshotManager) RollbackSnapshot(snapshotID string) error {
	var cmd *exec.Cmd
	switch sm.fsType {
	case "btrfs":
		snapshotPath := filepath.Join(sm.snapshotDir, snapshotID)
		cmd = exec.Command("btrfs", "subvolume", "set-default", snapshotPath)
	case "zfs":
		cmd = exec.Command("zfs", "rollback", snapshotID)
	default:
		return fmt.Errorf("unsupported filesystem type: %s", sm.fsType)
	}

	return cmd.Run()
}

func (sm *SnapshotManager) findDataset(path string) string {
	// In a real implementation, we need to find the ZFS dataset for the path
	return "tank" + path
}
