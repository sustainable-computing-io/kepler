#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2025 The Kepler Authors
# SPDX-License-Identifier: Apache-2.0
#
# Sets the "GPU e2e" commit status for a PR run.
#
# usage: status.sh pending
#        status.sh final <gpu-job-result> [<passed> <skipped>]
#
#   <gpu-job-result>  result of the GPU job: success, failure, cancelled, ...
#   <passed>          number of GPU tests that passed
#   <skipped>         number of GPU tests that skipped
#
# Environment:
#   SHA                commit to set the status on
#   RUN_URL            link of the run, shown with the status
#   GITHUB_REPOSITORY  owner/name, set by GitHub Actions
#   GITHUB_RUN_ID      id of this run, set by GitHub Actions
#   GH_TOKEN           token for gh with statuses: write
#
# The workflow runs one run per PR at a time, so two runs never set the status
# at once. A status that links to a newer run is still left alone, which covers
# re-running the jobs of an older run after a newer one.
set -euo pipefail

usage="usage: status.sh pending | status.sh final <gpu-job-result> [<passed> <skipped>]"
context="GPU e2e"
runner="the CNCF A10 runner"

case "${1:-}" in
pending)
	state=pending description="Running on ${runner}"
	;;
final)
	result=${2:?$usage}
	passed=${3:-} skipped=${4:-}
	case "$result" in
	success)
		state=success description="GPU tests passed on ${runner}"
		if [[ "$passed" =~ ^[0-9]+$ ]] && [[ "$skipped" =~ ^[0-9]+$ ]]; then
			description="${passed} GPU tests passed on ${runner}"
			if [ "$skipped" -gt 0 ]; then
				description="${passed} GPU tests passed, ${skipped} skipped on ${runner}"
			fi
		fi
		;;
	failure) state=failure description="GPU e2e failed, see the run summary" ;;
	*) state=error description="The GPU run did not finish (${result})" ;;
	esac
	;;
*)
	echo "$usage" >&2
	exit 2
	;;
esac

latest=$(gh api "repos/${GITHUB_REPOSITORY}/commits/${SHA}/statuses" \
	--jq "map(select(.context == \"${context}\")) | .[0].target_url // \"\"")
latest_run=${latest##*/runs/}
if [[ "$latest_run" =~ ^[0-9]+$ ]] && [ "$latest_run" -gt "$GITHUB_RUN_ID" ]; then
	echo "A newer run, ${latest}, owns the ${context} status, so this run leaves it alone."
	exit 0
fi

gh api "repos/${GITHUB_REPOSITORY}/statuses/${SHA}" \
	-f state="$state" -f context="$context" \
	-f description="$description" -f target_url="$RUN_URL" >/dev/null
echo "Set ${context} on ${SHA:0:12} to ${state}: ${description}"
