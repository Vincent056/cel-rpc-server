# AI Provider Timeout Configuration

## Issue

When using large language models like `qwen3:32b` with Ollama, you may encounter `context deadline exceeded` errors. This happens because:

1. Large models take longer to generate responses
2. The default timeouts are optimized for cloud-based APIs (OpenAI, Gemini)
3. Local models running on limited hardware need more time

## Current Timeout Settings

After the recent updates:

- **HTTP Client Timeout**: 300 seconds (5 minutes)
- **Context Timeout**: Inherited from parent context (usually 30 seconds)

## Recommendations

### For Large Local Models (Ollama)

1. **Use smaller models if possible**:
   ```bash
   export CUSTOM_AI_MODEL=llama2:7b    # Faster than 32b models
   export CUSTOM_AI_MODEL=mistral:7b   # Good balance of speed and quality
   ```

2. **Increase hardware resources**:
   - Ensure Ollama has enough RAM allocated
   - Use GPU acceleration if available

3. **Configure Ollama for better performance**:
   ```bash
   # Set number of parallel requests
   export OLLAMA_NUM_PARALLEL=1
   
   # Increase context window if needed
   export OLLAMA_NUM_CTX=4096
   ```

### For Production Use

1. **Use cloud-based APIs** for better reliability:
   ```bash
   # OpenAI (fastest)
   export AI_PROVIDER=openai
   export OPENAI_API_KEY=your-key
   
   # Gemini (good alternative)
   export AI_PROVIDER=gemini
   export GEMINI_API_KEY=your-key
   ```

## Debugging Timeout Issues

1. **Check Ollama status**:
   ```bash
   curl http://localhost:11434/api/tags
   ```

2. **Test model directly**:
   ```bash
   curl -X POST http://localhost:11434/api/generate \
     -d '{"model": "qwen3:32b", "prompt": "test", "stream": false}'
   ```

3. **Monitor resource usage**:
   ```bash
   # Check if Ollama is CPU-bound
   top -p $(pgrep ollama)
   ```

## Future Improvements

To better handle large models, consider:

1. Implementing streaming responses
2. Adding configurable timeouts via environment variables
3. Implementing retry logic with exponential backoff
4. Adding model-specific timeout configurations