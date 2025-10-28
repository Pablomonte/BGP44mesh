# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go.mod and go.sum first for better caching
COPY daemon-go/go.mod daemon-go/go.sum ./
RUN go mod download

# Copy source code
COPY daemon-go/ ./

# Build the daemon
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o bgp-daemon ./cmd/bgp-daemon

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    tinc \
    ca-certificates \
    bash

# Create tinc directories
RUN mkdir -p /var/run/tinc

# Copy daemon binary from builder
COPY --from=builder /build/bgp-daemon /usr/local/bin/bgp-daemon

# Make it executable
RUN chmod +x /usr/local/bin/bgp-daemon

# Default command
CMD ["/usr/local/bin/bgp-daemon"]
