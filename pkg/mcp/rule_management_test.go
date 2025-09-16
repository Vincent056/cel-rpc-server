package mcp

import (
	"context"
	"fmt"
	"testing"

	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
	"github.com/mark3labs/mcp-go/mcp"
)

// MockRuleStore is a mock implementation of RuleStore for testing
type MockRuleStore struct {
	rules map[string]*celv1.CELRule
}

func NewMockRuleStore() *MockRuleStore {
	return &MockRuleStore{
		rules: make(map[string]*celv1.CELRule),
	}
}

func (m *MockRuleStore) Save(rule *celv1.CELRule) error {
	m.rules[rule.Id] = rule
	return nil
}

func (m *MockRuleStore) Get(id string) (*celv1.CELRule, error) {
	rule, exists := m.rules[id]
	if !exists {
		return nil, fmt.Errorf("rule not found")
	}
	return rule, nil
}

func (m *MockRuleStore) List(filter *celv1.ListRulesFilter, pageSize int32, pageToken string, sortBy string, ascending bool) ([]*celv1.CELRule, string, int32, error) {
	var rules []*celv1.CELRule
	for _, rule := range m.rules {
		rules = append(rules, rule)
	}
	return rules, "", int32(len(rules)), nil
}

func (m *MockRuleStore) Delete(id string) error {
	delete(m.rules, id)
	return nil
}

func TestRuleManagementTools(t *testing.T) {
	mockService := &MockCELValidationService{}
	mockRuleStore := NewMockRuleStore()

	// Create MCP server with rule store
	server, err := NewMCPServer(mockService, mockRuleStore)
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	// Check that rule management tools are registered
	tools := server.GetTools()
	ruleTools := []string{"add_rule", "list_rules", "remove_rule"}
	foundTools := make(map[string]bool)

	for _, tool := range tools {
		foundTools[tool.Name] = true
	}

	for _, toolName := range ruleTools {
		if !foundTools[toolName] {
			t.Errorf("Expected tool %s to be registered", toolName)
		} else {
			t.Logf("Found rule management tool: %s", toolName)
		}
	}
}

func TestAddRuleTool(t *testing.T) {
	mockService := &MockCELValidationService{}
	mockRuleStore := NewMockRuleStore()

	server, err := NewMCPServer(mockService, mockRuleStore)
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	// Test add_rule handler
	handler := server.GetToolHandlers()["add_rule"]
	if handler == nil {
		t.Fatal("add_rule handler not found")
	}

	// Create test input
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "add_rule",
			Arguments: map[string]interface{}{
				"name":        "Test Rule",
				"description": "This is a test rule",
				"expression":  "pods.items.all(pod, pod.spec.securityContext.runAsNonRoot == true)",
				"inputs": []map[string]interface{}{
					{
						"name": "pods",
						"type": "kubernetes",
						"kubernetes": map[string]interface{}{
							"version":  "v1",
							"resource": "pods",
						},
					},
				},
				"category": "security",
				"severity": "high",
				"tags":     []string{"test", "security"},
			},
		},
	}

	ctx := context.Background()
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Handler returned error result: %v", result.Content)
	}

	// Check that rule was added to store
	if len(mockRuleStore.rules) != 1 {
		t.Errorf("Expected 1 rule in store, got %d", len(mockRuleStore.rules))
	}

	t.Logf("Successfully added rule to store")
}

func TestListRulesTool(t *testing.T) {
	mockService := &MockCELValidationService{}
	mockRuleStore := NewMockRuleStore()

	// Add a test rule to the store
	mockRuleStore.Save(&celv1.CELRule{
		Id:          "test-rule-1",
		Name:        "Test Rule 1",
		Description: "First test rule",
		Expression:  "true",
		Category:    "security",
		Severity:    "high",
		Tags:        []string{"test"},
	})

	server, err := NewMCPServer(mockService, mockRuleStore)
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	// Test list_rules handler
	handler := server.GetToolHandlers()["list_rules"]
	if handler == nil {
		t.Fatal("list_rules handler not found")
	}

	// Create test input
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "list_rules",
			Arguments: map[string]interface{}{
				"page_size": 10,
			},
		},
	}

	ctx := context.Background()
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Handler returned error result: %v", result.Content)
	}

	t.Logf("Successfully listed rules from store")
}

func TestRemoveRuleTool(t *testing.T) {
	mockService := &MockCELValidationService{}
	mockRuleStore := NewMockRuleStore()

	// Add a test rule to the store
	ruleID := "test-rule-to-remove"
	mockRuleStore.Save(&celv1.CELRule{
		Id:          ruleID,
		Name:        "Test Rule to Remove",
		Description: "This rule will be removed",
		Expression:  "true",
	})

	server, err := NewMCPServer(mockService, mockRuleStore)
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	// Test remove_rule handler
	handler := server.GetToolHandlers()["remove_rule"]
	if handler == nil {
		t.Fatal("remove_rule handler not found")
	}

	// Create test input
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "remove_rule",
			Arguments: map[string]interface{}{
				"rule_id": ruleID,
			},
		},
	}

	ctx := context.Background()
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Handler returned error result: %v", result.Content)
	}

	// Check that rule was removed from store
	if len(mockRuleStore.rules) != 0 {
		t.Errorf("Expected 0 rules in store, got %d", len(mockRuleStore.rules))
	}

	t.Logf("Successfully removed rule from store")
}

func TestTestRuleTool(t *testing.T) {
	mockService := &MockCELValidationService{}
	mockRuleStore := NewMockRuleStore()

	// Add a test rule with test cases to the store
	ruleID := "test-rule-with-cases"
	mockRuleStore.Save(&celv1.CELRule{
		Id:          ruleID,
		Name:        "Test Rule With Cases",
		Description: "This rule has test cases",
		Expression:  "input.value > 10",
		Inputs: []*celv1.RuleInput{
			{
				Name: "input",
				InputType: &celv1.RuleInput_Kubernetes{
					Kubernetes: &celv1.KubernetesInput{
						Version:  "v1",
						Resource: "configmaps",
					},
				},
			},
		},
		TestCases: []*celv1.RuleTestCase{
			{
				Id:             "test-1",
				Description:    "Test with value > 10",
				ExpectedResult: true,
				TestData: map[string]string{
					"input": `{"value": 15}`,
				},
			},
			{
				Id:             "test-2",
				Description:    "Test with value < 10",
				ExpectedResult: false,
				TestData: map[string]string{
					"input": `{"value": 5}`,
				},
			},
		},
	})

	server, err := NewMCPServer(mockService, mockRuleStore)
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	// Test test_rule handler
	handler := server.GetToolHandlers()["test_rule"]
	if handler == nil {
		t.Fatal("test_rule handler not found")
	}

	// Test with test_cases mode
	t.Run("TestCasesMode", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "test_rule",
				Arguments: map[string]interface{}{
					"rule_id":   ruleID,
					"test_mode": "test_cases",
				},
			},
		}

		ctx := context.Background()
		result, err := handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		// Result may have errors from actual test execution, but the handler itself should work
		t.Logf("Test result for test_cases mode: IsError=%v", result.IsError)
	})

	// Test with custom test data
	t.Run("CustomTestData", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "test_rule",
				Arguments: map[string]interface{}{
					"rule_id":   ruleID,
					"test_mode": "test_cases",
					"test_data": map[string]interface{}{
						"input": map[string]interface{}{
							"value": 20,
						},
					},
				},
			},
		}

		ctx := context.Background()
		result, err := handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		t.Logf("Test result for custom data: IsError=%v", result.IsError)
	})

	// Test with invalid rule ID
	t.Run("InvalidRuleID", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "test_rule",
				Arguments: map[string]interface{}{
					"rule_id": "non-existent-rule",
				},
			},
		}

		ctx := context.Background()
		result, err := handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error result for non-existent rule")
		}
	})
}
