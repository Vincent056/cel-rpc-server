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

# Expose the RPC & MCP ports

# Make sure OPENAI_API_KEY is set as well as KUBECONFIG
# ENV OPENAI_API_KEY=${OPENAI_API_KEY}

EXPOSE 8349

# Run the server
ENTRYPOINT ["/usr/local/bin/cel-rpc-server"]
