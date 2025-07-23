package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"
)

// OpenAILLMClient implements LLMClient using OpenAI API
type OpenAILLMClient struct {
	apiKey     string
	httpClient *http.Client
	model      string
	baseURL    string
}

// NewOpenAILLMClient creates a new OpenAI LLM client
func NewOpenAILLMClient(apiKey string) *OpenAILLMClient {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	return &OpenAILLMClient{
		apiKey:     apiKey,
		baseURL:    "https://api.openai.com/v1",
		model:      "gpt-4.1", // This model supports structured outputs with JSON schema
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// sanitizeResponse fixes common issues with AI responses
func (c *OpenAILLMClient) sanitizeResponse(response []byte, result interface{}) ([]byte, error) {
	// Check if result expects an array/slice
	resultType := reflect.TypeOf(result)
	if resultType.Kind() == reflect.Ptr {
		resultType = resultType.Elem()
	}

	// If expecting an array but got an object with a single array field
	if resultType.Kind() == reflect.Slice {
		var tempObj map[string]interface{}
		if err := json.Unmarshal(response, &tempObj); err == nil && len(tempObj) == 1 {
			// Check if there's a single key that contains an array
			for key, value := range tempObj {
				if arr, ok := value.([]interface{}); ok {
					// Found an array, return it directly
					if arrJSON, err := json.Marshal(arr); err == nil {
						fmt.Printf("DEBUG: Unwrapped array from object key '%s'\n", key)
						return arrJSON, nil
					}
				}
			}
		}
		// If it's already an array or we can't unwrap, return as-is
		return response, nil
	}

	// Parse the response for object types
	var tempData map[string]interface{}
	if err := json.Unmarshal(response, &tempData); err != nil {
		return response, nil // Return original if can't parse
	}

	modified := false

	// Fix inputs field if it's an array of arrays instead of array of objects
	if inputs, exists := tempData["inputs"]; exists {
		switch inp := inputs.(type) {
		case []interface{}:
			fixedInputs := []interface{}{}
			needsFix := false

			for _, item := range inp {
				switch v := item.(type) {
				case []interface{}:
					// This is an array - check if it's a single-element array containing an object
					if len(v) == 1 {
						if obj, ok := v[0].(map[string]interface{}); ok {
							fixedInputs = append(fixedInputs, obj)
							needsFix = true
							continue
						}
					}

					// Otherwise, try to convert to object (array of key-value pairs)
					inputObj := make(map[string]interface{})
					for _, kv := range v {
						if kvMap, ok := kv.(map[string]interface{}); ok {
							if key, hasKey := kvMap["key"]; hasKey {
								if val, hasVal := kvMap["value"]; hasVal {
									inputObj[fmt.Sprintf("%v", key)] = val
								}
							}
						}
					}
					if len(inputObj) > 0 {
						fixedInputs = append(fixedInputs, inputObj)
					} else {
						// If we couldn't extract key-value pairs, keep original
						fixedInputs = append(fixedInputs, v)
					}
					needsFix = true
				case map[string]interface{}:
					// Already an object, keep as is
					fixedInputs = append(fixedInputs, v)
				default:
					// Keep as is
					fixedInputs = append(fixedInputs, v)
				}
			}

			if needsFix {
				tempData["inputs"] = fixedInputs
				modified = true
			}
		}
	}

	// For Intent type, convert JSON strings back to actual objects
	if _, isIntent := result.(*Intent); isIntent {
		// Convert entities_json string to entities map
		if entitiesJSON, exists := tempData["entities_json"]; exists {
			if jsonStr, ok := entitiesJSON.(string); ok {
				var entities map[string]interface{}
				if err := json.Unmarshal([]byte(jsonStr), &entities); err == nil {
					tempData["entities"] = entities
					delete(tempData, "entities_json")
					modified = true
				}
			}
		}

		// Convert required_steps_json string to array
		if stepsJSON, exists := tempData["required_steps_json"]; exists {
			if jsonStr, ok := stepsJSON.(string); ok {
				var steps []string
				if err := json.Unmarshal([]byte(jsonStr), &steps); err == nil {
					tempData["required_steps"] = steps
					delete(tempData, "required_steps_json")
					modified = true
				}
			}
		}

		// Convert context_json string to context map
		if contextJSON, exists := tempData["context_json"]; exists {
			if jsonStr, ok := contextJSON.(string); ok {
				var context map[string]interface{}
				if err := json.Unmarshal([]byte(jsonStr), &context); err == nil {
					tempData["context"] = context
					delete(tempData, "context_json")
					modified = true
				}
			}
		}

		// Convert suggested_tasks_json string to array
		if tasksJSON, exists := tempData["suggested_tasks_json"]; exists {
			if jsonStr, ok := tasksJSON.(string); ok {
				var tasks []interface{}
				if err := json.Unmarshal([]byte(jsonStr), &tasks); err == nil {
					tempData["suggested_tasks"] = tasks
					delete(tempData, "suggested_tasks_json")
					modified = true
				}
			}
		}
	}

	// Fix entities if it's an array instead of an object (for other response types)
	if entities, exists := tempData["entities"]; exists {
		switch e := entities.(type) {
		case []interface{}:
			// Only convert if it's not already handled above
			if _, isIntent := result.(*Intent); !isIntent {
				// Convert array to object
				entitiesObj := make(map[string]interface{})
				for i, item := range e {
					entitiesObj[fmt.Sprintf("entity_%d", i)] = item
				}
				tempData["entities"] = entitiesObj
				modified = true
			}
		}
	}

	// Fix parameters field if it's an array (should be object)
	if params, exists := tempData["parameters"]; exists {
		switch p := params.(type) {
		case []interface{}:
			// Convert array to object
			paramsObj := make(map[string]interface{})
			if len(p) == 0 {
				tempData["parameters"] = paramsObj
			} else {
				for i, item := range p {
					paramsObj[fmt.Sprintf("param_%d", i)] = item
				}
				tempData["parameters"] = paramsObj
			}
			modified = true
		}
	}

	// Fix compliance_mappings if it's not the right structure
	if mappings, exists := tempData["compliance_mappings"]; exists {
		switch m := mappings.(type) {
		case []interface{}:
			// Convert array of key-value pairs to map
			mappingsObj := make(map[string]interface{})
			for _, item := range m {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if key, hasKey := itemMap["key"]; hasKey {
						if value, hasValue := itemMap["value"]; hasValue {
							keyStr := fmt.Sprintf("%v", key)
							// Convert value to array of strings
							mappingsObj[keyStr] = []interface{}{fmt.Sprintf("%v", value)}
						}
					}
				}
			}
			tempData["compliance_mappings"] = mappingsObj
			modified = true
		case map[string]interface{}:
			// Check if it has the framework_0, framework_1 pattern from previous sanitization
			needsCleanup := false
			cleanedMappings := make(map[string]interface{})

			for k, v := range m {
				if strings.HasPrefix(k, "framework_") {
					// This is from our previous sanitization, extract the actual mappings
					if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
						if mapping, ok := arr[0].(map[string]interface{}); ok {
							if key, hasKey := mapping["key"]; hasKey {
								if value, hasValue := mapping["value"]; hasValue {
									keyStr := fmt.Sprintf("%v", key)
									cleanedMappings[keyStr] = []interface{}{fmt.Sprintf("%v", value)}
									needsCleanup = true
								}
							}
						}
					}
				} else {
					// Ensure values are arrays
					if _, ok := v.([]interface{}); !ok {
						cleanedMappings[k] = []interface{}{v}
						needsCleanup = true
					} else {
						cleanedMappings[k] = v
					}
				}
			}

			if needsCleanup {
				tempData["compliance_mappings"] = cleanedMappings
				modified = true
			}
		}
	}

	// Fix test_scenarios if it's an array of arrays (should be array of objects)
	if scenarios, exists := tempData["test_scenarios"]; exists {
		switch s := scenarios.(type) {
		case []interface{}:
			fixedScenarios := []interface{}{}
			needsFix := false

			for _, scenario := range s {
				switch sc := scenario.(type) {
				case []interface{}:
					// This is an array of objects, likely each array contains a single test scenario
					// Extract the first object from the array if it exists
					if len(sc) > 0 {
						if scenarioObj, ok := sc[0].(map[string]interface{}); ok {
							fixedScenarios = append(fixedScenarios, scenarioObj)
							needsFix = true
							continue
						}
					}

					// If that didn't work, try to convert array of key-value pairs to object
					scenarioObj := make(map[string]interface{})
					for _, kv := range sc {
						if kvMap, ok := kv.(map[string]interface{}); ok {
							if key, hasKey := kvMap["key"]; hasKey {
								if value, hasValue := kvMap["value"]; hasValue {
									keyStr := fmt.Sprintf("%v", key)
									scenarioObj[keyStr] = value
								}
							}
						}
					}
					if len(scenarioObj) > 0 {
						fixedScenarios = append(fixedScenarios, scenarioObj)
					} else {
						// If we couldn't extract key-value pairs, keep original
						fixedScenarios = append(fixedScenarios, sc)
					}
					needsFix = true
				case map[string]interface{}:
					// Already an object, keep as is
					fixedScenarios = append(fixedScenarios, sc)
				default:
					// Keep as is
					fixedScenarios = append(fixedScenarios, sc)
				}
			}

			if needsFix {
				tempData["test_scenarios"] = fixedScenarios
				modified = true
			}
		}
	}

	// Fix input_transformations in subtasks if they are arrays (should be objects)
	if subtasks, exists := tempData["subtasks"]; exists {
		if subtasksList, ok := subtasks.([]interface{}); ok {
			for i, subtask := range subtasksList {
				if subtaskMap, ok := subtask.(map[string]interface{}); ok {
					if inputTrans, exists := subtaskMap["input_transformations"]; exists {
						switch it := inputTrans.(type) {
						case []interface{}:
							// Convert array to object
							transObj := make(map[string]interface{})
							if len(it) == 0 {
								subtaskMap["input_transformations"] = transObj
							} else {
								for j, item := range it {
									if str, ok := item.(string); ok {
										transObj[fmt.Sprintf("step_%d", j)] = str
									} else {
										transObj[fmt.Sprintf("param_%d", j)] = item
									}
								}
								subtaskMap["input_transformations"] = transObj
							}
							subtasksList[i] = subtaskMap
							modified = true
						}
					}
				}
			}
			if modified {
				tempData["subtasks"] = subtasksList
			}
		}
	}

	// Fix suggested_tasks if it has issues with parameters field
	if tasks, exists := tempData["suggested_tasks"]; exists {
		if tasksList, ok := tasks.([]interface{}); ok {
			tasksModified := false
			for i, task := range tasksList {
				if taskMap, ok := task.(map[string]interface{}); ok {
					if params, exists := taskMap["parameters"]; exists {
						switch p := params.(type) {
						case []interface{}:
							// Convert array to object
							paramsObj := make(map[string]interface{})
							for j, item := range p {
								paramsObj[fmt.Sprintf("param_%d", j)] = item
							}
							taskMap["parameters"] = paramsObj
							tasksList[i] = taskMap
							tasksModified = true
						}
					}
				}
			}
			if tasksModified {
				tempData["suggested_tasks"] = tasksList
				modified = true
			}
		}
	}

	// If we modified the data, re-marshal it
	if modified {
		if fixedJSON, err := json.Marshal(tempData); err == nil {
			return fixedJSON, nil
		}
	}

	return response, nil
}

// Analyze implements the LLMClient interface
func (c *OpenAILLMClient) Analyze(ctx context.Context, prompt string, result interface{}) error {
	if c.apiKey == "" {
		return fmt.Errorf("OpenAI API key not configured")
	}

	// Always enable debug logging
	fmt.Printf("DEBUG: Analyze invoked with result type: %T\n", result)

	// Build JSON schema from the result type - we'll include it in the prompt instead
	jsonSchema := c.buildJSONSchema(result)

	// Convert schema to pretty JSON for the prompt
	schemaJSON, err := json.MarshalIndent(jsonSchema, "", "  ")
	if err != nil {
		fmt.Printf("DEBUG: Failed to marshal schema: %v\n", err)
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	fmt.Printf("DEBUG: Generated JSON Schema:\n%s\n", string(schemaJSON))

	// Enhance the prompt with schema information
	enhancedPrompt := fmt.Sprintf(`%s

IMPORTANT: You must respond with a valid JSON object that EXACTLY matches this schema:

%s

Rules:
1. The response must be a single JSON object (not an array)
2. All required fields must be present
3. Field types must match exactly as specified in the schema
4. For map/object fields (like "entities", "parameters", "compliance_mappings"), use JSON objects {}, not arrays []
5. For array fields, use JSON arrays []
6. Do not include any text before or after the JSON object`, prompt, string(schemaJSON))

	messages := []map[string]string{
		{
			"role": "system",
			"content": `You are an expert AI assistant specialized in Kubernetes, security, compliance, and CEL (Common Expression Language).

CEL EXPRESSION EXPERTISE:

📌 Core Principles:
• Kubernetes resources always come as {"items": [...]} - access via resource.items
• Variables from inputs are ALWAYS defined - never check has(variableName)
• Use has() only for object fields: has(pod.spec.serviceAccountName)
• .all() returns TRUE for empty lists (perfect for compliance rules)
• .exists() returns FALSE for empty lists (use for "at least one" checks)

📚 Expression Patterns by Category:

SECURITY & COMPLIANCE:
• Service accounts: pods.items.all(p, has(p.spec.serviceAccountName) && p.spec.serviceAccountName != "")
• Resource limits: pods.items.all(p, p.spec.containers.all(c, has(c.resources.limits.cpu)))
• Non-root: pods.items.all(p, has(p.spec.securityContext.runAsNonRoot) && p.spec.securityContext.runAsNonRoot)

MULTI-RESOURCE VALIDATION:
• Service-Deployment match: services.items.all(s, deployments.items.exists(d, d.metadata.name == s.metadata.name))
• ConfigMap references: pods.items.all(p, !has(p.spec.volumes) || p.spec.volumes.all(v, !has(v.configMap) || configmaps.items.exists(cm, cm.metadata.name == v.configMap.name)))

CONFIGURATION CHECKS:
• Required labels: resources.items.all(r, has(r.metadata.labels.app) && has(r.metadata.labels.version))
• Naming patterns: resources.items.all(r, r.metadata.name.matches("^[a-z][a-z0-9-]*[a-z0-9]$"))

TEST DATA STRUCTURE:
{"variable_name": {"items": [...]}} for all Kubernetes resources

Always respond with valid JSON only, no additional text.`,
		},
		{
			"role":    "user",
			"content": enhancedPrompt,
		},
	}

	requestBody := map[string]interface{}{
		"model":       c.model,
		"messages":    messages,
		"temperature": 0.7,
		"max_tokens":  8000,
		"response_format": map[string]interface{}{
			"type": "json_object",
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		fmt.Printf("DEBUG: Failed to marshal request: %v\n", err)
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	fmt.Printf("DEBUG: Sending request to OpenAI API...\n")

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		fmt.Printf("DEBUG: Failed to create request: %v\n", err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		fmt.Printf("DEBUG: Failed to send request: %v\n", err)
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("DEBUG: Failed to read response: %v\n", err)
		return fmt.Errorf("failed to read response: %w", err)
	}

	fmt.Printf("DEBUG: Response status: %d\n", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("DEBUG: OpenAI Error Response:\n%s\n", string(body))
		return fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		fmt.Printf("DEBUG: Failed to unmarshal response: %v\n", err)
		fmt.Printf("DEBUG: Raw response body:\n%s\n", string(body))
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if apiResponse.Error != nil {
		fmt.Printf("DEBUG: API returned error: %s (type: %s)\n", apiResponse.Error.Message, apiResponse.Error.Type)
		return fmt.Errorf("OpenAI API error: %s (type: %s)", apiResponse.Error.Message, apiResponse.Error.Type)
	}

	if len(apiResponse.Choices) == 0 || apiResponse.Choices[0].Message.Content == "" {
		fmt.Printf("DEBUG: No content in response\n")
		return fmt.Errorf("no content in OpenAI response")
	}

	content := apiResponse.Choices[0].Message.Content
	fmt.Printf("DEBUG: Raw AI response:\n%s\n", content)

	// Stream the AI response to the client if stream channel is available
	fmt.Printf("DEBUG: Checking for stream channel in context...\n")
	if streamChan := ctx.Value(streamChannelContextKey); streamChan != nil {
		fmt.Printf("DEBUG: Found stream channel in context: %T\n", streamChan)
		// Try bidirectional channel first, then send-only
		var ch chan<- interface{}
		if bidirectionalCh, ok := streamChan.(chan interface{}); ok {
			ch = bidirectionalCh
			fmt.Printf("DEBUG: Stream channel is bidirectional\n")
		} else if sendOnlyCh, ok := streamChan.(chan<- interface{}); ok {
			ch = sendOnlyCh
			fmt.Printf("DEBUG: Stream channel is send-only\n")
		}

		if ch != nil {
			fmt.Printf("DEBUG: Streaming AI response to client\n")
			// Send the actual AI response content as a thinking message
			aiResponseMsg := map[string]interface{}{
				"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
				"thinking": map[string]string{
					"message": fmt.Sprintf("🤖 AI Response:\n```json\n%s\n```", content),
				},
			}
			select {
			case ch <- aiResponseMsg:
				fmt.Printf("DEBUG: Successfully sent AI response\n")
			default:
				// Channel full, skip
				fmt.Printf("DEBUG: Stream channel full, skipping AI response\n")
			}
		} else {
			fmt.Printf("DEBUG: Stream channel is not the correct type: %T\n", streamChan)
		}
	} else {
		fmt.Printf("DEBUG: No stream channel in context\n")
	}

	// Log token usage
	if apiResponse.Usage.TotalTokens > 0 {
		fmt.Printf("DEBUG: Token usage - Prompt: %d, Completion: %d, Total: %d\n",
			apiResponse.Usage.PromptTokens,
			apiResponse.Usage.CompletionTokens,
			apiResponse.Usage.TotalTokens)
	}

	// First try to unmarshal directly
	if err := json.Unmarshal([]byte(content), result); err != nil {
		fmt.Printf("DEBUG: Direct unmarshal failed: %v\n", err)
		fmt.Printf("DEBUG: Attempting to sanitize response...\n")

		// Try to sanitize the response
		sanitized, sanitizeErr := c.sanitizeResponse([]byte(content), result)
		if sanitizeErr != nil {
			fmt.Printf("DEBUG: Sanitization error: %v\n", sanitizeErr)
			return fmt.Errorf("failed to parse AI response: %w (sanitization error: %v)", err, sanitizeErr)
		}

		fmt.Printf("DEBUG: Sanitized response:\n%s\n", string(sanitized))

		// Try again with sanitized response
		if err := json.Unmarshal(sanitized, result); err != nil {
			fmt.Printf("DEBUG: Failed to unmarshal sanitized response: %v\n", err)
			return fmt.Errorf("failed to parse sanitized AI response: %w", err)
		}
	}

	fmt.Printf("DEBUG: Successfully parsed response into %T\n", result)

	return nil
}

// AnalyzeStream provides streaming responses (for future use)
func (c *OpenAILLMClient) AnalyzeStream(ctx context.Context, prompt string, callback func(string) error) error {
	if c.apiKey == "" {
		return fmt.Errorf("OpenAI API key not configured")
	}

	messages := []map[string]string{
		{
			"role":    "system",
			"content": "You are an expert AI assistant specialized in Kubernetes, security, compliance, and CEL (Common Expression Language).",
		},
		{
			"role":    "user",
			"content": prompt,
		},
	}

	requestBody := map[string]interface{}{
		"model":       c.model,
		"messages":    messages,
		"temperature": 0.7,
		"stream":      true,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Process streaming response
	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk map[string]interface{}
		if err := decoder.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode stream: %w", err)
		}

		// Extract content from chunk
		if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if delta, ok := choice["delta"].(map[string]interface{}); ok {
					if content, ok := delta["content"].(string); ok {
						if err := callback(content); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

// SetModel allows changing the model
func (c *OpenAILLMClient) SetModel(model string) {
	c.model = model
}

// SetBaseURL allows changing the base URL (for custom deployments)
func (c *OpenAILLMClient) SetBaseURL(baseURL string) {
	c.baseURL = baseURL
}

// buildJSONSchema builds a JSON schema from a Go type using reflection
func (c *OpenAILLMClient) buildJSONSchema(v interface{}) map[string]interface{} {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Handle common types we use
	switch t.Name() {
	case "Intent":
		// Use a very simple, flat structure to test
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"type": map[string]interface{}{
					"type": "string",
				},
				"confidence": map[string]interface{}{
					"type": "number",
				},
				"entities_json": map[string]interface{}{
					"type":        "string",
					"description": "JSON string containing entities map",
				},
				"required_steps_json": map[string]interface{}{
					"type":        "string",
					"description": "JSON array of required steps",
				},
				"context_json": map[string]interface{}{
					"type":        "string",
					"description": "JSON string containing context map",
				},
				"suggested_tasks_json": map[string]interface{}{
					"type":        "string",
					"description": "JSON array of suggested tasks",
				},
			},
			"required":             []string{"type", "confidence", "entities_json", "required_steps_json", "context_json", "suggested_tasks_json"},
			"additionalProperties": false,
		}
	default:
		// Check if this is the AI rule generation response structure
		if t.Kind() == reflect.Struct {
			// Check if it has the fields that indicate it's a rule generation response
			hasName := false
			hasExpression := false
			hasInputs := false
			hasTestScenarios := false

			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				jsonTag := field.Tag.Get("json")
				fieldName := strings.Split(jsonTag, ",")[0]

				switch fieldName {
				case "name":
					hasName = true
				case "expression":
					hasExpression = true
				case "inputs":
					hasInputs = true
				case "test_scenarios":
					hasTestScenarios = true
				}
			}

			// If this looks like a rule generation response, build a custom schema
			if hasName && hasExpression && hasInputs && hasTestScenarios {
				return map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Name of the generated rule",
						},
						"expression": map[string]interface{}{
							"type":        "string",
							"description": "CEL expression for the rule",
						},
						"explanation": map[string]interface{}{
							"type":        "string",
							"description": "Human-readable explanation of what the rule does",
						},
						"variables": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
							"description": "List of variables used in the expression",
						},
						"inputs": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"name": map[string]interface{}{
										"type":        "string",
										"description": "Variable name",
									},
									"type": map[string]interface{}{
										"type":        "string",
										"description": "Input type (kubernetes, file, http)",
									},
									"spec": map[string]interface{}{
										"type":                 "object",
										"description":          "Type-specific configuration",
										"additionalProperties": true,
									},
								},
								"required":             []string{"name", "type", "spec"},
								"additionalProperties": false,
							},
							"description": "Array of input definitions",
						},
						"complexity_score": map[string]interface{}{
							"type":        "integer",
							"description": "Complexity score from 1-10",
						},
						"security_implications": map[string]interface{}{
							"type":        "string",
							"description": "Security implications of the rule",
						},
						"performance_impact": map[string]interface{}{
							"type":        "string",
							"description": "Performance impact assessment",
						},
						"test_scenarios": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type":                 "object",
								"additionalProperties": true,
							},
							"description": "Array of test scenario objects",
						},
						"optimization_suggestions": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
							"description": "Suggestions for optimizing the rule",
						},
						"related_rules": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
							"description": "Names of related rules",
						},
						"compliance_mappings": map[string]interface{}{
							"type":        "object",
							"description": "Map of compliance framework to list of requirements",
							"additionalProperties": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "string",
								},
							},
						},
					},
					"required":             []string{"name", "expression", "explanation", "variables", "inputs", "complexity_score", "security_implications", "performance_impact", "test_scenarios", "optimization_suggestions", "related_rules", "compliance_mappings"},
					"additionalProperties": false,
				}
			}
		}

		// For the planner response structure
		if t.Kind() == reflect.Struct && t.NumField() > 0 && t.Field(0).Name == "Subtasks" {
			return map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"subtasks": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"$ref": "#/$defs/Subtask",
						},
					},
				},
				"required":             []string{"subtasks"},
				"additionalProperties": false,
				"$defs": map[string]interface{}{
					"Subtask": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id": map[string]interface{}{
								"type":        "string",
								"description": "Unique identifier for the subtask",
							},
							"type": map[string]interface{}{
								"type":        "string",
								"description": "Task type from available types",
							},
							"description": map[string]interface{}{
								"type":        "string",
								"description": "What this subtask does",
							},
							"priority": map[string]interface{}{
								"type":        "integer",
								"minimum":     1,
								"maximum":     10,
								"description": "Priority from 1-10 where 10 is highest",
							},
							"dependencies": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "string",
								},
								"description": "Array of task IDs this depends on",
							},
							"estimated_duration": map[string]interface{}{
								"type":        "integer",
								"description": "Estimated time in seconds",
							},
							"required_capabilities": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "string",
								},
								"description": "Array of agent capabilities needed",
							},
							"input_transformations": map[string]interface{}{
								"type":        "object",
								"description": "Object mapping field names to transformation rules",
								"additionalProperties": map[string]interface{}{
									"type": "string",
								},
							},
						},
						"required":             []string{"id", "type", "description", "priority", "dependencies", "estimated_duration", "required_capabilities", "input_transformations"},
						"additionalProperties": false,
					},
				},
			}
		}

		// Build a generic schema based on the struct
		return c.buildGenericSchema(t)
	}
}

// buildGenericSchema builds a JSON schema for any struct type
func (c *OpenAILLMClient) buildGenericSchema(t reflect.Type) map[string]interface{} {
	if t.Kind() != reflect.Struct {
		return map[string]interface{}{
			"type":                 "object",
			"additionalProperties": false, // OpenAI requires this to be false
		}
	}

	properties := make(map[string]interface{})
	required := []string{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// Extract field name from json tag
		fieldName := strings.Split(jsonTag, ",")[0]
		required = append(required, fieldName)

		// Build schema for field type
		properties[fieldName] = c.buildFieldSchema(field.Type)
	}

	return map[string]interface{}{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

// buildFieldSchema builds schema for a single field
func (c *OpenAILLMClient) buildFieldSchema(t reflect.Type) map[string]interface{} {
	switch t.Kind() {
	case reflect.String:
		return map[string]interface{}{"type": "string"}
	case reflect.Int, reflect.Int32, reflect.Int64:
		return map[string]interface{}{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]interface{}{"type": "number"}
	case reflect.Bool:
		return map[string]interface{}{"type": "boolean"}
	case reflect.Slice:
		return map[string]interface{}{
			"type":  "array",
			"items": c.buildFieldSchema(t.Elem()),
		}
	case reflect.Map:
		// For maps, since OpenAI structured outputs have limitations,
		// we'll use object type with explicit additionalProperties
		valueType := t.Elem()

		// Special case for map[string][]string (like compliance_mappings)
		if valueType.Kind() == reflect.Slice && valueType.Elem().Kind() == reflect.String {
			return map[string]interface{}{
				"type":        "object",
				"description": "Object mapping keys to arrays of strings",
				"additionalProperties": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			}
		}

		// For simple map[string]string
		if valueType.Kind() == reflect.String {
			return map[string]interface{}{
				"type":        "object",
				"description": "Object mapping string keys to string values",
				"additionalProperties": map[string]interface{}{
					"type": "string",
				},
			}
		}

		// For map[string]interface{} (generic objects)
		return map[string]interface{}{
			"type":                 "object",
			"description":          "Generic object with string keys",
			"additionalProperties": true,
		}
	case reflect.Struct:
		return c.buildGenericSchema(t)
	case reflect.Interface:
		// For interface{} types, we need to be more specific
		// If it's expected to be an object with unknown properties, use additionalProperties
		return map[string]interface{}{
			"type":                 "object",
			"additionalProperties": false, // OpenAI requires this to be false
		}
	default:
		return map[string]interface{}{"type": "string"}
	}
}
