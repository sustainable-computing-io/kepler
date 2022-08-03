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

set -e

export IMAGE_TAG=${IMAGE_TAG:-devel}
export IMAGE_REPO=${IMAGE_REPO:-quay.io/sustainable_computing_io/kepler}
export CLUSTER_PROVIDER=${CLUSTER_PROVIDER:-kubernetes}

# we do not use `make cluster-clean` and `make cluster-deploy` because they trigger other actions
function main() {
    make build-manifest
    
    ./hack/cluster-clean.sh &
    CLEAN_PID=$!

    make _build_containerized
    make push-image

    echo "waiting for cluster-clean to finish"
    if ! wait $CLEAN_PID; then
        echo "cluster-clean failed, output was:"
        cat $TEMP_FILE
        exit 1
    fi

    ./hack/cluster-deploy.sh
}

main "$@"