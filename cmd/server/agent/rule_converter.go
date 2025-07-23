package agent

import (
	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"

	"github.com/Vincent056/celscanner"
)

// RuleConverter converts between protobuf and celscanner formats
type RuleConverter struct{}

// NewRuleConverter creates a new rule converter
func NewRuleConverter() *RuleConverter {
	return &RuleConverter{}
}

// ConvertToCelRule converts a protobuf GeneratedRule to celscanner.CelRule
func (c *RuleConverter) ConvertToCelRule(protoRule *celv1.GeneratedRule, ruleID string) celscanner.CelRule {
	// Convert protobuf inputs to celscanner inputs
	inputs := make([]celscanner.Input, 0, len(protoRule.SuggestedInputs))

	for _, protoInput := range protoRule.SuggestedInputs {
		input := &InputImpl{
			InputName: protoInput.Name, // This is the variable name used in CEL
		}

		switch inputType := protoInput.InputType.(type) {
		case *celv1.RuleInput_Kubernetes:
			input.InputType = celscanner.InputTypeKubernetes
			input.InputSpec = &KubernetesInput{
				Group:   inputType.Kubernetes.Group,
				Ver:     inputType.Kubernetes.Version,
				ResType: inputType.Kubernetes.Resource,
				Ns:      inputType.Kubernetes.Namespace,
				ResName: input.InputName,
			}

		case *celv1.RuleInput_File:
			input.InputType = celscanner.InputTypeFile
			input.InputSpec = &FileInput{
				FilePath:    inputType.File.Path,
				FileFormat:  inputType.File.Format,
				IsRecursive: inputType.File.Recursive,
				CheckPerms:  inputType.File.CheckPermissions,
			}
		}

		inputs = append(inputs, input)
	}

	// Create the rule implementation
	return &RuleImpl{
		ID:         ruleID,
		CelExpr:    protoRule.Expression,
		RuleInputs: inputs,
		RuleMetadata: &celscanner.RuleMetadata{
			Description: protoRule.Explanation,
		},
	}
}

// ConvertFromCelRule converts a celscanner.CelRule to protobuf format
func (c *RuleConverter) ConvertFromCelRule(rule celscanner.CelRule) *celv1.GeneratedRule {
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
		case celscanner.InputTypeKubernetes:
			if k8sSpec, ok := input.Spec().(celscanner.KubernetesInputSpec); ok {
				protoInput.InputType = &celv1.RuleInput_Kubernetes{
					Kubernetes: &celv1.KubernetesInput{
						Group:     k8sSpec.ApiGroup(),
						Version:   k8sSpec.Version(),
						Resource:  k8sSpec.ResourceType(),
						Namespace: k8sSpec.Namespace(),
					},
				}
			}

		case celscanner.InputTypeFile:
			if fileSpec, ok := input.Spec().(celscanner.FileInputSpec); ok {
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
func (c *RuleConverter) CreatePodSecurityRule() celscanner.CelRule {
	return &RuleImpl{
		ID:      "pod-security-check",
		CelExpr: "pods.items.all(p, has(p.spec.securityContext) && p.spec.securityContext.runAsNonRoot == true)",
		RuleInputs: []celscanner.Input{
			&InputImpl{
				InputName: "pods", // Variable name used in the CEL expression
				InputType: celscanner.InputTypeKubernetes,
				InputSpec: &KubernetesInput{
					Group:   "",
					Ver:     "v1",
					ResType: "pods",
					Ns:      "default",
				},
			},
		},
		RuleMetadata: &celscanner.RuleMetadata{
			Description: "Ensure all pods run with non-root security context",
			Name:        "pod-security-check",
			Extensions: map[string]interface{}{
				"severity": "high",
				"category": "security",
				"tags":     "pod,security",
				"version":  "1.0.0",
			},
		},
	}
}

// Implementations for the celscanner interfaces
// These match the ones from your interfaces.go file

type RuleImpl struct {
	ID           string                   `json:"id"`
	CelExpr      string                   `json:"expression"`
	RuleInputs   []celscanner.Input       `json:"inputs"`
	RuleMetadata *celscanner.RuleMetadata `json:"metadata,omitempty"`
}

func (r *RuleImpl) Identifier() string                 { return r.ID }
func (r *RuleImpl) Expression() string                 { return r.CelExpr }
func (r *RuleImpl) Inputs() []celscanner.Input         { return r.RuleInputs }
func (r *RuleImpl) Metadata() *celscanner.RuleMetadata { return r.RuleMetadata }

type InputImpl struct {
	InputName string               `json:"name"`
	InputType celscanner.InputType `json:"type"`
	InputSpec celscanner.InputSpec `json:"spec"`
}

func (i *InputImpl) Name() string               { return i.InputName }
func (i *InputImpl) Type() celscanner.InputType { return i.InputType }
func (i *InputImpl) Spec() celscanner.InputSpec { return i.InputSpec }

type KubernetesInput struct {
	Group   string `json:"group"`
	Ver     string `json:"version"`
	ResType string `json:"resourceType"`
	Ns      string `json:"namespace,omitempty"`
	ResName string `json:"name,omitempty"`
}

func (s *KubernetesInput) ApiGroup() string     { return s.Group }
func (s *KubernetesInput) Version() string      { return s.Ver }
func (s *KubernetesInput) ResourceType() string { return s.ResType }
func (s *KubernetesInput) Namespace() string    { return s.Ns }
func (s *KubernetesInput) Name() string         { return s.ResName }
func (s *KubernetesInput) Validate() error      { return nil }

type FileInput struct {
	FilePath    string `json:"path"`
	FileFormat  string `json:"format,omitempty"`
	IsRecursive bool   `json:"recursive,omitempty"`
	CheckPerms  bool   `json:"checkPermissions,omitempty"`
}

func (s *FileInput) Path() string           { return s.FilePath }
func (s *FileInput) Format() string         { return s.FileFormat }
func (s *FileInput) Recursive() bool        { return s.IsRecursive }
func (s *FileInput) CheckPermissions() bool { return s.CheckPerms }
func (s *FileInput) Validate() error        { return nil }
