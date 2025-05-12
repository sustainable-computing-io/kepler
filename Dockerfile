# Build the binary
FROM golang:1.23 AS builder

WORKDIR /workspace

COPY . .

RUN make build PRODUCTION=1

FROM registry.access.redhat.com/ubi9:latest

COPY --from=builder /workspace/bin/kepler-release /usr/bin/kepler

ENTRYPOINT ["/usr/bin/kepler"]
