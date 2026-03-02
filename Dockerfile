# ============================================================================
# NGINX Operator - Multi-stage Dockerfile
# ============================================================================
# Builds a minimal, secure container image for the NGINX Operator.
# Uses distroless base for minimal attack surface.
#
# Build args:
#   VERSION    - Semantic version (e.g., 1.0.0)
#   GIT_COMMIT - Git commit SHA
#   BUILD_DATE - ISO 8601 build timestamp
#
# Usage:
#   docker build --build-arg VERSION=1.0.0 -t nginx-operator:1.0.0 .
# ============================================================================

# --- Stage 1: Build ---
FROM golang:1.26-alpine AS builder

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /workspace

# Cache dependencies first (Docker layer caching optimization)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the operator binary with security flags
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-s -w \
      -X github.com/devops-click/nginx-operator/internal/version.Version=${VERSION} \
      -X github.com/devops-click/nginx-operator/internal/version.GitCommit=${GIT_COMMIT} \
      -X github.com/devops-click/nginx-operator/internal/version.BuildDate=${BUILD_DATE}" \
    -trimpath \
    -o /workspace/bin/operator \
    ./cmd/operator/

# --- Stage 2: Runtime ---
# Using distroless for minimal attack surface (no shell, no package manager)
FROM gcr.io/distroless/static:nonroot

LABEL org.opencontainers.image.title="NGINX Operator" \
      org.opencontainers.image.description="Kubernetes operator for managing NGINX servers" \
      org.opencontainers.image.source="https://github.com/devops-click/nginx-operator" \
      org.opencontainers.image.vendor="DevOps Click" \
      org.opencontainers.image.licenses="Apache-2.0"

WORKDIR /

# Copy the binary from builder
COPY --from=builder /workspace/bin/operator /operator

# Copy timezone data for proper time handling
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Run as non-root user (UID 65532 is the nonroot user in distroless)
USER 65532:65532

ENTRYPOINT ["/operator"]
