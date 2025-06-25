# Build stage
FROM golang:1.24.4-alpine AS builder

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o sigil cmd/sigil/main.go

# Runtime stage
FROM alpine:latest

# Install git (required for Sigil operations)
RUN apk add --no-cache git ca-certificates

# Create non-root user
RUN adduser -D -g '' sigil

# Copy binary from builder
COPY --from=builder /build/sigil /usr/local/bin/sigil

# Set ownership
RUN chown sigil:sigil /usr/local/bin/sigil

# Switch to non-root user
USER sigil

# Set working directory
WORKDIR /workspace

# Expose volume for workspace
VOLUME ["/workspace"]

# Set entrypoint
ENTRYPOINT ["sigil"]

# Default command (show help)
CMD ["--help"]