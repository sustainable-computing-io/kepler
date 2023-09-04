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

set -eu -o pipefail

# constants
PROJECT_ROOT="$(git rev-parse --show-toplevel)"
declare -r CLUSTER_PROVIDER=${CLUSTER_PROVIDER:-kind}
declare -r MANIFESTS_OUT_DIR=${MANIFESTS_OUT_DIR:-"_output/generated-manifest"}
declare -r ARTIFACT_DIR="${ARTIFACT_DIR:-/tmp/artifacts}"

source "$PROJECT_ROOT/hack/utils.sh"

must_gather() {
    info "running must gather on cluster"
    kubectl describe nodes >"$ARTIFACT_DIR/nodes"
    kubectl get pods --all-namespaces >"$ARTIFACT_DIR/pods"
    kubectl get deployment --all-namespaces >"$ARTIFACT_DIR/deployments"
    kubectl get daemonsets --all-namespaces >"$ARTIFACT_DIR/daemonsets"
    kubectl get services --all-namespaces >"$ARTIFACT_DIR/services"
    kubectl get endpoints --all-namespaces >"$ARTIFACT_DIR/endpoints"
    kubectl describe daemonset kepler-exporter -n kepler >"$ARTIFACT_DIR/kepler-daemonset-describe"
    kubectl get pods -n kepler -o yaml >"$ARTIFACT_DIR/kepler-pod-yaml"
    kubectl logs "$(kubectl -n kepler get pods -o name)" -n kepler >"$ARTIFACT_DIR/kepler-pod-logs"
}

check_deployment_status() {
    info "checking deployment status"
    kubectl rollout status daemonset kepler-exporter -n kepler --timeout 5m || {
        must_gather
        die "fail to check status of daemonset"
    }
    info "checking if kepler is still alive"
    kubectl get all -n kepler || {
        die "cannot get resources under kepler namespace"
    }
    info "checking for kepler logs"
    kubectl logs "$(kubectl -n kepler get pods -o name)" -n kepler || {
        die "cannot check kepler pod logs"
    }
}

intergration_test() {
    tags=$1
    if [ "$tags" == "" ]; then
        tags="bcc"
    fi
    info TAGS="$tags"
    kepler_ready=false
    until ${kepler_ready}; do
        kubectl logs "$(kubectl -n kepler get pods -o name)" -n kepler >"$ARTIFACT_DIR"/kepler.log
        if [ "$(grep -c "Started Kepler in" "$ARTIFACT_DIR"/kepler.log)" -ne '0' ]; then
            ok "Kepler start finish"
            kepler_ready=true
        else
            sleep 10
        fi
    done
    info "enabling port-forward on kepler service"
    while true; do kubectl port-forward --address localhost -n kepler service/kepler-exporter 9102:9102; done &
    info "checking for kepler logs"
    kubectl logs "$(kubectl -n kepler get pods -o name)" -n kepler
    info "get kepler pods"
    kubectl get pods -n kepler -o yaml
    info "running tests"
    run go test ./e2e/... --tags "$tags" -v --race --bench=. -cover --count=1 --vet=all
}

main() {
    [[ "$#" -lt 2 ]] && {
        echo "Usage: $0 <operation> <tag>"
        exit 1
    }
    local op="$1"
    shift
    local tag="$1"
    mkdir -p "$ARTIFACT_DIR"
    # verify the deployment of cluster
    case $op in
    kepler)
        check_deployment_status
        ;;
    test)
        intergration_test "$tag"
        ;;
    *)
        check_deployment_status
        intergration_test
        ;;
    esac
}

main "$@"
