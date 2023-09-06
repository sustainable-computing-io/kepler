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

set -e

CTR_CMD=${CTR_CMD-docker}

IMAGE_REPO=${IMAGE_REPO:-localhost:5001}
ESTIMATOR_REPO=${ESTIMATOR_REPO:-quay.io/sustainable_computing_io}
MODEL_SERVER_REPO=${MODEL_SERVER_REPO:-quay.io/sustainable_computing_io}
IMAGE_TAG=${IMAGE_TAG:-devel}

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
s390x*)
    ARCH="s390x"
    ;;
*)
    echo "invalid Arch, only support x86_64, ppc64le, aarch64"
    exit 1
    ;;
esac

# the cluster kind is a kubernetes cluster
if [[ ${CLUSTER_PROVIDER} = "kind" ]]; then
    CLUSTER_PROVIDER="kubernetes"
fi

function wait_containers_ready {
     echo "waiting for all containers to become ready ..."
     kubectl wait --for=condition=Ready pod --all --all-namespaces --timeout 5m
}
