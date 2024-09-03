package bpftest

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@v0.15.0 test test.bpf.c -- -I../../bpf/ -I../../bpf/include
//go:generate ../../hack/bpf-generate.sh test
