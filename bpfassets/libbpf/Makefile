ARCH=$(shell uname -m)
GOARCH=$(shell go env GOARCH)

TARGET := kepler

BPF_SRC := $(wildcard src/*.bpf.c src/*.bpf.h)
# in libbpf if $(ARCH) is x86_64, then set TARGET_ARCH to x86. if arch is aarch64, then set TARGET_ARCH to arm
TARGET_ARCH := $(shell echo $(ARCH) | sed -e 's/x86_64/x86/' -e 's/aarch64/arm64/' -e 's/s390x/s390/')

all: bpf.o/kepler.bpfeb.o bpf.o/kepler.bpfel.o

bpf.o:
	mkdir -p bpf.o

bpf.o/kepler.bpfeb.o: $(BPF_SRC) | bpf.o
	clang \
		-I./include \
		-target bpfeb \
		-O2 -g -c  \
		-o $@ $<

bpf.o/kepler.bpfel.o: $(BPF_SRC) | bpf.o
	clang \
		-I./include \
		-target bpfel \
		-O2 -g -c  \
		-o $@ $<
