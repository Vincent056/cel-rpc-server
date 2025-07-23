package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"connectrpc.com/connect"
	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
	mcp "github.com/mark3labs/mcp-go/mcp"
)

// TestCaseInput represents a test case with expected result and inputs
type TestCaseInput struct {
	ID             string                 `json:"id,omitempty"`
	Description    string                 `json:"description,omitempty"`
	ExpectedResult bool                   `json:"expected_result"`
	Inputs         map[string]interface{} `json:"inputs"`
}

// VerifyCELTestCasesInput represents the input for the verify_cel_with_tests tool
type VerifyCELTestCasesInput struct {
	Expression string `json:"expression"`
	Inputs     []struct {
		Name       string                 `json:"name"`
		Type       string                 `json:"type"`
		Kubernetes *KubernetesInputConfig `json:"kubernetes,omitempty"`
		File       *FileInputConfig       `json:"file,omitempty"`
		HTTP       *HTTPInputConfig       `json:"http,omitempty"`
	} `json:"inputs"`
	TestCases []TestCaseInput `json:"test_cases"`
}

// registerVerifyCELTestCasesTool registers the verify CEL with test cases tool
func (ms *MCPServer) registerVerifyCELTestCasesTool() error {
	tool := mcp.Tool{
		Name:        "verify_cel_with_tests",
		Description: "Verify CEL expressions using predefined test data without requiring cluster access",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"expression": map[string]interface{}{
					"type":        "string",
					"description": "The CEL expression to verify",
				},
				"inputs": map[string]interface{}{
					"type":        "array",
					"description": "Input sources for the CEL expression. Can be Kubernetes resources, files, or HTTP endpoints",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name": map[string]interface{}{
								"type":        "string",
								"description": "Variable name to use in CEL expression",
							},
							"type": map[string]interface{}{
								"type":        "string",
								"enum":        []string{"kubernetes", "file", "http"},
								"description": "Type of input source",
							},
							"kubernetes": map[string]interface{}{
								"type":        "object",
								"description": "Kubernetes resource configuration (when type is 'kubernetes')",
								"properties": map[string]interface{}{
									"group": map[string]interface{}{
										"type":        "string",
										"description": "API group (empty for core resources)",
									},
									"version": map[string]interface{}{
										"type":        "string",
										"description": "API version (e.g., 'v1')",
									},
									"resource": map[string]interface{}{
										"type":        "string",
										"description": "Resource type (plural, e.g., 'pods', 'configmaps')",
									},
								},
								"required": []string{"version", "resource"},
							},
							"file": map[string]interface{}{
								"type":        "object",
								"description": "File input configuration (when type is 'file')",
								"properties": map[string]interface{}{
									"path": map[string]interface{}{
										"type":        "string",
										"description": "Path to the file",
									},
									"format": map[string]interface{}{
										"type":        "string",
										"enum":        []string{"json", "yaml"},
										"description": "File format",
									},
								},
								"required": []string{"path", "format"},
							},
							"http": map[string]interface{}{
								"type":        "object",
								"description": "HTTP endpoint configuration (when type is 'http')",
								"properties": map[string]interface{}{
									"url": map[string]interface{}{
										"type":        "string",
										"description": "HTTP(S) endpoint URL",
									},
									"method": map[string]interface{}{
										"type":        "string",
										"enum":        []string{"GET", "POST"},
										"description": "HTTP method",
									},
									"headers": map[string]interface{}{
										"type":        "object",
										"description": "HTTP headers",
									},
								},
								"required": []string{"url"},
							},
						},
						"required": []string{"name", "type"},
					},
				},
				"test_cases": map[string]interface{}{
					"type":        "array",
					"description": "Test cases with expected results and sample data for each input variable",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id": map[string]interface{}{
								"type":        "string",
								"description": "Test case ID (optional)",
							},
							"description": map[string]interface{}{
								"type":        "string",
								"description": "Test case description",
							},
							"expected_result": map[string]interface{}{
								"type":        "boolean",
								"description": "Expected result of the CEL expression evaluation",
							},
							"inputs": map[string]interface{}{
								"type":        "object",
								"description": "Input data mapping variable names to their test values",
							},
						},
						"required": []string{"expected_result", "inputs"},
					},
				},
			},
			Required: []string{"expression", "inputs", "test_cases"},
		},
	}

	return ms.registerTool(tool, ms.handleVerifyCELTestCases)
}

// handleVerifyCELTestCases handles the verify CEL with test cases tool execution
func (ms *MCPServer) handleVerifyCELTestCases(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input VerifyCELTestCasesInput
	if err := req.BindArguments(&input); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to parse input: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Validate required fields
	if input.Expression == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Failed to parse input: expression is required",
				},
			},
			IsError: true,
		}, nil
	}

	if len(input.Inputs) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Failed to parse input: at least one input is required",
				},
			},
			IsError: true,
		}, nil
	}

	log.Printf("[MCP] Executing verify_cel_with_tests: %s", input.Expression)

	// Build the validation request
	valReq := &celv1.ValidateCELRequest{
		Expression: input.Expression,
		Inputs:     make([]*celv1.RuleInput, 0, len(input.Inputs)),
		TestCases:  make([]*celv1.RuleTestCase, 0, len(input.TestCases)),
	}

	// Convert inputs
	for _, inp := range input.Inputs {
		ruleInput := &celv1.RuleInput{
			Name: inp.Name,
		}

		switch inp.Type {
		case "kubernetes":
			if inp.Kubernetes == nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.TextContent{
							Type: "text",
							Text: fmt.Sprintf("Kubernetes config required for input %s", inp.Name),
						},
					},
					IsError: true,
				}, nil
			}
			ruleInput.InputType = &celv1.RuleInput_Kubernetes{
				Kubernetes: &celv1.KubernetesInput{
					Group:    inp.Kubernetes.Group,
					Version:  inp.Kubernetes.Version,
					Resource: inp.Kubernetes.Resource,
				},
			}
		case "file":
			if inp.File == nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.TextContent{
							Type: "text",
							Text: fmt.Sprintf("File config required for input %s", inp.Name),
						},
					},
					IsError: true,
				}, nil
			}
			ruleInput.InputType = &celv1.RuleInput_File{
				File: &celv1.FileInput{
					Path:   inp.File.Path,
					Format: inp.File.Format,
				},
			}
		case "http":
			if inp.HTTP == nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.TextContent{
							Type: "text",
							Text: fmt.Sprintf("HTTP config required for input %s", inp.Name),
						},
					},
					IsError: true,
				}, nil
			}
			ruleInput.InputType = &celv1.RuleInput_Http{
				Http: &celv1.HttpInput{
					Url:     inp.HTTP.URL,
					Method:  inp.HTTP.Method,
					Headers: inp.HTTP.Headers,
				},
			}
		default:
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Unknown input type: %s", inp.Type),
					},
				},
				IsError: true,
			}, nil
		}

		valReq.Inputs = append(valReq.Inputs, ruleInput)
	}

	// Convert test cases
	for i, tc := range input.TestCases {
		// Generate ID if not provided
		testID := tc.ID
		if testID == "" {
			testID = fmt.Sprintf("test-%d", i+1)
		}

		// Generate description if not provided
		description := tc.Description
		if description == "" {
			description = fmt.Sprintf("Test case %d", i+1)
		}

		testCase := &celv1.RuleTestCase{
			Id:             testID,
			Description:    description,
			ExpectedResult: tc.ExpectedResult,
			TestData:       make(map[string]string),
		}

		// Convert test data to JSON strings
		for varName, data := range tc.Inputs {
			jsonData, err := json.Marshal(data)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.TextContent{
							Type: "text",
							Text: fmt.Sprintf("Failed to marshal test data for %s in test case %s: %v", varName, testID, err),
						},
					},
					IsError: true,
				}, nil
			}
			testCase.TestData[varName] = string(jsonData)
		}

		valReq.TestCases = append(valReq.TestCases, testCase)
	}

	// Call the validation service using Connect
	resp, err := ms.service.ValidateCEL(ctx, connect.NewRequest(valReq))
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Validation failed: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Format the response
	result := formatValidationResponse(resp.Msg)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: result,
			},
		},
	}, nil
}
