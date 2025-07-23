package mcp

import (
	"context"
	"fmt"
	"log"
	"strings"

	"connectrpc.com/connect"
	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
	mcp "github.com/mark3labs/mcp-go/mcp"
)

// DiscoverResourceTypesInput represents the input for discover_resource_types tool
type DiscoverResourceTypesInput struct {
	Namespace string `json:"namespace,omitempty"`
}

// CountResourcesInput represents the input for count_resources tool
type CountResourcesInput struct {
	Resources []string `json:"resources"`
	Namespace string   `json:"namespace,omitempty"`
}

// GetResourceSamplesInput represents the input for get_resource_samples tool
type GetResourceSamplesInput struct {
	Resources []string `json:"resources"`
	Namespace string   `json:"namespace,omitempty"`
	Limit     int32    `json:"limit,omitempty"`
}

// registerDiscoverResourceTypesTool registers the discover resource types tool
func (ms *MCPServer) registerDiscoverResourceTypesTool() error {
	tool := mcp.Tool{
		Name:        "discover_resource_types",
		Description: "Discover available Kubernetes resource types in the cluster (fast mode, no counts)",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Namespace to filter resources (empty for all namespaces)",
				},
			},
		},
	}

	return ms.registerTool(tool, ms.handleDiscoverResourceTypes)
}

// registerCountResourcesTool registers the count resources tool
func (ms *MCPServer) registerCountResourcesTool() error {
	tool := mcp.Tool{
		Name:        "count_resources",
		Description: "Count instances of specific Kubernetes resource types",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"resources": map[string]interface{}{
					"type":        "array",
					"description": "Resource types to count (format: 'group.version/resource' or 'version/resource' for core resources)",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Namespace to filter resources (empty for all namespaces)",
				},
			},
			Required: []string{"resources"},
		},
	}

	return ms.registerTool(tool, ms.handleCountResources)
}

// registerGetResourceSamplesTool registers the get resource samples tool
func (ms *MCPServer) registerGetResourceSamplesTool() error {
	tool := mcp.Tool{
		Name:        "get_resource_samples",
		Description: "Get sample instances of specific Kubernetes resource types",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"resources": map[string]interface{}{
					"type":        "array",
					"description": "Resource types to get samples from (format: 'group.version/resource' or 'version/resource' for core resources)",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Namespace to filter resources (empty for all namespaces)",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of samples per resource type (default: 3)",
				},
			},
			Required: []string{"resources"},
		},
	}

	return ms.registerTool(tool, ms.handleGetResourceSamples)
}

// handleDiscoverResourceTypes handles the discover resource types tool execution
func (ms *MCPServer) handleDiscoverResourceTypes(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input DiscoverResourceTypesInput
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

	log.Printf("[MCP] Executing discover_resource_types (fast mode)")

	// Call discovery service with fast option (no counts, no samples)
	discReq := &celv1.DiscoverResourcesRequest{
		Namespace: input.Namespace,
		Options: &celv1.DiscoveryOptions{
			IncludeCount:   false,
			IncludeSamples: false,
		},
	}

	resp, err := ms.service.DiscoverResources(ctx, connect.NewRequest(discReq))
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Discovery failed: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Format the response
	result := formatDiscoveryResponse(resp.Msg, true)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: result,
			},
		},
	}, nil
}

// handleCountResources handles the count resources tool execution
func (ms *MCPServer) handleCountResources(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input CountResourcesInput
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

	log.Printf("[MCP] Executing count_resources for %d resource types", len(input.Resources))

	// Convert resource strings to GVR filters
	filters := make([]*celv1.GVRFilter, 0, len(input.Resources))
	for _, res := range input.Resources {
		filter := parseResourceString(res)
		if filter != nil {
			filters = append(filters, filter)
		}
	}

	// Call discovery service with count option
	discReq := &celv1.DiscoverResourcesRequest{
		Namespace:  input.Namespace,
		GvrFilters: filters,
		Options: &celv1.DiscoveryOptions{
			IncludeCount:   true,
			IncludeSamples: false,
		},
	}

	resp, err := ms.service.DiscoverResources(ctx, connect.NewRequest(discReq))
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Discovery failed: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Format the response focusing on counts
	result := formatCountResponse(resp.Msg)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: result,
			},
		},
	}, nil
}

// handleGetResourceSamples handles the get resource samples tool execution
func (ms *MCPServer) handleGetResourceSamples(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input GetResourceSamplesInput
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

	log.Printf("[MCP] Executing get_resource_samples for %d resource types", len(input.Resources))

	// Convert resource strings to GVR filters
	filters := make([]*celv1.GVRFilter, 0, len(input.Resources))
	for _, res := range input.Resources {
		filter := parseResourceString(res)
		if filter != nil {
			filters = append(filters, filter)
		}
	}

	// Default limit
	limit := input.Limit
	if limit == 0 {
		limit = 3
	}

	// Call discovery service with samples option
	discReq := &celv1.DiscoverResourcesRequest{
		Namespace:  input.Namespace,
		GvrFilters: filters,
		Options: &celv1.DiscoveryOptions{
			IncludeCount:      false,
			IncludeSamples:    true,
			MaxSamplesPerType: limit,
		},
	}

	resp, err := ms.service.DiscoverResources(ctx, connect.NewRequest(discReq))
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Discovery failed: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Format the response focusing on samples
	result := formatSamplesResponse(resp.Msg)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: result,
			},
		},
	}, nil
}

// Helper functions for formatting responses

func formatDiscoveryResponse(resp *celv1.ResourceDiscoveryResponse, fastMode bool) string {
	var result strings.Builder

	if fastMode {
		result.WriteString("## Available Resource Types (Fast Discovery)\n\n")
	} else {
		result.WriteString("## Discovered Resources\n\n")
	}

	for _, res := range resp.Resources {
		result.WriteString(fmt.Sprintf("- **%s**", res.ApiVersion))
		if res.Kind != "" {
			result.WriteString(fmt.Sprintf(" (%s)", res.Kind))
		}

		if res.Namespace != "" {
			result.WriteString(fmt.Sprintf(" in namespace %s", res.Namespace))
		}

		result.WriteString("\n")
	}

	result.WriteString(fmt.Sprintf("\n**Total resource types:** %d\n", len(resp.Resources)))

	if resp.Error != "" {
		result.WriteString(fmt.Sprintf("\n**Error:** %s\n", resp.Error))
	}

	return result.String()
}

func formatCountResponse(resp *celv1.ResourceDiscoveryResponse) string {
	var result strings.Builder

	result.WriteString("## Resource Counts\n\n")

	totalCount := int32(0)
	for _, res := range resp.Resources {
		result.WriteString(fmt.Sprintf("- **%s**: %d instances\n", res.ApiVersion, res.Count))
		if res.Kind != "" {
			result.WriteString(fmt.Sprintf("  Kind: %s\n", res.Kind))
		}
		totalCount += res.Count
	}

	result.WriteString(fmt.Sprintf("\n**Total instances across all types:** %d\n", totalCount))

	return result.String()
}

func formatSamplesResponse(resp *celv1.ResourceDiscoveryResponse) string {
	var result strings.Builder

	result.WriteString("## Resource Samples\n\n")

	for _, res := range resp.Resources {
		result.WriteString(fmt.Sprintf("### %s", res.ApiVersion))
		if res.Kind != "" {
			result.WriteString(fmt.Sprintf(" (%s)", res.Kind))
		}
		result.WriteString("\n\n")

		if len(res.Samples) == 0 {
			result.WriteString("No instances found.\n\n")
			continue
		}

		result.WriteString("**Sample instances:**\n")
		for _, sample := range res.Samples {
			result.WriteString(fmt.Sprintf("- %s", sample.Name))
			if sample.Namespace != "" {
				result.WriteString(fmt.Sprintf(" (namespace: %s)", sample.Namespace))
			}
			result.WriteString("\n")
		}
		result.WriteString("\n")
	}

	return result.String()
}

func formatGVR(group, version, resource string) string {
	if group == "" {
		return fmt.Sprintf("%s/%s", version, resource)
	}
	return fmt.Sprintf("%s.%s/%s", group, version, resource)
}

func parseResourceString(res string) *celv1.GVRFilter {
	// Parse formats like "apps.v1/deployments" or "v1/pods"
	parts := strings.Split(res, "/")
	if len(parts) != 2 {
		return nil
	}

	gv := parts[0]
	resource := parts[1]

	// Check if it contains a group
	if strings.Contains(gv, ".") {
		gvParts := strings.SplitN(gv, ".", 2)
		return &celv1.GVRFilter{
			Group:    gvParts[0],
			Version:  gvParts[1],
			Resource: resource,
		}
	}

	// No group, just version
	return &celv1.GVRFilter{
		Version:  gv,
		Resource: resource,
	}
}
