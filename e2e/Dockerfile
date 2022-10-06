FROM golang:1.18

COPY . .

CMD [go test ./e2e/... --race --bench=. -cover --count=1 --vet=all]