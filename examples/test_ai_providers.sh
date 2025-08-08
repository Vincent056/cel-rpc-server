#!/bin/bash

# Example script to test different AI providers with the CEL RPC server

echo "CEL RPC Server - AI Provider Test Script"
echo "========================================"
echo ""

# Function to run the server with different providers
test_provider() {
    local provider=$1
    echo "Testing $provider provider..."
    echo ""
    
    case $provider in
        openai)
            export AI_PROVIDER=openai
            export OPENAI_API_KEY=${OPENAI_API_KEY:-"your-openai-key"}
            export OPENAI_MODEL=${OPENAI_MODEL:-"gpt-4"}
            ;;
        gemini)
            export AI_PROVIDER=gemini
            export GEMINI_API_KEY=${GEMINI_API_KEY:-"your-gemini-key"}
            export GEMINI_MODEL=${GEMINI_MODEL:-"gemini-2.5-pro"}
            ;;
        custom_local)
            export AI_PROVIDER=custom
            export CUSTOM_AI_ENDPOINT="http://localhost:1234/v1/chat/completions"
            export CUSTOM_AI_MODEL="llama-2-7b-chat"
            # No API key needed for local models
            unset CUSTOM_AI_API_KEY
            ;;
        custom_claude)
            export AI_PROVIDER=custom
            export CUSTOM_AI_ENDPOINT="https://api.anthropic.com/v1/messages"
            export CUSTOM_AI_API_KEY=${CLAUDE_API_KEY:-"your-claude-key"}
            export CUSTOM_AI_HEADERS="anthropic-version:2023-06-01"
            ;;
        *)
            echo "Unknown provider: $provider"
            return 1
            ;;
    esac
    
    echo "Configuration:"
    echo "  AI_PROVIDER=$AI_PROVIDER"
    echo "  Endpoint: ${CUSTOM_AI_ENDPOINT:-default}"
    echo "  Model: ${OPENAI_MODEL}${GEMINI_MODEL}${CUSTOM_AI_MODEL}"
    echo ""
    
    # Start the server (for 30 seconds to test)
    echo "Starting server..."
    timeout 30 ../cmd/server/server 2>&1 | grep -E "(AI provider|ChatHandler)" &
    
    # Wait a bit for server to start
    sleep 5
    
    # Test with a simple request (you'd need to implement a client call here)
    echo "Server should be running with $provider provider"
    echo ""
    
    # Wait for timeout
    wait
    
    echo "Test completed for $provider"
    echo "----------------------------------------"
    echo ""
}

# Main menu
if [ $# -eq 0 ]; then
    echo "Usage: $0 <provider>"
    echo ""
    echo "Available providers:"
    echo "  openai        - OpenAI GPT models"
    echo "  gemini        - Google Gemini models"
    echo "  custom_local  - Local LLM (e.g., LM Studio, Ollama)"
    echo "  custom_claude - Anthropic Claude via custom endpoint"
    echo "  all           - Test all providers"
    echo ""
    exit 1
fi

case $1 in
    all)
        test_provider openai
        test_provider gemini
        test_provider custom_local
        test_provider custom_claude
        ;;
    *)
        test_provider $1
        ;;
esac

echo "All tests completed!"