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
set -o pipefail
set -x

source ./hack/common.sh

CLUSTER_PROVIDER=${CLUSTER_PROVIDER:-kubernetes}
MANIFESTS_OUT_DIR=${MANIFESTS_OUT_DIR:-"_output/generated-manifest"}
ARTIFACT_DIR="${ARTIFACT_DIR:-/tmp/artifacts}"

function must_gather() {
    mkdir -p $ARTIFACT_DIR
    kubectl describe nodes > "$ARTIFACT_DIR/nodes"
    kubectl get pods --all-namespaces > "$ARTIFACT_DIR/pods"
    kubectl get deployment --all-namespaces > "$ARTIFACT_DIR/deployments"
    kubectl get daemonsets --all-namespaces > "$ARTIFACT_DIR/daemonsets"
    kubectl get services --all-namespaces > "$ARTIFACT_DIR/services"
    kubectl get endpoints --all-namespaces > "$ARTIFACT_DIR/endpoints"
    kubectl describe daemonset kepler-exporter -n kepler > "$ARTIFACT_DIR/kepler-daemonset-describe"
    kubectl get pods -n kepler -o yaml > "$ARTIFACT_DIR/kepler-pod-yaml"
    kubectl logs $(kubectl -n kepler get pods -o name) -n kepler > "$ARTIFACT_DIR/kepler-pod-logs"
}

function check_deployment_status() {
    kubectl rollout status daemonset kepler-exporter -n kepler --timeout 5m || {
        must_gather
        exit 1
    }
    echo "check if kepler is still alive"
    kubectl logs $(kubectl -n kepler get pods -o name) -n kepler
    kubectl get all -n kepler
}

function intergration_test() {
    tags=$1
    if [ "$tags" == "" ]
    then
        tags="bcc"
    fi
    echo TAGS=$tags
    kepler_ready=false
    $CTR_CMD ps -a
    mkdir -p /tmp/.kube
    if [ "$CLUSTER_PROVIDER" == "microshift" ]
    then
        $CTR_CMD exec -i microshift cat /var/lib/microshift/resources/kubeadmin/kubeconfig > /tmp/.kube/config
    else
        kind get kubeconfig --name=kind > /tmp/.kube/config
    fi
    until ${kepler_ready} ; do
        kubectl logs $(kubectl -n kepler get pods -o name) -n kepler > kepler.log
	if [ `grep -c "Started Kepler in" kepler.log` -ne '0' ]; then
		echo "Kepler start finish"
		kepler_ready=true
	else
		sleep 10
	fi
    done
    rm -f kepler.log
    while true; do kubectl port-forward --address localhost -n kepler service/kepler-exporter 9102:9102; done &
    kubectl logs -n kepler daemonset/kepler-exporter
    kubectl get pods -n kepler -o yaml
    go test ./e2e/... --tags $tags -v --race --bench=. -cover --count=1 --vet=all
}

function main() {
    # verify the deployment of cluster
    case $1 in
    kepler)
        check_deployment_status
        ;;
    test)
        intergration_test $2
        ;;
    *)
        check_deployment_status
        intergration_test
        ;;
    esac
}

main "$@"
