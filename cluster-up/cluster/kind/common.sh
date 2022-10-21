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

CTR_CMD=${CTR_CMD-docker}

CONFIG_PATH="cluster-up/cluster"
KIND_VERSION=${KIND_VERSION:-0.15.0}
KIND_MANIFESTS_DIR="$CONFIG_PATH/${CLUSTER_PROVIDER}/manifests"
CLUSTER_NAME=${KIND_CLUSTER_NAME:-kind}
REGISTRY_NAME=${REGISTRY_NAME:-kind-registry}
REGISTRY_PORT=${REGISTRY_PORT:-5001}
KIND_DEFAULT_NETWORK="kind"

IMAGE_REPO=${IMAGE_REPO:-localhost:5001/kepler}
IMAGE_TAG=${IMAGE_TAG:-devel}

PROMETHEUS_OPERATOR_VERSION=${PROMETHEUS_OPERATOR_VERSION:-v0.11.0}
PROMETHEUS_ENABLE=${PROMETHEUS_ENABLE:-false}
PROMETHEUS_REPLICAS=${PROMETHEUS_REPLICAS:-1}
GRAFANA_ENABLE=${GRAFANA_ENABLE:-false}

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

function _get_prometheus_operator_images {
    grep -R "image:" kube-prometheus/manifests/*prometheus-* | awk '{print $3}'
    grep -R "image:" kube-prometheus/manifests/*prometheusOperator* | awk '{print $3}'
    grep -R "prometheus-config-reloader=" kube-prometheus/manifests/ | sed 's/.*=//g'
    if [ ${GRAFANA_ENABLE,,} == "true" ]; then
        grep -R "image:" kube-prometheus/manifests/*grafana* | awk '{print $3}'
    fi
}

function _load_prometheus_operator_images_to_local_registry {
    for img in $(_get_prometheus_operator_images); do
        $CTR_CMD pull $img
        $KIND load docker-image $img
    done
} 

function _deploy_prometheus_operator {
    git clone -b ${PROMETHEUS_OPERATOR_VERSION} --depth 1 https://github.com/prometheus-operator/kube-prometheus.git
    sed -i -e "s/replicas: 2/replicas: ${PROMETHEUS_REPLICAS}/g" kube-prometheus/manifests/prometheus-prometheus.yaml
    _load_prometheus_operator_images_to_local_registry
    kubectl create -f kube-prometheus/manifests/setup
    kubectl wait \
        --for condition=Established \
        --all CustomResourceDefinition \
        --namespace=monitoring
    for file in $(ls kube-prometheus/manifests/prometheusOperator-*); do
        kubectl create -f $file
    done
    for file in $(ls kube-prometheus/manifests/prometheus-*); do
        kubectl create -f $file
    done
    if [ ${GRAFANA_ENABLE,,} == "true" ]; then
        for file in $(ls kube-prometheus/manifests/grafana-*); do
            kubectl create -f $file
        done
    fi
    rm -rf kube-prometheus
    _wait_containers_ready monitoring
}

function _wait_kind_up {
    echo "Waiting for kind to be ready ..."
    
    while [ -z "$($CTR_CMD exec --privileged ${CLUSTER_NAME}-control-plane kubectl --kubeconfig=/etc/kubernetes/admin.conf get nodes -o=jsonpath='{.items..status.conditions[-1:].status}' | grep True)" ]; do
        echo "Waiting for kind to be ready ..."
        sleep 10
    done
    echo "Waiting for dns to be ready ..."
    kubectl wait -n kube-system --timeout=12m --for=condition=Ready -l k8s-app=kube-dns pods
}

function _wait_containers_ready {
    echo "Waiting for all containers to become ready ..."
    namespace=$1
    kubectl wait --for=condition=Ready pod --all -n $namespace --timeout 12m
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
    until [ -z "$($CTR_CMD ps -a | grep ${REGISTRY_NAME})" ]; do
        $CTR_CMD stop ${REGISTRY_NAME} || true
        $CTR_CMD rm ${REGISTRY_NAME} || true
        sleep 5
    done

    $CTR_CMD run \
        -d --restart=always \
        -p "127.0.0.1:${REGISTRY_PORT}:5000" \
        --name "${REGISTRY_NAME}" \
        registry:2

    # connect the registry to the cluster network if not already connected
    $CTR_CMD network connect "${KIND_DEFAULT_NETWORK}" "${REGISTRY_NAME}" || true

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

    kubectl kustomize manifests/kubernetes/vm > manifests/kubernetes/vm/deployment.yaml

    # make cluster-sync overwrite the CONFIG_OUT_DIR, so that we update the manifest dir directly.
    # TODO: configure the kepler yaml in the CONFIG_OUT_DIR, not in the MANIFEST DIR.
    echo "WARN: we are changing the file manifests/kubernetes/vm/deployment.yaml"
    sed -i -e "s/path: \/proc/path: \/proc-host/g" manifests/kubernetes/vm/deployment.yaml
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

    _wait_containers_ready kube-system
    _run_registry

    if [ ${PROMETHEUS_ENABLE,,} == "true" ]; then
        _deploy_prometheus_operator
    fi
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
    $CTR_CMD rm -f ${REGISTRY_NAME} >> /dev/null
    rm -f ${CONFIG_PATH}/${CLUSTER_PROVIDER}/kind.yml
}
