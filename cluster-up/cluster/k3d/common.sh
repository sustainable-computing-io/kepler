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

CTR_CMD=${CTR_CMD-docker}

CONFIG_PATH="cluster-up/cluster"
K3D_VERSION=${K3D_VERSION:-5.4.6}
K3D_MANIFESTS_DIR="$CONFIG_PATH/${CLUSTER_PROVIDER}/manifests"
CLUSTER_NAME=${K3D_CLUSTER_NAME:-kepler}
CLUSTER_NETWORK="kepler-network"
CLUSTER_SERVERS=${K3D_SERVERS:-1}
CLUSTER_AGENTS=${K3D_AGENTS:-1}
LOADBALANCER_PORT=${LOADBALANCER_PORT:-8081}

REGISTRY_NAME=${REGISTRY_NAME:-kepler-registry-local}
REGISTRY_PORT=${REGISTRY_PORT:-5001}

IMAGE_REPO=${IMAGE_REPO:-localhost:5001/kepler}
IMAGE_TAG=${IMAGE_TAG:-devel}

PROMETHEUS_OPERATOR_VERSION=${PROMETHEUS_OPERATOR_VERSION:-v0.11.0}
PROMETHEUS_ENABLE=${PROMETHEUS_ENABLE:-true}
PROMETHEUS_REPLICAS=${PROMETHEUS_REPLICAS:-1}
GRAFANA_ENABLE=${GRAFANA_ENABLE:-true}

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
    if [ ${GRAFANA_ENABLE} == "true" ]; then
        grep -R "image:" kube-prometheus/manifests/*grafana* | awk '{print $3}'
    fi
}

function _load_prometheus_operator_images_to_local_registry {
    for img in $(_get_prometheus_operator_images); do
        $CTR_CMD pull $img
        $CTR_CMD tag $img "localhost:${REGISTRY_PORT}/${img}"
        $CTR_CMD push "localhost:${REGISTRY_PORT}/${img}"
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
    for file in $(ls kube-prometheus/manifests/prometheusOperator-*.yaml); do
        kubectl create -f $file
    done
    for file in $(ls kube-prometheus/manifests/prometheus-*.yaml); do
        kubectl create -f $file
    done
    if [ ${GRAFANA_ENABLE} == "true" ]; then
        for file in $(ls kube-prometheus/manifests/grafana-*.yaml); do
            kubectl create -f $file
        done
    fi
    
    rm -rf kube-prometheus
    _wait_containers_ready monitoring
}

function _wait_k3d_up {
    echo "Waiting for k3d to be ready ..."
    
    while [ -z "$($CTR_CMD exec --privileged k3d-${CLUSTER_NAME}-server-0 kubectl --kubeconfig=/etc/rancher/k3s/k3s.yaml get nodes -o=jsonpath='{.items..status.conditions[-1:].status}' | grep True)" ]; do
        echo "Waiting for k3d to be ready ..."
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

function _fetch_k3d(){
    mkdir -p ${CONFIG_OUT_DIR}
    K3D="${CONFIG_OUT_DIR}"/.k3d
    if [ -f $K3D ]; then
        current_K3D_VERSION=$($K3D version | grep 'k3d version' | cut -d " " -f3)
    fi
    if [[ $current_K3D_VERSION != $K3D_VERSION ]]; then
        echo "Downloading k3d v$K3D_VERSION"
        if [[ "$OSTYPE" == "darwin"* ]]; then
            curl -LSs https://github.com/k3d-io/k3d/releases/download/v$K3D_VERSION/k3d-darwin-${ARCH} -o "$K3D"
        else
            curl -LSs https://github.com/k3d-io/k3d/releases/download/v$K3D_VERSION/k3d-linux-${ARCH} -o "$K3D"
        fi
        chmod +x "$K3D"
    fi
}

function _prepare_config() {
    echo "Building manifests..."

    cp $K3D_MANIFESTS_DIR/k3d.yml ${CONFIG_OUT_DIR}/k3d.yml
    sed -i -e "s/_cluster_name/${CLUSTER_NAME}/g" ${CONFIG_OUT_DIR}/k3d.yml
    sed -i -e "s/_cluster_network/${CLUSTER_NETWORK}/g" ${CONFIG_OUT_DIR}/k3d.yml
    sed -i -e "s/_cluster_servers/${CLUSTER_SERVERS}/g" ${CONFIG_OUT_DIR}/k3d.yml
    sed -i -e "s/_cluster_agents/${CLUSTER_AGENTS}/g" ${CONFIG_OUT_DIR}/k3d.yml
    sed -i -e "s/_registry_name/${REGISTRY_NAME}/g" ${CONFIG_OUT_DIR}/k3d.yml
    sed -i -e "s/_registry_port/\"${REGISTRY_PORT}\"/g" ${CONFIG_OUT_DIR}/k3d.yml

    sed -i -e "s/_loadbalancer_port/${LOADBALANCER_PORT}/g" ${CONFIG_OUT_DIR}/k3d.yml

    # make cluster-sync overwrite the CONFIG_OUT_DIR, so that we update the manifest dir directly.
    # TODO: configure the kepler yaml in the CONFIG_OUT_DIR, not in the MANIFEST DIR.
    echo "WARN: we are changing the file manifests/kubernetes/deployment.yaml"
    sed -i -e "s/path: \/proc/path: \/proc-host/g" manifests/kubernetes/deployment.yaml
}

function _get_nodes() {
    kubectl get nodes --no-headers
}

function _get_pods() {
    kubectl get pods --all-namespaces --no-headers
}

function _setup_k3d() {
     echo "Starting k3d with cluster name \"${CLUSTER_NAME}\""

    $K3D cluster create --config=${CONFIG_OUT_DIR}/k3d.yml
    $K3D kubeconfig get ${CLUSTER_NAME} > ${CONFIG_OUT_DIR}/.kubeconfig

    _wait_k3d_up
    kubectl cluster-info

    # wait until k8s pods are running
    while [ -n "$(_get_pods | grep -v 'Running\|Completed')" ]; do
        echo "Waiting for all pods to enter the Running state ..."
        _get_pods | >&2 grep -v 'Running\|Completed' || true
        sleep 10
    done

    # _wait_containers_ready kube-system

    # if [ ${PROMETHEUS_ENABLE} == "true" ]; then
    #     # _deploy_prometheus_operator
    # fi
}

function _k3d_up() {
    _fetch_k3d
    _prepare_config
    _setup_k3d
}

function up() {
    _k3d_up

    echo "${CLUSTER_PROVIDER} cluster '$CLUSTER_NAME' is ready"
}

function down() {
    _fetch_k3d
    if [ -z "$($K3D cluster list | grep ${CLUSTER_NAME})" ]; then
        return
    fi
    # Avoid failing an entire test run just because of a deletion error
    $K3D cluster delete ${CLUSTER_NAME} || "true"
    rm -f ${CONFIG_PATH}/${CLUSTER_PROVIDER}/k3d.yml
    rm -f ${CONFIG_PATH}/${CLUSTER_PROVIDER}/.k3d
}
