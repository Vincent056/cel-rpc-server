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

// GeminiProvider implements the AIProvider interface for Google's Gemini
type GeminiProvider struct {
	config     AIProviderConfig
	httpClient *http.Client
}

// GeminiRequest represents the request structure for Gemini API
type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

// GeminiContent represents a content item in Gemini
type GeminiContent struct {
	Role  string       `json:"role"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart represents a part of content
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiResponse represents the response structure from Gemini API
type GeminiResponse struct {
	Candidates []GeminiCandidate `json:"candidates"`
}

// GeminiCandidate represents a response candidate
type GeminiCandidate struct {
	Content GeminiContent `json:"content"`
}

// GenerateCELRule uses Gemini to generate a CEL rule based on the user's request
func (g *GeminiProvider) GenerateCELRule(ctx context.Context, userMessage string, ruleContext *celv1.RuleGenerationContext) (string, string, error) {
	if g.config.APIKey == "" {
		return "", "", fmt.Errorf("Gemini API key not configured")
	}

	// Build the conversation
	contents := []GeminiContent{
		{
			Role: "user",
			Parts: []GeminiPart{
				{Text: buildSystemPrompt() + "\n\n" + buildUserPrompt(userMessage, ruleContext)},
			},
		},
	}

	reqBody, err := json.Marshal(GeminiRequest{
		Contents: contents,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Gemini API endpoint
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		g.config.Model, g.config.APIKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode, string(body))
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", "", fmt.Errorf("no response from Gemini")
	}

	content := geminiResp.Candidates[0].Content.Parts[0].Text
	return parseRuleResponse(content)
}

// GenerateTestCases uses Gemini to generate test cases for a CEL rule
func (g *GeminiProvider) GenerateTestCases(ctx context.Context, rule string, resourceType string) ([]*celv1.RuleTestCase, error) {
	if g.config.APIKey == "" {
		return nil, fmt.Errorf("Gemini API key not configured")
	}

	// Build the conversation
	contents := []GeminiContent{
		{
			Role: "user",
			Parts: []GeminiPart{
				{Text: buildTestCaseSystemPrompt() + "\n\n" + buildTestCaseUserPrompt(rule, resourceType)},
			},
		},
	}

	reqBody, err := json.Marshal(GeminiRequest{
		Contents: contents,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Gemini API endpoint
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		g.config.Model, g.config.APIKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode, string(body))
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	content := geminiResp.Candidates[0].Content.Parts[0].Text
	return parseTestCasesResponse(content)
}
