# go get -u github.com/go-bindata/go-bindata/...
GOPATH=$(go env GOPATH)
for i in perf_event; do
	OUTPUT="pkg/bpfassets/${i}_bindata.go"
	${GOPATH}/bin/go-bindata -nocompress -nometadata -prefix "pkg/bpfassets" -pkg bpfassets -o ${OUTPUT} "./bpfassets/${i}/..."
	gofmt -s -w "${OUTPUT}"
done
