package bpftest

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@v0.15.0 test ../../bpf/test.bpf.c -- -I../../bpf/include
