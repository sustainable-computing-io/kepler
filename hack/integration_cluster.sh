#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
set -o errexit
#set -x
CLUSTER_NAME=${ENV_KIND_CLUSTER_NAME:-kind}

LOCAL_REGISTRY_NAME=${ENV_LOCAL_REGISTRY_NAME:-kind-registry}
LOCAL_REGISTRY_PORT=${ENV_LOCAL_REGISTRY_PORT:-5000}

NGINX_HTTP_PORT=${ENV_NETWORK_INGRESS_HTTP_PORT:-80}
NGINX_HTTPS_PORT=${ENV_NETWORK_INGRESS_HTTPS_PORT:-443}

main() {
    if [[ $# -lt 1 ]] ; then
        exit 0
    else
        MODE=$1
        shift
    fi

    if [ "${MODE}" == "kind" ]; then
        kindTest
    fi
}

function kindTest() {
    echo "install kind"
    go install sigs.k8s.io/kind@v0.12.0

    echo "Starting kind with cluster name \"${CLUSTER_NAME}\""

    local reg_name=${LOCAL_REGISTRY_NAME}
    local reg_port=${LOCAL_REGISTRY_PORT}
    local ingress_http_port=${NGINX_HTTP_PORT}
    local ingress_https_port=${NGINX_HTTPS_PORT}
    docker rm -f ${reg_name}
    kind delete cluster --name $CLUSTER_NAME

  cat <<EOF | kind create cluster -v=6 --name $CLUSTER_NAME --config=-
---
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
    extraPortMappings:
      - containerPort: 80
        hostPort: ${ingress_http_port}
        protocol: TCP
      - containerPort: 443
        hostPort: ${ingress_https_port}
        protocol: TCP
# create a cluster with the local registry enabled in containerd
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${reg_port}"]
    endpoint = ["http://${reg_name}:${reg_port}"]
EOF

    echo "Launching container registry \"${LOCAL_REGISTRY_NAME}\" at localhost:${LOCAL_REGISTRY_PORT}"
    running="$(docker inspect -f '{{.State.Running}}' "${reg_name}" 2>/dev/null || true)"
    if [ "${running}" != 'true' ]; then
        docker run \
        -d --restart=always -p "127.0.0.1:${reg_port}:5000" --name "${reg_name}" \
        registry:2
    fi

    # connect the registry to the cluster network
    # (the network may already be connected)
    docker network connect "kind" "${reg_name}" || true

    # Document the local registry
    # https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry
    cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${reg_port}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

    echo "Complete start kind"
}

main $*