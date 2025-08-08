package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
)

// OpenAIProvider implements the AIProvider interface for OpenAI
type OpenAIProvider struct {
	config     AIProviderConfig
	httpClient *http.Client
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
func (p *OpenAIProvider) GenerateCELRule(ctx context.Context, userMessage string, ruleContext *celv1.RuleGenerationContext) (string, string, error) {
	if p.config.APIKey == "" {
		return "", "", fmt.Errorf("OpenAI API key not configured")
	}

	messages := []Message{
		{Role: "system", Content: buildSystemPrompt()},
		{Role: "user", Content: buildUserPrompt(userMessage, ruleContext)},
	}

	reqBody, err := json.Marshal(OpenAIRequest{
		Model:    p.config.Model,
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
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.httpClient.Do(req)
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

	return parseRuleResponse(openAIResp.Choices[0].Message.Content)
}

// GenerateTestCases uses OpenAI to generate test cases for a CEL rule
func (p *OpenAIProvider) GenerateTestCases(ctx context.Context, rule string, resourceType string) ([]*celv1.RuleTestCase, error) {
	if p.config.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	messages := []Message{
		{Role: "system", Content: buildTestCaseSystemPrompt()},
		{Role: "user", Content: buildTestCaseUserPrompt(rule, resourceType)},
	}

	reqBody, err := json.Marshal(OpenAIRequest{
		Model:    p.config.Model,
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
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.httpClient.Do(req)
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

	return parseTestCasesResponse(openAIResp.Choices[0].Message.Content)
}
