package mcp

import (
	"context"
	"embed"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

//go:embed resources/*.md
var resourceFiles embed.FS

// registerResources registers all MCP resources for rule templates and examples
func (ms *MCPServer) registerResources() error {
	// Register CEL examples resource
	if err := ms.registerCELExamplesResource(); err != nil {
		return fmt.Errorf("failed to register CEL examples resource: %w", err)
	}

	// Register best practices resource
	if err := ms.registerBestPracticesResource(); err != nil {
		return fmt.Errorf("failed to register best practices resource: %w", err)
	}

	return nil
}

// registerCELExamplesResource registers a resource containing CEL expression examples and guide
func (ms *MCPServer) registerCELExamplesResource() error {
	resource := mcp.NewResource(
		"cel://examples/basic",
		"CEL Rule Generation Guide",
		mcp.WithResourceDescription("Step-by-step guide for generating CEL validation rules with examples and patterns"),
		mcp.WithMIMEType("text/markdown"),
	)

	handler := func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		// Read the CEL examples markdown file
		celExamplesData, err := resourceFiles.ReadFile("resources/cel_examples.md")
		if err != nil {
			return nil, fmt.Errorf("failed to read CEL examples file: %w", err)
		}

		content := mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/markdown",
			Text:     string(celExamplesData),
		}

		return []mcp.ResourceContents{content}, nil
	}

	return ms.registerResource(resource, handler)
}

// registerBestPracticesResource registers a resource containing best practices
func (ms *MCPServer) registerBestPracticesResource() error {
	resource := mcp.NewResource(
		"guide://best-practices/all",
		"Rule Writing Best Practices",
		mcp.WithResourceDescription("Comprehensive guide for writing effective CEL validation rules"),
		mcp.WithMIMEType("text/markdown"),
	)

	handler := func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		// Read the best practices markdown file
		bestPracticesData, err := resourceFiles.ReadFile("resources/best_practices.md")
		if err != nil {
			return nil, fmt.Errorf("failed to read best practices file: %w", err)
		}

		content := mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/markdown",
			Text:     string(bestPracticesData),
		}

		return []mcp.ResourceContents{content}, nil
	}

	return ms.registerResource(resource, handler)
}

// registerResource registers a resource with the MCP server
func (ms *MCPServer) registerResource(resource mcp.Resource, handler func(context.Context, mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)) error {
	ms.server.AddResource(resource, handler)
	return nil
}
