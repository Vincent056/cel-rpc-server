# MCP Rule Management Tools

This document describes the Model Context Protocol (MCP) tools for managing CEL rules in the rule library.

## Overview

The CEL RPC Server provides three MCP tools for rule management:
- `add_rule` - Add new CEL rules to the library
- `list_rules` - List and search existing rules
- `remove_rule` - Remove rules from the library

These tools are automatically registered when the MCP server starts with a configured rule store.

## Prerequisites

The rule management tools require:
1. The CEL RPC Server to be running with MCP support enabled
2. A configured rule store (currently using file-based storage in `./rules-library`)

## Tool Descriptions

### add_rule

Adds a new CEL rule to the rule library.

**Parameters:**
- `name` (required): Name of the rule
- `description` (required): Description of what the rule checks
- `expression` (required): The CEL expression for the rule
- `inputs` (required): Array of input sources for the CEL expression
  - `name`: Variable name to use in CEL expression
  - `type`: Type of input source (`kubernetes`, `file`, or `http`)
  - `kubernetes`: Kubernetes resource configuration (when type is `kubernetes`)
    - `group`: API group (empty for core resources)
    - `version`: API version (e.g., `v1`)
    - `resource`: Resource type (plural, e.g., `pods`, `configmaps`)
    - `namespace`: Namespace (optional)
- `tags` (optional): Array of tags for categorizing the rule
- `category` (optional): Rule category (e.g., security, compliance, performance)
- `severity` (optional): Severity level (low, medium, high, critical)
- `test_cases` (optional): Array of test cases for the rule
  - `description`: Description of the test case
  - `expected_result`: Expected result of the CEL expression
  - `inputs`: Test data for each input variable
- `metadata` (optional): Additional metadata
  - `compliance_framework`: Compliance framework (e.g., CIS, PCI-DSS, HIPAA)
  - `control_ids`: Array of control IDs from the compliance framework
  - `references`: Array of references or documentation URLs

**Example:**
```json
{
  "tool": "add_rule",
  "arguments": {
    "name": "Pod Security Context",
    "description": "Ensure all pods run as non-root",
    "expression": "pods.items.all(pod, pod.spec.securityContext.runAsNonRoot == true)",
    "inputs": [
      {
        "name": "pods",
        "type": "kubernetes",
        "kubernetes": {
          "version": "v1",
          "resource": "pods"
        }
      }
    ],
    "category": "security",
    "severity": "high",
    "tags": ["security", "pod-security", "non-root"]
  }
}
```

### list_rules

Lists CEL rules from the rule library with optional filters.

**Parameters:**
- `tags` (optional): Array of tags to filter by
- `category` (optional): Filter by category
- `severity` (optional): Filter by severity level (low, medium, high, critical)
- `compliance_framework` (optional): Filter by compliance framework
- `resource_type` (optional): Filter by Kubernetes resource type
- `search_text` (optional): Search text to filter rules by name, description, or expression
- `verified_only` (optional): Show only verified rules
- `page_size` (optional): Number of rules to return (default: 20, max: 100)

**Example:**
```json
{
  "tool": "list_rules",
  "arguments": {
    "category": "security",
    "severity": "high",
    "page_size": 50
  }
}
```

**Response Format:**
The tool returns a formatted list showing:
- Rule name and ID
- Description
- Category and severity
- Tags
- Verification status
- Input types

### remove_rule

Removes a CEL rule from the rule library.

**Parameters:**
- `rule_id` (required): ID of the rule to remove

**Example:**
```json
{
  "tool": "remove_rule",
  "arguments": {
    "rule_id": "93ac685f-ff17-4b4d-b0bd-658732c92ff2"
  }
}
```

## Integration with Rule Store

The rule management tools integrate with the FileRuleStore implementation which:
- Stores rules as JSON files in the `./rules-library` directory
- Maintains indexes for efficient filtering by tags, category, severity, etc.
- Supports both protobuf JSON format and legacy JSON format for backward compatibility
- Automatically generates UUIDs for new rules
- Tracks creation and modification timestamps

## Usage Examples

### Adding a Security Rule

```json
{
  "tool": "add_rule",
  "arguments": {
    "name": "Network Policy Required",
    "description": "Check if all namespaces have at least one NetworkPolicy",
    "expression": "namespaces.items.all(ns, networkpolicies.items.exists(np, np.metadata.namespace == ns.metadata.name))",
    "inputs": [
      {
        "name": "namespaces",
        "type": "kubernetes",
        "kubernetes": {
          "version": "v1",
          "resource": "namespaces"
        }
      },
      {
        "name": "networkpolicies",
        "type": "kubernetes",
        "kubernetes": {
          "group": "networking.k8s.io",
          "version": "v1",
          "resource": "networkpolicies"
        }
      }
    ],
    "category": "security",
    "severity": "high",
    "tags": ["network-security", "network-policy"],
    "metadata": {
      "compliance_framework": "CIS",
      "control_ids": ["5.3.2"],
      "references": ["https://www.cisecurity.org/benchmark/kubernetes"]
    }
  }
}
```

### Searching for Rules

```json
{
  "tool": "list_rules",
  "arguments": {
    "search_text": "network",
    "category": "security",
    "page_size": 10
  }
}
```

### Listing Rules by Compliance Framework

```json
{
  "tool": "list_rules",
  "arguments": {
    "compliance_framework": "CIS",
    "verified_only": true
  }
}
```

## Error Handling

The tools provide detailed error messages for common issues:
- Missing required parameters
- Invalid rule expressions
- Rule not found (for remove operations)
- Rule store unavailable

## Testing

The rule management tools include comprehensive tests in `pkg/mcp/rule_management_test.go`:
- `TestRuleManagementTools` - Verifies all tools are registered
- `TestAddRuleTool` - Tests adding rules to the store
- `TestListRulesTool` - Tests listing and filtering rules
- `TestRemoveRuleTool` - Tests removing rules from the store

Run tests with:
```bash
go test ./pkg/mcp/... -v -run "Test.*Rule.*Tool"
```

## Architecture

The rule management tools follow the MCP architecture:
1. Tools are registered in `pkg/mcp/server.go` when the MCP server starts
2. Tool definitions and handlers are in `pkg/mcp/rule_management_tools.go`
3. Each tool handler interacts with the RuleStore interface
4. The FileRuleStore implementation handles persistence to disk

## Future Enhancements

Potential improvements for the rule management tools:
- Batch operations for adding/removing multiple rules
- Export/import functionality
- Rule versioning and history tracking
- Rule validation before saving
- Rule templates and wizards
- Integration with external rule repositories
