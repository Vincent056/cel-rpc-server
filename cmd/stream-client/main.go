package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
	"github.com/Vincent056/cel-rpc-server/gen/cel/v1/celv1connect"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
)

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

	celClient := celv1connect.NewCELValidationServiceClient(
		client,
		"http://localhost:8090",
		connect.WithGRPCWeb(),
	)

	ctx := context.Background()

	// Test streaming validation
	fmt.Println("Starting streaming validation test...")
	fmt.Println("=====================================")

	stream := celClient.ValidateCELStream(ctx)

	// Start goroutine to receive messages
	done := make(chan bool)
	go func() {
		defer close(done)
		log.Println("Starting receive loop...")
		for {
			resp, err := stream.Receive()
			if err != nil {
				if err == io.EOF {
					log.Println("Stream closed by server")
				} else {
					log.Printf("Receive error: %v", err)
				}
				return
			}

			log.Printf("Received message: %T", resp.Response)

			switch msg := resp.Response.(type) {
			case *celv1.ValidateCELStreamResponse_NodeStatus:
				status := msg.NodeStatus
				fmt.Printf("[NODE STATUS] %s: %s - %s\n",
					status.NodeName, status.Status.String(), status.Message)

			case *celv1.ValidateCELStreamResponse_NodeResult:
				result := msg.NodeResult
				fmt.Printf("[RESULT] Node: %s, File: %s\n",
					result.NodeName, result.FilePath)
				fmt.Printf("         Status: %v\n", result.Result.Passed)
				fmt.Printf("         Details: %s\n\n", result.Result.Details)

			case *celv1.ValidateCELStreamResponse_Progress:
				progress := msg.Progress
				fmt.Printf("[PROGRESS] Total: %d, Completed: %d, Failed: %d, Pending: %d\n",
					progress.TotalNodes, progress.CompletedNodes,
					progress.FailedNodes, progress.PendingNodes)
				fmt.Println("           Node Statuses:")
				for node, status := range progress.NodeStatuses {
					fmt.Printf("           - %s: %s\n", node, status.Status.String())
				}
				fmt.Println()

			case *celv1.ValidateCELStreamResponse_Error:
				err := msg.Error
				fmt.Printf("[ERROR] %s - %s\n", err.Error, err.Details)
			}
		}
	}()

	// Send validation request for master nodes
	fmt.Println("\nSending validation request for master nodes...")
	err := stream.Send(&celv1.ValidateCELStreamRequest{
		Request: &celv1.ValidateCELStreamRequest_ValidationRequest{
			ValidationRequest: &celv1.ValidateCELRequest{
				Expression: "kubeletConfig.authentication.webhook.enabled == true",
				Inputs: []*celv1.RuleInput{
					{
						Name: "kubeletConfig",
						InputType: &celv1.RuleInput_File{
							File: &celv1.FileInput{
								Path:   "/etc/kubernetes/kubelet.conf",
								Format: "yaml",
								Target: &celv1.FileInput_NodeSelector{
									NodeSelector: &celv1.NodeSelector{
										Labels: map[string]string{
											"node-role.kubernetes.io/master": "",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	if err != nil {
		log.Fatalf("Failed to send request: %v", err)
	}

	// Wait a bit
	time.Sleep(2 * time.Second)

	// Send validation request for worker nodes
	fmt.Println("\nSending validation request for worker nodes...")
	err = stream.Send(&celv1.ValidateCELStreamRequest{
		Request: &celv1.ValidateCELStreamRequest_ValidationRequest{
			ValidationRequest: &celv1.ValidateCELRequest{
				Expression: "sshConfig.PermitRootLogin == \"no\"",
				Inputs: []*celv1.RuleInput{
					{
						Name: "sshConfig",
						InputType: &celv1.RuleInput_File{
							File: &celv1.FileInput{
								Path:   "/etc/ssh/sshd_config",
								Format: "text",
								Target: &celv1.FileInput_NodeSelector{
									NodeSelector: &celv1.NodeSelector{
										Labels: map[string]string{
											"node-role.kubernetes.io/worker": "",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	if err != nil {
		log.Fatalf("Failed to send request: %v", err)
	}

	// Wait for results
	time.Sleep(5 * time.Second)

	// Send cancel control message
	fmt.Println("\nSending cancel request for worker-1...")
	err = stream.Send(&celv1.ValidateCELStreamRequest{
		Request: &celv1.ValidateCELStreamRequest_Control{
			Control: &celv1.StreamControl{
				Action:   celv1.StreamControl_CANCEL,
				NodeName: "worker-1",
			},
		},
	})

	if err != nil {
		log.Printf("Failed to send control: %v", err)
	}

	// Wait a bit more
	time.Sleep(2 * time.Second)

	// Close stream
	fmt.Println("\nClosing stream...")
	stream.CloseRequest()

	// Wait for receive loop to finish
	<-done

	stream.CloseResponse()

	fmt.Println("Test completed!")
}
