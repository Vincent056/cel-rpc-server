package verification

import (
	"context"
	"testing"

	pb "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
)

func TestCELRuleVerifier_SingleInput(t *testing.T) {
	// Example: Pod security context rule with single input
	rule := &pb.CELRule{
		Id:          "pod-security-context",
		Name:        "Pod Security Context Check",
		Description: "Ensures all pods have security context defined",
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
				Id:          "tc1",
				Name:        "All pods have security context",
				Description: "Test with pods that have security context",
				TestData: map[string]string{
					"pods": `{
						"apiVersion": "v1",
						"kind": "List",
						"items": [
							{
								"apiVersion": "v1",
								"kind": "Pod",
								"metadata": {"name": "secure-pod"},
								"spec": {
									"securityContext": {"runAsUser": 1000},
									"containers": [{"name": "app", "image": "nginx"}]
								}
							}
						]
					}`,
				},
				ExpectedResult: true,
			},
			{
				Id:          "tc2",
				Name:        "Pod without security context",
				Description: "Test with pod missing security context",
				TestData: map[string]string{
					"pods": `{
						"apiVersion": "v1",
						"kind": "List",
						"items": [
							{
								"apiVersion": "v1",
								"kind": "Pod",
								"metadata": {"name": "insecure-pod"},
								"spec": {
									"containers": [{"name": "app", "image": "nginx"}]
								}
							}
						]
					}`,
				},
				ExpectedResult: false,
			},
			{
				Id:          "tc3",
				Name:        "Empty pod list",
				Description: "Test with no pods",
				TestData: map[string]string{
					"pods": `{"apiVersion": "v1", "kind": "List", "items": []}`,
				},
				ExpectedResult: true, // all(empty) is true
			},
		},
	}

	verifier := NewCELRuleVerifier()
	result, err := verifier.VerifyCELRule(context.Background(), rule)

	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	}

	if !result.OverallPassed {
		t.Errorf("Expected all test cases to pass")
		for _, tc := range result.TestCases {
			if !tc.Passed {
				t.Errorf("Test case %s failed: %s", tc.TestCaseName, tc.Error)
			}
		}
	}
}

func TestCELRuleVerifier_SingleInput_List(t *testing.T) {
	// Example: Pod security context rule with single input
	rule := &pb.CELRule{
		Id:          "pod-security-context",
		Name:        "Pod Security Context Check",
		Description: "Ensures all pods have security context defined",
		Expression:  "pods.items.all(pod, has(pod.specs.securityContext))",
		Inputs: []*pb.RuleInput{
			{
				Name: "pods",
				InputType: &pb.RuleInput_Kubernetes{
					Kubernetes: &pb.KubernetesInput{
						Group:    "",
						Version:  "v1",
						Resource: "pods/pod-a",
					},
				},
			},
		},
		TestCases: []*pb.RuleTestCase{
			{
				Id:          "tc1",
				Name:        "All pods have security context",
				Description: "Test with pods that have security context",
				TestData: map[string]string{
					"pods": `{
						"apiVersion": "v1",
						"kind": "List",
						"items": [
							{
								"apiVersion": "v1",
								"kind": "Pod",
								"metadata": {"name": "secure-pod"},
								"spec": {
									"securityContext": {"runAsUser": 1000},
									"containers": [{"name": "app", "image": "nginx"}]
								}
							}
						]
					}`,
				},
				ExpectedResult: true,
			},
			{
				Id:          "tc2",
				Name:        "Pod without security context",
				Description: "Test with pod missing security context",
				TestData: map[string]string{
					"pods": `{
						"apiVersion": "v1",
						"kind": "List",
						"items": [
							{
								"apiVersion": "v1",
								"kind": "Pod",
								"metadata": {"name": "insecure-pod"},
								"spec": {
									"containers": [{"name": "app", "image": "nginx"}]
								}
							}
						]
					}`,
				},
				ExpectedResult: false,
			},
			{
				Id:          "tc3",
				Name:        "Empty pod list",
				Description: "Test with no pods",
				TestData: map[string]string{
					"pods": `{"apiVersion": "v1", "kind": "List", "items": []}`,
				},
				ExpectedResult: true, // all(empty) is true
			},
		},
	}

	verifier := NewCELRuleVerifier()
	result, err := verifier.VerifyCELRule(context.Background(), rule)

	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	}

	if !result.OverallPassed {
		t.Errorf("Expected all test cases to pass")
		for _, tc := range result.TestCases {
			if !tc.Passed {
				t.Errorf("Test case %s failed: %s", tc.TestCaseName, tc.Error)
			}
		}
	}
}

func TestCELRuleVerifier_MultipleInputs(t *testing.T) {
	// Example: Namespace compliance rule with multiple inputs
	rule := &pb.CELRule{
		Id:          "namespace-network-policy",
		Name:        "Namespace Network Policy Compliance",
		Description: "Ensures all namespaces have network policies",
		Expression: `namespaces.items.all(ns, 
			networkpolicies.items.exists(np, np.metadata.namespace == ns.metadata.name)
		)`,
		Inputs: []*pb.RuleInput{
			{
				Name: "namespaces",
				InputType: &pb.RuleInput_Kubernetes{
					Kubernetes: &pb.KubernetesInput{
						Group:    "",
						Version:  "v1",
						Resource: "namespaces",
					},
				},
			},
			{
				Name: "networkpolicies",
				InputType: &pb.RuleInput_Kubernetes{
					Kubernetes: &pb.KubernetesInput{
						Group:    "networking.k8s.io",
						Version:  "v1",
						Resource: "networkpolicies",
					},
				},
			},
		},
		TestCases: []*pb.RuleTestCase{
			{
				Id:          "tc1",
				Name:        "All namespaces have network policies",
				Description: "Test with compliant namespaces",
				TestData: map[string]string{
					"namespaces": `{
						"apiVersion": "v1",
						"kind": "List",
						"items": [
							{
								"apiVersion": "v1",
								"kind": "Namespace",
								"metadata": {"name": "default"}
							},
							{
								"apiVersion": "v1",
								"kind": "Namespace",
								"metadata": {"name": "production"}
							}
						]
					}`,
					"networkpolicies": `{
						"apiVersion": "networking.k8s.io/v1",
						"kind": "List",
						"items": [
							{
								"apiVersion": "networking.k8s.io/v1",
								"kind": "NetworkPolicy",
								"metadata": {
									"name": "default-deny",
									"namespace": "default"
								},
								"spec": {
									"podSelector": {},
									"policyTypes": ["Ingress", "Egress"]
								}
							},
							{
								"apiVersion": "networking.k8s.io/v1",
								"kind": "NetworkPolicy",
								"metadata": {
									"name": "production-policy",
									"namespace": "production"
								},
								"spec": {
									"podSelector": {},
									"policyTypes": ["Ingress"]
								}
							}
						]
					}`,
				},
				ExpectedResult: true,
			},
			{
				Id:          "tc2",
				Name:        "Missing network policy",
				Description: "Test with namespace missing network policy",
				TestData: map[string]string{
					"namespaces": `{
						"apiVersion": "v1",
						"kind": "List",
						"items": [
							{
								"apiVersion": "v1",
								"kind": "Namespace",
								"metadata": {"name": "default"}
							},
							{
								"apiVersion": "v1",
								"kind": "Namespace",
								"metadata": {"name": "unprotected"}
							}
						]
					}`,
					"networkpolicies": `{
						"apiVersion": "networking.k8s.io/v1",
						"kind": "List",
						"items": [
							{
								"apiVersion": "networking.k8s.io/v1",
								"kind": "NetworkPolicy",
								"metadata": {
									"name": "default-deny",
									"namespace": "default"
								},
								"spec": {
									"podSelector": {},
									"policyTypes": ["Ingress"]
								}
							}
						]
					}`,
				},
				ExpectedResult: false,
			},
			{
				Id:          "tc3",
				Name:        "No network policies provided",
				Description: "Test with missing network policy data (defaults to empty)",
				TestData: map[string]string{
					"namespaces": `{
						"apiVersion": "v1",
						"kind": "List",
						"items": [
							{
								"apiVersion": "v1",
								"kind": "Namespace",
								"metadata": {"name": "default"}
							}
						]
					}`,
					// No networkpolicies data provided - will default to empty list
				},
				ExpectedResult: false,
			},
		},
	}

	verifier := NewCELRuleVerifier()
	result, err := verifier.VerifyCELRule(context.Background(), rule)

	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	}

	// Check expected results
	expectedResults := map[string]bool{
		"tc1": true,
		"tc2": true,
		"tc3": true,
	}

	for _, tc := range result.TestCases {
		if expected, ok := expectedResults[tc.TestCaseID]; ok {
			if tc.Passed != expected {
				t.Errorf("Test case %s: expected passed=%v, got %v (error: %s)",
					tc.TestCaseID, expected, tc.Passed, tc.Error)
			}
		}
	}
}

func TestCELRuleVerifier_ComplexRule(t *testing.T) {
	// Example: Complex deployment validation with multiple resources
	rule := &pb.CELRule{
		Id:          "deployment-config-validation",
		Name:        "Deployment Configuration Validation",
		Description: "Validates deployments have proper configuration",
		Expression: `
			deployments.items.all(dep, 
				dep.spec.replicas > 1 && 
				configmaps.items.exists(cm, 
					cm.metadata.name == dep.metadata.name + "-config" &&
					cm.metadata.namespace == dep.metadata.namespace
				) &&
				services.items.exists(svc,
					svc.metadata.namespace == dep.metadata.namespace &&
					svc.spec.selector.app == dep.metadata.labels.app
				)
			)
		`,
		Inputs: []*pb.RuleInput{
			{
				Name: "deployments",
				InputType: &pb.RuleInput_Kubernetes{
					Kubernetes: &pb.KubernetesInput{
						Group:    "apps",
						Version:  "v1",
						Resource: "deployments",
					},
				},
			},
			{
				Name: "configmaps",
				InputType: &pb.RuleInput_Kubernetes{
					Kubernetes: &pb.KubernetesInput{
						Group:    "",
						Version:  "v1",
						Resource: "configmaps",
					},
				},
			},
			{
				Name: "services",
				InputType: &pb.RuleInput_Kubernetes{
					Kubernetes: &pb.KubernetesInput{
						Group:    "",
						Version:  "v1",
						Resource: "services",
					},
				},
			},
		},
		TestCases: []*pb.RuleTestCase{
			{
				Id:          "tc1",
				Name:        "Valid deployment configuration",
				Description: "Deployment with proper replicas, configmap, and service",
				TestData: map[string]string{
					"deployments": `{
						"apiVersion": "apps/v1",
						"kind": "List",
						"items": [{
							"apiVersion": "apps/v1",
							"kind": "Deployment",
							"metadata": {
								"name": "web-app",
								"namespace": "production",
								"labels": {"app": "web"}
							},
							"spec": {
								"replicas": 3,
								"selector": {"matchLabels": {"app": "web"}},
								"template": {
									"metadata": {"labels": {"app": "web"}},
									"spec": {
										"containers": [{
											"name": "web",
											"image": "nginx:latest"
										}]
									}
								}
							}
						}]
					}`,
					"configmaps": `{
						"apiVersion": "v1",
						"kind": "List",
						"items": [{
							"apiVersion": "v1",
							"kind": "ConfigMap",
							"metadata": {
								"name": "web-app-config",
								"namespace": "production"
							},
							"data": {
								"config.yaml": "key: value"
							}
						}]
					}`,
					"services": `{
						"apiVersion": "v1",
						"kind": "List",
						"items": [{
							"apiVersion": "v1",
							"kind": "Service",
							"metadata": {
								"name": "web-service",
								"namespace": "production"
							},
							"spec": {
								"selector": {"app": "web"},
								"ports": [{"port": 80, "targetPort": 8080}]
							}
						}]
					}`,
				},
				ExpectedResult: true,
			},
			{
				Id:          "tc2",
				Name:        "Missing configmap",
				Description: "Deployment without required configmap",
				TestData: map[string]string{
					"deployments": `{
						"apiVersion": "apps/v1",
						"kind": "List",
						"items": [{
							"apiVersion": "apps/v1",
							"kind": "Deployment",
							"metadata": {
								"name": "web-app",
								"namespace": "production",
								"labels": {"app": "web"}
							},
							"spec": {
								"replicas": 3,
								"selector": {"matchLabels": {"app": "web"}}
							}
						}]
					}`,
					"configmaps": `{
						"apiVersion": "v1",
						"kind": "List",
						"items": []
					}`,
					"services": `{
						"apiVersion": "v1",
						"kind": "List",
						"items": [{
							"apiVersion": "v1",
							"kind": "Service",
							"metadata": {
								"name": "web-service",
								"namespace": "production"
							},
							"spec": {
								"selector": {"app": "web"},
								"ports": [{"port": 80}]
							}
						}]
					}`,
				},
				ExpectedResult: false,
			},
		},
	}

	verifier := NewCELRuleVerifier()
	result, err := verifier.VerifyCELRule(context.Background(), rule)

	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	}

	t.Logf("Rule: %s", result.RuleName)
	t.Logf("Overall: %v (Passed: %d/%d)", result.OverallPassed, result.PassedCount, result.TotalCount)

	for _, tc := range result.TestCases {
		t.Logf("  Test Case: %s - %v", tc.TestCaseName, tc.Passed)
		if tc.Error != "" {
			t.Logf("    Error: %s", tc.Error)
		}
	}
}
