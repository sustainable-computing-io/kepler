ARCH=$(shell uname -m)
GOARCH=$(shell go env GOARCH)

TARGET := kepler
TARGET_BPF := $(TARGET).bpf.o
BPF_SRC := bpfassets/libbpf/src/*.bpf.c
# in libbpf if $(ARCH) is x86_64, then set TARGET_ARCH to x86. if arch is aarch64, then set TARGET_ARCH to arm
TARGET_ARCH := $(shell echo $(ARCH) | sed -e 's/x86_64/x86/' -e 's/aarch64/arm64/' -e 's/s390x/s390/')

$(TARGET_BPF): $(BPF_SRC)
	clang \
		-I /usr/include/$(ARCH)-linux-gnu \
		-D __TARGET_ARCH_$(TARGET_ARCH) \
		-O2 -g -c -target bpf \
		-o bpfassets/libbpf/bpf.o/$(GOARCH)_$@ $<
