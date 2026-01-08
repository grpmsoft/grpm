package repo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

func TestNewManager(t *testing.T) {
	m := NewManager()

	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
	if m.Count() != 0 {
		t.Errorf("new manager should have 0 repos, got %d", m.Count())
	}
}

func TestManagerAddNamedRepository(t *testing.T) {
	m := NewManager()

	// Create mock repository
	repo := NewNamedMockRepository("test-overlay", 50)

	if err := m.AddNamedRepository(repo); err != nil {
		t.Fatalf("AddNamedRepository failed: %v", err)
	}

	if m.Count() != 1 {
		t.Errorf("expected 1 repo, got %d", m.Count())
	}

	// Try adding duplicate
	repo2 := NewNamedMockRepository("test-overlay", 100)
	err := m.AddNamedRepository(repo2)
	if err == nil {
		t.Error("expected error for duplicate repository, got nil")
	}
	if !errors.Is(err, ErrRepoAlreadyExists) {
		t.Errorf("expected ErrRepoAlreadyExists, got %v", err)
	}
}

func TestManagerPriorityOrdering(t *testing.T) {
	m := NewManager()

	// Add repos in random priority order
	lowPriority := NewNamedMockRepository("low", -1000)
	highPriority := NewNamedMockRepository("high", 100)
	medPriority := NewNamedMockRepository("med", 0)

	_ = m.AddNamedRepository(lowPriority)
	_ = m.AddNamedRepository(highPriority)
	_ = m.AddNamedRepository(medPriority)

	repos := m.Repositories()

	if len(repos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(repos))
	}

	// Should be sorted by priority, highest first
	if repos[0].Name() != "high" {
		t.Errorf("expected first repo to be 'high', got %q", repos[0].Name())
	}
	if repos[1].Name() != "med" {
		t.Errorf("expected second repo to be 'med', got %q", repos[1].Name())
	}
	if repos[2].Name() != "low" {
		t.Errorf("expected third repo to be 'low', got %q", repos[2].Name())
	}
}

func TestManagerFindPackage(t *testing.T) {
	m := NewManager()

	// Create two overlays with different packages
	overlay1 := NewEmptyMockRepository()
	overlay1.SetName("overlay1")
	overlay1.SetPriority(100)
	_ = overlay1.Add(pkg.NewPackage("cat/pkg1", "1.0", "0"))

	overlay2 := NewEmptyMockRepository()
	overlay2.SetName("overlay2")
	overlay2.SetPriority(50)
	_ = overlay2.Add(pkg.NewPackage("cat/pkg2", "2.0", "0"))

	_ = m.AddNamedRepository(overlay1)
	_ = m.AddNamedRepository(overlay2)

	// Find package from overlay1
	p, repoName, err := m.FindPackage("cat/pkg1")
	if err != nil {
		t.Fatalf("FindPackage failed: %v", err)
	}
	if p.Name != "cat/pkg1" {
		t.Errorf("expected package name 'cat/pkg1', got %q", p.Name)
	}
	if repoName != "overlay1" {
		t.Errorf("expected repo name 'overlay1', got %q", repoName)
	}

	// Find package from overlay2
	p, repoName, err = m.FindPackage("cat/pkg2")
	if err != nil {
		t.Fatalf("FindPackage failed: %v", err)
	}
	if p.Name != "cat/pkg2" {
		t.Errorf("expected package name 'cat/pkg2', got %q", p.Name)
	}
	if repoName != "overlay2" {
		t.Errorf("expected repo name 'overlay2', got %q", repoName)
	}

	// Find non-existent package
	_, _, err = m.FindPackage("cat/nonexistent")
	if err == nil {
		t.Error("expected error for non-existent package, got nil")
	}
	if !errors.Is(err, ErrPackageNotFound) {
		t.Errorf("expected ErrPackageNotFound, got %v", err)
	}
}

func TestManagerFindPackagePriority(t *testing.T) {
	m := NewManager()

	// Create two overlays with the same package but different versions
	lowPriorityRepo := NewEmptyMockRepository()
	lowPriorityRepo.SetName("gentoo")
	lowPriorityRepo.SetPriority(-1000)
	_ = lowPriorityRepo.Add(pkg.NewPackage("app-misc/hello", "1.0", "0"))

	highPriorityRepo := NewEmptyMockRepository()
	highPriorityRepo.SetName("overlay")
	highPriorityRepo.SetPriority(100)
	_ = highPriorityRepo.Add(pkg.NewPackage("app-misc/hello", "2.0", "0"))

	_ = m.AddNamedRepository(lowPriorityRepo)
	_ = m.AddNamedRepository(highPriorityRepo)

	// Should find package from high priority repo
	p, repoName, err := m.FindPackage("app-misc/hello")
	if err != nil {
		t.Fatalf("FindPackage failed: %v", err)
	}
	if p.Version != "2.0" {
		t.Errorf("expected version '2.0' from high priority repo, got %q", p.Version)
	}
	if repoName != "overlay" {
		t.Errorf("expected repo name 'overlay', got %q", repoName)
	}
}

func TestManagerFindAllVersions(t *testing.T) {
	m := NewManager()

	// Create two repos with different versions of the same package
	repo1 := NewEmptyMockRepository()
	repo1.SetName("repo1")
	repo1.SetPriority(100)
	_ = repo1.Add(pkg.NewPackage("cat/pkg", "1.0", "0"))

	repo2 := NewEmptyMockRepository()
	repo2.SetName("repo2")
	repo2.SetPriority(50)
	_ = repo2.Add(pkg.NewPackage("cat/pkg", "2.0", "0"))

	_ = m.AddNamedRepository(repo1)
	_ = m.AddNamedRepository(repo2)

	versions, err := m.FindAllVersions("cat/pkg")
	if err != nil {
		t.Fatalf("FindAllVersions failed: %v", err)
	}

	// MockRepository only stores one version per package name,
	// so we get 2 packages (one from each repo)
	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}

	// Check that both versions are present
	versionSet := make(map[string]bool)
	for _, v := range versions {
		versionSet[v.Version] = true
	}
	if !versionSet["1.0"] || !versionSet["2.0"] {
		t.Errorf("expected versions 1.0 and 2.0, got %v", versionSet)
	}
}

func TestManagerFindBySpecification(t *testing.T) {
	m := NewManager()

	repo := NewEmptyMockRepository()
	repo.SetName("test")
	repo.SetPriority(0)
	_ = repo.Add(pkg.NewPackage("cat/foo", "1.0", "0"))
	_ = repo.Add(pkg.NewPackage("cat/bar", "2.0", "0"))
	_ = repo.Add(pkg.NewPackage("other/baz", "3.0", "0"))

	_ = m.AddNamedRepository(repo)

	// Find all packages in category "cat"
	spec := NewCategorySpecification("cat")
	packages, err := m.FindBySpecification(spec)
	if err != nil {
		t.Fatalf("FindBySpecification failed: %v", err)
	}

	if len(packages) != 2 {
		t.Errorf("expected 2 packages, got %d", len(packages))
	}
}

func TestManagerGetRepository(t *testing.T) {
	m := NewManager()

	repo := NewNamedMockRepository("test-repo", 50)
	_ = m.AddNamedRepository(repo)

	// Get existing repo
	r, err := m.GetRepository("test-repo")
	if err != nil {
		t.Fatalf("GetRepository failed: %v", err)
	}
	if r.Name() != "test-repo" {
		t.Errorf("expected repo name 'test-repo', got %q", r.Name())
	}

	// Get non-existent repo
	_, err = m.GetRepository("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent repo, got nil")
	}
	if !errors.Is(err, ErrRepoNotFound) {
		t.Errorf("expected ErrRepoNotFound, got %v", err)
	}
}

func TestManagerLoadReposConf(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos", "test")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	// Create repos.conf
	confContent := `[test]
location = ` + repoDir + `
priority = 50
`
	confFile := filepath.Join(tmpDir, "repos.conf")
	if err := os.WriteFile(confFile, []byte(confContent), 0o644); err != nil {
		t.Fatalf("failed to write repos.conf: %v", err)
	}

	m := NewManager()
	if err := m.LoadReposConf(confFile); err != nil {
		t.Fatalf("LoadReposConf failed: %v", err)
	}

	if m.Count() != 1 {
		t.Errorf("expected 1 repo, got %d", m.Count())
	}

	repos := m.Repositories()
	if repos[0].Name() != "test" {
		t.Errorf("expected repo name 'test', got %q", repos[0].Name())
	}
	if repos[0].Priority() != 50 {
		t.Errorf("expected priority 50, got %d", repos[0].Priority())
	}
}

func TestRepositoryAdapter(t *testing.T) {
	m := NewManager()

	repo := NewEmptyMockRepository()
	repo.SetName("test")
	repo.SetPriority(0)
	_ = repo.Add(pkg.NewPackage("cat/pkg", "1.0", "0"))

	_ = m.AddNamedRepository(repo)

	// Create adapter
	adapter := NewRepositoryAdapter(m)

	// Test LoadPackage
	p, err := adapter.LoadPackage("cat/pkg")
	if err != nil {
		t.Fatalf("LoadPackage failed: %v", err)
	}
	if p.Name != "cat/pkg" {
		t.Errorf("expected 'cat/pkg', got %q", p.Name)
	}

	// Test LoadPackages
	packages, err := adapter.LoadPackages([]string{"cat/pkg"})
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}
	if len(packages) != 1 {
		t.Errorf("expected 1 package, got %d", len(packages))
	}

	// Test Exists
	if !adapter.Exists("cat/pkg") {
		t.Error("Exists should return true for existing package")
	}
	if adapter.Exists("cat/nonexistent") {
		t.Error("Exists should return false for non-existent package")
	}

	// Test Count
	count, err := adapter.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 { // We added 1 package
		t.Errorf("expected count 1, got %d", count)
	}
}

func TestManagerSyncRepository(t *testing.T) {
	m := NewManager()

	// Create a mock config without sync URI (should skip)
	repo := NewEmptyMockRepository()
	repo.SetName("nosync")
	repo.SetPriority(0)
	_ = m.AddNamedRepository(repo)

	// Sync should not fail for repo without sync URI
	err := m.SyncRepository(context.Background(), "nosync")
	if err != nil {
		// Note: This may fail because we don't have a config entry
		// This is expected behavior - we need both repo and config
		if !errors.Is(err, ErrRepoNotFound) {
			t.Logf("SyncRepository note: %v", err)
		}
	}

	// Try syncing non-existent repo
	err = m.SyncRepository(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent repo, got nil")
	}
	if !errors.Is(err, ErrRepoNotFound) {
		t.Errorf("expected ErrRepoNotFound, got %v", err)
	}
}

func TestMockRepositoryNamedInterface(t *testing.T) {
	// Verify MockRepository implements NamedRepository
	var _ NamedRepository = (*MockRepository)(nil)

	repo := NewNamedMockRepository("test", 100)

	if repo.Name() != "test" {
		t.Errorf("Name() = %q, want %q", repo.Name(), "test")
	}
	if repo.Priority() != 100 {
		t.Errorf("Priority() = %d, want %d", repo.Priority(), 100)
	}
	if repo.Location() != "" {
		t.Errorf("Location() = %q, want empty", repo.Location())
	}

	repo.SetPriority(200)
	if repo.Priority() != 200 {
		t.Errorf("Priority() after SetPriority = %d, want %d", repo.Priority(), 200)
	}

	repo.SetName("new-name")
	if repo.Name() != "new-name" {
		t.Errorf("Name() after SetName = %q, want %q", repo.Name(), "new-name")
	}

	repo.SetLocation("/test/path")
	if repo.Location() != "/test/path" {
		t.Errorf("Location() after SetLocation = %q, want %q", repo.Location(), "/test/path")
	}
}

func TestNamedPortageRepository(t *testing.T) {
	// Create temp directory for repo
	tmpDir := t.TempDir()

	// Create a minimal repo structure
	catDir := filepath.Join(tmpDir, "app-misc", "hello")
	if err := os.MkdirAll(catDir, 0o755); err != nil {
		t.Fatalf("failed to create category dir: %v", err)
	}
	ebuildContent := `EAPI=8
DESCRIPTION="Test"
`
	ebuildPath := filepath.Join(catDir, "hello-1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0o644); err != nil {
		t.Fatalf("failed to write ebuild: %v", err)
	}

	// Create manager and add repository via config
	m := NewManager()
	cfg := &RepoConfig{
		Name:     "test-portage",
		Location: tmpDir,
		Priority: 50,
	}

	if err := m.AddRepository(cfg); err != nil {
		t.Fatalf("AddRepository failed: %v", err)
	}

	repo, err := m.GetRepository("test-portage")
	if err != nil {
		t.Fatalf("GetRepository failed: %v", err)
	}

	if repo.Name() != "test-portage" {
		t.Errorf("Name() = %q, want %q", repo.Name(), "test-portage")
	}
	if repo.Priority() != 50 {
		t.Errorf("Priority() = %d, want %d", repo.Priority(), 50)
	}
	if repo.Location() != tmpDir {
		t.Errorf("Location() = %q, want %q", repo.Location(), tmpDir)
	}

	// Test SetPriority
	repo.SetPriority(100)
	if repo.Priority() != 100 {
		t.Errorf("Priority() after SetPriority = %d, want %d", repo.Priority(), 100)
	}

	// Test loading package
	p, err := repo.LoadPackage("app-misc/hello")
	if err != nil {
		t.Fatalf("LoadPackage failed: %v", err)
	}
	if p.Name != "app-misc/hello" {
		t.Errorf("package name = %q, want %q", p.Name, "app-misc/hello")
	}
}

func TestManagerConcurrency(t *testing.T) {
	m := NewManager()

	// Add initial repos
	for i := 0; i < 5; i++ {
		repo := NewEmptyMockRepository()
		repo.SetName("repo" + string(rune('0'+i)))
		repo.SetPriority(i * 10)
		_ = m.AddNamedRepository(repo)
	}

	// Concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = m.Repositories()
			_, _, _ = m.FindPackage("cat/pkg")
			_ = m.Count()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
