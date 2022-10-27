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

version=$(kubectl version --short | grep 'Client Version' | sed 's/.*v//g' | cut -b -4)
if [ 1 -eq "$(echo "${version} < 1.21" | bc)" ]
then
    echo "You need to update your kubectl version to 1.21+ to support kustomize"
    exit 1
fi

CLUSTER_PROVIDER=${CLUSTER_PROVIDER:-kubernetes}
IMAGE_TAG=${IMAGE_TAG:-latest}
IMAGE_REPO=${IMAGE_REPO:-quay.io/sustainable_computing_io/kepler}

MANIFESTS_OUT_DIR=${MANIFESTS_OUT_DIR:-"_output/manifests/${CLUSTER_PROVIDER}/generated"}

source cluster-up/common.sh

echo "Building manifests..."

rm -rf ${MANIFESTS_OUT_DIR}
mkdir -p ${MANIFESTS_OUT_DIR}
cp -r manifests/${CLUSTER_PROVIDER}/* ${MANIFESTS_OUT_DIR}/

echo "kustomize manifests..."
kubectl kustomize ${MANIFESTS_OUT_DIR}/bm > ${MANIFESTS_OUT_DIR}/bm/deployment.yaml
kubectl kustomize ${MANIFESTS_OUT_DIR}/vm > ${MANIFESTS_OUT_DIR}/vm/deployment.yaml

if [[ $CLUSTER_PROVIDER == "openshift" ]]; then
    cat manifests/${CLUSTER_PROVIDER}/kepler/01-kepler-install.yaml | sed "s|image:.*|image: $IMAGE_REPO:$IMAGE_TAG|" > ${MANIFESTS_OUT_DIR}/kepler/01-kepler-install.yaml
else
    sed -i "s|image:.*|image: $IMAGE_REPO:$IMAGE_TAG|" ${MANIFESTS_OUT_DIR}/bm/deployment.yaml
    sed -i "s|image:.*|image: $IMAGE_REPO:$IMAGE_TAG|" ${MANIFESTS_OUT_DIR}/vm/deployment.yaml
fi

echo "Done $0"
