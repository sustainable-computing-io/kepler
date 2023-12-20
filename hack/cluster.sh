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

# NOTE: assumes that the project root is one level up
PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." >/dev/null 2>&1 && pwd)
declare -r PROJECT_ROOT

# NOTE: this allows common settings to be stored as `.env` file
# shellcheck disable=SC1091
[[ -f "$PROJECT_ROOT/.env" ]] && source "$PROJECT_ROOT/.env"

# NOTE: these settings can be overridden in the .env file
declare -r LOCAL_DEV_CLUSTER_DIR="${LOCAL_DEV_CLUSTER_DIR:-"$PROJECT_ROOT/local-dev-cluster"}"
declare -r LOCAL_DEV_CLUSTER_VERSION="${LOCAL_DEV_CLUSTER_VERSION:-v0.0.3}"
declare -r KIND_WORKER_NODES=${KIND_WORKER_NODES:-2}
# Supported CLUSTER_PROVIDER are kind,microshift
export CLUSTER_PROVIDER=${CLUSTER_PROVIDER:-kind}
export KIND_WORKER_NODES

clone_local_dev_cluster() {
	if [ -d "$LOCAL_DEV_CLUSTER_DIR" ]; then
		echo "using local local-dev-cluster"
		return 0
	fi

	echo "downloading local-dev-cluster"
	git clone -b "$LOCAL_DEV_CLUSTER_VERSION" \
		https://github.com/sustainable-computing-io/local-dev-cluster.git \
		--depth=1 \
		"$LOCAL_DEV_CLUSTER_DIR"
}

main() {
	local op="$1"
	shift

	cd "$PROJECT_ROOT"

	clone_local_dev_cluster
	echo "Bringing cluster - $op"
	"$LOCAL_DEV_CLUSTER_DIR/main.sh" "$op"
}

main "$@"
