# CEL RPC Server

A high-performance gRPC server for validating Kubernetes resources using Common Expression Language (CEL). Features AI-powered rule generation with support for multiple AI providers including OpenAI, Google Gemini, and local models via Ollama.

**Note:** AI providers are only required for frontend/backend API workflows that use AI-powered rule generation. When using this server purely as an MCP (Model Context Protocol) server, no AI API keys are needed.

## MCP Server Usage (No AI Keys Required)

The server can be used as an MCP (Model Context Protocol) server without any AI API keys:

```bash
# Start the server for MCP usage (no AI keys needed)
podman run -d \
  --name cel-rpc-server \
  -p 8349:8349 \
  -v ./rules-library:/home/celuser/app/rules-library:Z \
  --replace \
  ghcr.io/vincent056/cel-rpc-server

```

### MCP Configuration

Add this to your MCP client configuration:

```json
{
  "mcpServers": {
    "cel-validation": {
      "url": "http://localhost:8349/mcp",
      "headers": {
        "Content-Type": "application/json"
      }
    }
  }
}
```

This provides CEL validation, rule management, and Kubernetes resource discovery capabilities to AI assistants without requiring the server itself to have AI provider access.

**MCP Tools Available:**
- `verify_cel_with_tests` - Test CEL expressions with sample data
- `verify_cel_live_resources` - Validate CEL against live cluster resources
- `discover_resource_types` - Discover available Kubernetes resources
- `count_resources` - Count instances of specific resource types
- `get_resource_samples` - Get sample resources from cluster
- `add_rule`, `list_rules`, `remove_rule`, `test_rule` - Rule management

## Features

- **CEL Validation**: Validate Kubernetes resources using CEL expressions
- **AI-Powered Rule Generation**: Generate CEL rules from natural language descriptions
- **Multi-Provider Support**: Use OpenAI, Gemini, or any OpenAPI-compatible AI provider
- **Resource Discovery**: Discover and analyze Kubernetes cluster resources
- **Streaming Responses**: Real-time streaming for chat interactions
- **MCP Tools Integration**: Model Context Protocol tools for enhanced functionality

## Quick Start

### Option 1: Using Pre-built Container Image (Recommended)

The easiest way to run the CEL RPC server is using the pre-built container image with Podman or Docker.

#### With Podman

```bash
# Pull the image
podman pull ghcr.io/vincent056/cel-rpc-server

# Run with OpenAI (default)
podman run \
  --name cel-rpc-server \
  -p 8349:8349 \
  -e OPENAI_API_KEY=your-openai-api-key \
  -v ./rules-library:/home/celuser/app/rules-library:Z \
  --replace \
  ghcr.io/vincent056/cel-rpc-server

# Run with Gemini
podman run -d \
  --name cel-rpc-server \
  -p 8349:8349 \
  -e AI_PROVIDER=gemini \
  -e GEMINI_API_KEY=your-gemini-api-key \
  -e DISABLE_INFORMATION_GATHERING=true \
  -v ./rules-library:/home/celuser/app/rules-library:Z \
  --replace \
  ghcr.io/vincent056/cel-rpc-server



# Run with local Ollama
podman run -d \
  --name cel-rpc-server \
  -p 8349:8349 \
  --network host \
  -e AI_PROVIDER=custom \
  -e CUSTOM_AI_ENDPOINT=http://localhost:11434/api/generate \
  -e CUSTOM_AI_MODEL=llama2:7b \
  -e DISABLE_INFORMATION_GATHERING=true \
  -v ./rules-library:/home/celuser/app/rules-library:Z \
  ghcr.io/vincent056/cel-rpc-server
```

**Volume Mounts:**
- `~/.kube/config:/KUBECONFIG/kubeconfig` - Mounts your Kubernetes config for cluster access
- `./rules-library:/home/celuser/app/rules-library` - Mounts local rules directory for custom CEL rules

**Important Notes:**
- The `:Z` flag in Podman volume mounts is for SELinux contexts
- If kubeconfig doesn't exist, the server will run in mock mode (no actual cluster access)
- For the easiest setup experience, use: `./scripts/quick-start.sh`

#### With Docker

Simply replace `podman` with `docker` in the above commands:

```bash
# Pull the image
docker pull ghcr.io/vincent056/cel-rpc-server

# Run with OpenAI
docker run -d \
  --name cel-rpc-server \
  -p 8349:8349 \
  -e OPENAI_API_KEY=your-openai-api-key \
  -v ~/.kube/config:/KUBECONFIG/kubeconfig:ro \
  -v ./rules-library:/home/celuser/app/rules-library:ro \
  ghcr.io/vincent056/cel-rpc-server
```

### Option 2: Building and Running Locally

#### Prerequisites

- Go 1.21+ installed
- Access to a Kubernetes cluster (with kubeconfig configured)
- (Optional) AI provider API key

#### Build from Source

```bash
# Clone the repository
git clone https://github.com/vincent056/cel-rpc-server.git
cd cel-rpc-server

# Build the binary
go build -o cel-rpc-server cmd/server/*.go

# Or use the build script
./build-scanner.sh
```

#### Run the Binary

```bash
# Run with OpenAI (default)
export OPENAI_API_KEY=your-openai-api-key
./cel-rpc-server

  # Run with Gemini
  export AI_PROVIDER=gemini
  export GEMINI_API_KEY=your-gemini-api-key
  ./cel-rpc-server

# Run with Ollama
export AI_PROVIDER=custom
export CUSTOM_AI_ENDPOINT=http://localhost:11434/api/generate
export CUSTOM_AI_MODEL=llama2:7b
export DISABLE_INFORMATION_GATHERING=true
./cel-rpc-server
```

## Configuration

### Rules Library

The server looks for CEL rule definitions in the `./rules-library` directory. When running in a container, mount your local rules directory:

```bash
# Create rules directory if it doesn't exist
mkdir -p ./rules-library

# Mount when running container
-v ./rules-library:/home/celuser/app/rules-library:Z  # Podman
-v ./rules-library:/home/celuser/app/rules-library:ro  # Docker
```

Rules should be in YAML format. See the [rules-library](./rules-library) directory for examples.

### Environment Variables

| Variable | Description | Default | Required For |
|----------|-------------|---------|--------------|
| `AI_PROVIDER` | AI provider to use: `openai`, `gemini`, `custom` | `openai` | Frontend/Backend AI workflows |
| `OPENAI_API_KEY` | OpenAI API key (when using OpenAI) | - | AI-powered rule generation |
| `OPENAI_MODEL` | OpenAI model to use | `gpt-4.1` | OpenAI workflows |
| `GEMINI_API_KEY` | Google Gemini API key (when using Gemini) | - | AI-powered rule generation |
| `GEMINI_MODEL` | Gemini model to use | `gemini-2.5-flash` | Gemini workflows |
| `CUSTOM_AI_ENDPOINT` | Custom AI endpoint URL | - | Custom AI workflows |
| `CUSTOM_AI_MODEL` | Model name for custom endpoint | - | Custom AI workflows |
| `CUSTOM_AI_API_KEY` | API key for custom endpoint | - | Custom AI workflows |
| `CUSTOM_AI_HEADERS` | Custom headers (format: `Header1:Value1,Header2:Value2`) | - | Custom AI workflows |
| `DISABLE_INFORMATION_GATHERING` | Disable AI research phase for faster responses | `false` | AI workflows optimization |

**Note:** All AI-related environment variables are **optional** when using the server purely as an MCP server for CEL validation and rule management.

### AI Provider Examples

#### OpenAI (Cloud)
```bash
export AI_PROVIDER=openai
export OPENAI_API_KEY=sk-...
export OPENAI_MODEL=gpt-4.1
```

#### Google Gemini (Cloud)
```bash
export AI_PROVIDER=gemini
export GEMINI_API_KEY=AIza...
```

#### Ollama (Local)
```bash
export AI_PROVIDER=custom
export CUSTOM_AI_ENDPOINT=http://localhost:11434/api/generate
export CUSTOM_AI_MODEL=mistral:7b  # or llama2, codellama, etc.
export DISABLE_INFORMATION_GATHERING=true  # Recommended for local models
```

#### Anthropic Claude (via Custom)
```bash
export AI_PROVIDER=custom
export CUSTOM_AI_ENDPOINT=https://api.anthropic.com/v1/messages
export CUSTOM_AI_API_KEY=sk-ant-...
export CUSTOM_AI_HEADERS="anthropic-version:2023-06-01"
```

## Usage

Once the server is running, it exposes several endpoints:

### gRPC/Connect Endpoints

- **CEL Validation Service**: `http://localhost:8349/cel.v1.CELValidationService/`
  - `ValidateCEL`: Validate CEL expressions
  - `ChatAssist`: AI-powered chat assistance for rule generation (requires AI API key)
  - `GetResourceSamples`: Get sample resources from the cluster
  - And more...

- **MCP Tools**: `http://localhost:8349/mcp`
  - Model Context Protocol tools for enhanced AI interactions
  - **No AI API key required** - works independently for CEL validation and rule management

### Example: Using grpcurl

```bash
# List available services
grpcurl -plaintext localhost:8349 list

# Get resource samples
grpcurl -plaintext -d '{"resourceType": "pods", "namespace": "default", "maxSamples": 5}' \
  localhost:8349 cel.v1.CELValidationService/GetResourceSamples

# Generate a CEL rule using AI
grpcurl -plaintext -d '{"message": "create a rule to ensure all pods have resource limits"}' \
  localhost:8349 cel.v1.CELValidationService/ChatAssist
```

### Example: Using the Web UI

Open your browser to `http://localhost:8349` to access the built-in web interface.


## Performance Tips

### For Local Models (Ollama)

1. **Use smaller models** for better performance:
   ```bash
   export CUSTOM_AI_MODEL=llama2:7b  # Instead of 32b models
   ```

2. **Disable information gathering** to reduce API calls:
   ```bash
   export DISABLE_INFORMATION_GATHERING=true
   ```

3. **Ensure adequate resources**:
   - Allocate sufficient RAM to Ollama
   - Use GPU acceleration if available

### For Production Use

1. **Use cloud-based APIs** (OpenAI, Gemini) for reliability
2. **Configure appropriate timeouts** for your use case
3. **Monitor resource usage** and scale accordingly

## Kubeconfig Setup

### Recommended: Runtime Setup (No File Mounting)

The easiest and most secure approach is to run the container first, then provide kubeconfig afterward:

```bash
# 1. Start the container (no AI key needed for MCP usage)
podman run -d \
  --name cel-rpc-server \
  -p 8349:8349 \
  -v ./rules-library:/home/celuser/app/rules-library:Z \
  ghcr.io/vincent056/cel-rpc-server

# Or with AI provider for frontend/backend workflows
podman run -d \
  --name cel-rpc-server \
  -p 8349:8349 \
  -e OPENAI_API_KEY=your-openai-api-key \
  -v ./rules-library:/home/celuser/app/rules-library:Z \
  ghcr.io/vincent056/cel-rpc-server

# 2. Set up kubeconfig after startup (choose one method):

# Method A: From your local kubeconfig
podman exec cel-rpc-server setup-kubeconfig --from-command "kubectl config view --raw"

# Method B: Paste kubeconfig content directly
podman exec -it cel-rpc-server setup-kubeconfig --from-stdin

# Method C: Copy file then setup (if you have permission issues)
podman cp ~/.kube/config cel-rpc-server:/tmp/my-kubeconfig
podman exec cel-rpc-server setup-kubeconfig --from-file /tmp/my-kubeconfig

# 3. Test the setup
podman exec cel-rpc-server setup-kubeconfig --test
```

**Advantages of Runtime Setup:**
- ✅ No file permission issues
- ✅ No SELinux problems
- ✅ Secure (kubeconfig only exists in container)
- ✅ Easy to change kubeconfig without restarting
- ✅ Works on all systems

### Alternative: File Mounting (Traditional)

If you prefer to mount your kubeconfig file:

```bash
# Ensure your kubeconfig is readable
chmod 644 ~/.kube/config

# Run with mounted kubeconfig
podman run -d \
  --name cel-rpc-server \
  -p 8349:8349 \
  -e OPENAI_API_KEY=your-openai-api-key \
  -v ~/.kube/config:/KUBECONFIG/kubeconfig:Z \
  -v ./rules-library:/home/celuser/app/rules-library:Z \
  ghcr.io/vincent056/cel-rpc-server
```

### Running Without Kubeconfig (Mock Mode)

For testing or when you don't have a cluster:

```bash
# Run in mock mode (no AI key needed for MCP usage)
podman run -d \
  --name cel-rpc-server \
  -p 8349:8349 \
  -v ./rules-library:/home/celuser/app/rules-library:Z \
  ghcr.io/vincent056/cel-rpc-server

# Or with AI provider for frontend/backend workflows
podman run -d \
  --name cel-rpc-server \
  -p 8349:8349 \
  -e OPENAI_API_KEY=your-openai-api-key \
  -v ./rules-library:/home/celuser/app/rules-library:Z \
  ghcr.io/vincent056/cel-rpc-server

# The server automatically detects no kubeconfig and uses mock mode
```

## Troubleshooting

### Container Issues

```bash
# Check logs
podman logs cel-rpc-server

# Check if server is running
curl http://localhost:8349/

# Check container environment
podman exec cel-rpc-server env | grep KUBECONFIG
```

### AI Provider Issues

```bash
# Test Ollama connection
curl http://localhost:11434/api/tags

# Verify environment variables
podman exec cel-rpc-server env | grep AI_
```

### Kubeconfig Issues

```bash
# Check kubeconfig status
podman exec cel-rpc-server setup-kubeconfig --status

# Test current kubeconfig
podman exec cel-rpc-server setup-kubeconfig --test

# Clear and reset kubeconfig
podman exec cel-rpc-server setup-kubeconfig --clear

# Set up new kubeconfig
podman exec -it cel-rpc-server setup-kubeconfig --from-stdin
```

### Kubernetes Access Issues

```bash
# Test kubeconfig inside container
podman exec cel-rpc-server kubectl config view

# Check file permissions (if using mounted files)
podman exec cel-rpc-server ls -la /KUBECONFIG/

# Verify cluster connectivity
podman exec cel-rpc-server kubectl cluster-info
```

## Development

### Building the Container Image

```bash
# Build locally
podman build -t cel-rpc-server .

# Or using Docker
docker build -t cel-rpc-server .
```

### Running Tests

```bash
go test ./...
```

### Permission Denied Errors

If you still encounter "permission denied" errors after using the setup script:

1. **Use the quick start script**:
   ```bash
   ./scripts/quick-start.sh
   ```

2. **Manual permission fix**:
   ```bash
   chmod 644 ~/.kube/config
   chmod 755 ~/.kube
   ```

3. **Alternative: Copy kubeconfig into running container**:
   ```bash
   podman cp ~/.kube/config cel-rpc-server:/KUBECONFIG/kubeconfig
   podman exec cel-rpc-server chown celuser:celuser /KUBECONFIG/kubeconfig
   ```

4. **SELinux contexts** (Podman/RHEL/Fedora):
   ```bash
   # Use :Z flag for proper SELinux labeling
   -v ~/.kube/config:/KUBECONFIG/kubeconfig:Z
   ```

### Connection Issues

- Ensure the server is listening on the correct port (default: 8349)
- Check container logs: `podman logs cel-rpc-server`
- Verify your Kubernetes cluster is accessible from within the container

## License

[Add your license information here]

## Contributing

[Add contribution guidelines here]

## Support

For issues and questions:
- GitHub Issues: [your-repo-url]/issues
- Documentation: See `AI_PROVIDER_CONFIG.md` for detailed AI configuration options