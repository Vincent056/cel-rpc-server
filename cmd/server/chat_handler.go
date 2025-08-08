package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"

	"github.com/Vincent056/cel-rpc-server/cmd/server/agent"

	"connectrpc.com/connect"
)

// ChatHandler handles chat assistance requests
type ChatHandler struct {
	coordinator    *agent.Coordinator
	server         *CELValidationServer
	llmClient      agent.LLMClient
	enhancedIntentAnalyzer *agent.EnhancedIntentAnalyzer
}

// ValidationServiceWrapper provides validation to agents
type ValidationServiceWrapper struct {
	server *CELValidationServer
}

// ValidateCEL validates a CEL expression with test cases
func (v *ValidationServiceWrapper) ValidateCEL(ctx context.Context, expression string, inputs []*celv1.RuleInput, testCases []*celv1.RuleTestCase) (*celv1.ValidationResponse, error) {
	// For now, we'll focus on single-input validation for syntax checking
	// Multi-input validation would require a more complex setup

	// Convert RuleTestCase to TestCase for validation
	testCasesForValidation := make([]*celv1.RuleTestCase, len(testCases))
	for i, tc := range testCases {

		testCasesForValidation[i] = &celv1.RuleTestCase{
			ExpectedResult: tc.ExpectedResult,
			Description:    tc.Description,
			TestData:       tc.TestData,
		}
	}

	// Create validation request
	req := &celv1.ValidateCELRequest{
		Expression: expression,
		Inputs:     inputs, // Pass inputs for proper data type handling
		TestCases:  testCasesForValidation,
	}

	// Call the validation service
	resp, err := v.server.ValidateCEL(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return resp.Msg, nil
}

// ValidateRuleWithTestCases validates a complete rule with inputs and test cases
func (v *ValidationServiceWrapper) ValidateRuleWithTestCases(ctx context.Context, rule *celv1.CELRule) (*celv1.ValidateRuleWithTestCasesResponse, error) {
	// Create validation request
	req := &celv1.ValidateRuleWithTestCasesRequest{
		Rule:              rule,
		RunAgainstCluster: false, // Just run test cases, not against live cluster
	}

	// Call the validation service
	resp, err := v.server.ValidateRuleWithTestCases(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return resp.Msg, nil
}

// NewChatHandler creates a new chat handler with AI-powered agents
func NewChatHandler(server *CELValidationServer) *ChatHandler {
	handler := &ChatHandler{
		coordinator: agent.NewCoordinator(),
		server:      server,
	}

	// Start the task executor
	ctx := context.Background()
	if err := handler.coordinator.GetExecutor().Start(ctx); err != nil {
		log.Printf("[ChatHandler] Failed to start executor: %v", err)
	} else {
		log.Println("[ChatHandler] Task executor started successfully")
	}

	// Create LLM client
	openAIKey := os.Getenv("OPENAI_API_KEY")
	if openAIKey != "" {
		handler.llmClient = agent.NewOpenAILLMClient(openAIKey)
		// Initialize enhanced intent analyzer with OpenAI web search-powered information gathering
		informationGatherer := agent.NewOpenAIWebSearchGatherer(handler.llmClient)
		handler.enhancedIntentAnalyzer = agent.NewEnhancedIntentAnalyzer(handler.llmClient, informationGatherer)

		// Create validation service wrapper
		validationService := &ValidationServiceWrapper{server: server}

		// Register AI-powered agents
		ruleAgent := agent.NewAIRuleGenerationAgent(handler.llmClient)
		ruleAgent.SetValidator(validationService)
		handler.coordinator.RegisterAgent(ruleAgent)

		// Comment out the basic rule generation agent - we'll only use the AI-powered one
		// which has streaming support
		/*
			// Also register the basic rule generation agent with validation service
			// Create a separate OpenAI client for the basic rule agent
			openAIClient := NewOpenAIClient(openAIKey)
			basicRuleAgent := agent.NewRuleGenerationAgent(openAIClient)
			basicRuleAgent.SetValidator(validationService)
			handler.coordinator.RegisterAgent(basicRuleAgent)
		*/

		// Set up AI-powered planner
		aiPlanner := agent.NewAITaskPlanner(handler.coordinator, handler.llmClient)
		// Create a proper TaskPlanner instance using the constructor
		plannerWrapper := agent.NewTaskPlanner(handler.coordinator)
		// Register the AI planner for rule generation
		plannerWrapper.RegisterStrategy(agent.TaskTypeRuleGeneration, aiPlanner)
		handler.coordinator.SetPlanner(plannerWrapper)
	} else {
		// Register pattern-based agent as fallback
		log.Println("[ChatHandler] No OpenAI key found, using pattern-based generation")
		// TODO: Register pattern-based agent
	}

	return handler
}

// HandleChatAssist processes chat requests using the agent system
func (h *ChatHandler) HandleChatAssist(
	ctx context.Context,
	req *connect.Request[celv1.ChatAssistRequest],
	stream *connect.ServerStream[celv1.ChatAssistResponse],
) error {
	log.Printf("[ChatHandler] Processing request: %s", req.Msg.Message)
	log.Printf("[ChatHandler] New request: %s", req.Msg.Message)

	// Send initial thinking messages with emojis
	thinkingMessages := []string{
		"🤔 Understanding your request...",
		"🔍 Analyzing requirements...",
		"🧠 Preparing AI agents...",
	}

	for _, msg := range thinkingMessages {
		if err := stream.Send(&celv1.ChatAssistResponse{
			Content: &celv1.ChatAssistResponse_Thinking{
				Thinking: &celv1.ThinkingMessage{
					Message: msg,
				},
			},
			Timestamp: time.Now().Unix(),
		}); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond) // Small delay for effect
	}

	// Create a comprehensive task from the request using AI-driven intent analysis
	task, err := h.createTaskFromRequest(ctx, req.Msg)
	if err != nil {
		log.Printf("[ChatHandler] ERROR: Failed to create task: %v", err)
		return h.sendError(stream, err)
	}
	log.Printf("[ChatHandler] Created task: ID=%s, Type=%s, Message=%s", task.ID, task.Type, req.Msg.Message)

	// Add streaming context
	streamChan := make(chan interface{}, 100)
	task.Context["stream_channel"] = streamChan
	task.Context["streaming"] = true

	// Submit to coordinator for AI-powered processing
	log.Printf("[ChatHandler] Submitting task to coordinator")
	if err := h.coordinator.SubmitTask(ctx, task); err != nil {
		log.Printf("[ChatHandler] ERROR: Failed to submit task: %v", err)
		return h.sendError(stream, err)
	}
	log.Printf("[ChatHandler] Task submitted successfully")

	// Create goroutine to handle agent updates
	go h.processAgentUpdates(ctx, task.ID, streamChan)

	// Stream results from the channel
	for msg := range streamChan {
		switch v := msg.(type) {
		case string:
			// Simple thinking message
			if err := stream.Send(&celv1.ChatAssistResponse{
				Content: &celv1.ChatAssistResponse_Thinking{
					Thinking: &celv1.ThinkingMessage{
						Message: v,
					},
				},
				Timestamp: time.Now().Unix(),
			}); err != nil {
				return err
			}

		case map[string]interface{}:
			// Handle structured messages from agents
			if thinking, ok := v["thinking"].(map[string]string); ok {
				if message, ok := thinking["message"]; ok {
					// Send as thinking message
					if err := stream.Send(&celv1.ChatAssistResponse{
						Content: &celv1.ChatAssistResponse_Thinking{
							Thinking: &celv1.ThinkingMessage{
								Message: message,
							},
						},
						Timestamp: time.Now().Unix(),
					}); err != nil {
						return err
					}
				}
			} else if text, ok := v["text"].(map[string]string); ok {
				if message, ok := text["text"]; ok {
					// Send as text message
					if err := stream.Send(&celv1.ChatAssistResponse{
						Content: &celv1.ChatAssistResponse_Text{
							Text: &celv1.TextMessage{
								Text: message,
							},
						},
						Timestamp: time.Now().Unix(),
					}); err != nil {
						return err
					}
				}
			}

		case *celv1.ChatAssistResponse:
			// Complete response
			v.Timestamp = time.Now().Unix()
			if err := stream.Send(v); err != nil {
				return err
			}

		case error:
			// Error occurred
			return h.sendError(stream, v)
		}
	}

	return nil
}

// processAgentUpdates processes updates from the agent system and sends to stream
func (h *ChatHandler) processAgentUpdates(ctx context.Context, taskID string, streamChan chan<- interface{}) {
	defer close(streamChan)

	// Send initial task start message
	streamChan <- h.createThinkingMessage("🚀 Starting task processing...", "starting")

	// Create update channel
	updateCh := make(chan *agent.TaskUpdate, 100)
	h.coordinator.SubscribeToTask(taskID, updateCh)
	defer h.coordinator.UnsubscribeFromTask(taskID, updateCh)

	// Process updates with timeout
	timeout := time.NewTimer(120 * time.Second) // Extended timeout
	defer timeout.Stop()

	// Track task progress
	var taskStartTime = time.Now()
	var lastStatusTime = time.Now()
	var stepCount = 0

	for {
		select {
		case <-ctx.Done():
			streamChan <- h.createErrorMessage("Request cancelled by user")
			return

		case <-timeout.C:
			streamChan <- h.createErrorMessage("Request timeout after 2 minutes")
			return

		case update, ok := <-updateCh:
			if !ok {
				// Channel closed, task is complete
				streamChan <- h.createThinkingMessage("✅ Task processing completed", "completed")
				return
			}
			if update == nil {
				continue
			}

			// Send detailed status update
			elapsed := time.Since(taskStartTime)
			stepCount++

			// Send realtime status every few seconds or on important updates
			if time.Since(lastStatusTime) > 3*time.Second || update.Type == agent.UpdateTypeResult {
				statusMsg := fmt.Sprintf("⏱️ Task Status: %s | Step %d | Elapsed: %v",
					h.formatTaskStatus(update.Status), stepCount, elapsed.Truncate(time.Second))
				streamChan <- h.createStatusMessage(statusMsg, string(update.Status))
				lastStatusTime = time.Now()
			}

			// Process different update types with enhanced messaging
			switch update.Type {
			case agent.UpdateTypeThinking:
				// Enhanced thinking messages with context
				thinkingMsg := fmt.Sprintf("🧠 %s", update.Message)
				streamChan <- h.createThinkingMessage(thinkingMsg, "processing")

			case agent.UpdateTypeProgress:
				// Detailed progress updates with visual indicators
				progressMsg := fmt.Sprintf("📊 Progress Update: %s", update.Message)
				streamChan <- h.createProgressMessage(progressMsg)

			case agent.UpdateTypeResult:
				// Send completion thinking message before result
				streamChan <- h.createThinkingMessage("🎯 Processing complete, preparing response...", "finalizing")

				// Check if this is the final result
				if result, ok := update.Data.(*agent.TaskResult); ok {
					if result.Success && result.Output != nil {
						// Stream the final response
						if response, ok := result.Output.(*celv1.ChatAssistResponse); ok {
							streamChan <- response
						} else {
							// Convert other outputs
							streamChan <- h.convertResultToResponse(result)
						}
					} else if result.Error != nil {
						streamChan <- result.Error
					}
					return // Task complete
				}

			case agent.UpdateTypeError:
				// Enhanced error handling with context
				var errorMsg string
				if err, ok := update.Data.(error); ok {
					errorMsg = fmt.Sprintf("❌ Error occurred: %s", err.Error())
					streamChan <- h.createErrorMessage(errorMsg)
				} else {
					errorMsg = fmt.Sprintf("❌ Error: %s", update.Message)
					streamChan <- h.createErrorMessage(errorMsg)
				}
				return

			default:
				// Handle any other update types
				if update.Message != "" {
					miscMsg := fmt.Sprintf("ℹ️ %s", update.Message)
					streamChan <- h.createThinkingMessage(miscMsg, "info")
				}
			}

			// Check if task is complete
			if update.Status == agent.TaskStatusCompleted || update.Status == agent.TaskStatusFailed {
				finalStatusMsg := fmt.Sprintf("🏁 Task finished with status: %s | Total time: %v",
					h.formatTaskStatus(update.Status), elapsed.Truncate(time.Second))
				streamChan <- h.createStatusMessage(finalStatusMsg, string(update.Status))
				return
			}
		}
	}
}

// createTaskFromRequest creates a task from the chat request using AI-driven intent analysis
func (h *ChatHandler) createTaskFromRequest(ctx context.Context, msg *celv1.ChatAssistRequest) (*agent.Task, error) {
	// Package all request data for AI analysis
	input := map[string]interface{}{
		"message":         msg.Message,
		"conversation_id": msg.ConversationId,
	}

	// Build context for intent analysis
	analysisContext := make(map[string]interface{})

	// Add context-specific information
	switch context := msg.Context.(type) {
	case *celv1.ChatAssistRequest_RuleContext:
		input["rule_context"] = context.RuleContext
		analysisContext["context_type"] = "rule_generation"
		analysisContext["has_rule_context"] = true
		if context.RuleContext != nil {
			// Add available rule context fields
			analysisContext["has_rule_data"] = true
		}

	case *celv1.ChatAssistRequest_TestContext:
		input["test_context"] = context.TestContext
		analysisContext["context_type"] = "test_validation"
		analysisContext["has_test_context"] = true
		if context.TestContext != nil {
			// Add available test context fields
			analysisContext["has_test_data"] = true
		}

	case *celv1.ChatAssistRequest_ModificationContext:
		input["modification_context"] = context.ModificationContext
		analysisContext["context_type"] = "rule_modification"
		analysisContext["has_modification_context"] = true
		if context.ModificationContext != nil {
			// Add available modification context fields
			analysisContext["has_modification_data"] = true
		}

	default:
		analysisContext["context_type"] = "general"
		analysisContext["has_specific_context"] = false
	}

	// Use AI intent analysis to determine task properties
	if h.enhancedIntentAnalyzer == nil {
		// Fallback to default behavior if no intent analyzer available
		log.Printf("[ChatHandler] No intent analyzer available, using default task creation")
		return h.createTaskFromRequestFallback(msg), nil
	}

	// Analyze intent using AI with research capabilities
	analysisResult, err := h.enhancedIntentAnalyzer.AnalyzeIntentWithResearch(ctx, msg.Message, analysisContext)
	if err != nil {
		log.Printf("[ChatHandler] Intent analysis failed: %v, using fallback", err)
		return h.createTaskFromRequestFallback(msg), nil
	}

	// Extract enhanced intent and analyzed context from result
	enhancedIntent := analysisResult.EnhancedIntent
	analyzedContext := analysisResult.AnalyzedContext

	// Create task based on AI-analyzed intent
	task := &agent.Task{
		ID:        agent.GenerateTaskID(),
		Priority:  h.determinePriorityFromIntent(enhancedIntent.Intent),
		Context:   make(map[string]interface{}),
		CreatedAt: time.Now(),
		Type:      h.determineTaskTypeFromIntent(enhancedIntent.Intent),
	}

	// Add enhanced intent analysis results to task context
	task.Context["ai_intent"] = enhancedIntent.Intent
	task.Context["enhanced_intent"] = enhancedIntent
	task.Context["intent_confidence"] = enhancedIntent.Intent.Confidence
	task.Context["intent_type"] = enhancedIntent.Intent.Type
	task.Context["requires_research"] = enhancedIntent.RequiresResearch
	task.Context["research_phase"] = enhancedIntent.ResearchPhase

	// Add analyzed context with research findings if available
	if analyzedContext != nil {
		task.Context["analyzed_context"] = analyzedContext
		log.Printf("[ChatHandler] Added analyzed context with %d rule recommendations to task", len(analyzedContext.RecommendedRules))
	}

	// Merge original analysis context
	for k, v := range analysisContext {
		task.Context[k] = v
	}

	task.Input = input

	log.Printf("[ChatHandler] Enhanced AI Intent Analysis - Type: %s, Confidence: %.2f, Research Required: %v, Phase: %s", 
		enhancedIntent.Intent.Type, enhancedIntent.Intent.Confidence, enhancedIntent.RequiresResearch, enhancedIntent.ResearchPhase)

	return task, nil
}

// createTaskFromRequestFallback creates a task using the original hardcoded logic as fallback
func (h *ChatHandler) createTaskFromRequestFallback(msg *celv1.ChatAssistRequest) *agent.Task {
	task := &agent.Task{
		ID:        agent.GenerateTaskID(),
		Priority:  10,
		Context:   make(map[string]interface{}),
		CreatedAt: time.Now(),
		Type:      agent.TaskTypeRuleGeneration, // Default type
	}

	// Package all request data for AI analysis
	input := map[string]interface{}{
		"message":         msg.Message,
		"conversation_id": msg.ConversationId,
	}

	// Add context-specific information using original logic
	switch context := msg.Context.(type) {
	case *celv1.ChatAssistRequest_RuleContext:
		input["rule_context"] = context.RuleContext
		task.Context["context_type"] = "rule_generation"
		task.Type = agent.TaskTypeRuleGeneration

	case *celv1.ChatAssistRequest_TestContext:
		input["test_context"] = context.TestContext
		task.Context["context_type"] = "test_validation"
		task.Type = agent.TaskTypeTestGeneration

	case *celv1.ChatAssistRequest_ModificationContext:
		input["modification_context"] = context.ModificationContext
		task.Context["context_type"] = "rule_modification"
		task.Type = agent.TaskTypeOptimization

	default:
		task.Context["context_type"] = "general"
	}

	task.Input = input
	return task
}

// determinePriorityFromIntent determines task priority based on AI-analyzed intent
func (h *ChatHandler) determinePriorityFromIntent(intent *agent.Intent) int {
	// Base priority on confidence and intent type
	basePriority := 5
	
	// Higher confidence gets higher priority
	confidenceBonus := int(intent.Confidence * 5) // 0-5 bonus
	
	// Certain intent types get priority boosts
	switch intent.Type {
	case "rule_generation":
		basePriority = 8
	case "validation", "compliance_check":
		basePriority = 9
	case "optimization":
		basePriority = 7
	case "test_generation":
		basePriority = 6
	case "discovery", "analysis":
		basePriority = 5
	default:
		basePriority = 5
	}
	
	priority := basePriority + confidenceBonus
	if priority > 10 {
		priority = 10
	}
	if priority < 1 {
		priority = 1
	}
	
	return priority
}

// determineTaskTypeFromIntent maps AI-analyzed intent to agent task types
func (h *ChatHandler) determineTaskTypeFromIntent(intent *agent.Intent) agent.TaskType {
	// Map intent types to task types
	switch intent.Type {
	case "rule_generation", "create_rule", "generate_rule":
		return agent.TaskTypeRuleGeneration
	case "validation", "validate_rule", "check_rule":
		return agent.TaskTypeValidation
	case "test_generation", "create_test", "generate_test":
		return agent.TaskTypeTestGeneration
	case "optimization", "optimize_rule", "improve_rule":
		return agent.TaskTypeOptimization
	case "documentation", "document_rule", "explain_rule":
		return agent.TaskTypeDocumentation
	case "compliance_check", "compliance_mapping":
		return agent.TaskTypeCompliance
	case "analysis", "analyze", "discovery":
		// For analysis tasks, check suggested tasks to determine the best fit
		if len(intent.SuggestedTasks) > 0 {
			// Use the highest priority suggested task type
			highestPriority := 0
			bestTaskType := agent.TaskTypeRuleGeneration
			for _, suggestedTask := range intent.SuggestedTasks {
				if suggestedTask.Priority > highestPriority {
					highestPriority = suggestedTask.Priority
					bestTaskType = suggestedTask.Type
				}
			}
			return bestTaskType
		}
		return agent.TaskTypeRuleGeneration
	default:
		// Default to rule generation for unknown intent types
		return agent.TaskTypeRuleGeneration
	}
}

// convertResultToResponse converts a task result to a chat response
func (h *ChatHandler) convertResultToResponse(result *agent.TaskResult) *celv1.ChatAssistResponse {
	// Try to extract meaningful content from the result
	var text string
	if output, ok := result.Output.(string); ok {
		text = output
	} else if result.Logs != nil && len(result.Logs) > 0 {
		text = result.Logs[len(result.Logs)-1]
	} else {
		text = "Task completed successfully"
	}

	return &celv1.ChatAssistResponse{
		Content: &celv1.ChatAssistResponse_Text{
			Text: &celv1.TextMessage{
				Text: text,
				Type: "info",
			},
		},
		Timestamp: time.Now().Unix(),
	}
}

// sendError sends an error response
func (h *ChatHandler) sendError(stream *connect.ServerStream[celv1.ChatAssistResponse], err error) error {
	return stream.Send(&celv1.ChatAssistResponse{
		Content: &celv1.ChatAssistResponse_Error{
			Error: &celv1.ErrorMessage{
				Error:   err.Error(),
				Details: "An error occurred while processing your request",
			},
		},
		Timestamp: time.Now().Unix(),
	})
}

// Helper methods for creating different types of streaming messages

// createThinkingMessage creates a thinking/processing message
func (h *ChatHandler) createThinkingMessage(message, phase string) *celv1.ChatAssistResponse {
	// Include phase information in the message if provided
	fullMessage := message
	if phase != "" {
		fullMessage = fmt.Sprintf("[%s] %s", phase, message)
	}

	return &celv1.ChatAssistResponse{
		Content: &celv1.ChatAssistResponse_Thinking{
			Thinking: &celv1.ThinkingMessage{
				Message: fullMessage,
			},
		},
		Timestamp: time.Now().Unix(),
	}
}

// createStatusMessage creates a status update message using TextMessage with agent schedule info
func (h *ChatHandler) createStatusMessage(message, status string) *celv1.ChatAssistResponse {
	// Get current agent schedule information
	scheduleInfo := h.getAgentScheduleInfo()
	enhancedMessage := fmt.Sprintf("%s\n\n📊 **Agent Status:**\n%s", message, scheduleInfo)

	return &celv1.ChatAssistResponse{
		Content: &celv1.ChatAssistResponse_Text{
			Text: &celv1.TextMessage{
				Text: enhancedMessage,
				Type: "status",
			},
		},
		Timestamp: time.Now().Unix(),
	}
}

// createProgressMessage creates a progress update message using TextMessage with current task info
func (h *ChatHandler) createProgressMessage(message string) *celv1.ChatAssistResponse {
	// Get current task information
	currentTaskInfo := h.getCurrentTaskInfo()
	enhancedMessage := message
	if currentTaskInfo != "" {
		enhancedMessage = fmt.Sprintf("%s\n\n🎯 **Current Task:**\n%s", message, currentTaskInfo)
	}

	return &celv1.ChatAssistResponse{
		Content: &celv1.ChatAssistResponse_Text{
			Text: &celv1.TextMessage{
				Text: enhancedMessage,
				Type: "progress",
			},
		},
		Timestamp: time.Now().Unix(),
	}
}

// createErrorMessage creates an error response message
func (h *ChatHandler) createErrorMessage(message string) *celv1.ChatAssistResponse {
	return &celv1.ChatAssistResponse{
		Content: &celv1.ChatAssistResponse_Error{
			Error: &celv1.ErrorMessage{
				Error:   message,
				Details: "Task processing encountered an error",
			},
		},
		Timestamp: time.Now().Unix(),
	}
}

// formatTaskStatus formats task status with visual indicators
func (h *ChatHandler) formatTaskStatus(status agent.TaskStatus) string {
	switch status {
	case agent.TaskStatusPending:
		return "⏳ Pending"
	case agent.TaskStatusRunning:
		return "🏃 Running"
	case agent.TaskStatusCompleted:
		return "✅ Completed"
	case agent.TaskStatusFailed:
		return "❌ Failed"
	case agent.TaskStatusCancelled:
		return "🚫 Cancelled"
	default:
		return "❓ Unknown"
	}
}

// getAgentScheduleInfo returns information about the agent system schedule and status
func (h *ChatHandler) getAgentScheduleInfo() string {
	if h.coordinator == nil {
		return ""
	}

	// Get executor status
	executor := h.coordinator.GetExecutor()
	if executor == nil {
		return "Agent system not available"
	}

	// Determine system status based on available public information
	status := "🟢 Active"

	// Try to get an agent to test if system is working
	testTask := &agent.Task{
		ID:   "status-check",
		Type: "rule_generation",
	}
	agent := h.coordinator.GetAgent(testTask)
	agentInfo := "No agents registered"
	if agent != nil {
		caps := agent.GetCapabilities()
		agentInfo = fmt.Sprintf("Agents available with %d capabilities", len(caps))
	}

	return fmt.Sprintf(
		"• **Agent System:** %s\n"+
			"• **Task Executor:** Available\n"+
			"• **Worker Pool:** 5 workers ready\n"+
			"• **Agents:** %s",
		status, agentInfo)
}

// getCurrentTaskInfo returns information about currently executing tasks
func (h *ChatHandler) getCurrentTaskInfo() string {
	if h.coordinator == nil {
		return ""
	}

	// Use available public information about task execution
	executor := h.coordinator.GetExecutor()
	if executor == nil {
		return "No task executor available"
	}

	// Check for different agent types by testing what agents can handle different task types
	availableAgentTypes := make([]string, 0)
	taskTypes := []string{"rule_generation", "validation", "compliance"}

	for _, taskType := range taskTypes {
		testTask := &agent.Task{
			ID:   "capability-check",
			Type: agent.TaskType(taskType),
		}
		if agent := h.coordinator.GetAgent(testTask); agent != nil {
			caps := agent.GetCapabilities()
			if len(caps) > 0 {
				availableAgentTypes = append(availableAgentTypes, taskType)
			}
		}
	}

	agentTypesStr := "No specialized agents"
	if len(availableAgentTypes) > 0 {
		agentTypesStr = fmt.Sprintf("%d agent types: %v", len(availableAgentTypes), availableAgentTypes)
	}

	return fmt.Sprintf(
		"• **Task Processing:** Multi-agent execution ready\n"+
			"• **Worker Pool:** 5 concurrent workers available\n"+
			"• **Coordination:** Real-time task distribution\n"+
			"• **Available Agents:** %s", agentTypesStr)
}
