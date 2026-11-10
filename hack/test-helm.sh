#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2025 The Kepler Authors
# SPDX-License-Identifier: Apache-2.0

# Requires Bash 4.4+
set -e -u -o pipefail

# Keep the default baseline pinned so publishing a release does not change CI.
declare -r BASELINE_VERSION=${BASELINE_VERSION:-0.12.0}
declare -r IMG_BASE=${IMG_BASE:-localhost:5001}
declare -r VERSION=${VERSION:-dev}
declare -r METRICS_PORT=${METRICS_PORT:-28284}

PROJECT_ROOT="$(git -C "$(dirname "${BASH_SOURCE[0]}")" rev-parse --show-toplevel)"
declare -r PROJECT_ROOT

# shellcheck source=hack/utils.bash
source "$PROJECT_ROOT/hack/utils.bash"

declare TMP_DIR=""
declare PORT_FORWARD_PID=""
declare -a RELEASES=()

print_help() {
	cat <<-EOF
		Usage: $0 [lint|install|upgrade|all]
		       $0 -h|--help

		Commands:
		  lint     Lint the checked-out chart (requires Helm only)
		  install  Test a fresh installation of the checked-out chart and candidate image
		  upgrade  Test default and --reuse-values upgrades from the released chart
		  all      Run lint, install, and upgrade tests (default)

		Deployment tests require helm, kubectl, curl, and a candidate image accessible
		to the cluster. They use the current kubectl context (honor KUBECONFIG) and
		clean up their releases.

		Environment:
		  BASELINE_VERSION  Released chart and image version, without v (default: 0.12.0)
		  IMG_BASE          Candidate image registry/namespace (default: localhost:5001)
		  VERSION           Candidate image tag (default: dev)
		  METRICS_PORT      Local port for metrics validation (default: 28284)
	EOF
}

validate() {
	local cmd="$1"
	local tool
	local -a tools=(helm)
	if [[ "$cmd" != lint ]]; then
		tools+=(kubectl curl)
	fi
	for tool in "${tools[@]}"; do
		command -v "$tool" >/dev/null || die "$tool is required"
	done
}

print_config() {
	header "Helm Test Configuration"
	info "Baseline: v$BASELINE_VERSION"
	info "Candidate image: $IMG_BASE/kepler:$VERSION"
	info "Metrics port: $METRICS_PORT"
	run kubectl config current-context
}

gather_logs() {
	local release="$1"
	header "Collecting diagnostics for $release"
	kubectl get pods -n "$release" -o wide || true
	kubectl get events -n "$release" --sort-by=.lastTimestamp || true
	kubectl logs -n "$release" -l "app.kubernetes.io/instance=$release" --prefix --tail=200 || true
}

cleanup() {
	local -i status=$? cleanup_status=0
	trap - EXIT
	if [[ -n "$PORT_FORWARD_PID" ]]; then
		kill "$PORT_FORWARD_PID" 2>/dev/null || true
		wait "$PORT_FORWARD_PID" 2>/dev/null || true
	fi
	if ((status != 0)); then
		fail "Helm validation failed (exit $status)"
		if [[ -f "$TMP_DIR/port-forward.log" ]]; then
			cat "$TMP_DIR/port-forward.log" || true
		fi
	fi

	local release
	for release in "${RELEASES[@]}"; do
		gather_logs "$release"
		header "Cleaning up $release"
		helm uninstall "$release" -n "$release" --ignore-not-found --wait --timeout=1m || cleanup_status=1
		kubectl delete namespace "$release" --ignore-not-found --timeout=1m || cleanup_status=1
	done
	rm -rf -- "$TMP_DIR" || cleanup_status=1
	if ((status == 0)); then
		status=$cleanup_status
	fi
	exit "$status"
}

metrics_ready() {
	local http_status metric
	# wait_until invokes this as a condition, so failures need explicit returns.
	kill -0 "$PORT_FORWARD_PID" 2>/dev/null || return 1
	# Wait for our listener before querying a potentially occupied local port.
	grep -Fq "Forwarding from 127.0.0.1:$METRICS_PORT -> 28282" "$TMP_DIR/port-forward.log" || return 1
	http_status=$(curl -sS --max-time 5 -o "$TMP_DIR/metrics.txt" -w '%{http_code}' \
		"http://127.0.0.1:$METRICS_PORT/metrics") || return 1
	[[ "$http_status" == 200 ]] || return 1
	for metric in kepler_container_cpu_joules_total kepler_pod_cpu_joules_total; do
		grep -q "^${metric}{" "$TMP_DIR/metrics.txt" || return 1
	done
	return 0
}

validate_permissions() {
	local release="$1"
	local pods pod
	# Exclude terminating pods from the previous revision after an upgrade.
	pods=$(kubectl get pods -n "$release" \
		-l "app.kubernetes.io/name=kepler,app.kubernetes.io/instance=$release" \
		-o go-template='{{range .items}}{{if not .metadata.deletionTimestamp}}{{.metadata.name}}{{"\n"}}{{end}}{{end}}')
	[[ -n "$pods" ]] || die "Helm deployment $release has no Kepler pods"
	while IFS= read -r pod; do
		# Save logs first so grep closing a pipe cannot hide a match under pipefail.
		kubectl logs "$pod" -n "$release" >"$TMP_DIR/kepler.log"
		if grep -q "needs root or CAP_SYS_PTRACE" "$TMP_DIR/kepler.log"; then
			fail "Helm deployment $pod cannot read /proc entries of other users"
			return 1
		fi
	done <<<"$pods"
}

validate_deployment() {
	local release="$1"

	header "Validating $release"
	run kubectl rollout status "daemonset/$release" -n "$release" --timeout=5m
	kubectl port-forward "service/$release" "$METRICS_PORT:28282" -n "$release" \
		--address=127.0.0.1 >"$TMP_DIR/port-forward.log" 2>&1 &
	PORT_FORWARD_PID=$!
	wait_until 30 2 "container/pod metrics for $release" metrics_ready

	kill "$PORT_FORWARD_PID"
	wait "$PORT_FORWARD_PID" || true
	PORT_FORWARD_PID=""
	validate_permissions "$release"
	ok "$release deployment is working"
}

create_namespace() {
	local release="$1"
	run kubectl create namespace "$release"
	RELEASES+=("$release")
}

cmd_lint() {
	header "Linting Helm chart"
	run helm lint manifests/helm/kepler
}

cmd_install() {
	local release="kepler-helm-test"
	header "Testing fresh Helm installation"
	create_namespace "$release"
	run helm install "$release" manifests/helm/kepler --namespace "$release" \
		--set "image.repository=$IMG_BASE/kepler" --set "image.tag=$VERSION" \
		--set 'config.cpu.preferredMeters={fake}' --wait --timeout=5m
	validate_deployment "$release"
}

test_upgrade() {
	local mode="$1"
	local release="kepler-upgrade-$mode"
	local -a upgrade_args=()

	header "Testing $mode upgrade from v$BASELINE_VERSION"
	create_namespace "$release"
	run helm install "$release" "$TMP_DIR/baseline.tgz" --namespace "$release" \
		--set "image.tag=v$BASELINE_VERSION" \
		--set 'config.cpu.preferredMeters={fake}' --wait --timeout=5m

	if [[ "$mode" == reuse ]]; then
		upgrade_args+=(--reuse-values)
	fi
	# Override the image so reused values cannot keep the released binary.
	run helm upgrade "$release" manifests/helm/kepler --namespace "$release" "${upgrade_args[@]}" \
		--set "image.repository=$IMG_BASE/kepler" --set "image.tag=$VERSION" \
		--set 'config.cpu.preferredMeters={fake}' --wait --timeout=5m
	validate_deployment "$release"
}

cmd_upgrade() {
	run curl -fsSL --retry 3 --connect-timeout 10 --max-time 60 \
		"https://github.com/sustainable-computing-io/kepler/releases/download/v${BASELINE_VERSION}/kepler-helm-${BASELINE_VERSION}.tgz" \
		-o "$TMP_DIR/baseline.tgz"

	test_upgrade default
	test_upgrade reuse
}

cmd_all() {
	cmd_lint
	cmd_install
	cmd_upgrade
}

main() {
	local cmd="${1:-all}"
	if [[ $# -gt 1 ]]; then
		err "Unexpected arguments: $*"
		print_help
		return 1
	fi
	case "$cmd" in
	-h | --help)
		print_help
		return 0
		;;
	lint | install | upgrade | all) ;;
	*)
		err "Unknown command: $cmd"
		print_help
		return 1
		;;
	esac

	cd "$PROJECT_ROOT"
	validate "$cmd"
	if [[ "$cmd" == lint ]]; then
		cmd_lint
		return 0
	fi
	print_config

	TMP_DIR=$(mktemp -d)
	trap cleanup EXIT
	trap 'exit 130' INT
	trap 'exit 143' TERM

	"cmd_$cmd"
}

main "$@"
