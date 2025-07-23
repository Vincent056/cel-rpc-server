package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
)

// Agent represents an autonomous agent that can execute tasks
type Agent interface {
	GetID() string
	GetCapabilities() []string
	CanHandle(task *Task) bool
	Execute(ctx context.Context, task *Task) (*TaskResult, error)
}

// Task represents a unit of work for agents
type Task struct {
	ID           string
	Type         TaskType
	Priority     int
	Input        interface{}
	Context      map[string]interface{}
	CreatedAt    time.Time
	Deadline     *time.Time
	Dependencies []string
}

// TaskType represents different types of tasks
type TaskType string

const (
	TaskTypeRuleGeneration    TaskType = "rule_generation"
	TaskTypeValidation        TaskType = "validation"
	TaskTypeResourceDiscovery TaskType = "resource_discovery"
	TaskTypeTestGeneration    TaskType = "test_generation"
	TaskTypeSecurity          TaskType = "security_scan"
	TaskTypeCompliance        TaskType = "compliance_check"
	TaskTypeOptimization      TaskType = "optimization"
	TaskTypeDocumentation     TaskType = "documentation"
	TaskTypeCELExpression     TaskType = "cel_expression"
)

// TaskResult represents the outcome of a task execution
type TaskResult struct {
	TaskID        string
	Success       bool
	Output        interface{}
	Error         error
	ExecutionTime time.Duration
	Logs          []string
	NextTasks     []*Task // Tasks to execute next
}

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// UpdateType represents the type of task update
type UpdateType string

const (
	UpdateTypeThinking UpdateType = "thinking"
	UpdateTypeProgress UpdateType = "progress"
	UpdateTypeResult   UpdateType = "result"
	UpdateTypeError    UpdateType = "error"
)

// TaskUpdate represents an update from task execution
type TaskUpdate struct {
	TaskID  string
	Type    UpdateType
	Status  TaskStatus
	Message string
	Data    interface{}
	Time    time.Time
}

// Coordinator manages agents and task execution
type Coordinator struct {
	agents    map[string]Agent
	taskQueue *TaskQueue
	executor  *TaskExecutor
	planner   *TaskPlanner
	memory    *Memory
	mu        sync.RWMutex

	// Task tracking
	taskResults map[string]*TaskResult
	taskUpdates map[string][]chan *TaskUpdate
	updatesMu   sync.RWMutex
}

// NewCoordinator creates a new agent coordinator
func NewCoordinator() *Coordinator {
	c := &Coordinator{
		agents:      make(map[string]Agent),
		taskQueue:   NewTaskQueue(),
		memory:      NewMemory(),
		taskResults: make(map[string]*TaskResult),
		taskUpdates: make(map[string][]chan *TaskUpdate),
	}
	c.executor = NewTaskExecutor(c)
	c.planner = NewTaskPlanner(c)
	return c
}

// GetExecutor returns the task executor
func (c *Coordinator) GetExecutor() *TaskExecutor {
	return c.executor
}

// SetPlanner sets a custom task planner
func (c *Coordinator) SetPlanner(planner *TaskPlanner) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.planner = planner
}

// RegisterAgent registers a new agent with the coordinator
func (c *Coordinator) RegisterAgent(agent Agent) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := agent.GetID()
	if _, exists := c.agents[id]; exists {
		return fmt.Errorf("agent %s already registered", id)
	}

	c.agents[id] = agent
	log.Printf("[Coordinator] Registered agent: %s with capabilities: %v", id, agent.GetCapabilities())
	return nil
}

// SubmitTask submits a new task for execution
func (c *Coordinator) SubmitTask(ctx context.Context, task *Task) error {
	// Use planner to break down complex tasks
	subtasks, err := c.planner.Plan(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to plan task: %w", err)
	}

	// If no subtasks, execute the original task directly
	if len(subtasks) == 0 {
		log.Printf("[Coordinator] No subtasks created, executing original task %s directly", task.ID)
		if err := c.taskQueue.Add(task); err != nil {
			return fmt.Errorf("failed to queue task: %w", err)
		}
	} else {
		// Add subtasks to queue
		log.Printf("[Coordinator] Adding %d subtasks to queue", len(subtasks))
		for _, t := range subtasks {
			if err := c.taskQueue.Add(t); err != nil {
				return fmt.Errorf("failed to queue task: %w", err)
			}
		}
	}

	// Start execution
	go c.executor.Start(ctx)

	return nil
}

// ProcessUserRequest processes a natural language request
func (c *Coordinator) ProcessUserRequest(ctx context.Context, request string, context *celv1.RuleGenerationContext) (*celv1.ChatAssistResponse, error) {
	// Create a high-level task from the request
	task := &Task{
		ID:       generateTaskID(),
		Type:     TaskTypeRuleGeneration,
		Priority: 10,
		Input: map[string]interface{}{
			"message": request,
			"context": context,
		},
		Context:   make(map[string]interface{}),
		CreatedAt: time.Now(),
	}

	// Submit task and wait for result
	if err := c.SubmitTask(ctx, task); err != nil {
		return nil, err
	}

	// Wait for task completion (simplified - in real implementation, use channels)
	result, err := c.waitForTask(ctx, task.ID, 30*time.Second)
	if err != nil {
		return nil, err
	}

	// Convert result to ChatAssistResponse
	response, ok := result.Output.(*celv1.ChatAssistResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected result type")
	}

	return response, nil
}

// waitForTask waits for a task to complete
func (c *Coordinator) waitForTask(ctx context.Context, taskID string, timeout time.Duration) (*TaskResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("task timeout")
		case <-ticker.C:
			if result := c.executor.GetResult(taskID); result != nil {
				return result, nil
			}
		}
	}
}

// WaitForTask is the public version of waitForTask
func (c *Coordinator) WaitForTask(ctx context.Context, taskID string, timeout time.Duration) (*TaskResult, error) {
	return c.waitForTask(ctx, taskID, timeout)
}

// GetAgent finds an appropriate agent for a task
func (c *Coordinator) GetAgent(task *Task) Agent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	log.Printf("[Coordinator] Looking for agent for task type: %s", task.Type)
	log.Printf("[Coordinator] Registered agents: %d", len(c.agents))

	for agentID, agent := range c.agents {
		canHandle := agent.CanHandle(task)
		log.Printf("[Coordinator] Agent %s (%T) can handle: %v", agentID, agent, canHandle)
		if canHandle {
			return agent
		}
	}

	log.Printf("[Coordinator] ERROR: No agent found for task type %s", task.Type)
	return nil
}

// GetTaskResult retrieves a task result
func (c *Coordinator) GetTaskResult(taskID string) *TaskResult {
	c.updatesMu.RLock()
	defer c.updatesMu.RUnlock()
	return c.taskResults[taskID]
}

// SubscribeToTask subscribes to updates for a specific task
func (c *Coordinator) SubscribeToTask(taskID string, ch chan *TaskUpdate) {
	c.updatesMu.Lock()
	defer c.updatesMu.Unlock()

	if c.taskUpdates[taskID] == nil {
		c.taskUpdates[taskID] = make([]chan *TaskUpdate, 0)
	}
	c.taskUpdates[taskID] = append(c.taskUpdates[taskID], ch)
}

// UnsubscribeFromTask unsubscribes from task updates
func (c *Coordinator) UnsubscribeFromTask(taskID string, ch chan *TaskUpdate) {
	c.updatesMu.Lock()
	defer c.updatesMu.Unlock()

	channels := c.taskUpdates[taskID]
	for i, channel := range channels {
		if channel == ch {
			c.taskUpdates[taskID] = append(channels[:i], channels[i+1:]...)
			break
		}
	}
}

// PublishTaskUpdate publishes an update for a task
func (c *Coordinator) PublishTaskUpdate(update *TaskUpdate) {
	c.updatesMu.RLock()
	channels := c.taskUpdates[update.TaskID]
	c.updatesMu.RUnlock()

	for _, ch := range channels {
		select {
		case ch <- update:
		default:
			// Channel full, skip
		}
	}
}

// StoreTaskResult stores a task result
func (c *Coordinator) StoreTaskResult(taskID string, result *TaskResult) {
	c.updatesMu.Lock()
	c.taskResults[taskID] = result
	c.updatesMu.Unlock()

	// Publish completion update
	status := TaskStatusCompleted
	if !result.Success {
		status = TaskStatusFailed
	}

	c.PublishTaskUpdate(&TaskUpdate{
		TaskID:  taskID,
		Type:    UpdateTypeResult,
		Status:  status,
		Message: "Task completed",
		Data:    result,
		Time:    time.Now(),
	})
}

// CleanupTask removes all update channels for a completed task
func (c *Coordinator) CleanupTask(taskID string) {
	c.updatesMu.Lock()
	defer c.updatesMu.Unlock()

	// Remove channels for this task (don't close them as subscribers handle nil)
	delete(c.taskUpdates, taskID)

	// Also clean up stored results after some time
	// (keep them for a while in case of late subscribers)
	go func() {
		time.Sleep(5 * time.Minute)
		c.updatesMu.Lock()
		delete(c.taskResults, taskID)
		c.updatesMu.Unlock()
	}()
}

// Memory stores context and learning from past executions
type Memory struct {
	mu          sync.RWMutex
	taskHistory map[string]*TaskResult
	patterns    map[string]*Pattern
	knowledge   map[string]interface{}
}

// Pattern represents a learned pattern from past executions
type Pattern struct {
	ID          string
	Type        string
	Trigger     string
	Actions     []string
	SuccessRate float64
	LastUsed    time.Time
}

// NewMemory creates a new memory store
func NewMemory() *Memory {
	return &Memory{
		taskHistory: make(map[string]*TaskResult),
		patterns:    make(map[string]*Pattern),
		knowledge:   make(map[string]interface{}),
	}
}

// Store stores a task result in memory
func (m *Memory) Store(taskID string, result *TaskResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taskHistory[taskID] = result
}

// LearnPattern learns a new pattern from task execution
func (m *Memory) LearnPattern(pattern *Pattern) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patterns[pattern.ID] = pattern
}

// GetRelevantPatterns retrieves patterns relevant to a task
func (m *Memory) GetRelevantPatterns(task *Task) []*Pattern {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var relevant []*Pattern
	// Simple implementation - in reality, use ML/similarity matching
	for _, p := range m.patterns {
		if p.Type == string(task.Type) {
			relevant = append(relevant, p)
		}
	}

	return relevant
}

// GenerateTaskID generates a unique task ID
func GenerateTaskID() string {
	return generateTaskID()
}

func generateTaskID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}
