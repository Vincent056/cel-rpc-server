# MCP Server Usage Examples and Best Practices

## Overview

This document provides examples and best practices for using the CEL RPC Server's MCP (Model Context Protocol) interface.

## Available Tools

### 1. verify_cel_with_tests

Verify CEL expressions using predefined test data without requiring cluster access.

**Example Request:**
```json
{
  "expression": "resource.spec.containers.all(c, has(c.resources.limits))",
  "inputs": [
    {
      "name": "resource",
      "type": "kubernetes",
      "kubernetes": {
        "group": "apps",
        "version": "v1",
        "resource": "deployments"
      }
    }
  ],
  "test_cases": [
    {
      "resource": {
        "spec": {
          "containers": [
            {
              "name": "app",
              "image": "nginx",
              "resources": {
                "limits": {
                  "cpu": "100m",
                  "memory": "128Mi"
                }
              }
            }
          ]
        }
      }
    }
  ]
}
```

### 2. verify_cel_live_resources

Verify CEL expressions against live Kubernetes resources in your cluster.

**Example Request:**
```json
{
  "expression": "resource.spec.replicas >= 2",
  "inputs": [
    {
      "name": "resource",
      "group": "",
      "version": "apps/v1",
      "resource": "deployments",
      "namespace": "default"
    }
  ]
}
```

### 3. discover_resource_types

Discover available Kubernetes resource types in the cluster.

**Example Request:**
```json
{
  "namespace": "default",
  "include_count": true,
  "include_samples": false
}
```

### 4. count_resources

Count specific resources in the cluster.

**Example Request:**
```json
{
  "resources": ["pods", "deployments", "services"],
  "namespace": "default"
}
```

### 5. get_resource_samples

Get sample resources for testing CEL expressions.

**Example Request:**
```json
{
  "resources": ["pods"],
  "namespace": "default",
  "limit": 5
}
```

## Common CEL Expression Examples

### Security Rules

```cel
# Ensure pods run as non-root
resource.spec.securityContext.runAsNonRoot == true

# Require security context
has(resource.spec.securityContext)

# Check for privileged containers
resource.spec.containers.all(c, !c.securityContext.privileged)
```

### Resource Management

```cel
# All containers must have resource limits
resource.spec.containers.all(c, has(c.resources.limits))

# Check CPU limits
resource.spec.containers.all(c, c.resources.limits.cpu != null)

# Memory limits within range
resource.spec.containers.all(c, 
  c.resources.limits.memory >= "128Mi" && 
  c.resources.limits.memory <= "2Gi"
)
```

### High Availability

```cel
# Deployment replicas check
resource.spec.replicas >= 2

# Pod disruption budget
resource.spec.minAvailable >= 1

# Anti-affinity rules
has(resource.spec.affinity.podAntiAffinity)
```

### Labeling and Metadata

```cel
# Required labels
has(resource.metadata.labels.app) && 
has(resource.metadata.labels.version)

# Label format validation
resource.metadata.labels.app.matches("^[a-z0-9-]+$")

# Annotation requirements
has(resource.metadata.annotations["company.com/owner"])
```

## Best Practices

### 1. Test Your Rules Locally First

Always use `verify_cel_with_tests` before running against live resources:

```bash
# Test with mock data first
mcp call verify_cel_with_tests < test_rule.json

# Then verify against live resources
mcp call verify_cel_live_resources < live_rule.json
```

### 2. Use Resource Discovery

Before writing rules, discover what resources are available:

```bash
# Discover all resource types
mcp call discover_resource_types '{"namespace": ""}'

# Get sample resources
mcp call get_resource_samples '{"resources": ["pods"], "limit": 3}'
```

### 3. Structured Error Handling

The MCP server returns structured errors. Always check the `IsError` field:

```json
{
  "content": [
    {
      "type": "text",
      "text": "Error: Invalid expression syntax"
    }
  ],
  "isError": true
}
```

### 4. Performance Considerations

- Use `test_cases` for rapid iteration during development
- Limit namespace scope when possible
- Use resource filters to reduce evaluation scope

### 5. Expression Guidelines

- Always check for field existence with `has()` before accessing
- Use `.all()` for container/volume validations
- Prefer explicit type checks over implicit conversions

## Integration Examples

### Claude Desktop Integration

Add to your Claude Desktop configuration:

```json
{
  "mcpServers": {
    "cel-rpc": {
      "command": "cel-rpc-server",
      "args": ["--mcp"],
      "env": {
        "KUBECONFIG": "/path/to/kubeconfig"
      }
    }
  }
}
```

### Shell Usage

```bash
# Start MCP server
cel-rpc-server --mcp

# In another terminal, use the client
echo '{"expression": "has(resource.metadata.name)", "inputs": [...]}' | \
  mcp call verify_cel_with_tests
```

### Programmatic Usage

```go
// Example Go client
client := mcp.NewClient("cel-rpc-server")
result, err := client.CallTool("verify_cel_with_tests", map[string]interface{}{
    "expression": "resource.spec.replicas >= 2",
    "inputs": []map[string]interface{}{
        {
            "name": "resource",
            "type": "kubernetes",
            "kubernetes": map[string]string{
                "resource": "deployments",
            },
        },
    },
    "test_cases": testData,
})
```

## Troubleshooting

### Common Issues

1. **Expression Syntax Errors**
   - Use the CEL specification for correct syntax
   - Test with simple expressions first

2. **Resource Not Found**
   - Check resource discovery output
   - Verify RBAC permissions

3. **Performance Issues**
   - Limit namespace scope
   - Use pagination for large result sets

### Debug Mode

Enable debug logging:

```bash
cel-rpc-server --mcp --debug
```

This will show:
- All MCP requests/responses
- CEL evaluation steps
- Resource fetch operations 