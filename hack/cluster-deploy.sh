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
declare -r CTR_CMD=${CTR_CMD:-docker}
declare -r CLUSTER_PROVIDER=${CLUSTER_PROVIDER:-kind}
declare -r MANIFESTS_OUT_DIR=${MANIFESTS_OUT_DIR:-"_output/generated-manifest"}

declare IMAGE_REPO="${IMAGE_REPO:-localhost:5001}"
declare IMAGE_TAG="${IMAGE_TAG:-devel}"
declare NO_BUILD=false

source "$PROJECT_ROOT/hack/utils.sh"

build_kepler() {
    header "Build Kepler"
    $NO_BUILD && {
        info "skipping building of kepler image"
        return 0
    }
    run make build_containerized \
        IMAGE_REPO="$IMAGE_REPO" \
        IMAGE_TAG="$IMAGE_TAG" \
        CTR_CMD="$CTR_CMD"
}
push_kepler() {
    header "Push Kepler Image"
    $NO_BUILD && {
        info "skipping pushing of kepler image"
        return 0
    }
    run make push-image \
        IMAGE_REPO="$IMAGE_REPO" \
        IMAGE_TAG="$IMAGE_TAG" \
        CTR_CMD="$CTR_CMD"
}

cluster_prereqs() {
    header "Setting up prerequisites"
    build_kepler
    push_kepler

    [[ ! -d "${MANIFESTS_OUT_DIR}" ]] && {
        err "Directory ${MANIFESTS_OUT_DIR} DOES NOT exists"
        if [[ "$CLUSTER_PROVIDER" == "microshift" ]]; then
            header 'Running: make build-manifest OPTS="OPENSHIFT_DEPLOY"'
            run make build-manifest \
                OPTS="OPENSHIFT_DEPLOY" \
                CLUSTER_PROVIDER="$CLUSTER_PROVIDER" \
                IMAGE_REPO="$IMAGE_REPO" \
                IMAGE_TAG="$IMAGE_TAG" \
                CTR_CMD="$CTR_CMD"
            run sed "s/localhost:5001/registry:5000/g" "${MANIFESTS_OUT_DIR}"/deployment.yaml >"${MANIFESTS_OUT_DIR}"/deployment.yaml.tmp &&
                mv "${MANIFESTS_OUT_DIR}"/deployment.yaml.tmp "${MANIFESTS_OUT_DIR}"/deployment.yaml
            return $?
        else
            header 'Running: make build-manifest OPTS="CI_DEPLOY"'
            run make build-manifest \
                OPTS="CI_DEPLOY" \
                CLUSTER_PROVIDER="$CLUSTER_PROVIDER" \
                IMAGE_REPO="$IMAGE_REPO" \
                IMAGE_TAG="$IMAGE_TAG" \
                CTR_CMD="$CTR_CMD"
            return $?
        fi
    }
    return 0
}

main() {

    cluster_prereqs || {
        fail "fail to configure cluster-prereqs."
        return 1
    }

    info "Deploying manifests with image:"
    run grep "image:" "${MANIFESTS_OUT_DIR}"/deployment.yaml

    kubectl apply -f "${MANIFESTS_OUT_DIR}"/deployment.yaml

    info "Verifying kepler deployment"
    "$PROJECT_ROOT"/hack/verify.sh "kepler"
}

main "$@"
