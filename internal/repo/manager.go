package repo

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/pkg"
	syncer "github.com/grpmsoft/grpm/internal/sync"
)

// Manager handles multiple package repositories with priority ordering.
// Repositories are searched in priority order (highest first) when resolving packages.
// This implements the overlay system used in Gentoo.
type Manager struct {
	mu      sync.RWMutex
	repos   []NamedRepository // Sorted by priority (highest first)
	configs map[string]*RepoConfig
}

// NamedRepository extends Repository with name and priority information.
// This interface is required for repositories managed by Manager.
type NamedRepository interface {
	Repository
	// Name returns the repository name (e.g., "gentoo", "overlay")
	Name() string
	// Priority returns the repository priority (higher = checked first)
	Priority() int
	// SetPriority sets the repository priority
	SetPriority(priority int)
	// Location returns the filesystem path of the repository
	Location() string
}

// Errors for repository manager operations.
var (
	ErrRepoNotFound      = errors.New("repository not found")
	ErrRepoAlreadyExists = errors.New("repository already exists")
	ErrPackageNotFound   = errors.New("package not found in any repository")
)

// NewManager creates a new repository manager.
func NewManager() *Manager {
	return &Manager{
		repos:   make([]NamedRepository, 0),
		configs: make(map[string]*RepoConfig),
	}
}

// AddRepository adds a repository with the given configuration.
// The repository is created from the configuration and added to the manager.
func (m *Manager) AddRepository(cfg *RepoConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicates
	if _, exists := m.configs[cfg.Name]; exists {
		return fmt.Errorf("%w: %s", ErrRepoAlreadyExists, cfg.Name)
	}

	// Create repository from config
	repo, err := m.createRepositoryFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating repository %s: %w", cfg.Name, err)
	}

	m.configs[cfg.Name] = cfg
	m.repos = append(m.repos, repo)
	m.sortByPriority()

	return nil
}

// AddNamedRepository adds an existing NamedRepository to the manager.
// Useful for adding mock repositories in tests.
func (m *Manager) AddNamedRepository(repo NamedRepository) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicates
	for _, r := range m.repos {
		if r.Name() == repo.Name() {
			return fmt.Errorf("%w: %s", ErrRepoAlreadyExists, repo.Name())
		}
	}

	m.repos = append(m.repos, repo)
	m.sortByPriority()

	return nil
}

// createRepositoryFromConfig creates a NamedRepository from configuration.
func (m *Manager) createRepositoryFromConfig(cfg *RepoConfig) (NamedRepository, error) {
	// Create the underlying Portage repository
	portageRepo, err := NewPortageRepository(cfg.Location)
	if err != nil {
		return nil, err
	}

	// Wrap with named repository
	return &namedPortageRepository{
		PortageRepository: portageRepo,
		name:              cfg.Name,
		priority:          cfg.Priority,
		location:          cfg.Location,
	}, nil
}

// LoadReposConf loads repository configuration from the specified path.
// Path can be a single file or a directory containing .conf files.
func (m *Manager) LoadReposConf(path string) error {
	configs, err := LoadReposConf(path)
	if err != nil {
		return err
	}

	for _, cfg := range configs {
		if err := m.AddRepository(cfg); err != nil {
			// Log warning but continue with other repos
			logging.Debug("Warning: failed to add repository %s: %v", cfg.Name, err)
		}
	}

	return nil
}

// LoadDefaults loads the default Gentoo repository configuration.
func (m *Manager) LoadDefaults() error {
	for _, cfg := range DefaultReposConf() {
		if err := m.AddRepository(cfg); err != nil {
			return err
		}
	}
	return nil
}

// FindPackage searches repositories in priority order and returns the first match.
// Returns the package, repository name, and any error.
func (m *Manager) FindPackage(atom string) (*pkg.Package, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, repo := range m.repos {
		p, err := repo.LoadPackage(atom)
		if err == nil {
			return p, repo.Name(), nil
		}
		// Continue to next repo if not found
	}

	return nil, "", fmt.Errorf("%w: %s", ErrPackageNotFound, atom)
}

// FindAllVersions finds all versions of a package across all repositories.
// Returns packages grouped by repository with repo name as source.
func (m *Manager) FindAllVersions(name string) ([]*pkg.Package, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var allVersions []*pkg.Package

	for _, repo := range m.repos {
		versions, err := repo.GetAllVersions(name)
		if err != nil {
			continue // Package not in this repo
		}
		allVersions = append(allVersions, versions...)
	}

	if len(allVersions) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrPackageNotFound, name)
	}

	return allVersions, nil
}

// FindBySpecification finds packages matching the specification across all repos.
// Results are returned in priority order (highest priority repo first).
func (m *Manager) FindBySpecification(spec Specification) ([]*pkg.Package, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*pkg.Package
	seen := make(map[string]bool) // Track seen package+version to avoid duplicates

	for _, repo := range m.repos {
		packages, err := repo.FindBySpecification(spec)
		if err != nil {
			continue
		}

		for _, p := range packages {
			key := p.Name + "@" + p.Version
			if !seen[key] {
				seen[key] = true
				results = append(results, p)
			}
		}
	}

	return results, nil
}

// SyncAll synchronizes all repositories that have AutoSync enabled.
func (m *Manager) SyncAll(ctx context.Context) error {
	m.mu.RLock()
	configs := make([]*RepoConfig, 0, len(m.configs))
	for _, cfg := range m.configs {
		configs = append(configs, cfg)
	}
	m.mu.RUnlock()

	var syncErrors []error

	for _, cfg := range configs {
		if !cfg.AutoSync {
			logging.Debug("Skipping %s (auto-sync disabled)", cfg.Name)
			continue
		}

		if err := m.syncRepository(ctx, cfg); err != nil {
			syncErrors = append(syncErrors, fmt.Errorf("syncing %s: %w", cfg.Name, err))
		}
	}

	if len(syncErrors) > 0 {
		return fmt.Errorf("sync failed for %d repositories: %w", len(syncErrors), syncErrors[0])
	}

	return nil
}

// SyncRepository synchronizes a single repository by name.
func (m *Manager) SyncRepository(ctx context.Context, name string) error {
	m.mu.RLock()
	cfg, exists := m.configs[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("%w: %s", ErrRepoNotFound, name)
	}

	return m.syncRepository(ctx, cfg)
}

// syncRepository performs the actual sync operation.
func (m *Manager) syncRepository(ctx context.Context, cfg *RepoConfig) error {
	if cfg.SyncURI == "" {
		logging.Debug("Skipping %s (no sync-uri)", cfg.Name)
		return nil
	}

	// Determine sync method
	method := syncer.MethodAuto
	switch cfg.SyncType {
	case "rsync":
		method = syncer.MethodRsync
	case "git":
		method = syncer.MethodGit
	}

	s, err := syncer.NewSyncer(method)
	if err != nil {
		return fmt.Errorf("creating syncer: %w", err)
	}

	syncConfig := &syncer.SyncConfig{
		Method:    method,
		RepoPath:  cfg.Location,
		SourceURL: cfg.SyncURI,
		Verbose:   true,
	}

	logging.Debug("Syncing %s from %s...", cfg.Name, cfg.SyncURI)
	result, err := s.Sync(ctx, syncConfig)
	if err != nil {
		return err
	}

	logging.Debug("Synced %s: %d files changed in %s", cfg.Name, result.FilesChanged, result.Duration)
	return nil
}

// GetRepository returns a repository by name.
func (m *Manager) GetRepository(name string) (NamedRepository, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, repo := range m.repos {
		if repo.Name() == name {
			return repo, nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, name)
}

// Repositories returns all repositories sorted by priority (highest first).
func (m *Manager) Repositories() []NamedRepository {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make([]NamedRepository, len(m.repos))
	copy(result, m.repos)
	return result
}

// Count returns the total number of repositories.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.repos)
}

// sortByPriority sorts repositories by priority (highest first).
// Must be called with lock held.
func (m *Manager) sortByPriority() {
	sort.Slice(m.repos, func(i, j int) bool {
		return m.repos[i].Priority() > m.repos[j].Priority()
	})
}

// namedPortageRepository wraps PortageRepository with name and priority.
type namedPortageRepository struct {
	*PortageRepository
	name     string
	priority int
	location string
}

// Name returns the repository name.
func (r *namedPortageRepository) Name() string {
	return r.name
}

// Priority returns the repository priority.
func (r *namedPortageRepository) Priority() int {
	return r.priority
}

// SetPriority sets the repository priority.
func (r *namedPortageRepository) SetPriority(priority int) {
	r.priority = priority
}

// Location returns the filesystem path of the repository.
func (r *namedPortageRepository) Location() string {
	return r.location
}

// RepositoryAdapter wraps Manager to implement the Repository interface.
// This allows Manager to be used wherever a single Repository is expected.
type RepositoryAdapter struct {
	manager *Manager
}

// NewRepositoryAdapter creates a Repository adapter for the Manager.
func NewRepositoryAdapter(manager *Manager) *RepositoryAdapter {
	return &RepositoryAdapter{manager: manager}
}

// LoadPackage loads a package from the highest priority repository.
func (a *RepositoryAdapter) LoadPackage(name string) (*pkg.Package, error) {
	p, _, err := a.manager.FindPackage(name)
	return p, err
}

// LoadPackageVersion loads a specific version of a package.
func (a *RepositoryAdapter) LoadPackageVersion(name, version string) (*pkg.Package, error) {
	for _, repo := range a.manager.Repositories() {
		p, err := repo.LoadPackageVersion(name, version)
		if err == nil {
			return p, nil
		}
	}
	return nil, fmt.Errorf("version %s not found for %s in any repository", version, name)
}

// LoadPackages loads multiple packages.
func (a *RepositoryAdapter) LoadPackages(names []string) ([]*pkg.Package, error) {
	result := make([]*pkg.Package, 0, len(names))
	for _, name := range names {
		p, err := a.LoadPackage(name)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, nil
}

// FindBySpecification finds packages matching the specification.
func (a *RepositoryAdapter) FindBySpecification(spec Specification) ([]*pkg.Package, error) {
	return a.manager.FindBySpecification(spec)
}

// FindByAtom finds all packages matching a PMS-compliant atom across all repositories.
func (a *RepositoryAdapter) FindByAtom(atom *pkg.Atom) ([]*pkg.Package, error) {
	if atom == nil {
		return nil, fmt.Errorf("atom is nil")
	}

	var results []*pkg.Package
	seen := make(map[string]bool)

	for _, repo := range a.manager.Repositories() {
		packages, err := repo.FindByAtom(atom)
		if err != nil {
			continue
		}
		for _, p := range packages {
			key := p.Name + "@" + p.Version
			if !seen[key] {
				seen[key] = true
				results = append(results, p)
			}
		}
	}

	return results, nil
}

// GetAllVersions returns all versions of a package.
func (a *RepositoryAdapter) GetAllVersions(packageName string) ([]*pkg.Package, error) {
	return a.manager.FindAllVersions(packageName)
}

// Exists checks if a package exists in any repository.
func (a *RepositoryAdapter) Exists(name string) bool {
	_, _, err := a.manager.FindPackage(name)
	return err == nil
}

// Count returns the total number of unique packages across all repos.
// Note: This is expensive for large repositories.
func (a *RepositoryAdapter) Count() (int, error) {
	total := 0
	for _, repo := range a.manager.Repositories() {
		count, err := repo.Count()
		if err != nil {
			continue
		}
		total += count
	}
	return total, nil
}
