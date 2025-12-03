# Build the binary
FROM golang:1.24 AS builder

# Build arguments for binary
ARG VERSION
ARG GIT_COMMIT
ARG GIT_BRANCH

WORKDIR /workspace

COPY . .

RUN make build \
  PRODUCTION=1 \
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
  org.opencontainers.image.licenses="Apache-2.0 AND GPL-2.0-only AND BSD-2-Clause" \
  org.opencontainers.image.vendor="sustainable-computing-io" \
  org.opencontainers.image.title="Kepler" \
  org.opencontainers.image.documentation="https://sustainable-computing.io/" \
  org.opencontainers.image.description="Kepler (Kubernetes-based Efficient Power Level Exporter) is a Prometheus exporter that measures energy consumption metrics at the container, pod, and node levels in Kubernetes clusters"

COPY --from=builder /workspace/bin/kepler-release /usr/bin/kepler

ENTRYPOINT ["/usr/bin/kepler"]
