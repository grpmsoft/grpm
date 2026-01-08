package install

import (
	"fmt"
	"os"

	"github.com/grpmsoft/grpm/internal/state"
)

// Collision represents a file collision between packages.
type Collision struct {
	// Path is the conflicting file path
	Path string

	// ExistingOwner is the package that already owns this file
	ExistingOwner string

	// NewOwner is the package trying to install this file
	NewOwner string

	// Type of collision
	Type CollisionType
}

// CollisionType represents the type of file collision.
type CollisionType int

const (
	// CollisionFileExists - file exists but not owned by any package
	CollisionFileExists CollisionType = iota

	// CollisionOwnedByOther - file is owned by another package
	CollisionOwnedByOther

	// CollisionProtected - file is in protected directory
	CollisionProtected
)

// String returns string representation of collision type.
func (ct CollisionType) String() string {
	switch ct {
	case CollisionFileExists:
		return "file exists"
	case CollisionOwnedByOther:
		return "owned by other package"
	case CollisionProtected:
		return "protected path"
	default:
		return "unknown"
	}
}

// CollisionDetector detects file collisions during package installation.
type CollisionDetector struct {
	db *state.PackageDatabase

	// Protected paths that should never be overwritten
	protectedPaths []string
}

// NewCollisionDetector creates a new collision detector.
func NewCollisionDetector(db *state.PackageDatabase) *CollisionDetector {
	return &CollisionDetector{
		db: db,
		protectedPaths: []string{
			"/etc/passwd",
			"/etc/shadow",
			"/etc/group",
			"/etc/gshadow",
			"/etc/fstab",
			"/etc/mtab",
			"/boot/vmlinuz",
			"/boot/initramfs",
		},
	}
}

// Detect checks for file collisions.
//
// This method checks if any of the files to be installed:
//   - Already exist on the filesystem but are not tracked
//   - Are owned by another package
//   - Are in protected paths
//
// Parameters:
//   - filesToInstall: list of absolute file paths to check
//   - packageName: name of package trying to install these files
//
// Returns a list of collisions found (empty if none).
func (cd *CollisionDetector) Detect(filesToInstall []string, packageName string) ([]Collision, error) {
	collisions := make([]Collision, 0)

	for _, path := range filesToInstall {
		// Check if path is protected
		if cd.isProtected(path) {
			collisions = append(collisions, Collision{
				Path:     path,
				NewOwner: packageName,
				Type:     CollisionProtected,
			})
			continue
		}

		// Check if file exists
		if _, err := os.Stat(path); err == nil {
			// File exists - check who owns it
			owner, err := cd.db.WhoOwns(path)
			if err != nil {
				// File exists but not owned by any package
				collisions = append(collisions, Collision{
					Path:     path,
					NewOwner: packageName,
					Type:     CollisionFileExists,
				})
			} else if owner != packageName {
				// File is owned by another package
				collisions = append(collisions, Collision{
					Path:          path,
					ExistingOwner: owner,
					NewOwner:      packageName,
					Type:          CollisionOwnedByOther,
				})
			}
			// else: owned by same package (upgrade/replace) - no collision
		}
	}

	return collisions, nil
}

// isProtected checks if path is in protected list.
func (cd *CollisionDetector) isProtected(path string) bool {
	for _, protected := range cd.protectedPaths {
		if path == protected {
			return true
		}

		// Check if path is inside protected directory (only for directories)
		// Example: /etc/passwd/subfile would be protected if /etc was protected directory
		// But we only protect exact file matches from our list
	}

	return false
}

// AddProtectedPath adds a path to the protected list.
func (cd *CollisionDetector) AddProtectedPath(path string) {
	cd.protectedPaths = append(cd.protectedPaths, path)
}

// String returns a human-readable collision report.
func (c Collision) String() string {
	switch c.Type {
	case CollisionFileExists:
		return fmt.Sprintf("file exists: %s (not tracked by any package)", c.Path)
	case CollisionOwnedByOther:
		return fmt.Sprintf("file conflict: %s (owned by %s, wanted by %s)",
			c.Path, c.ExistingOwner, c.NewOwner)
	case CollisionProtected:
		return fmt.Sprintf("protected file: %s (cannot be overwritten)", c.Path)
	default:
		return fmt.Sprintf("collision: %s", c.Path)
	}
}
