package agent

import (
	"context"
	"fmt"
	"strings"
)

// TaskPlanner plans and decomposes complex tasks into subtasks
type TaskPlanner struct {
	coordinator *Coordinator
	strategies  map[TaskType]PlanningStrategy
}

// PlanningStrategy defines how to plan for a specific task type
type PlanningStrategy interface {
	Plan(ctx context.Context, task *Task) ([]*Task, error)
}

// NewTaskPlanner creates a new task planner
func NewTaskPlanner(coordinator *Coordinator) *TaskPlanner {
	planner := &TaskPlanner{
		coordinator: coordinator,
		strategies:  make(map[TaskType]PlanningStrategy),
	}

	// Register default strategies
	planner.strategies[TaskTypeRuleGeneration] = &RuleGenerationStrategy{planner: planner}
	planner.strategies[TaskTypeValidation] = &ValidationStrategy{planner: planner}
	planner.strategies[TaskTypeCompliance] = &ComplianceStrategy{planner: planner}

	return planner
}

// Plan breaks down a task into subtasks
func (p *TaskPlanner) Plan(ctx context.Context, task *Task) ([]*Task, error) {
	// Check if there's a specific strategy for this task type
	if strategy, exists := p.strategies[task.Type]; exists {
		return strategy.Plan(ctx, task)
	}

	// Default: task doesn't need decomposition
	return []*Task{task}, nil
}

// RuleGenerationStrategy plans rule generation tasks
type RuleGenerationStrategy struct {
	planner *TaskPlanner
}

func (s *RuleGenerationStrategy) Plan(ctx context.Context, task *Task) ([]*Task, error) {
	subtasks := make([]*Task, 0)

	// Extract message from input
	input, ok := task.Input.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid input for rule generation")
	}

	message, _ := input["message"].(string)

	// Step 1: Understand intent
	intentTask := &Task{
		ID:           fmt.Sprintf("%s-intent", task.ID),
		Type:         TaskTypeRuleGeneration,
		Priority:     task.Priority,
		Input:        input,
		Context:      map[string]interface{}{"step": "analyze_intent"},
		Dependencies: []string{},
	}
	subtasks = append(subtasks, intentTask)

	// Step 2: Discover resources (if needed)
	if strings.Contains(strings.ToLower(message), "pod") ||
		strings.Contains(strings.ToLower(message), "deployment") ||
		strings.Contains(strings.ToLower(message), "service") {
		discoveryTask := &Task{
			ID:           fmt.Sprintf("%s-discovery", task.ID),
			Type:         TaskTypeResourceDiscovery,
			Priority:     task.Priority,
			Input:        input,
			Context:      map[string]interface{}{"parent": task.ID},
			Dependencies: []string{intentTask.ID},
		}
		subtasks = append(subtasks, discoveryTask)
	}

	// Step 3: Generate rule
	generateTask := &Task{
		ID:           fmt.Sprintf("%s-generate", task.ID),
		Type:         TaskTypeRuleGeneration,
		Priority:     task.Priority,
		Input:        input,
		Context:      map[string]interface{}{"step": "generate_rule"},
		Dependencies: []string{intentTask.ID},
	}
	subtasks = append(subtasks, generateTask)

	// Step 4: Validate rule
	validateTask := &Task{
		ID:           fmt.Sprintf("%s-validate", task.ID),
		Type:         TaskTypeValidation,
		Priority:     task.Priority,
		Input:        input,
		Context:      map[string]interface{}{"parent": task.ID},
		Dependencies: []string{generateTask.ID},
	}
	subtasks = append(subtasks, validateTask)

	// Step 5: Generate test cases
	testGenTask := &Task{
		ID:           fmt.Sprintf("%s-testgen", task.ID),
		Type:         TaskTypeTestGeneration,
		Priority:     task.Priority - 1,
		Input:        input,
		Context:      map[string]interface{}{"parent": task.ID},
		Dependencies: []string{generateTask.ID},
	}
	subtasks = append(subtasks, testGenTask)

	return subtasks, nil
}

// ValidationStrategy plans validation tasks
type ValidationStrategy struct {
	planner *TaskPlanner
}

func (s *ValidationStrategy) Plan(ctx context.Context, task *Task) ([]*Task, error) {
	// Validation might involve multiple steps
	subtasks := make([]*Task, 0)

	// Step 1: Syntax validation
	syntaxTask := &Task{
		ID:       fmt.Sprintf("%s-syntax", task.ID),
		Type:     TaskTypeValidation,
		Priority: task.Priority,
		Input:    task.Input,
		Context:  map[string]interface{}{"validation_type": "syntax"},
	}
	subtasks = append(subtasks, syntaxTask)

	// Step 2: Semantic validation
	semanticTask := &Task{
		ID:           fmt.Sprintf("%s-semantic", task.ID),
		Type:         TaskTypeValidation,
		Priority:     task.Priority,
		Input:        task.Input,
		Context:      map[string]interface{}{"validation_type": "semantic"},
		Dependencies: []string{syntaxTask.ID},
	}
	subtasks = append(subtasks, semanticTask)

	// Step 3: Runtime validation (if applicable)
	runtimeTask := &Task{
		ID:           fmt.Sprintf("%s-runtime", task.ID),
		Type:         TaskTypeValidation,
		Priority:     task.Priority - 1,
		Input:        task.Input,
		Context:      map[string]interface{}{"validation_type": "runtime"},
		Dependencies: []string{semanticTask.ID},
	}
	subtasks = append(subtasks, runtimeTask)

	return subtasks, nil
}

// ComplianceStrategy plans compliance check tasks
type ComplianceStrategy struct {
	planner *TaskPlanner
}

func (s *ComplianceStrategy) Plan(ctx context.Context, task *Task) ([]*Task, error) {
	subtasks := make([]*Task, 0)

	// Compliance checks typically involve multiple standards
	standards := []string{"cis", "nist", "pci", "hipaa"}

	for _, standard := range standards {
		checkTask := &Task{
			ID:       fmt.Sprintf("%s-%s", task.ID, standard),
			Type:     TaskTypeCompliance,
			Priority: task.Priority,
			Input:    task.Input,
			Context: map[string]interface{}{
				"standard": standard,
				"parent":   task.ID,
			},
		}
		subtasks = append(subtasks, checkTask)
	}

	// Add a summary task that depends on all checks
	summaryTask := &Task{
		ID:       fmt.Sprintf("%s-summary", task.ID),
		Type:     TaskTypeCompliance,
		Priority: task.Priority - 1,
		Input:    task.Input,
		Context:  map[string]interface{}{"step": "summarize"},
		Dependencies: func() []string {
			deps := make([]string, len(standards))
			for i, std := range standards {
				deps[i] = fmt.Sprintf("%s-%s", task.ID, std)
			}
			return deps
		}(),
	}
	subtasks = append(subtasks, summaryTask)

	return subtasks, nil
}

// RegisterStrategy registers a custom planning strategy
func (p *TaskPlanner) RegisterStrategy(taskType TaskType, strategy PlanningStrategy) {
	p.strategies[taskType] = strategy
}
