# MCP (Model Context Protocol) Integration

The CEL RPC Server now includes optional MCP tool support, allowing it to expose its functionality as MCP tools that can be consumed by AI agents and other MCP clients.

## Enabling MCP Support

MCP support is disabled by default. To enable it, set the `ENABLE_MCP` environment variable:

```bash
ENABLE_MCP=true ./cel-server
```

When enabled, the server will expose MCP endpoints alongside the regular Connect RPC endpoints.

## MCP Endpoints

When MCP is enabled, the following endpoints are available:

- `GET /mcp/tools` - List available MCP tools
- `POST /mcp/tools/execute` - Execute a specific tool
- `GET /mcp/health` - Health check endpoint

## Available Tools

### 1. verify_cel

Verifies CEL expressions against test cases or live resources.

**Input Schema:**
```json
{
  "expression": "string",
  "test_cases": [...],
  "inputs": [...]
}
```

### 2. discover_resources

Discovers Kubernetes resources in a cluster with filtering options.

**Input Schema:**
```json
{
  "namespace": "string",
  "gvr_filters": [...],
  "options": {...}
}
```

## Example Usage

### Listing Available Tools

```bash
curl http://localhost:8349/mcp/tools
```

### Executing a Tool

```bash
curl -X POST http://localhost:8349/mcp/tools/execute \
  -H "Content-Type: application/json" \
  -d '{
    "tool": "verify_cel",
    "input": {
      "expression": "pod.metadata.name == \"test\"",
      "test_cases": [{
        "inputs": {"pod": {"metadata": {"name": "test"}}},
        "expected_result": true,
        "description": "Should match pod name"
      }]
    }
  }'
```

## Integration with LangGraph

The MCP tools are designed to work seamlessly with the LangGraph workflow. Simply point your MCP client to the CEL server with MCP enabled:

```python
mcp_client = MCPClient(base_url="http://localhost:8349")
```

## Configuration

You can configure the server port using the `PORT` environment variable:

```bash
ENABLE_MCP=true PORT=9000 ./cel-server
```

This will start the server on port 9000 with both Connect RPC and MCP endpoints available. 