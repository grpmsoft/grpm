package daemon

import (
	"fmt"
	"strings"
	"sync"

	"github.com/grpmsoft/grpm/internal/application"
)

// ConflictType defines the type of package conflict
type ConflictType int

const (
	ConflictNone        ConflictType = iota
	ConflictSamePackage              // Same package being installed/removed
	ConflictDependency               // Shared dependency conflict
	ConflictSlot                     // Slot conflict
	ConflictBlocker                  // Package blocker conflict
)

func (c ConflictType) String() string {
	switch c {
	case ConflictNone:
		return "none"
	case ConflictSamePackage:
		return "same_package"
	case ConflictDependency:
		return "dependency"
	case ConflictSlot:
		return "slot"
	case ConflictBlocker:
		return "blocker"
	default:
		return "unknown"
	}
}

// PackageConflict represents a conflict between jobs
type PackageConflict struct {
	Type           ConflictType
	Job1           *Job
	Job2           *Job
	ConflictingPkg string
	Reason         string
}

// ConflictDetector detects conflicts between package operations
type ConflictDetector struct {
	packageService *application.PackageService
	mu             sync.RWMutex
}

// NewConflictDetector creates a new conflict detector
func NewConflictDetector(pkgService *application.PackageService) *ConflictDetector {
	return &ConflictDetector{
		packageService: pkgService,
	}
}

// DetectConflicts checks if a new job conflicts with existing jobs
func (cd *ConflictDetector) DetectConflicts(newJob *Job, existingJobs []*Job) (*PackageConflict, error) {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	// Only check against running or pending jobs
	activeJobs := filterActiveJobs(existingJobs)
	if len(activeJobs) == 0 {
		return nil, nil
	}

	// Get affected packages for new job (including dependencies)
	newAffected, err := cd.getAffectedPackages(newJob)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve new job dependencies: %w", err)
	}

	// Check each active job for conflicts
	for _, existingJob := range activeJobs {
		conflict := cd.checkJobConflict(newJob, existingJob, newAffected)
		if conflict != nil {
			return conflict, nil
		}
	}

	return nil, nil
}

// checkJobConflict checks if two jobs conflict
func (cd *ConflictDetector) checkJobConflict(newJob, existingJob *Job, newAffected []string) *PackageConflict {
	// 1. Check same package conflict
	if isSamePackage(newJob.PackageName, existingJob.PackageName) {
		return &PackageConflict{
			Type:           ConflictSamePackage,
			Job1:           newJob,
			Job2:           existingJob,
			ConflictingPkg: newJob.PackageName,
			Reason: fmt.Sprintf("Cannot perform %s on %s while %s is in progress",
				newJob.Type, newJob.PackageName, existingJob.Type),
		}
	}

	// 2. Check dependency conflicts
	existingAffected, err := cd.getAffectedPackages(existingJob)
	if err != nil {
		// If we can't resolve existing job dependencies, be conservative
		// and assume potential conflict
		return &PackageConflict{
			Type:           ConflictDependency,
			Job1:           newJob,
			Job2:           existingJob,
			ConflictingPkg: existingJob.PackageName,
			Reason:         "Cannot determine dependency safety - assuming conflict",
		}
	}

	// Check for shared packages in dependency trees
	for _, newPkg := range newAffected {
		for _, existingPkg := range existingAffected {
			if isSamePackage(newPkg, existingPkg) {
				return &PackageConflict{
					Type:           ConflictDependency,
					Job1:           newJob,
					Job2:           existingJob,
					ConflictingPkg: newPkg,
					Reason: fmt.Sprintf("Both jobs affect shared dependency: %s",
						extractPackageName(newPkg)),
				}
			}
		}
	}

	// 3. Check slot conflicts (Phase 4+)
	// TODO: Implement slot conflict detection using package metadata

	// 4. Check blocker conflicts (Phase 4+)
	// TODO: Implement blocker detection (packages that block each other)

	return nil
}

// getAffectedPackages returns all packages affected by this job
// (the package itself + all dependencies)
func (cd *ConflictDetector) getAffectedPackages(job *Job) ([]string, error) {
	affected := []string{job.PackageName}

	// For install/update operations, resolve dependencies
	if job.Type == JobTypeInstall || job.Type == JobTypeUpdate {
		result, err := cd.packageService.ResolvePackage([]string{job.PackageName})
		if err != nil {
			// If resolution fails, return just the package itself
			// Better to be conservative
			return affected, nil
		}

		// Add all packages that would be installed/updated
		for pkgName, version := range result.PackagesToInstall {
			affected = append(affected, fmt.Sprintf("%s-%s", pkgName, version))
		}

		// Add packages that would be updated
		affected = append(affected, result.PackagesToUpdate...)
	}

	return affected, nil
}

// filterActiveJobs returns only running or pending jobs
func filterActiveJobs(jobs []*Job) []*Job {
	active := make([]*Job, 0, len(jobs))
	for _, job := range jobs {
		if job.GetStatus() == JobStatusRunning || job.GetStatus() == JobStatusPending {
			active = append(active, job)
		}
	}
	return active
}

// isSamePackage checks if two package names refer to the same package
// Handles cases like:
// - "dev-lang/go" vs "dev-lang/go-1.22.0"
// - "sys-libs/glibc" vs "sys-libs/glibc-2.38"
func isSamePackage(pkg1, pkg2 string) bool {
	// Extract base package name (category/package without version)
	base1 := extractPackageName(pkg1)
	base2 := extractPackageName(pkg2)
	return base1 == base2
}

// extractPackageName extracts the base package name without version
// "dev-lang/go-1.22.0" → "dev-lang/go"
// "dev-python/pytest-7.4.0-r1" → "dev-python/pytest"
// "sys-libs/glibc" → "sys-libs/glibc"
func extractPackageName(pkg string) string {
	// Simple version: split by last "-" and check if it's a version
	parts := strings.Split(pkg, "/")
	if len(parts) != 2 {
		return pkg // Return as-is if not in category/package format
	}

	category := parts[0]
	nameWithVersion := parts[1]

	// Try to find version separator by iterating from left to right
	// Version typically starts after the last word-like component
	dashIndices := make([]int, 0)
	for i, r := range nameWithVersion {
		if r == '-' {
			dashIndices = append(dashIndices, i)
		}
	}

	if len(dashIndices) == 0 {
		return pkg // No dashes, return as-is
	}

	// Try each dash from right to left to find version start
	for i := len(dashIndices) - 1; i >= 0; i-- {
		dashPos := dashIndices[i]
		potentialVersion := nameWithVersion[dashPos+1:]

		if looksLikeVersion(potentialVersion) {
			return category + "/" + nameWithVersion[:dashPos]
		}
	}

	return pkg
}

// looksLikeVersion checks if string looks like a version number
// Examples: "1.22.0", "2.38", "3.11.2-r1", "7.4.0_alpha"
func looksLikeVersion(s string) bool {
	if len(s) == 0 {
		return false
	}

	// Check if first character is a digit
	if s[0] < '0' || s[0] > '9' {
		return false
	}

	// Contains at least one digit and version-like characters
	hasDigit := false
	for _, r := range s {
		if r >= '0' && r <= '9' {
			hasDigit = true
			continue
		}
		if r >= 'a' && r <= 'z' {
			continue // Allow alpha, beta, rc, etc.
		}
		if r >= 'A' && r <= 'Z' {
			continue // Allow uppercase version suffixes
		}
		if r == '.' || r == '-' || r == '_' {
			continue // Version separators
		}
		return false // Invalid character
	}

	return hasDigit
}

// FormatConflictError formats a conflict into a user-friendly error message
func FormatConflictError(conflict *PackageConflict) string {
	return fmt.Sprintf(
		"Package conflict detected (%s):\n"+
			"  New job: %s %s\n"+
			"  Conflicts with: %s %s (job %s)\n"+
			"  Reason: %s",
		conflict.Type,
		conflict.Job1.Type, conflict.Job1.PackageName,
		conflict.Job2.Type, conflict.Job2.PackageName, conflict.Job2.ID[:8],
		conflict.Reason,
	)
}
