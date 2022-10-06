#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
set -o errexit
#set -x

main() {
    #kubectl -n kepler get pod
    #kubectl -n kepler get pods -oname
    #kubectl port-forward $(kubectl -n kepler get pods -oname) 9102:9102 -n kepler -v7 &
    #go test ./e2e/... --race --bench=. -cover --count=1 --vet=all
    build_test_image
    push_test_image
    run_job
}

function build_test_image() {
    docker build -f ./e2e/Dockerfile -t localhost:5001/keplere2etest:latest .
}

function _fetch_kind() {
    mkdir -p ${CONFIG_OUT_DIR}
    KIND="${CONFIG_OUT_DIR}"/.kind
    if [ -f $KIND ]; then
        current_kind_version=$($KIND --version |& awk '{print $3}')
    fi
    if [[ $current_kind_version != $KIND_VERSION ]]; then
        echo "Downloading kind v$KIND_VERSION"
        if [[ "$OSTYPE" == "darwin"* ]]; then
            curl -LSs https://github.com/kubernetes-sigs/kind/releases/download/v$KIND_VERSION/kind-darwin-${ARCH} -o "$KIND"
        else
            curl -LSs https://github.com/kubernetes-sigs/kind/releases/download/v$KIND_VERSION/kind-linux-${ARCH} -o "$KIND"
        fi
        chmod +x "$KIND"
    fi
}

function push_test_image() {
    _fetch_kind
    $KIND load docker-image localhost:5001/keplere2etest:latest
}

function run_job() {
    kubectl apply -f ./e2e job-k8s.yaml
    pods=$(kubectl get pods --selector=job-name=test -n kepler --output=jsonpath='{.items[*].metadata.name}')
    echo $pods
    kubectl logs $pods
}

main
