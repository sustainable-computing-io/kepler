#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2025 The Kepler Authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

die() {
	printf '%s\n' "$*" >&2
	exit 1
}

if [[ ! ${RELEASE_TAG:-} =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
	die "RELEASE_TAG must be a stable version such as v1.2.3"
fi
version=${RELEASE_TAG#v}
if [[ ${1:-} == version ]]; then
	printf '%s\n' "$version"
	exit 0
fi

die "Usage: $0 version"
