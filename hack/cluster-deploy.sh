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

PROJECT_ROOT="$(git rev-parse --show-toplevel)"
declare -r PROJECT_ROOT

source "$PROJECT_ROOT/hack/utils.bash"

declare -r MANIFESTS_OUT_DIR="${MANIFESTS_OUT_DIR:-_output/generated-manifest}"

declare CLUSTER_PROVIDER="${CLUSTER_PROVIDER:-kind}"
declare CTR_CMD="${CTR_CMD:-docker}"
declare IMAGE_TAG="${IMAGE_TAG:-devel}"
declare IMAGE_REPO="${IMAGE_REPO:-localhost:5001}"
declare OPTS="${OPTS:-}"
declare NO_BUILDS="${NO_BUILDS:-false}"

build_manifest() {
	$NO_BUILDS && {
		ok "Skipping building manifests"
		return 0
	}
	header "Build Kepler Manifest"
	run make build-manifest \
		OPTS="$OPTS" \
		IMAGE_REPO="$IMAGE_REPO" \
		IMAGE_TAG="$IMAGE_TAG"
}

build_kepler() {
	header "Build Kepler Image"
	$NO_BUILDS && {
		ok "Skipping building of images"
		return 0
	}
	run make build_containerized \
		IMAGE_REPO="$IMAGE_REPO" \
		IMAGE_TAG="$IMAGE_TAG" \
		VERSION="devel"
}
push_kepler() {
	header "Push Kepler Image"
	$NO_BUILDS && {
		ok "Skipping pushing of images"
		return 0
	}
	run make push-image \
		IMAGE_REPO="$IMAGE_REPO" \
		IMAGE_TAG="$IMAGE_TAG"
}
run_kepler() {
	header "Running Kepler"

	[[ ! -d "$MANIFESTS_OUT_DIR" ]] && die "Directory ${MANIFESTS_OUT_DIR} DOES NOT exists. Run make generate first."

	[[ "$CLUSTER_PROVIDER" == "microshift" ]] && {
		sed "s/localhost:5001/registry:5000/g" "${MANIFESTS_OUT_DIR}"/deployment.yaml >"${MANIFESTS_OUT_DIR}"/deployment.yaml.tmp &&
			mv "${MANIFESTS_OUT_DIR}"/deployment.yaml.tmp "${MANIFESTS_OUT_DIR}"/deployment.yaml
	}

	kubectl apply -f "${MANIFESTS_OUT_DIR}" || true
}
clean_kepler() {
	header "Cleaning Kepler"
	kubectl delete --ignore-not-found=true -f "${MANIFESTS_OUT_DIR}"/*.yaml || true
}
verify_kepler() {
	header "Verifying Kepler"
	run ./hack/verify.sh kepler || {
		die "Kepler validation failed ❌"
	}
	ok "Kepler deployed successfully"
}
deploy_kepler() {
	header "Build and Deploy Kepler"

	build_manifest
	build_kepler
	push_kepler
	clean_kepler
	run_kepler
	verify_kepler
}
main() {
	export CTR_CMD="$CTR_CMD"
	cd "$PROJECT_ROOT"

	deploy_kepler
}
main "$@"
