# CEL Rule Verification System

## Overview

The CEL Rule Verification system allows you to test CEL rules against predefined test data before deployment. This ensures that rules behave as expected across different scenarios.

## Key Features

1. **Proto-based Structure**: Uses the `CELRule` protobuf definition with embedded test cases
2. **Multiple Input Support**: Handles rules with multiple Kubernetes resources as inputs
3. **Local File Fetching**: Uses temporary files to simulate Kubernetes API responses
4. **Automatic Empty Defaults**: Missing test data defaults to empty resource lists

## Architecture

### Data Flow

```
CELRule (Proto) → CELRuleVerifier → Temporary Test Data Files → CEL Scanner → Verification Results
```

### Key Components

1. **CELRule Proto**: Contains the rule definition and test cases
2. **RuleTestCase**: Individual test scenario with test data map
3. **CELRuleVerifier**: Orchestrates the verification process
4. **Local File Fetcher**: Reads test data from temporary files

## Usage

### 1. Single Input Rule

For rules that query a single Kubernetes resource:

```go
rule := &pb.CELRule{
    Id:          "pod-security",
    Name:        "Pod Security Check",
    Expression:  "pods.items.all(pod, has(pod.spec.securityContext))",
    Inputs: []*pb.RuleInput{
        {
            Name: "pods",
            InputType: &pb.RuleInput_Kubernetes{
                Kubernetes: &pb.KubernetesInput{
                    Group:    "",
                    Version:  "v1",
                    Resource: "pods",
                },
            },
        },
    },
    TestCases: []*pb.RuleTestCase{
        {
            Id:   "tc1",
            Name: "Secure pods",
            TestData: map[string]string{
                "pods": `{"apiVersion": "v1", "kind": "List", "items": [...]}`,
            },
            ExpectedResult: true,
        },
    },
}
```

### 2. Multiple Input Rule

For rules that correlate multiple resources:

```go
rule := &pb.CELRule{
    Id:         "namespace-compliance",
    Expression: `namespaces.items.all(ns, 
        networkpolicies.items.exists(np, 
            np.metadata.namespace == ns.metadata.name
        )
    )`,
    Inputs: []*pb.RuleInput{
        {
            Name: "namespaces",
            // ... Kubernetes input spec
        },
        {
            Name: "networkpolicies", 
            // ... Kubernetes input spec
        },
    },
    TestCases: []*pb.RuleTestCase{
        {
            Id: "tc1",
            TestData: map[string]string{
                "namespaces":      `{...}`, // Namespace list JSON
                "networkpolicies": `{...}`, // NetworkPolicy list JSON
            },
            ExpectedResult: true,
        },
    },
}
```

### 3. Test Data Format

Test data must be valid Kubernetes List objects:

```json
{
    "apiVersion": "v1",
    "kind": "List",
    "items": [
        {
            "apiVersion": "v1",
            "kind": "Pod",
            "metadata": {"name": "test-pod"},
            "spec": {
                "containers": [{"name": "app", "image": "nginx"}]
            }
        }
    ]
}
```

### 4. Missing Test Data

If a test case doesn't provide data for an input, an empty list is used:

```go
TestCases: []*pb.RuleTestCase{
    {
        Id: "tc1",
        TestData: map[string]string{
            "pods": `{...}`,
            // "services" input not provided - defaults to empty list
        },
        ExpectedResult: false,
    },
}
```

## Test Case Design Guidelines

### 1. Comprehensive Coverage

Include test cases for:
- **Happy path**: Resources that should pass the rule
- **Failure cases**: Resources that should fail the rule
- **Edge cases**: Empty lists, missing fields, etc.
- **Complex scenarios**: Multiple resources with relationships

### 2. Realistic Test Data

Use realistic Kubernetes resource structures:
- Include required fields (apiVersion, kind, metadata)
- Use proper resource specifications
- Maintain referential integrity between resources

### 3. Clear Naming

Use descriptive test case names:
- ✅ "All pods have security context"
- ✅ "Namespace missing network policy"
- ❌ "Test 1"
- ❌ "Failure case"

## Verification Results

The verifier returns structured results:

```go
type VerificationResult struct {
    RuleID         string            // Rule identifier
    RuleName       string            // Human-readable name
    TestCases      []*TestCaseResult // Individual test results
    OverallPassed  bool              // All tests passed?
    PassedCount    int               // Number of passed tests
    FailedCount    int               // Number of failed tests
    TotalCount     int               // Total test count
}

type TestCaseResult struct {
    TestCaseID      string // Test identifier
    TestCaseName    string // Test name
    Passed          bool   // Did test pass?
    ExpectedResult  bool   // What was expected
    ActualResult    string // What actually happened
    Error           string // Error details if failed
}
```

## Integration Example

```go
// Create verifier
verifier := verification.NewCELRuleVerifier()

// Verify rule
result, err := verifier.VerifyCELRule(context.Background(), rule)
if err != nil {
    log.Fatalf("Verification failed: %v", err)
}

// Check results
if result.OverallPassed {
    fmt.Printf("✅ All %d test cases passed!\n", result.TotalCount)
} else {
    fmt.Printf("❌ %d/%d test cases failed\n", 
        result.FailedCount, result.TotalCount)
    
    for _, tc := range result.TestCases {
        if !tc.Passed {
            fmt.Printf("  - %s: %s\n", tc.TestCaseName, tc.Error)
        }
    }
}
```

## Best Practices

1. **Test Before Deploy**: Always verify rules before using in production
2. **Version Test Data**: Keep test data in sync with rule changes
3. **Document Expectations**: Use test descriptions to explain why a test should pass/fail
4. **Test Incrementally**: Start with simple test cases, add complexity gradually
5. **Reuse Test Data**: Create test data libraries for common resource types

## Troubleshooting

### Common Issues

1. **JSON Parse Errors**: Ensure test data is valid JSON
2. **Missing Required Fields**: Include apiVersion, kind, and items for Lists
3. **Type Mismatches**: Ensure field types match Kubernetes API specs
4. **Empty Results**: Check if test data includes the "items" array

### Debug Tips

1. Enable debug logging in scan config
2. Inspect temporary test files during debugging
3. Validate test data against Kubernetes schemas
4. Use smaller test cases to isolate issues 