# CEL Go Scanner

A powerful, flexible compliance scanning library for Kubernetes and system resources using Google's Common Expression Language (CEL).

## Features

- **CEL-based Rules**: Write compliance rules using Google's CEL for powerful, expressive evaluations
- **Multi-Input Support**: Fetch data from Kubernetes clusters, filesystems, system commands, and HTTP APIs
- **Flexible Architecture**: Modular fetcher system supporting custom data sources
- **Live Cluster Integration**: Real-time scanning of live Kubernetes clusters
- **System Monitoring**: Monitor system health, services, and security configurations
- **Security-First**: Built-in security controls for safe system command execution
- **Rich Metadata**: Extensible metadata system for compliance frameworks (CIS, STIG, etc.)

## Quick Start

### Using Make Targets (Recommended)

Run examples easily with the provided Makefile targets:

```bash
# Run all examples
make examples

# Run specific examples
make example-basic             # Basic usage patterns
make example-complex           # Advanced CEL expressions  
make example-kubernetes        # Kubernetes resource scanning
make example-filesystem        # File and directory scanning
make example-system-monitoring # System health monitoring
make example-system-security   # Security compliance checks
make example-live-kubernetes   # Live cluster scanning (requires cluster)

# Build and manage examples
make build-examples            # Build all example binaries
make clean-examples            # Clean up example binaries
make help                      # Show all available targets
```

### Manual Usage

```bash
# Install dependencies
go mod tidy

# Run a specific example
cd examples/basic
go run main.go

# Run tests
go test ./...
```

## Examples

### 🔧 Basic Usage
```go
// Create a simple compliance rule
rule := celscanner.NewRuleBuilder("pod-security").
    WithKubernetesInput("pods", "", "v1", "pods", "default", "").
    SetExpression(`size(pods.items) > 0`).
    WithName("Pod Count Check").
    Build()

// Create scanner and run
scanner := celscanner.NewScanner(fetcher, logger)
results, err := scanner.Scan(ctx, celscanner.ScanConfig{Rules: []celscanner.CelRule{rule}})
```

### 🔒 System Security Monitoring
```go
// Monitor system security compliance
rule := celscanner.NewRuleBuilder("selinux-check").
    WithSystemInput("selinux", "", "getenforce", []string{}).
    SetExpression(`selinux.success && contains(selinux.output, "Enforcing")`).
    WithName("SELinux Enforcement").
    WithExtension("severity", "HIGH").
    Build()
```

### 🌐 Live Kubernetes Scanning
```go
// Scan live Kubernetes cluster
rule := celscanner.NewRuleBuilder("pod-security").
    WithKubernetesInput("pods", "", "v1", "pods", "", "").
    SetExpression(`pods.items.all(pod, has(pod.spec.securityContext))`).
    WithName("Pod Security Context").
    Build()
```

## Available Examples

| Example | Description | Make Target |
|---------|-------------|-------------|
| **Basic** | Fundamental usage patterns and API introduction | `make example-basic` |
| **Complex** | Advanced CEL expressions and rule composition | `make example-complex` |
| **Kubernetes** | Kubernetes resource scanning with mock data | `make example-kubernetes` |
| **Filesystem** | File and directory scanning patterns | `make example-filesystem` |
| **Live Kubernetes** | Real cluster scanning (requires kubeconfig) | `make example-live-kubernetes` |
| **System Monitoring** | System health and service monitoring | `make example-system-monitoring` |
| **System Security** | Security compliance and hardening checks | `make example-system-security` |

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   CEL Rules     │───▶│  Scanner Engine  │───▶│ Compliance      │
│                 │    │                  │    │ Results         │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                               │
                               ▼
                    ┌──────────────────┐
                    │ Composite Fetcher│
                    └──────────────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
    ┌─────────────────┐ ┌─────────────┐ ┌─────────────────┐
    │ Kubernetes      │ │ System      │ │ Filesystem      │
    │ Fetcher         │ │ Fetcher     │ │ Fetcher         │
    └─────────────────┘ └─────────────┘ └─────────────────┘
```

## Development

```bash
# Run tests with coverage
make test-coverage

# Format and lint code
make quality

# Build project
make build

# See all available targets
make help
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details. 