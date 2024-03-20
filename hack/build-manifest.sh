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
declare -r PROJECT_ROOT

source "$PROJECT_ROOT/hack/utils.bash"

declare CLUSTER_PROVIDER="${CLUSTER_PROVIDER:-kind}"

declare IMAGE_TAG=${IMAGE_TAG:-latest}
declare MODEL_SERVER_IMAGE_TAG=${MODEL_SERVER_IMAGE_TAG:-latest}
declare IMAGE_REPO=${IMAGE_REPO:-quay.io/sustainable_computing_io}
declare MODEL_SERVER_REPO=${MODEL_SERVER_REPO:-${IMAGE_REPO}}
declare EXPORTER_IMAGE_NAME=${EXPORTER_IMAGE_NAME:-kepler}
declare MODEL_SERVER_IMAGE_NAME=${MODEL_SERVER_IMAGE_NAME:-kepler_model_server}
declare EXPORTER_IMG=${EXPORTER_IMG:-${IMAGE_REPO}/${EXPORTER_IMAGE_NAME}:${IMAGE_TAG}}
declare MODEL_SERVER_IMG=${MODEL_SERVER_IMG:-${MODEL_SERVER_REPO}/${MODEL_SERVER_IMAGE_NAME}:${MODEL_SERVER_IMAGE_TAG}}

declare -r MANIFESTS_OUT_DIR=${MANIFESTS_OUT_DIR:-"_output/generated-manifest"}

declare BM_DEPLOY=false
declare ROOTLESS_DEPLOY=false
declare OPENSHIFT_DEPLOY=false
declare ESTIMATOR_SIDECAR_DEPLOY=false
declare CI_DEPLOY=false
declare DEBUG_DEPLOY=false
declare MODEL_SERVER_DEPLOY=false
declare QAT_DEPLOY=false
declare PROMETHEUS_DEPLOY=false
declare HIGH_GRANULARITY=false
declare DCGM_DEPLOY=false

ensure_all_tools() {
	header "Ensuring all tools are installed"
	"$PROJECT_ROOT/hack/tools.sh" all
}

remove_empty_patch() {
	file="${1:?}"
	sed <"$file" "/^patchesStrategicMerge.*/s/\[\]//" >"${file}.tmp"
	mv "${file}.tmp" "${file}"
}

uncomment_patch() {
	regex="patch-${1}"
	file="${2:?}"
	remove_empty_patch "$file"
	uncomment "$regex" "$file"
}

uncomment_path() {
	regex="..\/${1}"
	file="${2:?}"
	remove_empty_patch "$file"
	uncomment "$regex" "$file"
}

uncomment() {
	regex="${1:?}"
	file="${2:?}"
	sed <"$file" "/^# .*${regex}.*/s/^# //" >"${file}.tmp"
	mv "${file}.tmp" "${file}"
}

deploy_prometheus() {
	header "Prometheus Deployment"
	$PROMETHEUS_DEPLOY || {
		skip "skipping prometheus deployment"
		return 0
	}
	info "deploying prometheus"
	uncomment prometheus_common "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
	uncomment prometheus_role "${MANIFESTS_OUT_DIR}"/rbac/kustomization.yaml

	ok "Prometheus deployment configured"

	$HIGH_GRANULARITY || {
		skip "skipping prometheus deployment with high granularity"
		return 0
	}
	info "deploying prometheus with high granularity"
	uncomment prometheus_high "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
	uncomment_patch high-granularity "${MANIFESTS_OUT_DIR}"/base/kustomization.yaml

	ok "Prometheus deployment with high granularity configured"
}
deploy_bm() {
	header "Baremetal Deployment"
	$BM_DEPLOY || {
		skip "skipping baremetal deployment"
		return 0
	}
	uncomment_patch bm "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
	ok "Baremetal deployment configured"
}
deploy_rootless() {
	header "Rootless Deployment"
	$ROOTLESS_DEPLOY || {
		skip "skipping rootless deployment"
		return 0
	}
	uncomment_patch rootless "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
	ok "Rootless deployment configured"
}
deploy_openshift() {
	header "OpenShift Deployment"
	$OPENSHIFT_DEPLOY || {
		skip "skipping openshift deployment"
		return 0
	}
	uncomment_patch openshift "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
	uncomment openshift_scc "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
	ok "OpenShift deployment configured"
}
deploy_estimator_sidecar() {
	header "Estimator Sidecar Deployment"
	$ESTIMATOR_SIDECAR_DEPLOY || {
		skip "skipping estimator with sidecar deployment"
		return 0
	}
	uncomment_patch estimator-sidecar "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
	ok "Estimator sidecar deployment configured"
}
deploy_ci() {
	header "CI Deployment"
	$CI_DEPLOY || {
		skip "skipping ci deployment"
		return 0
	}
	uncomment_patch ci "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
	[[ $CLUSTER_PROVIDER == 'kind' ]] && {
		uncomment_patch kind "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
	}
	ok "CI deployment configured for ${CLUSTER_PROVIDER}"
}
deploy_debug() {
	header "Debug Deployment"
	$DEBUG_DEPLOY || {
		skip "skipping debug deployment"
		return 0
	}
	uncomment_patch debug "${MANIFESTS_OUT_DIR}"/base/kustomization.yaml
	ok "Debug deployment configured"
}
deploy_model_server() {
	header "Model Server Deployment"
	$MODEL_SERVER_DEPLOY || {
		skip "skipping model server deployment"
		return 0
	}
	uncomment_path model-server "${MANIFESTS_OUT_DIR}"/base/kustomization.yaml
	uncomment_patch model-server-kepler-config "${MANIFESTS_OUT_DIR}"/base/kustomization.yaml
	$OPENSHIFT_DEPLOY && {
		uncomment_patch openshift "${MANIFESTS_OUT_DIR}"/model-server/kustomization.yaml
	}
	ok "Model server deployment configured"
}
deploy_qat() {
	header "QAT Deployment"
	$QAT_DEPLOY || {
		skip "skipping qat deployment"
		return 0
	}
	uncomment_patch qat "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
	ok "QAT deployment configured"
}
deploy_dcgm() {
	header "DCGM Deployment"
	$DCGM_DEPLOY || {
		skip "skipping dcgm deployment"
		return 0
	}
	uncomment_patch dcgm "${MANIFESTS_OUT_DIR}"/exporter/kustomization.yaml
	ok "DCGM deployment configured"
}
build_manifest() {
	info "Building manifests ..."
	for deploy in $(declare -F | cut -f3 -d ' ' | grep 'deploy_'); do
		"$deploy" || return 1
	done
	line 50 heavy
	pushd "${MANIFESTS_OUT_DIR}"/exporter
	run kustomize edit set image kepler="${EXPORTER_IMG}"
	run kustomize edit set image kepler_model_server="${MODEL_SERVER_IMG}"
	popd
	pushd "${MANIFESTS_OUT_DIR}"/model-server
	run kustomize edit set image kepler_model_server="${MODEL_SERVER_IMG}"
	popd
	info "kustomize manifests..."
	kustomize build "${MANIFESTS_OUT_DIR}"/base >"${MANIFESTS_OUT_DIR}"/deployment.yaml

	ok "Manifests build successfully."
	info "run kubectl create -f _output/generated-manifest/deployment.yaml to deploy"
	return 0
}

main() {
	export PATH="$PROJECT_ROOT/tmp/bin:$PATH"

	local opts=$1
	shift
	ensure_all_tools

	for opt in ${opts}; do
		info "Setting $opt as True"
		eval "$opt=true"
	done

	info "move to untrack workspace ${MANIFESTS_OUT_DIR}"
	run rm -rf "${MANIFESTS_OUT_DIR}"
	run mkdir -p "${MANIFESTS_OUT_DIR}"
	run cp -r manifests/config/* "${MANIFESTS_OUT_DIR}"/

	build_manifest || {
		fail "Fail to build the manifests"
		return 1
	}
}
main "$@"
