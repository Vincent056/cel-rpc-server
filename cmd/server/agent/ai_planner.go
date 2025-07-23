package agent

import (
	"context"
	"fmt"
	"log"
	"time"
)

// AITaskPlanner uses AI for intelligent task planning
type AITaskPlanner struct {
	coordinator    *Coordinator
	llmClient      LLMClient
	intentAnalyzer *IntentAnalyzer
}

// Ensure AITaskPlanner implements TaskPlanner
var _ PlanningStrategy = (*AITaskPlanner)(nil)

// NewAITaskPlanner creates a new AI-powered task planner
func NewAITaskPlanner(coordinator *Coordinator, llmClient LLMClient) *AITaskPlanner {
	return &AITaskPlanner{
		coordinator:    coordinator,
		llmClient:      llmClient,
		intentAnalyzer: NewIntentAnalyzer(llmClient),
	}
}

// Plan uses AI to break down complex tasks
func (p *AITaskPlanner) Plan(ctx context.Context, task *Task) ([]*Task, error) {
	// First, analyze the intent
	input, _ := task.Input.(map[string]interface{})
	intent, err := p.intentAnalyzer.AnalyzeIntent(ctx, fmt.Sprintf("%v", input), task.Context)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze intent: %w", err)
	}

	// Use AI to generate execution plan
	planPrompt := fmt.Sprintf(`Create a detailed execution plan for this task:
Task Type: %s
Intent: %v
Input: %v
Context: %v

Generate subtasks with the following considerations:
- Each subtask needs a unique identifier
- Define the task type from available types
- Provide a clear description of what the subtask does
- Set priority from 1-10 (10 being highest)
- List dependencies (array of task IDs)
- Estimate duration in seconds
- Specify required agent capabilities
- Define input transformations as key-value mappings

IMPORTANT:
- 'input_transformations' MUST be an object with string keys and string values, NOT an array
- Example: {"resource_type": "pod", "namespace": "default"}
- If no transformations needed, use empty object {}
- Do NOT use arrays like ["value1", "value2"] for input_transformations

Example correct subtask:
{
  "id": "task-1",
  "type": "analysis",
  "description": "Analyze Pod resource structure",
  "priority": 10,
  "dependencies": [],
  "estimated_duration": 30,
  "required_capabilities": ["kubernetes_analysis"],
  "input_transformations": {"source_field": "resource_type", "target_format": "lowercase"}
}

IMPORTANT RULES:
1. ONLY create subtasks if the main task CANNOT be handled directly
2. For rule_generation tasks, DO NOT create subtasks - it can be handled directly
3. Maximum 2 subtasks allowed
4. ONLY use these task types that have agents: rule_generation, analysis
5. DO NOT create tasks for: validation, simulation, error_handling, progress_checkpoint, deployment
6. If the task can be handled by a single agent, return empty subtasks array`, task.Type, intent, input, task.Context)

	var planResponse struct {
		Subtasks []struct {
			ID                   string                 `json:"id"`
			Type                 string                 `json:"type"`
			Description          string                 `json:"description"`
			Priority             int                    `json:"priority"`
			Dependencies         []string               `json:"dependencies"`
			EstimatedDuration    int                    `json:"estimated_duration"`
			RequiredCapabilities []string               `json:"required_capabilities"`
			InputTransformations map[string]interface{} `json:"input_transformations"`
		} `json:"subtasks"`
	}

	err = p.llmClient.Analyze(ctx, planPrompt, &planResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to generate execution plan: %w", err)
	}

	// Convert to tasks
	subtasks := make([]*Task, 0, len(planResponse.Subtasks))
	for _, taskData := range planResponse.Subtasks {
		// Create the context for the subtask, preserving stream_channel if it exists
		subtaskContext := map[string]interface{}{
			"parent_task":           task.ID,
			"description":           taskData.Description,
			"required_capabilities": taskData.RequiredCapabilities,
			"estimated_duration":    taskData.EstimatedDuration,
		}

		// Explicitly preserve stream_channel from parent task if it exists
		if streamChan, ok := task.Context["stream_channel"]; ok {
			subtaskContext["stream_channel"] = streamChan
			log.Printf("[AIPlanner] Preserving stream_channel for subtask %s", taskData.ID)
		} else {
			log.Printf("[AIPlanner] No stream_channel found in parent task context")
		}

		// Also preserve streaming flag
		if streaming, ok := task.Context["streaming"]; ok {
			subtaskContext["streaming"] = streaming
		}

		// Preserve context_type
		if contextType, ok := task.Context["context_type"]; ok {
			subtaskContext["context_type"] = contextType
		}

		subtask := &Task{
			ID:           fmt.Sprintf("%s-%s", task.ID, taskData.ID),
			Type:         TaskType(taskData.Type),
			Priority:     taskData.Priority,
			Input:        p.transformInput(input, taskData.InputTransformations),
			Context:      mergeMaps(task.Context, subtaskContext),
			Dependencies: taskData.Dependencies,
			CreatedAt:    time.Now(),
		}

		if taskData.EstimatedDuration > 0 {
			deadline := time.Now().Add(time.Duration(taskData.EstimatedDuration) * time.Second)
			subtask.Deadline = &deadline
		}

		subtasks = append(subtasks, subtask)
	}

	// Optimize execution plan
	optimized, err := p.optimizePlan(ctx, subtasks)
	if err != nil {
		// Log error but continue with original plan
		fmt.Printf("Failed to optimize plan: %v\n", err)
		return subtasks, nil
	}

	return optimized, nil
}

// optimizePlan uses AI to optimize the execution plan
func (p *AITaskPlanner) optimizePlan(ctx context.Context, tasks []*Task) ([]*Task, error) {
	// If no tasks, return empty list
	if len(tasks) == 0 {
		return tasks, nil
	}

	optimizePrompt := fmt.Sprintf(`Optimize this execution plan for efficiency:
%s

Consider:
- Parallel execution opportunities
- Resource utilization
- Minimizing total execution time
- Reducing inter-task communication overhead
- Load balancing across agents

Return ONLY a JSON array (not an object) with the optimized task list:
[
  {
    "id": "task-1",
    "priority": 10,
    "dependencies": []
  },
  {
    "id": "task-2",
    "priority": 9,
    "dependencies": ["task-1"]
  }
]

IMPORTANT: Return ONLY the array, do not wrap it in an object like {"tasks": [...]}`, tasks)

	var optimizedData []struct {
		ID           string   `json:"id"`
		Priority     int      `json:"priority"`
		Dependencies []string `json:"dependencies"`
	}

	err := p.llmClient.Analyze(ctx, optimizePrompt, &optimizedData)
	if err != nil {
		return tasks, err
	}

	// Apply optimizations
	taskMap := make(map[string]*Task)
	for _, task := range tasks {
		taskMap[task.ID] = task
	}

	for _, opt := range optimizedData {
		if task, exists := taskMap[opt.ID]; exists {
			task.Priority = opt.Priority
			task.Dependencies = opt.Dependencies
		}
	}

	return tasks, nil
}

// AdaptPlan dynamically adapts the plan based on runtime feedback
func (p *AITaskPlanner) AdaptPlan(ctx context.Context, originalPlan []*Task, feedback map[string]interface{}) ([]*Task, error) {
	adaptPrompt := fmt.Sprintf(`Adapt this execution plan based on runtime feedback:
Original Plan: %v
Feedback: %v

Consider:
- Failed tasks and their reasons
- Performance bottlenecks
- Resource constraints discovered
- New requirements or constraints

Generate an adapted plan that addresses the issues.`, originalPlan, feedback)

	var adaptedPlan []*Task
	err := p.llmClient.Analyze(ctx, adaptPrompt, &adaptedPlan)
	if err != nil {
		return nil, fmt.Errorf("failed to adapt plan: %w", err)
	}

	return adaptedPlan, nil
}

// PredictOutcome uses AI to predict task execution outcomes
func (p *AITaskPlanner) PredictOutcome(ctx context.Context, task *Task, historicalData map[string]interface{}) (map[string]interface{}, error) {
	predictPrompt := fmt.Sprintf(`Predict the outcome of this task execution:
Task: %v
Historical Data: %v

Provide predictions for:
- success_probability: 0-1
- estimated_duration: seconds
- potential_issues: array of potential problems
- resource_requirements: estimated resources needed
- recommended_optimizations: suggestions for better execution

Return JSON response:
{
  "success_probability": 0.95,
  "estimated_duration": 30,
  "potential_issues": [
    "API rate limiting might slow down execution",
    "Large dataset could exceed memory limits"
  ],
  "resource_requirements": {
    "memory": "512MB",
    "cpu": "0.5 cores",
    "network": "moderate bandwidth"
  },
  "recommended_optimizations": [
    "Batch API requests to avoid rate limits",
    "Process data in chunks to manage memory"
  ]
}`, task, historicalData)

	var prediction map[string]interface{}
	err := p.llmClient.Analyze(ctx, predictPrompt, &prediction)
	if err != nil {
		return nil, fmt.Errorf("failed to predict outcome: %w", err)
	}

	return prediction, nil
}

// transformInput transforms input based on AI-generated transformations
func (p *AITaskPlanner) transformInput(original map[string]interface{}, transformations map[string]interface{}) interface{} {
	// Apply transformations
	result := make(map[string]interface{})

	// Copy original
	for k, v := range original {
		result[k] = v
	}

	// Apply transformations
	for k, v := range transformations {
		switch transformation := v.(type) {
		case string:
			// Simple mapping
			if val, exists := original[transformation]; exists {
				result[k] = val
			}
		case map[string]interface{}:
			// Complex transformation
			result[k] = transformation
		default:
			result[k] = v
		}
	}

	return result
}

// mergeMaps merges two maps
func mergeMaps(m1, m2 map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m1 {
		result[k] = v
	}
	for k, v := range m2 {
		result[k] = v
	}
	return result
}
