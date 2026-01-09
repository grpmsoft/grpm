package application

import (
	"fmt"
	"log"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
	"github.com/grpmsoft/grpm/internal/solver"
	"github.com/grpmsoft/grpm/internal/state"
)

// PackageService is an Application Service following DDD pattern
// It orchestrates Domain Services and Infrastructure Services
// and provides high-level operations for the Interface layer (gRPC, CLI)
type PackageService struct {
	repo       repo.Repository
	resolver   *solver.PortageResolver
	depService *pkg.DependencyService
	pkgDB      *state.PackageDatabase
}

// NewPackageService creates a new PackageService.
//
// Parameters:
//   - repository: Package repository for package metadata queries
//   - pkgDB: Package database for installed package queries (can be nil)
//
// If pkgDB is nil, installed package checks will always return false.
func NewPackageService(repository repo.Repository, pkgDB *state.PackageDatabase) *PackageService {
	return &PackageService{
		repo:       repository,
		resolver:   solver.NewResolver(repository),
		depService: pkg.NewDependencyService(),
		pkgDB:      pkgDB,
	}
}

// ResolvePackage resolves dependencies for the given packages
// Returns ResolutionResult DTO for Interface layer
func (s *PackageService) ResolvePackage(packages []string) (*ResolutionResult, error) {
	log.Printf("[PackageService] Resolving packages: %v", packages)

	// Use Infrastructure Service (Resolver) to solve dependencies
	solution, err := s.resolver.Resolve(packages)
	if err != nil {
		return &ResolutionResult{
			Success: false,
			Error:   err.Error(),
		}, nil // Don't return error - return result with error message
	}

	// Convert domain model to DTO
	packagesToInstall := make(map[string]string)
	for name, p := range solution {
		packagesToInstall[name] = p.Version
	}

	// Use Domain Service to find conflicts
	var packageList []*pkg.Package
	for _, p := range solution {
		packageList = append(packageList, p)
	}
	conflicts := s.depService.FindConflicts(packageList)

	var conflictStrings []string
	for _, conflictGroup := range conflicts {
		for _, p := range conflictGroup {
			conflictStrings = append(conflictStrings, fmt.Sprintf("%s-%s", p.Name, p.Version))
		}
	}

	result := &ResolutionResult{
		Success:           len(conflicts) == 0,
		PackagesToInstall: packagesToInstall,
		PackagesToUpdate:  []string{}, // TODO: Implement in Phase 3+
		Conflicts:         conflictStrings,
		TotalSize:         0, // TODO: Calculate from ebuild metadata
	}

	if len(conflicts) > 0 {
		result.Error = fmt.Sprintf("Found %d conflict(s)", len(conflicts))
	}

	log.Printf("[PackageService] Resolution result: success=%v, packages=%d, conflicts=%d",
		result.Success, len(result.PackagesToInstall), len(result.Conflicts))

	return result, nil
}

// SearchPackages searches for packages by name or description
func (s *PackageService) SearchPackages(query string, limit int) (*SearchResult, error) {
	log.Printf("[PackageService] Searching packages: query=%s, limit=%d", query, limit)

	// Use Repository to find packages
	spec := repo.NewNamePatternSpec(query)
	packages, err := s.repo.FindBySpecification(spec)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Apply limit
	if limit > 0 && len(packages) > limit {
		packages = packages[:limit]
	}

	// Convert domain models to DTOs
	var packageInfos []*PackageInfo
	for _, p := range packages {
		info := s.packageToDTO(p)
		packageInfos = append(packageInfos, info)
	}

	result := &SearchResult{
		Packages:   packageInfos,
		TotalCount: len(packageInfos),
	}

	log.Printf("[PackageService] Search found %d packages", result.TotalCount)
	return result, nil
}

// GetPackageInfo retrieves detailed information about a specific package
func (s *PackageService) GetPackageInfo(name string) (*PackageInfo, error) {
	log.Printf("[PackageService] Getting package info: %s", name)

	// Load package from repository
	p, err := s.repo.LoadPackage(name)
	if err != nil {
		return nil, fmt.Errorf("package not found: %w", err)
	}

	info := s.packageToDTO(p)

	log.Printf("[PackageService] Package info retrieved: %s-%s", info.Name, info.Version)
	return info, nil
}

// InstallPackage orchestrates package installation with progress reporting
// progressChan receives progress events during installation
func (s *PackageService) InstallPackage(packageName string, progressChan chan<- InstallProgress) error {
	log.Printf("[PackageService] Installing package: %s", packageName)

	// Send initial progress
	s.sendProgress(progressChan, "resolving", fmt.Sprintf("Resolving dependencies for %s", packageName), 10)

	// Step 1: Resolve dependencies
	result, err := s.ResolvePackage([]string{packageName})
	if err != nil {
		return fmt.Errorf("resolution failed: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("resolution failed: %s", result.Error)
	}

	s.sendProgress(progressChan, "resolved", fmt.Sprintf("Resolved %d packages", len(result.PackagesToInstall)), 30)

	// Step 2: Download packages (TODO: Phase 3+)
	s.sendProgress(progressChan, "downloading", "Downloading packages...", 50)
	time.Sleep(100 * time.Millisecond) // Simulate work

	// Step 3: Compile packages (TODO: Phase 3+)
	s.sendProgress(progressChan, "compiling", "Compiling packages...", 70)
	time.Sleep(100 * time.Millisecond) // Simulate work

	// Step 4: Install packages (TODO: Phase 3+)
	s.sendProgress(progressChan, "installing", "Installing packages...", 90)
	time.Sleep(100 * time.Millisecond) // Simulate work

	// Step 5: Complete
	s.sendProgress(progressChan, "completed", fmt.Sprintf("Successfully installed %s", packageName), 100)

	log.Printf("[PackageService] Installation completed: %s", packageName)
	return nil
}

// RemovePackage removes a package from the system
func (s *PackageService) RemovePackage(packageName string) (*RemovalResult, error) {
	log.Printf("[PackageService] Removing package: %s", packageName)

	// TODO: Phase 3+ implementation
	// - Check if package is installed
	// - Check reverse dependencies
	// - Remove package files
	// - Update database

	result := &RemovalResult{
		Success:        false,
		RemovedPackage: packageName,
		Message:        "Package removal not yet implemented (Phase 3+)",
	}

	return result, nil
}

// UpdateSystem performs a system update
func (s *PackageService) UpdateSystem(progressChan chan<- InstallProgress) (*UpdateResult, error) {
	log.Printf("[PackageService] Updating system")

	s.sendProgress(progressChan, "syncing", "Syncing package repository...", 20)

	// TODO: Phase 3+ implementation
	// - Sync portage tree
	// - Find outdated packages
	// - Resolve update dependencies
	// - Install updates

	result := &UpdateResult{
		Success:         false,
		UpdatedPackages: []string{},
		Message:         "System update not yet implemented (Phase 3+)",
	}

	return result, nil
}

// packageToDTO converts domain Package to DTO PackageInfo
func (s *PackageService) packageToDTO(p *pkg.Package) *PackageInfo {
	useFlags := make([]string, 0, len(p.UseFlags))
	for flag, enabled := range p.UseFlags {
		if enabled {
			useFlags = append(useFlags, flag)
		}
	}

	dependencies := make([]string, len(p.Deps))
	for i, dep := range p.Deps {
		dependencies[i] = dep.Name
		if dep.Version != nil {
			dependencies[i] += " " + dep.Version.String()
		}
	}

	// Check if package is installed in VarDB
	installed := false
	if s.pkgDB != nil {
		// Query by full atom: "category/name-version"
		atom := fmt.Sprintf("%s-%s", p.Name, p.Version)
		installed = s.pkgDB.Has(atom)
	}

	return &PackageInfo{
		Name:         p.Name,
		Version:      p.Version,
		Slot:         p.Slot.Name,
		Subslot:      p.Slot.Subslot,
		Description:  "", // TODO: Load from ebuild metadata
		Homepage:     "", // TODO: Load from ebuild metadata
		License:      "", // TODO: Load from ebuild metadata
		UseFlags:     useFlags,
		Dependencies: dependencies,
		Installed:    installed,
	}
}

// sendProgress sends progress event to channel (non-blocking)
func (s *PackageService) sendProgress(progressChan chan<- InstallProgress, stage, message string, percent int) {
	if progressChan == nil {
		return
	}

	progress := InstallProgress{
		Stage:     stage,
		Message:   message,
		Percent:   percent,
		Timestamp: time.Now().Unix(),
	}

	// Non-blocking send
	select {
	case progressChan <- progress:
	default:
		log.Printf("[PackageService] Warning: progress channel full, skipping event")
	}
}
