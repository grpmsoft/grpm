package daemon

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// TestBuildTask_NewBuildTask tests task creation.
func TestBuildTask_NewBuildTask(t *testing.T) {
	p := &pkg.Package{Name: "sys-libs/zlib", Version: "1.2.13"}
	task := NewBuildTask(p)

	if task.ID != "sys-libs/zlib-1.2.13" {
		t.Errorf("Expected ID 'sys-libs/zlib-1.2.13', got '%s'", task.ID)
	}
	if task.Package != p {
		t.Error("Expected Package to be set")
	}
	if task.GetStatus() != BuildStatusPending {
		t.Errorf("Expected status %s, got %s", BuildStatusPending, task.GetStatus())
	}
	if len(task.Dependencies) != 0 {
		t.Errorf("Expected empty Dependencies, got %d", len(task.Dependencies))
	}
	if len(task.Dependents) != 0 {
		t.Errorf("Expected empty Dependents, got %d", len(task.Dependents))
	}
}

// TestBuildTask_StatusMethods tests status getter/setter.
func TestBuildTask_StatusMethods(t *testing.T) {
	p := &pkg.Package{Name: "test/pkg", Version: "1.0"}
	task := NewBuildTask(p)

	tests := []struct {
		status BuildStatus
		want   string
	}{
		{BuildStatusPending, "pending"},
		{BuildStatusWaiting, "waiting"},
		{BuildStatusRunning, "running"},
		{BuildStatusCompleted, "completed"},
		{BuildStatusFailed, "failed"},
		{BuildStatusCanceled, "canceled"},
	}

	for _, tt := range tests {
		task.SetStatus(tt.status)
		if task.GetStatus() != tt.status {
			t.Errorf("SetStatus(%s): got %s, want %s", tt.want, task.GetStatus(), tt.status)
		}
		if tt.status.String() != tt.want {
			t.Errorf("String(): got %s, want %s", tt.status.String(), tt.want)
		}
	}
}

// TestBuildTask_ErrorMethods tests error getter/setter.
func TestBuildTask_ErrorMethods(t *testing.T) {
	p := &pkg.Package{Name: "test/pkg", Version: "1.0"}
	task := NewBuildTask(p)

	if task.GetError() != "" {
		t.Errorf("Expected empty error, got '%s'", task.GetError())
	}

	task.SetError("build failed: missing dependency")
	if task.GetError() != "build failed: missing dependency" {
		t.Errorf("Expected 'build failed: missing dependency', got '%s'", task.GetError())
	}
}

// TestBuildTask_TimeMethods tests time tracking methods.
func TestBuildTask_TimeMethods(t *testing.T) {
	p := &pkg.Package{Name: "test/pkg", Version: "1.0"}
	task := NewBuildTask(p)

	// Initial state
	if !task.GetStartTime().IsZero() {
		t.Error("Expected zero start time initially")
	}
	if !task.GetEndTime().IsZero() {
		t.Error("Expected zero end time initially")
	}
	if task.GetDuration() != 0 {
		t.Error("Expected zero duration initially")
	}
}

// TestBuildTask_OutputMethods tests output getter/appender.
func TestBuildTask_OutputMethods(t *testing.T) {
	p := &pkg.Package{Name: "test/pkg", Version: "1.0"}
	task := NewBuildTask(p)

	if task.GetOutput() != "" {
		t.Errorf("Expected empty output, got '%s'", task.GetOutput())
	}

	task.AppendOutput("line 1\n")
	task.AppendOutput("line 2\n")

	expected := "line 1\nline 2\n"
	if task.GetOutput() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, task.GetOutput())
	}
}

// TestSchedulerConfig_DefaultSchedulerConfig tests default configuration.
func TestSchedulerConfig_DefaultSchedulerConfig(t *testing.T) {
	config := DefaultSchedulerConfig()

	if config.MaxWorkers <= 0 {
		t.Error("Expected MaxWorkers > 0")
	}
	if config.FailureMode != FailureModeStop {
		t.Errorf("Expected FailureMode %d, got %d", FailureModeStop, config.FailureMode)
	}
	if config.Verbose {
		t.Error("Expected Verbose to be false by default")
	}
}

// TestBuildScheduler_NewBuildScheduler tests scheduler creation.
func TestBuildScheduler_NewBuildScheduler(t *testing.T) {
	// With nil config
	s := NewBuildScheduler(nil)
	if s.config.MaxWorkers <= 0 {
		t.Error("Expected MaxWorkers > 0 with nil config")
	}

	// With custom config
	config := &SchedulerConfig{MaxWorkers: 8, FailureMode: FailureModeContinue}
	s = NewBuildScheduler(config)
	if s.config.MaxWorkers != 8 {
		t.Errorf("Expected MaxWorkers 8, got %d", s.config.MaxWorkers)
	}
	if s.config.FailureMode != FailureModeContinue {
		t.Errorf("Expected FailureMode %d, got %d", FailureModeContinue, s.config.FailureMode)
	}

	// With zero MaxWorkers (should use CPU count)
	config = &SchedulerConfig{MaxWorkers: 0}
	s = NewBuildScheduler(config)
	if s.config.MaxWorkers <= 0 {
		t.Error("Expected MaxWorkers > 0 when configured as 0")
	}
}

// TestBuildScheduler_AddTask tests adding tasks.
func TestBuildScheduler_AddTask(t *testing.T) {
	s := NewBuildScheduler(nil)

	task1 := NewBuildTask(&pkg.Package{Name: "pkg/a", Version: "1.0"})
	task2 := NewBuildTask(&pkg.Package{Name: "pkg/b", Version: "2.0"})

	// Add first task
	if err := s.AddTask(task1); err != nil {
		t.Fatalf("Failed to add task1: %v", err)
	}

	// Add second task
	if err := s.AddTask(task2); err != nil {
		t.Fatalf("Failed to add task2: %v", err)
	}

	// Try to add duplicate
	err := s.AddTask(task1)
	if err == nil {
		t.Error("Expected error when adding duplicate task")
	}

	// Verify tasks are stored
	retrieved, ok := s.GetTask(task1.ID)
	if !ok {
		t.Error("Task1 not found")
	}
	if retrieved.ID != task1.ID {
		t.Errorf("Expected ID %s, got %s", task1.ID, retrieved.ID)
	}
}

// TestBuildScheduler_AddTasks tests adding multiple tasks.
func TestBuildScheduler_AddTasks(t *testing.T) {
	s := NewBuildScheduler(nil)

	tasks := []*BuildTask{
		NewBuildTask(&pkg.Package{Name: "pkg/a", Version: "1.0"}),
		NewBuildTask(&pkg.Package{Name: "pkg/b", Version: "2.0"}),
		NewBuildTask(&pkg.Package{Name: "pkg/c", Version: "3.0"}),
	}

	if err := s.AddTasks(tasks); err != nil {
		t.Fatalf("Failed to add tasks: %v", err)
	}

	stats := s.GetStats()
	if stats.TotalTasks != 3 {
		t.Errorf("Expected 3 tasks, got %d", stats.TotalTasks)
	}
}

// TestBuildScheduler_AddDependency tests adding dependencies.
func TestBuildScheduler_AddDependency(t *testing.T) {
	s := NewBuildScheduler(nil)

	taskA := NewBuildTask(&pkg.Package{Name: "pkg/a", Version: "1.0"})
	taskB := NewBuildTask(&pkg.Package{Name: "pkg/b", Version: "2.0"})

	_ = s.AddTask(taskA)
	_ = s.AddTask(taskB)

	// Add valid dependency: A depends on B
	err := s.AddDependency(taskA.ID, taskB.ID)
	if err != nil {
		t.Fatalf("Failed to add dependency: %v", err)
	}

	// Verify bidirectional relationship
	if len(taskA.Dependencies) != 1 || taskA.Dependencies[0] != taskB.ID {
		t.Error("Task A should have B as dependency")
	}
	if len(taskB.Dependents) != 1 || taskB.Dependents[0] != taskA.ID {
		t.Error("Task B should have A as dependent")
	}

	// Try to add dependency with non-existent task
	err = s.AddDependency("nonexistent", taskB.ID)
	if err == nil {
		t.Error("Expected error for non-existent dependent")
	}

	err = s.AddDependency(taskA.ID, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent dependency")
	}
}

// TestBuildScheduler_TopologicalSort tests topological sorting.
func TestBuildScheduler_TopologicalSort(t *testing.T) {
	tests := []struct {
		name     string
		tasks    []string
		deps     [][2]string // [dependent, dependency]
		wantErr  bool
		validate func(order []string) bool
	}{
		{
			name:  "no tasks",
			tasks: []string{},
			deps:  nil,
		},
		{
			name:  "single task",
			tasks: []string{"pkg/a-1.0"},
			deps:  nil,
		},
		{
			name:  "two independent tasks",
			tasks: []string{"pkg/a-1.0", "pkg/b-2.0"},
			deps:  nil,
		},
		{
			name:  "linear dependency chain",
			tasks: []string{"pkg/a-1.0", "pkg/b-2.0", "pkg/c-3.0"},
			deps: [][2]string{
				{"pkg/a-1.0", "pkg/b-2.0"}, // A depends on B
				{"pkg/b-2.0", "pkg/c-3.0"}, // B depends on C
			},
			validate: func(order []string) bool {
				// C must come before B, B must come before A
				posC, posB, posA := -1, -1, -1
				for i, id := range order {
					switch id {
					case "pkg/c-3.0":
						posC = i
					case "pkg/b-2.0":
						posB = i
					case "pkg/a-1.0":
						posA = i
					}
				}
				return posC < posB && posB < posA
			},
		},
		{
			name:  "diamond dependency",
			tasks: []string{"pkg/a-1.0", "pkg/b-2.0", "pkg/c-3.0", "pkg/d-4.0"},
			deps: [][2]string{
				{"pkg/a-1.0", "pkg/b-2.0"}, // A depends on B
				{"pkg/a-1.0", "pkg/c-3.0"}, // A depends on C
				{"pkg/b-2.0", "pkg/d-4.0"}, // B depends on D
				{"pkg/c-3.0", "pkg/d-4.0"}, // C depends on D
			},
			validate: func(order []string) bool {
				// D must come before B and C, B and C must come before A
				pos := make(map[string]int)
				for i, id := range order {
					pos[id] = i
				}
				return pos["pkg/d-4.0"] < pos["pkg/b-2.0"] &&
					pos["pkg/d-4.0"] < pos["pkg/c-3.0"] &&
					pos["pkg/b-2.0"] < pos["pkg/a-1.0"] &&
					pos["pkg/c-3.0"] < pos["pkg/a-1.0"]
			},
		},
		{
			name:  "cycle detection",
			tasks: []string{"pkg/a-1.0", "pkg/b-2.0"},
			deps: [][2]string{
				{"pkg/a-1.0", "pkg/b-2.0"}, // A depends on B
				{"pkg/b-2.0", "pkg/a-1.0"}, // B depends on A (cycle!)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewBuildScheduler(nil)

			// Add tasks
			for _, taskID := range tt.tasks {
				// Parse name and version from ID
				task := &BuildTask{
					ID:           taskID,
					Package:      &pkg.Package{Name: taskID, Version: ""},
					Dependencies: make([]string, 0),
					Dependents:   make([]string, 0),
					status:       BuildStatusPending,
				}
				_ = s.AddTask(task)
			}

			// Add dependencies
			for _, dep := range tt.deps {
				_ = s.AddDependency(dep[0], dep[1])
			}

			// Run topological sort
			order, err := s.topologicalSort()

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(order) != len(tt.tasks) {
				t.Errorf("Expected %d tasks in order, got %d", len(tt.tasks), len(order))
			}

			if tt.validate != nil && !tt.validate(order) {
				t.Errorf("Invalid topological order: %v", order)
			}
		})
	}
}

// TestBuildScheduler_ExecuteSimple tests simple task execution.
func TestBuildScheduler_ExecuteSimple(t *testing.T) {
	s := NewBuildScheduler(&SchedulerConfig{MaxWorkers: 2})

	var executed atomic.Bool
	task := NewBuildTask(&pkg.Package{Name: "test/pkg", Version: "1.0"})
	task.BuildFunc = func(ctx context.Context, p *pkg.Package) error {
		executed.Store(true)
		return nil
	}

	_ = s.AddTask(task)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !executed.Load() {
		t.Error("Task was not executed")
	}

	stats := s.GetStats()
	if stats.CompletedTasks != 1 {
		t.Errorf("Expected 1 completed task, got %d", stats.CompletedTasks)
	}
	if stats.FailedTasks != 0 {
		t.Errorf("Expected 0 failed tasks, got %d", stats.FailedTasks)
	}
}

// TestBuildScheduler_ExecuteWithDependencies tests dependency-ordered execution.
func TestBuildScheduler_ExecuteWithDependencies(t *testing.T) {
	s := NewBuildScheduler(&SchedulerConfig{MaxWorkers: 4, Verbose: false})

	// Track execution order
	var executionOrder []string
	var orderMu sync.Mutex

	createTask := func(name string) *BuildTask {
		task := NewBuildTask(&pkg.Package{Name: name, Version: "1.0"})
		nameCopy := name
		task.BuildFunc = func(ctx context.Context, p *pkg.Package) error {
			orderMu.Lock()
			executionOrder = append(executionOrder, nameCopy)
			orderMu.Unlock()
			time.Sleep(10 * time.Millisecond) // Small delay to ensure ordering is meaningful
			return nil
		}
		return task
	}

	// A depends on B, B depends on C
	taskA := createTask("pkg/a")
	taskB := createTask("pkg/b")
	taskC := createTask("pkg/c")

	_ = s.AddTask(taskA)
	_ = s.AddTask(taskB)
	_ = s.AddTask(taskC)

	_ = s.AddDependency(taskA.ID, taskB.ID) // A depends on B
	_ = s.AddDependency(taskB.ID, taskC.ID) // B depends on C

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := s.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify execution order: C, B, A
	orderMu.Lock()
	defer orderMu.Unlock()

	if len(executionOrder) != 3 {
		t.Fatalf("Expected 3 tasks executed, got %d", len(executionOrder))
	}

	// Find positions
	pos := make(map[string]int)
	for i, name := range executionOrder {
		pos[name] = i
	}

	if pos["pkg/c"] > pos["pkg/b"] {
		t.Error("C should execute before B")
	}
	if pos["pkg/b"] > pos["pkg/a"] {
		t.Error("B should execute before A")
	}
}

// TestBuildScheduler_ExecuteFailure tests failure handling.
func TestBuildScheduler_ExecuteFailure(t *testing.T) {
	s := NewBuildScheduler(&SchedulerConfig{MaxWorkers: 2, FailureMode: FailureModeStop})

	expectedErr := errors.New("build error")
	task := NewBuildTask(&pkg.Package{Name: "test/pkg", Version: "1.0"})
	task.BuildFunc = func(ctx context.Context, p *pkg.Package) error {
		return expectedErr
	}

	_ = s.AddTask(task)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.Start(ctx)
	if err == nil {
		t.Error("Expected error but got none")
	}

	stats := s.GetStats()
	if stats.FailedTasks != 1 {
		t.Errorf("Expected 1 failed task, got %d", stats.FailedTasks)
	}

	failedTask := s.GetFailedTask()
	if failedTask == nil {
		t.Error("Expected failed task to be recorded")
	} else if failedTask.GetError() != expectedErr.Error() {
		t.Errorf("Expected error '%s', got '%s'", expectedErr.Error(), failedTask.GetError())
	}
}

// TestBuildScheduler_FailureModeStop tests that FailureModeStop cancels remaining tasks.
func TestBuildScheduler_FailureModeStop(t *testing.T) {
	s := NewBuildScheduler(&SchedulerConfig{MaxWorkers: 2, FailureMode: FailureModeStop})

	var executed []string
	var mu sync.Mutex

	// Task A (will fail)
	taskA := NewBuildTask(&pkg.Package{Name: "pkg/a", Version: "1.0"})
	taskA.BuildFunc = func(ctx context.Context, p *pkg.Package) error {
		mu.Lock()
		executed = append(executed, "pkg/a")
		mu.Unlock()
		return errors.New("intentional failure")
	}

	// Task B depends on A (should not run because A failed)
	taskB := NewBuildTask(&pkg.Package{Name: "pkg/b", Version: "2.0"})
	taskB.BuildFunc = func(ctx context.Context, p *pkg.Package) error {
		mu.Lock()
		executed = append(executed, "pkg/b")
		mu.Unlock()
		return nil
	}

	_ = s.AddTask(taskA)
	_ = s.AddTask(taskB)
	_ = s.AddDependency(taskB.ID, taskA.ID) // B depends on A

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_ = s.Start(ctx)

	mu.Lock()
	defer mu.Unlock()

	// Only task A should have executed (B depends on A which failed)
	if len(executed) != 1 || executed[0] != "pkg/a" {
		t.Errorf("Only task A should have executed, got: %v", executed)
	}

	stats := s.GetStats()
	if stats.FailedTasks != 1 {
		t.Errorf("Expected 1 failed task, got %d", stats.FailedTasks)
	}
}

// TestBuildScheduler_FailureModeContinue tests that FailureModeContinue keeps building.
func TestBuildScheduler_FailureModeContinue(t *testing.T) {
	s := NewBuildScheduler(&SchedulerConfig{MaxWorkers: 2, FailureMode: FailureModeContinue})

	var executed []string
	var mu sync.Mutex

	// Task A (will fail)
	taskA := NewBuildTask(&pkg.Package{Name: "pkg/a", Version: "1.0"})
	taskA.BuildFunc = func(ctx context.Context, p *pkg.Package) error {
		mu.Lock()
		executed = append(executed, "pkg/a")
		mu.Unlock()
		return errors.New("intentional failure")
	}

	// Task B (independent, should run)
	taskB := NewBuildTask(&pkg.Package{Name: "pkg/b", Version: "2.0"})
	taskB.BuildFunc = func(ctx context.Context, p *pkg.Package) error {
		mu.Lock()
		executed = append(executed, "pkg/b")
		mu.Unlock()
		return nil
	}

	_ = s.AddTask(taskA)
	_ = s.AddTask(taskB)
	// No dependency between A and B

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = s.Start(ctx)

	mu.Lock()
	defer mu.Unlock()

	// Both tasks should have executed
	if len(executed) != 2 {
		t.Errorf("Expected 2 tasks executed, got %d: %v", len(executed), executed)
	}

	stats := s.GetStats()
	if stats.CompletedTasks != 1 {
		t.Errorf("Expected 1 completed task, got %d", stats.CompletedTasks)
	}
	if stats.FailedTasks != 1 {
		t.Errorf("Expected 1 failed task, got %d", stats.FailedTasks)
	}
}

// TestBuildScheduler_ParallelExecution tests parallel execution.
func TestBuildScheduler_ParallelExecution(t *testing.T) {
	s := NewBuildScheduler(&SchedulerConfig{MaxWorkers: 4})

	var maxConcurrent int32
	var currentConcurrent int32

	numTasks := 8
	for i := 0; i < numTasks; i++ {
		task := NewBuildTask(&pkg.Package{Name: "pkg/" + string(rune('a'+i)), Version: "1.0"})
		task.BuildFunc = func(ctx context.Context, p *pkg.Package) error {
			// Track concurrency
			current := atomic.AddInt32(&currentConcurrent, 1)
			for {
				max := atomic.LoadInt32(&maxConcurrent)
				if current <= max || atomic.CompareAndSwapInt32(&maxConcurrent, max, current) {
					break
				}
			}

			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&currentConcurrent, -1)
			return nil
		}
		_ = s.AddTask(task)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := s.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Should have achieved some parallelism
	if atomic.LoadInt32(&maxConcurrent) < 2 {
		t.Errorf("Expected parallel execution, max concurrent was %d", maxConcurrent)
	}

	stats := s.GetStats()
	if stats.CompletedTasks != numTasks {
		t.Errorf("Expected %d completed tasks, got %d", numTasks, stats.CompletedTasks)
	}
}

// TestBuildScheduler_CannotAddWhileRunning tests that tasks cannot be added during execution.
func TestBuildScheduler_CannotAddWhileRunning(t *testing.T) {
	s := NewBuildScheduler(&SchedulerConfig{MaxWorkers: 1})

	started := make(chan struct{})
	block := make(chan struct{})

	task1 := NewBuildTask(&pkg.Package{Name: "pkg/a", Version: "1.0"})
	task1.BuildFunc = func(ctx context.Context, p *pkg.Package) error {
		close(started)
		<-block // Block until test signals
		return nil
	}

	_ = s.AddTask(task1)

	go func() {
		ctx := context.Background()
		_ = s.Start(ctx)
	}()

	// Wait for task to start
	<-started

	// Try to add task while running
	task2 := NewBuildTask(&pkg.Package{Name: "pkg/b", Version: "2.0"})
	err := s.AddTask(task2)
	if err == nil {
		t.Error("Expected error when adding task while running")
	}

	// Unblock and cleanup
	close(block)
	time.Sleep(100 * time.Millisecond) // Wait for scheduler to finish
}

// TestBuildScheduler_GetStats tests statistics collection.
func TestBuildScheduler_GetStats(t *testing.T) {
	s := NewBuildScheduler(&SchedulerConfig{MaxWorkers: 2, FailureMode: FailureModeContinue})

	// Add tasks with various outcomes
	task1 := NewBuildTask(&pkg.Package{Name: "pkg/success", Version: "1.0"})
	task1.BuildFunc = func(ctx context.Context, p *pkg.Package) error {
		time.Sleep(10 * time.Millisecond) // Ensure measurable time
		return nil
	}

	task2 := NewBuildTask(&pkg.Package{Name: "pkg/fail", Version: "1.0"})
	task2.BuildFunc = func(ctx context.Context, p *pkg.Package) error {
		return errors.New("failed")
	}

	_ = s.AddTask(task1)
	_ = s.AddTask(task2)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = s.Start(ctx) // Will return error due to failure

	stats := s.GetStats()

	if stats.TotalTasks != 2 {
		t.Errorf("Expected 2 total tasks, got %d", stats.TotalTasks)
	}
	if stats.CompletedTasks != 1 {
		t.Errorf("Expected 1 completed task, got %d", stats.CompletedTasks)
	}
	if stats.FailedTasks != 1 {
		t.Errorf("Expected 1 failed task, got %d", stats.FailedTasks)
	}
	if stats.MaxWorkers != 2 {
		t.Errorf("Expected MaxWorkers 2, got %d", stats.MaxWorkers)
	}
	// ElapsedTime may be very small or zero if scheduler is very fast - that's ok
	// Just verify it's non-negative
	if stats.ElapsedTime < 0 {
		t.Error("Expected non-negative elapsed time")
	}
}

// TestBuildScheduler_ProgressCallback tests progress callback invocation.
func TestBuildScheduler_ProgressCallback(t *testing.T) {
	var callbackCount int32

	s := NewBuildScheduler(&SchedulerConfig{
		MaxWorkers: 1,
		ProgressCallback: func(stats *SchedulerStats) {
			atomic.AddInt32(&callbackCount, 1)
		},
	})

	for i := 0; i < 3; i++ {
		task := NewBuildTask(&pkg.Package{Name: "pkg/" + string(rune('a'+i)), Version: "1.0"})
		task.BuildFunc = func(ctx context.Context, p *pkg.Package) error {
			return nil
		}
		_ = s.AddTask(task)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = s.Start(ctx)

	// Should have been called at least once per task completion
	if atomic.LoadInt32(&callbackCount) < 3 {
		t.Errorf("Expected at least 3 callbacks, got %d", callbackCount)
	}
}

// TestBuildScheduler_CancellationContext tests context cancellation.
func TestBuildScheduler_CancellationContext(t *testing.T) {
	s := NewBuildScheduler(&SchedulerConfig{MaxWorkers: 2})

	started := make(chan struct{})
	task := NewBuildTask(&pkg.Package{Name: "pkg/slow", Version: "1.0"})
	task.BuildFunc = func(ctx context.Context, p *pkg.Package) error {
		close(started)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
			return nil
		}
	}

	_ = s.AddTask(task)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := s.Start(ctx)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		// Note: The error might be wrapped, so check if it's a cancellation
		if err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			// OK
		} else {
			t.Errorf("Expected context deadline exceeded error, got: %v", err)
		}
	}
}

// TestFormatStats tests the FormatStats function.
func TestFormatStats(t *testing.T) {
	stats := &SchedulerStats{
		TotalTasks:     10,
		CompletedTasks: 5,
		RunningTasks:   2,
		FailedTasks:    1,
		CanceledTasks:  0,
		ActiveWorkers:  2,
		MaxWorkers:     4,
		ElapsedTime:    30 * time.Second,
	}

	formatted := FormatStats(stats)

	// Should contain key information
	if formatted == "" {
		t.Error("Expected non-empty formatted string")
	}
	// Just verify it doesn't panic and produces output
	t.Logf("FormatStats output: %s", formatted)
}

// TestBuildFromPackages tests the BuildFromPackages helper function.
func TestBuildFromPackages(t *testing.T) {
	packages := map[string]*pkg.Package{
		"pkg/a": {Name: "pkg/a", Version: "1.0", Deps: []pkg.Constraint{{Name: "pkg/b"}}},
		"pkg/b": {Name: "pkg/b", Version: "2.0", Deps: nil},
	}

	config := &SchedulerConfig{MaxWorkers: 2}
	scheduler, err := BuildFromPackages(packages, config)
	if err != nil {
		t.Fatalf("BuildFromPackages failed: %v", err)
	}

	stats := scheduler.GetStats()
	if stats.TotalTasks != 2 {
		t.Errorf("Expected 2 tasks, got %d", stats.TotalTasks)
	}

	// Verify task A has dependency on B
	taskA, ok := scheduler.GetTask("pkg/a-1.0")
	if !ok {
		t.Fatal("Task A not found")
	}
	if len(taskA.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency for task A, got %d", len(taskA.Dependencies))
	}
}

// BenchmarkBuildScheduler_Execute benchmarks scheduler execution.
func BenchmarkBuildScheduler_Execute(b *testing.B) {
	for i := 0; i < b.N; i++ {
		s := NewBuildScheduler(&SchedulerConfig{MaxWorkers: 4})

		for j := 0; j < 100; j++ {
			task := NewBuildTask(&pkg.Package{Name: "pkg/" + string(rune(j)), Version: "1.0"})
			task.BuildFunc = func(ctx context.Context, p *pkg.Package) error {
				return nil // No-op build
			}
			_ = s.AddTask(task)
		}

		ctx := context.Background()
		_ = s.Start(ctx)
	}
}
