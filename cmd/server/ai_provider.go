package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
)

// AIProvider defines the interface for AI providers
type AIProvider interface {
	GenerateCELRule(ctx context.Context, userMessage string, ruleContext *celv1.RuleGenerationContext) (expression string, explanation string, err error)
	GenerateTestCases(ctx context.Context, rule string, resourceType string) ([]*celv1.RuleTestCase, error)
}

// AIProviderConfig holds configuration for AI providers
type AIProviderConfig struct {
	Provider      string // "openai", "gemini", "custom"
	APIKey        string
	Endpoint      string // For custom endpoints
	Model         string
	MaxRetries    int
	Timeout       time.Duration
	CustomHeaders map[string]string // For custom authentication headers
}

// NewAIProvider creates an AI provider based on configuration
func NewAIProvider(config AIProviderConfig) (AIProvider, error) {
	// Set defaults
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	// Get API key from environment if not provided
	if config.APIKey == "" {
		switch config.Provider {
		case "openai":
			config.APIKey = os.Getenv("OPENAI_API_KEY")
		case "gemini":
			config.APIKey = os.Getenv("GEMINI_API_KEY")
		case "custom":
			config.APIKey = os.Getenv("CUSTOM_AI_API_KEY")
		}
	}

	// Set default models if not specified
	if config.Model == "" {
		switch config.Provider {
		case "openai":
			config.Model = "gpt-4"
		case "gemini":
			config.Model = "gemini-2.5-pro"
		}
	}

	httpClient := &http.Client{
		Timeout: config.Timeout,
	}

	switch strings.ToLower(config.Provider) {
	case "openai":
		return &OpenAIProvider{
			config:     config,
			httpClient: httpClient,
		}, nil
	case "gemini":
		return &GeminiProvider{
			config:     config,
			httpClient: httpClient,
		}, nil
	case "custom":
		if config.Endpoint == "" {
			return nil, fmt.Errorf("custom provider requires endpoint URL")
		}
		return &CustomProvider{
			config:     config,
			httpClient: httpClient,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", config.Provider)
	}
}

// Helper functions for building prompts
func buildSystemPrompt() string {
	return `You are an expert in Kubernetes security and Common Expression Language (CEL). 
Your task is to generate CEL rules for Kubernetes resource validation based on user requirements.

CEL Rules Guidelines:
- Use 'resource' as the main variable representing the Kubernetes resource
- For pods: resource.spec.containers, resource.metadata, resource.spec.securityContext
- For deployments: resource.spec.replicas, resource.spec.template.spec
- Use has() to check field existence before accessing
- Use .all() for array validations
- Keep expressions concise and readable

Respond in JSON format:
{
  "expression": "the CEL expression",
  "explanation": "brief explanation of what the rule checks"
}`
}

func buildUserPrompt(userMessage string, ruleContext *celv1.RuleGenerationContext) string {
	return fmt.Sprintf(`Generate a CEL rule for: %s

Context:
Resource Type: %s
API Version: %s
Namespace: %s

Focus on security, compliance, and best practices.`,
		userMessage,
		getResourceType(ruleContext),
		getApiVersion(ruleContext),
		getNamespace(ruleContext))
}

func buildTestCaseSystemPrompt() string {
	return `You are an expert in Kubernetes and CEL rules. Generate test cases for CEL rules.
IMPORTANT: All Kubernetes resources are returned as lists with an 'items' field, even singleton resources.
Return a JSON array of test cases with this structure:
[
  {
    "name": "descriptive test name",
    "description": "what this test validates",
    "resource": {
      "items": [
        {kubernetes resource JSON}
      ]
    },
    "expected": true/false
  }
]
Note: The resource field must ALWAYS contain an "items" array, even for single resources.`
}

func buildTestCaseUserPrompt(rule string, resourceType string) string {
	return fmt.Sprintf(`Generate 3-5 test cases for this CEL rule:
Rule: %s
Resource Type: %s

Include both passing and failing test cases.`, rule, resourceType)
}

// parseRuleResponse attempts to parse AI response into expression and explanation
func parseRuleResponse(content string) (expression string, explanation string, err error) {
	var result struct {
		Expression  string `json:"expression"`
		Explanation string `json:"explanation"`
	}

	if err := json.Unmarshal([]byte(content), &result); err != nil {
		// Try to extract from plain text if JSON parsing fails
		lines := strings.Split(content, "\n")
		if len(lines) >= 2 {
			result.Expression = strings.TrimSpace(lines[0])
			result.Explanation = strings.TrimSpace(strings.Join(lines[1:], " "))
		} else {
			return "", "", fmt.Errorf("failed to parse AI response: %w", err)
		}
	}

	return result.Expression, result.Explanation, nil
}

// parseTestCasesResponse attempts to parse AI response into test cases
func parseTestCasesResponse(content string) ([]*celv1.RuleTestCase, error) {
	var testCasesData []struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Resource    map[string]interface{} `json:"resource"`
		Expected    bool                   `json:"expected"`
	}

	if err := json.Unmarshal([]byte(content), &testCasesData); err != nil {
		return nil, fmt.Errorf("failed to parse test cases: %w", err)
	}

	// Convert to protobuf test cases
	testCases := make([]*celv1.RuleTestCase, len(testCasesData))
	for i, tc := range testCasesData {
		resourceJSON, _ := json.Marshal(tc.Resource)
		testCases[i] = &celv1.RuleTestCase{
			Id:             fmt.Sprintf("test-%d", i+1),
			Name:           tc.Name,
			Description:    tc.Description,
			TestData:       map[string]string{"resource": string(resourceJSON)},
			ExpectedResult: tc.Expected,
		}
	}

	return testCases, nil
}

// Helper functions
func getResourceType(ctx *celv1.RuleGenerationContext) string {
	if ctx == nil || ctx.ResourceType == "" {
		return "pods" // Default to pods
	}
	return ctx.ResourceType
}

func getApiVersion(ctx *celv1.RuleGenerationContext) string {
	if ctx == nil || ctx.ApiVersion == "" {
		return "v1"
	}
	return ctx.ApiVersion
}

func getNamespace(ctx *celv1.RuleGenerationContext) string {
	if ctx == nil || ctx.Namespace == "" {
		return "default"
	}
	return ctx.Namespace
}
