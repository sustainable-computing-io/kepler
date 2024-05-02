#!/usr/bin/env bash

set -e

# Version of libbpf to fetch headers from
LIBBPF_VERSION=1.4.1

# The headers we want
prefix=libbpf-"$LIBBPF_VERSION"
headers=(
    "$prefix"/LICENSE.BSD-2-Clause
    "$prefix"/src/bpf_helper_defs.h
    "$prefix"/src/bpf_helpers.h
)

# Fetch libbpf release and extract the desired headers
curl -sL "https://github.com/libbpf/libbpf/archive/refs/tags/v${LIBBPF_VERSION}.tar.gz" | \
    tar -C ./bpfassets/libbpf/include/bpf -xz --xform='s#.*/##' "${headers[@]}"
