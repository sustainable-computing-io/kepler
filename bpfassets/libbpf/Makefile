ARCH=$(shell uname -m)
GOARCH=$(shell go env GOARCH)

TARGET := kepler
TARGET_BPF := $(TARGET).bpf.o
BPF_SRC := bpfassets/libbpf/src/*.bpf.c

$(TARGET_BPF): $(BPF_SRC)
	clang \
		-I /usr/include/$(ARCH)-linux-gnu \
		-O2 -g -c -target bpf \
		-o bpfassets/libbpf/bpf.o/$(GOARCH)_$@ $<