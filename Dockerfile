# Build the binary
FROM golang:1.23 AS builder

WORKDIR /workspace

COPY . .

RUN make build

FROM registry.access.redhat.com/ubi9:latest

COPY --from=builder /workspace/bin/kepler /usr/bin/kepler

ENTRYPOINT ["/usr/bin/kepler"]
