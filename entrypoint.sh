#!/bin/bash

# Entrypoint script to handle kubeconfig setup safely without modifying user files

set -e

# Function to log messages
log() {
    echo "[ENTRYPOINT] $1"
}

# Function to safely handle kubeconfig setup
setup_kubeconfig_safely() {
    local source_path="$1"
    local target_path="/tmp/kubeconfig"
    
    if [ -f "$source_path" ]; then
        if [ -r "$source_path" ]; then
            log "Kubeconfig found and readable at $source_path"
            
            # Copy to a temporary location with proper permissions
            cp "$source_path" "$target_path" 2>/dev/null || {
                log "Could not copy kubeconfig, trying alternative approach..."
                return 1
            }
            
            # Set secure permissions on the copy
            chmod 600 "$target_path"
            chown celuser:celuser "$target_path" 2>/dev/null || true
            
            # Update KUBECONFIG to point to our safe copy
            export KUBECONFIG="$target_path"
            log "Using safe kubeconfig copy at $target_path"
            
            # Validate the kubeconfig if kubectl is available
            if command -v kubectl >/dev/null 2>&1; then
                if kubectl config view --kubeconfig="$target_path" >/dev/null 2>&1; then
                    log "Kubeconfig validation successful"
                else
                    log "Warning: Kubeconfig file appears to be invalid"
                fi
            fi
            
            return 0
        else
            log "Warning: Kubeconfig file exists at $source_path but is not readable"
            log "Consider running: chmod 644 $source_path (on the host)"
            return 1
        fi
    else
        log "No kubeconfig found at $source_path - application will run in mock mode"
        return 1
    fi
}

# Main entrypoint logic
main() {
    log "Starting CEL RPC Server..."
    
    # Handle kubeconfig setup safely
    KUBECONFIG_SOURCE="${KUBECONFIG:-/KUBECONFIG/kubeconfig}"
    log "Checking for kubeconfig at: $KUBECONFIG_SOURCE"
    
    # Try to set up kubeconfig safely (without modifying user files)
    if setup_kubeconfig_safely "$KUBECONFIG_SOURCE"; then
        log "Kubeconfig setup successful"
    else
        # Try alternative locations
        for alt_path in "/KUBECONFIG/kubeconfig" "/root/.kube/config" "/home/celuser/.kube/config"; do
            if [ "$alt_path" != "$KUBECONFIG_SOURCE" ]; then
                log "Trying alternative kubeconfig location: $alt_path"
                if setup_kubeconfig_safely "$alt_path"; then
                    break
                fi
            fi
        done
    fi
    
    # Check for runtime kubeconfig setup
    if [ -f "/tmp/kubeconfig-env" ]; then
        log "Loading runtime kubeconfig environment..."
        source /tmp/kubeconfig-env
    fi
    
    # Create a background script to monitor for runtime kubeconfig changes
    cat > /tmp/monitor-kubeconfig.sh << 'EOF'
#!/bin/bash
while true; do
    if [ -f "/tmp/kubeconfig-env" ] && [ -f "/tmp/kubeconfig" ]; then
        # Export the kubeconfig environment for any new processes
        export KUBECONFIG=/tmp/kubeconfig
        # Signal that kubeconfig is available (the server can check this)
        touch /tmp/kubeconfig-ready
    else
        rm -f /tmp/kubeconfig-ready
    fi
    sleep 5
done
EOF
    chmod +x /tmp/monitor-kubeconfig.sh
    /tmp/monitor-kubeconfig.sh &
    
    log "Environment setup complete"
    log "KUBECONFIG=${KUBECONFIG:-not set - using mock mode}"
    log "AI_PROVIDER=${AI_PROVIDER:-openai}"
    
    if [ -z "$KUBECONFIG" ] || [ ! -f "$KUBECONFIG" ]; then
        log "No kubeconfig detected. To set up after startup, run:"
        log "  podman exec cel-rpc-server setup-kubeconfig --help"
    fi
    
    # Execute the main application
    log "Executing: $@"
    exec "$@"
}

# Run main function with all arguments
main "$@"
