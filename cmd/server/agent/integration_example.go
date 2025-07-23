package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
)

// CompleteAgentSystem shows how all components connect together
type CompleteAgentSystem struct {
	// Core components
	coordinator *Coordinator
	queue       *TaskQueue
	executor    *TaskExecutor
	planner     *TaskPlanner
	memory      *Memory

	// AI components
	llmClient      LLMClient
	intentAnalyzer *IntentAnalyzer
	aiPlanner      *AITaskPlanner

	// Specialized agents
	ruleAgent       *AIRuleGenerationAgent
	validationAgent Agent
	discoveryAgent  Agent

	// Utilities
	ruleConverter *RuleConverter
}

// NewCompleteAgentSystem creates and connects all components
func NewCompleteAgentSystem(openAIKey string) (*CompleteAgentSystem, error) {
	system := &CompleteAgentSystem{}

	// 1. Create LLM client
	system.llmClient = NewOpenAILLMClient(openAIKey)

	// 2. Create core infrastructure
	system.coordinator = NewCoordinator()
	system.queue = system.coordinator.taskQueue
	system.executor = system.coordinator.executor
	system.memory = system.coordinator.memory

	// 3. Create AI components
	system.intentAnalyzer = NewIntentAnalyzer(system.llmClient)
	system.aiPlanner = NewAITaskPlanner(system.coordinator, system.llmClient)

	// 4. Create specialized agents
	system.ruleAgent = NewAIRuleGenerationAgent(system.llmClient)

	// 5. Register agents with coordinator
	if err := system.coordinator.RegisterAgent(system.ruleAgent); err != nil {
		return nil, err
	}

	// 6. Set up AI-powered planner
	system.planner = system.coordinator.planner
	// Override with AI planner for intelligent task decomposition
	system.planner.RegisterStrategy(TaskTypeRuleGeneration, system.aiPlanner)

	// 7. Create utility components
	system.ruleConverter = NewRuleConverter()

	// 8. Start executor workers
	ctx := context.Background()
	system.executor.Start(ctx)

	return system, nil
}

// ProcessUserRequest is the main entry point for user requests
func (s *CompleteAgentSystem) ProcessUserRequest(ctx context.Context, message string, ruleContext *celv1.RuleGenerationContext) (*celv1.ChatAssistResponse, error) {
	log.Printf("[AgentSystem] Processing request: %s", message)

	// Step 1: Analyze intent
	intent, err := s.intentAnalyzer.AnalyzeIntent(ctx, message, map[string]interface{}{
		"resourceType": ruleContext.ResourceType,
		"apiVersion":   ruleContext.ApiVersion,
		"namespace":    ruleContext.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("intent analysis failed: %w", err)
	}

	log.Printf("[AgentSystem] Analyzed intent: %+v", intent)

	// Step 2: Create master task based on intent
	masterTask := &Task{
		ID:       GenerateTaskID(),
		Type:     TaskType(intent.Type),
		Priority: 10,
		Input: map[string]interface{}{
			"message": message,
			"context": ruleContext,
			"intent":  intent,
		},
		Context: map[string]interface{}{
			"user_message": message,
			"intent":       intent,
			"streaming":    true,
		},
		CreatedAt: time.Now(),
	}

	// Step 3: Use AI planner to decompose into subtasks
	subtasks, err := s.aiPlanner.Plan(ctx, masterTask)
	if err != nil {
		return nil, fmt.Errorf("task planning failed: %w", err)
	}

	log.Printf("[AgentSystem] Generated %d subtasks", len(subtasks))

	// Step 4: Submit tasks to queue
	for _, task := range subtasks {
		if err := s.queue.Add(task); err != nil {
			log.Printf("[AgentSystem] Failed to queue task %s: %v", task.ID, err)
		}
	}

	// Step 5: Create result channel for streaming
	resultChan := make(chan *TaskUpdate, 100)
	s.coordinator.SubscribeToTask(masterTask.ID, resultChan)
	defer s.coordinator.UnsubscribeFromTask(masterTask.ID, resultChan)

	// Step 6: Process results and build response
	var finalResponse *celv1.ChatAssistResponse
	timeout := time.After(30 * time.Second)

	for {
		select {
		case update := <-resultChan:
			log.Printf("[AgentSystem] Task update: %s - %s", update.Type, update.Message)

			// Handle different update types
			switch update.Type {
			case UpdateTypeThinking:
				// Could stream thinking messages
				log.Printf("[AgentSystem] Thinking: %s", update.Message)

			case UpdateTypeProgress:
				// Could stream progress updates
				log.Printf("[AgentSystem] Progress: %s", update.Message)

			case UpdateTypeResult:
				// Check if this is the final result
				if result, ok := update.Data.(*TaskResult); ok && result.TaskID == masterTask.ID {
					if response, ok := result.Output.(*celv1.ChatAssistResponse); ok {
						finalResponse = response
						goto Done
					}
				}

			case UpdateTypeError:
				// Handle errors
				return nil, fmt.Errorf("task error: %s", update.Message)
			}

		case <-timeout:
			return nil, fmt.Errorf("request timeout")

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

Done:
	// Step 7: Store in memory for learning
	s.memory.Store(masterTask.ID, &TaskResult{
		TaskID:  masterTask.ID,
		Success: finalResponse != nil,
		Output:  finalResponse,
	})

	return finalResponse, nil
}

// Example: Complete workflow for rule generation
func ExampleCompleteWorkflow() {
	// Initialize system
	system, err := NewCompleteAgentSystem("your-openai-key")
	if err != nil {
		log.Fatal(err)
	}

	// Create context
	ctx := context.Background()

	// User request
	message := "Generate a rule to ensure all pods have resource limits"
	ruleContext := &celv1.RuleGenerationContext{
		ResourceType:   "Pod",
		ApiVersion:     "v1",
		Namespace:      "default",
		UseLiveCluster: true,
	}

	// Process request
	response, err := system.ProcessUserRequest(ctx, message, ruleContext)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	// Handle response
	switch content := response.Content.(type) {
	case *celv1.ChatAssistResponse_Rule:
		rule := content.Rule
		log.Printf("Generated Rule: %s", rule.Expression)
		log.Printf("Explanation: %s", rule.Explanation)

		// Convert to celscanner format if needed
		celRule := system.ruleConverter.ConvertToCelRule(rule, "generated-rule-1")
		log.Printf("Converted Rule ID: %s", celRule.Identifier())
	}
}

// StreamingExample shows how to handle streaming responses
func (s *CompleteAgentSystem) ProcessWithStreaming(
	ctx context.Context,
	message string,
	ruleContext *celv1.RuleGenerationContext,
	stream chan<- interface{},
) error {
	defer close(stream)

	// Step 1: Send initial thinking
	stream <- &celv1.ChatAssistResponse{
		Content: &celv1.ChatAssistResponse_Thinking{
			Thinking: &celv1.ThinkingMessage{
				Message: "🔍 Analyzing your request...",
			},
		},
	}

	// Step 2: Analyze intent
	intent, err := s.intentAnalyzer.AnalyzeIntent(ctx, message, nil)
	if err != nil {
		stream <- &celv1.ChatAssistResponse{
			Content: &celv1.ChatAssistResponse_Error{
				Error: &celv1.ErrorMessage{
					Error:   "Failed to understand request",
					Details: err.Error(),
				},
			},
		}
		return err
	}

	// Step 3: Create and execute task with streaming
	task := &Task{
		ID:       GenerateTaskID(),
		Type:     TaskTypeRuleGeneration,
		Priority: 10,
		Input: map[string]interface{}{
			"message": message,
			"context": ruleContext,
		},
		Context: map[string]interface{}{
			"stream":    stream,
			"streaming": true,
			"intent":    intent,
		},
		CreatedAt: time.Now(),
	}

	// Step 4: Subscribe to updates
	updateChan := make(chan *TaskUpdate, 100)
	s.coordinator.SubscribeToTask(task.ID, updateChan)
	defer s.coordinator.UnsubscribeFromTask(task.ID, updateChan)

	// Step 5: Submit task
	if err := s.coordinator.SubmitTask(ctx, task); err != nil {
		return err
	}

	// Step 6: Stream updates
	for update := range updateChan {
		switch update.Type {
		case UpdateTypeThinking:
			stream <- &celv1.ChatAssistResponse{
				Content: &celv1.ChatAssistResponse_Thinking{
					Thinking: &celv1.ThinkingMessage{
						Message: update.Message,
					},
				},
			}

		case UpdateTypeProgress:
			// Convert progress to appropriate message
			stream <- &celv1.ChatAssistResponse{
				Content: &celv1.ChatAssistResponse_Text{
					Text: &celv1.TextMessage{
						Text: update.Message,
						Type: "info",
					},
				},
			}

		case UpdateTypeResult:
			if result, ok := update.Data.(*TaskResult); ok {
				if response, ok := result.Output.(*celv1.ChatAssistResponse); ok {
					stream <- response
					return nil
				}
			}

		case UpdateTypeError:
			stream <- &celv1.ChatAssistResponse{
				Content: &celv1.ChatAssistResponse_Error{
					Error: &celv1.ErrorMessage{
						Error:   "Task failed",
						Details: update.Message,
					},
				},
			}
			return fmt.Errorf(update.Message)
		}
	}

	return nil
}

// AdvancedPlanningExample shows AI-powered task planning
func (s *CompleteAgentSystem) DemonstrateAIPlanning(ctx context.Context) {
	// Complex task requiring intelligent decomposition
	complexTask := &Task{
		ID:   GenerateTaskID(),
		Type: TaskTypeCompliance,
		Input: map[string]interface{}{
			"request": "Ensure all deployments in production are compliant with CIS benchmarks and have proper security controls",
		},
		Context: map[string]interface{}{
			"frameworks": []string{"cis", "nist"},
			"namespace":  "production",
		},
	}

	// AI planner will intelligently break this down
	subtasks, err := s.aiPlanner.Plan(ctx, complexTask)
	if err != nil {
		log.Printf("Planning failed: %v", err)
		return
	}

	// Show the intelligent plan
	log.Printf("AI Generated Plan:")
	for i, task := range subtasks {
		log.Printf("%d. %s (Priority: %d, Type: %s)",
			i+1, task.ID, task.Priority, task.Type)
		log.Printf("   Dependencies: %v", task.Dependencies)
	}

	// The AI planner might generate:
	// 1. Discovery task - Find all deployments
	// 2. Security scan task - Check security contexts
	// 3. Compliance check task - Verify CIS controls
	// 4. Validation task - Test configurations
	// 5. Report generation task - Summarize findings
}

// MemoryAndLearningExample shows how the system learns
func (s *CompleteAgentSystem) DemonstrateMemoryLearning(ctx context.Context) {
	// Execute similar tasks to build patterns
	for i := 0; i < 5; i++ {
		task := &Task{
			ID:   GenerateTaskID(),
			Type: TaskTypeRuleGeneration,
			Input: map[string]interface{}{
				"message": fmt.Sprintf("Generate security rule for pods - iteration %d", i),
			},
		}

		// Execute and store results
		result := &TaskResult{
			TaskID:  task.ID,
			Success: true,
			Output:  "pod security rule",
		}
		s.memory.Store(task.ID, result)
	}

	// System learns patterns
	pattern := &Pattern{
		ID:          "pod-security-pattern",
		Type:        "rule_generation",
		Trigger:     "security rule for pods",
		Actions:     []string{"analyze", "generate", "validate"},
		SuccessRate: 1.0,
		LastUsed:    time.Now(),
	}
	s.memory.LearnPattern(pattern)

	// Future similar requests will use learned patterns
	newTask := &Task{
		Type: TaskTypeRuleGeneration,
		Input: map[string]interface{}{
			"message": "Generate another security rule for pods",
		},
	}

	// Get relevant patterns
	patterns := s.memory.GetRelevantPatterns(newTask)
	log.Printf("Found %d relevant patterns for task", len(patterns))
}
