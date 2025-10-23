# Multi-stage Dockerfile for fan-controller
# Build stage: Compile Go application
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /build

# Copy go.mod and go.sum first for better layer caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o fan-control .

# Runtime stage: Minimal Alpine image with required tools
FROM alpine:3.19

# Install required packages
RUN apk --no-cache add \
    ipmitool \
    smartmontools \
    wget \
    tzdata

# Create non-root user for security
RUN addgroup -g 1000 fancontrol && \
    adduser -D -u 1000 -G fancontrol fancontrol

# Copy binary from builder stage
COPY --from=builder /build/fan-control /usr/local/bin/fan-control

# Copy default configuration
COPY config.yaml /config/config.yaml

# Set proper permissions
RUN chmod +x /usr/local/bin/fan-control && \
    chown -R fancontrol:fancontrol /config

# Create health check script
RUN echo '#!/bin/sh' > /usr/local/bin/healthcheck.sh && \
    echo 'wget --quiet --tries=1 --spider http://localhost:9090/health || exit 1' >> /usr/local/bin/healthcheck.sh && \
    chmod +x /usr/local/bin/healthcheck.sh

# Expose metrics port
EXPOSE 9090

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD /usr/local/bin/healthcheck.sh

# Switch to non-root user
USER fancontrol

# Set entrypoint and default command
ENTRYPOINT ["/usr/local/bin/fan-control"]
CMD ["--config", "/config/config.yaml"]
