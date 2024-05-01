ARCH=$(shell uname -m)
GOARCH=$(shell go env GOARCH)

TARGET := kepler
TARGET_BPF := bpf.o/$(GOARCH)_$(TARGET).bpf.o
BPF_SRC := $(wildcard src/*.bpf.c src/*.bpf.h)
# in libbpf if $(ARCH) is x86_64, then set TARGET_ARCH to x86. if arch is aarch64, then set TARGET_ARCH to arm
TARGET_ARCH := $(shell echo $(ARCH) | sed -e 's/x86_64/x86/' -e 's/aarch64/arm64/' -e 's/s390x/s390/')

default: $(TARGET_BPF)

bpf.o:
	mkdir -p bpf.o

$(TARGET_BPF): $(BPF_SRC) | bpf.o
	clang \
		-I./include \
		-D __TARGET_ARCH_$(TARGET_ARCH) \
		-O2 -g -c -target bpf \
		-o $@ $<
