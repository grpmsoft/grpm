package daemon

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// JobStatus represents the status of a job
type JobStatus int

const (
	JobStatusPending JobStatus = iota
	JobStatusRunning
	JobStatusCompleted
	JobStatusFailed
	JobStatusCancelled
)

func (s JobStatus) String() string {
	switch s {
	case JobStatusPending:
		return "pending"
	case JobStatusRunning:
		return "running"
	case JobStatusCompleted:
		return "completed"
	case JobStatusFailed:
		return "failed"
	case JobStatusCancelled:
		return "canceled"
	default:
		return "unknown"
	}
}

// JobPriority defines job execution priority
type JobPriority int

const (
	JobPriorityLow JobPriority = iota
	JobPriorityNormal
	JobPriorityHigh
)

// JobType defines the type of operation
type JobType string

const (
	JobTypeInstall JobType = "install"
	JobTypeRemove  JobType = "remove"
	JobTypeUpdate  JobType = "update"
	JobTypeSync    JobType = "sync"
)

// Job represents a package operation to be executed
type Job struct {
	// Immutable fields (safe to read without lock)
	ID          string
	Type        JobType
	PackageName string
	Priority    JobPriority
	CreatedAt   time.Time

	// Mutable fields (protected by mu)
	mu          sync.RWMutex
	status      JobStatus
	progress    int    // 0-100
	error       string // Error message if failed
	startedAt   *time.Time
	completedAt *time.Time

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Progress callback
	progressCallback func(progress int, message string)

	// Execution function
	execute func(ctx context.Context, job *Job) error
}

// NewJob creates a new job
func NewJob(jobType JobType, packageName string, priority JobPriority) *Job {
	ctx, cancel := context.WithCancel(context.Background())
	return &Job{
		ID:          uuid.New().String(),
		Type:        jobType,
		PackageName: packageName,
		Priority:    priority,
		status:      JobStatusPending,
		CreatedAt:   time.Now(),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// GetStatus returns job status (thread-safe)
func (j *Job) GetStatus() JobStatus {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.status
}

// SetStatus sets job status (thread-safe)
func (j *Job) SetStatus(status JobStatus) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status = status
}

// GetProgress returns job progress (thread-safe)
func (j *Job) GetProgress() int {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.progress
}

// GetError returns job error (thread-safe)
func (j *Job) GetError() string {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.error
}

// GetStartedAt returns job start time (thread-safe)
func (j *Job) GetStartedAt() *time.Time {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if j.startedAt == nil {
		return nil
	}
	t := *j.startedAt
	return &t
}

// GetCompletedAt returns job completion time (thread-safe)
func (j *Job) GetCompletedAt() *time.Time {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if j.completedAt == nil {
		return nil
	}
	t := *j.completedAt
	return &t
}

// Cancel cancels the job (thread-safe)
func (j *Job) Cancel() {
	j.cancel()
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status = JobStatusCancelled
}

// UpdateProgress updates job progress (thread-safe)
func (j *Job) UpdateProgress(progress int, message string) {
	j.mu.Lock()
	j.progress = progress
	callback := j.progressCallback
	j.mu.Unlock()

	if callback != nil {
		callback(progress, message)
	}
}

// JobQueue manages concurrent job execution with worker pool
type JobQueue struct {
	// Configuration
	maxWorkers int
	queueSize  int

	// Job storage
	jobs   map[string]*Job
	jobsMu sync.RWMutex

	// Queue channels
	queue chan *Job
	done  chan struct{}

	// Worker tracking
	activeWorkers int
	workersMu     sync.RWMutex
	wg            sync.WaitGroup

	// Conflict detection
	conflictDetector *ConflictDetector

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

// NewJobQueue creates a new job queue with conflict detection
func NewJobQueue(maxWorkers, queueSize int, detector *ConflictDetector) *JobQueue {
	ctx, cancel := context.WithCancel(context.Background())
	return &JobQueue{
		maxWorkers:       maxWorkers,
		queueSize:        queueSize,
		jobs:             make(map[string]*Job),
		queue:            make(chan *Job, queueSize),
		done:             make(chan struct{}),
		conflictDetector: detector,
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Start starts the job queue workers
func (jq *JobQueue) Start() {
	// Start worker pool
	for i := 0; i < jq.maxWorkers; i++ {
		jq.wg.Add(1)
		go jq.worker(i)
	}
}

// worker is the worker goroutine
func (jq *JobQueue) worker(id int) {
	defer jq.wg.Done()

	for {
		select {
		case <-jq.ctx.Done():
			return

		case job := <-jq.queue:
			jq.incrementActiveWorkers()
			jq.executeJob(job)
			jq.decrementActiveWorkers()
		}
	}
}

// executeJob executes a single job
func (jq *JobQueue) executeJob(job *Job) {
	// Check if job was canceled before execution started
	if job.GetStatus() == JobStatusCancelled {
		return
	}

	// Update job status (thread-safe)
	now := time.Now()
	job.mu.Lock()
	job.startedAt = &now
	job.status = JobStatusRunning
	job.mu.Unlock()
	jq.updateJob(job)

	// Execute job
	err := job.execute(job.ctx, job)

	// Update completion status (thread-safe)
	completedAt := time.Now()
	job.mu.Lock()

	job.completedAt = &completedAt

	// Preserve canceled status if job was canceled during execution
	if job.status == JobStatusCancelled {
		job.mu.Unlock()
		jq.updateJob(job)
		return
	}

	if err != nil {
		// Check if error is due to context cancellation
		if job.ctx.Err() != nil {
			// Context was canceled
			job.status = JobStatusCancelled
		} else {
			job.status = JobStatusFailed
		}
		job.error = err.Error()
	} else {
		job.status = JobStatusCompleted
		job.progress = 100
	}

	job.mu.Unlock()
	jq.updateJob(job)
}

// Submit submits a new job to the queue after checking for conflicts
func (jq *JobQueue) Submit(job *Job) error {
	// Check for conflicts with active jobs (if detector is configured)
	if jq.conflictDetector != nil {
		jq.jobsMu.RLock()
		allJobs := make([]*Job, 0, len(jq.jobs))
		for _, j := range jq.jobs {
			allJobs = append(allJobs, j)
		}
		jq.jobsMu.RUnlock()

		conflict, err := jq.conflictDetector.DetectConflicts(job, allJobs)
		if err != nil {
			return fmt.Errorf("conflict detection failed: %w", err)
		}
		if conflict != nil {
			return fmt.Errorf("package conflict: %s", FormatConflictError(conflict))
		}
	}

	// Store job
	jq.jobsMu.Lock()
	jq.jobs[job.ID] = job
	jq.jobsMu.Unlock()

	// Try to enqueue (non-blocking)
	select {
	case jq.queue <- job:
		return nil
	default:
		return fmt.Errorf("job queue is full")
	}
}

// GetJob retrieves a job by ID
func (jq *JobQueue) GetJob(id string) (*Job, error) {
	jq.jobsMu.RLock()
	defer jq.jobsMu.RUnlock()

	job, exists := jq.jobs[id]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", id)
	}

	return job, nil
}

// ListJobs returns all jobs
func (jq *JobQueue) ListJobs() []*Job {
	jq.jobsMu.RLock()
	defer jq.jobsMu.RUnlock()

	jobs := make([]*Job, 0, len(jq.jobs))
	for _, job := range jq.jobs {
		jobs = append(jobs, job)
	}

	return jobs
}

// CancelJob cancels a job by ID
func (jq *JobQueue) CancelJob(id string) error {
	job, err := jq.GetJob(id)
	if err != nil {
		return err
	}

	job.Cancel()
	jq.updateJob(job)
	return nil
}

// Stop stops the job queue gracefully
func (jq *JobQueue) Stop(ctx context.Context) error {
	// Signal workers to stop
	jq.cancel()

	// Wait for workers to finish (with timeout)
	done := make(chan struct{})
	go func() {
		jq.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		close(jq.done)
		return nil
	case <-ctx.Done():
		return fmt.Errorf("job queue shutdown timeout")
	}
}

// GetStats returns queue statistics
func (jq *JobQueue) GetStats() map[string]interface{} {
	jq.jobsMu.RLock()
	defer jq.jobsMu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_jobs"] = len(jq.jobs)
	stats["queue_length"] = len(jq.queue)
	stats["max_workers"] = jq.maxWorkers
	stats["active_workers"] = jq.getActiveWorkers()

	// Count by status
	statusCounts := make(map[string]int)
	for _, job := range jq.jobs {
		statusCounts[job.GetStatus().String()]++
	}
	stats["jobs_by_status"] = statusCounts

	return stats
}

// Helper methods
func (jq *JobQueue) updateJob(job *Job) {
	jq.jobsMu.Lock()
	defer jq.jobsMu.Unlock()
	jq.jobs[job.ID] = job
}

func (jq *JobQueue) incrementActiveWorkers() {
	jq.workersMu.Lock()
	defer jq.workersMu.Unlock()
	jq.activeWorkers++
}

func (jq *JobQueue) decrementActiveWorkers() {
	jq.workersMu.Lock()
	defer jq.workersMu.Unlock()
	jq.activeWorkers--
}

func (jq *JobQueue) getActiveWorkers() int {
	jq.workersMu.RLock()
	defer jq.workersMu.RUnlock()
	return jq.activeWorkers
}
