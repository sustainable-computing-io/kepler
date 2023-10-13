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

set -ex

export CLUSTER_PROVIDER=${CLUSTER_PROVIDER:-kind}

# set options
# for example: ./build-manifest.sh "ESTIMATOR_SIDECAR_DEPLOY OPENSHIFT_DEPLOY"
DEPLOY_OPTIONS=$1
for opt in ${DEPLOY_OPTIONS}; do export "$opt"=true; done

version=$(kubectl version --short | grep 'Client Version' | sed 's/.*v//g' | cut -b -4)
if [ 1 -eq "$(echo "${version} < 1.21" | bc)" ]; then
    echo "You need to update your kubectl version to 1.21+ to support kustomize"
    exit 1
fi

KUSTOMIZE=${PWD}/bin/kustomize
SED="sed -i"
if [ "$(uname)" == "Darwin" ]; then
    SED="sed -i .bak "
fi

remove_empty_patch() {
    file="${1:?}"
    ${SED} -e "/^patchesStrategicMerge.*/s/\[\]//" "$file"
}

uncomment_patch() {
    regex="patch-${1}"
    file="${2:?}"
    remove_empty_patch "$file"
    uncomment "$regex" "$file"
}

uncomment_path() {
    regex="..\/${1}"
    file="${2:?}"
    remove_empty_patch "$file"
    uncomment "$regex" "$file"
}

uncomment() {
    regex="${1:?}"
    file="${2:?}"
    ${SED} -e "/^# .*${regex}.*/s/^# //" "$file"
}

IMAGE_TAG=${IMAGE_TAG:-latest}
MODEL_SERVER_IMAGE_TAG=${MODEL_SERVER_IMAGE_TAG:-latest}
IMAGE_REPO=${IMAGE_REPO:-quay.io/sustainable_computing_io}
MODEL_SERVER_REPO=${MODEL_SERVER_REPO:-${IMAGE_REPO}}
EXPORTER_IMAGE_NAME=${EXPORTER_IMAGE_NAME:-kepler}
MODEL_SERVER_IMAGE_NAME=${MODEL_SERVER_IMAGE_NAME:-kepler_model_server}

MANIFESTS_OUT_DIR=${MANIFESTS_OUT_DIR:-"_output/generated-manifest"}

# shellcheck source=hack/common.sh
source hack/common.sh

echo "Building manifests..."

echo "move to untrack workspace ${MANIFESTS_OUT_DIR}"
rm -rf "${MANIFESTS_OUT_DIR}"
mkdir -p "${MANIFESTS_OUT_DIR}"
cp -r manifests/config/* "${MANIFESTS_OUT_DIR}"/

if [ -n "${BM_DEPLOY}" ]; then
    echo "baremetal deployment"
    uncomment_patch bm "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
fi

if [ -n "${ROOTLESS}" ]; then
    echo "rootless deployment"
    uncomment_patch rootless "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
fi

if [ -n "${OPENSHIFT_DEPLOY}" ]; then
    echo "deployment on openshift"
    uncomment_patch openshift "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
    uncomment openshift_scc "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
fi

if [ -n "${PROMETHEUS_DEPLOY}" ]; then
    echo "deployment with prometheus"
    uncomment prometheus_ "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
    uncomment prometheus_ "${MANIFESTS_OUT_DIR}"/rbac/kustomization.yaml
    if [ -n "${HIGH_GRANULARITY}" ]; then
        echo "enable high metric granularity in Prometheus"
        uncomment_patch high-granularity "${MANIFESTS_OUT_DIR}"/base/kustomization.yaml
    fi
fi

if [ -n "${ESTIMATOR_SIDECAR_DEPLOY}" ]; then
    echo "enable estimator-sidecar"
    uncomment_patch estimator-sidecar "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
fi

if [ -n "${CI_DEPLOY}" ]; then
    echo "enable ci ${CLUSTER_PROVIDER}"
    uncomment_patch ci "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
    case ${CLUSTER_PROVIDER} in
    microshift) ;;
    *)
        uncomment_patch kind "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
        ;;
    esac
fi

if [ -n "${DEBUG_DEPLOY}" ]; then
    echo "enable debug"
    uncomment_patch debug "${MANIFESTS_OUT_DIR}"/base/kustomization.yaml
fi

if [ -n "${MODEL_SERVER_DEPLOY}" ]; then
    echo "enable model-server"
    uncomment_path model-server "${MANIFESTS_OUT_DIR}"/base/kustomization.yaml
    uncomment_patch model-server-kepler-config "${MANIFESTS_OUT_DIR}"/base/kustomization.yaml
    if [ -n "${OPENSHIFT_DEPLOY}" ]; then
        uncomment_patch openshift "${MANIFESTS_OUT_DIR}"/model-server/kustomization.yaml
    fi

    if [ -n "${TRAINER_DEPLOY}" ]; then
        echo "enable online-trainer of model-server"
        uncomment_patch trainer "${MANIFESTS_OUT_DIR}"/model-server/kustomization.yaml
        if [ -n "${OPENSHIFT_DEPLOY}" ]; then
            uncomment_patch train-ocp "${MANIFESTS_OUT_DIR}"/model-server/kustomization.yaml
        fi
    fi
fi

if [ -n "${QAT_DEPLOY}" ]; then
    echo "enable qat"
    uncomment_patch qat "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
fi

echo "set manager image"
EXPORTER_IMG=${IMAGE_REPO}/${EXPORTER_IMAGE_NAME}:${IMAGE_TAG}
MODEL_SERVER_IMG=${MODEL_SERVER_REPO}/${MODEL_SERVER_IMAGE_NAME}:${MODEL_SERVER_IMAGE_TAG}
pushd "${MANIFESTS_OUT_DIR}"/exporter
${KUSTOMIZE} edit set image kepler="${EXPORTER_IMG}"
${KUSTOMIZE} edit set image kepler_model_server="${MODEL_SERVER_IMG}"
popd
pushd "${MANIFESTS_OUT_DIR}"/model-server
${KUSTOMIZE} edit set image kepler_model_server="${MODEL_SERVER_IMG}"
popd

echo "kustomize manifests..."
${KUSTOMIZE} build "${MANIFESTS_OUT_DIR}"/base >"${MANIFESTS_OUT_DIR}"/deployment.yaml

for opt in ${DEPLOY_OPTIONS}; do unset "$opt"; done

echo "Done $0"
