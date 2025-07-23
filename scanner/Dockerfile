FROM golang:1.24.3-alpine AS builder

# Install dependencies
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the scanner
RUN go build -o /cel-scanner ./scanner/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /cel-scanner /cel-scanner

ENTRYPOINT ["/cel-scanner"] 