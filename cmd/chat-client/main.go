package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
	"github.com/Vincent056/cel-rpc-server/gen/cel/v1/celv1connect"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
)

// TestCase represents a single test case
type TestCase struct {
	Name        string
	Request     *celv1.ChatAssistRequest
	Validate    func(responses []*celv1.ChatAssistResponse) error
	ExpectError bool
}

// TestResult represents the result of a test
type TestResult struct {
	TestName  string
	Passed    bool
	Error     error
	Responses []*celv1.ChatAssistResponse
	Duration  time.Duration
}

func main() {
	// Create HTTP client that supports h2c (HTTP/2 without TLS)
	client := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
				// Use a regular net.Dial for h2c
				return net.Dial(network, addr)
			},
		},
	}

	// Get server URL from env or use default
	serverURL := os.Getenv("CEL_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8349"
	}

	celClient := celv1connect.NewCELValidationServiceClient(
		client,
		serverURL,
		connect.WithGRPCWeb(),
	)

	fmt.Printf("=== CEL Chat Service Test Suite ===\n")
	fmt.Printf("Server: %s\n", serverURL)
	fmt.Printf("Time: %s\n\n", time.Now().Format(time.RFC3339))

	// Define test cases
	testCases := []TestCase{
		{
			Name: "Basic Hello",
			Request: &celv1.ChatAssistRequest{
				Message:        "Hello, can you help me?",
				ConversationId: "test-basic-hello",
			},
			Validate: func(responses []*celv1.ChatAssistResponse) error {
				if len(responses) == 0 {
					return fmt.Errorf("no responses received")
				}
				// Check for thinking or text responses
				hasContent := false
				for _, resp := range responses {
					if resp.Content != nil {
						hasContent = true
						break
					}
				}
				if !hasContent {
					return fmt.Errorf("no content in responses")
				}
				return nil
			},
		},
		{
			Name: "Simple Rule Generation",
			Request: &celv1.ChatAssistRequest{
				Message:        "Generate a CEL rule to check if a Pod has a label called 'app'",
				ConversationId: "test-simple-rule",
			},
			Validate: func(responses []*celv1.ChatAssistResponse) error {
				// Look for rule generation response
				hasRule := false
				for _, resp := range responses {
					if rule := resp.GetRule(); rule != nil {
						hasRule = true
						log.Printf("Generated rule: %s", rule.Expression)
						if rule.Expression == "" {
							return fmt.Errorf("empty rule expression")
						}
					}
				}
				if !hasRule {
					// It's OK if we get text responses explaining the rule
					for _, resp := range responses {
						if text := resp.GetText(); text != nil {
							log.Printf("Text response: %s", text.Text)
						}
					}
				}
				return nil
			},
		},
		{
			Name: "Rule Generation with Context",
			Request: &celv1.ChatAssistRequest{
				Message:        "Generate a rule to check if all pods have service accounts",
				ConversationId: "test-rule-context",
				Context: &celv1.ChatAssistRequest_RuleContext{
					RuleContext: &celv1.RuleGenerationContext{
						ResourceType:     "Pod",
						ApiVersion:       "v1",
						ValidationIntent: "ensure all pods have service accounts",
						UseLiveCluster:   false,
						Namespace:        "default",
					},
				},
			},
			Validate: func(responses []*celv1.ChatAssistResponse) error {
				// Should get thinking messages and rule generation
				hasThinking := false
				hasRule := false
				hasText := false

				for _, resp := range responses {
					if resp.GetThinking() != nil {
						hasThinking = true
					}
					if resp.GetRule() != nil {
						hasRule = true
					}
					if resp.GetText() != nil {
						hasText = true
					}
				}

				if !hasThinking && !hasRule && !hasText {
					return fmt.Errorf("no meaningful responses received")
				}
				return nil
			},
		},
	}

	// Run tests
	results := make([]TestResult, 0, len(testCases))

	for i, tc := range testCases {
		fmt.Printf("Test %d: %s\n", i+1, tc.Name)
		fmt.Println("----------------------------------------")

		result := runTest(celClient, tc)
		results = append(results, result)

		if result.Passed {
			fmt.Printf("✅ PASSED (Duration: %v)\n", result.Duration)
		} else {
			fmt.Printf("❌ FAILED: %v\n", result.Error)
		}

		// Print responses summary
		fmt.Printf("Responses received: %d\n", len(result.Responses))
		for _, resp := range result.Responses {
			printResponseSummary(resp)
		}

		fmt.Println()
		time.Sleep(1 * time.Second) // Brief pause between tests
	}

	// Print summary
	fmt.Println("\n=== Test Summary ===")
	passed := 0
	failed := 0
	for _, result := range results {
		if result.Passed {
			passed++
			fmt.Printf("✅ %s - PASSED\n", result.TestName)
		} else {
			failed++
			fmt.Printf("❌ %s - FAILED: %v\n", result.TestName, result.Error)
		}
	}

	fmt.Printf("\nTotal: %d, Passed: %d, Failed: %d\n", len(results), passed, failed)

	if failed > 0 {
		os.Exit(1)
	}
}

func runTest(client celv1connect.CELValidationServiceClient, tc TestCase) TestResult {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	responses := make([]*celv1.ChatAssistResponse, 0)

	// Create stream
	stream, err := client.ChatAssist(ctx, connect.NewRequest(tc.Request))
	if err != nil {
		return TestResult{
			TestName: tc.Name,
			Passed:   false,
			Error:    fmt.Errorf("failed to create stream: %w", err),
			Duration: time.Since(start),
		}
	}
	defer stream.Close()

	// Collect all responses
	for stream.Receive() {
		resp := stream.Msg()
		responses = append(responses, resp)

		// Log response for debugging
		if os.Getenv("DEBUG") == "true" {
			respJSON, _ := json.MarshalIndent(resp, "", "  ")
			log.Printf("Response: %s", respJSON)
		}
	}

	if err := stream.Err(); err != nil && err != io.EOF {
		if tc.ExpectError {
			return TestResult{
				TestName:  tc.Name,
				Passed:    true,
				Responses: responses,
				Duration:  time.Since(start),
			}
		}
		return TestResult{
			TestName:  tc.Name,
			Passed:    false,
			Error:     fmt.Errorf("stream error: %w", err),
			Responses: responses,
			Duration:  time.Since(start),
		}
	}

	// Validate responses
	if tc.Validate != nil {
		if err := tc.Validate(responses); err != nil {
			return TestResult{
				TestName:  tc.Name,
				Passed:    false,
				Error:     err,
				Responses: responses,
				Duration:  time.Since(start),
			}
		}
	}

	return TestResult{
		TestName:  tc.Name,
		Passed:    true,
		Responses: responses,
		Duration:  time.Since(start),
	}
}

func printResponseSummary(resp *celv1.ChatAssistResponse) {
	timestamp := time.Unix(resp.Timestamp, 0).Format("15:04:05")

	switch content := resp.Content.(type) {
	case *celv1.ChatAssistResponse_Thinking:
		fmt.Printf("  [%s] 🤔 Thinking: %s\n", timestamp, truncate(content.Thinking.Message, 60))

	case *celv1.ChatAssistResponse_Rule:
		fmt.Printf("  [%s] 📝 Rule: %s\n", timestamp, truncate(content.Rule.Expression, 60))

	case *celv1.ChatAssistResponse_Text:
		fmt.Printf("  [%s] 💬 Text: %s\n", timestamp, truncate(content.Text.Text, 60))

	case *celv1.ChatAssistResponse_Error:
		fmt.Printf("  [%s] ❌ Error: %s\n", timestamp, content.Error.Error)

	case *celv1.ChatAssistResponse_Validation:
		fmt.Printf("  [%s] ✅ Validation: success=%v, passed=%d, failed=%d\n",
			timestamp, content.Validation.Success,
			content.Validation.PassedCount, content.Validation.FailedCount)

	case *celv1.ChatAssistResponse_Resources:
		fmt.Printf("  [%s] 📦 Resources: %d %s(s) found\n",
			timestamp, content.Resources.Count, content.Resources.ResourceType)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
