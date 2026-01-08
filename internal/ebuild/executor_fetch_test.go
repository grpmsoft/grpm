package ebuild

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/grpmsoft/grpm/internal/fetch"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// mockFetcher is a mock implementation of fetch.Fetcher for testing.
type mockFetcher struct {
	fetchCalled   bool
	fetchOneCalls int
	distfiles     []fetch.Distfile
	destDir       string
	shouldFail    bool
	failError     error
}

func (m *mockFetcher) Fetch(ctx context.Context, distfiles []fetch.Distfile, destDir string) error {
	m.fetchCalled = true
	m.distfiles = distfiles
	m.destDir = destDir
	if m.shouldFail {
		return m.failError
	}
	return nil
}

func (m *mockFetcher) FetchOne(ctx context.Context, distfile fetch.Distfile, destDir string) error {
	m.fetchOneCalls++
	if m.shouldFail {
		return m.failError
	}
	return nil
}

func TestExecutor_WithFetcher(t *testing.T) {
	// Create test package
	testPkg := &pkg.Package{
		Name:    "app-misc/hello",
		Version: "2.10",
		Slot:    pkg.NewSlot("0", ""),
	}

	// Create executor with mock fetcher
	mf := &mockFetcher{}

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(),
		DistDir: t.TempDir(),
		Fetcher: mf,
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// Verify fetcher is set
	if executor.Fetcher == nil {
		t.Error("Fetcher should be set on executor")
	}

	// Verify RepoPath is set
	if executor.RepoPath != opts.PortDir {
		t.Errorf("RepoPath = %q, want %q", executor.RepoPath, opts.PortDir)
	}
}

func TestExecutor_FetchDistfiles_NoFetcher(t *testing.T) {
	// Create test package
	testPkg := &pkg.Package{
		Name:    "app-misc/hello",
		Version: "2.10",
		Slot:    pkg.NewSlot("0", ""),
	}

	// Create executor without fetcher
	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(),
		DistDir: t.TempDir(),
		Fetcher: nil, // No fetcher
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// fetchDistfiles should return nil when no fetcher is configured
	err = executor.fetchDistfiles(context.Background())
	if err != nil {
		t.Errorf("fetchDistfiles should return nil when no fetcher: %v", err)
	}
}

func TestExecutor_FetchDistfiles_NoManifest(t *testing.T) {
	// Create test package
	testPkg := &pkg.Package{
		Name:    "app-misc/hello",
		Version: "2.10",
		Slot:    pkg.NewSlot("0", ""),
	}

	mf := &mockFetcher{}

	// Create executor with empty repo (no Manifest file)
	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: t.TempDir(), // Empty, no Manifest
		DistDir: t.TempDir(),
		Fetcher: mf,
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// fetchDistfiles should succeed when no Manifest (skip fetch)
	err = executor.fetchDistfiles(context.Background())
	if err != nil {
		t.Errorf("fetchDistfiles should succeed when no Manifest: %v", err)
	}

	// Fetcher should not be called
	if mf.fetchCalled {
		t.Error("Fetcher should not be called when no Manifest")
	}
}

func TestExecutor_FetchDistfiles_WithManifest(t *testing.T) {
	// Create test package
	testPkg := &pkg.Package{
		Name:    "app-misc/hello",
		Version: "2.10",
		Slot:    pkg.NewSlot("0", ""),
	}

	mf := &mockFetcher{}

	// Create temporary repo with Manifest
	repoDir := t.TempDir()
	pkgDir := filepath.Join(repoDir, "app-misc", "hello")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir: %v", err)
	}

	// Create Manifest file
	manifestContent := `DIST hello-2.10.tar.gz 725946 BLAKE2B abc123 SHA512 def456
`
	manifestPath := filepath.Join(pkgDir, "Manifest")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("Failed to write Manifest: %v", err)
	}

	distDir := t.TempDir()

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: repoDir,
		DistDir: distDir,
		Fetcher: mf,
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// fetchDistfiles should call the fetcher
	err = executor.fetchDistfiles(context.Background())
	if err != nil {
		t.Errorf("fetchDistfiles failed: %v", err)
	}

	// Verify fetcher was called
	if !mf.fetchCalled {
		t.Error("Fetcher should be called")
	}

	// Verify correct distfiles were passed
	if len(mf.distfiles) != 1 {
		t.Errorf("Expected 1 distfile, got %d", len(mf.distfiles))
	}

	if len(mf.distfiles) > 0 && mf.distfiles[0].Filename != "hello-2.10.tar.gz" {
		t.Errorf("Distfile filename = %q, want %q", mf.distfiles[0].Filename, "hello-2.10.tar.gz")
	}

	// Verify destination directory
	if mf.destDir != distDir {
		t.Errorf("destDir = %q, want %q", mf.destDir, distDir)
	}

	// Verify A environment variable is set
	if executor.Env.A != "hello-2.10.tar.gz" {
		t.Errorf("Env.A = %q, want %q", executor.Env.A, "hello-2.10.tar.gz")
	}
}

func TestExecutor_FetchDistfiles_MultipleFiles(t *testing.T) {
	// Create test package
	testPkg := &pkg.Package{
		Name:    "dev-libs/openssl",
		Version: "3.0.0",
		Slot:    pkg.NewSlot("0", "3"),
	}

	mf := &mockFetcher{}

	// Create temporary repo with Manifest
	repoDir := t.TempDir()
	pkgDir := filepath.Join(repoDir, "dev-libs", "openssl")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir: %v", err)
	}

	// Create Manifest file with multiple DIST entries
	manifestContent := `DIST openssl-3.0.0.tar.gz 14973764 BLAKE2B abc123 SHA512 def456
DIST openssl-3.0.0-patches.tar.xz 12345 BLAKE2B 111222 SHA512 333444
`
	manifestPath := filepath.Join(pkgDir, "Manifest")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("Failed to write Manifest: %v", err)
	}

	opts := ExecutorOptions{
		TmpDir:  t.TempDir(),
		PortDir: repoDir,
		DistDir: t.TempDir(),
		Fetcher: mf,
	}

	executor, err := NewExecutor(testPkg, opts)
	if err != nil {
		t.Fatalf("NewExecutor failed: %v", err)
	}

	// fetchDistfiles should call the fetcher
	err = executor.fetchDistfiles(context.Background())
	if err != nil {
		t.Errorf("fetchDistfiles failed: %v", err)
	}

	// Verify both distfiles were passed
	if len(mf.distfiles) != 2 {
		t.Errorf("Expected 2 distfiles, got %d", len(mf.distfiles))
	}

	// Verify A environment variable contains both files
	expectedA := "openssl-3.0.0.tar.gz openssl-3.0.0-patches.tar.xz"
	if executor.Env.A != expectedA {
		t.Errorf("Env.A = %q, want %q", executor.Env.A, expectedA)
	}
}

func TestPhaseFetch_Defined(t *testing.T) {
	// Verify PhaseFetch is defined and has correct value
	if PhaseFetch.String() != "fetch" {
		t.Errorf("PhaseFetch.String() = %q, want %q", PhaseFetch.String(), "fetch")
	}

	// Verify it's considered a build phase
	if !PhaseFetch.IsBuildPhase() {
		t.Error("PhaseFetch should be a build phase")
	}

	// Verify IsFetchPhase works
	if !PhaseFetch.IsFetchPhase() {
		t.Error("PhaseFetch.IsFetchPhase() should return true")
	}

	if PhaseUnpack.IsFetchPhase() {
		t.Error("PhaseUnpack.IsFetchPhase() should return false")
	}
}

func TestEnvironment_AVariable(t *testing.T) {
	testPkg := &pkg.Package{
		Name:    "app-misc/hello",
		Version: "2.10",
		Slot:    pkg.NewSlot("0", ""),
	}

	env, err := NewEnvironment(testPkg, "/var/tmp/portage", "/var/db/repos/gentoo", "/var/cache/distfiles")
	if err != nil {
		t.Fatalf("NewEnvironment failed: %v", err)
	}

	// A should initially be empty
	if env.A != "" {
		t.Errorf("Initial A = %q, want empty", env.A)
	}

	// Set A
	env.A = "hello-2.10.tar.gz"

	// Verify it appears in ToMap
	envMap := env.ToMap()
	if envMap["A"] != "hello-2.10.tar.gz" {
		t.Errorf("envMap[A] = %q, want %q", envMap["A"], "hello-2.10.tar.gz")
	}

	// Verify it appears in ToSlice
	envSlice := env.ToSlice()
	found := false
	for _, s := range envSlice {
		if s == "A=hello-2.10.tar.gz" {
			found = true
			break
		}
	}
	if !found {
		t.Error("A=hello-2.10.tar.gz not found in ToSlice()")
	}
}

func TestJoinStrings(t *testing.T) {
	tests := []struct {
		input    []string
		sep      string
		expected string
	}{
		{[]string{}, " ", ""},
		{[]string{"a"}, " ", "a"},
		{[]string{"a", "b"}, " ", "a b"},
		{[]string{"a", "b", "c"}, "-", "a-b-c"},
	}

	for _, tt := range tests {
		result := joinStrings(tt.input, tt.sep)
		if result != tt.expected {
			t.Errorf("joinStrings(%v, %q) = %q, want %q", tt.input, tt.sep, result, tt.expected)
		}
	}
}

func TestIsManifestNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"manifest not found", fetch.ErrManifestNotFound, true},
		{"download failed", fetch.ErrDownloadFailed, false},
		{"wrapped manifest not found", fmt.Errorf("context: %w", fetch.ErrManifestNotFound), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isManifestNotFound(tt.err)
			if result != tt.expected {
				t.Errorf("isManifestNotFound(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}
