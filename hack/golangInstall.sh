#!/usr/bin/env bash
export CPU_ARCH=$(uname -m |sed -e "s/x86_64/amd64/" |sed -e "s/aarch64/arm64/")
curl -LO https://go.dev/dl/go1.18.1.linux-$CPU_ARCH.tar.gz; mkdir -p /usr/local; tar -C /usr/local -xzf go1.18.1.linux-$CPU_ARCH.tar.gz; rm -f go1.18.1.linux-$CPU_ARCH.tar.gz
