package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
)

// CustomProvider implements the AIProvider interface for custom OpenAPI-compatible endpoints
type CustomProvider struct {
	config     AIProviderConfig
	httpClient *http.Client
}

// CustomRequest represents a generic request structure for custom endpoints
// This follows the OpenAI API structure as a common standard
type CustomRequest struct {
	Model       string                 `json:"model,omitempty"`
	Messages    []Message              `json:"messages,omitempty"`
	Prompt      string                 `json:"prompt,omitempty"` // For simpler APIs
	Temperature float32                `json:"temperature,omitempty"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Stream      bool                   `json:"stream"`
	Extra       map[string]interface{} `json:"-"` // For any additional fields
}

// CustomResponse represents a generic response structure
type CustomResponse struct {
	// OpenAI-style response
	Choices []struct {
		Message Message `json:"message,omitempty"`
		Text    string  `json:"text,omitempty"`
		Delta   struct {
			Content string `json:"content,omitempty"`
		} `json:"delta,omitempty"`
	} `json:"choices,omitempty"`

	// Alternative simple response formats
	Response   string `json:"response,omitempty"`
	Text       string `json:"text,omitempty"`
	Content    string `json:"content,omitempty"`
	Completion string `json:"completion,omitempty"`
	Output     string `json:"output,omitempty"`
	Result     string `json:"result,omitempty"`
}

// GenerateCELRule uses a custom endpoint to generate a CEL rule
func (c *CustomProvider) GenerateCELRule(ctx context.Context, userMessage string, ruleContext *celv1.RuleGenerationContext) (string, string, error) {
	if c.config.Endpoint == "" {
		return "", "", fmt.Errorf("custom endpoint URL not configured")
	}

	// Build the request based on whether the endpoint expects messages or a simple prompt
	var reqBody []byte
	var err error

	// Try OpenAI-style format first (most common)
	if c.config.Model != "" || strings.Contains(c.config.Endpoint, "chat") || strings.Contains(c.config.Endpoint, "completions") {
		messages := []Message{
			{Role: "system", Content: buildSystemPrompt()},
			{Role: "user", Content: buildUserPrompt(userMessage, ruleContext)},
		}

		req := CustomRequest{
			Model:    c.config.Model,
			Messages: messages,
			Stream:   false,
		}

		reqBody, err = json.Marshal(req)
	} else {
		// Fallback to simple prompt format
		prompt := buildSystemPrompt() + "\n\n" + buildUserPrompt(userMessage, ruleContext)
		req := CustomRequest{
			Prompt: prompt,
			Model:  c.config.Model,
			Stream: false,
		}

		reqBody, err = json.Marshal(req)
	}

	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.Endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add authentication
	if c.config.APIKey != "" {
		// Try common authentication patterns
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
		req.Header.Set("X-API-Key", c.config.APIKey)
		req.Header.Set("Api-Key", c.config.APIKey)
	}

	// Add any custom headers
	for k, v := range c.config.CustomHeaders {
		req.Header.Set(k, v)
	}

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
		return "", "", fmt.Errorf("custom API error (status %d): %s", resp.StatusCode, string(body))
	}

	var customResp CustomResponse
	if err := json.Unmarshal(body, &customResp); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Extract content from various possible response formats
	var content string

	// Try OpenAI-style response first
	if len(customResp.Choices) > 0 {
		if customResp.Choices[0].Message.Content != "" {
			content = customResp.Choices[0].Message.Content
		} else if customResp.Choices[0].Text != "" {
			content = customResp.Choices[0].Text
		} else if customResp.Choices[0].Delta.Content != "" {
			content = customResp.Choices[0].Delta.Content
		}
	}

	// Try simple response formats
	if content == "" {
		if customResp.Response != "" {
			content = customResp.Response
		} else if customResp.Text != "" {
			content = customResp.Text
		} else if customResp.Content != "" {
			content = customResp.Content
		} else if customResp.Completion != "" {
			content = customResp.Completion
		} else if customResp.Output != "" {
			content = customResp.Output
		} else if customResp.Result != "" {
			content = customResp.Result
		}
	}

	if content == "" {
		return "", "", fmt.Errorf("no content found in custom API response")
	}

	return parseRuleResponse(content)
}

// GenerateTestCases uses a custom endpoint to generate test cases
func (c *CustomProvider) GenerateTestCases(ctx context.Context, rule string, resourceType string) ([]*celv1.RuleTestCase, error) {
	if c.config.Endpoint == "" {
		return nil, fmt.Errorf("custom endpoint URL not configured")
	}

	// Build the request
	var reqBody []byte
	var err error

	// Try OpenAI-style format first
	if c.config.Model != "" || strings.Contains(c.config.Endpoint, "chat") || strings.Contains(c.config.Endpoint, "completions") {
		messages := []Message{
			{Role: "system", Content: buildTestCaseSystemPrompt()},
			{Role: "user", Content: buildTestCaseUserPrompt(rule, resourceType)},
		}

		req := CustomRequest{
			Model:    c.config.Model,
			Messages: messages,
			Stream:   false,
		}

		reqBody, err = json.Marshal(req)
	} else {
		// Fallback to simple prompt format
		prompt := buildTestCaseSystemPrompt() + "\n\n" + buildTestCaseUserPrompt(rule, resourceType)
		req := CustomRequest{
			Prompt: prompt,
			Model:  c.config.Model,
			Stream: false,
		}

		reqBody, err = json.Marshal(req)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.Endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add authentication
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
		req.Header.Set("X-API-Key", c.config.APIKey)
		req.Header.Set("Api-Key", c.config.APIKey)
	}

	// Add any custom headers
	for k, v := range c.config.CustomHeaders {
		req.Header.Set(k, v)
	}

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
		return nil, fmt.Errorf("custom API error (status %d): %s", resp.StatusCode, string(body))
	}

	var customResp CustomResponse
	if err := json.Unmarshal(body, &customResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Extract content from various possible response formats
	var content string

	// Try OpenAI-style response first
	if len(customResp.Choices) > 0 {
		if customResp.Choices[0].Message.Content != "" {
			content = customResp.Choices[0].Message.Content
		} else if customResp.Choices[0].Text != "" {
			content = customResp.Choices[0].Text
		} else if customResp.Choices[0].Delta.Content != "" {
			content = customResp.Choices[0].Delta.Content
		}
	}

	// Try simple response formats
	if content == "" {
		if customResp.Response != "" {
			content = customResp.Response
		} else if customResp.Text != "" {
			content = customResp.Text
		} else if customResp.Content != "" {
			content = customResp.Content
		} else if customResp.Completion != "" {
			content = customResp.Completion
		} else if customResp.Output != "" {
			content = customResp.Output
		} else if customResp.Result != "" {
			content = customResp.Result
		}
	}

	if content == "" {
		return nil, fmt.Errorf("no content found in custom API response")
	}

	return parseTestCasesResponse(content)
}
