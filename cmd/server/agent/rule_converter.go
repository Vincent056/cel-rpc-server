package agent

import (
	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"

	"github.com/ComplianceAsCode/compliance-sdk/pkg/scanner"
)

// RuleConverter converts between protobuf and celscanner formats
type RuleConverter struct{}

// NewRuleConverter creates a new rule converter
func NewRuleConverter() *RuleConverter {
	return &RuleConverter{}
}

// ConvertToCelRule converts a protobuf GeneratedRule to scanner.CelRule
func (c *RuleConverter) ConvertToCelRule(protoRule *celv1.GeneratedRule, ruleID string) scanner.CelRule {
	// Convert protobuf inputs to scanner inputs
	inputs := make([]scanner.Input, 0, len(protoRule.SuggestedInputs))

	for _, protoInput := range protoRule.SuggestedInputs {
		switch inputType := protoInput.InputType.(type) {
		case *celv1.RuleInput_Kubernetes:
			k8s := inputType.Kubernetes
			input := scanner.NewKubernetesInput(
				protoInput.Name,
				k8s.Group,
				k8s.Version,
				k8s.Resource,
				k8s.Namespace,
				k8s.ResourceName,
			)
			inputs = append(inputs, input)

		case *celv1.RuleInput_File:
			file := inputType.File
			input := scanner.NewFileInput(
				protoInput.Name,
				file.Path,
				file.Format,
				file.Recursive,
				file.CheckPermissions,
			)
			inputs = append(inputs, input)
		}
	}

	// Use the scanner's built-in rule creation instead of custom implementation
	return scanner.NewCelRuleWithMetadata(
		ruleID,
		protoRule.Expression,
		inputs,
		&scanner.RuleMetadata{
			Description: protoRule.Explanation,
		},
	)
}

// ConvertFromCelRule converts a scanner.CelRule to protobuf format
func (c *RuleConverter) ConvertFromCelRule(rule scanner.CelRule) *celv1.GeneratedRule {
	protoRule := &celv1.GeneratedRule{
		Expression:  rule.Expression(),
		Explanation: "",
		Variables:   make([]string, 0),
	}

	// Extract metadata if available
	if metadata := rule.Metadata(); metadata != nil {
		protoRule.Explanation = metadata.Description
	}

	// Convert inputs
	protoInputs := make([]*celv1.RuleInput, 0, len(rule.Inputs()))
	variables := make([]string, 0)

	for _, input := range rule.Inputs() {
		protoInput := &celv1.RuleInput{
			Name: input.Name(),
		}
		variables = append(variables, input.Name())

		switch input.Type() {
		case scanner.InputTypeKubernetes:
			if k8sSpec, ok := input.Spec().(scanner.KubernetesInputSpec); ok {
				protoInput.InputType = &celv1.RuleInput_Kubernetes{
					Kubernetes: &celv1.KubernetesInput{
						Group:     k8sSpec.ApiGroup(),
						Version:   k8sSpec.Version(),
						Resource:  k8sSpec.ResourceType(),
						Namespace: k8sSpec.Namespace(),
					},
				}
			}

		case scanner.InputTypeFile:
			if fileSpec, ok := input.Spec().(scanner.FileInputSpec); ok {
				protoInput.InputType = &celv1.RuleInput_File{
					File: &celv1.FileInput{
						Path:             fileSpec.Path(),
						Format:           fileSpec.Format(),
						Recursive:        fileSpec.Recursive(),
						CheckPermissions: fileSpec.CheckPermissions(),
					},
				}
			}
		}

		protoInputs = append(protoInputs, protoInput)
	}

	protoRule.SuggestedInputs = protoInputs
	protoRule.Variables = variables

	return protoRule
}

// Example usage for creating a rule that checks pod security
func (c *RuleConverter) CreatePodSecurityRule() scanner.CelRule {
	inputs := []scanner.Input{
		scanner.NewKubernetesInput("pods", "", "v1", "pods", "default", ""),
	}

	metadata := &scanner.RuleMetadata{
		Description: "Ensure all pods run with non-root security context",
		Name:        "pod-security-check",
		Extensions: map[string]interface{}{
			"severity": "high",
			"category": "security",
			"tags":     "pod,security",
			"version":  "1.0.0",
		},
	}

	return scanner.NewCelRuleWithMetadata(
		"pod-security-check",
		"pods.items.all(p, has(p.spec.securityContext) && p.spec.securityContext.runAsNonRoot == true)",
		inputs,
		metadata,
	)
}

// Note: Custom rule and input implementations have been removed.
// The compliance-sdk now provides built-in implementations through
// scanner.NewCelRule(), scanner.NewKubernetesInput(), scanner.NewFileInput(), etc.
