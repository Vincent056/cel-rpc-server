package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"strings"
	"time"

	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"

	"github.com/Vincent056/cel-rpc-server/cmd/server/agent"

	"connectrpc.com/connect"
)

// ChatHandler handles chat assistance requests
type ChatHandler struct {
	coordinator            *agent.Coordinator
	server                 *CELValidationServer
	llmClient              agent.LLMClient
	coordinator            *agent.Coordinator
	server                 *CELValidationServer
	llmClient              agent.LLMClient
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

	// Initialize AI provider
	aiProviderConfig := getAIProviderConfig()

	if aiProviderConfig.APIKey != "" || aiProviderConfig.Provider == "custom" {
		log.Printf("[ChatHandler] Initializing with AI provider: %s\n", aiProviderConfig.Provider)

		// Create AI provider
		aiProvider, err := NewAIProvider(aiProviderConfig)
		if err != nil {
			log.Printf("[ChatHandler] Failed to create AI provider: %v\n", err)
			// Fall back to pattern-based generation
		} else {
			// Create the appropriate LLM client based on the provider
			llmConfig := agent.LLMConfig{
				Provider:      aiProviderConfig.Provider,
				APIKey:        aiProviderConfig.APIKey,
				Endpoint:      aiProviderConfig.Endpoint,
				Model:         aiProviderConfig.Model,
				CustomHeaders: aiProviderConfig.CustomHeaders,
			}

			genericLLMClient, err := agent.NewGenericLLMClient(llmConfig)
			if err != nil {
				log.Printf("[ChatHandler] Failed to create generic LLM client: %v\n", err)
				// Fall back to pattern-based generation
			} else {
				handler.llmClient = genericLLMClient
			}

			// Only initialize AI components if we have a valid LLM client
			if handler.llmClient != nil {
				// Initialize enhanced intent analyzer
				var informationGatherer agent.InformationGatherer

				// Check if information gathering is enabled (default: enabled for backward compatibility)
				if os.Getenv("DISABLE_INFORMATION_GATHERING") != "true" {
					informationGatherer = agent.NewOpenAIWebSearchGatherer(handler.llmClient)
					log.Println("[ChatHandler] Information gathering enabled")
				} else {
					log.Println("[ChatHandler] Information gathering disabled")
				}

				handler.enhancedIntentAnalyzer = agent.NewEnhancedIntentAnalyzer(handler.llmClient, informationGatherer)
	// Initialize AI provider
	aiProviderConfig := getAIProviderConfig()

	if aiProviderConfig.APIKey != "" || aiProviderConfig.Provider == "custom" {
		log.Printf("[ChatHandler] Initializing with AI provider: %s\n", aiProviderConfig.Provider)

		// Create AI provider
		aiProvider, err := NewAIProvider(aiProviderConfig)
		if err != nil {
			log.Printf("[ChatHandler] Failed to create AI provider: %v\n", err)
			// Fall back to pattern-based generation
		} else {
			// Create the appropriate LLM client based on the provider
			llmConfig := agent.LLMConfig{
				Provider:      aiProviderConfig.Provider,
				APIKey:        aiProviderConfig.APIKey,
				Endpoint:      aiProviderConfig.Endpoint,
				Model:         aiProviderConfig.Model,
				CustomHeaders: aiProviderConfig.CustomHeaders,
			}

			genericLLMClient, err := agent.NewGenericLLMClient(llmConfig)
			if err != nil {
				log.Printf("[ChatHandler] Failed to create generic LLM client: %v\n", err)
				// Fall back to pattern-based generation
			} else {
				handler.llmClient = genericLLMClient
			}

			// Only initialize AI components if we have a valid LLM client
			if handler.llmClient != nil {
				// Initialize enhanced intent analyzer
				var informationGatherer agent.InformationGatherer

				// Check if information gathering is enabled (default: enabled for backward compatibility)
				if os.Getenv("DISABLE_INFORMATION_GATHERING") != "true" {
					informationGatherer = agent.NewOpenAIWebSearchGatherer(handler.llmClient)
					log.Println("[ChatHandler] Information gathering enabled")
				} else {
					log.Println("[ChatHandler] Information gathering disabled")
				}

				handler.enhancedIntentAnalyzer = agent.NewEnhancedIntentAnalyzer(handler.llmClient, informationGatherer)

				// Create validation service wrapper
				validationService := &ValidationServiceWrapper{server: server}
				// Create validation service wrapper
				validationService := &ValidationServiceWrapper{server: server}

				// Register AI-powered agents
				ruleAgent := agent.NewAIRuleGenerationAgent(handler.llmClient)
				ruleAgent.SetValidator(validationService)
				handler.coordinator.RegisterAgent(ruleAgent)
				// Register AI-powered agents
				ruleAgent := agent.NewAIRuleGenerationAgent(handler.llmClient)
				ruleAgent.SetValidator(validationService)
				handler.coordinator.RegisterAgent(ruleAgent)

				// Also register the basic rule generation agent with the new AI provider
				basicRuleAgent := agent.NewRuleGenerationAgent(aiProvider)
				basicRuleAgent.SetValidator(validationService)
				handler.coordinator.RegisterAgent(basicRuleAgent)
				// Also register the basic rule generation agent with the new AI provider
				basicRuleAgent := agent.NewRuleGenerationAgent(aiProvider)
				basicRuleAgent.SetValidator(validationService)
				handler.coordinator.RegisterAgent(basicRuleAgent)

				// Set up AI-powered planner
				aiPlanner := agent.NewAITaskPlanner(handler.coordinator, handler.llmClient)
				// Create a proper TaskPlanner instance using the constructor
				plannerWrapper := agent.NewTaskPlanner(handler.coordinator)
				// Register the AI planner for rule generation
				plannerWrapper.RegisterStrategy(agent.TaskTypeRuleGeneration, aiPlanner)
				handler.coordinator.SetPlanner(plannerWrapper)
			}
		}
				// Set up AI-powered planner
				aiPlanner := agent.NewAITaskPlanner(handler.coordinator, handler.llmClient)
				// Create a proper TaskPlanner instance using the constructor
				plannerWrapper := agent.NewTaskPlanner(handler.coordinator)
				// Register the AI planner for rule generation
				plannerWrapper.RegisterStrategy(agent.TaskTypeRuleGeneration, aiPlanner)
				handler.coordinator.SetPlanner(plannerWrapper)
			}
		}
	} else {
		// Register pattern-based agent as fallback
		log.Printf("[ChatHandler] No AI provider configured, using pattern-based generation")
		log.Printf("[ChatHandler] No AI provider configured, using pattern-based generation")
		// TODO: Register pattern-based agent
	}

	return handler
}

// getAIProviderConfig builds AI provider configuration from environment variables
func getAIProviderConfig() AIProviderConfig {
	config := AIProviderConfig{
		Provider: os.Getenv("AI_PROVIDER"), // "openai", "gemini", "custom"
	}

	// Default to OpenAI if not specified
	if config.Provider == "" {
		config.Provider = "openai"
	}

	// Get API key based on provider
	switch config.Provider {
	case "openai":
		config.APIKey = os.Getenv("OPENAI_API_KEY")
		config.Model = os.Getenv("OPENAI_MODEL")
		if config.Model == "" {
			config.Model = "gpt-4.1"
		}
	case "gemini":
		config.APIKey = os.Getenv("GEMINI_API_KEY")
		config.Model = os.Getenv("GEMINI_MODEL")
		if config.Model == "" {
			config.Model = "gemini-2.5-flash"
		}
	case "custom":
		config.APIKey = os.Getenv("CUSTOM_AI_API_KEY")
		config.Endpoint = os.Getenv("CUSTOM_AI_ENDPOINT")
		config.Model = os.Getenv("CUSTOM_AI_MODEL")

		// Parse custom headers if provided
		customHeaders := os.Getenv("CUSTOM_AI_HEADERS")
		if customHeaders != "" {
			config.CustomHeaders = make(map[string]string)
			// Format: "Header1:Value1,Header2:Value2"
			for _, header := range strings.Split(customHeaders, ",") {
				parts := strings.SplitN(header, ":", 2)
				if len(parts) == 2 {
					config.CustomHeaders[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
				}
			}
		}
	}

	return config
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

		// Check if this is a "needs more information" error
		if strings.Contains(err.Error(), "provide details") || strings.Contains(err.Error(), "more information") {
			// Send a clarification question instead of an error
			questionMsg := h.createQuestionMessage(
				err.Error(),
				[]string{
					"Create a Kubernetes pod security rule",
					"Check for resource limits in deployments",
					"Validate namespace has network policies",
					"Ensure containers run as non-root",
				},
				"I need more specific information to generate the right rule for you",
				celv1.QuestionType_QUESTION_TYPE_CLARIFICATION,
			)
			if sendErr := stream.Send(questionMsg); sendErr != nil {
				return sendErr
			}
			return nil
		}

		return h.sendError(stream, err)
	}
	log.Printf("[ChatHandler] Created task: ID=%s, Type=%s, Message=%s", task.ID, task.Type, req.Msg.Message)

	// Check if this is a clarification task
	if task.Type == "clarification_needed" {
		// Send clarification question directly
		clarificationMsg, _ := task.Context["clarification_message"].(string)
		if clarificationMsg == "" {
			clarificationMsg = "I need more information to help you. Could you please provide more details about what kind of rule you want to create?"
		}

		// Get information needs from intent analysis
		infoNeeds, _ := task.Context["information_needs"].([]string)

		// Determine suggested options based on the message and information needs
		var options []string

		// If we have specific information needs, create targeted suggestions
		if len(infoNeeds) > 0 {
			// Add a message about what information is needed
			clarificationMsg += "\n\nSpecifically, I need to know:"
			for _, need := range infoNeeds {
				clarificationMsg += "\n• " + need
			}
		}

		// Check what metadata we already have to provide better suggestions
		originalIntent, _ := task.Context["original_intent"].(*agent.Intent)
		var userPrompt string
		if originalIntent != nil && originalIntent.Metadata.UserInitialPrompt != "" {
			userPrompt = originalIntent.Metadata.UserInitialPrompt
		}

		// Provide targeted suggestions based on what's missing
		if userPrompt != "" {
			// Try to understand what the user might want based on their vague request
			if strings.Contains(strings.ToLower(userPrompt), "check") || strings.Contains(strings.ToLower(userPrompt), "validate") {
				clarificationMsg += "\n\nI see you want to check or validate something. Please specify:"
				clarificationMsg += "\n• What resource type (Pod, Deployment, Service, etc.)?"
				clarificationMsg += "\n• What condition should be checked?"
				clarificationMsg += "\n• What platform (Kubernetes, OpenShift, etc.)?"
			}
		}

		// Provide example suggestions based on common use cases
		if len(options) == 0 {
			options = []string{
				"Create a rule to ensure Kubernetes pods have resource limits defined",
				"Check if all deployments have at least 2 replicas for high availability",
				"Validate that pods don't run as root user for security",
				"Ensure all namespaces have network policies defined",
				"Check that containers don't use 'latest' image tags",
				"Verify pods use explicit service accounts (not default)",
				"Validate that persistent volumes have proper access modes",
				"Check if services have proper selectors defined",
			}
		}

		questionMsg := h.createQuestionMessage(
			clarificationMsg,
			options,
			"I need more specific information to generate the right rule for you",
			celv1.QuestionType_QUESTION_TYPE_CLARIFICATION,
		)

		if err := stream.Send(questionMsg); err != nil {
			return err
		}

		return nil
	}

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

			// Handle question messages from agents
			if question, ok := v["question"].(map[string]interface{}); ok {
				questionText, _ := question["text"].(string)
				options, _ := question["options"].([]string)
				context, _ := question["context"].(string)
				questionTypeStr, _ := question["type"].(string)

				// Map question type string to enum
				questionType := celv1.QuestionType_QUESTION_TYPE_UNSPECIFIED
				switch questionTypeStr {
				case "clarification":
					questionType = celv1.QuestionType_QUESTION_TYPE_CLARIFICATION
				case "confirmation":
					questionType = celv1.QuestionType_QUESTION_TYPE_CONFIRMATION
				case "choice":
					questionType = celv1.QuestionType_QUESTION_TYPE_CHOICE
				case "additional_info":
					questionType = celv1.QuestionType_QUESTION_TYPE_ADDITIONAL_INFO
				}

				if err := stream.Send(h.createQuestionMessage(questionText, options, context, questionType)); err != nil {
					return err
				}
			}

			// Handle clarification needed messages from agents
			if clarification, ok := v["clarification_needed"].(map[string]interface{}); ok {
				message, _ := clarification["message"].(string)
				options, _ := clarification["suggestions"].([]string)

				if message == "" {
					message = "I need more specific information to generate the rule you want. Could you provide more details?"
				}

				if err := stream.Send(h.createQuestionMessage(
					message,
					options,
					"Please provide more specific details about your requirements",
					celv1.QuestionType_QUESTION_TYPE_CLARIFICATION,
				)); err != nil {
					return err
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

	// Add conversation history to input if available
	if len(msg.PreviousMessages) > 0 {
		input["previous_messages"] = msg.PreviousMessages
	}

	// Add existing rule to input if available
	if msg.ExistingRule != nil {
		input["existing_rule"] = msg.ExistingRule
	}

	// Build context for intent analysis
	analysisContext := make(map[string]interface{})

	// Analyze conversation history for context
	if len(msg.PreviousMessages) > 0 {
		analysisContext["has_conversation_history"] = true
		analysisContext["previous_message_count"] = len(msg.PreviousMessages)

		// Check if the last message was a question from the assistant
		if len(msg.PreviousMessages) > 0 {
			lastMsg := msg.PreviousMessages[len(msg.PreviousMessages)-1]
			// Look for question indicators in the last assistant message
			if lastMsg.Role == "assistant" && strings.Contains(strings.ToLower(lastMsg.Content), "?") {
				// Check for clarification keywords
				clarificationKeywords := []string{"please specify", "need to know", "could you provide", "what kind of", "which resource", "what platform"}
				for _, keyword := range clarificationKeywords {
					if strings.Contains(strings.ToLower(lastMsg.Content), keyword) {
						analysisContext["is_answering_clarification"] = true
						analysisContext["previous_question"] = lastMsg.Content
						break
					}
				}
			}
		}

		// Look for recently generated rules in conversation
		var lastGeneratedRule *celv1.GeneratedRule
		for i := len(msg.PreviousMessages) - 1; i >= 0; i-- {
			prevMsg := msg.PreviousMessages[i]
			if prevMsg.Role == "assistant" && prevMsg.Rule != nil {
				lastGeneratedRule = prevMsg.Rule
				break
			}
		}

		if lastGeneratedRule != nil {
			analysisContext["has_previous_rule"] = true
			analysisContext["previous_rule_expression"] = lastGeneratedRule.Expression
			analysisContext["previous_rule_name"] = lastGeneratedRule.Name

			// Check if user is asking to modify the rule
			messageLower := strings.ToLower(msg.Message)
			modificationKeywords := []string{"change", "modify", "update", "fix", "improve", "regenerate", "instead", "but", "different", "adjust"}
			for _, keyword := range modificationKeywords {
				if strings.Contains(messageLower, keyword) {
					analysisContext["likely_modification_request"] = true
					break
				}
			}
		}

		// Add conversation summary for AI analysis
		var conversationSummary []string
		for _, prevMsg := range msg.PreviousMessages {
			summary := fmt.Sprintf("%s: %s", prevMsg.Role, prevMsg.Content)
			if len(summary) > 100 {
				summary = summary[:97] + "..."
			}
			conversationSummary = append(conversationSummary, summary)
		}
		analysisContext["conversation_summary"] = conversationSummary
	}

	// Check if an existing rule is provided for modification
	if msg.ExistingRule != nil {
		analysisContext["has_existing_rule"] = true
		analysisContext["existing_rule_id"] = msg.ExistingRule.Id
		analysisContext["existing_rule_name"] = msg.ExistingRule.Name
		analysisContext["existing_rule_expression"] = msg.ExistingRule.Expression
	}

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

	// Check if we need clarification based on multiple factors
	needsClarification := false
	clarificationMessage := ""

	// First check if this is an answer to a previous clarification question
	isAnsweringClarification, _ := analysisContext["is_answering_clarification"].(bool)
	if isAnsweringClarification {
		// Don't ask for clarification if user is answering our question
		// The intent analyzer should have more context now
		log.Printf("[ChatHandler] User is answering a clarification question, proceeding with intent: %s", enhancedIntent.Intent.PrimaryIntent)
	} else {
		// Check type is not_supported
		if enhancedIntent.Intent.PrimaryIntent == "not_supported" {
			needsClarification = true
			clarificationMessage = enhancedIntent.Intent.ErrorMessage
		}

		// Check for low confidence (below 0.5)
		if enhancedIntent.Intent.Confidence < 0.5 {
			needsClarification = true
			if clarificationMessage == "" {
				clarificationMessage = fmt.Sprintf("I'm not confident I understand your request (confidence: %.0f%%). %s",
					enhancedIntent.Intent.Confidence*100, enhancedIntent.Intent.IntentSummary)
			}
		}
	}

	// Check if intent summary indicates missing information
	summaryLower := strings.ToLower(enhancedIntent.Intent.IntentSummary)
	missingInfoKeywords := []string{"generic", "lacks context", "missing", "unclear", "vague", "insufficient", "need more", "too broad"}
	for _, keyword := range missingInfoKeywords {
		if strings.Contains(summaryLower, keyword) {
			needsClarification = true
			if clarificationMessage == "" {
				clarificationMessage = enhancedIntent.Intent.IntentSummary
			}
			break
		}
	}

	// Check if there are specific information needs
	if len(enhancedIntent.Intent.InformationNeeds) > 0 {
		needsClarification = true
		if clarificationMessage == "" {
			clarificationMessage = "To generate an accurate rule, I need more information."
		}
	}

	if needsClarification {
		// Create a special task that will ask for clarification
		task := &agent.Task{
			ID:        agent.GenerateTaskID(),
			Priority:  10,
			Context:   make(map[string]interface{}),
			CreatedAt: time.Now(),
			Type:      "clarification_needed",
		}

		// Add the error message and intent to context
		task.Context["needs_clarification"] = true
		task.Context["clarification_message"] = clarificationMessage
		task.Context["original_intent"] = enhancedIntent.Intent
		task.Context["information_needs"] = enhancedIntent.Intent.InformationNeeds
		task.Context["intent_confidence"] = enhancedIntent.Intent.Confidence
		task.Input = input

		return task, nil
	}

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
	task.Context["intent_summary"] = enhancedIntent.Intent.IntentSummary
	task.Context["user_provided_metadata"] = enhancedIntent.Intent.Metadata
	task.Context["intent_confidence"] = enhancedIntent.Intent.Confidence
	task.Context["intent_type"] = enhancedIntent.Intent.PrimaryIntent
	task.Context["requires_research"] = enhancedIntent.RequiresResearch
	task.Context["research_phase"] = enhancedIntent.ResearchPhase

	// Add conversation history to task context
	task.Context["previous_messages"] = msg.PreviousMessages
	task.Context["conversation_id"] = msg.ConversationId

	// If there's an existing rule, add it to the task context
	if msg.ExistingRule != nil {
		task.Context["existing_rule"] = msg.ExistingRule
	}

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
		enhancedIntent.Intent.PrimaryIntent, enhancedIntent.Intent.Confidence, enhancedIntent.RequiresResearch, enhancedIntent.ResearchPhase)

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
	switch intent.PrimaryIntent {
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
	switch intent.PrimaryIntent {
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

// createQuestionMessage creates a question message for AI to ask clarification
func (h *ChatHandler) createQuestionMessage(question string, options []string, context string, questionType celv1.QuestionType) *celv1.ChatAssistResponse {
	return &celv1.ChatAssistResponse{
		Content: &celv1.ChatAssistResponse_Question{
			Question: &celv1.QuestionMessage{
				Question: question,
				Options:  options,
				Context:  context,
				Type:     questionType,
			},
		},
		Timestamp: time.Now().Unix(),
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
			"• **Available Agents:** %s", agentTypesStr)
}
