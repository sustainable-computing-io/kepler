# Build the binary
FROM golang:1.23 AS builder

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

COPY --from=builder /workspace/bin/kepler-release /usr/bin/kepler

ENTRYPOINT ["/usr/bin/kepler"]
