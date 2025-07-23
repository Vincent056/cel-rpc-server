package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
)

// OpenAIClient handles communication with OpenAI API
type OpenAIClient struct {
	apiKey     string
	httpClient *http.Client
	model      string
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey string) *OpenAIClient {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	return &OpenAIClient{
		apiKey:     apiKey,
		httpClient: &http.Client{},
		model:      "gpt-4.1", // or "gpt-4" or "gpt-4-1106-preview"
	}
}

type OpenAIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Index   int     `json:"index"`
	Message Message `json:"message"`
	Delta   *Delta  `json:"delta,omitempty"`
}

type Delta struct {
	Content string `json:"content"`
}

// GenerateCELRule uses OpenAI to generate a CEL rule based on the user's request
func (c *OpenAIClient) GenerateCELRule(ctx context.Context, userMessage string, ruleContext *celv1.RuleGenerationContext) (string, string, error) {
	if c.apiKey == "" {
		// Fallback to pattern-based generation if no API key
		return "", "", fmt.Errorf("OpenAI API key not configured")
	}

	systemPrompt := `You are an expert in Kubernetes security and Common Expression Language (CEL). 
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

	// Build user prompt with context
	userPrompt := fmt.Sprintf(`Generate a CEL rule for: %s

Context:
Resource Type: %s
API Version: %s
Namespace: %s

Focus on security, compliance, and best practices.`,
		userMessage,
		getResourceType(ruleContext),
		getApiVersion(ruleContext),
		getNamespace(ruleContext))

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	reqBody, err := json.Marshal(OpenAIRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var openAIResp OpenAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return "", "", fmt.Errorf("no response from OpenAI")
	}

	// Parse the JSON response from GPT-4
	var result struct {
		Expression  string `json:"expression"`
		Explanation string `json:"explanation"`
	}

	content := openAIResp.Choices[0].Message.Content
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		// Try to extract from plain text if JSON parsing fails
		// Simple fallback parsing
		lines := strings.Split(content, "\n")
		if len(lines) >= 2 {
			result.Expression = strings.TrimSpace(lines[0])
			result.Explanation = strings.TrimSpace(strings.Join(lines[1:], " "))
		} else {
			return "", "", fmt.Errorf("failed to parse OpenAI response: %w", err)
		}
	}

	return result.Expression, result.Explanation, nil
}

// GenerateTestCases uses OpenAI to generate test cases for a CEL rule
func (c *OpenAIClient) GenerateTestCases(ctx context.Context, rule string, resourceType string) ([]*celv1.RuleTestCase, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	systemPrompt := `You are an expert in Kubernetes and CEL rules. Generate test cases for CEL rules.
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

	userPrompt := fmt.Sprintf(`Generate 3-5 test cases for this CEL rule:
Rule: %s
Resource Type: %s

Include both passing and failing test cases.`, rule, resourceType)

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	reqBody, err := json.Marshal(OpenAIRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var openAIResp OpenAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	// Parse the JSON response
	var testCasesData []struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Resource    map[string]interface{} `json:"resource"`
		Expected    bool                   `json:"expected"`
	}

	content := openAIResp.Choices[0].Message.Content
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

// Add helper functions at the end of the file
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
