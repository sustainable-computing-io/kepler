ARCH=$(shell uname -m)

TARGET := kepler
TARGET_BPF := $(TARGET).bpf.o
BPF_SRC := bpfassets/libbpf/src/*.bpf.c

$(TARGET_BPF): $(BPF_SRC)
	clang \
		-I /usr/include/$(ARCH)-linux-gnu \
		-O2 -c -target bpf \
		-o bpfassets/libbpf/bpf.o/$(ARCH)_$@ $<