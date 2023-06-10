#!/bin/bash
UNAME=$(uname -r)
ARCH=x86
DIR=$(dirname $0)
CFILE=${CFILE:-"${DIR}/../bpfassets/perf_event/perf_event.c"}
OUTPUT=${OUTPUT:-"${DIR}/../_output/bpf-modules/perf_event.o"}
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
clang -cc1 -triple x86_64-unknown-linux-gnu -emit-llvm-bc -emit-llvm-uselists \
 -D__KERNEL__ -D__ASM_SYSREG_H -Wno-unused-value -Wno-pointer-sign -Wno-compare-distinct-pointer-types -Wno-unused -Wno-compare-distinct-pointer-types \
 -D __BPF_TRACING__ -D__TARGET_ARCH_x86 \
 -I `pwd`/_output/bpf -I /lib/modules/${UNAME}/build/arch/${ARCH}/include/uapi -I /lib/modules/${UNAME}/build/include/uapi \
 -I /lib/modules/${UNAME}/build/arch/${ARCH}/include/generated  \
 -I /lib/modules/${UNAME}/build/arch/${ARCH}/include/generated/uapi \
 -I /lib/modules/${UNAME}/build/include/generated/uapi \
 -I /usr/include \
 -D __KERNEL__ -D KBUILD_MODNAME="bcc" \
 -D MAP_SIZE=10240 -D NUM_CPUS=256 -D SET_GROUP_ID -D NUMCPUS=256 -O2 \
 -main-file-name perf_event.c -x c ${CFILE} -o - | \
	llc -march=bpf -filetype=obj -o ${OUTPUT}
