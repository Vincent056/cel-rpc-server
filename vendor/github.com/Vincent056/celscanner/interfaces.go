/*
Copyright © 2024 Red Hat Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package celscanner

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CelRule defines what's needed for CEL expression evaluation
type CelRule interface {
	// Identifier returns a unique identifier for this rule
	Identifier() string

	// Expression returns the CEL expression to evaluate
	Expression() string

	// Inputs returns the list of inputs needed for evaluation
	Inputs() []Input

	// Metadata returns optional rule metadata for compliance reporting
	Metadata() *RuleMetadata
}

// ScanEnvironment contains information about the environment where the scan is running
type ScanEnvironment struct {
	// TODO: Add environment information
}

// RuleMetadata contains metadata information for a rule
type RuleMetadata struct {
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Extensions  map[string]interface{} `json:"extensions,omitempty"`
}

// CheckResultMetadata contains metadata information for a check result
type CheckResultMetadata struct {
	Environment ScanEnvironment        `json:"environment,omitempty"`
	Extensions  map[string]interface{} `json:"extensions,omitempty"`
}

// Input defines a generic input that a CEL rule needs
type Input interface {
	// Name returns the name to bind this input to in the CEL context
	Name() string

	// Type returns the type of input (kubernetes, file, system, etc.)
	Type() InputType

	// Spec returns the input specification
	Spec() InputSpec
}

// InputType represents the different types of inputs supported
type InputType string

const (
	// InputTypeKubernetes represents Kubernetes resources
	InputTypeKubernetes InputType = "kubernetes"

	// InputTypeFile represents file system inputs
	InputTypeFile InputType = "file"

	// InputTypeSystem represents system service/process inputs
	InputTypeSystem InputType = "system"

	// InputTypeHTTP represents HTTP API inputs
	InputTypeHTTP InputType = "http"

	// InputTypeDatabase represents database inputs
	InputTypeDatabase InputType = "database"
)

// InputSpec is a generic interface for input specifications
type InputSpec interface {
	// Validate checks if the input specification is valid
	Validate() error
}

// KubernetesInputSpec specifies a Kubernetes resource input
type KubernetesInputSpec interface {
	InputSpec

	// ApiGroup returns the API group (e.g., "apps", "")
	ApiGroup() string

	// Version returns the API version (e.g., "v1", "v1beta1")
	Version() string

	// ResourceType returns the resource type (e.g., "pods", "configmaps")
	ResourceType() string

	// Namespace returns the namespace to search in (empty for cluster-scoped)
	Namespace() string

	// Name returns the specific resource name (empty for all resources)
	Name() string
}

// FileInputSpec specifies a file system input
type FileInputSpec interface {
	InputSpec

	// Path returns the file or directory path
	Path() string

	// Format returns the expected file format (json, yaml, text, etc.)
	Format() string

	// Recursive indicates if directory traversal should be recursive
	Recursive() bool

	// CheckPermissions indicates if file permissions should be included
	CheckPermissions() bool
}

// SystemInputSpec specifies a system service/process input
type SystemInputSpec interface {
	InputSpec

	// ServiceName returns the system service name
	ServiceName() string

	// Command returns the command to execute (alternative to service)
	Command() string

	// Args returns command arguments
	Args() []string
}

// HTTPInputSpec specifies an HTTP API input
type HTTPInputSpec interface {
	InputSpec

	// URL returns the HTTP endpoint URL
	URL() string

	// Method returns the HTTP method (GET, POST, etc.)
	Method() string

	// Headers returns HTTP headers
	Headers() map[string]string

	// Body returns the request body
	Body() []byte
}

// CelVariable defines a variable available in CEL expressions
type CelVariable interface {
	// Name returns the variable name
	Name() string

	// Namespace returns the namespace context
	Namespace() string

	// Value returns the variable value
	Value() string

	// GroupVersionKind returns the Kubernetes GVK for this variable
	GroupVersionKind() schema.GroupVersionKind
}

// InputFetcher retrieves data for different input types
type InputFetcher interface {
	// FetchInputs retrieves data for the specified inputs
	FetchInputs(inputs []Input, variables []CelVariable) (map[string]interface{}, error)

	// SupportsInputType returns whether this fetcher supports the given input type
	SupportsInputType(inputType InputType) bool
}

// ScanLogger handles logging during CEL evaluation
type ScanLogger interface {
	// Debug logs debug information
	Debug(msg string, args ...interface{})

	// Info logs informational messages
	Info(msg string, args ...interface{})

	// Error logs error messages
	Error(msg string, args ...interface{})
}

// ScanResult represents the result of evaluating a CEL rule
type ScanResult struct {
	// RuleID is the identifier of the rule that was evaluated
	RuleID string

	// Status indicates the result of the evaluation
	Status ScanStatus

	// Message provides additional context about the result
	Message string

	// Details contains any additional result data
	Details map[string]interface{}
}

// ScanStatus represents the possible outcomes of a CEL rule evaluation
type ScanStatus string

const (
	// StatusPass indicates the rule evaluation passed
	StatusPass ScanStatus = "PASS"

	// StatusFail indicates the rule evaluation failed
	StatusFail ScanStatus = "FAIL"

	// StatusError indicates an error occurred during evaluation
	StatusError ScanStatus = "ERROR"

	// StatusSkip indicates the rule was skipped
	StatusSkip ScanStatus = "SKIP"
)

// ===== IMPLEMENTATION TYPES =====

// RuleImpl provides a complete implementation of CelRule with rich metadata support
type RuleImpl struct {
	ID           string        `json:"id"`
	CelExpr      string        `json:"expression"`
	RuleInputs   []Input       `json:"inputs"`
	RuleMetadata *RuleMetadata `json:"metadata,omitempty"`
}

// Identifier returns the rule ID
func (r *RuleImpl) Identifier() string { return r.ID }

// Expression returns the CEL expression
func (r *RuleImpl) Expression() string { return r.CelExpr }

// Inputs returns the rule inputs
func (r *RuleImpl) Inputs() []Input { return r.RuleInputs }

// Metadata returns the rule metadata
func (r *RuleImpl) Metadata() *RuleMetadata { return r.RuleMetadata }

// InputImpl provides a concrete implementation of the Input interface
type InputImpl struct {
	InputName string    `json:"name"`
	InputType InputType `json:"type"`
	InputSpec InputSpec `json:"spec"`
}

func (i *InputImpl) Name() string    { return i.InputName }
func (i *InputImpl) Type() InputType { return i.InputType }
func (i *InputImpl) Spec() InputSpec { return i.InputSpec }

// KubernetesInput provides a concrete implementation of KubernetesInputSpec
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

// FileInput provides a concrete implementation of FileInputSpec
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

// SystemInput provides a concrete implementation of SystemInputSpec
type SystemInput struct {
	Service string   `json:"service,omitempty"`
	Cmd     string   `json:"command,omitempty"`
	CmdArgs []string `json:"args,omitempty"`
}

func (s *SystemInput) ServiceName() string { return s.Service }
func (s *SystemInput) Command() string     { return s.Cmd }
func (s *SystemInput) Args() []string      { return s.CmdArgs }
func (s *SystemInput) Validate() error     { return nil }

// HTTPInput provides a concrete implementation of HTTPInputSpec
type HTTPInput struct {
	Endpoint    string            `json:"url"`
	HTTPMethod  string            `json:"method,omitempty"`
	HTTPHeaders map[string]string `json:"headers,omitempty"`
	HTTPBody    []byte            `json:"body,omitempty"`
}

func (s *HTTPInput) URL() string                { return s.Endpoint }
func (s *HTTPInput) Method() string             { return s.HTTPMethod }
func (s *HTTPInput) Headers() map[string]string { return s.HTTPHeaders }
func (s *HTTPInput) Body() []byte               { return s.HTTPBody }
func (s *HTTPInput) Validate() error            { return nil }

// ===== CONVENIENCE CONSTRUCTORS =====

// NewRule creates a new CEL rule with optional metadata
func NewRule(id, expression string, inputs []Input) CelRule {
	return &RuleImpl{
		ID:         id,
		CelExpr:    expression,
		RuleInputs: inputs,
	}
}

// NewRuleWithMetadata creates a new CEL rule with metadata
func NewRuleWithMetadata(id, expression string, inputs []Input, metadata *RuleMetadata) CelRule {
	return &RuleImpl{
		ID:           id,
		CelExpr:      expression,
		RuleInputs:   inputs,
		RuleMetadata: metadata,
	}
}

// NewKubernetesInput creates a Kubernetes resource input
func NewKubernetesInput(name, group, version, resourceType, namespace, resourceName string) Input {
	return &InputImpl{
		InputName: name,
		InputType: InputTypeKubernetes,
		InputSpec: &KubernetesInput{
			Group:   group,
			Ver:     version,
			ResType: resourceType,
			Ns:      namespace,
			ResName: resourceName,
		},
	}
}

// NewFileInput creates a file system input
func NewFileInput(name, path, format string, recursive bool, checkPermissions bool) Input {
	return &InputImpl{
		InputName: name,
		InputType: InputTypeFile,
		InputSpec: &FileInput{
			FilePath:    path,
			FileFormat:  format,
			IsRecursive: recursive,
			CheckPerms:  checkPermissions,
		},
	}
}

// NewSystemInput creates a system service/process input
func NewSystemInput(name, service, command string, args []string) Input {
	return &InputImpl{
		InputName: name,
		InputType: InputTypeSystem,
		InputSpec: &SystemInput{
			Service: service,
			Cmd:     command,
			CmdArgs: args,
		},
	}
}

// NewHTTPInput creates an HTTP API input
func NewHTTPInput(name, url, method string, headers map[string]string, body []byte) Input {
	return &InputImpl{
		InputName: name,
		InputType: InputTypeHTTP,
		InputSpec: &HTTPInput{
			Endpoint:    url,
			HTTPMethod:  method,
			HTTPHeaders: headers,
			HTTPBody:    body,
		},
	}
}

// ===== BUILDER PATTERN =====

// RuleBuilder provides a fluent API for building rules
type RuleBuilder struct {
	rule *RuleImpl
}

// NewRuleBuilder creates a new rule builder with just the ID
func NewRuleBuilder(id string) *RuleBuilder {
	return &RuleBuilder{
		rule: &RuleImpl{
			ID:         id,
			CelExpr:    "", // Expression set later
			RuleInputs: make([]Input, 0),
		},
	}
}

// WithInput adds an input to the rule
func (b *RuleBuilder) WithInput(input Input) *RuleBuilder {
	b.rule.RuleInputs = append(b.rule.RuleInputs, input)
	return b
}

// WithKubernetesInput adds a Kubernetes input to the rule
func (b *RuleBuilder) WithKubernetesInput(name, group, version, resourceType, namespace, resourceName string) *RuleBuilder {
	input := NewKubernetesInput(name, group, version, resourceType, namespace, resourceName)
	return b.WithInput(input)
}

// WithFileInput adds a file input to the rule
func (b *RuleBuilder) WithFileInput(name, path, format string, recursive, checkPermissions bool) *RuleBuilder {
	input := NewFileInput(name, path, format, recursive, checkPermissions)
	return b.WithInput(input)
}

// WithSystemInput adds a system input to the rule
func (b *RuleBuilder) WithSystemInput(name, service, command string, args []string) *RuleBuilder {
	input := NewSystemInput(name, service, command, args)
	return b.WithInput(input)
}

// WithHTTPInput adds an HTTP input to the rule
func (b *RuleBuilder) WithHTTPInput(name, url, method string, headers map[string]string, body []byte) *RuleBuilder {
	input := NewHTTPInput(name, url, method, headers, body)
	return b.WithInput(input)
}

// SetExpression sets the CEL expression and validates it against available inputs
func (b *RuleBuilder) SetExpression(expression string) *RuleBuilder {
	b.rule.CelExpr = expression
	return b
}

// WithMetadata sets the rule metadata
func (b *RuleBuilder) WithMetadata(metadata *RuleMetadata) *RuleBuilder {
	b.rule.RuleMetadata = metadata
	return b
}

// WithName sets the rule name in metadata
func (b *RuleBuilder) WithName(name string) *RuleBuilder {
	if b.rule.RuleMetadata == nil {
		b.rule.RuleMetadata = &RuleMetadata{}
	}
	b.rule.RuleMetadata.Name = name
	return b
}

// WithDescription sets the rule description in metadata
func (b *RuleBuilder) WithDescription(description string) *RuleBuilder {
	if b.rule.RuleMetadata == nil {
		b.rule.RuleMetadata = &RuleMetadata{}
	}
	b.rule.RuleMetadata.Description = description
	return b
}

// WithExtension adds an extension to the rule metadata
func (b *RuleBuilder) WithExtension(key string, value interface{}) *RuleBuilder {
	if b.rule.RuleMetadata == nil {
		b.rule.RuleMetadata = &RuleMetadata{}
	}
	if b.rule.RuleMetadata.Extensions == nil {
		b.rule.RuleMetadata.Extensions = make(map[string]interface{})
	}
	b.rule.RuleMetadata.Extensions[key] = value
	return b
}

// Build returns the completed rule with validation
func (b *RuleBuilder) Build() (CelRule, error) {
	// Validate that we have essential components
	if b.rule.ID == "" {
		return nil, fmt.Errorf("Rule ID is required")
	}
	if b.rule.CelExpr == "" {
		return nil, fmt.Errorf("Rule expression is required")
	}
	if len(b.rule.RuleInputs) == 0 {
		return nil, fmt.Errorf("At least one input is required")
	}

	// TODO: Add expression validation against input names
	// This could include:
	// 1. Parse the CEL expression
	// 2. Extract variable references
	// 3. Ensure all referenced variables have corresponding inputs

	return b.rule, nil
}

// GetAvailableInputNames returns the names of all defined inputs (useful for expression building)
func (b *RuleBuilder) GetAvailableInputNames() []string {
	names := make([]string, len(b.rule.RuleInputs))
	for i, input := range b.rule.RuleInputs {
		names[i] = input.Name()
	}
	return names
}

// ===== USAGE EXAMPLES =====

// Example usage with new pattern:
//
// // Simple rule - inputs first, expression last
// rule := NewRuleBuilder("pods-exist").
//     WithKubernetesInput("pods", "", "v1", "pods", "", "").
//     SetExpression("size(pods) > 0").
//     WithName("Pod Existence Check").
//     Build()
//
// // Complex rule with multiple inputs
// rule := NewRuleBuilder("security-check").
//     WithKubernetesInput("pods", "", "v1", "pods", "", "").
//     WithFileInput("policy", "/etc/security/policy.yaml", "yaml", false, false).
//     WithSystemInput("audit", "", "auditctl -l", []string{}).
//     SetExpression("pods.all(pod, has(pod.spec.securityContext)) && policy.strictMode && size(audit) > 0").
//     WithName("Security Compliance Check").
//     WithDescription("Comprehensive security validation").
//     WithSeverity("CRITICAL").
//     WithLabel("category", "security").
//     WithAnnotation("owner", "security-team").
//     Build()
//
// // Performance-focused rule
// rule := NewRuleBuilder("performance-check").
//     WithFileInput("config", "/etc/app/config.yaml", "yaml", false, false).
//     WithKubernetesInput("nodes", "", "v1", "nodes", "", "").
//     SetExpression("config.database.pool_size >= 10 && size(nodes.filter(n, n.status.capacity.cpu > '4')) >= 2").
//     WithName("Performance Requirements").
//     WithSeverity("MEDIUM").
//     Build()
