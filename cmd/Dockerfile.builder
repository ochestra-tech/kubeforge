# KubeForge Builder - produces a standalone binary for host execution
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make bash

# Set working directory
WORKDIR /src

# Copy source code
COPY . .

# Download dependencies
RUN go mod download

# Build the application for different platforms
RUN mkdir -p /dist && \
    # Linux AMD64
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /dist/kubeforge-linux-amd64 cmd/kubeforge/main.go && \
    # Linux ARM64
    GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /dist/kubeforge-linux-arm64 cmd/kubeforge/main.go

# Copy assets directory for runtime use
RUN mkdir -p /dist/assets && \
    cp -r assets/* /dist/assets/

# Create install script
RUN echo '#!/bin/bash\n\
    ARCH=$(uname -m)\n\
    if [ "$ARCH" = "x86_64" ]; then\n\
    BINARY="kubeforge-linux-amd64"\n\
    elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then\n\
    BINARY="kubeforge-linux-arm64"\n\
    else\n\
    echo "Unsupported architecture: $ARCH"\n\
    exit 1\n\
    fi\n\
    \n\
    echo "Installing KubeForge for $ARCH..."\n\
    mkdir -p /usr/local/lib/kubeforge/assets\n\
    cp -r assets/* /usr/local/lib/kubeforge/assets/\n\
    cp $BINARY /usr/local/bin/kubeforge\n\
    chmod +x /usr/local/bin/kubeforge\n\
    echo "KubeForge installed successfully."\n\
    ' > /dist/install.sh && chmod +x /dist/install.sh

# Minimal runtime container for distribution (if needed)
FROM alpine:3.18 AS runtime
COPY --from=builder /dist /kubeforge
ENTRYPOINT ["/bin/sh"]