package daemon

import (
	"testing"

	"github.com/grpmsoft/grpm/internal/application"
	"github.com/grpmsoft/grpm/internal/repo"
)

// TestConflictDetector_SamePackage tests same package conflict detection
func TestConflictDetector_SamePackage(t *testing.T) {
	mockRepo := repo.NewMockRepository()
	pkgService := application.NewPackageService(mockRepo)
	detector := NewConflictDetector(pkgService)

	// Create first job (running)
	job1 := NewJob(JobTypeInstall, "dev-lang/go", JobPriorityNormal)
	job1.SetStatus(JobStatusRunning)

	// Create second job (same package)
	job2 := NewJob(JobTypeInstall, "dev-lang/go", JobPriorityNormal)

	conflict, err := detector.DetectConflicts(job2, []*Job{job1})
	if err != nil {
		t.Fatalf("DetectConflicts failed: %v", err)
	}

	if conflict == nil {
		t.Fatal("Expected conflict, got nil")
	}

	if conflict.Type != ConflictSamePackage {
		t.Errorf("Expected ConflictSamePackage, got %s", conflict.Type)
	}

	if conflict.ConflictingPkg != "dev-lang/go" {
		t.Errorf("Expected conflicting package 'dev-lang/go', got '%s'", conflict.ConflictingPkg)
	}
}

// TestConflictDetector_SamePackageWithVersion tests version handling
func TestConflictDetector_SamePackageWithVersion(t *testing.T) {
	mockRepo := repo.NewMockRepository()
	pkgService := application.NewPackageService(mockRepo)
	detector := NewConflictDetector(pkgService)

	// Create first job with version
	job1 := NewJob(JobTypeInstall, "dev-lang/go-1.22.0", JobPriorityNormal)
	job1.SetStatus(JobStatusRunning)

	// Create second job with different version
	job2 := NewJob(JobTypeInstall, "dev-lang/go-1.23.0", JobPriorityNormal)

	conflict, err := detector.DetectConflicts(job2, []*Job{job1})
	if err != nil {
		t.Fatalf("DetectConflicts failed: %v", err)
	}

	if conflict == nil {
		t.Fatal("Expected conflict (same package, different versions), got nil")
	}

	if conflict.Type != ConflictSamePackage {
		t.Errorf("Expected ConflictSamePackage, got %s", conflict.Type)
	}
}

// TestConflictDetector_DifferentPackagesNoConflict tests no conflict
func TestConflictDetector_DifferentPackagesNoConflict(t *testing.T) {
	mockRepo := repo.NewMockRepository()
	pkgService := application.NewPackageService(mockRepo)
	detector := NewConflictDetector(pkgService)

	// Create first job
	job1 := NewJob(JobTypeInstall, "dev-lang/go", JobPriorityNormal)
	job1.SetStatus(JobStatusRunning)

	// Create second job (different package)
	job2 := NewJob(JobTypeInstall, "dev-lang/python", JobPriorityNormal)

	conflict, err := detector.DetectConflicts(job2, []*Job{job1})
	if err != nil {
		t.Fatalf("DetectConflicts failed: %v", err)
	}

	if conflict != nil {
		t.Errorf("Expected no conflict, got: %s", conflict.Reason)
	}
}

// TestConflictDetector_CompletedJobNoConflict tests completed jobs don't conflict
func TestConflictDetector_CompletedJobNoConflict(t *testing.T) {
	mockRepo := repo.NewMockRepository()
	pkgService := application.NewPackageService(mockRepo)
	detector := NewConflictDetector(pkgService)

	// Create completed job (same package)
	job1 := NewJob(JobTypeInstall, "dev-lang/go", JobPriorityNormal)
	job1.SetStatus(JobStatusCompleted)

	// Create new job (same package)
	job2 := NewJob(JobTypeInstall, "dev-lang/go", JobPriorityNormal)

	conflict, err := detector.DetectConflicts(job2, []*Job{job1})
	if err != nil {
		t.Fatalf("DetectConflicts failed: %v", err)
	}

	if conflict != nil {
		t.Errorf("Expected no conflict with completed job, got: %s", conflict.Reason)
	}
}

// TestConflictDetector_MultipleActiveJobs tests multiple running jobs
func TestConflictDetector_MultipleActiveJobs(t *testing.T) {
	mockRepo := repo.NewMockRepository()
	pkgService := application.NewPackageService(mockRepo)
	detector := NewConflictDetector(pkgService)

	// Create multiple active jobs
	job1 := NewJob(JobTypeInstall, "dev-lang/python", JobPriorityNormal)
	job1.SetStatus(JobStatusRunning)

	job2 := NewJob(JobTypeInstall, "dev-lang/ruby", JobPriorityNormal)
	job2.SetStatus(JobStatusPending)

	job3 := NewJob(JobTypeInstall, "dev-lang/rust", JobPriorityNormal)
	job3.SetStatus(JobStatusCompleted)

	// Try to install same package as running job
	newJob := NewJob(JobTypeInstall, "dev-lang/python-3.11", JobPriorityNormal)

	conflict, err := detector.DetectConflicts(newJob, []*Job{job1, job2, job3})
	if err != nil {
		t.Fatalf("DetectConflicts failed: %v", err)
	}

	if conflict == nil {
		t.Fatal("Expected conflict with running python job")
	}

	if conflict.Job2.ID != job1.ID {
		t.Errorf("Expected conflict with job1, got conflict with job %s", conflict.Job2.ID)
	}
}

// TestExtractPackageName tests package name extraction
func TestExtractPackageName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"dev-lang/go-1.22.0", "dev-lang/go"},
		{"dev-lang/go", "dev-lang/go"},
		{"sys-libs/glibc-2.38", "sys-libs/glibc"},
		{"app-editors/vim-9.0.1234", "app-editors/vim"},
		{"dev-python/pytest-7.4.0-r1", "dev-python/pytest"},
		{"invalid-format", "invalid-format"},
		{"no-version-here", "no-version-here"},
	}

	for _, tt := range tests {
		result := extractPackageName(tt.input)
		if result != tt.expected {
			t.Errorf("extractPackageName(%s) = %s, expected %s",
				tt.input, result, tt.expected)
		}
	}
}

// TestLooksLikeVersion tests version detection
func TestLooksLikeVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"1.22.0", true},
		{"2.38", true},
		{"9.0.1234", true},
		{"7.4.0-r1", true},
		{"3.11.2_alpha", true},
		{"go", false},
		{"python", false},
		{"", false},
		{"r1", false},
		{"_beta", false},
	}

	for _, tt := range tests {
		result := looksLikeVersion(tt.input)
		if result != tt.expected {
			t.Errorf("looksLikeVersion(%s) = %v, expected %v",
				tt.input, result, tt.expected)
		}
	}
}

// TestIsSamePackage tests package comparison
func TestIsSamePackage(t *testing.T) {
	tests := []struct {
		pkg1     string
		pkg2     string
		expected bool
	}{
		{"dev-lang/go", "dev-lang/go", true},
		{"dev-lang/go-1.22", "dev-lang/go-1.23", true},
		{"dev-lang/go", "dev-lang/go-1.22", true},
		{"dev-lang/python", "dev-lang/go", false},
		{"sys-libs/glibc-2.38", "sys-libs/glibc-2.39", true},
		{"app-editors/vim", "app-editors/neovim", false},
	}

	for _, tt := range tests {
		result := isSamePackage(tt.pkg1, tt.pkg2)
		if result != tt.expected {
			t.Errorf("isSamePackage(%s, %s) = %v, expected %v",
				tt.pkg1, tt.pkg2, result, tt.expected)
		}
	}
}

// TestFormatConflictError tests error formatting
func TestFormatConflictError(t *testing.T) {
	job1 := NewJob(JobTypeInstall, "dev-lang/go", JobPriorityNormal)
	job2 := NewJob(JobTypeInstall, "dev-lang/python", JobPriorityNormal)

	conflict := &PackageConflict{
		Type:           ConflictSamePackage,
		Job1:           job1,
		Job2:           job2,
		ConflictingPkg: "dev-lang/go",
		Reason:         "Test reason",
	}

	errMsg := FormatConflictError(conflict)

	// Check that error message contains key information
	if errMsg == "" {
		t.Error("Expected non-empty error message")
	}

	// Should contain job types
	if !contains(errMsg, string(JobTypeInstall)) {
		t.Error("Error message should contain job type")
	}

	// Should contain conflict type
	if !contains(errMsg, ConflictSamePackage.String()) {
		t.Error("Error message should contain conflict type")
	}

	// Should contain reason
	if !contains(errMsg, "Test reason") {
		t.Error("Error message should contain reason")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
