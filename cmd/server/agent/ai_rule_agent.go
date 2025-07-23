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
	case TaskTypeCELExpression:
		return a.executeCELExpressionGeneration(ctx, task, streamChan)
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

	message, _ := input["message"].(string)
	intent, _ := input["intent"].(*Intent)

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

	if intent != nil {
		a.sendThinking(streamChan, fmt.Sprintf("🎯 Intent: %s (confidence: %.2f)", intent.Type, intent.Confidence))
	}

	// Use advanced AI generation
	a.sendThinking(streamChan, "🧠 Generating comprehensive CEL rule using AI...")

	// Extract requirements
	requirements := make(map[string]interface{})
	if intent != nil {
		requirements["intent_type"] = intent.Type
		requirements["confidence"] = intent.Confidence
		requirements["entities"] = intent.Entities
	}
	if ruleContext != nil {
		requirements["resource_type"] = ruleContext.ResourceType
		requirements["api_version"] = ruleContext.ApiVersion
		requirements["namespace"] = ruleContext.Namespace
	}

	// Use the advanced generateRuleWithAI function
	result, err := a.generateRuleWithAI(ctx, task, intent, requirements)
	if err != nil {
		log.Printf("[AIRuleGenerationAgent] Advanced AI generation failed: %v", err)
		// Fallback to simple generation
		a.sendThinking(streamChan, "⚡ Using simplified generation as fallback...")

		expression, explanation, err := a.generateWithAI(ctx, message, ruleContext, intent)
		if err != nil {
			log.Printf("[AIRuleGenerationAgent] Simple AI generation also failed: %v", err)
			expression, explanation = a.generateFromPatterns(message, ruleContext)
		}

		// Create a basic result
		variables := a.extractVariables(expression)
		suggestedInputs := a.createSuggestedInputs(ruleContext, variables)

		response := &celv1.ChatAssistResponse{
			Content: &celv1.ChatAssistResponse_Rule{
				Rule: &celv1.GeneratedRule{
					Expression:      expression,
					Explanation:     explanation,
					Variables:       variables,
					SuggestedInputs: suggestedInputs,
				},
			},
		}

		// Just send success message, the result will be sent via task completion
		a.sendThinking(streamChan, "✅ Rule generated successfully!")

		return &TaskResult{
			TaskID:  task.ID,
			Success: true,
			Output:  response,
			Logs:    []string{"Rule generated using fallback method"},
		}, nil
	}

	// Advanced generation succeeded - just send success message
	if result.Success && result.Output != nil {
		a.sendThinking(streamChan, "✅ Comprehensive CEL rule generated successfully!")
	}

	return result, nil
}

// generateRuleWithAI generates rules using pure AI
func (a *AIRuleGenerationAgent) generateRuleWithAI(ctx context.Context, task *Task, intent *Intent, requirements map[string]interface{}) (*TaskResult, error) {
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

	// Create comprehensive prompt for rule generation
	generatePrompt := fmt.Sprintf(`Generate a CEL (Common Expression Language) rule based on this analysis:

User Request: %s
Intent Analysis: %v
Requirements: %v
Resource Context: %v

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
  "optimization_suggestions": ["list of optimizations"],
  "related_rules": ["other rules that might be relevant"],
  "compliance_mappings": {
    "framework": ["requirements"]
  }
}

📋 RULE GENERATION GUIDELINES - PLEASE READ CAREFULLY:

1️⃣ VARIABLE NAMING RULES:
- Each input MUST have a unique, descriptive variable name
- Use plural for Kubernetes resources: 'pods', 'services', 'configmaps'
- Use descriptive names for configs: 'sshConfig', 'nginxConfig'
- NEVER use generic names like 'resource' or 'input1'

2️⃣ SINGLE vs MULTI-INPUT RULES:
Single Input Example:
- Checking all pods have resource limits
- Input: pods
- Expression: pods.items.all(p, p.spec.containers.all(c, has(c.resources.limits)))

Multi-Input Example:
- Checking services match deployment selectors
- Inputs: services, deployments
- Expression: services.items.all(s, deployments.items.exists(d, d.metadata.name == s.metadata.name && d.spec.selector.matchLabels == s.spec.selector))

📚 CEL EXPRESSION EXAMPLES BY USE CASE:

🔸 SECURITY & COMPLIANCE CHECKS:

1. Service Account Requirements:
   • All pods must have service account: 
     pods.items.all(p, has(p.spec.serviceAccountName) && p.spec.serviceAccountName != "")
   • No default service account:
     pods.items.all(p, has(p.spec.serviceAccountName) && p.spec.serviceAccountName != "default")

2. Resource Limits:
   • CPU and memory limits required:
     pods.items.all(p, p.spec.containers.all(c, has(c.resources.limits.cpu) && has(c.resources.limits.memory)))
   • Memory limit at least 128Mi:
     pods.items.all(p, p.spec.containers.all(c, int(c.resources.limits.memory) >= 134217728))

3. Security Context:
   • Run as non-root:
     pods.items.all(p, has(p.spec.securityContext.runAsNonRoot) && p.spec.securityContext.runAsNonRoot == true)
   • No privileged containers:
     pods.items.all(p, p.spec.containers.all(c, !has(c.securityContext.privileged) || c.securityContext.privileged == false))

🔸 CONFIGURATION VALIDATION:

4. Labels and Annotations:
   • Required labels present:
     pods.items.all(p, has(p.metadata.labels.app) && has(p.metadata.labels.version))
   • Label format validation:
     deployments.items.all(d, d.metadata.labels.app.matches("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"))

5. Naming Conventions:
   • Resource naming standards:
     services.items.all(s, s.metadata.name.matches("^[a-z]{2,10}-[a-z]{2,10}$"))
   • No test/debug names in production:
     namespaces.items.all(ns, !ns.metadata.name.matches(".*test.*|.*debug.*|.*tmp.*"))

🔸 CROSS-RESOURCE VALIDATION:

6. Service-Deployment Matching:
   • Every service has matching deployment:
     services.items.all(s, deployments.items.exists(d, d.metadata.name == s.metadata.name && d.metadata.namespace == s.metadata.namespace))

7. ConfigMap References:
   • All referenced ConfigMaps exist:
     pods.items.all(p, !has(p.spec.volumes) || p.spec.volumes.all(v, !has(v.configMap) || configmaps.items.exists(cm, cm.metadata.name == v.configMap.name)))

8. NetworkPolicy Coverage:
   • All namespaces have NetworkPolicy:
     namespaces.items.all(ns, ns.metadata.name == "kube-system" || networkpolicies.items.exists(np, np.metadata.namespace == ns.metadata.name))

🔸 RESOURCE QUOTAS & LIMITS:

9. Replica Constraints:
   • Minimum replicas for HA:
     deployments.items.all(d, d.spec.replicas >= 2)
   • Maximum replicas limit:
     deployments.items.all(d, d.spec.replicas <= 10)

10. Storage Validation:
    • PVC size limits:
      persistentvolumeclaims.items.all(pvc, int(pvc.spec.resources.requests.storage) <= 100*1024*1024*1024)
    • Storage class specified:
      persistentvolumeclaims.items.all(pvc, has(pvc.spec.storageClassName) && pvc.spec.storageClassName != "")

🔸 ADVANCED PATTERNS:

11. Conditional Logic:
    • Different rules for different environments:
      pods.items.all(p, 
        (p.metadata.labels.env == "prod" && p.spec.containers.all(c, has(c.resources.limits))) ||
        (p.metadata.labels.env != "prod")
      )

12. Complex Field Navigation:
    • Check ingress TLS configuration:
      ingresses.items.all(i, i.spec.tls.all(t, t.hosts.all(h, h.matches("^[a-z0-9.-]+$"))))

13. Time-based Checks:
    • Resources not older than 30 days:
      pods.items.all(p, timestamp(p.metadata.creationTimestamp) > timestamp(now) - duration('720h'))

14. Aggregation Checks:
    • Total replicas across deployments:
      deployments.items.map(d, d.spec.replicas).sum() <= 100

IMPORTANT Kubernetes Data Structure:
- ALL Kubernetes resources come as: {"items": [resource1, resource2, ...]}
- ALWAYS access via: resourceVariable.items
- Empty result is: {"items": []}
- Single resource is still: {"items": [singleResource]}

📚 COMPLETE WORKING EXAMPLES:

🔹 SINGLE-INPUT RULES:

1. Pod Service Account Check:
   Inputs: [{"name": "pods", "type": "kubernetes", "spec": {"resource": "pods"}}]
   Expression: pods.items.all(p, has(p.spec.serviceAccountName) && p.spec.serviceAccountName != "")
   Test Pass: {"pods": {"items": [{"metadata": {"name": "pod1"}, "spec": {"serviceAccountName": "sa1"}}]}}
   Test Fail: {"pods": {"items": [{"metadata": {"name": "pod1"}, "spec": {"serviceAccountName": ""}}]}}

2. Container Resource Limits:
   Inputs: [{"name": "pods", "type": "kubernetes", "spec": {"resource": "pods"}}]
   Expression: pods.items.all(p, p.spec.containers.all(c, has(c.resources) && has(c.resources.limits) && has(c.resources.limits.cpu) && has(c.resources.limits.memory)))
   Test Pass: {"pods": {"items": [{"metadata": {"name": "pod1"}, "spec": {"containers": [{"name": "app", "resources": {"limits": {"cpu": "100m", "memory": "128Mi"}}}]}}]}}

🔹 MULTI-INPUT RULES:

3. Service-Deployment Matching:
   Inputs: [
     {"name": "services", "type": "kubernetes", "spec": {"resource": "services"}},
     {"name": "deployments", "type": "kubernetes", "spec": {"resource": "deployments"}}
   ]
   Expression: services.items.all(s, deployments.items.exists(d, d.metadata.name == s.metadata.name && has(d.spec.selector) && has(s.spec.selector) && d.spec.selector.matchLabels == s.spec.selector))
   Test Data: {
     "services": {"items": [{"metadata": {"name": "web"}, "spec": {"selector": {"app": "web"}}}]},
     "deployments": {"items": [{"metadata": {"name": "web"}, "spec": {"selector": {"matchLabels": {"app": "web"}}}}]}
   }

4. Pod-ConfigMap Reference:
   Inputs: [
     {"name": "pods", "type": "kubernetes", "spec": {"resource": "pods"}},
     {"name": "configmaps", "type": "kubernetes", "spec": {"resource": "configmaps"}}
   ]
   Expression: pods.items.all(p, !has(p.spec.volumes) || p.spec.volumes.all(v, !has(v.configMap) || configmaps.items.exists(cm, cm.metadata.name == v.configMap.name)))
   Test Data: {
     "pods": {"items": [{"metadata": {"name": "pod1"}, "spec": {"volumes": [{"name": "config", "configMap": {"name": "app-config"}}]}}]},
     "configmaps": {"items": [{"metadata": {"name": "app-config"}, "data": {"key": "value"}}]}
   }

5. NetworkPolicy Coverage:
   Inputs: [
     {"name": "namespaces", "type": "kubernetes", "spec": {"resource": "namespaces"}},
     {"name": "networkpolicies", "type": "kubernetes", "spec": {"resource": "networkpolicies"}}
   ]
   Expression: namespaces.items.all(ns, ns.metadata.name == "kube-system" || networkpolicies.items.exists(np, np.metadata.namespace == ns.metadata.name))
   Description: All non-system namespaces must have at least one NetworkPolicy

Common Functions Reference:
- has(field): Check if field exists
- size(list): Get list length
- type(value): Get value type
- string(value): Convert to string
- int(value): Convert to integer
- matches(string, regex): Regex matching
- timestamp(string): Parse timestamp
- duration(string): Parse duration

ALWAYS test edge cases:
- Empty list: {"items": []}
- Missing optional fields
- Empty string values
- Zero/negative numbers

📌 KEY CEL BEHAVIORS TO REMEMBER:
• Variables from inputs (like 'pods') are ALWAYS defined - no need to check existence
• .all() returns TRUE for empty lists - perfect for "all resources must..." rules
• .exists() returns FALSE for empty lists - use for "at least one resource must..."
• has() only checks object fields, not variables
• Use size(list) == 0 to explicitly check for empty lists

🔗 COMMON MULTI-INPUT PATTERNS:

1. Reference Validation (Pod → ConfigMap/Secret):
   pods.items.all(p, !has(p.spec.volumes) || p.spec.volumes.all(v, 
     (!has(v.configMap) || configmaps.items.exists(cm, cm.metadata.name == v.configMap.name)) &&
     (!has(v.secret) || secrets.items.exists(s, s.metadata.name == v.secret.name))
   ))

2. Label Matching (Service → Deployment):
   services.items.all(s, deployments.items.exists(d, 
     d.metadata.name == s.metadata.name && 
     d.spec.selector.matchLabels == s.spec.selector
   ))

3. Resource Quota Compliance:
   namespaces.items.all(ns, 
     ns.metadata.name == "kube-system" || 
     resourcequotas.items.exists(rq, rq.metadata.namespace == ns.metadata.name)
   )

4. Cross-Namespace Validation:
   ingresses.items.all(i, i.spec.rules.all(rule, 
     rule.http.paths.all(path, services.items.exists(s, 
       s.metadata.name == path.backend.service.name && 
       s.metadata.namespace == i.metadata.namespace
     ))
   ))
`, message, intent, requirements, ruleContext)

	var generatedRule struct {
		Name                    string                   `json:"name"`
		Expression              string                   `json:"expression"`
		Explanation             string                   `json:"explanation"`
		Variables               []string                 `json:"variables"`
		Inputs                  []RuleInputDefinition    `json:"inputs"`
		ComplexityScore         int                      `json:"complexity_score"`
		SecurityImplications    string                   `json:"security_implications"`
		PerformanceImpact       string                   `json:"performance_impact"`
		TestScenarios           []map[string]interface{} `json:"test_scenarios"`
		OptimizationSuggestions []string                 `json:"optimization_suggestions"`
		RelatedRules            []string                 `json:"related_rules"`
		ComplianceMappings      map[string][]string      `json:"compliance_mappings"`
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
	validationResult, err := a.validateGeneratedRule(ctx, generatedRule.Expression, ruleContext, generatedRule)
	if err != nil {
		log.Printf("[AIRuleGenerationAgent] Rule validation failed: %v", err)

		// Extract stream channel for sending updates
		var streamChan chan<- interface{}
		if ch, ok := task.Context["stream_channel"].(chan interface{}); ok {
			streamChan = ch
		}

		// Self-correction: If validation failed, try to fix it
		a.sendThinking(streamChan, "❌ Rule validation failed, attempting to self-correct...")

		// Extract validation errors
		validationErrors := []string{}
		if syntaxErrors, ok := validationResult["syntax_errors"].([]string); ok {
			validationErrors = append(validationErrors, syntaxErrors...)
		}
		if err != nil {
			validationErrors = append(validationErrors, err.Error())
		}

		// Create self-correction prompt
		correctionPrompt := fmt.Sprintf(`The previously generated CEL rule failed validation with these errors:
%v

Original rule:
{
  "name": "%s",
  "expression": "%s",
  "inputs": %v
}

Please fix the rule by:
1. Ensuring all variables used in the expression are defined in the inputs array
2. Making sure the input format matches the expected structure  
3. For kubernetes inputs, use the correct field name "resource" not "resourceType"
4. Ensure the expression is syntactically valid CEL

Generate a corrected response with the same structure as before, ensuring all fields are present.`,
			strings.Join(validationErrors, "\n"),
			generatedRule.Name,
			generatedRule.Expression,
			generatedRule.Inputs)

		// Try to generate a corrected rule
		correctedResponse := &struct {
			Name                    string                   `json:"name"`
			Expression              string                   `json:"expression"`
			Explanation             string                   `json:"explanation"`
			Variables               []string                 `json:"variables"`
			Inputs                  []RuleInputDefinition    `json:"inputs"`
			ComplexityScore         int                      `json:"complexity_score"`
			SecurityImplications    string                   `json:"security_implications"`
			PerformanceImpact       string                   `json:"performance_impact"`
			TestScenarios           []map[string]interface{} `json:"test_scenarios"`
			OptimizationSuggestions []string                 `json:"optimization_suggestions"`
			RelatedRules            []string                 `json:"related_rules"`
			ComplianceMappings      map[string][]string      `json:"compliance_mappings"`
		}{}

		if err := a.llmClient.Analyze(ctx, correctionPrompt, correctedResponse); err == nil {
			// Use the corrected rule
			generatedRule = *correctedResponse
			a.sendThinking(streamChan, "✅ Rule corrected successfully!")

			// Re-validate the corrected rule
			validationResult, err = a.validateGeneratedRule(ctx, generatedRule.Expression, ruleContext, generatedRule)
			if err != nil {
				log.Printf("[AIRuleGenerationAgent] Corrected rule still failed validation: %v", err)
			}
		} else {
			log.Printf("[AIRuleGenerationAgent] Failed to generate corrected rule: %v", err)
		}
	}

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
								Name                    string                   `json:"name"`
								Expression              string                   `json:"expression"`
								Explanation             string                   `json:"explanation"`
								Variables               []string                 `json:"variables"`
								Inputs                  []RuleInputDefinition    `json:"inputs"`
								ComplexityScore         int                      `json:"complexity_score"`
								SecurityImplications    string                   `json:"security_implications"`
								PerformanceImpact       string                   `json:"performance_impact"`
								TestScenarios           []map[string]interface{} `json:"test_scenarios"`
								OptimizationSuggestions []string                 `json:"optimization_suggestions"`
								RelatedRules            []string                 `json:"related_rules"`
								ComplianceMappings      map[string][]string      `json:"compliance_mappings"`
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

	// Send detailed analysis as a text message
	if streamChan != nil {
		detailsText := fmt.Sprintf(`📋 **Rule Analysis Details**

**Complexity Score:** %d/10
**Security Implications:** %s
**Performance Impact:** %s

**Optimization Suggestions:**
%s

**Related Rules:**
%s

**Compliance Mappings:**
%s`,
			generatedRule.ComplexityScore,
			generatedRule.SecurityImplications,
			generatedRule.PerformanceImpact,
			formatList(generatedRule.OptimizationSuggestions),
			formatList(generatedRule.RelatedRules),
			formatComplianceMappings(generatedRule.ComplianceMappings),
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
			fmt.Sprintf("Validation result: %v", validationResult),
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

// generateWithAI uses the LLM to generate a CEL rule
func (a *AIRuleGenerationAgent) generateWithAI(ctx context.Context, message string, ruleContext *celv1.RuleGenerationContext, intent *Intent) (string, string, error) {
	if a.llmClient == nil {
		return "", "", fmt.Errorf("LLM client not initialized")
	}

	prompt := a.buildGenerationPrompt(message, ruleContext, intent)

	var response struct {
		Expression  string                   `json:"expression"`
		Explanation string                   `json:"explanation"`
		Variables   []string                 `json:"variables"`
		Inputs      []map[string]interface{} `json:"inputs"`
	}

	err := a.llmClient.Analyze(ctx, prompt, &response)
	if err != nil {
		return "", "", err
	}

	return response.Expression, response.Explanation, nil
}

// Helper functions for safe context access
func getApiVersion(context *celv1.RuleGenerationContext) string {
	if context != nil && context.ApiVersion != "" {
		return context.ApiVersion
	}
	return "v1"
}

func getNamespace(context *celv1.RuleGenerationContext) string {
	if context != nil && context.Namespace != "" {
		return context.Namespace
	}
	return "default"
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

func formatComplianceMappings(mappings map[string][]string) string {
	if len(mappings) == 0 {
		return "• None"
	}
	result := ""
	for framework, controls := range mappings {
		result += fmt.Sprintf("• %s: %s\n", framework, strings.Join(controls, ", "))
	}
	return strings.TrimSpace(result)
}

// buildGenerationPrompt creates the LLM prompt for rule generation
func (a *AIRuleGenerationAgent) buildGenerationPrompt(message string, context *celv1.RuleGenerationContext, intent *Intent) string {
	resourceType := "resource"
	if context != nil && context.ResourceType != "" {
		resourceType = strings.ToLower(context.ResourceType) + "s"
	}

	prompt := fmt.Sprintf(`Generate a CEL (Common Expression Language) rule based on this request:
User Request: %s

Context:
- Resource Type: %s
- Kubernetes API Version: %s
- Namespace: %s

CRITICAL REQUIREMENTS:
1. Use descriptive variable names instead of generic 'resource'
2. For Pods, use variable name 'pods' (e.g., pods.items.all(...))
3. For Deployments, use 'deployments' (e.g., deployments.items.all(...))
4. For Services, use 'services', etc.
5. Always use the plural form of the resource type as the variable name
6. IMPORTANT: All Kubernetes resources are returned as lists with an 'items' field, even single resources like ClusterVersion

KUBERNETES RESOURCE STRUCTURE:
- ALL resources come as: {"items": [resource1, resource2, ...]}
- ALWAYS access via: variableName.items
- Empty result: {"items": []}
- Single resource: {"items": [singleResource]}

CORRECT Examples with Full Context:

1. Pod Service Account (your case):
   Variable: pods
   Expression: pods.items.all(p, has(p.spec.serviceAccountName) && p.spec.serviceAccountName != "")
   
2. Deployment Replicas:
   Variable: deployments  
   Expression: deployments.items.all(d, has(d.spec.replicas) && d.spec.replicas >= 2)
   
3. Container Resource Limits:
   Variable: pods
   Expression: pods.items.all(p, p.spec.containers.all(c, has(c.resources) && has(c.resources.limits) && has(c.resources.limits.memory)))

4. ConfigMap Required Keys:
   Variable: configmaps
   Expression: configmaps.items.all(cm, has(cm.data) && has(cm.data.required_key) && cm.data.required_key != "")

COMMON MISTAKES TO AVOID:
❌ pods.all(...) - Missing .items
❌ has(p.spec) && p.spec.field - Should be: has(p.spec) && has(p.spec.field)
❌ p.spec.has('field') - Should be: has(p.spec.field)
❌ field is string - Should be: type(field) == type("")
❌ resource.items.all(...) - Don't use generic 'resource'

Respond with JSON:
{
  "expression": "the CEL expression with proper variable names and .items access",
  "explanation": "detailed explanation of what the rule does",
  "variables": ["list", "of", "variables", "used"],
  "inputs": [
    {
      "name": "variable_name",
      "type": "kubernetes",
      "resource": "plural_resource_type"
    }
  ],
  "test_scenarios": [
    {
      "name": "All resources valid",
      "description": "Test when all resources meet the requirement",
      "should_pass": true,
      "example_data": [{
        "pods": {
          "items": [
            {"metadata": {"name": "pod1"}, "spec": {"serviceAccountName": "sa1"}},
            {"metadata": {"name": "pod2"}, "spec": {"serviceAccountName": "sa2"}}
          ]
        }
      }]
    },
    {
      "name": "Resource missing required field",
      "description": "Test when a resource fails the requirement",
      "should_pass": false,
      "example_data": [{
        "pods": {
          "items": [
            {"metadata": {"name": "pod1"}, "spec": {"serviceAccountName": ""}}
          ]
        }
      }]
    },
    {
      "name": "Empty list",
      "description": "Test with no resources (should pass for .all())",
      "should_pass": true,
      "example_data": [{
        "pods": {
          "items": []
        }
      }]
    }
  ]
}

TEST SCENARIO REQUIREMENTS:
1. ALWAYS wrap test data in {"items": [...]} structure
2. Include realistic Kubernetes resource fields
3. ALWAYS include these test cases:
   - Valid case where all resources pass (should_pass: true)
   - Invalid case where at least one resource fails (should_pass: false)
   - Empty list case {"items": []} (should_pass: true for .all(), false for .exists())
   - Edge case with missing optional fields
4. Use descriptive test names like "All pods have service accounts" not "Test 1"
5. The example_data is an array with one object per input variable

📝 TEST SCENARIO STRUCTURE:

For SINGLE-INPUT rules:
{
  "test_scenarios": [{
    "name": "All pods have service accounts",
    "should_pass": true,
    "example_data": [{
      "pods": {
        "items": [{"metadata": {"name": "pod1"}, "spec": {"serviceAccountName": "sa1"}}]
      }
    }]
  }]
}

For MULTI-INPUT rules:
{
  "test_scenarios": [{
    "name": "Services match deployments",
    "should_pass": true,
    "example_data": [{
      "services": {
        "items": [{"metadata": {"name": "web"}, "spec": {"selector": {"app": "web"}}}]
      },
      "deployments": {
        "items": [{"metadata": {"name": "web"}, "spec": {"selector": {"matchLabels": {"app": "web"}}}}]
      }
    }]
  }]
}

CRITICAL RULES:
1. example_data is an ARRAY with ONE OBJECT containing ALL inputs
2. Each input variable name MUST match the name in your inputs array
3. ALL data must be wrapped in {"items": [...]}
4. Include at least 3 test scenarios: valid case, invalid case, edge case`, message, resourceType, getApiVersion(context), getNamespace(context))

	if intent != nil {
		prompt += fmt.Sprintf("\n\nDetected Intent: %s\nEntities: %v", intent.Type, intent.Entities)
	}

	return prompt
}

// generateFromPatterns falls back to pattern-based generation
func (a *AIRuleGenerationAgent) generateFromPatterns(message string, context *celv1.RuleGenerationContext) (string, string) {
	lowerMessage := strings.ToLower(message)
	resourceType := "pods"
	if context != nil && context.ResourceType != "" {
		resourceType = strings.ToLower(context.ResourceType) + "s"
	}

	// Updated patterns with proper variable names
	patterns := []struct {
		keywords    []string
		expression  string
		explanation string
	}{
		{
			keywords:    []string{"resource limit", "resources", "limits"},
			expression:  fmt.Sprintf("%s.items.all(p, p.spec.containers.all(c, has(c.resources.limits.memory) && has(c.resources.limits.cpu)))", resourceType),
			explanation: "This rule ensures all containers have CPU and memory limits defined",
		},
		{
			keywords:    []string{"security context", "runasnonroot", "non-root"},
			expression:  fmt.Sprintf("%s.items.all(p, has(p.spec.securityContext) && p.spec.securityContext.runAsNonRoot == true)", resourceType),
			explanation: "This rule ensures pods run with non-root security context",
		},
		{
			keywords:    []string{"replica", "high availability", "ha"},
			expression:  "deployments.items.all(d, d.spec.replicas >= 2)",
			explanation: "This rule ensures deployments have at least 2 replicas for high availability",
		},
	}

	// Find matching pattern
	for _, pattern := range patterns {
		for _, keyword := range pattern.keywords {
			if strings.Contains(lowerMessage, keyword) {
				return pattern.expression, pattern.explanation
			}
		}
	}

	// Default rule with proper variable name
	return fmt.Sprintf("%s.items.all(item, has(item.metadata.name))", resourceType),
		"This is a basic rule that checks if all resources have a name"
}

// extractVariables extracts variable names from the expression
func (a *AIRuleGenerationAgent) extractVariables(expression string) []string {
	variables := []string{}

	// Look for common patterns
	words := strings.Fields(expression)
	for _, word := range words {
		// Check if it looks like a variable (starts with letter, contains only alphanumeric)
		if len(word) > 0 && (strings.HasSuffix(word, ".items") || strings.Contains(expression, word+".")) {
			varName := strings.TrimSuffix(word, ".items")
			if varName != "" && !contains(variables, varName) {
				variables = append(variables, varName)
			}
		}
	}

	// If no variables found, try to extract from the beginning
	if len(variables) == 0 && strings.Contains(expression, ".") {
		parts := strings.Split(expression, ".")
		if len(parts) > 0 {
			variables = append(variables, parts[0])
		}
	}

	return variables
}

// createSuggestedInputs creates input suggestions based on context
func (a *AIRuleGenerationAgent) createSuggestedInputs(context *celv1.RuleGenerationContext, variables []string) []*celv1.RuleInput {
	inputs := []*celv1.RuleInput{}

	if context == nil || len(variables) == 0 {
		return inputs
	}

	for _, varName := range variables {
		// Determine resource type from variable name
		resourceType := varName
		if strings.HasSuffix(varName, "s") {
			// It's already plural
		} else {
			resourceType = varName + "s"
		}

		input := &celv1.RuleInput{
			Name: varName,
			InputType: &celv1.RuleInput_Kubernetes{
				Kubernetes: &celv1.KubernetesInput{
					Group:     getGroupFromApiVersion(getApiVersion(context)),
					Version:   getVersionFromApiVersion(getApiVersion(context)),
					Resource:  resourceType,
					Namespace: getNamespace(context),
				},
			},
		}
		inputs = append(inputs, input)
	}

	return inputs
}

// generateTestCases generates test cases for the rule
func (a *AIRuleGenerationAgent) generateTestCases(ctx context.Context, expression string, resourceType string) []*celv1.RuleTestCase {
	// For now, return empty - this would use AI to generate test cases
	return []*celv1.RuleTestCase{}
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

// executeCELExpressionGeneration handles pure CEL expression generation without full rule context
func (a *AIRuleGenerationAgent) executeCELExpressionGeneration(ctx context.Context, task *Task, streamChan chan<- interface{}) (*TaskResult, error) {
	a.sendThinking(streamChan, "🔧 Generating CEL expression...")

	// Extract input
	input, ok := task.Input.(map[string]interface{})
	if !ok {
		return &TaskResult{
			TaskID:  task.ID,
			Success: false,
			Error:   fmt.Errorf("invalid input format"),
		}, nil
	}

	requirement, _ := input["requirement"].(string)
	resourceTypes, _ := input["resource_types"].([]string)

	// Add stream channel to context for LLM client if available
	if streamChan != nil {
		ctx = context.WithValue(ctx, streamChannelContextKey, streamChan)
	}

	// Generate focused CEL expression prompt
	prompt := fmt.Sprintf(`Generate a CEL expression for this specific requirement:

Requirement: %s
Resource Types: %v

Select the most appropriate CEL pattern from these categories:

%s

Generate ONLY the CEL expression that best matches the requirement. Consider:
1. Single vs multi-resource validation
2. Security and compliance implications
3. Performance considerations
4. Edge cases (empty lists, missing fields)

Return JSON with:
{
  "expression": "the CEL expression",
  "pattern_category": "which pattern category this fits",
  "inputs": [
    {
      "name": "variable_name",
      "type": "kubernetes",
      "spec": {
        "group": "",
        "version": "v1",
        "resource": "resourcetype"
      }
    }
  ],
  "explanation": "brief explanation of what this checks",
  "edge_cases": ["list of edge cases this handles"]
}`, requirement, resourceTypes, a.getCELPatternExamples())

	var result struct {
		Expression      string                `json:"expression"`
		PatternCategory string                `json:"pattern_category"`
		Inputs          []RuleInputDefinition `json:"inputs"`
		Explanation     string                `json:"explanation"`
		EdgeCases       []string              `json:"edge_cases"`
	}

	err := a.llmClient.Analyze(ctx, prompt, &result)
	if err != nil {
		// Fallback to pattern matching
		expression := a.generateExpressionFromPattern(requirement, resourceTypes)
		result.Expression = expression
		result.Explanation = "Generated from pattern matching"
	}

	// Create response
	response := &celv1.ChatAssistResponse{
		Content: &celv1.ChatAssistResponse_Text{
			Text: &celv1.TextMessage{
				Text: fmt.Sprintf("```cel\n%s\n```\n\n%s", result.Expression, result.Explanation),
				Type: "info",
			},
		},
	}

	return &TaskResult{
		TaskID:  task.ID,
		Success: true,
		Output:  response,
		Logs:    []string{fmt.Sprintf("Generated CEL expression for: %s", requirement)},
	}, nil
}

// getCELPatternExamples returns the categorized CEL expression examples
func (a *AIRuleGenerationAgent) getCELPatternExamples() string {
	return `🔸 SECURITY & COMPLIANCE CHECKS:
• Service accounts: pods.items.all(p, has(p.spec.serviceAccountName) && p.spec.serviceAccountName != "")
• Resource limits: pods.items.all(p, p.spec.containers.all(c, has(c.resources.limits.cpu) && has(c.resources.limits.memory)))
• Non-root: pods.items.all(p, has(p.spec.securityContext.runAsNonRoot) && p.spec.securityContext.runAsNonRoot == true)

🔸 CONFIGURATION VALIDATION:
• Required labels: pods.items.all(p, has(p.metadata.labels.app) && has(p.metadata.labels.version))
• Naming standards: services.items.all(s, s.metadata.name.matches("^[a-z]{2,10}-[a-z]{2,10}$"))

🔸 CROSS-RESOURCE VALIDATION:
• Service-Deployment: services.items.all(s, deployments.items.exists(d, d.metadata.name == s.metadata.name))
• ConfigMap refs: pods.items.all(p, !has(p.spec.volumes) || p.spec.volumes.all(v, !has(v.configMap) || configmaps.items.exists(cm, cm.metadata.name == v.configMap.name)))

🔸 RESOURCE QUOTAS:
• Min replicas: deployments.items.all(d, d.spec.replicas >= 2)
• Storage limits: persistentvolumeclaims.items.all(pvc, int(pvc.spec.resources.requests.storage) <= 100*1024*1024*1024)`
}

// generateExpressionFromPattern generates a CEL expression based on common patterns
func (a *AIRuleGenerationAgent) generateExpressionFromPattern(requirement string, resourceTypes []string) string {
	req := strings.ToLower(requirement)

	// Default to first resource type if provided
	resourceVar := "resources"
	if len(resourceTypes) > 0 {
		resourceVar = strings.ToLower(resourceTypes[0])
		if !strings.HasSuffix(resourceVar, "s") {
			resourceVar += "s"
		}
	}

	// Pattern matching for common requirements
	switch {
	case strings.Contains(req, "service account"):
		return fmt.Sprintf("%s.items.all(item, has(item.spec.serviceAccountName) && item.spec.serviceAccountName != \"\")", resourceVar)
	case strings.Contains(req, "resource limit"):
		return fmt.Sprintf("%s.items.all(item, item.spec.containers.all(c, has(c.resources.limits)))", resourceVar)
	case strings.Contains(req, "replica"):
		return "deployments.items.all(d, d.spec.replicas >= 2)"
	case strings.Contains(req, "label"):
		return fmt.Sprintf("%s.items.all(item, has(item.metadata.labels.app))", resourceVar)
	default:
		return fmt.Sprintf("%s.items.all(item, has(item.metadata.name))", resourceVar)
	}
}

// Helper functions
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getStringSlice(m map[string]interface{}, key string) []string {
	if v, ok := m[key].([]interface{}); ok {
		result := make([]string, len(v))
		for i, item := range v {
			if s, ok := item.(string); ok {
				result[i] = s
			}
		}
		return result
	}
	return nil
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

// validateGeneratedRule validates the AI-generated rule
func (a *AIRuleGenerationAgent) validateGeneratedRule(ctx context.Context, expression string, ruleContext *celv1.RuleGenerationContext, generatedRule struct {
	Name                    string                   `json:"name"`
	Expression              string                   `json:"expression"`
	Explanation             string                   `json:"explanation"`
	Variables               []string                 `json:"variables"`
	Inputs                  []RuleInputDefinition    `json:"inputs"`
	ComplexityScore         int                      `json:"complexity_score"`
	SecurityImplications    string                   `json:"security_implications"`
	PerformanceImpact       string                   `json:"performance_impact"`
	TestScenarios           []map[string]interface{} `json:"test_scenarios"`
	OptimizationSuggestions []string                 `json:"optimization_suggestions"`
	RelatedRules            []string                 `json:"related_rules"`
	ComplianceMappings      map[string][]string      `json:"compliance_mappings"`
}) (map[string]interface{}, error) {
	// First, try to compile the CEL expression locally
	syntaxErrors := []string{}
	securityIssues := []string{}
	performanceConcerns := []string{}

	// Try real validation if validator is available
	if a.validator != nil {
		// Check if this is a multi-input rule
		inputs := generatedRule.Inputs
		if len(inputs) > 1 {
			// For multi-input rules, we need to provide test data for each input
			testCase := &celv1.RuleTestCase{
				Id:             "syntax-check",
				Name:           "Syntax Validation",
				Description:    "Validate CEL expression syntax",
				TestData:       make(map[string]string),
				ExpectedResult: true,
			}

			// Create dummy data for each input
			for _, input := range inputs {
				switch input.Type {
				case "kubernetes":
					spec, _ := input.Spec.(map[string]interface{})
					resourceType, _ := spec["resourceType"].(string)
					// Create appropriate test data based on resource type
					switch resourceType {
					case "namespaces":
						testCase.TestData[input.Name] = `{"items":[{"metadata":{"name":"test-ns"}}]}`
					case "pods":
						testCase.TestData[input.Name] = `{"items":[{"metadata":{"name":"test-pod","namespace":"test-ns"}}]}`
					default:
						testCase.TestData[input.Name] = `{"items":[]}`
					}
				default:
					testCase.TestData[input.Name] = `{}`
				}
			}

			// Convert inputs to proto format for validation
			protoInputs := make([]*celv1.RuleInput, len(inputs))
			for i, input := range inputs {
				protoInput := &celv1.RuleInput{
					Name: input.Name,
				}

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
				default:
					// Skip non-kubernetes inputs for now
				}

				protoInputs[i] = protoInput
			}

			// Try to validate
			resp, err := a.validator.ValidateCEL(ctx, expression, protoInputs, []*celv1.RuleTestCase{testCase})
			if err != nil {
				syntaxErrors = append(syntaxErrors, fmt.Sprintf("Validation error: %v", err))
			} else if len(resp.Results) > 0 && resp.Results[0].Error != "" {
				syntaxErrors = append(syntaxErrors, resp.Results[0].Error)
			}
		} else {
			// Single input rule - use simple test case
			testCase := &celv1.RuleTestCase{
				Id:          "syntax-check",
				Name:        "Syntax Validation",
				Description: "Validate CEL expression syntax",
				TestData: map[string]string{
					"resource": `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test"},"spec":{}}`,
				},
				ExpectedResult: true,
			}

			// If we have input info, use the actual input name
			if len(inputs) == 1 {
				inputName := inputs[0].Name
				testCase.TestData = map[string]string{
					inputName: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test"},"spec":{}}`,
				}
			}

			// For single input validation, create a simple proto input
			var protoInputs []*celv1.RuleInput
			if len(inputs) == 1 {
				input := inputs[0]
				protoInput := &celv1.RuleInput{
					Name: input.Name,
				}

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
				}

				protoInputs = []*celv1.RuleInput{protoInput}
			}

			// Try to validate
			resp, err := a.validator.ValidateCEL(ctx, expression, protoInputs, []*celv1.RuleTestCase{testCase})
			if err != nil {
				syntaxErrors = append(syntaxErrors, fmt.Sprintf("Validation error: %v", err))
			} else if len(resp.Results) > 0 && resp.Results[0].Error != "" {
				syntaxErrors = append(syntaxErrors, resp.Results[0].Error)
			}
		}
	}

	// Basic syntax checks as fallback
	if strings.Count(expression, "(") != strings.Count(expression, ")") {
		syntaxErrors = append(syntaxErrors, "Mismatched parentheses")
	}
	if strings.Count(expression, "[") != strings.Count(expression, "]") {
		syntaxErrors = append(syntaxErrors, "Mismatched brackets")
	}
	if strings.Contains(expression, "resource.") && !strings.Contains(expression, "has(") {
		performanceConcerns = append(performanceConcerns, "Accessing nested fields without has() check may cause runtime errors")
	}

	// Use AI for comprehensive validation
	validatePrompt := fmt.Sprintf(`Validate this CEL expression for correctness and best practices:
Expression: %s
Context: %v
Known Syntax Errors: %v

Check for:
- Syntax correctness
- Logical consistency
- Security vulnerabilities
- Performance issues
- Best practice violations

Return JSON with validation results:
{
  "is_valid": true/false,
  "syntax_errors": ["error1", "error2"],
  "security_issues": ["issue1", "issue2"],
  "performance_concerns": ["concern1", "concern2"],
  "best_practices": ["violation1", "violation2"],
  "suggestions": ["improvement1", "improvement2"],
  "risk_score": 1-10
}`, expression, ruleContext, syntaxErrors)

	var aiValidation map[string]interface{}
	err := a.llmClient.Analyze(ctx, validatePrompt, &aiValidation)
	if err != nil {
		// If AI validation fails, use our basic validation
		return map[string]interface{}{
			"is_valid":             len(syntaxErrors) == 0,
			"syntax_errors":        syntaxErrors,
			"security_issues":      securityIssues,
			"performance_concerns": performanceConcerns,
			"best_practices":       []string{},
			"suggestions":          []string{},
			"risk_score":           5,
		}, nil
	}

	// Merge local and AI validation results
	if aiSyntaxErrors, ok := aiValidation["syntax_errors"].([]interface{}); ok && len(syntaxErrors) == 0 {
		for _, err := range aiSyntaxErrors {
			syntaxErrors = append(syntaxErrors, fmt.Sprintf("%v", err))
		}
	}

	// Update AI validation with our findings
	if len(syntaxErrors) > 0 {
		aiValidation["syntax_errors"] = syntaxErrors
		aiValidation["is_valid"] = false
	}

	return aiValidation, nil
}

// determineNextTasks uses AI to determine follow-up tasks
func (a *AIRuleGenerationAgent) determineNextTasks(ctx context.Context, currentTask *Task, generatedRule interface{}) []*Task {
	nextTaskPrompt := fmt.Sprintf(`Based on this rule generation result, what follow-up tasks should be executed?
Current Task: %v
Generated Rule: %v

Consider:
- Test case generation needs
- Validation requirements
- Documentation needs
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
