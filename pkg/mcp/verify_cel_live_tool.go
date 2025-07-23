package mcp

import (
	"context"
	"fmt"
	"log"

	"connectrpc.com/connect"
	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
	mcp "github.com/mark3labs/mcp-go/mcp"
)

// VerifyCELLiveInput represents the input for the verify_cel_live_resources tool
type VerifyCELLiveInput struct {
	Expression string `json:"expression"`
	Inputs     []struct {
		Name      string `json:"name"`
		Group     string `json:"group"`
		Version   string `json:"version"`
		Resource  string `json:"resource"`
		Namespace string `json:"namespace,omitempty"`
		Limit     int32  `json:"limit,omitempty"`
	} `json:"inputs"`
}

// registerVerifyCELLiveTool registers the verify CEL live resources tool
func (ms *MCPServer) registerVerifyCELLiveTool() error {
	tool := mcp.Tool{
		Name:        "verify_cel_live_resources",
		Description: "Verify CEL expressions against live Kubernetes resources. Requires cluster access",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"expression": map[string]interface{}{
					"type":        "string",
					"description": "The CEL expression to verify (must use .items.all() format due to List wrapping)",
				},
				"inputs": map[string]interface{}{
					"type":        "array",
					"description": "Kubernetes resources to fetch from the cluster",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name": map[string]interface{}{
								"type":        "string",
								"description": "Variable name to use in CEL expression",
							},
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
							"namespace": map[string]interface{}{
								"type":        "string",
								"description": "Namespace to query (empty for all namespaces or cluster-scoped resources)",
							},
						},
						"required": []string{"name", "version", "resource"},
					},
				},
			},
			Required: []string{"expression", "inputs"},
		},
	}

	return ms.registerTool(tool, ms.handleVerifyCELLive)
}

// handleVerifyCELLive handles the verify CEL live resources tool execution
func (ms *MCPServer) handleVerifyCELLive(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input VerifyCELLiveInput
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

	log.Printf("[MCP] Executing verify_cel_live_resources: %s", input.Expression)
	log.Printf("[MCP] Fetching %d resource types from cluster", len(input.Inputs))

	// Build the validation request
	valReq := &celv1.ValidateCELRequest{
		Expression: input.Expression,
		Inputs:     make([]*celv1.RuleInput, 0, len(input.Inputs)),
	}

	// Convert inputs to Kubernetes inputs
	for _, inp := range input.Inputs {
		ruleInput := &celv1.RuleInput{
			Name: inp.Name,
			InputType: &celv1.RuleInput_Kubernetes{
				Kubernetes: &celv1.KubernetesInput{
					Group:     inp.Group,
					Version:   inp.Version,
					Resource:  inp.Resource,
					Namespace: inp.Namespace,
				},
			},
		}
		valReq.Inputs = append(valReq.Inputs, ruleInput)
	}

	// Call the validation service
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
