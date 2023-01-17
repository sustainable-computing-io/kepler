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

source ./hack/common.sh

CLUSTER_PROVIDER=${CLUSTER_PROVIDER:-kubernetes}
MANIFESTS_OUT_DIR=${MANIFESTS_OUT_DIR:-"_output/generated-manifest"}

function main() {
    [ ! -d "${MANIFESTS_OUT_DIR}" ] && echo "Directory ${MANIFESTS_OUT_DIR} DOES NOT exists. Run make generate first."
    
    echo "Deploying manifests..."

    # Ignore errors because some clusters might not have prometheus operator
    echo "Deploying with image:"
    cat ${MANIFESTS_OUT_DIR}/deployment.yaml | grep "image:"

    kubectl apply -f ${MANIFESTS_OUT_DIR} || true
    
    # round for 3 times and each for 60s
    # check if the rollout status is running
    deploy_status=1
    for i in 1 2 3
    do
        echo "check deployment status for round $i"
        kubectl rollout status daemonset kepler-exporter -n kepler --timeout 60s
        #check rollout status
        if [ $? -eq 0 ]
        then
            deploy_status=0
            break
        fi
    done 
    # if deployment in error
    if test $[deploy_status] -eq 1
    then
        echo "Check the status of the kepler-exporter"
        kubectl -n kepler describe daemonset.apps/kepler-exporter
        echo "Check the logs of the kepler-exporter"
        kubectl -n kepler logs daemonset.apps/kepler-exporter
    fi
}

main "$@"
