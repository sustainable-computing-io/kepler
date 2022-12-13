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

# set options
# for example: ./build-manifest.sh "ESTIMATOR_SIDECAR_DEPLOY OPENSHIFT_DEPLOY"
DEPLOY_OPTIONS=$1
for opt in ${DEPLOY_OPTIONS}; do export $opt=true; done;

version=$(kubectl version --short | grep 'Client Version' | sed 's/.*v//g' | cut -b -4)
if [ 1 -eq "$(echo "${version} < 1.21" | bc)" ]
then
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
    ${SED} -e "/^patchesStrategicMerge.*/s/\[\]//" $file
}

uncomment_patch() {
    regex="patch-${1}"
    file="${2:?}"
    remove_empty_patch $file
    uncomment $regex $file
}

uncomment_path() {
    regex="..\/${1}"
    file="${2:?}"
    remove_empty_patch $file
    uncomment $regex $file
}

uncomment() {
    regex="${1:?}"
    file="${2:?}"
    ${SED} -e "/^# .*${regex}.*/s/^# //" $file
}

IMAGE_TAG=${IMAGE_TAG:-latest}
ESTIMATOR_IMAGE_TAG=${ESTIMATOR_IMAGE_TAG:-latest}
MODEL_SERVER_IMAGE_TAG=${MODEL_SERVER_IMAGE_TAG:-latest}
IMAGE_REPO=${IMAGE_REPO:-quay.io/sustainable_computing_io}
ESTIMATOR_REPO=${ESTIMATOR_REPO:-${IMAGE_REPO}}
MODEL_SERVER_REPO=${MODEL_SERVER_REPO:-${IMAGE_REPO}}
EXPORTER_IMAGE_NAME=${EXPORTER_IMAGE_NAME:-kepler}
ESTIMATOR_IMAGE_NAME=${ESTIMATOR_IMAGE_NAME:-kepler-estimator}
MODEL_SERVER_IMAGE_NAME=${MODEL_SERVER_IMAGE_NAME:-kepler_model_server}

MANIFESTS_OUT_DIR=${MANIFESTS_OUT_DIR:-"_output/generated-manifest"}

source cluster-up/common.sh

echo "Building manifests..."

echo "move to untrack workspace ${MANIFESTS_OUT_DIR}"
rm -rf ${MANIFESTS_OUT_DIR}
mkdir -p ${MANIFESTS_OUT_DIR}
cp -r manifests/config/* ${MANIFESTS_OUT_DIR}/

if [ ! -z ${BM_DEPLOY} ]; then
    echo "baremetal deployment"
    uncomment_patch bm ${MANIFESTS_OUT_DIR}/exporter/kustomization.yaml
fi

if [ ! -z ${OPENSHIFT_DEPLOY} ]; then
    echo "deployment on openshift"
    uncomment_patch openshift ${MANIFESTS_OUT_DIR}/exporter/kustomization.yaml
    uncomment openshift_scc ${MANIFESTS_OUT_DIR}/exporter/kustomization.yaml
    if [ ! -z ${CLUSTER_PREREQ_DEPLOY} ]; then
        echo "deploy cluster-prereqs"
        if [ -z ${BM_DEPLOY} ]; then
            uncomment "-cgroupv2" ${MANIFESTS_OUT_DIR}/cluster-prereqs/kustomization.yaml
        fi
    fi
fi

if [ ! -z ${PROMETHEUS_DEPLOY} ]; then
    echo "deployment with prometheus"
    uncomment prometheus_ ${MANIFESTS_OUT_DIR}/exporter/kustomization.yaml
    uncomment prometheus_ ${MANIFESTS_OUT_DIR}/rbac/kustomization.yaml
fi


if [ ! -z ${ESTIMATOR_SIDECAR_DEPLOY} ]; then
    echo "enable estimator-sidecar"
    uncomment_patch estimator-sidecar ${MANIFESTS_OUT_DIR}/exporter/kustomization.yaml
fi

if [ ! -z ${CI_DEPLOY} ]; then
    echo "enable ci"
    uncomment_patch ci ${MANIFESTS_OUT_DIR}/exporter/kustomization.yaml
fi

if [ ! -z ${MODEL_SERVER_DEPLOY} ]; then
    echo "enable model-server"
    uncomment_path model-server ${MANIFESTS_OUT_DIR}/base/kustomization.yaml
    uncomment_patch model-server-kepler-config ${MANIFESTS_OUT_DIR}/base/kustomization.yaml
    if [ ! -z ${OPENSHIFT_DEPLOY} ]; then
        uncomment_patch openshift ${MANIFESTS_OUT_DIR}/model-server/kustomization.yaml
    fi

    if [ ! -z ${TRAINER_DEPLOY} ]; then
        echo "enable online-trainer of model-server"
        uncomment_patch trainer ${MANIFESTS_OUT_DIR}/model-server/kustomization.yaml
        if [ ! -z ${OPENSHIFT_DEPLOY} ]; then
            uncomment_patch train-ocp ${MANIFESTS_OUT_DIR}/model-server/kustomization.yaml
        fi
    fi
fi

echo "set manager image"
EXPORTER_IMG=${IMAGE_REPO}/${EXPORTER_IMAGE_NAME}:${IMAGE_TAG}
ESTIMATOR_IMG=${ESTIMATOR_REPO}/${ESTIMATOR_IMAGE_NAME}:${ESTIMATOR_IMAGE_TAG}
MODEL_SERVER_IMG=${MODEL_SERVER_REPO}/${MODEL_SERVER_IMAGE_NAME}:${MODEL_SERVER_IMAGE_TAG}
pushd ${MANIFESTS_OUT_DIR}/exporter;${KUSTOMIZE} edit set image kepler=${EXPORTER_IMG}; ${KUSTOMIZE} edit set image kepler-estimator=${ESTIMATOR_IMG}; popd
pushd ${MANIFESTS_OUT_DIR}/model-server;${KUSTOMIZE} edit set image kepler=${MODEL_SERVER_IMG}; popd

echo "kustomize manifests..."
${KUSTOMIZE} build ${MANIFESTS_OUT_DIR}/base > ${MANIFESTS_OUT_DIR}/deployment.yaml

for opt in ${DEPLOY_OPTIONS}; do unset $opt; done;

echo "Done $0"