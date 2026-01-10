package eclass

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewHybridLoader(t *testing.T) {
	cache := NewCache()
	loader := NewHybridLoader(cache, nil)

	if loader == nil {
		t.Fatal("NewHybridLoader returned nil")
	}

	if loader.cache != cache {
		t.Error("cache not set correctly")
	}

	if loader.executor == nil {
		t.Error("executor not created")
	}
}

func TestHybridLoaderDynamicInherit(t *testing.T) {
	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test eclass
	eclassContent := `
# test.eclass
IUSE="test-flag"
DEPEND="dev-libs/test"
`
	if err := os.WriteFile(filepath.Join(eclassDir, "test.eclass"), []byte(eclassContent), 0644); err != nil {
		t.Fatal(err)
	}

	cache, err := NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	loader := NewHybridLoader(cache, nil,
		WithHybridOutput(&stdout, &stderr),
		WithVerbose(true),
	)

	ctx := context.Background()
	if err := loader.Inherit(ctx, []string{"test"}); err != nil {
		t.Fatalf("Inherit failed: %v", err)
	}

	// Verify inherited
	inherited := loader.GetExecutor().GetInherited()
	if len(inherited) != 1 || inherited[0] != "test" {
		t.Errorf("expected inherited=['test'], got %v", inherited)
	}
}

// MockGoEclass implements GoEclassImpl for testing.
type MockGoEclass struct {
	name         string
	executed     bool
	phasesCalled map[string]bool
}

func (m *MockGoEclass) Name() string {
	return m.name
}

func (m *MockGoEclass) Execute(ctx context.Context, env map[string]string) error {
	m.executed = true
	env["IUSE"] = "mock-flag"
	env["DEPEND"] = "mock-dep"
	return nil
}

func (m *MockGoEclass) HasPhaseFunction(phase string) bool {
	return phase == "src_compile"
}

func (m *MockGoEclass) ExecutePhase(ctx context.Context, phase string, env map[string]string) error {
	if m.phasesCalled == nil {
		m.phasesCalled = make(map[string]bool)
	}
	m.phasesCalled[phase] = true
	return nil
}

func TestHybridLoaderGoFallback(t *testing.T) {
	cache := NewCache() // Empty cache - no dynamic eclasses

	mockEclass := &MockGoEclass{name: "mock"}

	var stdout, stderr bytes.Buffer
	loader := NewHybridLoader(cache, nil,
		WithGoFallback(mockEclass),
		WithHybridOutput(&stdout, &stderr),
		WithVerbose(true),
	)

	ctx := context.Background()
	if err := loader.Inherit(ctx, []string{"mock"}); err != nil {
		t.Fatalf("Inherit failed: %v", err)
	}

	if !mockEclass.executed {
		t.Error("expected Go fallback to be executed")
	}
}

func TestHybridLoaderNotFound(t *testing.T) {
	cache := NewCache()

	var stdout, stderr bytes.Buffer
	loader := NewHybridLoader(cache, nil,
		WithHybridOutput(&stdout, &stderr),
	)

	ctx := context.Background()
	err := loader.Inherit(ctx, []string{"nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent eclass")
	}
}

func TestHybridLoaderHasGoFallback(t *testing.T) {
	cache := NewCache()
	mockEclass := &MockGoEclass{name: "test"}

	loader := NewHybridLoader(cache, nil, WithGoFallback(mockEclass))

	if !loader.HasGoFallback("test") {
		t.Error("expected HasGoFallback('test') to return true")
	}

	if loader.HasGoFallback("other") {
		t.Error("expected HasGoFallback('other') to return false")
	}
}

func TestBuildCacheFromRepos(t *testing.T) {
	tmpDir := t.TempDir()

	// Create repo1 (master)
	repo1 := filepath.Join(tmpDir, "gentoo")
	repo1Eclass := filepath.Join(repo1, "eclass")
	if err := os.MkdirAll(repo1Eclass, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo1Eclass, "base.eclass"), []byte("# base"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create repo2 (overlay)
	repo2 := filepath.Join(tmpDir, "overlay")
	repo2Eclass := filepath.Join(repo2, "eclass")
	if err := os.MkdirAll(repo2Eclass, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo2Eclass, "custom.eclass"), []byte("# custom"), 0644); err != nil {
		t.Fatal(err)
	}

	cache, err := BuildCacheFromRepos([]string{repo1, repo2}, nil)
	if err != nil {
		t.Fatalf("BuildCacheFromRepos failed: %v", err)
	}

	// Should have both eclasses
	if !cache.Has("base") {
		t.Error("expected 'base' eclass")
	}
	if !cache.Has("custom") {
		t.Error("expected 'custom' eclass")
	}
}

func TestDefaultLocations(t *testing.T) {
	locs := DefaultLocations()

	if len(locs) < 1 {
		t.Error("expected at least one default location")
	}

	// Check that expected paths are present
	found := false
	for _, loc := range locs {
		if loc == "/var/db/repos/gentoo/eclass" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected /var/db/repos/gentoo/eclass in default locations")
	}
}

func TestFindRepositoryEclass(t *testing.T) {
	tmpDir := t.TempDir()

	// Create repo with eclass
	repo := filepath.Join(tmpDir, "repo")
	eclassDir := filepath.Join(repo, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatal(err)
	}

	eclassPath := filepath.Join(eclassDir, "test.eclass")
	if err := os.WriteFile(eclassPath, []byte("# test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Find existing
	found, err := FindRepositoryEclass("test", []string{repo})
	if err != nil {
		t.Fatalf("FindRepositoryEclass failed: %v", err)
	}
	if found != eclassPath {
		t.Errorf("expected %s, got %s", eclassPath, found)
	}

	// Find nonexistent
	_, err = FindRepositoryEclass("nonexistent", []string{repo})
	if err == nil {
		t.Error("expected error for nonexistent eclass")
	}
}

func TestHybridLoaderDoubleInherit(t *testing.T) {
	tmpDir := t.TempDir()
	eclassDir := filepath.Join(tmpDir, "eclass")
	if err := os.MkdirAll(eclassDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(eclassDir, "test.eclass"), []byte("# test"), 0644); err != nil {
		t.Fatal(err)
	}

	cache, err := NewCacheWithLocations([]string{eclassDir})
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	loader := NewHybridLoader(cache, nil, WithHybridOutput(&stdout, &stderr))

	ctx := context.Background()

	// First inherit
	if err := loader.Inherit(ctx, []string{"test"}); err != nil {
		t.Fatal(err)
	}

	// Second inherit (same eclass) should skip
	stdout.Reset()
	if err := loader.Inherit(ctx, []string{"test"}); err != nil {
		t.Fatal(err)
	}

	// Should mention skipping
	output := stdout.String()
	if output == "" {
		t.Error("expected output mentioning skip")
	}
}
