package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
)

// contextKey type for context values
type agentContextKey string

const streamChannelContextKey agentContextKey = "stream_channel"

// AIRuleGenerationAgent is an AI-powered rule generation agent
type AIRuleGenerationAgent struct {
	id        string
	llmClient LLMClient
	validator ValidationService // Add validation service
}

// NewAIRuleGenerationAgent creates a new AI-powered rule generation agent
func NewAIRuleGenerationAgent(llmClient LLMClient) *AIRuleGenerationAgent {
	return &AIRuleGenerationAgent{
		id:        "ai-rule-generation-agent",
		llmClient: llmClient,
	}
}

// SetValidator sets the validation service
func (a *AIRuleGenerationAgent) SetValidator(validator ValidationService) {
	a.validator = validator
}

// GetID returns the agent ID
func (a *AIRuleGenerationAgent) GetID() string {
	return a.id
}

// GetCapabilities returns the agent's capabilities
func (a *AIRuleGenerationAgent) GetCapabilities() []string {
	return []string{
		"cel_rule_generation",
		"rule_explanation",
		"test_case_generation",
		"rule_optimization",
		"compliance_mapping",
		"security_analysis",
		"performance_optimization",
		"multi_rule_composition",
		"context_aware_generation",
	}
}

// CanHandle checks if this agent can handle the given task
func (a *AIRuleGenerationAgent) CanHandle(task *Task) bool {
	// Use AI to determine if this agent can handle the task
	ctx := context.Background()

	canHandlePrompt := fmt.Sprintf(`Determine if a rule generation agent with these capabilities can handle this task:
Capabilities: %v
Task Type: %s
Task Input: %v
Task Context: %v

Respond with JSON: {"can_handle": true/false, "confidence": 0-1, "reason": "explanation"}

Example response:
{
  "can_handle": true,
  "confidence": 0.95,
  "reason": "I can generate CEL rules for Kubernetes resource validation"
}`,
		a.GetCapabilities(), task.Type, task.Input, task.Context)

	var response struct {
		CanHandle  bool    `json:"can_handle"`
		Confidence float64 `json:"confidence"`
		Reason     string  `json:"reason"`
	}

	err := a.llmClient.Analyze(ctx, canHandlePrompt, &response)
	if err != nil {
		log.Printf("[AIRuleGenerationAgent] Failed to determine capability: %v", err)
		return false
	}

	return response.CanHandle && response.Confidence > 0.7
}

// Execute executes the task
func (a *AIRuleGenerationAgent) Execute(ctx context.Context, task *Task) (*TaskResult, error) {
	log.Printf("[AIRuleGenerationAgent] Executing task %s of type %s", task.ID, task.Type)
	log.Printf("[AIRuleGenerationAgent] Task input: %+v", task.Input)
	log.Printf("[AIRuleGenerationAgent] Task context keys: %v", getMapKeys(task.Context))

	// Check if we have a stream channel for real-time updates
	var streamChan chan<- interface{}
	if ch, ok := task.Context["stream_channel"].(chan interface{}); ok {
		streamChan = ch
		log.Printf("[AIRuleGenerationAgent] Stream channel found in context")
	} else {
		log.Printf("[AIRuleGenerationAgent] No stream channel in context")
	}

	// Send initial thinking message
	a.sendThinking(streamChan, "🤖 AI Rule Agent activated...")

	// Analyze the task type and route accordingly
	switch task.Type {
	case TaskTypeRuleGeneration:
		return a.executeRuleGeneration(ctx, task, streamChan)
	case TaskTypeTestGeneration:
		return a.executeTestGeneration(ctx, task, streamChan)
	default:
		return &TaskResult{
			TaskID:  task.ID,
			Success: false,
			Error:   fmt.Errorf("unsupported task type: %s", task.Type),
		}, nil
	}
}

// sendThinking sends a thinking message if streaming is enabled
func (a *AIRuleGenerationAgent) sendThinking(streamChan chan<- interface{}, message string) {
	if streamChan != nil {
		// Create a properly formatted thinking message
		thinkingMsg := map[string]interface{}{
			"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
			"thinking": map[string]string{
				"message": message,
			},
		}
		select {
		case streamChan <- thinkingMsg:
			log.Printf("[AIRuleGenerationAgent] Sent thinking message: %s", message)
		default:
			// Channel full, skip
			log.Printf("[AIRuleGenerationAgent] Stream channel full, skipping thinking message")
		}
	}
}

// executeRuleGeneration handles rule generation tasks
func (a *AIRuleGenerationAgent) executeRuleGeneration(ctx context.Context, task *Task, streamChan chan<- interface{}) (*TaskResult, error) {
	// Extract input
	input, ok := task.Input.(map[string]interface{})
	if !ok {
		return &TaskResult{
			TaskID:  task.ID,
			Success: false,
			Error:   fmt.Errorf("invalid input format"),
		}, nil
	}

	// message, _ := input["message"].(string)
	intent, _ := input["intent"].(*Intent)

	// Extract enhanced intent context with research findings
	var enhancedIntent *EnhancedIntent
	var analyzedContext *AnalyzedContext
	if ei, ok := task.Context["enhanced_intent"]; ok {
		enhancedIntent, _ = ei.(*EnhancedIntent)
		log.Printf("[AIRuleGenerationAgent] Found enhanced intent with research: %v", enhancedIntent != nil)
		if enhancedIntent != nil {
			log.Printf("[AIRuleGenerationAgent] Enhanced intent requires research: %v, phase: %s", enhancedIntent.RequiresResearch, enhancedIntent.ResearchPhase)
		}
	}

	// Extract analyzed context with research findings
	if ac, ok := task.Context["analyzed_context"]; ok {
		analyzedContext, _ = ac.(*AnalyzedContext)
		log.Printf("[AIRuleGenerationAgent] Found analyzed context with research: %v", analyzedContext != nil)
		if analyzedContext != nil {
			log.Printf("[AIRuleGenerationAgent] Research context available with %d rule recommendations", len(analyzedContext.RecommendedRules))
			log.Printf("[AIRuleGenerationAgent] Documentation URLs: %v", analyzedContext.Context["documentation_urls"])
		}
	}

	// Extract rule context
	var ruleContext *celv1.RuleGenerationContext
	if rc, ok := input["rule_context"]; ok {
		ruleContext, _ = rc.(*celv1.RuleGenerationContext)
	}
	if ruleContext == nil {
		// Try to extract from nested context
		if ctxData, ok := input["context"]; ok {
			ruleContext, _ = ctxData.(*celv1.RuleGenerationContext)
		}
	}

	// Update task input with context for generateRuleWithAI
	if ruleContext != nil {
		input["context"] = ruleContext
	}

	// Send thinking updates
	a.sendThinking(streamChan, "📝 Understanding your requirements...")
	// found research context
	if analyzedContext != nil {
		a.sendThinking(streamChan, fmt.Sprintf("🔍 Found research context: %s", analyzedContext.Summary))
	}
	// found enhanced intent
	if intent != nil {
		a.sendThinking(streamChan, fmt.Sprintf("🎯 Intent: %s (confidence: %.2f), intent summary: %s", intent.PrimaryIntent, intent.Confidence, intent.IntentSummary))
	}

	// Use advanced AI generation
	a.sendThinking(streamChan, "🧠 Generating comprehensive CEL rule using AI...")

	// Extract requirements
	requirements := make(map[string]interface{})
	if intent != nil {
		requirements["intent_type"] = intent.PrimaryIntent
		requirements["confidence"] = intent.Confidence
		requirements["intent_summary"] = intent.IntentSummary
		requirements["metadata"] = intent.Metadata
	}
	if ruleContext != nil {
		requirements["resource_type"] = ruleContext.ResourceType
		requirements["api_version"] = ruleContext.ApiVersion
		requirements["namespace"] = ruleContext.Namespace
	}

	// Use the advanced generateRuleWithAI function with research context
	result, err := a.generateRuleWithAI(ctx, task, intent, requirements, enhancedIntent, analyzedContext)
	if err != nil {
		// failed to generate rule with AI, return error
		a.sendThinking(streamChan, "TaskID: "+task.ID+" failed to generate rule with AI, error: "+err.Error())
		return &TaskResult{
			TaskID:  task.ID,
			Success: false,
			Error:   err,
		}, err
	}

	// Advanced generation succeeded - just send success message
	if result.Success && result.Output != nil {
		a.sendThinking(streamChan, "✅ Comprehensive CEL rule generated successfully!")
	}

	return result, nil
}

// generateRuleWithAI generates rules using pure AI with research context
func (a *AIRuleGenerationAgent) generateRuleWithAI(ctx context.Context, task *Task, intent *Intent, requirements map[string]interface{}, enhancedIntent *EnhancedIntent, analyzedContext *AnalyzedContext) (*TaskResult, error) {
	input, _ := task.Input.(map[string]interface{})
	message, _ := input["message"].(string)
	ruleContext, _ := input["context"].(*celv1.RuleGenerationContext)

	// Add stream channel to context for LLM client if available
	if ch, ok := task.Context["stream_channel"].(chan interface{}); ok {
		// Add stream channel to context for LLM client
		ctx = context.WithValue(ctx, streamChannelContextKey, ch)
		log.Printf("[AIRuleGenerationAgent] Added stream channel to context for LLM client")
	} else {
		log.Printf("[AIRuleGenerationAgent] No stream channel found in task context")
	}

	// Build research context section if available
	researchSection := ""
	if analyzedContext != nil {
		researchSection = fmt.Sprintf(`

📚 RESEARCH FINDINGS:

Summary: %s

Key Requirements:
`, analyzedContext.Summary)
		for _, req := range analyzedContext.KeyRequirements {
			researchSection += fmt.Sprintf("- %s\n", req)
		}
		researchSection += "\nConstraints:\n"
		for _, constraint := range analyzedContext.Constraints {
			researchSection += fmt.Sprintf("- %s\n", constraint)
		}
		researchSection += "\nRecommended Rules from Documentation:\n"
		for i, rec := range analyzedContext.RecommendedRules {
			researchSection += fmt.Sprintf("%d. %s (Priority: %d)\n", i+1, rec.Description, rec.Priority)
			if rec.CELPattern != "" {
				researchSection += fmt.Sprintf("   Suggested CEL: %s\n", rec.CELPattern)
			}
			researchSection += fmt.Sprintf("   Rationale: %s\n", rec.Rationale)
		}
		if docUrls, ok := analyzedContext.Context["documentation_urls"].([]interface{}); ok {
			researchSection += "\nDocumentation Sources:\n"
			for _, url := range docUrls {
				if urlStr, ok := url.(string); ok {
					researchSection += fmt.Sprintf("- %s\n", urlStr)
				}
			}
		}
	}

	// Create comprehensive prompt for rule generation
	generatePrompt := fmt.Sprintf(`Generate a CEL (Common Expression Language) rule based on this analysis:

User Request: %s
Intent Analysis: %v
Requirements: %v
Resource Context: %v%s

🎯 STEP-BY-STEP GENERATION APPROACH:

Step 1: Analyze the request
- What resources are being validated?
- Is this a single-resource check or cross-resource validation?
- What compliance/security requirement is being enforced?

Step 2: Design the inputs
- Single input for rules checking one resource type
- Multiple inputs for cross-resource validation
- Use descriptive variable names (plural for K8s resources)

Step 3: Write the CEL expression
- Start with the main resource: resource.items.all(...)
- Add existence checks: has(field) before accessing
- For multi-input: use .exists() to check relationships

Step 4: Create test scenarios
- Valid case: all resources pass
- Invalid case: at least one fails
- Edge case: empty lists, missing fields
- For multi-input: test relationship scenarios

Generate a response with:
{
  "name": "a descriptive name for the rule (e.g., 'Pod Service Account Requirement', 'SSH Root Login Disabled')",
  "expression": "the CEL expression",
  "explanation": "detailed explanation of what the rule does",
  "variables": ["list", "of", "variables"],
  "inputs": [
    {
      "name": "variable name to use in CEL expression (e.g., var-p1, configData, sshConfig)",
      "type": "kubernetes|file|system|http",
      "spec": {
        // For kubernetes: {"group": "", "version": "v1", "resource": "pods", "namespace": "default"}
        // For file: {"path": "/etc/ssh/sshd_config", "format": "text"}
        // For system: {"command": "systemctl", "args": ["status", "sshd"]}
        // For http: {"url": "https://api.example.com/data", "method": "GET"}
      }
    }
  ],
  "complexity_score": 1-10,
  "security_implications": "security analysis",
  "performance_impact": "performance analysis",
  "test_scenarios": [
    {
      "name": "Valid pods with service accounts",
      "description": "All pods have non-empty service accounts",
      "should_pass": true,
      "example_data": [
        {
          "pods": {
            "items": [
              {"metadata": {"name": "pod1"}, "spec": {"serviceAccountName": "sa1"}},
              {"metadata": {"name": "pod2"}, "spec": {"serviceAccountName": "sa2"}}
            ]
          }
        }
      ]
    },
    {
      "name": "Invalid - pod with empty service account",
      "description": "One pod has empty service account name",
      "should_pass": false,
      "example_data": [
        {
          "pods": {
            "items": [
              {"metadata": {"name": "pod1"}, "spec": {"serviceAccountName": ""}}
            ]
          }
        }
      ]
    }
  ],
}
`, message, intent, requirements, ruleContext, researchSection)

	var generatedRule struct {
		Name                 string                   `json:"name"`
		Expression           string                   `json:"expression"`
		Explanation          string                   `json:"explanation"`
		Variables            []string                 `json:"variables"`
		Inputs               []RuleInputDefinition    `json:"inputs"`
		ComplexityScore      int                      `json:"complexity_score"`
		SecurityImplications string                   `json:"security_implications"`
		PerformanceImpact    string                   `json:"performance_impact"`
		TestScenarios        []map[string]interface{} `json:"test_scenarios"`
	}

	err := a.llmClient.Analyze(ctx, generatePrompt, &generatedRule)
	if err != nil {
		return nil, fmt.Errorf("failed to generate rule: %w", err)
	}

	// Convert inputs to protobuf format
	protoInputs := make([]*celv1.RuleInput, len(generatedRule.Inputs))
	usedVariables := make([]string, 0)

	for i, input := range generatedRule.Inputs {
		protoInput := &celv1.RuleInput{
			Name: input.Name, // This is the variable name used in CEL expression
		}
		usedVariables = append(usedVariables, input.Name)

		// Set the input type based on the spec
		switch input.Type {
		case "kubernetes":
			if spec, ok := input.Spec.(map[string]interface{}); ok {
				protoInput.InputType = &celv1.RuleInput_Kubernetes{
					Kubernetes: &celv1.KubernetesInput{
						Group:     getString(spec, "group"),
						Version:   getString(spec, "version"),
						Resource:  getStringWithFallback(spec, "resource", "resourceType"),
						Namespace: getString(spec, "namespace"),
					},
				}
			}
		case "file":
			if spec, ok := input.Spec.(map[string]interface{}); ok {
				protoInput.InputType = &celv1.RuleInput_File{
					File: &celv1.FileInput{
						Path:   getString(spec, "path"),
						Format: getString(spec, "format"),
					},
				}
			}
		// Note: System and HTTP types might not be defined in the current proto
		// You'll need to add them to the proto file if needed
		default:
			log.Printf("[AIRuleGenerationAgent] Unsupported input type: %s", input.Type)
			continue
		}

		protoInputs[i] = protoInput
	}

	// Validate the generated rule
	// 	validationResult, err := a.validateGeneratedRule(ctx, generatedRule.Expression, ruleContext, generatedRule)
	// 	if err != nil {
	// 		log.Printf("[AIRuleGenerationAgent] Rule validation failed: %v", err)

	// 		// Extract stream channel for sending updates
	// 		var streamChan chan<- interface{}
	// 		if ch, ok := task.Context["stream_channel"].(chan interface{}); ok {
	// 			streamChan = ch
	// 		}

	// 		// Self-correction: If validation failed, try to fix it
	// 		a.sendThinking(streamChan, "❌ Rule validation failed, attempting to self-correct...")

	// 		// Extract validation errors
	// 		validationErrors := []string{}
	// 		if syntaxErrors, ok := validationResult["syntax_errors"].([]string); ok {
	// 			validationErrors = append(validationErrors, syntaxErrors...)
	// 		}
	// 		if err != nil {
	// 			validationErrors = append(validationErrors, err.Error())
	// 		}

	// 		// Create self-correction prompt
	// 		correctionPrompt := fmt.Sprintf(`The previously generated CEL rule failed validation with these errors:
	// %v

	// Original rule:
	// {
	//   "name": "%s",
	//   "expression": "%s",
	//   "inputs": %v
	// }

	// Please fix the rule by:
	// 1. Ensuring all variables used in the expression are defined in the inputs array
	// 2. Making sure the input format matches the expected structure
	// 3. For kubernetes inputs, use the correct field name "resource" not "resourceType"
	// 4. Ensure the expression is syntactically valid CEL

	// Generate a corrected response with the same structure as before, ensuring all fields are present.`,
	// 			strings.Join(validationErrors, "\n"),
	// 			generatedRule.Name,
	// 			generatedRule.Expression,
	// 			generatedRule.Inputs)

	// 		// Try to generate a corrected rule
	// 		correctedResponse := &struct {
	// 			Name                 string                   `json:"name"`
	// 			Expression           string                   `json:"expression"`
	// 			Explanation          string                   `json:"explanation"`
	// 			Variables            []string                 `json:"variables"`
	// 			Inputs               []RuleInputDefinition    `json:"inputs"`
	// 			ComplexityScore      int                      `json:"complexity_score"`
	// 			SecurityImplications string                   `json:"security_implications"`
	// 			PerformanceImpact    string                   `json:"performance_impact"`
	// 			TestScenarios        []map[string]interface{} `json:"test_scenarios"`
	// 		}{}

	// 		if err := a.llmClient.Analyze(ctx, correctionPrompt, correctedResponse); err == nil {
	// 			// Use the corrected rule
	// 			generatedRule = *correctedResponse
	// 			a.sendThinking(streamChan, "✅ Rule corrected successfully!")

	// 			// Re-validate the corrected rule
	// 			validationResult, err = a.validateGeneratedRule(ctx, generatedRule.Expression, ruleContext, generatedRule)
	// 			if err != nil {
	// 				log.Printf("[AIRuleGenerationAgent] Corrected rule still failed validation: %v", err)
	// 			}
	// 		} else {
	// 			log.Printf("[AIRuleGenerationAgent] Failed to generate corrected rule: %v", err)
	// 		}
	// 	}

	// Create comprehensive response
	response := &celv1.ChatAssistResponse{
		Content: &celv1.ChatAssistResponse_Rule{
			Rule: &celv1.GeneratedRule{
				Name:            generatedRule.Name,
				Expression:      generatedRule.Expression,
				Explanation:     generatedRule.Explanation,
				Variables:       usedVariables, // Use actual variable names from inputs
				SuggestedInputs: protoInputs,
				TestCases:       []*celv1.RuleTestCase{}, // Initialize test cases
			},
		},
	}

	// Convert test scenarios to proper test cases
	if len(generatedRule.TestScenarios) > 0 {
		testCases := make([]*celv1.RuleTestCase, 0, len(generatedRule.TestScenarios))

		for i, scenario := range generatedRule.TestScenarios {
			// Extract test case information
			name := fmt.Sprintf("Test Case %d", i+1)
			if n, ok := scenario["name"].(string); ok {
				name = n
			}

			description := "Generated test case"
			if d, ok := scenario["description"].(string); ok {
				description = d
			}

			// Determine expected result
			expectedResult := true
			if exp, ok := scenario["expected"]; ok {
				switch v := exp.(type) {
				case bool:
					expectedResult = v
				case string:
					expectedResult = strings.ToLower(v) == "true" || v == "pass"
				}
			} else if shouldPass, ok := scenario["should_pass"]; ok {
				// Also check for should_pass field for backward compatibility
				switch v := shouldPass.(type) {
				case bool:
					expectedResult = v
				case string:
					expectedResult = strings.ToLower(v) == "true" || v == "pass"
				}
			}

			// Convert input to JSON string for test data
			testData := make(map[string]string)
			if input, ok := scenario["example_data"]; ok {
				if inputArray, ok := input.([]interface{}); ok {
					for _, inputData := range inputArray {
						if inputMap, ok := inputData.(map[string]interface{}); ok {
							// Each inputMap is like {"pods": {...}}
							for varName, varData := range inputMap {
								// Convert the data to JSON string
								if jsonBytes, err := json.Marshal(varData); err == nil {
									testData[varName] = string(jsonBytes)
								}
							}
						}
					}
				}
			}

			testCase := &celv1.RuleTestCase{
				Id:             fmt.Sprintf("test-%d", i+1),
				Name:           name,
				Description:    description,
				TestData:       testData,
				ExpectedResult: expectedResult,
				IsPassing:      false, // Will be determined when test is run
			}

			testCases = append(testCases, testCase)
		}

		// Add test cases to the rule
		response.Content.(*celv1.ChatAssistResponse_Rule).Rule.TestCases = testCases

		// Extract stream channel for sending updates
		var streamChan chan<- interface{}
		if ch, ok := task.Context["stream_channel"].(chan interface{}); ok {
			streamChan = ch
		}

		a.sendThinking(streamChan, fmt.Sprintf("🧪 Generated %d test cases for the rule", len(testCases)))

		// Execute test cases to verify the rule works correctly
		if a.validator != nil && len(testCases) > 0 {
			a.sendThinking(streamChan, "🧪 Running test cases to verify rule...")

			// Create a CEL rule object for validation
			celRule := &celv1.CELRule{
				Name:       generatedRule.Name,
				Expression: generatedRule.Expression,
				Inputs:     protoInputs, // Use the proto inputs we already created
				TestCases:  testCases,
			}

			// Try up to 3 times to get a rule that passes all tests
			maxRetries := 3
			for attempt := 1; attempt <= maxRetries; attempt++ {
				testResp, testErr := a.validator.ValidateRuleWithTestCases(ctx, celRule)
				if testErr != nil {
					log.Printf("[AIRuleGenerationAgent] Test execution failed: %v", testErr)
					a.sendThinking(streamChan, fmt.Sprintf("❌ Test execution failed: %v", testErr))
					break
				}

				if testResp != nil {
					// Check if all tests passed
					if testResp.AllPassed {
						a.sendThinking(streamChan, fmt.Sprintf("✅ All %d test cases passed!", len(testCases)))
						break
					} else {
						// Get details of failed tests
						failedTests := []string{}
						for _, result := range testResp.TestResults {
							if !result.Passed {
								failureMsg := fmt.Sprintf("Test '%s': %s", result.TestCaseId, result.Error)
								failedTests = append(failedTests, failureMsg)
								log.Printf("[AIRuleGenerationAgent] %s", failureMsg)
							}
						}

						// Try to fix if not last attempt
						if attempt < maxRetries {
							a.sendThinking(streamChan, fmt.Sprintf("⚠️  %d test case(s) failed - attempting to fix rule (attempt %d/%d)",
								len(failedTests), attempt, maxRetries))

							// Create correction prompt with test failures
							testFixPrompt := fmt.Sprintf(`The generated CEL rule is failing some test cases. Please fix the rule.

Current rule:
{
  "name": "%s",
  "expression": "%s",
  "inputs": %v
}

Test failures:
%s

Test scenarios that should work:
%v

Please generate a corrected rule that passes all test cases. Make sure the expression correctly handles all the test scenarios.
Generate a response with the same structure as before.`,
								generatedRule.Name,
								generatedRule.Expression,
								generatedRule.Inputs,
								strings.Join(failedTests, "\n"),
								generatedRule.TestScenarios)

							// Try to generate a corrected rule
							correctedResponse := &struct {
								Name                 string                   `json:"name"`
								Expression           string                   `json:"expression"`
								Explanation          string                   `json:"explanation"`
								Variables            []string                 `json:"variables"`
								Inputs               []RuleInputDefinition    `json:"inputs"`
								ComplexityScore      int                      `json:"complexity_score"`
								SecurityImplications string                   `json:"security_implications"`
								PerformanceImpact    string                   `json:"performance_impact"`
								TestScenarios        []map[string]interface{} `json:"test_scenarios"`
							}{}

							if err := a.llmClient.Analyze(ctx, testFixPrompt, correctedResponse); err == nil {
								// Update the rule with the corrected version
								generatedRule = *correctedResponse
								a.sendThinking(streamChan, "🔧 Rule updated based on test results")

								// Update the CEL rule object with new expression
								celRule.Expression = generatedRule.Expression

								// Update the response with new expression
								response.Content.(*celv1.ChatAssistResponse_Rule).Rule.Expression = generatedRule.Expression
								response.Content.(*celv1.ChatAssistResponse_Rule).Rule.Explanation = generatedRule.Explanation
							} else {
								log.Printf("[AIRuleGenerationAgent] Failed to generate corrected rule: %v", err)
								a.sendThinking(streamChan, "❌ Failed to generate corrected rule")
								break
							}
						} else {
							// Final attempt failed
							a.sendThinking(streamChan, fmt.Sprintf("❌ %d/%d test cases still failing after %d attempts",
								len(failedTests), len(testCases), maxRetries))
						}
					}
				}
			}
		}
	}

	// Get stream channel if available
	var streamChan chan<- interface{}
	if ch, ok := task.Context["stream_channel"].(chan interface{}); ok {
		streamChan = ch
	}

	// Send thinking message about test cases if any were generated
	if len(response.Content.(*celv1.ChatAssistResponse_Rule).Rule.TestCases) > 0 && streamChan != nil {
		a.sendThinking(streamChan, fmt.Sprintf("🧪 Generated %d test cases for the rule", len(response.Content.(*celv1.ChatAssistResponse_Rule).Rule.TestCases)))
	}

	// Build research context section for detailed analysis
	researchDetailsSection := ""
	if analyzedContext != nil {
		researchDetailsSection = fmt.Sprintf(`

📚 **Research Context:**

**Summary:** %s

**Key Requirements:**
%s

**Constraints:**
%s

**Rule Recommendations from Documentation:**
%s`,
			analyzedContext.Summary,
			formatList(analyzedContext.KeyRequirements),
			formatList(analyzedContext.Constraints),
			formatRuleRecommendations(analyzedContext.RecommendedRules),
		)

		// Add documentation URLs if available
		if docUrls, ok := analyzedContext.Context["documentation_urls"].([]interface{}); ok && len(docUrls) > 0 {
			researchDetailsSection += "\n\n**Documentation Sources:**\n"
			for _, url := range docUrls {
				if urlStr, ok := url.(string); ok {
					researchDetailsSection += fmt.Sprintf("- %s\n", urlStr)
				}
			}
		}
	}

	// Send detailed analysis as a text message
	if streamChan != nil {
		detailsText := fmt.Sprintf(`📋 **Rule Analysis Details**

**Complexity Score:** %d/10
**Security Implications:** %s
**Performance Impact:** %s

`,
			generatedRule.ComplexityScore,
			generatedRule.SecurityImplications,
			generatedRule.PerformanceImpact,
		)

		detailsMessage := &celv1.ChatAssistResponse{
			Content: &celv1.ChatAssistResponse_Text{
				Text: &celv1.TextMessage{
					Text: detailsText,
				},
			},
		}

		select {
		case streamChan <- detailsMessage:
			log.Printf("[AIRuleGenerationAgent] Detailed analysis sent to stream")
		default:
			log.Printf("[AIRuleGenerationAgent] Failed to send detailed analysis")
		}
	}

	// Determine next tasks based on AI analysis
	nextTasks := a.determineNextTasks(ctx, task, generatedRule)

	return &TaskResult{
		TaskID:  task.ID,
		Success: true,
		Output:  response,
		Logs: []string{
			fmt.Sprintf("Generated rule with complexity score: %d", generatedRule.ComplexityScore),
			fmt.Sprintf("Security implications: %s", generatedRule.SecurityImplications),
			fmt.Sprintf("Performance impact: %s", generatedRule.PerformanceImpact),
			fmt.Sprintf("Input variables: %v", usedVariables),
		},
		NextTasks: nextTasks,
	}, nil
}

// RuleInputDefinition represents an input definition from AI
type RuleInputDefinition struct {
	Name string      `json:"name"`
	Type string      `json:"type"`
	Spec interface{} `json:"spec"`
}

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func formatList(items []string) string {
	if len(items) == 0 {
		return "• None"
	}
	result := ""
	for _, item := range items {
		result += fmt.Sprintf("• %s\n", item)
	}
	return strings.TrimSpace(result)
}

// formatRuleRecommendations formats rule recommendations from research context for display
func formatRuleRecommendations(recommendations []RuleRecommendation) string {
	if len(recommendations) == 0 {
		return "None found"
	}

	var result []string
	for i, rec := range recommendations {
		recText := fmt.Sprintf("%d. **%s** (Priority: %d)\n   %s", i+1, rec.Description, rec.Priority, rec.Rationale)
		if rec.CELPattern != "" {
			recText += fmt.Sprintf("\n   Suggested CEL: `%s`", rec.CELPattern)
		}
		result = append(result, recText)
	}

	return strings.Join(result, "\n\n")
}

// executeTestGeneration handles test generation tasks
func (a *AIRuleGenerationAgent) executeTestGeneration(ctx context.Context, task *Task, streamChan chan<- interface{}) (*TaskResult, error) {
	a.sendThinking(streamChan, "🧪 Test generation not yet implemented")

	return &TaskResult{
		TaskID:  task.ID,
		Success: true,
		Output:  []*celv1.RuleTestCase{},
		Logs:    []string{"Test generation not yet implemented"},
	}, nil
}

// Helper functions
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getStringWithFallback(m map[string]interface{}, key string, fallback string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	if v, ok := m[fallback].(string); ok {
		return v
	}
	return ""
}

// determineNextTasks uses AI to determine follow-up tasks
func (a *AIRuleGenerationAgent) determineNextTasks(ctx context.Context, currentTask *Task, generatedRule interface{}) []*Task {
	nextTaskPrompt := fmt.Sprintf(`Based on this rule generation result, what follow-up tasks should be executed?
Current Task: %v
Generated Rule: %v

Consider:
- Test case generation needs
- Validation requirements
- Optimization opportunities

Return a JSON array of task suggestions:
[
  {
    "type": "test_generation",
    "priority": 9,
    "description": "Generate comprehensive test cases for the rule",
    "reason": "Ensure rule works correctly with various inputs"
  },
  {
    "type": "documentation",
    "priority": 7,
    "description": "Create documentation for the rule",
    "reason": "Help users understand how to use the rule"
  }
]`, currentTask, generatedRule)

	var taskSuggestions []struct {
		Type        string `json:"type"`
		Priority    int    `json:"priority"`
		Description string `json:"description"`
		Reason      string `json:"reason"`
	}

	err := a.llmClient.Analyze(ctx, nextTaskPrompt, &taskSuggestions)
	if err != nil {
		log.Printf("[AIRuleGenerationAgent] Failed to determine next tasks: %v", err)
		return nil
	}

	nextTasks := make([]*Task, 0)
	for _, suggestion := range taskSuggestions {
		nextTask := &Task{
			ID:       GenerateTaskID(),
			Type:     TaskType(suggestion.Type),
			Priority: suggestion.Priority,
			Input:    currentTask.Input,
			Context: map[string]interface{}{
				"parent_task": currentTask.ID,
				"description": suggestion.Description,
				"reason":      suggestion.Reason,
			},
			CreatedAt: currentTask.CreatedAt,
		}
		nextTasks = append(nextTasks, nextTask)
	}

	return nextTasks
}

// OptimizeRule uses AI to optimize an existing rule
func (a *AIRuleGenerationAgent) OptimizeRule(ctx context.Context, rule string, context map[string]interface{}) (string, error) {
	optimizePrompt := fmt.Sprintf(`Optimize this CEL rule for better performance and clarity:
Rule: %s
Context: %v

Provide:
- Optimized expression
- Explanation of changes
- Performance improvements
- Maintainability improvements

Return JSON response:
{
  "optimized_expression": "improved CEL expression",
  "changes": "Description of what was changed and why",
  "performance_gain": "Expected performance improvement",
  "maintainability": "How the changes improve maintainability"
}`, rule, context)

	var optimization struct {
		OptimizedExpression string `json:"optimized_expression"`
		Changes             string `json:"changes"`
		PerformanceGain     string `json:"performance_gain"`
		Maintainability     string `json:"maintainability"`
	}

	err := a.llmClient.Analyze(ctx, optimizePrompt, &optimization)
	if err != nil {
		return rule, err
	}

	return optimization.OptimizedExpression, nil
}

// Helper functions
func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

func getGroupFromApiVersion(apiVersion string) string {
	if apiVersion == "" || apiVersion == "v1" {
		return ""
	}
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}

func getVersionFromApiVersion(apiVersion string) string {
	if apiVersion == "" {
		return "v1"
	}
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 2 {
		return parts[1]
	}
	return apiVersion
}
