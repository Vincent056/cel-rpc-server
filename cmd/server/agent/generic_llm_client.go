package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"
)

// GenericLLMClient implements LLMClient using the AI provider system
type GenericLLMClient struct {
	provider   AIProvider
	httpClient *http.Client
	config     LLMConfig
}

// LLMConfig holds configuration for the LLM client
type LLMConfig struct {
	Provider      string
	APIKey        string
	Endpoint      string
	Model         string
	CustomHeaders map[string]string
}

// AIProvider interface for LLM operations
type AIProvider interface {
	SendRequest(ctx context.Context, messages []Message, stream bool) (*ProviderResponse, error)
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ProviderResponse represents the response from an AI provider
type ProviderResponse struct {
	Content string
	Usage   *TokenUsage
}

// TokenUsage tracks token consumption
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// NewGenericLLMClient creates a new generic LLM client
func NewGenericLLMClient(config LLMConfig) (*GenericLLMClient, error) {
	var provider AIProvider

	switch strings.ToLower(config.Provider) {
	case "openai":
		provider = &OpenAIProvider{
			apiKey:  config.APIKey,
			baseURL: "https://api.openai.com/v1",
			model:   config.Model,
		}
	case "gemini":
		provider = &GeminiProvider{
			apiKey: config.APIKey,
			model:  config.Model,
		}
	case "custom":
		// For Ollama and other custom endpoints
		provider = &CustomProvider{
			endpoint:      config.Endpoint,
			apiKey:        config.APIKey,
			model:         config.Model,
			customHeaders: config.CustomHeaders,
		}
	default:
		return nil, fmt.Errorf("unsupported provider: %s", config.Provider)
	}

	return &GenericLLMClient{
		provider:   provider,
		httpClient: &http.Client{Timeout: 300 * time.Second}, // Longer timeout for large models
		config:     config,
	}, nil
}

// Analyze implements the LLMClient interface
func (c *GenericLLMClient) Analyze(ctx context.Context, prompt string, schema interface{}) error {
	fmt.Printf("DEBUG: Analyze invoked with result type: %T\n", schema)
	fmt.Printf("DEBUG: Using AI provider: %s\n", c.config.Provider)

	// Create system message for structured output
	systemPrompt := "You are an AI assistant that analyzes user input and returns structured JSON responses. Always respond with valid JSON that matches the provided schema."

	// Add schema information to the prompt if available
	if schema != nil {
		schemaJSON, _ := json.MarshalIndent(schema, "", "  ")
		systemPrompt += fmt.Sprintf("\n\nExpected JSON schema:\n%s", string(schemaJSON))
	}

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}

	fmt.Printf("DEBUG: Sending request to %s provider...\n", c.config.Provider)
	resp, err := c.provider.SendRequest(ctx, messages, false)
	if err != nil {
		return fmt.Errorf("provider request failed: %w", err)
	}

	// Log token usage if available
	if resp.Usage != nil {
		fmt.Printf("DEBUG: Token usage - Prompt: %d, Completion: %d, Total: %d\n",
			resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	}

	// Parse the response into the provided schema
	if err := json.Unmarshal([]byte(resp.Content), schema); err != nil {
		// Try to sanitize the response
		sanitized, sanitizeErr := c.sanitizeResponse([]byte(resp.Content), schema)
		if sanitizeErr == nil {
			if unmarshalErr := json.Unmarshal(sanitized, schema); unmarshalErr == nil {
				fmt.Printf("DEBUG: Successfully parsed response into %T\n", schema)
				return nil
			}
		}
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("DEBUG: Successfully parsed response into %T\n", schema)
	return nil
}

// AnalyzeWithWebSearch implements the LLMClient interface
func (c *GenericLLMClient) AnalyzeWithWebSearch(ctx context.Context, prompt string, schema interface{}) error {
	// For now, just use regular Analyze
	// Web search functionality can be added later if needed
	return c.Analyze(ctx, prompt, schema)
}

// sanitizeResponse attempts to fix common JSON issues
func (c *GenericLLMClient) sanitizeResponse(response []byte, result interface{}) ([]byte, error) {
	// First, try to extract JSON from markdown code blocks
	responseStr := string(response)

	// Check for ```json blocks
	if strings.Contains(responseStr, "```json") {
		start := strings.Index(responseStr, "```json")
		if start != -1 {
			start += 7 // Move past ```json
			end := strings.Index(responseStr[start:], "```")
			if end != -1 {
				jsonStr := strings.TrimSpace(responseStr[start : start+end])
				response = []byte(jsonStr)
				fmt.Println("DEBUG: Extracted JSON from markdown code block")
			}
		}
	} else if strings.Contains(responseStr, "```") {
		// Check for generic ``` blocks
		start := strings.Index(responseStr, "```")
		if start != -1 {
			start += 3 // Move past ```
			// Skip any language identifier on the same line
			newlineIdx := strings.Index(responseStr[start:], "\n")
			if newlineIdx != -1 && newlineIdx < 20 { // Reasonable limit for language identifier
				start += newlineIdx + 1
			}
			end := strings.Index(responseStr[start:], "```")
			if end != -1 {
				jsonStr := strings.TrimSpace(responseStr[start : start+end])
				// Try to parse it as JSON
				var test json.RawMessage
				if err := json.Unmarshal([]byte(jsonStr), &test); err == nil {
					response = []byte(jsonStr)
					fmt.Println("DEBUG: Extracted JSON from generic markdown code block")
				}
			}
		}
	}

	// Implementation similar to OpenAILLMClient's sanitizeResponse
	resultType := reflect.TypeOf(result)
	if resultType.Kind() == reflect.Ptr {
		resultType = resultType.Elem()
	}

	// If expecting an array but got an object with a single array field
	if resultType.Kind() == reflect.Slice {
		var tempObj map[string]interface{}
		if err := json.Unmarshal(response, &tempObj); err == nil && len(tempObj) == 1 {
			for key, value := range tempObj {
				if arr, ok := value.([]interface{}); ok {
					if arrJSON, err := json.Marshal(arr); err == nil {
						fmt.Printf("DEBUG: Unwrapped array from object key '%s'\n", key)
						return arrJSON, nil
					}
				}
			}
		}
	}

	return response, nil
}

// Provider implementations

// OpenAIProvider implements AIProvider for OpenAI
type OpenAIProvider struct {
	apiKey  string
	baseURL string
	model   string
}

func (p *OpenAIProvider) SendRequest(ctx context.Context, messages []Message, stream bool) (*ProviderResponse, error) {
	reqBody := map[string]interface{}{
		"model":    p.model,
		"messages": messages,
		"stream":   stream,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, err
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	return &ProviderResponse{
		Content: openAIResp.Choices[0].Message.Content,
		Usage: &TokenUsage{
			PromptTokens:     openAIResp.Usage.PromptTokens,
			CompletionTokens: openAIResp.Usage.CompletionTokens,
			TotalTokens:      openAIResp.Usage.TotalTokens,
		},
	}, nil
}

// GeminiProvider implements AIProvider for Google Gemini
type GeminiProvider struct {
	apiKey string
	model  string
}

func (p *GeminiProvider) SendRequest(ctx context.Context, messages []Message, stream bool) (*ProviderResponse, error) {
	// Convert messages to Gemini format
	contents := []map[string]interface{}{}

	// Combine system and user messages since Gemini doesn't support system role
	var combinedUserMessage string
	for _, msg := range messages {
		if msg.Role == "system" {
			// Prepend system message to user message
			combinedUserMessage = msg.Content + "\n\n"
		} else if msg.Role == "user" {
			combinedUserMessage += msg.Content
		}
	}

	// Add as a single user message
	if combinedUserMessage != "" {
		contents = append(contents, map[string]interface{}{
			"role": "user",
			"parts": []map[string]string{
				{"text": combinedUserMessage},
			},
		})
	}

	reqBody := map[string]interface{}{
		"contents": contents,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		p.model, p.apiKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	return &ProviderResponse{
		Content: geminiResp.Candidates[0].Content.Parts[0].Text,
	}, nil
}

// CustomProvider implements AIProvider for custom endpoints like Ollama
type CustomProvider struct {
	endpoint      string
	apiKey        string
	model         string
	customHeaders map[string]string
}

func (p *CustomProvider) SendRequest(ctx context.Context, messages []Message, stream bool) (*ProviderResponse, error) {
	// For Ollama, we need to format the request differently
	if strings.Contains(p.endpoint, "ollama") || strings.Contains(p.endpoint, "11434") {
		return p.sendOllamaRequest(ctx, messages)
	}

	// For OpenAI-compatible endpoints
	return p.sendOpenAICompatibleRequest(ctx, messages, stream)
}

func (p *CustomProvider) sendOllamaRequest(ctx context.Context, messages []Message) (*ProviderResponse, error) {
	fmt.Printf("DEBUG: Sending request to Ollama at %s with model %s\n", p.endpoint, p.model)

	// Combine messages into a single prompt for Ollama
	var prompt strings.Builder
	for _, msg := range messages {
		prompt.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}

	reqBody := map[string]interface{}{
		"model":  p.model,
		"prompt": prompt.String(),
		"stream": false,
		"format": "json", // Request JSON format
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	fmt.Printf("DEBUG: Ollama request body: %s\n", string(body))

	req, err := http.NewRequestWithContext(ctx, "POST", p.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	// Add custom headers if any
	for k, v := range p.customHeaders {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 300 * time.Second} // Much longer timeout for large models like qwen3:32b
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	fmt.Printf("DEBUG: Ollama response status: %d\n", resp.StatusCode)
	fmt.Printf("DEBUG: Ollama raw response: %s\n", string(respBody))

	var ollamaResp struct {
		Response string `json:"response"`
		Model    string `json:"model"`
		Done     bool   `json:"done"`
	}

	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Ollama response: %w", err)
	}

	fmt.Printf("DEBUG: Ollama parsed response: %s\n", ollamaResp.Response)

	return &ProviderResponse{
		Content: ollamaResp.Response,
	}, nil
}

func (p *CustomProvider) sendOpenAICompatibleRequest(ctx context.Context, messages []Message, stream bool) (*ProviderResponse, error) {
	reqBody := map[string]interface{}{
		"model":    p.model,
		"messages": messages,
		"stream":   stream,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	// Add authentication if API key is provided
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	// Add custom headers
	for k, v := range p.customHeaders {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Try to parse as OpenAI format
	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &openAIResp); err == nil && len(openAIResp.Choices) > 0 {
		return &ProviderResponse{
			Content: openAIResp.Choices[0].Message.Content,
		}, nil
	}

	// Try simple format
	var simpleResp struct {
		Response string `json:"response"`
		Text     string `json:"text"`
		Content  string `json:"content"`
	}

	if err := json.Unmarshal(respBody, &simpleResp); err == nil {
		content := simpleResp.Response
		if content == "" {
			content = simpleResp.Text
		}
		if content == "" {
			content = simpleResp.Content
		}
		if content != "" {
			return &ProviderResponse{
				Content: content,
			}, nil
		}
	}

	return nil, fmt.Errorf("unable to parse response format")
}
