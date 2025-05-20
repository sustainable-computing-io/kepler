#!/usr/bin/env bash

set -eu -o pipefail

# config
declare -r VERSION=${VERSION:-v0.0.9}
declare -r CLUSTER_PROVIDER=${CLUSTER_PROVIDER:-kind}
declare -r GRAFANA_ENABLE=${GRAFANA_ENABLE:-false}
declare -r KIND_WORKER_NODES=${KIND_WORKER_NODES:-2}

# constants
PROJECT_ROOT="$(git rev-parse --show-toplevel)"
declare -r PROJECT_ROOT
declare -r TMP_DIR="$PROJECT_ROOT/tmp"
declare -r DEV_CLUSTER_DIR="$TMP_DIR/local-dev-cluster"
declare -r BIN_DIR="$TMP_DIR/bin"

source "$PROJECT_ROOT/hack/utils.bash"

git_checkout() {

	[[ -d "$DEV_CLUSTER_DIR" ]] || {
		info "git cloning local-dev-cluster - version $VERSION"
		run git clone -b "$VERSION" \
			https://github.com/sustainable-computing-io/local-dev-cluster.git \
			"$DEV_CLUSTER_DIR"
		return $?
	}

	cd "$DEV_CLUSTER_DIR"

	# NOTE: bail out if the git status is dirty as changes will be overwritten by git reset
	git diff --shortstat --exit-code >/dev/null || {
		err "local-dev-cluster has been modified"
		info "save/discard the changes and rerun the command"
		return 1
	}

	run git fetch --tags
	if [[ "$(git cat-file -t "$VERSION")" == tag ]]; then
		run git reset --hard "$VERSION"
	else
		run git reset --hard "origin/$VERSION"
	fi
}

on_cluster_up() {
	info 'Next: "make deploy" to run Kepler'
}

on_cluster_restart() {
	on_cluster_up
}

on_cluster_down() {
	info "all done"
}

main() {
	local op="$1"
	shift
	cd "$PROJECT_ROOT"
	export PATH="$BIN_DIR:$PATH"
	mkdir -p "${TMP_DIR}"

	header "Running Cluster Setup Script for $op"
	git_checkout
	export CLUSTER_PROVIDER
	export GRAFANA_ENABLE
	export KIND_WORKER_NODES
	cd "$DEV_CLUSTER_DIR"
	"$DEV_CLUSTER_DIR/main.sh" "$op"

	# NOTE: take additional actions after local-dev-cluster performs the "$OP"
	cd "$PROJECT_ROOT"
	on_cluster_"$op"
}

main "$@"
