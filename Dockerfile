# Build the binary
FROM golang:1.24 AS builder

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

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /workspace/bin/kepler-release /usr/bin/kepler
ENTRYPOINT ["/usr/bin/kepler"]
