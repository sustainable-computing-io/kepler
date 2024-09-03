#!/bin/bash

PROJECT_ROOT="$(git rev-parse --show-toplevel)"
PKG_BPF="$PROJECT_ROOT/pkg/bpf/"
PKG_BPFTEST="$PROJECT_ROOT/pkg/bpftest/"

generate() {
# Copy the generated file to a new file
cp kepler_bpfeb.go mockbpf.go

# Modify the build tag of the new file
sed -i 's|//go:build mips \|\| mips64 \|\| ppc64 \|\| s390x|//go:build mockbpf|' mockbpf.go
sed -i '3i\
// +build mockbpf
' mockbpf.go

sed -i '/_ "embed"/d' mockbpf.go
sed -i '/^\/\/go:embed kepler_bpfeb.o/d' mockbpf.go
gofmt -w mockbpf.go
}

test() {
# Copy the generated file to a new file
cp test_bpfeb.go mocktest.go

# Modify the build tag of the new file
sed -i 's|//go:build mips \|\| mips64 \|\| ppc64 \|\| s390x|//go:build mockbpf|' mocktest.go
sed -i '3i\
// +build mockbpf
' mocktest.go

sed -i '/_ "embed"/d' mocktest.go
sed -i '/^\/\/go:embed test_bpfeb.o/d' mocktest.go
gofmt -w mocktest.go
}

clean() {
# Remove generated build tags when the real bpf2go files are generate
sed -i '/^\/\/go:build mockbpf/d' "$PKG_BPF/mockbpf.go"
sed -i '/^\/\/ +build mockbpf/d' "$PKG_BPF/mockbpf.go"
sed -i '/^\/\/go:build mockbpf/d' "$PKG_BPFTEST/mocktest.go"
sed -i '/^\/\/ +build mockbpf/d' "$PKG_BPFTEST/mocktest.go"
}

main() {
	local ret=0
	case "${1-}" in
	generate)
		generate
		;;
    test)
		test
		;;
    clean)
		clean
		;;
	*)
		die "invalid args"
		;;
	esac
	return $ret
}

main "$@"
