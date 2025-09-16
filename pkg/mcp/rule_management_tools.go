package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
	"github.com/google/uuid"
	mcp "github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/encoding/protojson"
)

// AddRuleInput represents the input for add_rule tool
type AddRuleInput struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Expression  string             `json:"expression"`
	Inputs      []RuleInputConfig  `json:"inputs"`
	Tags        []string           `json:"tags,omitempty"`
	Category    string             `json:"category,omitempty"`
	Severity    string             `json:"severity,omitempty"`
	TestCases   []TestCaseInput    `json:"test_cases,omitempty"`
	Metadata    *RuleMetadataInput `json:"metadata,omitempty"`
}

// RuleInputConfig represents a rule input configuration
type RuleInputConfig struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Kubernetes *KubernetesInputConfig `json:"kubernetes,omitempty"`
	File       *FileInputConfig       `json:"file,omitempty"`
	HTTP       *HTTPInputConfig       `json:"http,omitempty"`
}

// RuleMetadataInput represents rule metadata
type RuleMetadataInput struct {
	ComplianceFramework string   `json:"compliance_framework,omitempty"`
	ControlIDs          []string `json:"control_ids,omitempty"`
	References          []string `json:"references,omitempty"`
}

// ListRulesInput represents the input for list_rules tool
type ListRulesInput struct {
	Tags                []string `json:"tags,omitempty"`
	Category            string   `json:"category,omitempty"`
	Severity            string   `json:"severity,omitempty"`
	ComplianceFramework string   `json:"compliance_framework,omitempty"`
	ResourceType        string   `json:"resource_type,omitempty"`
	SearchText          string   `json:"search_text,omitempty"`
	VerifiedOnly        bool     `json:"verified_only,omitempty"`
	PageSize            int32    `json:"page_size,omitempty"`
}

// RemoveRuleInput represents the input for remove_rule tool
type RemoveRuleInput struct {
	RuleID string `json:"rule_id"`
}

// registerAddRuleTool registers the add rule tool
func (ms *MCPServer) registerAddRuleTool() error {
	tool := mcp.Tool{
		Name:        "add_rule",
		Description: "Add a new CEL rule to the rule library",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the rule",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Description of what the rule checks",
				},
				"expression": map[string]interface{}{
					"type":        "string",
					"description": "The CEL expression for the rule",
				},
				"inputs": map[string]interface{}{
					"type":        "array",
					"description": "Input sources for the CEL expression",
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
								"description": "Kubernetes resource configuration",
								"properties": map[string]interface{}{
									"group": map[string]interface{}{
										"type":        "string",
										"description": "API group",
									},
									"version": map[string]interface{}{
										"type":        "string",
										"description": "API version",
									},
									"resource": map[string]interface{}{
										"type":        "string",
										"description": "Resource type (plural)",
									},
									"namespace": map[string]interface{}{
										"type":        "string",
										"description": "Namespace",
									},
								},
								"required": []string{"version", "resource"},
							},
						},
						"required": []string{"name", "type"},
					},
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"description": "Tags for categorizing the rule",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"category": map[string]interface{}{
					"type":        "string",
					"description": "Rule category (e.g., security, compliance, performance)",
				},
				"severity": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"low", "medium", "high", "critical"},
					"description": "Severity level of the rule",
				},
				"test_cases": map[string]interface{}{
					"type":        "array",
					"description": "Test cases for the rule",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"description": map[string]interface{}{
								"type":        "string",
								"description": "Description of the test case",
							},
							"expected_result": map[string]interface{}{
								"type":        "boolean",
								"description": "Expected result of the CEL expression",
							},
							"inputs": map[string]interface{}{
								"type":        "object",
								"description": "Test data for each input variable",
							},
						},
						"required": []string{"expected_result", "inputs"},
					},
				},
				"metadata": map[string]interface{}{
					"type":        "object",
					"description": "Additional metadata for the rule",
					"properties": map[string]interface{}{
						"compliance_framework": map[string]interface{}{
							"type":        "string",
							"description": "Compliance framework (e.g., CIS, PCI-DSS, HIPAA)",
						},
						"control_ids": map[string]interface{}{
							"type":        "array",
							"description": "Control IDs from the compliance framework",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						"references": map[string]interface{}{
							"type":        "array",
							"description": "References or documentation URLs",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
			Required: []string{"name", "description", "expression", "inputs"},
		},
	}

	return ms.registerTool(tool, ms.handleAddRule)
}

// registerListRulesTool registers the list rules tool
func (ms *MCPServer) registerListRulesTool() error {
	tool := mcp.Tool{
		Name:        "list_rules",
		Description: "List CEL rules from the rule library with optional filters",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tags": map[string]interface{}{
					"type":        "array",
					"description": "Filter by tags",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"category": map[string]interface{}{
					"type":        "string",
					"description": "Filter by category",
				},
				"severity": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"low", "medium", "high", "critical"},
					"description": "Filter by severity level",
				},
				"compliance_framework": map[string]interface{}{
					"type":        "string",
					"description": "Filter by compliance framework",
				},
				"resource_type": map[string]interface{}{
					"type":        "string",
					"description": "Filter by Kubernetes resource type",
				},
				"search_text": map[string]interface{}{
					"type":        "string",
					"description": "Search text to filter rules by name, description, or expression",
				},
				"verified_only": map[string]interface{}{
					"type":        "boolean",
					"description": "Show only verified rules",
				},
				"page_size": map[string]interface{}{
					"type":        "integer",
					"description": "Number of rules to return (default: 20, max: 100)",
				},
			},
		},
	}

	return ms.registerTool(tool, ms.handleListRules)
}

// registerRemoveRuleTool registers the remove rule tool
func (ms *MCPServer) registerRemoveRuleTool() error {
	tool := mcp.Tool{
		Name:        "remove_rule",
		Description: "Remove a CEL rule from the rule library",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"rule_id": map[string]interface{}{
					"type":        "string",
					"description": "ID of the rule to remove",
				},
			},
			Required: []string{"rule_id"},
		},
	}

	return ms.registerTool(tool, ms.handleRemoveRule)
}

// handleAddRule handles the add rule tool execution
func (ms *MCPServer) handleAddRule(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input AddRuleInput
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

	log.Printf("[MCP] Executing add_rule: %s", input.Name)

	// Convert input to CELRule
	rule := &celv1.CELRule{
		Id:          uuid.New().String(),
		Name:        input.Name,
		Description: input.Description,
		Expression:  input.Expression,
		Tags:        input.Tags,
		Category:    input.Category,
		Severity:    input.Severity,
	}

	// Convert inputs
	for _, inputCfg := range input.Inputs {
		ruleInput := &celv1.RuleInput{
			Name: inputCfg.Name,
		}

		switch inputCfg.Type {
		case "kubernetes":
			if inputCfg.Kubernetes != nil {
				ruleInput.InputType = &celv1.RuleInput_Kubernetes{
					Kubernetes: &celv1.KubernetesInput{
						Group:        inputCfg.Kubernetes.Group,
						Version:      inputCfg.Kubernetes.Version,
						Resource:     inputCfg.Kubernetes.Resource,
						Namespace:    inputCfg.Kubernetes.Namespace,
						ResourceName: inputCfg.Kubernetes.ResourceName,
					},
				}
			}
		case "file":
			if inputCfg.File != nil {
				ruleInput.InputType = &celv1.RuleInput_File{
					File: &celv1.FileInput{
						Path:   inputCfg.File.Path,
						Format: inputCfg.File.Format,
					},
				}
			}
		case "http":
			if inputCfg.HTTP != nil {
				ruleInput.InputType = &celv1.RuleInput_Http{
					Http: &celv1.HttpInput{
						Url:     inputCfg.HTTP.URL,
						Method:  inputCfg.HTTP.Method,
						Headers: inputCfg.HTTP.Headers,
					},
				}
			}
		}

		rule.Inputs = append(rule.Inputs, ruleInput)
	}

	// Convert test cases
	for _, tc := range input.TestCases {
		testCase := &celv1.RuleTestCase{
			Description:    tc.Description,
			ExpectedResult: tc.ExpectedResult,
		}

		// Convert test inputs to TestData map
		if tc.Inputs != nil {
			testCase.TestData = make(map[string]string)
			for key, value := range tc.Inputs {
				valueJSON, err := json.Marshal(value)
				if err == nil {
					testCase.TestData[key] = string(valueJSON)
				}
			}
		}

		rule.TestCases = append(rule.TestCases, testCase)
	}

	// Convert metadata
	if input.Metadata != nil {
		rule.Metadata = &celv1.RuleMetadata{
			ComplianceFramework: input.Metadata.ComplianceFramework,
			References:          input.Metadata.References,
		}
		// Store control IDs in custom fields if provided
		if len(input.Metadata.ControlIDs) > 0 {
			if rule.Metadata.CustomFields == nil {
				rule.Metadata.CustomFields = make(map[string]string)
			}
			controlIDsJSON, err := json.Marshal(input.Metadata.ControlIDs)
			if err == nil {
				rule.Metadata.CustomFields["control_ids"] = string(controlIDsJSON)
			}
		}
	}

	// Save the rule using the RuleStore
	if ms.ruleStore != nil {
		if err := ms.ruleStore.Save(rule); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Failed to save rule: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
	} else {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Rule store not available",
				},
			},
			IsError: true,
		}, nil
	}

	// Return success with rule details
	mo := protojson.MarshalOptions{UseProtoNames: true, Indent: "  "}
	ruleJSON, _ := mo.Marshal(rule)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Successfully added rule:\nID: %s\nName: %s\nDescription: %s\n\nFull rule:\n%s",
					rule.Id, rule.Name, rule.Description, string(ruleJSON)),
			},
		},
	}, nil
}

// handleListRules handles the list rules tool execution
func (ms *MCPServer) handleListRules(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input ListRulesInput
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

	log.Printf("[MCP] Executing list_rules with filters")

	// Set default page size
	if input.PageSize == 0 {
		input.PageSize = 20
	} else if input.PageSize > 100 {
		input.PageSize = 100
	}

	// Create filter
	filter := &celv1.ListRulesFilter{
		Tags:                input.Tags,
		Category:            input.Category,
		Severity:            input.Severity,
		ComplianceFramework: input.ComplianceFramework,
		ResourceType:        input.ResourceType,
		SearchText:          input.SearchText,
		VerifiedOnly:        input.VerifiedOnly,
	}

	// List rules using the RuleStore
	if ms.ruleStore == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Rule store not available",
				},
			},
			IsError: true,
		}, nil
	}

	rules, nextPageToken, totalCount, err := ms.ruleStore.List(filter, input.PageSize, "", "name", true)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to list rules: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Format the response
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d rules (showing %d):\n\n", totalCount, len(rules)))

	for i, rule := range rules {
		result.WriteString(fmt.Sprintf("%d. %s (ID: %s)\n", i+1, rule.Name, rule.Id))
		result.WriteString(fmt.Sprintf("   Description: %s\n", rule.Description))

		if rule.Category != "" {
			result.WriteString(fmt.Sprintf("   Category: %s\n", rule.Category))
		}

		if rule.Severity != "" {
			result.WriteString(fmt.Sprintf("   Severity: %s\n", rule.Severity))
		}

		if len(rule.Tags) > 0 {
			result.WriteString(fmt.Sprintf("   Tags: %v\n", rule.Tags))
		}

		if rule.IsVerified {
			result.WriteString("   Status: ✓ Verified\n")
		}

		// Show input types
		if len(rule.Inputs) > 0 {
			result.WriteString("   Inputs: ")
			var inputTypes []string
			for _, input := range rule.Inputs {
				switch input.GetInputType().(type) {
				case *celv1.RuleInput_Kubernetes:
					k8s := input.GetKubernetes()
					inputTypes = append(inputTypes, fmt.Sprintf("%s (k8s: %s)", input.Name, k8s.Resource))
				case *celv1.RuleInput_File:
					inputTypes = append(inputTypes, fmt.Sprintf("%s (file)", input.Name))
				case *celv1.RuleInput_Http:
					inputTypes = append(inputTypes, fmt.Sprintf("%s (http)", input.Name))
				}
			}
			result.WriteString(strings.Join(inputTypes, ", ") + "\n")
		}

		result.WriteString("\n")
	}

	if nextPageToken != "" {
		result.WriteString(fmt.Sprintf("\nMore results available. Next page token: %s\n", nextPageToken))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: result.String(),
			},
		},
	}, nil
}

// handleRemoveRule handles the remove rule tool execution
func (ms *MCPServer) handleRemoveRule(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input RemoveRuleInput
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

	log.Printf("[MCP] Executing remove_rule: %s", input.RuleID)

	// Remove the rule using the RuleStore
	if ms.ruleStore == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Rule store not available",
				},
			},
			IsError: true,
		}, nil
	}

	// First, try to get the rule details before deletion
	rule, err := ms.ruleStore.Get(input.RuleID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Rule not found: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Delete the rule
	if err := ms.ruleStore.Delete(input.RuleID); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to remove rule: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Successfully removed rule:\nID: %s\nName: %s\nDescription: %s",
					rule.Id, rule.Name, rule.Description),
			},
		},
	}, nil
}
