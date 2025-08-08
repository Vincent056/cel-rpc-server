# Build stage
FROM golang:1.24 AS builder

# Set working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .


# Build the binary (statically linked for minimal final image)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /cel-rpc-server ./cmd/server

# --- Runtime use ubi as base image
FROM registry.access.redhat.com/ubi9/ubi:latest

# Copy binary from the builder stage
COPY --from=builder /cel-rpc-server /usr/local/bin/cel-rpc-server

# Create a non-root user to run the application
RUN useradd -u 1001 -m -s /bin/bash celuser

# Expose the RPC & MCP ports
# Premake the KUBECONFIG directory with proper permissions
RUN mkdir -p /KUBECONFIG && \
    touch /KUBECONFIG/kubeconfig && \
    chown -R celuser:celuser /KUBECONFIG && \
    chmod 755 /KUBECONFIG && \
    chmod 600 /KUBECONFIG/kubeconfig

# Make sure KUBECONFIG is set
ENV KUBECONFIG=/KUBECONFIG/kubeconfig
ENV OPENAI_API_KEY=

# Create working directory for the application and rules library
RUN mkdir -p /home/celuser/app && \
    mkdir -p /home/celuser/app/rules-library && \
    chown -R celuser:celuser /home/celuser/app

# Switch to non-root user
USER celuser

# Set working directory
WORKDIR /home/celuser/app

EXPOSE 8349

# Run the server
ENTRYPOINT ["/usr/local/bin/cel-rpc-server"]
