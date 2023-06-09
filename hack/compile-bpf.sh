#!/bin/bash
UNAME=$(uname -r)
ARCH=x86
DIR=$(dirname $0)
OUTPUT=${OUTPUT:-"main.bc"}
VMLINUX_H=${VMLINUX_H:-"https://raw.githubusercontent.com/iovisor/bcc/master/libbpf-tools/x86/vmlinux_518.h"}
if [ ! -f ${DIR}/../bpfassets/perf_event/perf_event.c ]; then
    echo "perf_event.c not found"
    exit 1
fi
if [ ! -f ${DIR}/../_output/bpf/vmlinux.h ]; then
    echo "downloading vmlinux.h"
    mkdir -p ${DIR}/../_output/bpf
    curl -s ${VMLINUX_H} -o ${DIR}/../_output/bpf/vmlinux.h
fi
clang -cc1 -triple x86_64-unknown-linux-gnu -emit-llvm-bc -emit-llvm-uselists -disable-free \
 -clear-ast-before-backend -disable-llvm-verifier -discard-value-names  -mrelocation-model static -fno-jump-tables \
 -mframe-pointer=none -fmath-errno -ffp-contract=on -fno-rounding-math -mconstructor-aliases -target-cpu x86-64 -tune-cpu generic \
 -mllvm -treat-scalable-fixed-error-as-warning -debug-info-kind=constructor -dwarf-version=5 -debugger-tuning=gdb \
 -fcoverage-compilation-dir=/usr/src/kernels/${UNAME} -nostdsysteminc -nobuiltininc -resource-dir lib/clang/14.0.6 \
 -Wno-unknown-attributes \
 -D __BPF_TRACING__ \
 -I `pwd`/_output/bpf -I /lib/modules/${UNAME}/build/arch/${ARCH}/include/uapi -I /lib/modules/${UNAME}/build/include/uapi \
 -I /lib/modules/${UNAME}/build/arch/${ARCH}/include/generated  \
 -I /lib/modules/${UNAME}/build/arch/${ARCH}/include/generated/uapi \
 -I /lib/modules/${UNAME}/build/include/generated/uapi \
 -I /usr/include \
 -D __KERNEL__ -D KBUILD_MODNAME="bcc" \
 -D MAP_SIZE=10240 -D NUM_CPUS=256 -D SET_GROUP_ID -D NUMCPUS=256 -O2 \
 -Wno-deprecated-declarations -Wno-gnu-variable-sized-type-not-at-end -Wno-pragma-once-outside-header \
 -Wno-address-of-packed-member -Wno-unknown-warning-option -Wno-unused-value -Wno-pointer-sign \
 -fdebug-compilation-dir=/usr/src/kernels/${UNAME} -ferror-limit 19 -fgnuc-version=4.2.1 \
 -vectorize-loops -vectorize-slp -faddrsig -D__GCC_HAVE_DWARF2_CFI_ASM=1 \
-main-file-name perf_event.c -x c ${DIR}/../bpfassets/perf_event/perf_event.c -o ${OUTPUT}
