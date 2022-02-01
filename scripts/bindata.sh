# go get -u github.com/go-bindata/go-bindata/...
for i in perf_event; do
	OUTPUT="pkg/bpf_assets/${i}_bindata.go"
	${GOPATH}/bin/go-bindata -nocompress -nometadata -prefix "pkg/bpf_assets" -pkg bpf_assets -o ${OUTPUT} "./bpf_assets/${i}/..."
	gofmt -s -w "${OUTPUT}"
done
