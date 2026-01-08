package daemon

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/application"
	"github.com/grpmsoft/grpm/internal/repo"
)

// TestJob_NewJob tests job creation
func TestJob_NewJob(t *testing.T) {
	job := NewJob(JobTypeInstall, "test-package", JobPriorityNormal)

	if job.ID == "" {
		t.Error("Expected non-empty job ID")
	}

	if job.Type != JobTypeInstall {
		t.Errorf("Expected type %s, got %s", JobTypeInstall, job.Type)
	}

	if job.PackageName != "test-package" {
		t.Errorf("Expected package 'test-package', got '%s'", job.PackageName)
	}

	if job.GetStatus() != JobStatusPending {
		t.Errorf("Expected status %s, got %s", JobStatusPending, job.GetStatus())
	}

	if job.Priority != JobPriorityNormal {
		t.Errorf("Expected priority %d, got %d", JobPriorityNormal, job.Priority)
	}
}

// TestJob_Cancel tests job cancellation
func TestJob_Cancel(t *testing.T) {
	job := NewJob(JobTypeInstall, "test-package", JobPriorityNormal)
	job.Cancel()

	if job.GetStatus() != JobStatusCancelled {
		t.Errorf("Expected status %s, got %s", JobStatusCancelled, job.GetStatus())
	}

	// Verify context is canceled
	select {
	case <-job.ctx.Done():
		// OK
	default:
		t.Error("Expected job context to be canceled")
	}
}

// TestJob_UpdateProgress tests progress updates
func TestJob_UpdateProgress(t *testing.T) {
	job := NewJob(JobTypeInstall, "test-package", JobPriorityNormal)

	progressUpdates := make([]int, 0)
	job.progressCallback = func(progress int, message string) {
		progressUpdates = append(progressUpdates, progress)
	}

	job.UpdateProgress(25, "Downloading")
	job.UpdateProgress(50, "Compiling")
	job.UpdateProgress(75, "Installing")

	if len(progressUpdates) != 3 {
		t.Errorf("Expected 3 progress updates, got %d", len(progressUpdates))
	}

	expected := []int{25, 50, 75}
	for i, val := range expected {
		if progressUpdates[i] != val {
			t.Errorf("Progress update %d: expected %d, got %d", i, val, progressUpdates[i])
		}
	}
}

// TestJobQueue_NewJobQueue tests queue creation
func TestJobQueue_NewJobQueue(t *testing.T) {
	jq := NewJobQueue(4, 100, nil) // nil detector for simple test

	if jq.maxWorkers != 4 {
		t.Errorf("Expected 4 workers, got %d", jq.maxWorkers)
	}

	if jq.queueSize != 100 {
		t.Errorf("Expected queue size 100, got %d", jq.queueSize)
	}

	if jq.jobs == nil {
		t.Error("Expected jobs map to be initialized")
	}

	if jq.queue == nil {
		t.Error("Expected queue channel to be initialized")
	}
}

// TestJobQueue_Submit tests job submission
func TestJobQueue_Submit(t *testing.T) {
	jq := NewJobQueue(2, 10, nil)

	job := NewJob(JobTypeInstall, "test-package", JobPriorityNormal)
	job.execute = func(ctx context.Context, j *Job) error {
		return nil
	}

	err := jq.Submit(job)
	if err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Verify job is stored
	retrieved, err := jq.GetJob(job.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve job: %v", err)
	}

	if retrieved.ID != job.ID {
		t.Errorf("Expected job ID %s, got %s", job.ID, retrieved.ID)
	}
}

// TestJobQueue_ExecuteJob tests job execution
func TestJobQueue_ExecuteJob(t *testing.T) {
	jq := NewJobQueue(2, 10, nil)
	jq.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = jq.Stop(ctx)
	}()

	var executed atomic.Bool
	job := NewJob(JobTypeInstall, "test-package", JobPriorityNormal)
	job.execute = func(ctx context.Context, j *Job) error {
		executed.Store(true)
		j.UpdateProgress(50, "Working...")
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	err := jq.Submit(job)
	if err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Wait for job to complete
	time.Sleep(200 * time.Millisecond)

	if !executed.Load() {
		t.Error("Job was not executed")
	}

	// Verify job status
	retrieved, _ := jq.GetJob(job.ID)
	if retrieved.GetStatus() != JobStatusCompleted {
		t.Errorf("Expected status %s, got %s", JobStatusCompleted, retrieved.GetStatus())
	}

	if retrieved.GetProgress() != 100 {
		t.Errorf("Expected progress 100, got %d", retrieved.GetProgress())
	}

	if retrieved.GetStartedAt() == nil {
		t.Error("Expected StartedAt to be set")
	}

	if retrieved.GetCompletedAt() == nil {
		t.Error("Expected CompletedAt to be set")
	}
}

// TestJobQueue_ExecuteJobFailure tests job execution with error
func TestJobQueue_ExecuteJobFailure(t *testing.T) {
	jq := NewJobQueue(2, 10, nil)
	jq.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = jq.Stop(ctx)
	}()

	expectedErr := errors.New("installation failed")
	job := NewJob(JobTypeInstall, "test-package", JobPriorityNormal)
	job.execute = func(ctx context.Context, j *Job) error {
		return expectedErr
	}

	err := jq.Submit(job)
	if err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Wait for job to complete
	time.Sleep(200 * time.Millisecond)

	// Verify job status
	retrieved, _ := jq.GetJob(job.ID)
	if retrieved.GetStatus() != JobStatusFailed {
		t.Errorf("Expected status %s, got %s", JobStatusFailed, retrieved.GetStatus())
	}

	if retrieved.GetError() != expectedErr.Error() {
		t.Errorf("Expected error '%s', got '%s'", expectedErr.Error(), retrieved.GetError())
	}
}

// TestJobQueue_MultipleJobs tests concurrent job execution
func TestJobQueue_MultipleJobs(t *testing.T) {
	jq := NewJobQueue(2, 10, nil)
	jq.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = jq.Stop(ctx)
	}()

	numJobs := 5
	executedCount := 0
	var mu sync.Mutex

	for i := 0; i < numJobs; i++ {
		job := NewJob(JobTypeInstall, fmt.Sprintf("package-%d", i), JobPriorityNormal)
		job.execute = func(ctx context.Context, j *Job) error {
			mu.Lock()
			executedCount++
			mu.Unlock()
			time.Sleep(50 * time.Millisecond)
			return nil
		}

		err := jq.Submit(job)
		if err != nil {
			t.Fatalf("Failed to submit job %d: %v", i, err)
		}
	}

	// Wait for all jobs to complete
	time.Sleep(1 * time.Second)

	mu.Lock()
	defer mu.Unlock()
	if executedCount != numJobs {
		t.Errorf("Expected %d jobs executed, got %d", numJobs, executedCount)
	}

	// Verify all jobs are completed
	jobs := jq.ListJobs()
	for _, job := range jobs {
		if job.GetStatus() != JobStatusCompleted {
			t.Errorf("Job %s: expected status %s, got %s", job.ID, JobStatusCompleted, job.GetStatus())
		}
	}
}

// TestJobQueue_CancelJob tests job cancellation
func TestJobQueue_CancelJob(t *testing.T) {
	jq := NewJobQueue(1, 10, nil)
	jq.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = jq.Stop(ctx)
	}()

	job := NewJob(JobTypeInstall, "test-package", JobPriorityNormal)
	job.execute = func(ctx context.Context, j *Job) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			return nil
		}
	}

	err := jq.Submit(job)
	if err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Cancel job immediately
	time.Sleep(10 * time.Millisecond)
	err = jq.CancelJob(job.ID)
	if err != nil {
		t.Fatalf("Failed to cancel job: %v", err)
	}

	// Verify job is canceled
	retrieved, _ := jq.GetJob(job.ID)
	if retrieved.GetStatus() != JobStatusCancelled {
		t.Errorf("Expected status %s, got %s", JobStatusCancelled, retrieved.GetStatus())
	}
}

// TestJobQueue_GetStats tests statistics retrieval
func TestJobQueue_GetStats(t *testing.T) {
	jq := NewJobQueue(4, 100, nil)
	jq.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = jq.Stop(ctx)
	}()

	// Submit some jobs
	for i := 0; i < 3; i++ {
		job := NewJob(JobTypeInstall, fmt.Sprintf("package-%d", i), JobPriorityNormal)
		job.execute = func(ctx context.Context, j *Job) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		}
		_ = jq.Submit(job)
	}

	stats := jq.GetStats()

	if stats["total_jobs"].(int) != 3 {
		t.Errorf("Expected 3 total jobs, got %d", stats["total_jobs"])
	}

	if stats["max_workers"].(int) != 4 {
		t.Errorf("Expected 4 max workers, got %d", stats["max_workers"])
	}
}

// TestJobQueue_GracefulShutdown tests graceful shutdown
func TestJobQueue_GracefulShutdown(t *testing.T) {
	jq := NewJobQueue(2, 10, nil)
	jq.Start()

	// Submit jobs
	for i := 0; i < 3; i++ {
		job := NewJob(JobTypeInstall, fmt.Sprintf("package-%d", i), JobPriorityNormal)
		job.execute = func(ctx context.Context, j *Job) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		}
		_ = jq.Submit(job)
	}

	// Stop with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := jq.Stop(ctx)
	if err != nil {
		t.Fatalf("Failed to stop job queue: %v", err)
	}

	// Verify queue is stopped
	select {
	case <-jq.done:
		// OK
	default:
		t.Error("Expected queue to be stopped")
	}
}

// TestJobQueue_ConflictDetection tests conflict detection in job submission
func TestJobQueue_ConflictDetection(t *testing.T) {
	// Create mock repository and service for conflict detection
	mockRepo := repo.NewMockRepository()
	pkgService := application.NewPackageService(mockRepo)
	detector := NewConflictDetector(pkgService)

	// Create job queue with conflict detection
	jq := NewJobQueue(2, 10, detector)
	jq.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = jq.Stop(ctx)
	}()

	// Submit first job (install go)
	job1 := NewJob(JobTypeInstall, "dev-lang/go", JobPriorityNormal)
	job1.execute = func(ctx context.Context, j *Job) error {
		time.Sleep(500 * time.Millisecond) // Keep running
		return nil
	}

	err := jq.Submit(job1)
	if err != nil {
		t.Fatalf("Failed to submit first job: %v", err)
	}

	// Give job1 time to start
	time.Sleep(50 * time.Millisecond)

	// Try to submit second job (same package) - should be rejected
	job2 := NewJob(JobTypeInstall, "dev-lang/go-1.22", JobPriorityNormal)
	job2.execute = func(ctx context.Context, j *Job) error {
		return nil
	}

	err = jq.Submit(job2)
	if err == nil {
		t.Error("Expected conflict error, but job was accepted")
	}

	// Check error message contains conflict information
	if !contains(err.Error(), "conflict") {
		t.Errorf("Expected error to mention 'conflict', got: %v", err)
	}

	// Wait for first job to complete
	time.Sleep(600 * time.Millisecond)

	// Now submit should succeed (no active jobs)
	job3 := NewJob(JobTypeInstall, "dev-lang/go-1.23", JobPriorityNormal)
	job3.execute = func(ctx context.Context, j *Job) error {
		return nil
	}

	err = jq.Submit(job3)
	if err != nil {
		t.Errorf("Expected job3 to be accepted after job1 completed, got error: %v", err)
	}
}

// TestJobQueue_DifferentPackagesNoConflict tests parallel execution of different packages
func TestJobQueue_DifferentPackagesNoConflict(t *testing.T) {
	// Create mock repository and service for conflict detection
	mockRepo := repo.NewMockRepository()
	pkgService := application.NewPackageService(mockRepo)
	detector := NewConflictDetector(pkgService)

	// Create job queue with conflict detection
	jq := NewJobQueue(4, 10, detector)
	jq.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = jq.Stop(ctx)
	}()

	// Submit multiple jobs for different packages (should not conflict)
	packages := []string{"dev-lang/go", "dev-lang/python", "dev-lang/ruby", "dev-lang/rust"}
	var executedMu sync.Mutex
	executed := make(map[string]bool)

	for _, pkg := range packages {
		pkgCopy := pkg // Capture for closure
		job := NewJob(JobTypeInstall, pkgCopy, JobPriorityNormal)
		job.execute = func(ctx context.Context, j *Job) error {
			executedMu.Lock()
			executed[pkgCopy] = true
			executedMu.Unlock()
			time.Sleep(100 * time.Millisecond)
			return nil
		}

		err := jq.Submit(job)
		if err != nil {
			t.Errorf("Job for %s should not conflict with others, got error: %v", pkg, err)
		}
	}

	// Wait for all jobs to complete
	time.Sleep(500 * time.Millisecond)

	// Verify all jobs executed
	executedMu.Lock()
	defer executedMu.Unlock()
	if len(executed) != len(packages) {
		t.Errorf("Expected %d jobs to execute, but only %d did", len(packages), len(executed))
	}
}
