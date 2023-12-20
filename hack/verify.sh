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
declare -r MANIFESTS_OUT_DIR="${MANIFESTS_OUT_DIR:-_output/generated-manifest}"
declare -r EXPORTER="${EXPORTER:-kepler-exporter}"
declare -r KEPLER_NS="${KEPLER_NS:-kepler}"
declare -r MONITORING_NS="${MONITORING_NS:-monitoring}"
declare -r KUBECONFIG_PATH=${KUBECONFIG_ROOT_DIR:-$HOME/.kube/config}

source "$PROJECT_ROOT/hack/utils.bash"

declare CLUSTER_PROVIDER="${CLUSTER_PROVIDER:-kind}"
declare CTR_CMD="${CTR_CMD:-docker}"
declare ARTIFACT_DIR="${ARTIFACT_DIR:-/tmp/artifacts}"

must_gather() {
	header "Running must gather"

	run kubectl describe nodes | tee "$ARTIFACT_DIR/nodes"
	run kubectl get pods --all-namespaces | tee "$ARTIFACT_DIR/pods"
	run kubectl get deployment --all-namespaces | tee "$ARTIFACT_DIR/deployments"
	run kubectl get daemonsets --all-namespaces | tee "$ARTIFACT_DIR/daemonsets"
	run kubectl get services --all-namespaces | tee "$ARTIFACT_DIR/services"
	run kubectl get endpoints --all-namespaces | tee "$ARTIFACT_DIR/endpoints"
	run kubectl describe daemonset "$EXPORTER" -n "$KEPLER_NS" | tee "$ARTIFACT_DIR/kepler-daemonset-describe"
	run kubectl get pods -n "$KEPLER_NS" -o yaml | tee "$ARTIFACT_DIR/kepler-pod.yaml"
	log_kepler
}

check_deployment_status() {
	header "Checking Kepler status"
	kubectl rollout status daemonset "$EXPORTER" -n "$KEPLER_NS" --timeout 5m || {
		must_gather
		fail "Kepler status invalid ❌"
		return 1
	}
	ok "Kepler status valid 🙂"
	return 0
}

log_kepler() {
	run kubectl logs -n "$KEPLER_NS" daemonset/"$EXPORTER" | tee "$ARTIFACT_DIR/kepler.log"
}
watch_service() {
	local port="$1"
	local ns="$2"
	local svn="$3"
	shift 3
	kubectl port-forward --address localhost -n "$ns" service/"$svn" "$port":"$port"
}

intergration_test() {
	log_kepler &

	local ret=0
	go test ./e2e/integration-test/... -v --race --bench=. -cover --count=1 --vet=all \
		2>&1 | tee "$ARTIFACT_DIR/e2e.log" || ret=1

	# terminate jobs
	{ jobs -p | xargs -I {} -- pkill -TERM -P {}; } || true
	wait
	sleep 1

	return $ret
}

#TODO Optimze platform-validation tests
platform_validation() {
	mkdir -p /tmp/.kube

	if [[ "$CLUSTER_PROVIDER" == "microshift" ]]; then
		run $CTR_CMD exec -i microshift cat /var/lib/microshift/resources/kubeadmin/kubeconfig >/tmp/.kube/config
	else
		run kind get kubeconfig --name=kind >/tmp/.kube/config
	fi

	watch_service 9102 "$KEPLER_NS" "$EXPORTER" &
	watch_service 9090 "$MONITORING_NS" prometheus-k8s &

	local ret=0
	cd ./e2e/platform-validation
	ginkgo -v --json-report=platform_validation_report.json --race -cover --vet=all \
		2>&1 | tee "$ARTIFACT_DIR/e2e-platform.log" || ret=1

	# terminate both jobs
	{ jobs -p | xargs -I {} -- pkill -TERM -P {}; } || true
	wait
	sleep 1

	info "Dump platform-validation.env..."
	run cat platform-validation.env
	info "Dump power.csv..."
	run power.csv
	# cleanup
	run rm -f platform-validation.env power.csv

	return $ret
}

main() {
	mkdir -p "$ARTIFACT_DIR"
	export KUBECONFIG="$KUBECONFIG_PATH"

	local ret=0
	case "${1-}" in
	kepler)
		check_deployment_status
		;;
	integration)
		intergration_test || {
			ret=1
			fail "Kepler integration test failed ❌"
			line 50
		}
		;;
	platform)
		platform_validation || {
			ret=1
			fail "Kepler platform validation failed ❌"
			line 50
		}
		;;
	*)
		die "invalid args"
		;;
	esac
	return $ret
}

main "$@"
