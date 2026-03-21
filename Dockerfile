# Build the binary
FROM --platform=$BUILDPLATFORM golang:1.24 AS builder

# Build arguments for binary
ARG VERSION
ARG GIT_COMMIT
ARG GIT_BRANCH
ARG TARGETARCH
ARG BUILDARCH

# Install cross-compiler if building for a different architecture
RUN if [ "$BUILDARCH" = "$TARGETARCH" ]; then \
      echo "Native build, no cross-compiler needed"; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
      apt-get update && apt-get install -y --no-install-recommends \
        gcc-aarch64-linux-gnu libc6-dev-arm64-cross && rm -rf /var/lib/apt/lists/*; \
    elif [ "$TARGETARCH" = "amd64" ]; then \
      apt-get update && apt-get install -y --no-install-recommends \
        gcc-x86-64-linux-gnu libc6-dev-amd64-cross && rm -rf /var/lib/apt/lists/*; \
    fi

WORKDIR /workspace

COPY . .

RUN CROSS_CC=""; \
    if [ "$TARGETARCH" = "arm64" ] && [ "$BUILDARCH" != "arm64" ]; then \
      CROSS_CC=aarch64-linux-gnu-gcc; \
    elif [ "$TARGETARCH" = "amd64" ] && [ "$BUILDARCH" != "amd64" ]; then \
      CROSS_CC=x86_64-linux-gnu-gcc; \
    fi; \
    make build \
      CGO_ENABLED=1 \
      PRODUCTION=1 \
      GOARCH=${TARGETARCH} \
      ${CROSS_CC:+CC=${CROSS_CC}} \
      VERSION=${VERSION} \
      GIT_COMMIT=${GIT_COMMIT} \
      GIT_BRANCH=${GIT_BRANCH}

FROM registry.access.redhat.com/ubi9:latest

# Build arguments for labels
ARG VERSION
ARG GIT_COMMIT
ARG BUILD_TIME

LABEL org.opencontainers.image.created=${BUILD_TIME} \
  org.opencontainers.image.source="https://github.com/sustainable-computing-io/kepler" \
  org.opencontainers.image.version=${VERSION} \
  org.opencontainers.image.revision=${GIT_COMMIT} \
  org.opencontainers.image.licenses="Apache-2.0" \
  org.opencontainers.image.vendor="sustainable-computing-io" \
  org.opencontainers.image.title="Kepler" \
  org.opencontainers.image.documentation="https://sustainable-computing.io/" \
  org.opencontainers.image.description="Kepler (Kubernetes-based Efficient Power Level Exporter) is a Prometheus exporter that measures energy consumption metrics at the container, pod, and node levels in Kubernetes clusters"

COPY --from=builder /workspace/bin/kepler-release /usr/bin/kepler

ENTRYPOINT ["/usr/bin/kepler"]
