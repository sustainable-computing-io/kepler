package bpf

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@v0.15.0 kepler kepler.bpf.c -- -I../../bpf/ -I../../bpf/include
//go:generate ../../hack/bpf-generate.sh generate
