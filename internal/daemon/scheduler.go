// Package daemon provides the GRPM daemon service and parallel build scheduler.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// BuildStatus represents the status of a build task.
type BuildStatus int

const (
	// BuildStatusPending indicates the build is waiting to start.
	BuildStatusPending BuildStatus = iota
	// BuildStatusWaiting indicates the build is waiting for dependencies.
	BuildStatusWaiting
	// BuildStatusRunning indicates the build is currently executing.
	BuildStatusRunning
	// BuildStatusCompleted indicates the build finished successfully.
	BuildStatusCompleted
	// BuildStatusFailed indicates the build failed.
	BuildStatusFailed
	// BuildStatusCanceled indicates the build was canceled.
	BuildStatusCanceled
)

// String returns a human-readable representation of the build status.
func (s BuildStatus) String() string {
	switch s {
	case BuildStatusPending:
		return "pending"
	case BuildStatusWaiting:
		return "waiting"
	case BuildStatusRunning:
		return "running"
	case BuildStatusCompleted:
		return "completed"
	case BuildStatusFailed:
		return "failed"
	case BuildStatusCanceled:
		return "canceled"
	default:
		return "unknown"
	}
}

// FailureMode defines how the scheduler handles build failures.
type FailureMode int

const (
	// FailureModeStop stops all builds when any build fails.
	FailureModeStop FailureMode = iota
	// FailureModeContinue continues building non-dependent packages on failure.
	FailureModeContinue
)

// BuildTask represents a single package build task.
type BuildTask struct {
	// ID is a unique identifier for the task.
	ID string
	// Package is the package to build.
	Package *pkg.Package
	// Dependencies is a list of task IDs this task depends on.
	Dependencies []string
	// Dependents is a list of task IDs that depend on this task.
	Dependents []string

	// status is the current build status (protected by mu).
	status BuildStatus
	// error is the error message if build failed (protected by mu).
	error string
	// startTime is when the build started (protected by mu).
	startTime time.Time
	// endTime is when the build completed (protected by mu).
	endTime time.Time
	// output is the build output/logs (protected by mu).
	output string
	// mu protects mutable fields.
	mu sync.RWMutex

	// BuildFunc is the function that performs the actual build.
	// If nil, a default no-op is used.
	BuildFunc func(ctx context.Context, pkg *pkg.Package) error
}

// NewBuildTask creates a new build task for the given package.
func NewBuildTask(p *pkg.Package) *BuildTask {
	return &BuildTask{
		ID:           p.Name + "-" + p.Version,
		Package:      p,
		Dependencies: make([]string, 0),
		Dependents:   make([]string, 0),
		status:       BuildStatusPending,
	}
}

// GetStatus returns the current build status (thread-safe).
func (t *BuildTask) GetStatus() BuildStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

// SetStatus sets the build status (thread-safe).
func (t *BuildTask) SetStatus(status BuildStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status = status
}

// GetError returns the error message if the build failed (thread-safe).
func (t *BuildTask) GetError() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.error
}

// SetError sets the error message (thread-safe).
func (t *BuildTask) SetError(err string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.error = err
}

// GetStartTime returns the build start time (thread-safe).
func (t *BuildTask) GetStartTime() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.startTime
}

// GetEndTime returns the build end time (thread-safe).
func (t *BuildTask) GetEndTime() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.endTime
}

// GetDuration returns the build duration (thread-safe).
func (t *BuildTask) GetDuration() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.startTime.IsZero() {
		return 0
	}
	if t.endTime.IsZero() {
		return time.Since(t.startTime)
	}
	return t.endTime.Sub(t.startTime)
}

// GetOutput returns the build output (thread-safe).
func (t *BuildTask) GetOutput() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.output
}

// AppendOutput appends to the build output (thread-safe).
func (t *BuildTask) AppendOutput(output string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.output += output
}

// SchedulerStats holds statistics about the scheduler state.
type SchedulerStats struct {
	// TotalTasks is the total number of tasks in the queue.
	TotalTasks int
	// PendingTasks is the number of tasks waiting to start.
	PendingTasks int
	// WaitingTasks is the number of tasks waiting for dependencies.
	WaitingTasks int
	// RunningTasks is the number of currently running tasks.
	RunningTasks int
	// CompletedTasks is the number of successfully completed tasks.
	CompletedTasks int
	// FailedTasks is the number of failed tasks.
	FailedTasks int
	// CanceledTasks is the number of canceled tasks.
	CanceledTasks int
	// ActiveWorkers is the number of workers currently building.
	ActiveWorkers int
	// MaxWorkers is the maximum number of parallel workers.
	MaxWorkers int
	// StartTime is when the scheduler started.
	StartTime time.Time
	// ElapsedTime is how long the scheduler has been running.
	ElapsedTime time.Duration
}

// SchedulerConfig holds configuration for the build scheduler.
type SchedulerConfig struct {
	// MaxWorkers is the maximum number of parallel builds.
	// Defaults to runtime.NumCPU() if <= 0.
	MaxWorkers int
	// FailureMode defines how to handle build failures.
	FailureMode FailureMode
	// Verbose enables detailed logging.
	Verbose bool
	// ProgressCallback is called when build status changes.
	ProgressCallback func(stats *SchedulerStats)
}

// DefaultSchedulerConfig returns a sensible default configuration.
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		MaxWorkers:  runtime.NumCPU(),
		FailureMode: FailureModeStop,
		Verbose:     false,
	}
}

// BuildScheduler manages parallel package builds with dependency awareness.
type BuildScheduler struct {
	config *SchedulerConfig

	// Task storage
	tasks   map[string]*BuildTask // task ID -> task
	tasksMu sync.RWMutex

	// Build order (topologically sorted)
	buildOrder []string
	orderMu    sync.RWMutex

	// State tracking
	completedSet map[string]bool // Set of completed task IDs
	completedMu  sync.RWMutex

	// Worker management
	activeWorkers int32 // atomic counter
	workerWg      sync.WaitGroup

	// Channels for coordination
	readyQueue chan *BuildTask // Tasks ready to build
	results    chan *BuildTask // Completed tasks

	// Lifecycle
	ctx        context.Context
	cancel     context.CancelFunc
	startTime  time.Time
	isRunning  atomic.Bool
	hasFailed  atomic.Bool
	failedTask *BuildTask // First task that failed
	failedMu   sync.RWMutex
}

// NewBuildScheduler creates a new build scheduler with the given configuration.
func NewBuildScheduler(config *SchedulerConfig) *BuildScheduler {
	if config == nil {
		config = DefaultSchedulerConfig()
	}
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = runtime.NumCPU()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &BuildScheduler{
		config:       config,
		tasks:        make(map[string]*BuildTask),
		completedSet: make(map[string]bool),
		buildOrder:   make([]string, 0),
		readyQueue:   make(chan *BuildTask, config.MaxWorkers*2),
		results:      make(chan *BuildTask, config.MaxWorkers*2),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// AddTask adds a build task to the scheduler.
// Tasks must be added before calling Start().
func (s *BuildScheduler) AddTask(task *BuildTask) error {
	if s.isRunning.Load() {
		return errors.New("cannot add tasks while scheduler is running")
	}

	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()

	if _, exists := s.tasks[task.ID]; exists {
		return fmt.Errorf("task %s already exists", task.ID)
	}

	s.tasks[task.ID] = task
	return nil
}

// AddTasks adds multiple build tasks to the scheduler.
func (s *BuildScheduler) AddTasks(tasks []*BuildTask) error {
	for _, task := range tasks {
		if err := s.AddTask(task); err != nil {
			return err
		}
	}
	return nil
}

// AddDependency adds a dependency relationship between tasks.
// The dependent task will not start until the dependency task completes.
func (s *BuildScheduler) AddDependency(dependentID, dependencyID string) error {
	if s.isRunning.Load() {
		return errors.New("cannot add dependencies while scheduler is running")
	}

	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()

	dependent, ok := s.tasks[dependentID]
	if !ok {
		return fmt.Errorf("dependent task not found: %s", dependentID)
	}

	dependency, ok := s.tasks[dependencyID]
	if !ok {
		return fmt.Errorf("dependency task not found: %s", dependencyID)
	}

	// Add bidirectional relationship
	dependent.Dependencies = append(dependent.Dependencies, dependencyID)
	dependency.Dependents = append(dependency.Dependents, dependentID)

	return nil
}

// Start begins executing build tasks in parallel.
// Blocks until all tasks complete or an error occurs.
func (s *BuildScheduler) Start(ctx context.Context) error {
	if s.isRunning.Swap(true) {
		return errors.New("scheduler is already running")
	}
	defer s.isRunning.Store(false)

	s.startTime = time.Now()

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.ctx = ctx

	// Compute topological order
	order, err := s.topologicalSort()
	if err != nil {
		return fmt.Errorf("dependency cycle detected: %w", err)
	}
	s.orderMu.Lock()
	s.buildOrder = order
	s.orderMu.Unlock()

	if s.config.Verbose {
		log.Printf("Build order: %v", order)
	}

	// Start workers
	for i := 0; i < s.config.MaxWorkers; i++ {
		s.workerWg.Add(1)
		go s.worker(ctx, i)
	}

	// Start result collector
	done := make(chan error, 1)
	go func() {
		done <- s.collectResults(ctx)
	}()

	// Queue initial ready tasks (those with no dependencies)
	s.queueReadyTasks()

	// Wait for completion or error
	err = <-done

	// Cancel workers and wait for them to finish
	cancel()
	close(s.readyQueue)
	s.workerWg.Wait()

	return err
}

// Stop gracefully stops the scheduler.
func (s *BuildScheduler) Stop() {
	s.cancel()
}

// GetStats returns current scheduler statistics.
func (s *BuildScheduler) GetStats() *SchedulerStats {
	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()

	stats := &SchedulerStats{
		TotalTasks:    len(s.tasks),
		MaxWorkers:    s.config.MaxWorkers,
		ActiveWorkers: int(atomic.LoadInt32(&s.activeWorkers)),
		StartTime:     s.startTime,
	}

	if !s.startTime.IsZero() {
		stats.ElapsedTime = time.Since(s.startTime)
	}

	for _, task := range s.tasks {
		switch task.GetStatus() {
		case BuildStatusPending:
			stats.PendingTasks++
		case BuildStatusWaiting:
			stats.WaitingTasks++
		case BuildStatusRunning:
			stats.RunningTasks++
		case BuildStatusCompleted:
			stats.CompletedTasks++
		case BuildStatusFailed:
			stats.FailedTasks++
		case BuildStatusCanceled:
			stats.CanceledTasks++
		}
	}

	return stats
}

// GetTask returns a task by ID.
func (s *BuildScheduler) GetTask(id string) (*BuildTask, bool) {
	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()
	task, ok := s.tasks[id]
	return task, ok
}

// GetBuildOrder returns the computed topological build order.
func (s *BuildScheduler) GetBuildOrder() []string {
	s.orderMu.RLock()
	defer s.orderMu.RUnlock()
	result := make([]string, len(s.buildOrder))
	copy(result, s.buildOrder)
	return result
}

// GetFailedTask returns the first task that failed, if any.
func (s *BuildScheduler) GetFailedTask() *BuildTask {
	s.failedMu.RLock()
	defer s.failedMu.RUnlock()
	return s.failedTask
}

// topologicalSort performs Kahn's algorithm for topological sorting.
// Returns an error if a cycle is detected.
func (s *BuildScheduler) topologicalSort() ([]string, error) {
	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()

	if len(s.tasks) == 0 {
		return nil, nil
	}

	// Calculate in-degrees (number of dependencies)
	inDegree := make(map[string]int)
	for id := range s.tasks {
		inDegree[id] = 0
	}
	for _, task := range s.tasks {
		for _, depID := range task.Dependencies {
			inDegree[task.ID]++
			_ = depID // Dependency is already counted
		}
	}

	// Find tasks with no dependencies (in-degree = 0)
	queue := make([]string, 0)
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	// Process queue
	result := make([]string, 0, len(s.tasks))
	for len(queue) > 0 {
		// Dequeue
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Reduce in-degree of dependents
		task := s.tasks[current]
		for _, dependentID := range task.Dependents {
			inDegree[dependentID]--
			if inDegree[dependentID] == 0 {
				queue = append(queue, dependentID)
			}
		}
	}

	// Check for cycle
	if len(result) != len(s.tasks) {
		return nil, errors.New("dependency cycle detected in build graph")
	}

	return result, nil
}

// worker is a goroutine that processes build tasks.
func (s *BuildScheduler) worker(ctx context.Context, id int) {
	defer s.workerWg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-s.readyQueue:
			if !ok {
				return // Channel closed
			}

			// Check if we should stop due to failure
			if s.config.FailureMode == FailureModeStop && s.hasFailed.Load() {
				task.SetStatus(BuildStatusCanceled)
				s.results <- task
				continue
			}

			// Execute the build
			atomic.AddInt32(&s.activeWorkers, 1)
			s.executeBuild(ctx, task)
			atomic.AddInt32(&s.activeWorkers, -1)

			// Send result
			s.results <- task
		}
	}
}

// executeBuild executes a single build task.
func (s *BuildScheduler) executeBuild(ctx context.Context, task *BuildTask) {
	// Mark as running
	task.mu.Lock()
	task.status = BuildStatusRunning
	task.startTime = time.Now()
	task.mu.Unlock()

	if s.config.Verbose {
		log.Printf(">>> Building %s", task.ID)
	}

	// Execute build function
	var err error
	if task.BuildFunc != nil {
		err = task.BuildFunc(ctx, task.Package)
	}

	// Update status
	task.mu.Lock()
	task.endTime = time.Now()
	if ctx.Err() != nil {
		task.status = BuildStatusCanceled
	} else if err != nil {
		task.status = BuildStatusFailed
		task.error = err.Error()
	} else {
		task.status = BuildStatusCompleted
	}
	task.mu.Unlock()

	if s.config.Verbose {
		if err != nil {
			log.Printf("!!! Build failed: %s - %v", task.ID, err)
		} else if task.GetStatus() == BuildStatusCanceled {
			log.Printf(">>> Build canceled: %s", task.ID)
		} else {
			log.Printf(">>> Build completed: %s (%.2fs)", task.ID, task.GetDuration().Seconds())
		}
	}
}

// collectResults processes completed tasks and queues newly ready tasks.
func (s *BuildScheduler) collectResults(ctx context.Context) error {
	pendingCount := len(s.tasks)

	for pendingCount > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case task := <-s.results:
			pendingCount--

			// Handle failure
			if task.GetStatus() == BuildStatusFailed {
				// Record first failure
				s.failedMu.Lock()
				if s.failedTask == nil {
					s.failedTask = task
				}
				s.failedMu.Unlock()

				if !s.hasFailed.Swap(true) && s.config.FailureMode == FailureModeStop {
					// First failure in stop mode - cancel remaining tasks
					s.cancelRemainingTasks()
				}
			}

			// Mark as completed in set
			if task.GetStatus() == BuildStatusCompleted {
				s.completedMu.Lock()
				s.completedSet[task.ID] = true
				s.completedMu.Unlock()

				// Queue newly ready dependents
				s.queueReadyDependents(task)
			}

			// Report progress
			if s.config.ProgressCallback != nil {
				s.config.ProgressCallback(s.GetStats())
			}
		}
	}

	// Return error if any task failed
	if s.hasFailed.Load() {
		failedTask := s.GetFailedTask()
		if failedTask != nil {
			return fmt.Errorf("build failed: %s: %s", failedTask.ID, failedTask.GetError())
		}
		return errors.New("build failed")
	}

	return nil
}

// queueReadyTasks queues all tasks that have no pending dependencies.
func (s *BuildScheduler) queueReadyTasks() {
	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()

	for _, task := range s.tasks {
		if len(task.Dependencies) == 0 {
			task.SetStatus(BuildStatusWaiting)
			s.readyQueue <- task
		} else {
			task.SetStatus(BuildStatusWaiting)
		}
	}
}

// queueReadyDependents queues dependents of a completed task that are now ready.
func (s *BuildScheduler) queueReadyDependents(completed *BuildTask) {
	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()

	for _, dependentID := range completed.Dependents {
		dependent, ok := s.tasks[dependentID]
		if !ok {
			continue
		}

		// Check if all dependencies are completed
		if s.areDependenciesCompleted(dependent) {
			// Queue for execution (non-blocking)
			select {
			case s.readyQueue <- dependent:
			default:
				// Queue full - this shouldn't happen with proper sizing
				log.Printf("Warning: ready queue full, dropping task %s", dependent.ID)
			}
		}
	}
}

// areDependenciesCompleted checks if all dependencies of a task are completed.
func (s *BuildScheduler) areDependenciesCompleted(task *BuildTask) bool {
	s.completedMu.RLock()
	defer s.completedMu.RUnlock()

	for _, depID := range task.Dependencies {
		if !s.completedSet[depID] {
			return false
		}
	}
	return true
}

// cancelRemainingTasks marks all pending/waiting tasks as canceled.
func (s *BuildScheduler) cancelRemainingTasks() {
	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()

	for _, task := range s.tasks {
		status := task.GetStatus()
		if status == BuildStatusPending || status == BuildStatusWaiting {
			task.SetStatus(BuildStatusCanceled)
		}
	}
}

// FormatStats returns a formatted string representation of scheduler stats.
func FormatStats(stats *SchedulerStats) string {
	return fmt.Sprintf(
		"[%d/%d] Running: %d | Completed: %d | Failed: %d | Canceled: %d | Workers: %d/%d | Elapsed: %s",
		stats.CompletedTasks+stats.RunningTasks,
		stats.TotalTasks,
		stats.RunningTasks,
		stats.CompletedTasks,
		stats.FailedTasks,
		stats.CanceledTasks,
		stats.ActiveWorkers,
		stats.MaxWorkers,
		stats.ElapsedTime.Round(time.Second),
	)
}

// BuildFromPackages creates a scheduler from a dependency-resolved package map.
// It extracts dependencies from package metadata and creates appropriate tasks.
func BuildFromPackages(packages map[string]*pkg.Package, config *SchedulerConfig) (*BuildScheduler, error) {
	scheduler := NewBuildScheduler(config)

	// Create tasks for all packages
	taskMap := make(map[string]*BuildTask)
	for _, p := range packages {
		task := NewBuildTask(p)
		taskMap[p.Name] = task
		if err := scheduler.AddTask(task); err != nil {
			return nil, fmt.Errorf("failed to add task for %s: %w", p.Name, err)
		}
	}

	// Add dependencies based on package dependencies
	for _, p := range packages {
		for _, dep := range p.Deps {
			depName := dep.Name
			// Check if dependency is in our package set
			if _, exists := taskMap[depName]; exists {
				taskID := p.Name + "-" + p.Version
				depTaskID := packages[depName].Name + "-" + packages[depName].Version
				if err := scheduler.AddDependency(taskID, depTaskID); err != nil {
					// Log warning but don't fail - dependency might be external
					log.Printf("Warning: could not add dependency %s -> %s: %v", taskID, depTaskID, err)
				}
			}
		}
	}

	return scheduler, nil
}
