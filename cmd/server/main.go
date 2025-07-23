package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/Vincent056/cel-rpc-server/cmd/server/verification"
	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
	"github.com/Vincent056/cel-rpc-server/gen/cel/v1/celv1connect"

	"github.com/Vincent056/cel-rpc-server/pkg/mcp"
	"github.com/Vincent056/celscanner"
	"github.com/Vincent056/celscanner/fetchers"
	serverv1 "github.com/mark3labs/mcp-go/server"
	"google.golang.org/protobuf/types/known/timestamppb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type CELValidationServer struct {
	scanner       *celscanner.Scanner
	kubeClient    kubernetes.Interface
	runtimeClient runtimeclient.Client
	dynamicClient dynamic.Interface

	// For tracking node scanner jobs
	jobTrackerMu sync.RWMutex
	jobTrackers  map[string]*NodeJobTracker

	// Rule library store
	ruleStore RuleStore

	// Chat handler for AI-powered assistance
	chatHandler *ChatHandler
}

// NodeJobTracker tracks a job running on a node
type NodeJobTracker struct {
	JobID      string
	NodeName   string
	Status     celv1.NodeScannerStatus_Status
	Message    string
	StartTime  time.Time
	ResultChan chan *celv1.NodeValidationResult
}

// ValidateCEL implements the CEL validation RPC method
func (s *CELValidationServer) ValidateCEL(
	ctx context.Context,
	req *connect.Request[celv1.ValidateCELRequest],
) (*connect.Response[celv1.ValidationResponse], error) {
	// Log request details for debugging
	log.Printf("[ValidateCEL] Request: expression=%q, inputs=%d, testCases=%d",
		req.Msg.Expression,
		len(req.Msg.Inputs),
		len(req.Msg.TestCases))

	// Validate expression is not empty
	if req.Msg.Expression == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("expression cannot be empty"))
	}

	// Use context timeout if not set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	// Use test cases if provided (offline validation)
	if len(req.Msg.TestCases) > 0 {
		return s.validateWithTestCases(ctx, req)
	}

	// Otherwise use inputs for live validation
	if len(req.Msg.Inputs) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("either test_cases or inputs must be provided"))
	}

	return s.validateWithMultipleInputs(ctx, req)
}

// validateWithMultipleInputs validates using the new multiple inputs API
func (s *CELValidationServer) validateWithMultipleInputs(
	ctx context.Context,
	req *connect.Request[celv1.ValidateCELRequest],
) (*connect.Response[celv1.ValidationResponse], error) {
	if len(req.Msg.Inputs) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("at least one input is required"))
	}

	log.Printf("[ValidateCEL] Multiple inputs validation: %d inputs", len(req.Msg.Inputs))

	// Check if any file inputs require node execution
	var hasNodeFileInputs bool
	for _, input := range req.Msg.Inputs {
		if fileInput, ok := input.InputType.(*celv1.RuleInput_File); ok {
			f := fileInput.File
			// Check if this file input targets nodes
			if f.GetNodeSelector() != nil || f.GetAllNodes() {
				hasNodeFileInputs = true
				break
			}
		}
	}

	// If we have node-targeted file inputs, use special handling
	if hasNodeFileInputs {
		return s.validateWithNodeExecution(ctx, req)
	}

	// Otherwise, use standard validation
	// Create a rule builder
	ruleBuilder := celscanner.NewRuleBuilder("multi-input-validation").
		SetExpression(req.Msg.Expression).
		WithName("Multi-Input Validation")

	// Process each input
	for _, input := range req.Msg.Inputs {
		switch inputType := input.InputType.(type) {
		case *celv1.RuleInput_Kubernetes:
			k8s := inputType.Kubernetes
			log.Printf("[ValidateCEL] Adding Kubernetes input '%s': %s/%s/%s",
				input.Name, k8s.Group, k8s.Version, k8s.Resource)

			if k8s.ResourceName != "" {
				// Single resource
				ruleBuilder.WithKubernetesInput(
					input.Name,
					k8s.Group,
					k8s.Version,
					k8s.Resource,
					k8s.Namespace,
					k8s.ResourceName,
				)
			} else {
				// All resources
				ruleBuilder.WithKubernetesInput(
					input.Name,
					k8s.Group,
					k8s.Version,
					k8s.Resource,
					k8s.Namespace,
					"",
				)
			}

		case *celv1.RuleInput_File:
			f := inputType.File
			log.Printf("[ValidateCEL] Adding File input '%s': %s", input.Name, f.Path)

			ruleBuilder.WithFileInput(
				input.Name,
				f.Path,
				f.Format,
				f.Recursive,
				f.CheckPermissions,
			)

		case *celv1.RuleInput_Http:
			// TODO: Implement HTTP input when celscanner supports it
			return nil, connect.NewError(connect.CodeUnimplemented,
				fmt.Errorf("HTTP input type not yet implemented"))

		default:
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("unknown input type for input '%s'", input.Name))
		}
	}

	rule, err := ruleBuilder.Build()
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("failed to build rule: %w", err))
	}

	// Create scan config
	config := celscanner.ScanConfig{
		Rules: []celscanner.CelRule{rule},
	}

	// Run validation
	startTime := time.Now()
	results, err := s.scanner.Scan(ctx, config)
	endTime := time.Now()

	resp := &celv1.ValidationResponse{
		Success: err == nil,
	}

	if err != nil {
		resp.Error = fmt.Sprintf("scan failed: %v", err)
		return connect.NewResponse(resp), nil
	}

	// Check if we have any results with errors that indicate cluster access issues
	for _, result := range results {
		if result.Status == celscanner.CheckResultError && result.ErrorMessage != "" {
			// Check if this is a CEL compilation error due to missing variables from cluster access failure
			if strings.Contains(result.ErrorMessage, "undeclared reference") {
				// Look for cluster access related errors in the error message or logs
				// This indicates the root cause is likely cluster connectivity, not CEL syntax
				for _, input := range req.Msg.Inputs {
					if k8sInput := input.GetKubernetes(); k8sInput != nil {
						// Check if the undeclared reference matches an input name
						if strings.Contains(result.ErrorMessage, fmt.Sprintf("undeclared reference to '%s'", input.Name)) {
							resp.Error = fmt.Sprintf("Failed to access Kubernetes cluster to fetch resources for input '%s' (%s/%s). This may be due to cluster connectivity issues, missing credentials, or invalid cluster configuration. Original CEL error: %s", 
								input.Name, k8sInput.Version, k8sInput.Resource, result.ErrorMessage)
							resp.Success = false
							return connect.NewResponse(resp), nil
						}
					}
				}
			}
		}
	}

	// Convert results
	for i, result := range results {
		evaluationContext := make(map[string]string)
		for _, inputData := range req.Msg.Inputs {
			if k8sInput := inputData.GetKubernetes(); k8sInput != nil {
				evaluationContext[inputData.Name] = fmt.Sprintf("%s/%s", k8sInput.Version, k8sInput.Resource)
			} else if fileInput := inputData.GetFile(); fileInput != nil {
				evaluationContext[inputData.Name] = fmt.Sprintf("file: %s", fileInput.Path)
			}
		}
		validationResult := &celv1.ValidationResult{
			EvaluationContext: evaluationContext,
			TestCase:          fmt.Sprintf("Resource %d validation", i+1),
			Passed:            result.Status == celscanner.CheckResultPass,
			Error:             result.ErrorMessage,
			Details:           fmt.Sprintf("Status: %s", result.Status),
		}

		// Add resource information from inputs
		if i < len(req.Msg.Inputs) {
			input := req.Msg.Inputs[i]
			if k8sInput := input.GetKubernetes(); k8sInput != nil {
				apiVersion := k8sInput.Version
				if k8sInput.Group != "" {
					apiVersion = k8sInput.Group + "/" + k8sInput.Version
				}
				validationResult.ResourceInfo = &celv1.ResourceInfo{
					ApiVersion: apiVersion,
					Kind:       k8sInput.Resource,
					Name:       k8sInput.ResourceName,
					Namespace:  k8sInput.Namespace,
				}
			}
		}

		// Add evaluation context
		validationResult.EvaluationContext = make(map[string]string)
		for _, input := range req.Msg.Inputs {
			if k8sInput := input.GetKubernetes(); k8sInput != nil {
				validationResult.EvaluationContext[input.Name] = fmt.Sprintf("%s/%s", k8sInput.Version, k8sInput.Resource)
			} else if fileInput := input.GetFile(); fileInput != nil {
				validationResult.EvaluationContext[input.Name] = fmt.Sprintf("file: %s", fileInput.Path)
			}
		}

		resp.Results = append(resp.Results, validationResult)
	}

	// If the validation failed and we have Kubernetes pod inputs, provide detailed violation info
	if len(results) > 0 && results[0].Status == celscanner.CheckResultFail {
		// Collect all inputs for analysis
		violations := s.analyzeViolations(ctx, req.Msg.Expression, req.Msg.Inputs)
		if violations != "" {
			// Update details with specific resource information
			for i, result := range resp.Results {
				if !result.Passed && result.ResourceInfo != nil {
					result.Details = fmt.Sprintf("Status: %s\n\nResource: %s/%s %s (namespace: %s)\n\n%s",
						results[i].Status,
						result.ResourceInfo.ApiVersion,
						result.ResourceInfo.Kind,
						result.ResourceInfo.Name,
						result.ResourceInfo.Namespace,
						violations)
				}
			}
		}
	}

	// Add performance metrics
	duration := endTime.Sub(startTime)
	resp.Performance = &celv1.PerformanceMetrics{
		TotalTimeMs:    duration.Milliseconds(),
		ResourcesCount: int32(len(results)),
		AverageTimeMs:  duration.Milliseconds() / int64(max(len(results), 1)),
	}

	return connect.NewResponse(resp), nil
}

// validateWithNodeExecution handles validation when file inputs target specific nodes
func (s *CELValidationServer) validateWithNodeExecution(
	ctx context.Context,
	req *connect.Request[celv1.ValidateCELRequest],
) (*connect.Response[celv1.ValidationResponse], error) {
	log.Printf("[ValidateCEL] Node execution validation")

	resp := &celv1.ValidationResponse{
		Success: true,
		Results: make([]*celv1.ValidationResult, 0),
	}

	// For each file input that targets nodes, create jobs
	for _, input := range req.Msg.Inputs {
		if fileInput, ok := input.InputType.(*celv1.RuleInput_File); ok {
			f := fileInput.File

			// Skip if not targeting nodes
			switch f.Target.(type) {
			case *celv1.FileInput_NodeSelector, *celv1.FileInput_AllNodes:
				// This needs node execution
				results, err := s.executeFileCheckOnNodes(ctx, req.Msg.Expression, input.Name, f)
				if err != nil {
					log.Printf("[ValidateCEL] Node execution failed for input %s: %v", input.Name, err)
					resp.Success = false
					resp.Error = fmt.Sprintf("node execution failed: %v", err)
					continue
				}

				// Convert node results to validation results
				for _, nodeResult := range results {
					resp.Results = append(resp.Results, &celv1.ValidationResult{
						TestCase: fmt.Sprintf("Node %s - %s", nodeResult.NodeName, f.Path),
						Passed:   nodeResult.Success,
						Error:    nodeResult.Error,
						Details:  fmt.Sprintf("Node: %s, Duration: %v", nodeResult.NodeName, nodeResult.Duration),
					})
				}
			default:
				// Regular file input, handle normally
				// TODO: Implement local file handling
			}
		}
	}

	return connect.NewResponse(resp), nil
}

// executeFileCheckOnNodes executes file checks on selected nodes
func (s *CELValidationServer) executeFileCheckOnNodes(
	ctx context.Context,
	expression string,
	inputName string,
	fileInput *celv1.FileInput,
) ([]*NodeExecutionResult, error) {
	// Select nodes based on file input configuration
	nodes, err := s.selectNodesForExecution(ctx, fileInput)
	if err != nil {
		return nil, fmt.Errorf("failed to select nodes: %w", err)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes matched selection criteria")
	}

	log.Printf("[ValidateCEL] Selected %d nodes for file check", len(nodes))

	// Execute on each node
	results := make([]*NodeExecutionResult, 0, len(nodes))
	for _, node := range nodes {
		result := s.executeOnNode(ctx, node.Name, expression, inputName, fileInput)
		results = append(results, result)
	}

	return results, nil
}

// NodeExecutionResult represents the result from a single node
type NodeExecutionResult struct {
	NodeName string
	Success  bool
	Error    string
	Duration time.Duration
}

// selectNodesForExecution selects nodes based on file input configuration
func (s *CELValidationServer) selectNodesForExecution(ctx context.Context, fileInput *celv1.FileInput) ([]corev1.Node, error) {
	// Check if we have a real Kubernetes client
	if s.kubeClient == nil {
		// Return mock nodes for testing
		log.Println("[ValidateCELStream] Running in mock mode - returning test nodes")
		return s.getMockNodes(fileInput), nil
	}

	// Get all nodes
	nodeList, err := s.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var selectedNodes []corev1.Node

	switch target := fileInput.Target.(type) {
	case *celv1.FileInput_AllNodes:
		if target.AllNodes {
			selectedNodes = nodeList.Items
		}
	case *celv1.FileInput_NodeSelector:
		selector := target.NodeSelector

		// Filter by node names if specified
		if len(selector.NodeNames) > 0 {
			nameSet := make(map[string]bool)
			for _, name := range selector.NodeNames {
				nameSet[name] = true
			}

			for _, node := range nodeList.Items {
				if nameSet[node.Name] {
					selectedNodes = append(selectedNodes, node)
				}
			}
		} else if len(selector.Labels) > 0 {
			// Filter by labels (e.g., node-role.kubernetes.io/worker)
			labelSelector := labels.SelectorFromSet(selector.Labels)
			for _, node := range nodeList.Items {
				if labelSelector.Matches(labels.Set(node.Labels)) {
					selectedNodes = append(selectedNodes, node)
				}
			}
		}
	default:
		// Default to worker nodes
		for _, node := range nodeList.Items {
			if _, ok := node.Labels["node-role.kubernetes.io/worker"]; ok {
				selectedNodes = append(selectedNodes, node)
			}
		}
	}

	return selectedNodes, nil
}

// getMockNodes returns mock nodes for testing
func (s *CELValidationServer) getMockNodes(fileInput *celv1.FileInput) []corev1.Node {
	mockNodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "master-1",
				Labels: map[string]string{
					"node-role.kubernetes.io/master":        "",
					"node-role.kubernetes.io/control-plane": "",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "worker-1",
				Labels: map[string]string{
					"node-role.kubernetes.io/worker": "",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "worker-2",
				Labels: map[string]string{
					"node-role.kubernetes.io/worker": "",
				},
			},
		},
	}

	// Filter based on file input selector
	var selectedNodes []corev1.Node

	switch target := fileInput.Target.(type) {
	case *celv1.FileInput_AllNodes:
		if target.AllNodes {
			return mockNodes
		}
	case *celv1.FileInput_NodeSelector:
		selector := target.NodeSelector

		// Filter by labels
		if len(selector.Labels) > 0 {
			labelSelector := labels.SelectorFromSet(selector.Labels)
			for _, node := range mockNodes {
				if labelSelector.Matches(labels.Set(node.Labels)) {
					selectedNodes = append(selectedNodes, node)
				}
			}
			return selectedNodes
		}

		// Filter by node names
		if len(selector.NodeNames) > 0 {
			nameSet := make(map[string]bool)
			for _, name := range selector.NodeNames {
				nameSet[name] = true
			}
			for _, node := range mockNodes {
				if nameSet[node.Name] {
					selectedNodes = append(selectedNodes, node)
				}
			}
			return selectedNodes
		}
	}

	// Default: return worker nodes
	for _, node := range mockNodes {
		if _, ok := node.Labels["node-role.kubernetes.io/worker"]; ok {
			selectedNodes = append(selectedNodes, node)
		}
	}

	return selectedNodes
}

// executeOnNode executes the check on a specific node (simplified for now)
func (s *CELValidationServer) executeOnNode(
	ctx context.Context,
	nodeName string,
	expression string,
	inputName string,
	fileInput *celv1.FileInput,
) *NodeExecutionResult {
	startTime := time.Now()

	// TODO: Implement actual job creation and execution
	// For now, return a mock result
	log.Printf("[ValidateCEL] Would execute on node %s: %s", nodeName, fileInput.Path)

	return &NodeExecutionResult{
		NodeName: nodeName,
		Success:  true,
		Error:    "",
		Duration: time.Since(startTime),
	}
}

func (s *CELValidationServer) validateWithTestCases(
	ctx context.Context,
	req *connect.Request[celv1.ValidateCELRequest],
) (*connect.Response[celv1.ValidationResponse], error) {
	// Log the incoming request inputs
	log.Printf("ValidateCEL request received with %d inputs", len(req.Msg.Inputs))
	for i, input := range req.Msg.Inputs {
		log.Printf("Input %d: name=%s, type=%T", i, input.Name, input.InputType)
		if k8s := input.GetKubernetes(); k8s != nil {
			log.Printf("  Kubernetes: group=%s, version=%s, resource=%s", k8s.Group, k8s.Version, k8s.Resource)
		}
	}

	// Convert ValidateCELRequest to CELRule format for the verifier
	celRule := &celv1.CELRule{
		Id:          fmt.Sprintf("validation-%d", time.Now().UnixNano()),
		Name:        "Validation Rule",
		Description: "Rule created for validation",
		Expression:  req.Msg.Expression,
		Inputs:      req.Msg.Inputs,
		TestCases:   make([]*celv1.RuleTestCase, 0, len(req.Msg.TestCases)),
	}

	// Convert test cases to RuleTestCase format
	for i, testCase := range req.Msg.TestCases {
		ruleTestCase := &celv1.RuleTestCase{
			Id:             fmt.Sprintf("tc-%d", i),
			Name:           testCase.Description,
			Description:    testCase.Description,
			ExpectedResult: testCase.ExpectedResult,
			TestData:       make(map[string]string),
		}

		// Handle different test case formats
		if len(testCase.TestData) > 0 {
			// Direct inputs field - convert to JSON strings
			for inputName, inputData := range testCase.TestData {
				// // Marshal to JSON string
				// jsonBytes, err := json.Marshal(inputData)
				// if err != nil {
				// 	return nil, connect.NewError(connect.CodeInvalidArgument,
				// 		fmt.Errorf("failed to marshal input %s in test case %d: %w", inputName, i, err))
				// }

				// // For test cases without rule inputs, use the data as-is
				// // The verifier will handle any necessary formatting
				ruleTestCase.TestData[inputName] = inputData
			}
		} else {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("test case %d has no test data", i))
		}

		celRule.TestCases = append(celRule.TestCases, ruleTestCase)
	}

	// Use the new verifier
	verifier := verification.NewCELRuleVerifier()
	verificationResult, err := verifier.VerifyCELRule(ctx, celRule)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("verification failed: %w", err))
	}

	// Convert verification results back to ValidationResponse format
	resp := &celv1.ValidationResponse{
		Success: verificationResult.OverallPassed,
		Results: make([]*celv1.ValidationResult, 0, len(verificationResult.TestCases)),
	}

	for _, testResult := range verificationResult.TestCases {
		// Create a clearer message about what happened
		var details string

		// testResult.ActualResult is the CEL expression evaluation result (PASS/FAIL)
		expressionPassed := testResult.ActualResult == "PASS"

		if testResult.ExpectedResult {
			// Test expected the expression to pass (evaluate to true)
			if expressionPassed {
				details = "✅ Expression correctly evaluated to true (passed) as expected"
			} else {
				details = "❌ Expression evaluated to false (failed) but was expected to pass"
			}
		} else {
			// Test expected the expression to fail (evaluate to false)
			if expressionPassed {
				details = "❌ Expression evaluated to true (passed) but was expected to fail"
			} else {
				details = "✅ Expression correctly evaluated to false (failed) as expected"
			}
		}

		// Add the actual vs expected in a clearer format
		details += fmt.Sprintf("\n   Expected expression to: %s | Actual result: %s",
			map[bool]string{true: "PASS", false: "FAIL"}[testResult.ExpectedResult],
			testResult.ActualResult)

		resp.Results = append(resp.Results, &celv1.ValidationResult{
			TestCase: testResult.TestCaseName,
			Passed:   testResult.Passed,
			Error:    testResult.Error,
			Details:  details,
		})
	}

	// Add performance metrics from actual timing
	if len(resp.Results) > 0 {
		totalMs := verificationResult.Duration.Milliseconds()
		avgMs := totalMs / int64(len(resp.Results))
		if avgMs == 0 && totalMs > 0 {
			avgMs = 1 // Minimum 1ms average if we have some duration
		}

		resp.Performance = &celv1.PerformanceMetrics{
			TotalTimeMs:    totalMs,
			AverageTimeMs:  avgMs,
			ResourcesCount: int32(len(resp.Results)),
		}
	}

	return connect.NewResponse(resp), nil
}

// ValidateCELStream implements bidirectional streaming for real-time validation
func (s *CELValidationServer) ValidateCELStream(
	ctx context.Context,
	stream *connect.BidiStream[celv1.ValidateCELStreamRequest, celv1.ValidateCELStreamResponse],
) error {
	log.Println("[ValidateCELStream] New stream connection established")

	// Track active jobs for this stream
	activeJobs := make(map[string]*NodeJobTracker)
	defer func() {
		// Cleanup jobs on stream close
		s.jobTrackerMu.Lock()
		for jobID := range activeJobs {
			delete(s.jobTrackers, jobID)
		}
		s.jobTrackerMu.Unlock()
	}()

	// Process incoming messages
	for {
		req, err := stream.Receive()
		if err != nil {
			if err == io.EOF {
				log.Println("[ValidateCELStream] Stream closed by client")
				return nil
			}
			return err
		}

		switch msg := req.Request.(type) {
		case *celv1.ValidateCELStreamRequest_ValidationRequest:
			// Handle validation request
			if err := s.handleStreamValidation(ctx, stream, msg.ValidationRequest, activeJobs); err != nil {
				// Send error response
				stream.Send(&celv1.ValidateCELStreamResponse{
					Response: &celv1.ValidateCELStreamResponse_Error{
						Error: &celv1.StreamError{
							Error:   fmt.Sprintf("%v", err),
							Details: "Failed to start validation",
						},
					},
				})
			}

		case *celv1.ValidateCELStreamRequest_Control:
			// Handle control messages
			if err := s.handleStreamControl(ctx, stream, msg.Control, activeJobs); err != nil {
				stream.Send(&celv1.ValidateCELStreamResponse{
					Response: &celv1.ValidateCELStreamResponse_Error{
						Error: &celv1.StreamError{
							Error:   fmt.Sprintf("%v", err),
							Details: "Failed to process control message",
						},
					},
				})
			}
		}
	}
}

func (s *CELValidationServer) handleStreamControl(ctx context.Context, stream *connect.BidiStream[celv1.ValidateCELStreamRequest, celv1.ValidateCELStreamResponse], control *celv1.StreamControl, activeJobs map[string]*NodeJobTracker) error {
	log.Printf("[ValidateCELStream] Control message: %v for node=%s, job=%s",
		control.Action, control.NodeName, control.JobId)

	switch control.Action {
	case celv1.StreamControl_CANCEL:
		// Cancel specific job or all jobs for a node
		if control.JobId != "" {
			if tracker, ok := activeJobs[control.JobId]; ok {
				tracker.Status = celv1.NodeScannerStatus_CANCELLED
				tracker.Message = "Cancelled by user"
				// TODO: Actually cancel the Kubernetes job
			}
		} else if control.NodeName != "" {
			// Cancel all jobs for a node
			for _, tracker := range activeJobs {
				if tracker.NodeName == control.NodeName {
					tracker.Status = celv1.NodeScannerStatus_CANCELLED
					tracker.Message = "Cancelled by user"
					// TODO: Actually cancel the Kubernetes job
				}
			}
		}

	case celv1.StreamControl_PAUSE:
		// TODO: Implement pause functionality
		return fmt.Errorf("pause not yet implemented")

	case celv1.StreamControl_RESUME:
		// TODO: Implement resume functionality
		return fmt.Errorf("resume not yet implemented")
	}

	return nil
}

func (s *CELValidationServer) handleStreamValidation(ctx context.Context, stream *connect.BidiStream[celv1.ValidateCELStreamRequest, celv1.ValidateCELStreamResponse], request *celv1.ValidateCELRequest, activeJobs map[string]*NodeJobTracker) error {
	log.Printf("[ValidateCELStream] Processing validation request with %d inputs", len(request.Inputs))

	// Find file inputs that need node execution
	var nodeFileInputs []*celv1.RuleInput
	for _, input := range request.Inputs {
		if fileInput, ok := input.InputType.(*celv1.RuleInput_File); ok {
			f := fileInput.File
			// Check if this needs node execution
			switch f.Target.(type) {
			case *celv1.FileInput_NodeSelector, *celv1.FileInput_AllNodes:
				nodeFileInputs = append(nodeFileInputs, input)
			}
		}
	}

	if len(nodeFileInputs) == 0 {
		return fmt.Errorf("no node-targeted file inputs found")
	}

	// Select nodes for each file input
	totalNodes := 0
	nodeJobs := make(map[string][]*celv1.RuleInput) // node -> inputs

	for _, input := range nodeFileInputs {
		fileInput := input.GetFile()
		nodes, err := s.selectNodesForExecution(ctx, fileInput)
		if err != nil {
			return fmt.Errorf("failed to select nodes: %w", err)
		}

		for _, node := range nodes {
			nodeJobs[node.Name] = append(nodeJobs[node.Name], input)
		}
		totalNodes = len(nodeJobs)
	}

	// Send initial progress
	progress := &celv1.ValidationProgress{
		TotalNodes:     int32(totalNodes),
		CompletedNodes: 0,
		FailedNodes:    0,
		PendingNodes:   int32(totalNodes),
		NodeStatuses:   make(map[string]*celv1.NodeScannerStatus),
	}

	log.Printf("[ValidateCELStream] Selected nodes for execution: %v", nodeJobs)

	// Initialize node statuses
	for nodeName := range nodeJobs {
		jobID := fmt.Sprintf("stream-%s-%d", strings.ReplaceAll(nodeName, ".", "-"), time.Now().UnixNano())
		progress.NodeStatuses[nodeName] = &celv1.NodeScannerStatus{
			NodeName:  nodeName,
			JobId:     jobID,
			Status:    celv1.NodeScannerStatus_PENDING,
			Message:   "Waiting to start",
			Timestamp: time.Now().Unix(),
		}
	}

	// Send initial progress
	if err := stream.Send(&celv1.ValidateCELStreamResponse{
		Response: &celv1.ValidateCELStreamResponse_Progress{
			Progress: progress,
		},
	}); err != nil {
		return err
	}

	// Start jobs for each node
	for nodeName, inputs := range nodeJobs {
		jobID := progress.NodeStatuses[nodeName].JobId

		// Create job tracker
		tracker := &NodeJobTracker{
			JobID:      jobID,
			NodeName:   nodeName,
			Status:     celv1.NodeScannerStatus_SCHEDULED,
			Message:    "Creating job",
			StartTime:  time.Now(),
			ResultChan: make(chan *celv1.NodeValidationResult, len(inputs)),
		}

		// Register tracker
		s.jobTrackerMu.Lock()
		s.jobTrackers[jobID] = tracker
		activeJobs[jobID] = tracker
		s.jobTrackerMu.Unlock()

		// Start goroutine to execute on node
		go s.executeNodeJobsWithStream(ctx, stream, tracker, request.Expression, inputs, progress)
	}

	return nil
}

// executeNodeJobsWithStream executes validation on a node and streams results
func (s *CELValidationServer) executeNodeJobsWithStream(
	ctx context.Context,
	stream *connect.BidiStream[celv1.ValidateCELStreamRequest, celv1.ValidateCELStreamResponse],
	tracker *NodeJobTracker,
	expression string,
	inputs []*celv1.RuleInput,
	progress *celv1.ValidationProgress,
) {
	// Update status to running
	tracker.Status = celv1.NodeScannerStatus_RUNNING
	tracker.Message = "Executing validation"

	statusUpdate := &celv1.ValidateCELStreamResponse{
		Response: &celv1.ValidateCELStreamResponse_NodeStatus{
			NodeStatus: &celv1.NodeScannerStatus{
				NodeName:  tracker.NodeName,
				JobId:     tracker.JobID,
				Status:    tracker.Status,
				Message:   tracker.Message,
				Timestamp: time.Now().Unix(),
			},
		},
	}
	stream.Send(statusUpdate)

	// Execute each input on the node
	for _, input := range inputs {
		fileInput := input.GetFile()

		// For now, simulate execution - in real implementation, this would create a K8s job
		result := &celv1.NodeValidationResult{
			NodeName:   tracker.NodeName,
			JobId:      tracker.JobID,
			FilePath:   fileInput.Path,
			Expression: expression,
			Result: &celv1.ValidationResult{
				TestCase: fmt.Sprintf("Node %s - %s", tracker.NodeName, fileInput.Path),
				Passed:   true, // Simulated
				Details:  "Simulated validation result",
			},
			DurationMs: 100, // Simulated
		}

		// Stream the result
		stream.Send(&celv1.ValidateCELStreamResponse{
			Response: &celv1.ValidateCELStreamResponse_NodeResult{
				NodeResult: result,
			},
		})
	}

	// Update final status
	tracker.Status = celv1.NodeScannerStatus_COMPLETED
	tracker.Message = "Validation completed"

	// Update progress
	s.jobTrackerMu.Lock()
	progress.CompletedNodes++
	progress.PendingNodes--
	progress.NodeStatuses[tracker.NodeName].Status = tracker.Status
	progress.NodeStatuses[tracker.NodeName].Message = tracker.Message
	s.jobTrackerMu.Unlock()

	// Send final status and progress
	stream.Send(&celv1.ValidateCELStreamResponse{
		Response: &celv1.ValidateCELStreamResponse_NodeStatus{
			NodeStatus: &celv1.NodeScannerStatus{
				NodeName:  tracker.NodeName,
				JobId:     tracker.JobID,
				Status:    tracker.Status,
				Message:   tracker.Message,
				Timestamp: time.Now().Unix(),
			},
		},
	})

	stream.Send(&celv1.ValidateCELStreamResponse{
		Response: &celv1.ValidateCELStreamResponse_Progress{
			Progress: progress,
		},
	})
}

// ChatAssist implements the AI-assisted CEL rule generation and validation
func (s *CELValidationServer) ChatAssist(
	ctx context.Context,
	req *connect.Request[celv1.ChatAssistRequest],
	stream *connect.ServerStream[celv1.ChatAssistResponse],
) error {
	log.Printf("[ChatAssist] New request: %s", req.Msg.Message)
	log.Printf("[ChatAssist] Conversation ID: %s", req.Msg.ConversationId)
	log.Printf("[ChatAssist] Context type: %T", req.Msg.Context)

	// Initialize chat handler if not already done
	if s.chatHandler == nil {
		// Check for OpenAI key
		if os.Getenv("OPENAI_API_KEY") == "" {
			// Send a helpful message about setting up OpenAI
			return stream.Send(&celv1.ChatAssistResponse{
				Content: &celv1.ChatAssistResponse_Text{
					Text: &celv1.TextMessage{
						Text: `⚠️ OpenAI API key not found.

To enable AI-powered rule generation, please set your OpenAI API key:
export OPENAI_API_KEY="your-api-key"

For now, I'll use pattern-based rule generation.`,
						Type: "warning",
					},
				},
				Timestamp: time.Now().Unix(),
			})
		}

		s.chatHandler = NewChatHandler(s)
		log.Println("[ChatAssist] Chat handler initialized")
	}

	// Delegate to chat handler
	return s.chatHandler.HandleChatAssist(ctx, req, stream)
}

// generateCELRule generates a CEL rule based on user intent (simplified for now)
func (s *CELValidationServer) generateCELRule(message string, context *celv1.RuleGenerationContext) (string, string) {
	lowerMessage := strings.ToLower(message)
	resourceType := strings.ToLower(context.ResourceType)

	// Simple rule generation based on common patterns
	if strings.Contains(lowerMessage, "resource limit") || strings.Contains(lowerMessage, "resource limits") {
		if resourceType == "pod" {
			return "resource.spec.containers.all(c, has(c.resources.limits))",
				"This rule ensures all containers in a pod have resource limits defined"
		} else if resourceType == "deployment" {
			return "resource.spec.template.spec.containers.all(c, has(c.resources.limits))",
				"This rule ensures all containers in a deployment's pod template have resource limits defined"
		}
	}

	if strings.Contains(lowerMessage, "replica") || strings.Contains(lowerMessage, "replicas") {
		if resourceType == "deployment" {
			return "resource.spec.replicas >= 2",
				"This rule ensures deployments have at least 2 replicas for high availability"
		}
	}

	if strings.Contains(lowerMessage, "label") {
		return "has(resource.metadata.labels) && resource.metadata.labels.size() > 0",
			"This rule ensures resources have at least one label defined"
	}

	if strings.Contains(lowerMessage, "security context") {
		if resourceType == "pod" {
			return "has(resource.spec.securityContext) && resource.spec.securityContext.runAsNonRoot == true",
				"This rule ensures pods run with non-root security context"
		}
	}

	// Default rule
	return "has(resource.metadata.name)",
		"This is a basic rule that checks if the resource has a name"
}

// discoverResourcesForValidation discovers resources in the cluster
func (s *CELValidationServer) discoverResourcesForValidation(ctx context.Context, context *celv1.RuleGenerationContext) (*celv1.ResourcesFound, error) {
	// Use the existing DiscoverResources method
	req := &celv1.DiscoverResourcesRequest{
		Namespace: context.Namespace,
		GvrFilters: []*celv1.GVRFilter{
			{
				Group:    getGroupFromResourceType(context.ApiVersion),
				Version:  getVersionFromResourceType(context.ApiVersion),
				Resource: pluralizeResourceType(context.ResourceType),
			},
		},
		Options: &celv1.DiscoveryOptions{
			IncludeCount:   true,
			IncludeSamples: false,
		},
	}

	resp, err := s.DiscoverResources(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}

	if len(resp.Msg.Resources) == 0 {
		return &celv1.ResourcesFound{
			ResourceType: context.ResourceType,
			Count:        0,
			Namespaces:   []string{},
		}, nil
	}

	// Extract namespaces
	namespaceSet := make(map[string]bool)
	totalCount := int32(0)
	for _, resource := range resp.Msg.Resources {
		totalCount += resource.Count
		if resource.Namespace != "" {
			namespaceSet[resource.Namespace] = true
		}
	}

	namespaces := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	return &celv1.ResourcesFound{
		ResourceType: context.ResourceType,
		Count:        totalCount,
		Namespaces:   namespaces,
	}, nil
}

// validateGeneratedRule validates a generated rule against live resources
func (s *CELValidationServer) validateGeneratedRule(ctx context.Context, rule string, context *celv1.RuleGenerationContext) (*celv1.ValidationMessage, error) {
	// Create a validation request
	req := &celv1.ValidateCELRequest{
		Expression: rule,
		Inputs: []*celv1.RuleInput{
			{
				Name: "resource",
				InputType: &celv1.RuleInput_Kubernetes{
					Kubernetes: &celv1.KubernetesInput{
						Group:     getGroupFromResourceType(context.ApiVersion),
						Version:   getVersionFromResourceType(context.ApiVersion),
						Resource:  pluralizeResourceType(context.ResourceType),
						Namespace: context.Namespace,
					},
				},
			},
		},
	}

	resp, err := s.ValidateCEL(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}

	// Convert to validation message
	passedCount := int32(0)
	failedCount := int32(0)
	details := make([]*celv1.ValidationDetail, 0)

	for _, result := range resp.Msg.Results {
		if result.Passed {
			passedCount++
		} else {
			failedCount++
		}

		// Extract resource name from test case (simplified)
		detail := &celv1.ValidationDetail{
			ResourceName: result.TestCase,
			Passed:       result.Passed,
			Reason:       result.Details,
		}
		details = append(details, detail)
	}

	return &celv1.ValidationMessage{
		Success:     resp.Msg.Success,
		PassedCount: passedCount,
		FailedCount: failedCount,
		Details:     details,
	}, nil
}

// sendHelpMessage sends help information
func (s *CELValidationServer) sendHelpMessage(stream *connect.ServerStream[celv1.ChatAssistResponse]) error {
	helpText := `# CEL Rule Assistant Help

I can help you:
1. **Generate CEL rules** - Describe what you want to validate
2. **Test rules** - Validate against live cluster or test data
3. **Learn CEL syntax** - Get examples and explanations

## Common Commands:
- "Generate a rule for [resource type] to [validation intent]"
- "Show me examples of CEL rules"
- "Validate pods in namespace [name]"
- "Test this rule: [CEL expression]"

## Supported Resources:
- Pods, Deployments, Services, ConfigMaps, Secrets
- StatefulSets, DaemonSets, Jobs, CronJobs
- And many more Kubernetes resources!

What would you like to do?`

	return stream.Send(&celv1.ChatAssistResponse{
		Content: &celv1.ChatAssistResponse_Text{
			Text: &celv1.TextMessage{
				Text: helpText,
				Type: "info",
			},
		},
		Timestamp: time.Now().Unix(),
	})
}

// sendExampleRules sends example CEL rules
func (s *CELValidationServer) sendExampleRules(stream *connect.ServerStream[celv1.ChatAssistResponse]) error {
	examples := `# Example CEL Rules

## Pod Validation:
- **Resource Limits**: ` + "`resource.spec.containers.all(c, has(c.resources.limits))`" + `
- **Security Context**: ` + "`has(resource.spec.securityContext) && resource.spec.securityContext.runAsNonRoot`" + `
- **Liveness Probe**: ` + "`resource.spec.containers.all(c, has(c.livenessProbe))`" + `

## Deployment Validation:
- **Replica Count**: ` + "`resource.spec.replicas >= 2`" + `
- **Rolling Update**: ` + "`resource.spec.strategy.type == 'RollingUpdate'`" + `
- **Pod Labels**: ` + "`has(resource.spec.template.metadata.labels)`" + `

## Service Validation:
- **Selector Labels**: ` + "`has(resource.spec.selector) && resource.spec.selector.size() > 0`" + `
- **Port Names**: ` + "`resource.spec.ports.all(p, has(p.name))`" + `

Try one of these or describe your own validation needs!`

	return stream.Send(&celv1.ChatAssistResponse{
		Content: &celv1.ChatAssistResponse_Text{
			Text: &celv1.TextMessage{
				Text: examples,
				Type: "info",
			},
		},
		Timestamp: time.Now().Unix(),
	})
}

// Helper functions for resource type parsing
func getGroupFromResourceType(apiVersion string) string {
	if apiVersion == "" || apiVersion == "v1" {
		return ""
	}
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}

func getVersionFromResourceType(apiVersion string) string {
	if apiVersion == "" {
		return "v1"
	}
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 2 {
		return parts[1]
	}
	return apiVersion
}

func pluralizeResourceType(resourceType string) string {
	resourceType = strings.ToLower(resourceType)

	// Special cases
	specialCases := map[string]string{
		"pod":                   "pods",
		"service":               "services",
		"deployment":            "deployments",
		"statefulset":           "statefulsets",
		"daemonset":             "daemonsets",
		"replicaset":            "replicasets",
		"configmap":             "configmaps",
		"secret":                "secrets",
		"ingress":               "ingresses",
		"persistentvolume":      "persistentvolumes",
		"persistentvolumeclaim": "persistentvolumeclaims",
		"storageclass":          "storageclasses",
		"namespace":             "namespaces",
		"node":                  "nodes",
		"job":                   "jobs",
		"cronjob":               "cronjobs",
	}

	if plural, ok := specialCases[resourceType]; ok {
		return plural
	}

	// Default: add 's'
	return resourceType + "s"
}

// DiscoverResources implements resource discovery with dynamic API discovery
func (s *CELValidationServer) DiscoverResources(
	ctx context.Context,
	req *connect.Request[celv1.DiscoverResourcesRequest],
) (*connect.Response[celv1.ResourceDiscoveryResponse], error) {
	log.Printf("DiscoverResources request: namespace=%s, gvr_filters=%d", req.Msg.Namespace, len(req.Msg.GvrFilters))

	// Extract options with defaults
	options := req.Msg.Options
	if options == nil {
		options = &celv1.DiscoveryOptions{
			IncludeCount:      true,
			IncludeSamples:    false,
			MaxSamplesPerType: 1,
		}
	}

	// Apply defaults for zero values
	if options.MaxSamplesPerType <= 0 {
		options.MaxSamplesPerType = 1
	}

	resp := &celv1.ResourceDiscoveryResponse{
		Success:   true,
		Resources: make([]*celv1.DiscoveredResource, 0),
	}

	// If no namespace specified, use all namespaces
	namespace := req.Msg.Namespace
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}

	// If GVR filters are provided, use them
	if len(req.Msg.GvrFilters) > 0 {
		for _, filter := range req.Msg.GvrFilters {
			resource, err := s.discoverResource(ctx, filter.Group, filter.Version, filter.Resource, namespace, options)
			if err != nil {
				log.Printf("Warning: failed to discover resource %s.%s/%s: %v", filter.Group, filter.Version, filter.Resource, err)
				continue
			}

			// Derive kind from resource name if not in filter
			kind := filter.Resource
			if strings.HasSuffix(kind, "ses") {
				kind = strings.TrimSuffix(kind, "es")
			} else if strings.HasSuffix(kind, "ies") {
				kind = strings.TrimSuffix(kind, "ies") + "y"
			} else if strings.HasSuffix(kind, "s") {
				kind = strings.TrimSuffix(kind, "s")
			}
			kind = strings.Title(kind)

			resource.Kind = kind
			resource.ApiVersion = filter.Group + "/" + filter.Version

			resp.Resources = append(resp.Resources, resource)
		}
		return connect.NewResponse(resp), nil
	}

	// Otherwise, discover all available resources dynamically
	discoveryClient := s.kubeClient.Discovery()

	// Get all API resources
	_, apiResourceLists, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		// Don't fail completely on discovery errors
		log.Printf("Warning: partial discovery error: %v", err)
	}

	// Track what we've already discovered to avoid duplicates
	seen := make(map[string]bool)

	// First pass: collect all resources without counting (fast mode)
	type resourceInfo struct {
		gv          schema.GroupVersion
		apiResource metav1.APIResource
		key         string
	}

	var resourcesToDiscover []resourceInfo

	for _, apiResourceList := range apiResourceLists {
		gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			continue
		}

		for _, apiResource := range apiResourceList.APIResources {
			// Skip subresources
			if strings.Contains(apiResource.Name, "/") {
				continue
			}

			// Skip non-listable resources
			if !contains(apiResource.Verbs, "list") {
				continue
			}

			// Create unique key
			key := fmt.Sprintf("%s/%s/%s", gv.Group, gv.Version, apiResource.Name)
			if seen[key] {
				continue
			}
			seen[key] = true

			resourcesToDiscover = append(resourcesToDiscover, resourceInfo{
				gv:          gv,
				apiResource: apiResource,
				key:         key,
			})
		}
	}

	log.Printf("Found %d unique resource types to discover", len(resourcesToDiscover))

	// If we don't need counts or samples, just return the resource types (super fast mode)
	if !options.IncludeCount && !options.IncludeSamples {
		for _, info := range resourcesToDiscover {
			apiVersion := info.gv.Version
			if info.gv.Group != "" {
				apiVersion = info.gv.Group + "/" + info.gv.Version
			}

			resp.Resources = append(resp.Resources, &celv1.DiscoveredResource{
				Kind:       info.apiResource.Kind,
				ApiVersion: apiVersion,
				Count:      -1, // Indicate count not available
			})
		}

		log.Printf("Fast discovery completed: found %d resource types", len(resp.Resources))
		return connect.NewResponse(resp), nil
	}

	// If we need counts or samples, use parallel processing
	type discoveryResult struct {
		resource *celv1.DiscoveredResource
		info     resourceInfo
		err      error
	}

	// Use a worker pool for parallel discovery
	workerCount := 10 // Limit concurrent API calls
	if len(resourcesToDiscover) < workerCount {
		workerCount = len(resourcesToDiscover)
	}

	jobs := make(chan resourceInfo, len(resourcesToDiscover))
	results := make(chan discoveryResult, len(resourcesToDiscover))

	// Start workers
	for w := 0; w < workerCount; w++ {
		go func() {
			for info := range jobs {
				// Discover resource with options
				resource, err := s.discoverResource(ctx, info.gv.Group, info.gv.Version, info.apiResource.Name, namespace, options)
				if err != nil {
					results <- discoveryResult{nil, info, err}
					continue
				}

				apiVersion := info.gv.Version
				if info.gv.Group != "" {
					apiVersion = info.gv.Group + "/" + info.gv.Version
				}

				resource.ApiVersion = apiVersion
				resource.Kind = info.apiResource.Kind

				results <- discoveryResult{resource, info, nil}
			}
		}()
	}

	// Send jobs
	for _, info := range resourcesToDiscover {
		jobs <- info
	}
	close(jobs)

	// Collect results
	successCount := 0
	for i := 0; i < len(resourcesToDiscover); i++ {
		result := <-results
		if result.err != nil {
			// Log but don't fail
			log.Printf("Debug: failed to discover %s: %v", result.info.key, result.err)
			continue
		}
		if result.resource != nil {
			resp.Resources = append(resp.Resources, result.resource)
			successCount++
		}
	}

	log.Printf("Parallel discovery completed: discovered %d/%d resources", successCount, len(resourcesToDiscover))

	// Sort by count descending for better UX (only if counts are included)
	if options.IncludeCount {
		sort.Slice(resp.Resources, func(i, j int) bool {
			return resp.Resources[i].Count > resp.Resources[j].Count
		})
	}

	return connect.NewResponse(resp), nil
}

// countResources counts resources of a specific type
func (s *CELValidationServer) countResources(ctx context.Context, group, version, resource, namespace string) (int32, error) {
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	// Use dynamic client for maximum compatibility
	list, err := s.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{
		Limit: 1000, // Limit for performance
	})
	if err != nil {
		return 0, err
	}

	return int32(len(list.Items)), nil
}

// discoverResource discovers a specific resource type with optional samples
func (s *CELValidationServer) discoverResource(ctx context.Context, group, version, resource, namespace string, options *celv1.DiscoveryOptions) (*celv1.DiscoveredResource, error) {
	// Add timeout for each resource discovery to prevent hanging
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	result := &celv1.DiscoveredResource{}

	// If we don't need count or samples, return immediately
	if !options.IncludeCount && !options.IncludeSamples {
		return result, nil
	}

	// Get list options
	listOptions := metav1.ListOptions{}
	if options.IncludeSamples || options.IncludeCount {
		// If we only need samples, limit the query
		if options.IncludeSamples && !options.IncludeCount {
			listOptions.Limit = int64(options.MaxSamplesPerType)
		} else if options.IncludeCount {
			// For counting, we can use a smaller limit and check if there's more
			listOptions.Limit = 100 // Reduced from 1000 for faster initial response
		}
	}

	// Use dynamic client for maximum compatibility
	list, err := s.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	// Set count if requested
	if options.IncludeCount {
		if len(list.Items) < int(listOptions.Limit) {
			// We got all items in one query
			result.Count = int32(len(list.Items))
		} else {
			// There might be more items, but for speed, we'll estimate
			// You could do a follow-up query with limit=1 and use the ResourceVersion
			// to get the exact count, but that would be slower
			result.Count = int32(len(list.Items)) // Just show what we got
			// Optionally add a flag to indicate this is a partial count
		}
	}

	// Add samples if requested
	if options.IncludeSamples && len(list.Items) > 0 {
		result.Samples = make([]*celv1.ResourceSample, 0, options.MaxSamplesPerType)

		for i, item := range list.Items {
			if i >= int(options.MaxSamplesPerType) {
				break
			}

			sample := &celv1.ResourceSample{
				Name: item.GetName(),
			}

			// Add namespace if present
			if ns := item.GetNamespace(); ns != "" {
				sample.Namespace = ns
			}

			// Add labels if present (but limit to avoid huge responses)
			if labels := item.GetLabels(); len(labels) > 0 && len(labels) < 20 {
				sample.Labels = labels
			}

			// Add creation timestamp if available
			if creationTime := item.GetCreationTimestamp(); !creationTime.IsZero() {
				sample.CreatedAt = timestamppb.New(creationTime.Time)
			}

			result.Samples = append(result.Samples, sample)
		}
	}

	return result, nil
}

// contains checks if a string slice contains a value
func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

// ExecuteAgentTask implements agent task execution with CEL validation
func (s *CELValidationServer) ExecuteAgentTask(
	ctx context.Context,
	req *connect.Request[celv1.AgentExecutionRequest],
) (*connect.Response[celv1.AgentExecutionResponse], error) {
	log.Printf("ExecuteAgentTask: taskId=%s, agentRole=%s", req.Msg.Task.Id, req.Msg.AgentRole)

	startTime := time.Now()
	resp := &celv1.AgentExecutionResponse{
		Success: true,
		Logs:    []string{},
	}

	// Parse context
	var taskContext map[string]interface{}
	if req.Msg.Task.ContextJson != "" {
		if err := json.Unmarshal([]byte(req.Msg.Task.ContextJson), &taskContext); err != nil {
			resp.Success = false
			resp.Error = fmt.Sprintf("failed to parse context: %v", err)
			return connect.NewResponse(resp), nil
		}
	}

	// Execute based on task type
	switch req.Msg.Task.Type {
	case "discover_resources":
		// Use resource discovery
		namespace := req.Msg.Task.Parameters["namespace"]
		if namespace == "" {
			namespace = "default"
		}
		discoverReq := &celv1.DiscoverResourcesRequest{
			// We can use GVR filters if needed
		}
		discoverResp, err := s.DiscoverResources(ctx, connect.NewRequest(discoverReq))
		if err != nil {
			resp.Success = false
			resp.Error = fmt.Sprintf("resource discovery failed: %v", err)
		} else {
			resultJSON, _ := json.Marshal(discoverResp.Msg)
			resp.ResultJson = string(resultJSON)
			resp.Logs = append(resp.Logs, fmt.Sprintf("Discovered %d resource types", len(discoverResp.Msg.Resources)))
		}

	case "validate_cel":
		// Create input based on parameters
		k8sInput := &celv1.RuleInput{
			Name: "resource",
			InputType: &celv1.RuleInput_Kubernetes{
				Kubernetes: &celv1.KubernetesInput{
					Group:     req.Msg.Task.Parameters["group"],
					Version:   req.Msg.Task.Parameters["version"],
					Resource:  req.Msg.Task.Parameters["resource"],
					Namespace: req.Msg.Task.Parameters["namespace"],
				},
			},
		}

		// Validate CEL expression
		celReq := &celv1.ValidateCELRequest{
			Expression: req.Msg.Task.Parameters["expression"],
			Inputs:     []*celv1.RuleInput{k8sInput},
		}
		validateResp, err := s.ValidateCEL(ctx, connect.NewRequest(celReq))
		if err != nil {
			resp.Success = false
			resp.Error = fmt.Sprintf("CEL validation failed: %v", err)
		} else {
			resultJSON, _ := json.Marshal(validateResp.Msg)
			resp.ResultJson = string(resultJSON)
			resp.Logs = append(resp.Logs, fmt.Sprintf("Validated CEL expression with %d results", len(validateResp.Msg.Results)))
		}

	default:
		// For other task types, return a mock response
		result := map[string]interface{}{
			"taskId":     req.Msg.Task.Id,
			"taskType":   req.Msg.Task.Type,
			"agentRole":  req.Msg.AgentRole,
			"objective":  req.Msg.Task.Objective,
			"status":     "completed",
			"parameters": req.Msg.Task.Parameters,
		}
		resultJSON, _ := json.Marshal(result)
		resp.ResultJson = string(resultJSON)
		resp.Logs = append(resp.Logs, fmt.Sprintf("Executed task: %s", req.Msg.Task.Type))
	}

	resp.ExecutionTimeMs = time.Since(startTime).Milliseconds()
	return connect.NewResponse(resp), nil
}

// Rule Library RPC Methods

// SaveRule saves a rule to the library after validation
func (s *CELValidationServer) SaveRule(
	ctx context.Context,
	req *connect.Request[celv1.SaveRuleRequest],
) (*connect.Response[celv1.SaveRuleResponse], error) {
	log.Printf("[SaveRule] Saving rule: %s", req.Msg.Rule.Name)

	rule := req.Msg.Rule
	resp := &celv1.SaveRuleResponse{}

	// Validate and run test cases if requested
	if req.Msg.ValidateBeforeSave || req.Msg.RunTestCases {
		validationReq := &celv1.ValidateRuleWithTestCasesRequest{
			Rule:              rule,
			RunAgainstCluster: false,
		}

		validationResp, err := s.ValidateRuleWithTestCases(ctx, connect.NewRequest(validationReq))
		if err != nil {
			resp.Success = false
			resp.Error = fmt.Sprintf("validation failed: %v", err)
			return connect.NewResponse(resp), nil
		}

		resp.TestResults = validationResp.Msg.TestResults

		if !validationResp.Msg.AllPassed {
			resp.Success = false
			resp.Error = "not all test cases passed"
			return connect.NewResponse(resp), nil
		}

		// Mark as verified if all tests pass
		rule.IsVerified = true
	}

	// Save to store
	if err := s.ruleStore.Save(rule); err != nil {
		resp.Success = false
		resp.Error = fmt.Sprintf("failed to save rule: %v", err)
		return connect.NewResponse(resp), nil
	}

	resp.Success = true
	resp.RuleId = rule.Id

	return connect.NewResponse(resp), nil
}

// GetRule retrieves a rule from the library
func (s *CELValidationServer) GetRule(
	ctx context.Context,
	req *connect.Request[celv1.GetRuleRequest],
) (*connect.Response[celv1.GetRuleResponse], error) {
	log.Printf("[GetRule] Getting rule: %s", req.Msg.RuleId)

	rule, err := s.ruleStore.Get(req.Msg.RuleId)
	if err != nil {
		return connect.NewResponse(&celv1.GetRuleResponse{
			Found: false,
		}), nil
	}

	return connect.NewResponse(&celv1.GetRuleResponse{
		Rule:  rule,
		Found: true,
	}), nil
}

// ListRules lists rules from the library with filtering
func (s *CELValidationServer) ListRules(
	ctx context.Context,
	req *connect.Request[celv1.ListRulesRequest],
) (*connect.Response[celv1.ListRulesResponse], error) {
	log.Printf("[ListRules] Received request with pageSize=%d, filter=%+v", req.Msg.PageSize, req.Msg.Filter)

	rules, nextPageToken, totalCount, err := s.ruleStore.List(
		req.Msg.Filter,
		req.Msg.PageSize,
		req.Msg.PageToken,
		req.Msg.SortBy,
		req.Msg.Ascending,
	)

	if err != nil {
		log.Printf("[ListRules] Error: %v", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	log.Printf("[ListRules] Returning %d rules (total: %d)", len(rules), totalCount)

	return connect.NewResponse(&celv1.ListRulesResponse{
		Rules:         rules,
		NextPageToken: nextPageToken,
		TotalCount:    totalCount,
	}), nil
}

// UpdateRule updates an existing rule
func (s *CELValidationServer) UpdateRule(
	ctx context.Context,
	req *connect.Request[celv1.UpdateRuleRequest],
) (*connect.Response[celv1.UpdateRuleResponse], error) {
	log.Printf("[UpdateRule] Updating rule: %s", req.Msg.Rule.Id)

	resp := &celv1.UpdateRuleResponse{}

	// Validate and run test cases if requested
	if req.Msg.ValidateBeforeUpdate || req.Msg.RunTestCases {
		validationReq := &celv1.ValidateRuleWithTestCasesRequest{
			Rule:              req.Msg.Rule,
			RunAgainstCluster: false,
		}

		validationResp, err := s.ValidateRuleWithTestCases(ctx, connect.NewRequest(validationReq))
		if err != nil {
			resp.Success = false
			resp.Error = fmt.Sprintf("validation failed: %v", err)
			return connect.NewResponse(resp), nil
		}

		resp.TestResults = validationResp.Msg.TestResults

		if !validationResp.Msg.AllPassed {
			resp.Success = false
			resp.Error = "not all test cases passed"
			return connect.NewResponse(resp), nil
		}

		// Mark as verified if all tests pass
		req.Msg.Rule.IsVerified = true
	}

	// Update in store
	if err := s.ruleStore.Update(req.Msg.Rule, req.Msg.UpdateFields); err != nil {
		resp.Success = false
		resp.Error = fmt.Sprintf("failed to update rule: %v", err)
		return connect.NewResponse(resp), nil
	}

	resp.Success = true
	return connect.NewResponse(resp), nil
}

// DeleteRule deletes a rule from the library
func (s *CELValidationServer) DeleteRule(
	ctx context.Context,
	req *connect.Request[celv1.DeleteRuleRequest],
) (*connect.Response[celv1.DeleteRuleResponse], error) {
	log.Printf("[DeleteRule] Deleting rule: %s", req.Msg.RuleId)

	if err := s.ruleStore.Delete(req.Msg.RuleId); err != nil {
		return connect.NewResponse(&celv1.DeleteRuleResponse{
			Success: false,
			Error:   err.Error(),
		}), nil
	}

	return connect.NewResponse(&celv1.DeleteRuleResponse{
		Success: true,
	}), nil
}

// ExportRules exports rules in various formats
func (s *CELValidationServer) ExportRules(
	ctx context.Context,
	req *connect.Request[celv1.ExportRulesRequest],
) (*connect.Response[celv1.ExportRulesResponse], error) {
	log.Printf("[ExportRules] Exporting rules in format: %v", req.Msg.Format)

	data, contentType, ruleCount, err := s.ruleStore.ExportRules(
		req.Msg.RuleIds,
		req.Msg.Filter,
		req.Msg.Format,
		req.Msg.IncludeTestCases,
		req.Msg.IncludeMetadata,
	)

	if err != nil {
		return connect.NewResponse(&celv1.ExportRulesResponse{
			Success: false,
			Error:   err.Error(),
		}), nil
	}

	return connect.NewResponse(&celv1.ExportRulesResponse{
		Success:     true,
		Data:        data,
		ContentType: contentType,
		RuleCount:   ruleCount,
	}), nil
}

// ImportRules imports rules from various formats
func (s *CELValidationServer) ImportRules(
	ctx context.Context,
	req *connect.Request[celv1.ImportRulesRequest],
) (*connect.Response[celv1.ImportRulesResponse], error) {
	log.Printf("[ImportRules] Importing rules in format: %v", req.Msg.Format)

	// Validate rules if requested
	if req.Msg.Options.ValidateAll {
		// TODO: Validate each rule before import
	}

	imported, skipped, failed, results, err := s.ruleStore.ImportRules(
		req.Msg.Data,
		req.Msg.Format,
		req.Msg.Options,
	)

	if err != nil && !req.Msg.Options.SkipOnError {
		return connect.NewResponse(&celv1.ImportRulesResponse{
			Success: false,
			Error:   err.Error(),
		}), nil
	}

	return connect.NewResponse(&celv1.ImportRulesResponse{
		Success:       err == nil,
		ImportedCount: imported,
		SkippedCount:  skipped,
		FailedCount:   failed,
		Results:       results,
	}), nil
}

// ValidateRuleWithTestCases validates a rule with its test cases
func (s *CELValidationServer) ValidateRuleWithTestCases(
	ctx context.Context,
	req *connect.Request[celv1.ValidateRuleWithTestCasesRequest],
) (*connect.Response[celv1.ValidateRuleWithTestCasesResponse], error) {
	log.Printf("[ValidateRuleWithTestCases] Validating rule: %s", req.Msg.Rule.Name)

	rule := req.Msg.Rule
	resp := &celv1.ValidateRuleWithTestCasesResponse{
		AllPassed:   true,
		TestResults: make([]*celv1.TestCaseResult, 0),
	}

	// Run each test case
	for _, testCase := range rule.TestCases {
		startTime := time.Now()
		result := &celv1.TestCaseResult{
			TestCaseId:   testCase.Id,
			TestCaseName: testCase.Name,
		}

		// Create a CEL validation request
		valReq := &celv1.ValidateCELRequest{
			Expression: rule.Expression,
			Inputs:     rule.Inputs, // Add inputs from the rule - IMPORTANT for proper test data handling
			TestCases: []*celv1.RuleTestCase{
				{
					ExpectedResult: testCase.ExpectedResult,
					TestData:       testCase.TestData,
					Description:    testCase.Description,
				},
			},
		}

		valResp, err := s.ValidateCEL(ctx, connect.NewRequest(valReq))
		if err != nil {
			result.Passed = false
			result.Error = err.Error()
		} else if len(valResp.Msg.Results) > 0 {
			valResult := valResp.Msg.Results[0]
			result.Passed = valResult.Passed
			result.ActualResult = fmt.Sprintf("%v", valResult.Passed)
			if valResult.Error != "" {
				result.Error = valResult.Error
			}
		}

		result.DurationMs = time.Since(startTime).Milliseconds()
		resp.TestResults = append(resp.TestResults, result)

		if !result.Passed {
			resp.AllPassed = false
		}
	}

	// Optionally run against live cluster
	if req.Msg.RunAgainstCluster && resp.AllPassed {
		clusterReq := &celv1.ValidateCELRequest{
			Expression: rule.Expression,
			Inputs:     rule.Inputs,
		}

		clusterResp, err := s.ValidateCEL(ctx, connect.NewRequest(clusterReq))
		if err == nil {
			resp.ClusterResults = clusterResp.Msg.Results
		}
	}

	return connect.NewResponse(resp), nil
}

// Helper function to get the first input name from a rule
func getFirstInputName(rule *celv1.CELRule) string {
	if len(rule.Inputs) > 0 {
		return rule.Inputs[0].Name
	}
	return "resource"
}

// analyzeViolations analyzes which resources violate the rule and why
func (s *CELValidationServer) analyzeViolations(ctx context.Context, expression string, inputs []*celv1.RuleInput) string {
	// Create Kubernetes client
	clientset, err := s.createK8sClient()
	if err != nil {
		return fmt.Sprintf("Failed to create Kubernetes client: %v", err)
	}

	var violations []string

	// Check for specific rule patterns
	if strings.Contains(expression, "namespaces") && strings.Contains(expression, "networkpolicies") {
		violations = s.analyzeNamespaceNetworkPolicyViolations(ctx, clientset, expression)
	} else if strings.Contains(expression, "namespaces") && strings.Contains(expression, "pods") {
		violations = s.analyzeNamespacePodViolations(ctx, clientset, expression)
	} else if strings.Contains(expression, "deployments") && strings.Contains(expression, "replicas") {
		violations = s.analyzeDeploymentReplicaViolations(ctx, clientset, expression)
	} else {
		// Single resource analysis
		for _, input := range inputs {
			if k8sInput, ok := input.InputType.(*celv1.RuleInput_Kubernetes); ok {
				switch k8sInput.Kubernetes.Resource {
				case "pods":
					violations = s.analyzeSinglePodViolations(ctx, clientset, expression, k8sInput.Kubernetes)
				case "services":
					violations = s.analyzeServiceViolations(ctx, clientset, expression, k8sInput.Kubernetes)
				default:
					// Generic analysis based on expression patterns
					violations = append(violations, s.analyzeGenericViolations(ctx, clientset, expression, input)...)
				}
			}
		}
	}

	if len(violations) == 0 {
		return "Status: FAIL - Unable to determine specific violations for this rule pattern"
	}

	// Limit violations to first 10 to avoid huge responses
	if len(violations) > 10 {
		remainingCount := len(violations) - 10
		violations = violations[:10]
		violations = append(violations, fmt.Sprintf("... and %d more violations", remainingCount))
	}

	return fmt.Sprintf("Status: FAIL\n\nViolations found:\n- %s", strings.Join(violations, "\n- "))
}

// analyzeNamespaceNetworkPolicyViolations checks which namespaces lack network policies
func (s *CELValidationServer) analyzeNamespaceNetworkPolicyViolations(ctx context.Context, clientset *kubernetes.Clientset, expression string) []string {
	var violations []string

	// Get all namespaces
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return []string{fmt.Sprintf("Failed to list namespaces: %v", err)}
	}

	// Get all network policies
	allNetPolicies, err := clientset.NetworkingV1().NetworkPolicies("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return []string{fmt.Sprintf("Failed to list network policies: %v", err)}
	}

	// Group network policies by namespace
	netPolByNamespace := make(map[string]int)
	for _, np := range allNetPolicies.Items {
		netPolByNamespace[np.Namespace]++
	}

	// Check each namespace
	for _, ns := range namespaces.Items {
		// Skip system namespaces
		if strings.HasPrefix(ns.Name, "kube-") || ns.Name == "default" {
			continue
		}

		if count, exists := netPolByNamespace[ns.Name]; !exists || count == 0 {
			violations = append(violations, fmt.Sprintf("Namespace '%s' has no network policies", ns.Name))
		}
	}

	return violations
}

// analyzeNamespacePodViolations checks namespace-pod relationships
func (s *CELValidationServer) analyzeNamespacePodViolations(ctx context.Context, clientset *kubernetes.Clientset, expression string) []string {
	var violations []string

	// Get all namespaces
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return []string{fmt.Sprintf("Failed to list namespaces: %v", err)}
	}

	// Get all pods
	allPods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return []string{fmt.Sprintf("Failed to list pods: %v", err)}
	}

	// Group pods by namespace
	podsByNamespace := make(map[string]int)
	for _, pod := range allPods.Items {
		podsByNamespace[pod.Namespace]++
	}

	// Check if expression is looking for namespaces without pods
	if strings.Contains(expression, "exists") && strings.Contains(expression, "namespace == ns.metadata.name") {
		for _, ns := range namespaces.Items {
			if count, exists := podsByNamespace[ns.Name]; !exists || count == 0 {
				violations = append(violations, fmt.Sprintf("Namespace '%s' has no pods", ns.Name))
			}
		}
	}

	return violations
}

// analyzeDeploymentReplicaViolations checks deployment replica requirements
func (s *CELValidationServer) analyzeDeploymentReplicaViolations(ctx context.Context, clientset *kubernetes.Clientset, expression string) []string {
	var violations []string

	deployments, err := clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return []string{fmt.Sprintf("Failed to list deployments: %v", err)}
	}

	// Extract replica requirement from expression
	minReplicas := int32(2) // default
	if strings.Contains(expression, ">= 3") {
		minReplicas = 3
	} else if strings.Contains(expression, ">= 1") {
		minReplicas = 1
	}

	for _, deploy := range deployments.Items {
		if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas < minReplicas {
			violations = append(violations, fmt.Sprintf("Deployment '%s/%s' has %d replicas (required: >= %d)",
				deploy.Namespace, deploy.Name, *deploy.Spec.Replicas, minReplicas))
		}
	}

	return violations
}

// analyzeSinglePodViolations is the refactored version of the original analyzePodViolations
func (s *CELValidationServer) analyzeSinglePodViolations(ctx context.Context, clientset *kubernetes.Clientset, expression string, k8sInput *celv1.KubernetesInput) []string {
	var violations []string

	pods, err := clientset.CoreV1().Pods(k8sInput.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return []string{fmt.Sprintf("Failed to list pods: %v", err)}
	}

	// Analyze based on the expression pattern
	if strings.Contains(expression, "serviceAccountName") {
		// Check service account violations
		for _, pod := range pods.Items {
			if pod.Spec.ServiceAccountName == "" {
				violations = append(violations, fmt.Sprintf("Pod '%s/%s' has no service account specified",
					pod.Namespace, pod.Name))
			}
		}
	} else if strings.Contains(expression, "runAsUser") && strings.Contains(expression, "== 0") {
		// Check for pods running as root
		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.SecurityContext != nil &&
					container.SecurityContext.RunAsUser != nil &&
					*container.SecurityContext.RunAsUser == 0 {
					violations = append(violations, fmt.Sprintf("Pod '%s/%s' container '%s' runs as root (UID 0)",
						pod.Namespace, pod.Name, container.Name))
				} else if container.SecurityContext == nil &&
					pod.Spec.SecurityContext != nil &&
					pod.Spec.SecurityContext.RunAsUser != nil &&
					*pod.Spec.SecurityContext.RunAsUser == 0 {
					violations = append(violations, fmt.Sprintf("Pod '%s/%s' container '%s' inherits root UID from pod security context",
						pod.Namespace, pod.Name, container.Name))
				}
			}
		}
	} else if strings.Contains(expression, "resources.limits") {
		// Check for resource limits
		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Resources.Limits == nil {
					violations = append(violations, fmt.Sprintf("Pod '%s/%s' container '%s' has no resource limits",
						pod.Namespace, pod.Name, container.Name))
				} else {
					var missing []string
					if _, hasCPU := container.Resources.Limits[corev1.ResourceCPU]; !hasCPU {
						missing = append(missing, "CPU")
					}
					if _, hasMemory := container.Resources.Limits[corev1.ResourceMemory]; !hasMemory {
						missing = append(missing, "memory")
					}
					if len(missing) > 0 {
						violations = append(violations, fmt.Sprintf("Pod '%s/%s' container '%s' missing %s limits",
							pod.Namespace, pod.Name, container.Name, strings.Join(missing, " and ")))
					}
				}
			}
		}
	}

	return violations
}

// analyzeServiceViolations checks service-specific violations
func (s *CELValidationServer) analyzeServiceViolations(ctx context.Context, clientset *kubernetes.Clientset, expression string, k8sInput *celv1.KubernetesInput) []string {
	var violations []string

	services, err := clientset.CoreV1().Services(k8sInput.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return []string{fmt.Sprintf("Failed to list services: %v", err)}
	}

	// Check for common service patterns
	if strings.Contains(expression, "selector") {
		for _, svc := range services.Items {
			if len(svc.Spec.Selector) == 0 {
				violations = append(violations, fmt.Sprintf("Service '%s/%s' has no selector defined",
					svc.Namespace, svc.Name))
			}
		}
	}

	return violations
}

// analyzeGenericViolations provides generic analysis for unrecognized patterns
func (s *CELValidationServer) analyzeGenericViolations(ctx context.Context, clientset *kubernetes.Clientset, expression string, input *celv1.RuleInput) []string {
	k8sInput, ok := input.InputType.(*celv1.RuleInput_Kubernetes)
	if !ok {
		return []string{"Non-Kubernetes input type not supported for violation analysis"}
	}

	// Provide more specific violation information
	resourceType := k8sInput.Kubernetes.Resource
	apiVersion := k8sInput.Kubernetes.Version
	if k8sInput.Kubernetes.Group != "" {
		apiVersion = k8sInput.Kubernetes.Group + "/" + k8sInput.Kubernetes.Version
	}

	return []string{fmt.Sprintf("CEL expression '%s' failed for %s %s.\nPlease check that the resource has the expected structure and fields referenced in the expression exist.", expression, apiVersion, resourceType)}
}

// createK8sClient creates a Kubernetes client
func (s *CELValidationServer) createK8sClient() (*kubernetes.Clientset, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			homeDir, _ := os.UserHomeDir()
			kubeconfig = filepath.Join(homeDir, ".kube", "config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	return kubernetes.NewForConfig(config)
}

// Initialize server
func NewCELValidationServer() (*CELValidationServer, error) {
	// Setup Kubernetes configuration
	kubeconfigPath := getKubeconfigPath()
	if kubeconfigPath == "" {
		log.Println("Warning: No kubeconfig found, running in mock mode")
		// Return a mock server for development
		ruleStore, err := NewFileRuleStore("./rules-library")
		if err != nil {
			return nil, fmt.Errorf("failed to create rule store: %v", err)
		}
		return &CELValidationServer{
			ruleStore: ruleStore,
		}, nil
	}

	// Create rest config
	restConfig, err := createKubeConfig(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes config: %v", err)
	}

	// Create Kubernetes clients
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %v", err)
	}

	runtimeClient, err := runtimeclient.New(restConfig, runtimeclient.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime client: %v", err)
	}

	// Create composite fetcher
	compositeFetcher := fetchers.NewCompositeFetcherBuilder().
		WithKubernetes(runtimeClient, clientset).
		WithFilesystem("").
		Build()

	// Create scanner
	scanner := celscanner.NewScanner(compositeFetcher, &ConnectLogger{})

	// Create rule store
	ruleStore, err := NewFileRuleStore("./rules-library")
	if err != nil {
		return nil, fmt.Errorf("failed to create rule store: %v", err)
	}

	return &CELValidationServer{
		scanner:       scanner,
		kubeClient:    clientset,
		runtimeClient: runtimeClient,
		dynamicClient: dynamicClient,
		jobTrackers:   make(map[string]*NodeJobTracker),
		ruleStore:     ruleStore,
	}, nil
}

// Utility functions
func getKubeconfigPath() string {
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return kubeconfig
	}
	if home := homedir.HomeDir(); home != "" {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}

func createKubeConfig(kubeconfigPath string) (*rest.Config, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}
	return config.GetConfig()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Logger implementation
type ConnectLogger struct{}

func (l *ConnectLogger) Debug(msg string, args ...interface{}) {
	if os.Getenv("DEBUG") == "true" {
		log.Printf("[DEBUG] "+msg, args...)
	}
}

func (l *ConnectLogger) Info(msg string, args ...interface{}) {
	log.Printf("[INFO] "+msg, args...)
}

func (l *ConnectLogger) Warn(msg string, args ...interface{}) {
	log.Printf("[WARN] "+msg, args...)
}

func (l *ConnectLogger) Error(msg string, args ...interface{}) {
	log.Printf("[ERROR] "+msg, args...)
}

func main() {
	server, err := NewCELValidationServer()
	if err != nil {
		log.Fatal("Failed to initialize server:", err)
	}

	// Create gorilla/mux router
	reflector := grpcreflect.NewStaticReflector(
		"cel.v1.CELValidationService",
	)

	mux := http.NewServeMux()
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))
	mux.Handle(grpcreflect.NewHandlerV1(reflector))

	// Create Connect handler
	path, handler := celv1connect.NewCELValidationServiceHandler(server)
	log.Printf("Registering handler at path: %s", path)
	mux.Handle(path, handler)

	// Check if MCP is enabled
	if mcpEnabled := os.Getenv("ENABLE_MCP"); mcpEnabled == "true" {
		log.Println("MCP support enabled")

	}

	mcpServer, err := mcp.NewMCPServer(server)
	if err != nil {
		log.Fatalf("Failed to create MCP server: %v", err)
	}
	mcpHandler := serverv1.NewStreamableHTTPServer(mcpServer.GetServer())
	mux.Handle("/mcp", mcpHandler)

	// Add debug handler to log all requests
	debugHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Incoming request: %s %s", r.Method, r.URL.Path)
		mux.ServeHTTP(w, r)
	})

	// Add CORS middleware for web clients
	corsHandler := withCORS(debugHandler)

	port := ":8349"
	if p := os.Getenv("PORT"); p != "" {
		port = ":" + p
	}
	log.Printf("streamable-http server is available at http://localhost%s/mcp", port)

	log.Printf("CEL Validation Server listening on %s", port)
	log.Printf("Connect RPC endpoint: http://localhost%s%s", port, path)

	http.ListenAndServe(
		port,
		// Use h2c so we can serve HTTP/2 without TLS
		h2c.NewHandler(corsHandler, &http2.Server{}),
	)
}

// CORS middleware
func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization, Connect-Protocol-Version")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			return
		}

		h.ServeHTTP(w, r)
	})
}
