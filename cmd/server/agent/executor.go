package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// TaskExecutor handles the execution of tasks
type TaskExecutor struct {
	coordinator *Coordinator
	workers     []*Worker
	results     map[string]*TaskResult
	resultsMu   sync.RWMutex
	running     bool
	runningMu   sync.Mutex
}

// Worker represents a worker that executes tasks
type Worker struct {
	id       int
	executor *TaskExecutor
	tasks    chan *Task
	quit     chan bool
}

// NewTaskExecutor creates a new task executor
func NewTaskExecutor(coordinator *Coordinator) *TaskExecutor {
	executor := &TaskExecutor{
		coordinator: coordinator,
		results:     make(map[string]*TaskResult),
		workers:     make([]*Worker, 5), // 5 concurrent workers
	}

	// Initialize workers
	for i := 0; i < 5; i++ {
		worker := &Worker{
			id:       i,
			executor: executor,
			tasks:    make(chan *Task, 10),
			quit:     make(chan bool),
		}
		executor.workers[i] = worker
	}

	return executor
}

// Start starts the task executor
func (e *TaskExecutor) Start(ctx context.Context) error {
	e.runningMu.Lock()
	if e.running {
		e.runningMu.Unlock()
		return nil
	}
	e.running = true
	e.runningMu.Unlock()

	// Start workers
	for _, worker := range e.workers {
		go worker.run(ctx)
	}

	// Process tasks from queue
	go e.processTasks(ctx)

	return nil
}

// Stop stops the task executor
func (e *TaskExecutor) Stop() {
	e.runningMu.Lock()
	defer e.runningMu.Unlock()

	if !e.running {
		return
	}

	e.running = false

	// Stop all workers
	for _, worker := range e.workers {
		worker.quit <- true
	}
}

// processTasks continuously processes tasks from the queue
func (e *TaskExecutor) processTasks(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			task := e.coordinator.taskQueue.Next()
			if task == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Find available worker
			assigned := false
			for _, worker := range e.workers {
				select {
				case worker.tasks <- task:
					assigned = true
					break
				default:
					continue
				}
				if assigned {
					break
				}
			}

			if !assigned {
				// All workers busy, put task back
				e.coordinator.taskQueue.Add(task)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

// GetResult retrieves a task result
func (e *TaskExecutor) GetResult(taskID string) *TaskResult {
	e.resultsMu.RLock()
	defer e.resultsMu.RUnlock()
	return e.results[taskID]
}

// storeResult stores a task result
func (e *TaskExecutor) storeResult(taskID string, result *TaskResult) {
	e.resultsMu.Lock()
	defer e.resultsMu.Unlock()
	e.results[taskID] = result

	// Also store in memory for learning
	e.coordinator.memory.Store(taskID, result)
}

// run is the main worker loop
func (w *Worker) run(ctx context.Context) {
	log.Printf("[Worker %d] Started", w.id)

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.quit:
			return
		case task := <-w.tasks:
			w.executeTask(ctx, task)
		}
	}
}

// executeTask executes a single task
func (w *Worker) executeTask(ctx context.Context, task *Task) {
	start := time.Now()
	log.Printf("[Worker %d] Executing task %s of type %s", w.id, task.ID, task.Type)
	log.Printf("[Worker %d] Task input type: %T, content: %+v", w.id, task.Input, task.Input)
	log.Printf("[Worker %d] Task context: %+v", w.id, task.Context)

	// Find appropriate agent
	agent := w.executor.coordinator.GetAgent(task)
	if agent == nil {
		log.Printf("[Worker %d] ERROR: No agent available for task type %s", w.id, task.Type)
		result := &TaskResult{
			TaskID:        task.ID,
			Success:       false,
			Error:         fmt.Errorf("no agent available for task type %s", task.Type),
			ExecutionTime: time.Since(start),
		}
		w.executor.storeResult(task.ID, result)
		return
	}
	log.Printf("[Worker %d] Found agent %T for task", w.id, agent)

	// Execute task
	result, err := agent.Execute(ctx, task)
	if err != nil {
		result = &TaskResult{
			TaskID:        task.ID,
			Success:       false,
			Error:         err,
			ExecutionTime: time.Since(start),
		}
	} else {
		result.ExecutionTime = time.Since(start)
	}

	// Store result
	w.executor.storeResult(task.ID, result)

	// Publish task completion update
	w.executor.coordinator.PublishTaskUpdate(&TaskUpdate{
		TaskID:  task.ID,
		Type:    UpdateTypeResult,
		Status:  TaskStatusCompleted,
		Message: "Task completed successfully",
		Data:    result,
	})

	// Clean up task channels after a small delay to ensure all subscribers receive the update
	go func() {
		time.Sleep(100 * time.Millisecond)
		w.executor.coordinator.CleanupTask(task.ID)
	}()

	// Queue any follow-up tasks
	if result.NextTasks != nil {
		for _, nextTask := range result.NextTasks {
			if err := w.executor.coordinator.taskQueue.Add(nextTask); err != nil {
				log.Printf("[Worker %d] Failed to queue follow-up task: %v", w.id, err)
			}
		}
	}

	log.Printf("[Worker %d] Completed task %s in %v", w.id, task.ID, result.ExecutionTime)
}
