package agent

import (
	"context"
	"fmt"
)

// IntentAnalyzer uses AI to understand user intent
type IntentAnalyzer struct {
	llmClient LLMClient
}

// LLMClient interface for language model interactions
type LLMClient interface {
	Analyze(ctx context.Context, prompt string, schema interface{}) error
}

// Intent represents analyzed user intent
type Intent struct {
	Type           string                 `json:"type"`
	Confidence     float64                `json:"confidence"`
	Entities       map[string]interface{} `json:"entities"`
	RequiredSteps  []string               `json:"required_steps"`
	Context        map[string]interface{} `json:"context"`
	SuggestedTasks []TaskSuggestion       `json:"suggested_tasks"`
}

// TaskSuggestion represents a suggested task based on intent
type TaskSuggestion struct {
	Type        TaskType               `json:"type"`
	Priority    int                    `json:"priority"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// NewIntentAnalyzer creates a new intent analyzer
func NewIntentAnalyzer(llmClient LLMClient) *IntentAnalyzer {
	return &IntentAnalyzer{
		llmClient: llmClient,
	}
}

// AnalyzeIntent uses AI to understand the user's intent
func (a *IntentAnalyzer) AnalyzeIntent(ctx context.Context, message string, context map[string]interface{}) (*Intent, error) {
	prompt := fmt.Sprintf(`Analyze this request and extract:
1. Primary intent type (e.g., 'rule_generation', 'validation', 'compliance_check', 'discovery')
2. Confidence score (0-1)
3. Entities as an OBJECT (not array) with string keys and values, e.g., {"resource_type": "Pod", "label": "app"}
4. Required steps as an array of strings
5. Context as an OBJECT with relevant details
6. Suggested tasks as an array of task objects

IMPORTANT:
- 'entities' MUST be an object with string keys and values, NOT an array. If no entities, use empty object {}
- 'context' MUST be an object, NOT an array
- 'suggested_tasks' MUST be an array of objects, each with: type, priority, description, parameters

Example correct response:
{
  "type": "rule_generation",
  "confidence": 0.95,
  "entities": {"resource_type": "Pod", "check_type": "label_validation"},
  "required_steps": ["analyze_resource", "generate_expression"],
  "context": {"domain": "kubernetes", "intent_clarity": "high"},
  "suggested_tasks": [
    {
      "type": "rule_generation",
      "priority": 10,
      "description": "Generate CEL rule for Pod label validation",
      "parameters": {"resource": "Pod", "validation": "has_label"}
    }
  ]
}

Message: %s
Context: %v`, message, context)

	var intent Intent
	err := a.llmClient.Analyze(ctx, prompt, &intent)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze intent: %w", err)
	}

	return &intent, nil
}

// AnalyzeTaskDependencies uses AI to determine task dependencies
func (a *IntentAnalyzer) AnalyzeTaskDependencies(ctx context.Context, tasks []TaskSuggestion) (map[string][]string, error) {
	prompt := fmt.Sprintf(`Analyze these tasks and determine their dependencies:
%s

Return a JSON object where keys are task descriptions and values are arrays of task descriptions that must complete first.
Consider logical dependencies, data flow, and optimal execution order.

Example response:
{
  "Generate validation rule": ["Analyze resource schema", "Extract compliance requirements"],
  "Test rule against cluster": ["Generate validation rule"],
  "Document rule usage": ["Generate validation rule", "Test rule against cluster"]
}`, tasks)

	var dependencies map[string][]string
	err := a.llmClient.Analyze(ctx, prompt, &dependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze dependencies: %w", err)
	}

	return dependencies, nil
}

// RefineIntent refines intent based on additional context or feedback
func (a *IntentAnalyzer) RefineIntent(ctx context.Context, originalIntent *Intent, feedback string) (*Intent, error) {
	prompt := fmt.Sprintf(`Refine the following intent analysis based on user feedback:

Original Intent: %v
User Feedback: %s

Provide an updated intent analysis that incorporates the feedback.

Return JSON in the same format as the original intent:
{
  "type": "refined_intent_type",
  "confidence": 0.0-1.0,
  "entities": {"key1": "value1", "key2": "value2"},
  "required_steps": ["step1", "step2"],
  "context": {"key": "value"},
  "suggested_tasks": [
    {
      "type": "task_type",
      "priority": 1-10,
      "description": "task description",
      "parameters": {"param": "value"}
    }
  ]
}`, originalIntent, feedback)

	var refinedIntent Intent
	err := a.llmClient.Analyze(ctx, prompt, &refinedIntent)
	if err != nil {
		return nil, fmt.Errorf("failed to refine intent: %w", err)
	}

	return &refinedIntent, nil
}

// ExtractComplexRequirements uses AI to extract complex requirements
func (a *IntentAnalyzer) ExtractComplexRequirements(ctx context.Context, message string) (map[string]interface{}, error) {
	prompt := fmt.Sprintf(`Extract all technical requirements from this message:
%s

Consider:
- Security requirements
- Compliance standards
- Performance criteria
- Resource constraints
- Business rules
- Technical specifications

Return a structured JSON with categorized requirements:
{
  "security": {
    "encryption": "TLS 1.2 or higher required",
    "authentication": "mTLS between services"
  },
  "compliance": {
    "frameworks": ["PCI-DSS", "HIPAA"],
    "controls": ["Access logging", "Data encryption at rest"]
  },
  "performance": {
    "response_time": "< 100ms p99",
    "throughput": "> 1000 RPS"
  },
  "resources": {
    "memory": "< 512MB per pod",
    "cpu": "< 0.5 cores"
  },
  "business_rules": ["No data retention beyond 30 days"],
  "technical": {
    "kubernetes_version": ">= 1.25",
    "required_features": ["NetworkPolicies", "PodSecurityStandards"]
  }
}`, message)

	var requirements map[string]interface{}
	err := a.llmClient.Analyze(ctx, prompt, &requirements)
	if err != nil {
		return nil, fmt.Errorf("failed to extract requirements: %w", err)
	}

	return requirements, nil
}
