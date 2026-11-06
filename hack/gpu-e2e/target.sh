#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2025 The Kepler Authors
# SPDX-License-Identifier: Apache-2.0
#
# Resolves what a GPU e2e run tests, for the workflow's quick check.
#
# Without a PR number, the run tests the branch the workflow runs on. With
# one, it needs the commit of the PR that was reviewed, and stops if the PR is
# not open or has new commits since, so only reviewed code reaches the GPU
# runner, where the tests run with sudo.
#
# Environment:
#   PR                 PR number, or empty
#   REVIEWED_SHA       commit of the PR that was reviewed, required with PR
#   GITHUB_REPOSITORY  owner/name, set by GitHub Actions
#   GITHUB_REF_NAME    branch tested without a PR, set by GitHub Actions
#   GITHUB_SHA         commit tested without a PR, set by GitHub Actions
#   GH_TOKEN           token for gh with pull-requests: read
#
# Writes sha=, target= and pr= lines to $GITHUB_OUTPUT, or to stdout.
set -euo pipefail

out=${GITHUB_OUTPUT:-/dev/stdout}
pr=${PR:-}
reviewed=$(printf '%s' "${REVIEWED_SHA:-}" | tr '[:upper:]' '[:lower:]')

if [ -z "$pr" ]; then
	echo "Testing ${GITHUB_REF_NAME} at ${GITHUB_SHA}"
	printf 'sha=%s\ntarget=%s\npr=\n' "$GITHUB_SHA" "$GITHUB_REF_NAME" >>"$out"
	exit 0
fi

if ! [[ "$pr" =~ ^[0-9]+$ ]]; then
	echo "::error::The PR number must be digits only, got '${pr}'"
	exit 1
fi
if ! [[ "$reviewed" =~ ^[0-9a-f]{7,40}$ ]]; then
	echo "::error::Give the commit of PR #${pr} you reviewed (7 to 40 hex characters), got '${reviewed}'"
	exit 1
fi

info=$(gh api "repos/${GITHUB_REPOSITORY}/pulls/${pr}" --jq '"\(.state) \(.head.sha)"')
read -r state head <<<"$info"
if [ "$state" != open ]; then
	echo "::error::PR #${pr} is ${state}, not open"
	exit 1
fi
if [[ "$head" != "$reviewed"* ]]; then
	echo "::error::PR #${pr} moved from ${reviewed} to ${head:0:12}. Review the new commits and run again."
	exit 1
fi

echo "Testing PR #${pr} at ${head}"
printf 'sha=%s\ntarget=PR #%s\npr=%s\n' "$head" "$pr" "$pr" >>"$out"
