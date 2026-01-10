//go:build integration

// Package integration provides integration tests for GRPM against real Gentoo packages.
//
// These tests validate GRPM's ability to build packages from the real Gentoo tree.
// They require a Gentoo system with /var/db/repos/gentoo available.
//
// Run with: go test -tags=integration ./tests/integration/...
package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/ebuild"
	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
)

// DefaultRepoPath is the standard Gentoo repository location.
const DefaultRepoPath = "/var/db/repos/gentoo"

// DefaultDistDir is the standard distfiles directory.
const DefaultDistDir = "/var/cache/distfiles"

// DefaultTmpDir is the standard build directory.
const DefaultTmpDir = "/var/tmp/portage"

// BuildSystem represents the build system used by a package.
type BuildSystem string

const (
	BuildSystemAutotools BuildSystem = "autotools"
	BuildSystemCMake     BuildSystem = "cmake"
	BuildSystemMeson     BuildSystem = "meson"
)

// PackageSpec defines a package to test.
type PackageSpec struct {
	// Atom is the package atom (e.g., "app-misc/hello")
	Atom string

	// BuildSystem is the expected build system
	BuildSystem BuildSystem

	// Complexity is the expected complexity level
	Complexity string // "simple", "medium", "complex"

	// Description is a human-readable description
	Description string

	// ExpectedPhases lists phases expected to succeed
	ExpectedPhases []ebuild.Phase

	// SkipReason if set, test is skipped with this reason
	SkipReason string

	// RequiredUSE are USE flags required for the test
	RequiredUSE []string
}

// PhaseResult contains single phase execution result.
type PhaseResult struct {
	Name     string
	Success  bool
	Output   string
	Error    error
	Duration time.Duration
}

// BuildResult contains build execution results.
type BuildResult struct {
	// Package is the package atom
	Package string

	// Version is the package version built
	Version string

	// Error is the overall build error (nil if successful)
	Error error

	// Phases contains results for each phase
	Phases map[string]PhaseResult

	// ImageDir is the installation image directory (D)
	ImageDir string

	// Duration is the total build time
	Duration time.Duration

	// FilesInstalled is the count of files in ImageDir
	FilesInstalled int
}

// Success returns true if the build succeeded.
func (r *BuildResult) Success() bool {
	return r.Error == nil
}

// PhaseSucceeded checks if a specific phase succeeded.
func (r *BuildResult) PhaseSucceeded(phase string) bool {
	if pr, ok := r.Phases[phase]; ok {
		return pr.Success
	}
	return false
}

// TestContext holds common test state.
type TestContext struct {
	T        *testing.T
	RepoPath string
	TmpDir   string
	DistDir  string
	Repo     *repo.PortageRepository
}

// NewTestContext creates a new test context.
func NewTestContext(t *testing.T) *TestContext {
	t.Helper()

	repoPath := os.Getenv("GRPM_REPO_PATH")
	if repoPath == "" {
		repoPath = DefaultRepoPath
	}

	distDir := os.Getenv("GRPM_DIST_DIR")
	if distDir == "" {
		distDir = DefaultDistDir
	}

	tmpDir := os.Getenv("GRPM_TMP_DIR")
	if tmpDir == "" {
		tmpDir = DefaultTmpDir
	}

	return &TestContext{
		T:        t,
		RepoPath: repoPath,
		TmpDir:   tmpDir,
		DistDir:  distDir,
	}
}

// Init initializes the test context and opens the repository.
func (tc *TestContext) Init() error {
	r, err := repo.NewPortageRepository(tc.RepoPath)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}
	tc.Repo = r
	return nil
}

// skipIfNoRepo skips the test if the Gentoo repository is not available.
func skipIfNoRepo(t *testing.T) {
	t.Helper()

	repoPath := os.Getenv("GRPM_REPO_PATH")
	if repoPath == "" {
		repoPath = DefaultRepoPath
	}

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		t.Skipf("Gentoo repository not found at %s (set GRPM_REPO_PATH to override)", repoPath)
	}

	// Check for profiles directory to verify it's a valid repo
	profilesDir := filepath.Join(repoPath, "profiles")
	if _, err := os.Stat(profilesDir); os.IsNotExist(err) {
		t.Skipf("Invalid repository at %s (no profiles directory)", repoPath)
	}
}

// skipIfNoDistfiles skips the test if distfiles directory is not accessible.
func skipIfNoDistfiles(t *testing.T) {
	t.Helper()

	distDir := os.Getenv("GRPM_DIST_DIR")
	if distDir == "" {
		distDir = DefaultDistDir
	}

	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		t.Skipf("Distfiles directory not found at %s (set GRPM_DIST_DIR to override)", distDir)
	}
}

// skipIfNoBuildTools skips the test if required build tools are not available.
func skipIfNoBuildTools(t *testing.T, tools ...string) {
	t.Helper()

	for _, tool := range tools {
		if _, err := findExecutable(tool); err != nil {
			t.Skipf("Required tool %q not found in PATH", tool)
		}
	}
}

// findExecutable searches for an executable in PATH.
func findExecutable(name string) (string, error) {
	pathEnv := os.Getenv("PATH")
	paths := filepath.SplitList(pathEnv)

	for _, dir := range paths {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}

	return "", fmt.Errorf("executable %q not found in PATH", name)
}

// buildPackage executes a full ebuild build cycle for the given package.
//
// This function:
//  1. Loads the package from the repository
//  2. Creates an executor with the appropriate options
//  3. Executes all standard build phases
//  4. Returns detailed results for each phase
//
// The build uses a temporary directory that is cleaned up after the test.
func buildPackage(t *testing.T, atom string) *BuildResult {
	t.Helper()

	startTime := time.Now()
	result := &BuildResult{
		Package: atom,
		Phases:  make(map[string]PhaseResult),
	}

	// Parse atom to get category/package
	tc := NewTestContext(t)
	if err := tc.Init(); err != nil {
		result.Error = fmt.Errorf("initializing test context: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Load package from repository
	loadedPkg, err := tc.Repo.LoadPackage(atom)
	if err != nil {
		result.Error = fmt.Errorf("loading package %s: %w", atom, err)
		result.Duration = time.Since(startTime)
		return result
	}
	result.Version = loadedPkg.Version

	// Create temporary build directory for this test
	buildDir := filepath.Join(tc.TmpDir, "grpm-test", strings.ReplaceAll(atom, "/", "_"))
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		result.Error = fmt.Errorf("creating build directory: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Cleanup build directory after test
	t.Cleanup(func() {
		_ = os.RemoveAll(buildDir)
	})

	// Construct ebuild path
	parts := strings.Split(atom, "/")
	if len(parts) != 2 {
		result.Error = fmt.Errorf("invalid atom format: %s", atom)
		result.Duration = time.Since(startTime)
		return result
	}
	category := parts[0]
	pkgName := parts[1]
	ebuildPath := filepath.Join(tc.RepoPath, category, pkgName, fmt.Sprintf("%s-%s.ebuild", pkgName, loadedPkg.Version))

	// Create executor options
	opts := ebuild.ExecutorOptions{
		TmpDir:        buildDir,
		PortDir:       tc.RepoPath,
		DistDir:       tc.DistDir,
		EbuildPath:    ebuildPath,
		EnableSandbox: false, // Disable sandbox for CI
		EnableTests:   false, // Skip tests by default
		KeepWork:      false, // Clean up after build
		DenyNetwork:   false, // Allow network for downloads
	}

	// Create executor
	executor, err := ebuild.NewExecutor(loadedPkg, opts)
	if err != nil {
		result.Error = fmt.Errorf("creating executor: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Parse ebuild to get phase functions
	if err := executor.ParseEbuild(); err != nil {
		result.Error = fmt.Errorf("parsing ebuild: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Execute standard build phases
	phases := ebuild.StandardPhases()
	phaseResults, err := executor.ExecutePhases(phases)
	if err != nil {
		result.Error = fmt.Errorf("executing phases: %w", err)
	}

	// Convert phase results
	for _, pr := range phaseResults {
		result.Phases[string(pr.Phase)] = PhaseResult{
			Name:     string(pr.Phase),
			Success:  pr.Success,
			Output:   pr.Output,
			Error:    pr.Error,
			Duration: time.Duration(pr.Duration) * time.Millisecond,
		}
	}

	// Get image directory and count files
	result.ImageDir = executor.GetImageDirectory()
	if result.Error == nil {
		result.FilesInstalled = countFiles(result.ImageDir)
	}

	result.Duration = time.Since(startTime)
	return result
}

// countFiles recursively counts files in a directory.
func countFiles(dir string) int {
	count := 0
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	return count
}

// assertPhaseSuccess asserts that a phase succeeded.
func assertPhaseSuccess(t *testing.T, result *BuildResult, phase string) {
	t.Helper()

	if !result.PhaseSucceeded(phase) {
		pr, ok := result.Phases[phase]
		if ok {
			t.Errorf("Phase %s failed: %v\nOutput: %s", phase, pr.Error, pr.Output)
		} else {
			t.Errorf("Phase %s was not executed", phase)
		}
	}
}

// assertFilesInstalled asserts that files were installed to the image directory.
func assertFilesInstalled(t *testing.T, result *BuildResult, minFiles int) {
	t.Helper()

	if result.FilesInstalled < minFiles {
		t.Errorf("Expected at least %d files installed, got %d", minFiles, result.FilesInstalled)
	}
}

// TestResult represents the result of a test run.
type TestResult struct {
	Package   string
	Version   string
	Success   bool
	Error     error
	Duration  time.Duration
	FileCount int
}

// TestSummary summarizes test results.
type TestSummary struct {
	TotalTests int
	Passed     int
	Failed     int
	Skipped    int
	Results    []TestResult
}

// PassRate returns the pass rate as a percentage.
func (s *TestSummary) PassRate() float64 {
	if s.TotalTests-s.Skipped == 0 {
		return 0
	}
	return float64(s.Passed) / float64(s.TotalTests-s.Skipped) * 100
}

// LogSummary logs the test summary.
func (s *TestSummary) LogSummary(t *testing.T) {
	t.Helper()
	t.Logf("=== Test Summary ===")
	t.Logf("Total: %d, Passed: %d, Failed: %d, Skipped: %d",
		s.TotalTests, s.Passed, s.Failed, s.Skipped)
	t.Logf("Pass Rate: %.1f%%", s.PassRate())
}

// loadPackage loads a package from the test repository.
func loadPackage(t *testing.T, atom string) *pkg.Package {
	t.Helper()

	tc := NewTestContext(t)
	if err := tc.Init(); err != nil {
		t.Fatalf("Failed to initialize test context: %v", err)
	}

	loadedPkg, err := tc.Repo.LoadPackage(atom)
	if err != nil {
		t.Fatalf("Failed to load package %s: %v", atom, err)
	}

	return loadedPkg
}

// packageExists checks if a package exists in the repository.
func packageExists(t *testing.T, atom string) bool {
	t.Helper()

	tc := NewTestContext(t)
	if err := tc.Init(); err != nil {
		return false
	}

	_, err := tc.Repo.LoadPackage(atom)
	return err == nil
}

// getPackageVersion returns the latest version of a package.
func getPackageVersion(t *testing.T, atom string) string {
	t.Helper()

	pkg := loadPackage(t, atom)
	return pkg.Version
}

// ebuildHasFunction checks if an ebuild defines a specific function.
func ebuildHasFunction(t *testing.T, atom, function string) bool {
	t.Helper()

	tc := NewTestContext(t)
	if err := tc.Init(); err != nil {
		return false
	}

	loadedPkg, err := tc.Repo.LoadPackage(atom)
	if err != nil {
		return false
	}

	parts := strings.Split(atom, "/")
	if len(parts) != 2 {
		return false
	}
	category := parts[0]
	pkgName := parts[1]
	ebuildPath := filepath.Join(tc.RepoPath, category, pkgName,
		fmt.Sprintf("%s-%s.ebuild", pkgName, loadedPkg.Version))

	parsed, err := ebuild.ParseEbuildScript(ebuildPath)
	if err != nil {
		return false
	}

	return parsed.HasFunction(function)
}

// getEbuildInherits returns the eclasses inherited by an ebuild.
func getEbuildInherits(t *testing.T, atom string) []string {
	t.Helper()

	tc := NewTestContext(t)
	if err := tc.Init(); err != nil {
		return nil
	}

	loadedPkg, err := tc.Repo.LoadPackage(atom)
	if err != nil {
		return nil
	}

	parts := strings.Split(atom, "/")
	if len(parts) != 2 {
		return nil
	}
	category := parts[0]
	pkgName := parts[1]
	ebuildPath := filepath.Join(tc.RepoPath, category, pkgName,
		fmt.Sprintf("%s-%s.ebuild", pkgName, loadedPkg.Version))

	parsed, err := ebuild.ParseEbuildScript(ebuildPath)
	if err != nil {
		return nil
	}

	return parsed.InheritedEclasses
}

// ParseResult contains ebuild parsing validation results.
type ParseResult struct {
	// Success indicates if parsing succeeded
	Success bool

	// Version is the package version
	Version string

	// EAPI is the ebuild API version
	EAPI string

	// Inherits lists inherited eclasses
	Inherits []string

	// Functions lists discovered functions
	Functions []string

	// Error contains parsing error if any
	Error error
}

// validatePackageParsing validates that a package can be parsed correctly.
//
// This is the core validation for v0.4.0 - we validate parsing and metadata
// extraction without requiring actual source files or full eclass support.
func validatePackageParsing(t *testing.T, atom string) *ParseResult {
	t.Helper()

	result := &ParseResult{}

	tc := NewTestContext(t)
	if err := tc.Init(); err != nil {
		result.Error = fmt.Errorf("initializing context: %w", err)
		return result
	}

	// Load package from repository
	loadedPkg, err := tc.Repo.LoadPackage(atom)
	if err != nil {
		result.Error = fmt.Errorf("loading package: %w", err)
		return result
	}
	result.Version = loadedPkg.Version

	// Parse ebuild script
	parts := strings.Split(atom, "/")
	if len(parts) != 2 {
		result.Error = fmt.Errorf("invalid atom format: %s", atom)
		return result
	}
	category := parts[0]
	pkgName := parts[1]
	ebuildPath := filepath.Join(tc.RepoPath, category, pkgName,
		fmt.Sprintf("%s-%s.ebuild", pkgName, loadedPkg.Version))

	parsed, err := ebuild.ParseEbuildScript(ebuildPath)
	if err != nil {
		result.Error = fmt.Errorf("parsing ebuild: %w", err)
		return result
	}

	result.EAPI = parsed.EAPI
	result.Inherits = parsed.InheritedEclasses

	// Convert map keys to slice
	result.Functions = make([]string, 0, len(parsed.DefinedFunctions))
	for fn := range parsed.DefinedFunctions {
		result.Functions = append(result.Functions, fn)
	}

	result.Success = true

	return result
}
