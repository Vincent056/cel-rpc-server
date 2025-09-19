#!/bin/bash

# Quick start script for CEL RPC Server
# This script provides the simplest way to get started

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    local color=$1
    local message=$2
    echo -e "${color}[QUICK-START]${NC} $message"
}

# Function to detect container engine
detect_container_engine() {
    if command -v podman >/dev/null 2>&1; then
        echo "podman"
    elif command -v docker >/dev/null 2>&1; then
        echo "docker"
    else
        print_status $RED "Neither Podman nor Docker found!"
        exit 1
    fi
}

main() {
    print_status $BLUE "CEL RPC Server - Quick Start"
    echo
    
    # Check for API key
    if [ -z "$OPENAI_API_KEY" ] && [ -z "$GEMINI_API_KEY" ]; then
        print_status $YELLOW "No AI API key detected. Set OPENAI_API_KEY or GEMINI_API_KEY for AI features."
        echo
    fi
    
    # Detect container engine
    CONTAINER_ENGINE=$(detect_container_engine)
    print_status $GREEN "Using $CONTAINER_ENGINE"
    
    # Create rules directory
    mkdir -p ./rules-library
    
    # Start container
    print_status $BLUE "Starting CEL RPC Server..."
    
    local cmd="$CONTAINER_ENGINE run -d --name cel-rpc-server -p 8349:8349"
    
    # Add API key if available
    if [ -n "$OPENAI_API_KEY" ]; then
        cmd="$cmd -e OPENAI_API_KEY=$OPENAI_API_KEY"
    elif [ -n "$GEMINI_API_KEY" ]; then
        cmd="$cmd -e AI_PROVIDER=gemini -e GEMINI_API_KEY=$GEMINI_API_KEY"
    fi
    
    # Add volume mount
    if [ "$CONTAINER_ENGINE" = "podman" ]; then
        cmd="$cmd -v ./rules-library:/home/celuser/app/rules-library:Z"
    else
        cmd="$cmd -v ./rules-library:/home/celuser/app/rules-library:ro"
    fi
    
    cmd="$cmd ghcr.io/vincent056/cel-rpc-server"
    
    print_status $BLUE "Running: $cmd"
    eval "$cmd"
    
    # Wait a moment for container to start
    sleep 2
    
    print_status $GREEN "✓ CEL RPC Server started successfully!"
    echo
    print_status $BLUE "Next steps:"
    echo "1. Set up kubeconfig (choose one):"
    echo "   $CONTAINER_ENGINE exec cel-rpc-server setup-kubeconfig --from-command 'kubectl config view --raw'"
    echo "   $CONTAINER_ENGINE exec -it cel-rpc-server setup-kubeconfig --from-stdin"
    echo
    echo "2. Test the setup:"
    echo "   $CONTAINER_ENGINE exec cel-rpc-server setup-kubeconfig --test"
    echo
    echo "3. Access the server:"
    echo "   Web UI: http://localhost:8349"
    echo "   gRPC: localhost:8349"
    echo
    echo "4. Check logs:"
    echo "   $CONTAINER_ENGINE logs cel-rpc-server"
    echo
    print_status $YELLOW "The server is running in mock mode until you set up kubeconfig."
}

# Show usage if help is requested
if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    echo "Usage: $0"
    echo
    echo "Quick start script for CEL RPC Server"
    echo
    echo "Environment Variables:"
    echo "  OPENAI_API_KEY    OpenAI API key (optional)"
    echo "  GEMINI_API_KEY    Google Gemini API key (optional)"
    echo
    echo "This script will:"
    echo "1. Start the CEL RPC Server container"
    echo "2. Show instructions for kubeconfig setup"
    echo "3. Provide next steps"
    echo
    exit 0
fi

main "$@"
