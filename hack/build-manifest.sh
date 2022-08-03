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
# Copyright 2022 IBM, Inc.
#

set -ex

CLUSTER_PROVIDER=${CLUSTER_PROVIDER:-kubernetes}
IMAGE_TAG=${IMAGE_TAG:-devel}
IMAGE_REPO=${IMAGE_REPO:-quay.io/sustainable_computing_io/kepler}

MANIFESTS_OUT_DIR=${MANIFESTS_OUT_DIR:-"_output/manifests/${CLUSTER_PROVIDER}/generated"}


echo "Building manifests..."

rm -rf ${MANIFESTS_OUT_DIR}
mkdir -p ${MANIFESTS_OUT_DIR}
cp -r manifests/${CLUSTER_PROVIDER}/* ${MANIFESTS_OUT_DIR}/

if [[ $CLUSTER_PROVIDER == "openshift" ]]; then
    cat manifests/${CLUSTER_PROVIDER}/kepler/01-kepler-install.yaml | sed "s|image:.*|image: $IMAGE_REPO:$IMAGE_TAG|" > ${MANIFESTS_OUT_DIR}/kepler/01-kepler-install.yaml
else
    cat manifests/${CLUSTER_PROVIDER}/deployment.yaml | sed "s|image:.*|image: $IMAGE_REPO:$IMAGE_TAG|" > ${MANIFESTS_OUT_DIR}/deployment.yaml
fi

echo "Done $0"