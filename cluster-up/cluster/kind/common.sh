#!/usr/bin/env bash
#
# This file is part of the Kepler project
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at

#     http://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Copyright 2022 The Kepler Contributors
#

set -ex pipefail

_registry_port="5001"
_registry_name="kind-registry"

CONFIG_PATH="cluster-up/cluster"
KIND_VERSION=${KIND_VERSION:-0.12.0}
KIND_MANIFESTS_DIR="$CONFIG_PATH/${CLUSTER_PROVIDER}/manifests"
CLUSTER_NAME=${KIND_CLUSTER_NAME:-kind}
REGISTRY_NAME=${REGISTRY_NAME:-kind-registry}
REGISTRY_PORT=${REGISTRY_PORT:-5001}
KIND_DEFAULT_NETWORK="kind"

IMAGE_REPO=${IMAGE_REPO:-localhost:5001/kepler}
IMAGE_TAG=${IMAGE_TAG:-$(git describe --tags | head -1)}

CONFIG_OUT_DIR=${CONFIG_OUT_DIR:-"_output/manifests/${CLUSTER_PROVIDER}/generated"}
rm -rf ${CONFIG_OUT_DIR}
mkdir -p ${CONFIG_OUT_DIR}

# check CPU arch
PLATFORM=$(uname -m)
case ${PLATFORM} in
x86_64* | i?86_64* | amd64*)
    ARCH="amd64"
    ;;
ppc64le)
    ARCH="ppc64le"
    ;;
aarch64* | arm64*)
    ARCH="arm64"
    ;;
*)
    echo "invalid Arch, only support x86_64, ppc64le, aarch64"
    exit 1
    ;;
esac

function _wait_kind_up {
    echo "Waiting for kind to be ready ..."
    
    while [ -z "$(docker exec --privileged ${CLUSTER_NAME}-control-plane kubectl --kubeconfig=/etc/kubernetes/admin.conf get nodes -o=jsonpath='{.items..status.conditions[-1:].status}' | grep True)" ]; do
        echo "Waiting for kind to be ready ..."
        sleep 10
    done
    echo "Waiting for dns to be ready ..."
    kubectl wait -n kube-system --timeout=12m --for=condition=Ready -l k8s-app=kube-dns pods
}

function _wait_containers_ready {
    echo "Waiting for all containers to become ready ..."
    kubectl wait --for=condition=Ready pod --all -n kube-system --timeout 12m
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

function _run_registry() {
    until [ -z "$(docker ps -a | grep ${REGISTRY_NAME})" ]; do
        docker stop ${REGISTRY_NAME} || true
        docker rm ${REGISTRY_NAME} || true
        sleep 5
    done

    docker run \
        -d --restart=always \
        -p "127.0.0.1:${REGISTRY_PORT}:5000" \
        --name "${REGISTRY_NAME}" \
        registry:2

    # connect the registry to the cluster network if not already connected
    docker network connect "${KIND_DEFAULT_NETWORK}" "${REGISTRY_NAME}" || true

    kubectl apply -f ${CONFIG_OUT_DIR}/local-registry.yml
}

function _prepare_config() {
    echo "Building manifests..."

    cp $KIND_MANIFESTS_DIR/kind.yml ${CONFIG_OUT_DIR}/kind.yml
    sed -i -e "s/$_registry_name/${REGISTRY_NAME}/g" ${CONFIG_OUT_DIR}/kind.yml
    sed -i -e "s/$_registry_port/${REGISTRY_PORT}/g" ${CONFIG_OUT_DIR}/kind.yml
    
    cp $KIND_MANIFESTS_DIR/local-registry.yml ${CONFIG_OUT_DIR}/local-registry.yml
    sed -i -e "s/$_registry_name/${REGISTRY_NAME}/g" ${CONFIG_OUT_DIR}/local-registry.yml
    sed -i -e "s/$_registry_port/${REGISTRY_PORT}/g" ${CONFIG_OUT_DIR}/local-registry.yml
}

function _get_nodes() {
    kubectl get nodes --no-headers
}

function _get_pods() {
    kubectl get pods --all-namespaces --no-headers
}

function _setup_kind() {
     echo "Starting kind with cluster name \"${CLUSTER_NAME}\""

    $KIND create cluster -v=6 --name=${CLUSTER_NAME} --config=${CONFIG_OUT_DIR}/kind.yml
    $KIND get kubeconfig --name=${CLUSTER_NAME} > ${CONFIG_OUT_DIR}/.kubeconfig

    _wait_kind_up
    kubectl cluster-info

    # wait until k8s pods are running
    while [ -n "$(_get_pods | grep -v Running)" ]; do
        echo "Waiting for all pods to enter the Running state ..."
        _get_pods | >&2 grep -v Running || true
        sleep 10
    done

    _wait_containers_ready
    _run_registry
}

function _kind_up() {
    _fetch_kind
    _prepare_config
    _setup_kind
}

function up() {
    _kind_up

    echo "${CLUSTER_PROVIDER} cluster '$CLUSTER_NAME' is ready"
}

function down() {
    _fetch_kind
    if [ -z "$($KIND get clusters | grep ${CLUSTER_NAME})" ]; then
        return
    fi
    # Avoid failing an entire test run just because of a deletion error
    $KIND delete cluster --name=${CLUSTER_NAME} || "true"
    docker rm -f ${REGISTRY_NAME} >> /dev/null
    rm -f ${CONFIG_PATH}/${CLUSTER_PROVIDER}/kind.yml
}
