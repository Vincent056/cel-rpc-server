package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"connectrpc.com/connect"
	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
	"github.com/Vincent056/cel-rpc-server/gen/cel/v1/celv1connect"
)

func main() {
	// Create a Connect client
	client := celv1connect.NewCELValidationServiceClient(
		http.DefaultClient,
		"http://localhost:8349",
	)

	// Create a context with longer timeout for large models
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	// Create the request
	req := connect.NewRequest(&celv1.ChatAssistRequest{
		Message: "a rule to check all pods have resource limits",
	})

	fmt.Println("Sending ChatAssist request: 'a rule to check all pods have resource limits'")
	fmt.Println("Using server: http://localhost:8349")
	fmt.Println("---")

	// Make the streaming call
	stream, err := client.ChatAssist(ctx, req)
	if err != nil {
		log.Fatalf("ChatAssist failed: %v", err)
	}
	defer stream.Close()

	// Read responses from the stream
	responseCount := 0
	for stream.Receive() {
		resp := stream.Msg()
		responseCount++

		fmt.Printf("Response #%d:\n", responseCount)
		fmt.Printf("  Timestamp: %d\n", resp.Timestamp)

		// Check which type of content this is
		switch content := resp.Content.(type) {
		case *celv1.ChatAssistResponse_Thinking:
			fmt.Printf("  Type: Thinking\n")
			fmt.Printf("  Content: %s\n", content.Thinking)
		case *celv1.ChatAssistResponse_Rule:
			fmt.Printf("  Type: Rule\n")
			fmt.Printf("  Rule Name: %s\n", content.Rule.Name)
			fmt.Printf("  Expression: %s\n", content.Rule.Expression)
			fmt.Printf("  Explanation: %s\n", content.Rule.Explanation)
			if len(content.Rule.Variables) > 0 {
				fmt.Printf("  Variables: %v\n", content.Rule.Variables)
			}
		case *celv1.ChatAssistResponse_Validation:
			fmt.Printf("  Type: Validation\n")
			fmt.Printf("  Success: %v\n", content.Validation.Success)
			fmt.Printf("  Passed: %d, Failed: %d\n", content.Validation.PassedCount, content.Validation.FailedCount)
			if len(content.Validation.Details) > 0 {
				fmt.Printf("  Details: %v\n", content.Validation.Details)
			}
		case *celv1.ChatAssistResponse_Text:
			fmt.Printf("  Type: Text\n")
			fmt.Printf("  Content: %s\n", content.Text)
		case *celv1.ChatAssistResponse_Error:
			fmt.Printf("  Type: Error\n")
			fmt.Printf("  Error: %s\n", content.Error)
		case *celv1.ChatAssistResponse_Resources:
			fmt.Printf("  Type: Resources\n")
			fmt.Printf("  Resource Type: %s\n", content.Resources.ResourceType)
			fmt.Printf("  Count: %d\n", content.Resources.Count)
			if len(content.Resources.Namespaces) > 0 {
				fmt.Printf("  Namespaces: %v\n", content.Resources.Namespaces)
			}
		default:
			fmt.Printf("  Type: Unknown\n")
		}
		fmt.Println()
	}

	if err := stream.Err(); err != nil && err != io.EOF {
		log.Fatalf("Stream error: %v", err)
	}

	fmt.Printf("Total responses received: %d\n", responseCount)
}
