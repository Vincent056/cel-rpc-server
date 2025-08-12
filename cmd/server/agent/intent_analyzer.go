package agent

import (
	"context"
	"fmt"
	"log"
)

// IntentAnalyzer uses AI to understand user intent
type IntentAnalyzer struct {
	llmClient LLMClient
}

// LLMClient interface for language model interactions
type LLMClient interface {
	Analyze(ctx context.Context, prompt string, schema interface{}) error
	AnalyzeWithWebSearch(ctx context.Context, prompt string, schema interface{}) error
}

// Intent represents analyzed user intent
type Intent struct {
	PrimaryIntent    string                 `json:"primary_intent"`
	IntentSummary    string                 `json:"intent_summary"`
	ErrorMessage     string                 `json:"error_message"`
	Context          map[string]interface{} `json:"context,omitempty"`
	Confidence       float64                `json:"confidence"`
	SuggestedTasks   []TaskSuggestion       `json:"suggested_tasks"`
	InformationNeeds []string               `json:"information_needs"`
	Metadata         IntentMetadata         `json:"metadata"`
}

// IntentMetadata contains extracted metadata from user's message
type IntentMetadata struct {
	Platforms                        []string `json:"platforms"`
	UserProvidedDescription          string   `json:"user_provided_description"`
	UserProvidedSeverity             string   `json:"user_provided_severity"`
	UserProvidedRationale            string   `json:"user_provided_rationale"`
	UserProvidedBenchmark            string   `json:"user_provided_benchmark"`
	UserProvidedControlID            string   `json:"user_provided_control_id"`
	UserProvidedImpact               string   `json:"user_provided_impact"`
	UserProvidedAudit                string   `json:"user_provided_audit"`
	UserProvidedRemediation          string   `json:"user_provided_remediation"`
	UserProvidedProfileApplicability string   `json:"user_provided_profile_applicability"`
	UserInitialPrompt                string   `json:"user_initial_prompt"`
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
1. Primary intent type (available options: rule_generation)
2. Confidence score (0-1)
3. Suggested tasks as an array of task objects
4. Intent metadata as an object with the following fields:
   - platforms: array of target platforms mentioned (e.g., ["kubernetes", "openshift", "rhel"])
   - user_provided_description: extract any description of the rule/control
   - user_provided_severity: extract any severity information
   - user_provided_rule_name: extract any rule name or title
   - user_provided_benchmark_version: extract any benchmark version mentioned
   - user_provided_rationale: extract any rationale or reason for the rule
   - user_provided_benchmark: extract any benchmark references (e.g., CIS, NIST, etc.)
   - user_provided_control_id: extract any control IDs mentioned
   - user_provided_impact: extract any impact or consequence information
   - user_provided_audit: extract any audit or checking procedures
   - user_provided_remediation: extract any remediation or fix instructions
   - user_provided_profile_applicability: extract any profile or applicability information (e.g., "Level 1", "Level 2", "Level 3")
   - user_initial_prompt: the summary of the user's request
5. Information needs as an array of strings, this should be must to have information to generate the rule, such as resource schema, what resource to check, etc: (e.g., ["What Kubernetes resource type to check", "What specific condition or property to validate", "Target platform (Kubernetes, OpenShift, etc.)"])
   - ALWAYS populate information_needs if critical information is missing (resource type, validation condition, platform, etc.)
6. Suggested tasks as an array of task objects
7. If user's request is not what we can handle, OR if the request is too vague/generic to generate a proper rule (like "write a rule", "check if a = b" without context), set PrimaryIntent as "not_supported" and provide a helpful error_message explaining what information is needed

IMPORTANT:
- 'entities' MUST be an object with string keys and values, NOT an array. If no entities, use empty object {}
- 'context' MUST be an object, NOT an array
- 'suggested_tasks' MUST be an array of objects, each with: type, priority, description, parameters
- For VAGUE requests (like "write a rule", "check if a = b", "create validation" without specifics):
  * Set confidence to 0.3 or below
  * Set primary_intent to "rule_generation" (not "not_supported" unless completely unrelated)
  * Include clear intent_summary explaining what's missing
  * ALWAYS populate information_needs with what's missing
  * Set error_message with helpful guidance
available task types: generate_compliance_rule, generate_test_case, generate_test_data_file, generate_test_data_file_from_template, generate_test_data_file_from_template_with_parameters, generate_test_data_file_from_template_with_parameters_and_context, generate_test_data_file_from_template_with_parameters_and_context_and_feedback, generate_test_data_file_from_template_with_parameters_and_context_and_feedback_and_test_case, generate_test_data_file_from_template_with_parameters_and_context_and_feedback_and_test_case_and_test_data_file

Example GOOD request response (clear and specific):
{
  "primary_intent": "rule_generation",
  "intent_summary": "User wants to generate a compliance rule to validate that Pods have required labels",
  "confidence": 0.95,
  "metadata": {
    "platforms": ["kubernetes", "openshift", "rhel"],
    "user_provided_description": "Validate all namespaces have network policies",
    "user_provided_severity": "High",
    "user_provided_rule_name": "Network Policy Validation",
    "user_provided_benchmark_version": "1.0.0",
    "user_provided_rationale": "To ensure that all namespaces have network policies",
	"user_provided_benchmark": "CIS",
	"user_provided_control_id": "1.1.1",
	"user_provided_impact": "If the namespace does not have a network policy, the system may not function correctly",
	"user_provided_audit": "Audit the namespaces to ensure they have network policies",
	"user_provided_remediation": "Add a network policy to the namespace, run command: kubectl apply -f network-policy.yaml",
	"user_provided_profile_applicability": "Level 1",
	"user_initial_prompt": "Create a rule to check if all namespaces have network policies"
  },
  "suggested_tasks": [
    {
      "type": "rule_generation",
      "priority": 1,
      "description": "Generate a compliance rule to validate that all namespaces have network policies",
      "parameters": {
        "description": "generate a kubernetes compliance rule to validate that all namespaces have network policies",
        "expression": "each namespace has a network policy",
        "input-var-1": "namespace",
		"input-var-2": "networkpolicy",
        ]
      }
    }
  ]
}

Example VAGUE request response:
For input "write a rule check if a = b":
{
  "primary_intent": "rule_generation",
  "intent_summary": "User wants to check equality between 'a' and 'b', but the request lacks critical context about what 'a' and 'b' represent, what resource type to validate, and the target platform.",
  "error_message": "To create a meaningful rule, I need to know: What do 'a' and 'b' represent? What Kubernetes/platform resource should this rule validate? What is the business purpose of this check?",
  "confidence": 0.2,
  "information_needs": [
    "What resource type should this rule apply to (Pod, Deployment, ConfigMap, etc.)?",
    "What do 'a' and 'b' represent in your context?",
    "What platform are you targeting (Kubernetes, OpenShift, etc.)?",
    "What is the compliance or security goal of this validation?"
  ],
  "metadata": {
    "platforms": [],
    "user_provided_description": "check if a = b",
    "user_initial_prompt": "write a rule check if a = b"
  },
  "suggested_tasks": []
}

Message: %s
Context: %v`, message, context)

	var intent Intent
	err := a.llmClient.Analyze(ctx, prompt, &intent)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze intent: %w", err)
	}

	// log intent
	log.Printf("[IntentAnalyzer] Analyzed intent Primary Intent: %s, Confidence: %f", intent.PrimaryIntent, intent.Confidence)
	log.Printf("[IntentAnalyzer] Analyzed intent Intent Summary: %s", intent.IntentSummary)
	log.Printf("[IntentAnalyzer] Analyzed intent Suggested Tasks: %v", intent.SuggestedTasks)
	log.Printf("[IntentAnalyzer] Analyzed intent Metadata Platforms: %v", intent.Metadata.Platforms)
	log.Printf("[IntentAnalyzer] Analyzed intent Metadata Description: %s", intent.Metadata.UserProvidedDescription)
	log.Printf("[IntentAnalyzer] Analyzed intent Metadata Benchmark: %s", intent.Metadata.UserProvidedBenchmark)
	log.Printf("[IntentAnalyzer] Analyzed intent Metadata Control ID: %s", intent.Metadata.UserProvidedControlID)
	log.Printf("[IntentAnalyzer] Analyzed intent Metadata Initial Prompt: %s", intent.Metadata.UserInitialPrompt)

	return &intent, nil
}

// AnalyzeTaskDependencies uses AI to determine task dependencies
func (a *IntentAnalyzer) AnalyzeTaskDependencies(ctx context.Context, tasks []TaskSuggestion) (map[string][]string, error) {
	prompt := fmt.Sprintf(`Analyze these tasks and determine their dependencies:
%v

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
