# Google Gemini Model Information

## Current Available Models (as of January 2025)

Google Gemini currently offers these models through their API:

### Gemini 2.5 (Latest Generation)
- **`gemini-2.5-flash`** - Latest flash model, very fast and efficient (default)
- **`gemini-2.5-pro`** - Latest pro model, higher quality but slower

### Gemini 2.0 (Previous Generation)
- **`gemini-2.0-flash-exp`** - Experimental flash model
- **`gemini-2.0-pro-exp`** - Experimental pro model

### Gemini 1.5 (Stable)
- **`gemini-1.5-flash`** - Fast, efficient model for most use cases
- **`gemini-1.5-pro`** - Higher quality, more capable but slower

### Gemini 1.0 (Legacy)
- **`gemini-pro`** - Being phased out, may not be available

## Choosing a Model

For the CEL RPC server:
- Use `gemini-2.5-flash` for fastest responses (default)
- Use `gemini-2.5-pro` for more complex rule generation
- Use `gemini-1.5-flash` if you need older stable version
- Use `gemini-1.5-pro` for stable, high-quality responses

## Configuration

```bash
# Use latest fast model (default)
export GEMINI_MODEL=gemini-2.5-flash

# Use latest high-quality model
export GEMINI_MODEL=gemini-2.5-pro

# Use previous generation models
export GEMINI_MODEL=gemini-2.0-flash-exp
export GEMINI_MODEL=gemini-2.0-pro-exp

# Use stable models
export GEMINI_MODEL=gemini-1.5-flash
export GEMINI_MODEL=gemini-1.5-pro
```

## Troubleshooting

If you get a "model not found" error, try:
1. Check the exact model name (case-sensitive)
2. Use a stable model like `gemini-1.5-flash`
3. Check Google's documentation for the latest available models

## API Differences

The Gemini API has some differences from OpenAI:
- Roles are `user` and `model` (not `system` and `assistant`)
- System prompts must be combined with user messages
- Response format may vary slightly