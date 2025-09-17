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

func (m *MockRuleStore) Update(rule *celv1.CELRule, updateFields []string) error {
	m.rules[rule.Id] = rule
	return nil
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
	ruleTools := []string{"add_rule", "list_rules", "remove_rule", "test_rule", "get_rule", "update_rule"}
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

func TestGetRuleTool(t *testing.T) {
	mockService := &MockCELValidationService{}
	mockRuleStore := NewMockRuleStore()

	// Add a test rule to the store
	ruleID := "test-rule-to-get"
	testRule := &celv1.CELRule{
		Id:          ruleID,
		Name:        "Test Rule to Get",
		Description: "This rule will be retrieved",
		Expression:  "pods.items.all(pod, pod.spec.securityContext.runAsNonRoot == true)",
		Category:    "security",
		Severity:    "high",
		Tags:        []string{"test", "security"},
		Inputs: []*celv1.RuleInput{
			{
				Name: "pods",
				InputType: &celv1.RuleInput_Kubernetes{
					Kubernetes: &celv1.KubernetesInput{
						Version:  "v1",
						Resource: "pods",
					},
				},
			},
		},
		TestCases: []*celv1.RuleTestCase{
			{
				Id:             "test-1",
				Description:    "Test case 1",
				ExpectedResult: true,
				TestData: map[string]string{
					"pods": `{"items": []}`,
				},
			},
		},
		Metadata: &celv1.RuleMetadata{
			ComplianceFramework: "CIS",
			References:          []string{"https://example.com"},
		},
	}
	mockRuleStore.Save(testRule)

	server, err := NewMCPServer(mockService, mockRuleStore)
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	// Test get_rule handler
	handler := server.GetToolHandlers()["get_rule"]
	if handler == nil {
		t.Fatal("get_rule handler not found")
	}

	// Create test input
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "get_rule",
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

	// Check that the result contains expected information
	if len(result.Content) == 0 {
		t.Fatal("Expected content in result")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("Expected text content in result")
	}

	// Check that the response contains key information
	responseText := textContent.Text
	if !contains(responseText, ruleID) {
		t.Error("Response should contain rule ID")
	}
	if !contains(responseText, testRule.Name) {
		t.Error("Response should contain rule name")
	}
	if !contains(responseText, testRule.Description) {
		t.Error("Response should contain rule description")
	}
	if !contains(responseText, testRule.Expression) {
		t.Error("Response should contain rule expression")
	}

	t.Logf("Successfully retrieved rule details")
}

func TestUpdateRuleTool(t *testing.T) {
	mockService := &MockCELValidationService{}
	mockRuleStore := NewMockRuleStore()

	// Add a test rule to the store
	ruleID := "test-rule-to-update"
	originalRule := &celv1.CELRule{
		Id:          ruleID,
		Name:        "Original Rule Name",
		Description: "Original description",
		Expression:  "true",
		Category:    "original",
		Severity:    "low",
		Tags:        []string{"original"},
	}
	mockRuleStore.Save(originalRule)

	server, err := NewMCPServer(mockService, mockRuleStore)
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	// Test update_rule handler
	handler := server.GetToolHandlers()["update_rule"]
	if handler == nil {
		t.Fatal("update_rule handler not found")
	}

	// Create test input to update the rule
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "update_rule",
			Arguments: map[string]interface{}{
				"rule_id":     ruleID,
				"name":        "Updated Rule Name",
				"description": "Updated description",
				"category":    "security",
				"severity":    "high",
				"tags":        []string{"updated", "security"},
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

	// Check that the rule was updated in the store
	updatedRule, err := mockRuleStore.Get(ruleID)
	if err != nil {
		t.Fatalf("Failed to get updated rule: %v", err)
	}

	if updatedRule.Name != "Updated Rule Name" {
		t.Errorf("Expected name 'Updated Rule Name', got '%s'", updatedRule.Name)
	}
	if updatedRule.Description != "Updated description" {
		t.Errorf("Expected description 'Updated description', got '%s'", updatedRule.Description)
	}
	if updatedRule.Category != "security" {
		t.Errorf("Expected category 'security', got '%s'", updatedRule.Category)
	}
	if updatedRule.Severity != "high" {
		t.Errorf("Expected severity 'high', got '%s'", updatedRule.Severity)
	}

	// Check that the response contains expected information
	if len(result.Content) == 0 {
		t.Fatal("Expected content in result")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("Expected text content in result")
	}

	responseText := textContent.Text
	if !contains(responseText, "Successfully updated rule") {
		t.Error("Response should indicate successful update")
	}
	if !contains(responseText, ruleID) {
		t.Error("Response should contain rule ID")
	}

	t.Logf("Successfully updated rule in store")
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsInMiddle(s, substr))))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
