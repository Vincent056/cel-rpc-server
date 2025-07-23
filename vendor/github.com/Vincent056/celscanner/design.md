# CEL Scanner Package Design Documentation

## Overview

The CEL Scanner package is a comprehensive compliance scanning framework that uses Google's CEL (Common Expression Language) to evaluate compliance rules against various data sources including Kubernetes resources, files, system commands, and HTTP APIs. This document provides an in-depth analysis of the package architecture, components, design patterns, and user stories.

### Key Features

- **Multi-Source Data Integration**: Kubernetes, Filesystem, System, HTTP, and Database inputs
- **Flexible Rule Engine**: CEL-based expressions with custom functions
- **Comprehensive Testing**: Built-in unit testing framework for rule validation
- **Security Focus**: Container-native scanning with security-first design
- **Production Ready**: Extensive error handling, logging, and performance optimization
- **Extensible Architecture**: Plugin-based fetcher system for custom data sources

## Architecture Overview

### Core Components

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                             CEL Scanner                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────┐ │
│  │   Core Scanner  │  │   Interfaces    │  │  Input Fetchers │  │ Testing │ │
│  │   (scanner.go)  │  │ (interfaces.go) │  │ (fetchers/ dir) │  │Framework│ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  └─────────┘ │
│                                                                             │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌─────────┐ │
│  │   Examples      │  │   Test Data     │  │   Benchmarks    │  │User     │ │
│  │ (examples/ dir) │  │ (testdata/ dir) │  │(benchmark_test) │  │Stories  │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  └─────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Fetcher Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Composite Fetcher                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐ ┌─────────────────┐  │
│  │  Kubernetes   │ │  Filesystem   │ │    System     │ │      HTTP       │  │
│  │   Fetcher     │ │   Fetcher     │ │   Fetcher     │ │    Fetcher      │  │
│  │ ┌───────────┐ │ │ ┌───────────┐ │ │ ┌───────────┐ │ │ ┌─────────────┐ │  │
│  │ │ API Client│ │ │ │JSON/YAML  │ │ │ │ Commands  │ │ │ │ REST APIs   │ │  │
│  │ │Discovery  │ │ │ │Parsing    │ │ │ │ Services  │ │ │ │ Headers     │ │  │
│  │ │Caching    │ │ │ │Directory  │ │ │ │ Security  │ │ │ │ Auth        │ │  │
│  │ └───────────┘ │ │ │Traversal  │ │ │ │ Timeouts  │ │ │ │ Timeouts    │ │  │
│  └───────────────┘ │ └───────────┘ │ │ └───────────┘ │ │ │ Retries     │ │  │
│                    └───────────────┘ └───────────────┘ │ └─────────────┘ │  │
│                                                        └─────────────────┘  │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                      Custom Fetcher Registry                            │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │ │
│  │  │  Database   │  │  Message    │  │    LDAP     │  │   User-Defined  │ │ │
│  │  │   Fetcher   │  │    Queue    │  │   Fetcher   │  │    Fetchers     │ │ │
│  │  │  (Future)   │  │  (Future)   │  │  (Future)   │  │                 │ │ │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────┘ │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Component Analysis

### 1. Core Scanner (`scanner.go`)

**Purpose**: Main orchestrator for compliance scanning using CEL expressions.

**Key Responsibilities**:
- Rule compilation and execution
- Resource fetching coordination
- CEL environment management
- Result generation and error handling

**Key Types**:
```go
type Scanner struct {
    resourceFetcher ResourceFetcher
    logger          Logger
}

type ScanConfig struct {
    Rules              []Rule
    Variables          []Variable
    ApiResourcePath    string
    EnableDebugLogging bool
}

type CheckResult struct {
    ID           string
    Name         string
    Status       CheckResultStatus
    Warnings     []string
    Annotations  map[string]string
    // ... other fields
}
```

**Design Patterns**:
- **Dependency Injection**: Scanner accepts ResourceFetcher and Logger interfaces
- **Strategy Pattern**: Different resource fetching strategies (API vs file-based)
- **Template Method**: Standardized scanning process with customizable steps

### 2. Interface Definitions (`interfaces.go`)

**Purpose**: Modern, extensible interface definitions for the CEL scanning framework.

**Key Interfaces**:
```go
type CelRule interface {
    Identifier() string
    Expression() string
    Inputs() []Input
}

type Input interface {
    Name() string
    Type() InputType
    Spec() InputSpec
}

type InputFetcher interface {
    FetchInputs(inputs []Input, variables []CelVariable) (map[string]interface{}, error)
    SupportsInputType(inputType InputType) bool
}
```

**Supported Input Types**:
- `InputTypeKubernetes`: Kubernetes resources
- `InputTypeFile`: File system resources
- `InputTypeSystem`: System commands and services
- `InputTypeHTTP`: HTTP API endpoints
- `InputTypeDatabase`: Database queries

**Design Patterns**:
- **Interface Segregation**: Separate interfaces for different concerns
- **Factory Pattern**: Convenience constructors for common use cases
- **Adapter Pattern**: Bridge between old and new interface definitions

### 3. Input Fetchers (`inputs/` directory)

#### 3.1 Kubernetes Fetcher (`kubernetes.go`)

**Purpose**: Retrieves Kubernetes resources via API or pre-fetched files.

**Key Features**:
- Dynamic resource discovery using API discovery client
- Caching mechanisms for performance
- Support for both live API and file-based operations
- Custom resource mapping configuration

**Architecture**:
```go
type KubernetesFetcher struct {
    client          runtimeclient.Client
    clientset       kubernetes.Interface
    discoveryClient discovery.DiscoveryInterface
    apiResourcePath string
    config          *ResourceMappingConfig
}
```

**Design Patterns**:
- **Cache Pattern**: Resource discovery caching
- **Strategy Pattern**: API vs file-based fetching
- **Builder Pattern**: Configuration with method chaining

#### 3.2 Filesystem Fetcher (`filesystem.go`)

**Purpose**: Reads and parses files and directories with various formats.

**Key Features**:
- Support for JSON, YAML, and text formats
- Recursive directory traversal
- File permission and metadata collection
- Format auto-detection

**Architecture**:
```go
type FilesystemFetcher struct {
    basePath string
}
```

**Design Patterns**:
- **Template Method**: Standardized file processing workflow
- **Strategy Pattern**: Different parsing strategies by format
- **Visitor Pattern**: Directory traversal with custom actions

#### 3.3 System Fetcher (`system.go`)

**Purpose**: Executes system commands and checks service status.

**Key Features**:
- Systemd service status checking
- Secure command execution with allowlists
- Timeout management
- Structured result format

**Architecture**:
```go
type SystemFetcher struct {
    commandTimeout         time.Duration
    allowArbitraryCommands bool
}

type SystemResult struct {
    Status   string
    Output   string
    ExitCode int
    Success  bool
    Metadata map[string]interface{}
}
```

**Design Patterns**:
- **Command Pattern**: Encapsulated command execution
- **Security Pattern**: Allowlist-based command filtering
- **Timeout Pattern**: Configurable execution timeouts

#### 3.4 HTTP Fetcher (`http.go`)

**Purpose**: Performs HTTP requests to REST APIs and web services for security scanning.

**Key Features**:
- Support for all HTTP methods (GET, POST, PUT, DELETE, OPTIONS, etc.)
- Custom headers and authentication (Bearer tokens, Basic auth)
- Request/response timeout management
- Automatic retry mechanisms with exponential backoff
- JSON and text response parsing
- Response time measurement for performance monitoring
- SSL/TLS certificate validation
- Redirect handling configuration

**Architecture**:
```go
type HTTPFetcher struct {
    client          *http.Client
    defaultTimeout  time.Duration
    followRedirects bool
    maxRetries      int
}

type HTTPResult struct {
    StatusCode   int                    // HTTP status code
    Headers      map[string][]string    // Response headers
    Body         interface{}            // Parsed JSON or raw text
    RawBody      string                 // Raw response body
    ResponseTime int64                  // Response time in milliseconds
    Success      bool                   // 2xx status code indicator
    Error        string                 // Error message if any
    Metadata     map[string]interface{} // Request metadata
}
```

**Use Cases**:
- REST API security validation
- Microservices health checking
- Security header verification
- Authentication/authorization testing
- CORS policy validation
- Rate limiting detection
- SSL/TLS configuration checks

**Design Patterns**:
- **Strategy Pattern**: Different authentication strategies
- **Retry Pattern**: Configurable retry with backoff
- **Circuit Breaker Pattern**: Failure detection and recovery
- **Decorator Pattern**: Request/response middleware

#### 3.5 Composite Fetcher (`composite.go`)

**Purpose**: Combines multiple fetchers for comprehensive input handling.

**Key Features**:
- Automatic fetcher selection by input type
- Support for custom fetcher registration
- Backwards compatibility with old interfaces
- Builder pattern for easy configuration

**Architecture**:
```go
type CompositeFetcher struct {
    kubernetesFetcher *KubernetesFetcher
    filesystemFetcher *FilesystemFetcher
    systemFetcher     *SystemFetcher
    customFetchers    map[InputType]InputFetcher
}
```

**Design Patterns**:
- **Composite Pattern**: Multiple fetchers as a single interface
- **Registry Pattern**: Custom fetcher registration
- **Builder Pattern**: Fluent configuration API

### 4. Unit Testing Framework (`testing/unit_test_helper.go`)

**Purpose**: Provides comprehensive testing utilities for validating CEL rules with mock data.

**Key Features**:
- Mock data generation for all input types
- Fluent assertion API for rule testing
- Test suite organization and execution
- Automated test reporting and comparison
- Performance benchmarking support
- Regression testing capabilities

**Architecture**:
```go
type RuleTester struct {
    scanner *celscanner.Scanner
    logger  TestLogger
}

type RuleTestCase struct {
    Name               string
    Description        string
    Rule               celscanner.CelRule
    MockData           map[string]interface{}
    ExpectedPass       bool
    ExpectedViolations []string
    Variables          []celscanner.CelVariable
}

type TestResult struct {
    TestCase   string
    Rule       string
    Passed     bool
    Expected   bool
    Actual     bool
    Duration   time.Duration
    Error      string
    Violations []string
    Message    string
}
```

**Testing Capabilities**:
- **Mock Data Creation**: Kubernetes pods, services, files, system commands, HTTP responses
- **Assertion Builders**: Fluent API for test expectations
- **Test Suites**: Organized test execution with setup/teardown
- **Result Comparison**: Baseline vs current test result analysis
- **Performance Testing**: Benchmark rule execution times
- **Report Generation**: Markdown and JSON test reports

**Usage Patterns**:
```go
// Fluent assertion API
tester.WithRule(rule).
    WithMockData("pods", mockPods).
    WithTestContext(t).
    ShouldPass()

// Test suite execution
suite := NewSecurityTestSuite("Pod Security").
    AddPodSecurityTest(rule, pods, true).
    Build()
result := tester.RunTestSuite(suite)
```

**Design Patterns**:
- **Builder Pattern**: Fluent test configuration
- **Mock Object Pattern**: Test data simulation
- **Template Method Pattern**: Standardized test execution
- **Strategy Pattern**: Different assertion strategies

## CEL Integration

### CEL Environment Setup

The scanner creates a CEL environment with:
- Dynamic variable declarations based on fetched resources
- Custom functions for JSON/YAML parsing
- Type-safe expression compilation
- Comprehensive error handling

### Custom Functions

```go
// JSON parsing function
jsonenvOpts := cel.Function("parseJSON",
    cel.MemberOverload("parseJSON_string",
        []*cel.Type{cel.StringType}, mapStrDyn, 
        cel.UnaryBinding(parseJSONString)))

// YAML parsing function
yamlenvOpts := cel.Function("parseYAML",
    cel.MemberOverload("parseYAML_string",
        []*cel.Type{cel.StringType}, mapStrDyn, 
        cel.UnaryBinding(parseYAMLString)))
```

### Expression Examples

```cel
// Kubernetes: Check if pods exist
pods.items.size() > 0

// Kubernetes: Security compliance check
pods.items.all(pod, 
    has(pod.spec.securityContext) && 
    pod.spec.securityContext.runAsNonRoot == true)

// Kubernetes: Resource limits validation
pods.items.all(pod, 
    pod.spec.containers.all(container, 
        has(container.resources) && 
        has(container.resources.limits)))

// Kubernetes: Cross-resource validation
pods.items.exists(pod, 
    services.items.exists(svc, 
        pod.metadata.labels.app == svc.spec.selector.app))

// HTTP: API health and security checks
api.success && 
api.statusCode == 200 && 
api.responseTime < 1000 &&
"X-Frame-Options" in api.headers

// HTTP: Authentication validation
!auth_endpoint.success && 
auth_endpoint.statusCode == 401 &&
has(auth_endpoint.headers) &&
"WWW-Authenticate" in auth_endpoint.headers

// File: Configuration validation
config.security.enabled == true &&
config.logging.level != "debug" &&
has(config.database.ssl) &&
config.database.ssl.verify == true

// System: Service and security checks
nginx_service.status == "active" &&
nginx_service.success == true &&
firewall_status.output.contains("active")

// Mixed: Multi-source validation
pods.items.size() > 0 &&
config.replicas <= pods.items.size() &&
health_check.success &&
log_config.content.contains("audit")
```

## Error Handling Strategy

### Compilation Errors
- Detailed error messages with context
- Undeclared variable detection
- Syntax error reporting
- Type mismatch identification

### Runtime Errors
- Graceful handling of missing resources
- Key access error warnings
- Evaluation failure recovery
- Contextual error annotations

### Error Result Structure
```go
type CheckResult struct {
    Status      CheckResultStatus  // PASS, FAIL, ERROR, NOT-APPLICABLE
    Warnings    []string          // User-friendly warnings
    Annotations map[string]string // Machine-readable context
}
```

## Performance Considerations

### Caching Mechanisms
- Resource discovery caching
- API resource mapping cache
- Global cache with thread safety

### Benchmarking
- Rule complexity benchmarks
- Resource size impact analysis
- Fetcher performance comparison
- Memory allocation tracking

### Optimization Strategies
- Lazy resource loading
- Concurrent fetching where possible
- Efficient CEL expression compilation
- Resource discovery batching

## Security Features

### Command Execution Security
- Allowlist-based command filtering
- Configurable arbitrary command restrictions
- Timeout protection
- Output sanitization

### Resource Access Control
- Kubernetes RBAC integration
- File system permission checks
- Network access restrictions
- Input validation

## Testing Strategy

### Unit Tests
- Individual component testing
- Mock implementations for dependencies
- Error condition coverage
- Edge case validation

### Integration Tests
- End-to-end scanning scenarios
- Multi-input type validation
- Resource fetching integration
- CEL expression evaluation

### Benchmark Tests
- Performance regression detection
- Scalability testing
- Memory usage analysis
- Comparative performance metrics

## Usage Patterns

### Basic Usage
```go
// Create scanner with composite fetcher
fetcher := NewCompositeFetcherBuilder().
    WithKubernetes(client, clientset).
    WithFilesystem("/etc").
    WithSystem(false).
    Build()

scanner := NewScanner(fetcher, logger)

// Define rules
rules := []Rule{
    NewRule("pods-exist", "pods.items.size() > 0", []Input{
        NewKubernetesInput("pods", "", "v1", "pods", "", ""),
    }),
}

// Execute scan
config := ScanConfig{Rules: rules}
results, err := scanner.Scan(ctx, config)
```

### Advanced Usage
```go
// Custom input types
customFetcher := NewCustomFetcher()
compositeFetcher.RegisterCustomFetcher(InputTypeHTTP, customFetcher)

// Mixed input validation
inputs := []Input{
    NewKubernetesInput("pods", "", "v1", "pods", "", ""),
    NewFileInput("config", "/etc/app/config.yaml", "yaml", false, true),
    NewSystemInput("nginx", "nginx", "", []string{}),
}

rule := NewRule("comprehensive-check", 
    "pods.items.size() > 0 && config.enabled && nginx.status == 'active'", 
    inputs)
```

## Extension Points

### Custom Input Fetchers
- Implement `InputFetcher` interface
- Register with `CompositeFetcher`
- Define custom input specifications

### Custom CEL Functions
- Extend CEL environment with domain-specific functions
- Add custom type converters
- Implement specialized operators

### Custom Result Processors
- Implement result transformation
- Add custom annotation processors
- Create specialized output formats

## User Stories

### 1. Platform Security Engineer

**Story**: As a platform security engineer, I want to scan my Kubernetes clusters for security compliance violations so that I can ensure our infrastructure meets security standards.

**Acceptance Criteria**:
- Scan pods for privileged containers and missing security contexts
- Validate resource limits and requests are properly configured
- Check for proper RBAC and service account configurations
- Generate reports in multiple formats (JSON, Markdown, HTML)
- Integrate with CI/CD pipelines for automated scanning

**Implementation**:
```go
// Security-focused scanning rules
rules := []celscanner.CelRule{
    NewRuleBuilder("no-privileged-containers").
        WithKubernetesInput("pods", "", "v1", "pods", "", "").
        SetExpression(`
            pods.items.all(pod,
                pod.spec.containers.all(container,
                    !has(container.securityContext.privileged) ||
                    container.securityContext.privileged == false
                )
            )
        `).
        WithName("No Privileged Containers").
        WithExtension("severity", "HIGH").
        Build(),
}
```

### 2. DevOps Engineer

**Story**: As a DevOps engineer, I want to validate my application configurations and infrastructure as code so that deployments are secure and compliant.

**Acceptance Criteria**:
- Scan configuration files for security misconfigurations
- Validate environment-specific settings
- Check system services and their configurations
- Monitor API endpoints for security headers and performance
- Automate compliance checks in deployment pipelines

**Implementation**:
```go
// Multi-source validation
fetcher := NewCompositeFetcherBuilder().
    WithKubernetes(client, clientset).
    WithFilesystem("/etc/myapp").
    WithSystem(false).
    WithHTTP(30*time.Second, true, 3).
    Build()

rules := []celscanner.CelRule{
    // File configuration validation
    NewRuleBuilder("secure-config").
        WithFileInput("config", "/etc/myapp/config.yaml", "yaml", false, false).
        SetExpression(`
            config.security.enabled == true &&
            config.logging.level != "debug" &&
            has(config.database.ssl)
        `).
        Build(),
    
    // API endpoint validation
    NewRuleBuilder("api-security").
        WithHTTPInput("api", "https://api.myapp.com/health", "GET", headers, nil).
        SetExpression(`
            api.success &&
            api.statusCode == 200 &&
            "X-Frame-Options" in api.headers
        `).
        Build(),
}
```

### 3. Compliance Officer

**Story**: As a compliance officer, I want to generate comprehensive compliance reports across our infrastructure so that I can demonstrate adherence to regulatory requirements.

**Acceptance Criteria**:
- Map security controls to specific regulations (SOC2, GDPR, HIPAA)
- Generate audit trails with timestamps and evidence
- Track compliance status over time
- Export reports for auditors and stakeholders
- Monitor compliance drift and violations

**Implementation**:
```go
// Compliance-focused rules with regulatory mapping
rules := []celscanner.CelRule{
    NewRuleBuilder("data-encryption-gdpr").
        WithKubernetesInput("secrets", "", "v1", "secrets", "", "").
        WithFileInput("tls_config", "/etc/ssl/app.conf", "text", false, false).
        SetExpression(`
            secrets.items.all(secret, secret.type == "tls") &&
            tls_config.content.contains("TLSv1.2")
        `).
        WithName("Data Encryption - GDPR Article 32").
        WithExtension("regulation", "GDPR").
        WithExtension("control", "Article 32").
        WithExtension("severity", "CRITICAL").
        Build(),
}
```

### 4. Application Developer

**Story**: As an application developer, I want to test my security rules during development so that I can catch compliance issues early in the development cycle.

**Acceptance Criteria**:
- Write unit tests for custom compliance rules
- Mock different scenarios and edge cases
- Validate rule logic before deployment
- Integration with existing test frameworks
- Fast feedback during development

**Implementation**:
```go
// Unit testing for compliance rules
func TestPodSecurityRules(t *testing.T) {
    tester := NewRuleTester()
    
    // Test privileged container detection
    privilegedPod := CreateMockPod("test-pod", "default", []map[string]interface{}{
        CreateMockContainer("app", "nginx:latest", true, nil),
    })
    
    rule := NewRuleBuilder("no-privileged").
        WithKubernetesInput("pods", "", "v1", "pods", "", "").
        SetExpression(`
            pods.items.all(pod,
                pod.spec.containers.all(container,
                    !has(container.securityContext.privileged) ||
                    container.securityContext.privileged == false
                )
            )
        `).
        Build()
    
    tester.WithRule(rule).
        WithMockData("pods", CreateMockKubernetesPods([]map[string]interface{}{privilegedPod})).
        WithTestContext(t).
        ShouldFail()
}
```

### 5. Site Reliability Engineer

**Story**: As an SRE, I want to continuously monitor the security posture of my running systems so that I can detect and respond to security issues quickly.

**Acceptance Criteria**:
- Run compliance scans on live systems without disruption
- Monitor configuration drift over time
- Alert on critical security violations
- Integrate with monitoring and alerting systems
- Maintain historical compliance data

**Implementation**:
```go
// Continuous monitoring setup
scanner := NewScanner(fetcher, logger)

// Schedule regular scans
ticker := time.NewTicker(15 * time.Minute)
for range ticker.C {
    results, err := scanner.Scan(context.Background(), config)
    if err != nil {
        logger.Error("Scan failed: %v", err)
        continue
    }
    
    // Process results and send alerts
    for _, result := range results {
        if result.Status == CheckResultFail {
            alertManager.SendAlert(Alert{
                Severity: result.Annotations["severity"],
                Message:  fmt.Sprintf("Compliance violation: %s", result.ID),
                Details:  result.Warnings,
            })
        }
    }
}
```

### 6. Security Auditor

**Story**: As a security auditor, I want to verify that security controls are properly implemented and functioning so that I can assess the organization's security posture.

**Acceptance Criteria**:
- Examine evidence of security control implementation
- Validate control effectiveness through testing
- Generate detailed audit reports with findings
- Track remediation of identified issues
- Compare current state against security baselines

**Implementation**:
```go
// Audit-focused scanning with evidence collection
auditRules := []celscanner.CelRule{
    // Network segmentation validation
    NewRuleBuilder("network-policies").
        WithKubernetesInput("netpols", "networking.k8s.io", "v1", "networkpolicies", "", "").
        SetExpression(`
            netpols.items.size() > 0 &&
            netpols.items.all(policy,
                has(policy.spec.podSelector) &&
                (has(policy.spec.ingress) || has(policy.spec.egress))
            )
        `).
        WithName("Network Segmentation Controls").
        WithExtension("control_family", "AC-4").
        WithExtension("evidence_type", "technical").
        Build(),
}

// Generate audit evidence
auditReport := AuditReport{
    Timestamp:    time.Now(),
    Auditor:      "external_auditor",
    Scope:        "production_cluster",
    Results:      results,
    Evidence:     collectEvidence(results),
    Remediation:  generateRemediationPlan(results),
}
```

## Design Principles

1. **Modularity**: Clear separation of concerns with pluggable components
2. **Extensibility**: Interface-based design allows easy extension
3. **Performance**: Caching and optimization throughout the stack
4. **Security**: Controlled execution with security boundaries
5. **Usability**: Builder patterns and convenience functions
6. **Reliability**: Comprehensive error handling and testing
7. **Maintainability**: Clear interfaces and documentation

## Conclusion

The CEL Scanner package demonstrates a well-architected compliance scanning framework that successfully combines:
- Powerful CEL expression evaluation
- Flexible input source integration
- Comprehensive error handling
- Performance optimization
- Security considerations
- Extensible design patterns

The architecture supports both simple use cases and complex compliance scenarios while maintaining clean separation of concerns and extensibility for future enhancements. 