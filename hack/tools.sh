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
# Copyright 2023 The Kepler Contributors
#

set -eu -o pipefail

# constants
PROJECT_ROOT="$(git rev-parse --show-toplevel)"
GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"

declare -r PROJECT_ROOT GOOS GOARCH
declare -r LOCAL_BIN="$PROJECT_ROOT/tmp/bin"

# tools
declare -r KUBECTL_VERSION=${KUBECTL_VERSION:-v1.28.4}
declare -r KUSTOMIZE_VERSION=${KUSTOMIZE_VERSION:-v4.5.2}
declare -r KUSTOMIZE_INSTALL_SCRIPT="https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"

declare -r JQ_VERSION=${JQ_VERSION:-1.7}
declare -r JQ_INSTALL_URL="https://github.com/jqlang/jq/releases/download/jq-$JQ_VERSION"

source "$PROJECT_ROOT/hack/utils.bash"

validate_version() {
	local cmd="$1"
	local version_arg="$2"
	local version_regex="$3"
	shift 3

	command -v "$cmd" >/dev/null 2>&1 || return 1
	[[ $(eval "$cmd $version_arg" | grep -o "$version_regex") =~ $version_regex ]] || {
		return 1
	}

	ok "$cmd installed successfully"
}

install_kustomize() {
	command -v kustomize >/dev/null 2>&1 &&
		[[ $(kustomize version --short | grep -o 'v[0-9].[0-9].[0-9]') == "$KUSTOMIZE_VERSION" ]] && {
		ok "kustomize $KUSTOMIZE_VERSION is already installed"
		return 0
	}

	info "installing kustomize version: $KUSTOMIZE_VERSION"
	(
		# NOTE: this handles softlinks properly
		cd "$LOCAL_BIN"
		curl -Ss $KUSTOMIZE_INSTALL_SCRIPT | bash -s -- "${KUSTOMIZE_VERSION:1}" .
	) || {
		fail "failed to install kustomize"
		return 1
	}
	ok "kustomize was installed successfully"
}

go_install() {
	local pkg="$1"
	local version="$2"
	shift 2

	info "installing $pkg version: $version"

	GOBIN=$LOCAL_BIN \
		go install "$pkg@$version" || {
		fail "failed to install $pkg - $version"
		return 1
	}
	ok "$pkg - $version was installed successfully"

}

curl_install() {
	local binary="$1"
	local url="$2"
	shift 2

	info "installing $binary"
	curl -sSLo "$LOCAL_BIN/$binary" "$url" || {
		fail "failed to install $binary"
		return 1
	}

	chmod +x "$LOCAL_BIN/$binary"
	ok "$binary was installed successfully"
}

install_jq() {
	validate_version jq --version "$JQ_VERSION" && {
		return 0
	}
	local os="$GOOS"
	[[ $os == "darwin" ]] && os="macos"

	curl_install jq "$JQ_INSTALL_URL/jq-$os-$GOARCH"
}

install_govulncheck() {
	go_install golang.org/x/vuln/cmd/govulncheck latest
}

install_kubectl() {
	local version_regex="Client Version: $KUBECTL_VERSION"

	validate_version kubectl "version --client" "$version_regex" && return 0

	info "installing kubectl version: $KUBECTL_VERSION"
	local install_url="https://dl.k8s.io/release/$KUBECTL_VERSION/bin/$GOOS/$GOARCH/kubectl"

	curl -Lo "$LOCAL_BIN/kubectl" "$install_url" || {
		fail "failed to install kubectl"
		return 1
	}
	chmod +x "$LOCAL_BIN/kubectl"
	ok "kubectl - $KUBECTL_VERSION was installed successfully"

}

install_all() {
	info "installing all tools ..."
	local ret=0
	for tool in $(declare -F | cut -f3 -d ' ' | grep install_ | grep -v 'install_all'); do
		"$tool" || ret=1
	done
	return $ret
}

main() {
	local op="${1:-all}"
	shift || true

	mkdir -p "$LOCAL_BIN"
	export PATH="$LOCAL_BIN:$PATH"
	install_"$op"
}

main "$@"
