FROM golang:1.18

COPY . .
RUN go mod tidy

CMD [go test ./e2e/... --race --bench=. -cover --count=1 --vet=all]