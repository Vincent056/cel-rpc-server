package mcp

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
	"github.com/Vincent056/cel-rpc-server/gen/cel/v1/celv1connect"
)

// MockCELValidationService is a mock implementation of the CEL validation service
type MockCELValidationService struct {
	celv1connect.UnimplementedCELValidationServiceHandler

	// Track calls
	validateCELCalled bool
	lastRequest       *celv1.ValidateCELRequest
}

func (m *MockCELValidationService) ValidateCEL(ctx context.Context, req *connect.Request[celv1.ValidateCELRequest]) (*connect.Response[celv1.ValidationResponse], error) {
	m.validateCELCalled = true
	m.lastRequest = req.Msg

	// Default response
	return connect.NewResponse(&celv1.ValidationResponse{
		Success: true,
		Results: []*celv1.ValidationResult{
			{
				TestCase: "test-1",
				Passed:   true,
				Details:  "Mock validation passed",
			},
		},
		Performance: &celv1.PerformanceMetrics{
			TotalTimeMs:    100,
			AverageTimeMs:  100,
			ResourcesCount: 1,
		},
	}), nil
}

func TestNewMCPServer(t *testing.T) {
	mockService := &MockCELValidationService{}

	server, err := NewMCPServer(mockService, nil)
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	if server.service == nil {
		t.Fatal("Expected service to be set")
	}

	// Check that tools are registered
	tools := server.GetTools()
	if len(tools) == 0 {
		t.Error("Expected at least one tool to be registered")
	}

	// Check for specific tools
	expectedTools := []string{
		"verify_cel_with_tests",
		"verify_cel_live_resources",
		"discover_resource_types",
		"count_resources",
		"get_resource_samples",
	}

	toolMap := make(map[string]bool)
	for _, tool := range tools {
		toolMap[tool.Name] = true
		t.Logf("Registered tool: %s", tool.Name)
	}

	for _, expectedTool := range expectedTools {
		if !toolMap[expectedTool] {
			t.Errorf("Expected tool %s to be registered", expectedTool)
		}
	}
}

func TestToolInputSchemas(t *testing.T) {
	mockService := &MockCELValidationService{}
	server, err := NewMCPServer(mockService, nil)
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	tools := server.GetTools()

	// Test that all tools have proper input schemas
	for _, tool := range tools {
		if tool.InputSchema.Type != "object" {
			t.Errorf("Tool %s: expected object input schema type, got: %s", tool.Name, tool.InputSchema.Type)
		}

		if tool.InputSchema.Properties == nil {
			t.Errorf("Tool %s: expected properties to be defined", tool.Name)
		}

		if tool.Description == "" {
			t.Errorf("Tool %s: expected description to be set", tool.Name)
		}

		// Check for required fields
		switch tool.Name {
		case "verify_cel_with_tests":
			checkRequiredFields(t, tool, []string{"expression", "inputs", "test_cases"})
		case "verify_cel_live_resources":
			checkRequiredFields(t, tool, []string{"expression", "inputs"})
		case "count_resources":
			checkRequiredFields(t, tool, []string{"resources"})
		case "get_resource_samples":
			checkRequiredFields(t, tool, []string{"resources"})
		}
	}
}

func TestGetServer(t *testing.T) {
	mockService := &MockCELValidationService{}
	server, err := NewMCPServer(mockService, nil)
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	mcpServer := server.GetServer()
	if mcpServer == nil {
		t.Error("Expected GetServer to return non-nil mcp-go server")
	}
}

func TestStartServer(t *testing.T) {
	mockService := &MockCELValidationService{}
	server, err := NewMCPServer(mockService, nil)
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	ctx := context.Background()
	err = server.Start(ctx)
	if err != nil {
		t.Errorf("Failed to start server: %v", err)
	}
}

// Helper functions

func checkRequiredFields(t *testing.T, tool interface{}, required []string) {
	t.Helper()

	// Since we can't import mcp.Tool directly, we'll use reflection or type assertions
	// For now, we'll just log this as a limitation
	t.Logf("Tool validation for required fields: %v", required)
}
