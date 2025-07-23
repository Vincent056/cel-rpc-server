package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Vincent056/celscanner"
)

// SimpleLogger implements the celscanner.Logger interface
type SimpleLogger struct{}

func (l *SimpleLogger) Debug(msg string, args ...interface{}) {
	log.Printf("[DEBUG] "+msg, args...)
}

func (l *SimpleLogger) Info(msg string, args ...interface{}) {
	log.Printf("[INFO] "+msg, args...)
}

func (l *SimpleLogger) Warn(msg string, args ...interface{}) {
	log.Printf("[WARN] "+msg, args...)
}

func (l *SimpleLogger) Error(msg string, args ...interface{}) {
	log.Printf("[ERROR] "+msg, args...)
}

func main() {
	var (
		expression   = flag.String("expression", "", "CEL expression to evaluate")
		inputName    = flag.String("input-name", "file", "Name to use for the input in the expression")
		filePath     = flag.String("file-path", "", "Path to the file to check")
		fileFormat   = flag.String("file-format", "yaml", "Format of the file (yaml, json, text)")
		outputFormat = flag.String("output-format", "json", "Output format (json, text)")
		nodeName     = os.Getenv("NODE_NAME")
	)

	flag.Parse()

	if *expression == "" || *filePath == "" {
		log.Fatal("expression and file-path are required")
	}

	log.Printf("[Scanner] Starting CEL scan on node %s", nodeName)
	log.Printf("[Scanner] Expression: %s", *expression)
	log.Printf("[Scanner] File: %s (format: %s)", *filePath, *fileFormat)
	log.Printf("[Scanner] Input name: %s", *inputName)

	// Create a CEL rule
	rule, err := celscanner.NewRuleBuilder("node-file-check").
		SetExpression(*expression).
		WithName(fmt.Sprintf("File check on %s", nodeName)).
		WithFileInput(*inputName, *filePath, *fileFormat, false, true).
		Build()
	if err != nil {
		log.Fatalf("Failed to create rule: %v", err)
	}

	// Create scan config
	config := celscanner.ScanConfig{
		Rules: []celscanner.CelRule{rule},
	}

	// Create scanner (no k8s client needed for file checks)
	scanner := celscanner.NewScanner(nil, &SimpleLogger{})

	// Run scan
	ctx := context.Background()
	results, err := scanner.Scan(ctx, config)
	if err != nil {
		log.Fatalf("Scan failed: %v", err)
	}

	// Output results
	if *outputFormat == "json" {
		for _, result := range results {
			// Add node information
			output := map[string]interface{}{
				"node":         nodeName,
				"file":         *filePath,
				"status":       string(result.Status),
				"errorMessage": result.ErrorMessage,
				"expression":   *expression,
			}

			jsonBytes, err := json.Marshal(output)
			if err != nil {
				log.Printf("Error marshaling result: %v", err)
				continue
			}

			fmt.Println(string(jsonBytes))
		}
	} else {
		// Text output
		for _, result := range results {
			fmt.Printf("Node: %s\n", nodeName)
			fmt.Printf("File: %s\n", *filePath)
			fmt.Printf("Status: %s\n", result.Status)
			if result.ErrorMessage != "" {
				fmt.Printf("Error: %s\n", result.ErrorMessage)
			}
			fmt.Println("---")
		}
	}

	log.Printf("[Scanner] Scan completed on node %s", nodeName)
}
