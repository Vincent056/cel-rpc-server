package verification

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	pb "github.com/Vincent056/cel-rpc-server/gen/cel/v1"

	"github.com/ComplianceAsCode/compliance-sdk/pkg/scanner"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CELRuleVerifier verifies CEL rules against their test cases
type CELRuleVerifier struct {
	// No longer need tempDir since we're using MockFetcher
}

// NewCELRuleVerifier creates a new rule verifier
func NewCELRuleVerifier() *CELRuleVerifier {
	return &CELRuleVerifier{}
}

// MockFetcher implements ResourceFetcher for testing with predefined data
type MockFetcher struct {
	data map[string]interface{}
}

// NewMockFetcher creates a new mock fetcher with predefined data
func NewMockFetcher(data map[string]interface{}) *MockFetcher {
	return &MockFetcher{data: data}
}

// FetchResources implements ResourceFetcher interface for testing
func (m *MockFetcher) FetchResources(ctx context.Context, rule scanner.Rule, variables []scanner.CelVariable) (map[string]interface{}, []string, error) {
	return m.data, nil, nil
}

// FetchInputs returns the predefined mock data
func (m *MockFetcher) FetchInputs(inputs []scanner.Input, variables []scanner.CelVariable) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, input := range inputs {
		if data, exists := m.data[input.Name()]; exists {
			result[input.Name()] = data
		} else {
			return nil, fmt.Errorf("mock data not found for input: %s", input.Name())
		}
	}
	return result, nil
}

// SupportsInputType returns true for all input types in mock mode
func (m *MockFetcher) SupportsInputType(inputType scanner.InputType) bool {
	return true
}

// VerifyCELRule verifies a CEL rule against all its test cases
func (v *CELRuleVerifier) VerifyCELRule(ctx context.Context, rule *pb.CELRule) (*VerificationResult, error) {
	startTime := time.Now()

	result := &VerificationResult{
		RuleID:    rule.Id,
		RuleName:  rule.Name,
		TestCases: make([]*TestCaseResult, 0, len(rule.TestCases)),
	}

	// Convert proto rule to celscanner rule
	celRule, err := v.convertToCELScannerRule(rule)
	if err != nil {
		return nil, fmt.Errorf("failed to convert rule: %w", err)
	}

	// Run each test case
	for _, testCase := range rule.TestCases {
		testResult := v.runTestCase(ctx, celRule, testCase)
		result.TestCases = append(result.TestCases, testResult)
	}

	// Calculate overall result
	result.calculateOverallResult()

	// Set total duration
	result.Duration = time.Since(startTime)

	return result, nil
}

// runTestCase executes a single test case using MockFetcher
func (v *CELRuleVerifier) runTestCase(ctx context.Context, rule scanner.CelRule, testCase *pb.RuleTestCase) *TestCaseResult {
	startTime := time.Now()

	result := &TestCaseResult{
		TestCaseID:      testCase.Id,
		TestCaseName:    testCase.Name,
		Description:     testCase.Description,
		ExpectedResult:  testCase.ExpectedResult,
		ExpectedMessage: testCase.ExpectedMessage,
	}

	// Prepare mock data from test case
	mockData := make(map[string]interface{})

	// Process each input in the rule
	for _, input := range rule.Inputs() {
		inputName := input.Name()

		// Check if test data exists for this input
		testDataJSON, exists := testCase.TestData[inputName]
		if !exists {
			// No data provided - create empty list for Kubernetes inputs
			if input.Type() == scanner.InputTypeKubernetes {
				testDataJSON = `{"apiVersion": "v1", "kind": "List", "items": []}`
			} else {
				// For non-Kubernetes inputs, use empty object
				testDataJSON = `{}`
			}
		}

		// Parse the JSON data
		var data interface{}
		if err := json.Unmarshal([]byte(testDataJSON), &data); err != nil {
			result.Error = fmt.Sprintf("Failed to parse test data for input %s: %v", inputName, err)
			result.Passed = false
			result.Duration = time.Since(startTime)
			return result
		}

		// Convert to proper Kubernetes unstructured objects for Kubernetes inputs
		if input.Type() == scanner.InputTypeKubernetes {
			// Check if data has "items" field to determine if it's a list
			if dataMap, ok := data.(map[string]interface{}); ok {
				if items, hasItems := dataMap["items"]; hasItems {
					// It's a list - create a proper UnstructuredList with required fields
					list := &unstructured.UnstructuredList{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "List",
							"metadata":   map[string]interface{}{},
						},
					}

					// Add the items
					if itemsList, ok := items.([]interface{}); ok {
						for _, item := range itemsList {
							if itemMap, ok := item.(map[string]interface{}); ok {
								list.Items = append(list.Items, unstructured.Unstructured{
									Object: itemMap,
								})
							}
						}
					}

					mockData[inputName] = list
				} else {
					// It's a single object - create an Unstructured with the data
					obj := &unstructured.Unstructured{
						Object: dataMap,
					}
					mockData[inputName] = obj
				}
			} else {
				// Not a map, use as-is
				mockData[inputName] = data
			}
		} else {
			// Non-Kubernetes input, use parsed data as-is
			mockData[inputName] = data
		}
	}

	// Create mock fetcher with test data
	mockFetcher := NewMockFetcher(mockData)

	// Create scanner with mock fetcher
	scannerInstance := scanner.NewScanner(mockFetcher, nil)

	// Create scan configuration
	config := scanner.ScanConfig{
		Rules:              []scanner.Rule{rule},
		Variables:          []scanner.CelVariable{},
		EnableDebugLogging: false,
	}

	// Run the scan
	results, err := scannerInstance.Scan(ctx, config)
	if err != nil {
		result.Error = fmt.Sprintf("Scan failed: %v", err)
		result.Passed = false
		result.Duration = time.Since(startTime)
		return result
	}

	if len(results) != 1 {
		result.Error = fmt.Sprintf("Expected 1 result, got %d", len(results))
		result.Passed = false
		result.Duration = time.Since(startTime)
		return result
	}

	// Analyze result
	scanResult := results[0]
	result.ActualResult = string(scanResult.Status)

	// Check if result matches expectation
	resultPassed := false
	switch scanResult.Status {
	case scanner.CheckResultPass:
		resultPassed = true
	case scanner.CheckResultFail:
		resultPassed = false
	case scanner.CheckResultError:
		result.Error = scanResult.ErrorMessage
		result.Passed = false
		result.Duration = time.Since(startTime)
		return result
	}

	result.Passed = resultPassed == testCase.ExpectedResult
	if !result.Passed {
		result.Error = fmt.Sprintf("Expected %v but got %v", testCase.ExpectedResult, resultPassed)
	}

	// Check expected message if provided
	if testCase.ExpectedMessage != "" && scanResult.ErrorMessage != testCase.ExpectedMessage {
		result.Error = fmt.Sprintf("Expected message '%s' but got '%s'", testCase.ExpectedMessage, scanResult.ErrorMessage)
		result.Passed = false
	}

	result.Duration = time.Since(startTime)
	return result
}

// convertToCELScannerRule converts proto rule to celscanner rule
func (v *CELRuleVerifier) convertToCELScannerRule(rule *pb.CELRule) (scanner.CelRule, error) {
	// Create rule builder
	builder := scanner.NewRuleBuilder(rule.Id, scanner.RuleTypeCEL).
		WithName(rule.Name).
		WithDescription(rule.Description).
		SetCelExpression(rule.Expression)

	// Add inputs
	for _, input := range rule.Inputs {
		log.Printf("Processing input %s with type: %T", input.Name, input.InputType)
		if input.InputType == nil {
			log.Printf("Warning: input %s has nil InputType", input.Name)
		}

		switch inputType := input.InputType.(type) {
		case *pb.RuleInput_Kubernetes:
			log.Printf("Kubernetes input detected for %s", input.Name)
			k8s := input.GetKubernetes()
			builder.WithKubernetesInput(
				input.Name,
				k8s.Group,
				k8s.Version,
				k8s.Resource,
				k8s.Namespace,
				k8s.ResourceName,
			)
		case *pb.RuleInput_File:
			log.Printf("File input detected for %s", input.Name)
			file := input.GetFile()
			builder.WithFileInput(
				input.Name,
				file.Path,
				file.Format,
				file.Recursive,
				file.CheckPermissions,
			)
			// Add file input support if needed
		case *pb.RuleInput_Http:
			log.Printf("HTTP input detected for %s", input.Name)
			http := input.GetHttp()
			builder.WithHTTPInput(
				input.Name,
				http.Url,
				http.Method,
				http.Headers,
				[]byte(http.Body),
			)
			// Add HTTP input support if needed
		default:
			log.Printf("Unsupported input type %T for %s", inputType, input.Name)
			return nil, fmt.Errorf("unsupported input type for %s", input.Name)
		}
	}
	// try catch builder.Build()
	builderRule, err := builder.BuildCelRule()
	if err != nil {
		return nil, fmt.Errorf("failed to build rule: %w", err)
	}

	return builderRule, nil
}

// VerificationResult represents the overall verification result
type VerificationResult struct {
	RuleID        string            `json:"rule_id"`
	RuleName      string            `json:"rule_name"`
	TestCases     []*TestCaseResult `json:"test_cases"`
	OverallPassed bool              `json:"overall_passed"`
	PassedCount   int               `json:"passed_count"`
	FailedCount   int               `json:"failed_count"`
	TotalCount    int               `json:"total_count"`
	Duration      time.Duration     `json:"duration"`
}

// TestCaseResult represents a single test case result
type TestCaseResult struct {
	TestCaseID      string        `json:"test_case_id"`
	TestCaseName    string        `json:"test_case_name"`
	Description     string        `json:"description"`
	Passed          bool          `json:"passed"`
	ExpectedResult  bool          `json:"expected_result"`
	ActualResult    string        `json:"actual_result"`
	ExpectedMessage string        `json:"expected_message,omitempty"`
	Error           string        `json:"error,omitempty"`
	Duration        time.Duration `json:"duration"`
}

// calculateOverallResult calculates the overall verification result
func (r *VerificationResult) calculateOverallResult() {
	r.TotalCount = len(r.TestCases)
	r.PassedCount = 0
	r.FailedCount = 0

	for _, tc := range r.TestCases {
		if tc.Passed {
			r.PassedCount++
		} else {
			r.FailedCount++
		}
	}

	r.OverallPassed = r.FailedCount == 0
}

// VerifyMultipleRules verifies multiple CEL rules
func (v *CELRuleVerifier) VerifyMultipleRules(ctx context.Context, rules []*pb.CELRule) ([]*VerificationResult, error) {
	results := make([]*VerificationResult, 0, len(rules))

	for _, rule := range rules {
		result, err := v.VerifyCELRule(ctx, rule)
		if err != nil {
			// Create error result
			result = &VerificationResult{
				RuleID:        rule.Id,
				RuleName:      rule.Name,
				OverallPassed: false,
				TestCases: []*TestCaseResult{{
					TestCaseID:   "error",
					TestCaseName: "Verification Error",
					Passed:       false,
					Error:        err.Error(),
				}},
			}
		}
		results = append(results, result)
	}

	return results, nil
}
