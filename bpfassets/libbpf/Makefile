ARCH=$(shell uname -m)
GOARCH=$(shell go env GOARCH)

TARGET := kepler
TARGET_BPF := $(TARGET).bpf.o
BPF_SRC := bpfassets/libbpf/src/*.bpf.c
# in libbpf if $(ARCH) is x86_64, then set TARGET_ARCH to x86
ifeq ($(ARCH),x86_64)
    TARGET_ARCH := x86
else
    TARGET_ARCH := $(ARCH)
endif

$(TARGET_BPF): $(BPF_SRC)
	clang \
		-I /usr/include/$(ARCH)-linux-gnu \
		-D __TARGET_ARCH_$(TARGET_ARCH) \
		-O2 -g -c -target bpf \
		-o bpfassets/libbpf/bpf.o/$(GOARCH)_$@ $<
