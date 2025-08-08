# CEL RPC Server

A high-performance gRPC server for validating Kubernetes resources using Common Expression Language (CEL). Features AI-powered rule generation with support for multiple AI providers including OpenAI, Google Gemini, and local models via Ollama.

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
  -v ~/.kube/config:/KUBECONFIG/kubeconfig:Z \
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
  -v ~/.kube/config:/KUBECONFIG/kubeconfig:Z \
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
  -v ~/.kube/config:/KUBECONFIG/kubeconfig:Z \
  -v ./rules-library:/home/celuser/app/rules-library:Z \
  ghcr.io/vincent056/cel-rpc-server
```

**Volume Mounts:**
- `~/.kube/config:/KUBECONFIG/kubeconfig` - Mounts your Kubernetes config for cluster access
- `./rules-library:/home/celuser/app/rules-library` - Mounts local rules directory for custom CEL rules

**Note:** The `:Z` flag in Podman volume mounts is for SELinux contexts. If you encounter permission issues, ensure your kubeconfig file is readable by the container user (UID 1001).

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

| Variable | Description | Default |
|----------|-------------|---------|
| `AI_PROVIDER` | AI provider to use: `openai`, `gemini`, `custom` | `openai` |
| `OPENAI_API_KEY` | OpenAI API key (when using OpenAI) | - |
| `OPENAI_MODEL` | OpenAI model to use | `gpt-4.1` |
| `GEMINI_API_KEY` | Google Gemini API key (when using Gemini) | - |
| `GEMINI_MODEL` | Gemini model to use | `gemini-2.5-flash` |
| `CUSTOM_AI_ENDPOINT` | Custom AI endpoint URL | - |
| `CUSTOM_AI_MODEL` | Model name for custom endpoint | - |
| `CUSTOM_AI_API_KEY` | API key for custom endpoint | - |
| `CUSTOM_AI_HEADERS` | Custom headers (format: `Header1:Value1,Header2:Value2`) | - |
| `DISABLE_INFORMATION_GATHERING` | Disable AI research phase for faster responses | `false` |

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
  - `ChatAssist`: AI-powered chat assistance for rule generation
  - `GetResourceSamples`: Get sample resources from the cluster
  - And more...

- **MCP Tools**: `http://localhost:8349/mcp`
  - Model Context Protocol tools for enhanced AI interactions

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

## Troubleshooting

### Container Issues

```bash
# Check logs
podman logs cel-rpc-server

# Check if server is running
curl http://localhost:8349/
```

### AI Provider Issues

```bash
# Test Ollama connection
curl http://localhost:11434/api/tags

# Verify environment variables
podman exec cel-rpc-server env | grep AI_
```

### Kubernetes Access

Ensure your kubeconfig is properly mounted:
```bash
# For podman/docker
-v ~/.kube/config:/root/.kube/config:ro

# For local binary
export KUBECONFIG=~/.kube/config
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

## Troubleshooting

### Permission Denied Errors

If you encounter "permission denied" errors when mounting kubeconfig:

1. **With Podman**: Ensure you use the `:Z` flag for SELinux contexts:
   ```bash
   -v ~/.kube/config:/KUBECONFIG/kubeconfig:Z
   ```

2. **File Permissions**: The container runs as user ID 1001. Ensure your kubeconfig is readable:
   ```bash
   chmod 644 ~/.kube/config
   ```

3. **Alternative Mount**: If issues persist, you can copy the kubeconfig into the container:
   ```bash
   podman cp ~/.kube/config cel-rpc-server:/KUBECONFIG/kubeconfig
   podman exec cel-rpc-server chown celuser:celuser /KUBECONFIG/kubeconfig
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