#!/bin/bash

# Runtime kubeconfig setup utility
# This script runs inside the container to set up kubeconfig after startup

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local color=$1
    local message=$2
    echo -e "${color}[KUBECONFIG-SETUP]${NC} $message"
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [OPTION]"
    echo
    echo "Set up kubeconfig for CEL RPC Server running in container"
    echo
    echo "Options:"
    echo "  --from-file PATH        Copy kubeconfig from file path"
    echo "  --from-stdin           Read kubeconfig from stdin"
    echo "  --from-command CMD     Execute command to get kubeconfig"
    echo "  --test                 Test current kubeconfig"
    echo "  --status               Show current kubeconfig status"
    echo "  --clear                Clear current kubeconfig"
    echo "  -h, --help             Show this help"
    echo
    echo "Examples:"
    echo "  # From host file (after copying to container)"
    echo "  $0 --from-file /tmp/my-kubeconfig"
    echo
    echo "  # From stdin (paste content)"
    echo "  $0 --from-stdin"
    echo
    echo "  # From kubectl command"
    echo "  $0 --from-command 'kubectl config view --raw'"
    echo
    echo "  # Test current setup"
    echo "  $0 --test"
    echo
}

# Function to validate kubeconfig content
validate_kubeconfig() {
    local config_file="$1"
    
    if [ ! -f "$config_file" ]; then
        print_status $RED "Config file not found: $config_file"
        return 1
    fi
    
    # Basic YAML/JSON validation
    if command -v kubectl >/dev/null 2>&1; then
        if kubectl config view --kubeconfig="$config_file" >/dev/null 2>&1; then
            print_status $GREEN "Kubeconfig validation successful"
            return 0
        else
            print_status $RED "Invalid kubeconfig format"
            return 1
        fi
    else
        # Fallback: check if it looks like YAML/JSON
        if grep -q "apiVersion\|clusters\|contexts\|users" "$config_file" 2>/dev/null; then
            print_status $YELLOW "Basic validation passed (kubectl not available for full validation)"
            return 0
        else
            print_status $RED "File doesn't appear to be a valid kubeconfig"
            return 1
        fi
    fi
}

# Function to setup kubeconfig from file
setup_from_file() {
    local source_file="$1"
    
    if [ ! -f "$source_file" ]; then
        print_status $RED "Source file not found: $source_file"
        return 1
    fi
    
    print_status $BLUE "Setting up kubeconfig from file: $source_file"
    
    # Copy to standard location
    cp "$source_file" /tmp/kubeconfig
    chmod 600 /tmp/kubeconfig
    chown celuser:celuser /tmp/kubeconfig 2>/dev/null || true
    
    # Validate
    if validate_kubeconfig /tmp/kubeconfig; then
        export KUBECONFIG=/tmp/kubeconfig
        print_status $GREEN "Kubeconfig setup complete"
        print_status $BLUE "KUBECONFIG is now set to: $KUBECONFIG"
        
        # Update environment for the running process if possible
        echo "export KUBECONFIG=/tmp/kubeconfig" > /tmp/kubeconfig-env
        
        return 0
    else
        rm -f /tmp/kubeconfig
        return 1
    fi
}

# Function to setup kubeconfig from stdin
setup_from_stdin() {
    print_status $BLUE "Reading kubeconfig from stdin..."
    print_status $YELLOW "Paste your kubeconfig content below, then press Ctrl+D:"
    
    # Read from stdin
    cat > /tmp/kubeconfig-input
    
    if [ -s /tmp/kubeconfig-input ]; then
        # Validate and set up
        if validate_kubeconfig /tmp/kubeconfig-input; then
            mv /tmp/kubeconfig-input /tmp/kubeconfig
            chmod 600 /tmp/kubeconfig
            chown celuser:celuser /tmp/kubeconfig 2>/dev/null || true
            
            export KUBECONFIG=/tmp/kubeconfig
            print_status $GREEN "Kubeconfig setup complete from stdin"
            print_status $BLUE "KUBECONFIG is now set to: $KUBECONFIG"
            
            echo "export KUBECONFIG=/tmp/kubeconfig" > /tmp/kubeconfig-env
            return 0
        else
            rm -f /tmp/kubeconfig-input
            return 1
        fi
    else
        print_status $RED "No input received"
        rm -f /tmp/kubeconfig-input
        return 1
    fi
}

# Function to setup kubeconfig from command
setup_from_command() {
    local command="$1"
    
    print_status $BLUE "Executing command: $command"
    
    # Execute command and capture output
    if eval "$command" > /tmp/kubeconfig-cmd 2>/dev/null; then
        if [ -s /tmp/kubeconfig-cmd ]; then
            # Validate and set up
            if validate_kubeconfig /tmp/kubeconfig-cmd; then
                mv /tmp/kubeconfig-cmd /tmp/kubeconfig
                chmod 600 /tmp/kubeconfig
                chown celuser:celuser /tmp/kubeconfig 2>/dev/null || true
                
                export KUBECONFIG=/tmp/kubeconfig
                print_status $GREEN "Kubeconfig setup complete from command"
                print_status $BLUE "KUBECONFIG is now set to: $KUBECONFIG"
                
                echo "export KUBECONFIG=/tmp/kubeconfig" > /tmp/kubeconfig-env
                return 0
            else
                rm -f /tmp/kubeconfig-cmd
                return 1
            fi
        else
            print_status $RED "Command produced no output"
            rm -f /tmp/kubeconfig-cmd
            return 1
        fi
    else
        print_status $RED "Command failed"
        rm -f /tmp/kubeconfig-cmd
        return 1
    fi
}

# Function to test current kubeconfig
test_kubeconfig() {
    # Check if runtime kubeconfig exists first
    local config_file="/tmp/kubeconfig"
    if [ ! -f "$config_file" ]; then
        config_file="${KUBECONFIG:-/KUBECONFIG/kubeconfig}"
    fi
    
    print_status $BLUE "Testing kubeconfig: $config_file"
    
    if [ ! -f "$config_file" ]; then
        print_status $RED "No kubeconfig found at $config_file"
        return 1
    fi
    
    # Test with kubectl if available
    if command -v kubectl >/dev/null 2>&1; then
        print_status $BLUE "Testing cluster connectivity..."
        
        if kubectl --kubeconfig="$config_file" cluster-info >/dev/null 2>&1; then
            print_status $GREEN "✓ Cluster connectivity test passed"
        else
            print_status $YELLOW "⚠ Cluster connectivity test failed (cluster may be unreachable)"
        fi
        
        if kubectl --kubeconfig="$config_file" auth can-i get pods >/dev/null 2>&1; then
            print_status $GREEN "✓ Basic permissions test passed"
        else
            print_status $YELLOW "⚠ Limited permissions (may not be able to list pods)"
        fi
        
        # Show current context
        local current_context
        current_context=$(kubectl --kubeconfig="$config_file" config current-context 2>/dev/null || echo "unknown")
        print_status $BLUE "Current context: $current_context"
        
    else
        print_status $YELLOW "kubectl not available - only basic validation performed"
    fi
    
    validate_kubeconfig "$config_file"
}

# Function to show current status
show_status() {
    print_status $BLUE "Kubeconfig Status:"
    echo
    
    if [ -n "$KUBECONFIG" ]; then
        echo "  KUBECONFIG environment variable: $KUBECONFIG"
    else
        echo "  KUBECONFIG environment variable: not set"
    fi
    
    for config_path in "/tmp/kubeconfig" "$HOME/.kube/config" "/KUBECONFIG/kubeconfig"; do
        if [ -f "$config_path" ]; then
            echo "  Found kubeconfig at: $config_path"
            ls -la "$config_path" 2>/dev/null || true
        fi
    done
    
    # Check if server process can find kubeconfig
    if [ -f "/tmp/kubeconfig-env" ]; then
        echo "  Runtime kubeconfig environment: available"
    else
        echo "  Runtime kubeconfig environment: not configured"
    fi
}

# Function to clear kubeconfig
clear_kubeconfig() {
    print_status $BLUE "Clearing kubeconfig..."
    
    rm -f /tmp/kubeconfig
    rm -f /tmp/kubeconfig-env
    unset KUBECONFIG
    
    print_status $GREEN "Kubeconfig cleared"
}

# Main function
main() {
    case "${1:-}" in
        --from-file)
            if [ -z "$2" ]; then
                print_status $RED "File path required"
                show_usage
                exit 1
            fi
            setup_from_file "$2"
            ;;
        --from-stdin)
            setup_from_stdin
            ;;
        --from-command)
            if [ -z "$2" ]; then
                print_status $RED "Command required"
                show_usage
                exit 1
            fi
            setup_from_command "$2"
            ;;
        --test)
            test_kubeconfig
            ;;
        --status)
            show_status
            ;;
        --clear)
            clear_kubeconfig
            ;;
        -h|--help|"")
            show_usage
            ;;
        *)
            print_status $RED "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
}

# Run main function
main "$@"
