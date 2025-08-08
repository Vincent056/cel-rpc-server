# AI Provider Configuration

This document explains how to configure different AI providers for the CEL RPC server.

## Supported Providers

The CEL RPC server now supports multiple AI providers:

1. **OpenAI** (default)
2. **Google Gemini**
3. **Custom OpenAPI-compatible endpoints**

## Configuration

AI providers are configured through environment variables.

### General Configuration

- `AI_PROVIDER`: Specifies which AI provider to use. Options: `openai`, `gemini`, `custom`. Defaults to `openai`.
- `DISABLE_INFORMATION_GATHERING`: Set to `true` to disable the information search/gathering phase. This can speed up responses and avoid timeouts with slower models.

### OpenAI Configuration

```bash
export AI_PROVIDER=openai  # Optional, this is the default
export OPENAI_API_KEY=your-openai-api-key
export OPENAI_MODEL=gpt-4  # Optional, defaults to gpt-4
```

Example models:
- `gpt-4.1`

### Gemini Configuration

```bash
export AI_PROVIDER=gemini
export GEMINI_API_KEY=your-gemini-api-key
export GEMINI_MODEL=gemini-2.5-pro  # Optional, defaults to gemini-2.5-pro
```

Example Supported models:
- `gemini-2.5-pro`
- `gemini-2.5-flash` (if you need image support in the future)

### Custom OpenAPI-Compatible Endpoints

For custom AI providers that follow the OpenAI API format:

```bash
export AI_PROVIDER=custom
export CUSTOM_AI_ENDPOINT=https://your-api-endpoint.com/v1/chat/completions
export CUSTOM_AI_API_KEY=your-api-key  # Optional, depending on your provider
export CUSTOM_AI_MODEL=your-model-name  # Optional
export CUSTOM_AI_HEADERS="X-Custom-Header:value,Another-Header:value2"  # Optional custom headers
```

The custom provider supports:
- OpenAI-compatible endpoints (most common)
- Simple prompt-based endpoints
- Various response formats

## Examples

### Using OpenAI (Default)

```bash
# Minimal configuration - uses OpenAI by default
export OPENAI_API_KEY=sk-...

# Or explicitly set provider
export AI_PROVIDER=openai
export OPENAI_API_KEY=sk-...
export OPENAI_MODEL=gpt-4-turbo-preview
```

### Using Google Gemini

```bash
export AI_PROVIDER=gemini
export GEMINI_API_KEY=AIza...
export GEMINI_MODEL=gemini-2.5-flash  # Optional, defaults to gemini-2.5-flash
```

Example Available Gemini models:
- `gemini-2.5-flash` - Latest fast model (default)
- `gemini-2.5-pro` - Latest high-quality model

### Using Anthropic Claude (via OpenAI-compatible endpoint)

```bash
export AI_PROVIDER=custom
export CUSTOM_AI_ENDPOINT=https://api.anthropic.com/v1/messages
export CUSTOM_AI_API_KEY=sk-ant-...
export CUSTOM_AI_HEADERS="anthropic-version:2023-06-01"
```

### Using Local LLM (e.g., LM Studio, Ollama)

```bash
export AI_PROVIDER=custom
export CUSTOM_AI_ENDPOINT=http://localhost:1234/v1/chat/completions
export CUSTOM_AI_MODEL=llama-2-7b-chat
# No API key needed for local models

# For large/slow models, disable information gathering to avoid timeouts
export DISABLE_INFORMATION_GATHERING=true
```

For Ollama specifically:
```bash
export AI_PROVIDER=custom
export CUSTOM_AI_ENDPOINT=http://localhost:11434/api/generate
export CUSTOM_AI_MODEL=mistral:7b  # or llama2:7b, qwen2.5:7b
export DISABLE_INFORMATION_GATHERING=true  # Recommended for local models
```

### Using Azure OpenAI

```bash
export AI_PROVIDER=custom
export CUSTOM_AI_ENDPOINT=https://your-resource.openai.azure.com/openai/deployments/your-deployment/chat/completions?api-version=2023-05-15
export CUSTOM_AI_API_KEY=your-azure-key
export CUSTOM_AI_HEADERS="api-key:your-azure-key"
```

## Backward Compatibility

The server maintains backward compatibility:
- If no `AI_PROVIDER` is set, it defaults to OpenAI
- The original `OPENAI_API_KEY` environment variable still works
- Existing configurations will continue to function without changes

## Performance Optimization

### Disabling Information Gathering

The information gathering phase adds an extra AI call to research platform-specific details. While helpful for accuracy, it can slow down responses, especially with large models.

To disable it:
```bash
export DISABLE_INFORMATION_GATHERING=true
```

**When to disable:**
- Using large local models (32B+ parameters)
- Running on limited hardware
- Need faster response times
- Working with well-known Kubernetes resources

**When to keep enabled:**
- Using fast cloud APIs (OpenAI, Gemini)
- Working with platform-specific features (OpenShift, cloud providers)
- Need compliance or security best practices
- Dealing with complex or uncommon configurations

## Troubleshooting

### Provider Not Working

1. Check that the API key is correctly set:
   ```bash
   echo $OPENAI_API_KEY  # or $GEMINI_API_KEY, etc.
   ```

2. Verify the provider is correctly configured:
   ```bash
   echo $AI_PROVIDER
   ```

3. Check server logs for error messages:
   ```
   [ChatHandler] Initializing with AI provider: <provider>
   [ChatHandler] Failed to create AI provider: <error>
   ```

### Custom Endpoint Issues

For custom endpoints, ensure:
1. The endpoint URL is complete and correct
2. The API follows OpenAI's chat completion format or a simple prompt/response format
3. Authentication headers are properly configured
4. The model name (if required) is correct

### JSON Parsing Errors

If you see errors like "invalid character '`' looking for beginning of value":
- This typically happens when the AI returns JSON wrapped in markdown code blocks
- The system automatically extracts JSON from markdown blocks (```json or ```)
- This is common with Gemini and some other providers
- No action needed - the system handles this automatically

## Response Format Support

The custom provider automatically handles various response formats:

### OpenAI Format
```json
{
  "choices": [{
    "message": {
      "content": "response text"
    }
  }]
}
```

### Simple Formats
```json
{
  "response": "response text"
}
// or
{
  "text": "response text"
}
// or
{
  "content": "response text"
}
```

## Docker Configuration

When using Docker, pass the environment variables:

```bash
docker run -e AI_PROVIDER=gemini -e GEMINI_API_KEY=your-key cel-rpc-server
```

Or in docker-compose.yml:

```yaml
services:
  cel-rpc-server:
    environment:
      - AI_PROVIDER=gemini
      - GEMINI_API_KEY=${GEMINI_API_KEY}
```