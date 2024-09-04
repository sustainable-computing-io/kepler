#!/bin/bash

PROJECT_ROOT="$(git rev-parse --show-toplevel)"
PKG_BPF="$PROJECT_ROOT/pkg/bpf/"
PKG_BPFTEST="$PROJECT_ROOT/pkg/bpftest/"

# The goal of this file is to automate the generation of
# pkg/bpf/mockbpf.go and pkg/bpftest/mocktest.go.
# When the real *_bpfe[lb].go files are generated they are
# copied to these mock files with their build flags modified
# and their go embed directive removed.
# The idea is that when the real *_bpfe[lb].go exist the mock
# files are excluded using a go tag. When the *_bpfe[lb].go
# are cleaned up, the mock files become a placeholder for the
# implementation so that golanglintci is happy.
# OPEN: MUST RUN `make clean` before committing mock files to
# the repo.

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
# Remove generated build tags in mock files when the real bpf2go files are cleaned up.
sed -i '/^\/\/go:build mockbpf/d' "$PKG_BPF/mockbpf.go"
sed -i '/^\/\/ +build mockbpf/d' "$PKG_BPF/mockbpf.go"
sed -i '/^\/\/go:build mockbpf/d' "$PKG_BPFTEST/mocktest.go"
sed -i '/^\/\/ +build mockbpf/d' "$PKG_BPFTEST/mocktest.go"
}

precommit(){
bpf2gofiles=$(git diff --cached --name-only | grep '.*_bpfe[lb]\.go$')
if [ ${#bpf2gofiles[@]} -gt 0 ]; then
    echo "Need to run make clean before committing ${bpf2gofiles[*]}"
    make clean
    return 1
fi
mockfiles=$(git diff --cached --name-only | grep '.*mock.*\.go$')
for filename in "$mockfiles"; do
  if grep -q "mockbpf" "$filename"; then
    echo "Need to run make clean before committing $filename"
    make clean
    return 1
  fi
done
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
    precommit)
        precommit
        ;;
	*)
		die "invalid args"
		;;
	esac
	return $ret
}

main "$@"
