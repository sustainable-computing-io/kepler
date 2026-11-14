#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2025 The Kepler Authors
# SPDX-License-Identifier: Apache-2.0
#
# Opens an issue for a weekly GPU e2e run that failed or did not finish, or
# comments on the one already open.
#
# usage: issue.sh <run-summary.md>
#
#   <run-summary.md>  the summary the GPU job wrote, or a missing file when the
#                     GPU job did not get to write one
#
# Environment:
#   GPU_RESULT         result of the GPU job: failure, cancelled, skipped, ...
#   SHA                commit tested
#   RUN_URL            link of the run
#   COMMIT_URL         link of the commit tested
#   WORKFLOW_URL       link of the workflow file
#   GITHUB_REPOSITORY  owner/name, set by GitHub Actions
#   GITHUB_RUN_ID      id of this run, set by GitHub Actions
#   GITHUB_API_URL     API base URL, set by GitHub Actions
#   GH_TOKEN           token for gh with actions: read and issues: write
#   GPU_E2E_DRY_RUN    when set, print the issue instead of posting it
set -euo pipefail

summary=${1:?usage: issue.sh <run-summary.md>}
title="Weekly GPU e2e run failed"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# The GPU job is skipped when the quick check failed; then that job's log
# shows why.
job_name="GPU tests on the A10"
if [ "$GPU_RESULT" = skipped ]; then
	job_name="Quick check"
fi
if [ "$GPU_RESULT" = cancelled ]; then
	outcome="did not finish (cancelled or timed out)" during="during"
	again="Did not finish this time (cancelled or timed out)."
else
	outcome="failed" during="in"
	again="Failed again."
fi

job=$(gh api "repos/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}/jobs" |
	jq -r --arg name "$job_name" '.jobs[] | select(.name == $name)
		| "\(.id)\t\(.html_url)\t\([.steps[] | select(.conclusion == "failure" or .conclusion == "cancelled") | .name][0] // "")"' || true)
job_id="" job_url="" failed_step=""
IFS=$'\t' read -r job_id job_url failed_step <<<"$job" || true

# Without per-test failure details in the summary (a build error, a crash, a
# timeout), show the end of the failed step's log instead. Job logs expire
# with the run, so the excerpt is what stays.
excerpt=""
if [ -n "$job_id" ] && ! grep -qs '<summary><strong>Why <code>' "$summary"; then
	# curl, not gh api: newer gh refuses to print the log's color codes.
	excerpt=$(curl -sfL --max-time 60 -H "Authorization: Bearer ${GH_TOKEN}" \
		"${GITHUB_API_URL}/repos/${GITHUB_REPOSITORY}/actions/jobs/${job_id}/logs" |
		sed -E -e 's/^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z ?//' -e $'s/\x1b\\[[0-9;]*[A-Za-z]//g' |
		awk '
			function show() { for (i = (n > 40 ? n - 39 : 1); i <= n; i++) print buf[i] }
			# A step starts with its script in a "Run" group; drop that.
			/^##\[group\]Run / { header = 1; next }
			/^##\[endgroup\]/ { if (header) n = 0; header = 0; next }
			header { next }
			{ sub(/^##\[group\]/, ""); buf[++n] = $0 }
			/^##\[error\](Process completed|The action|The operation)/ { show(); done = 1; exit }
			END { if (!done) show() }' || true)
fi

body="${work}/body.md"
{
	echo "The weekly GPU e2e run on \`main\` ${outcome}${failed_step:+ ${during} the step \`${failed_step}\`}."
	echo
	echo "| | |"
	echo "| --- | --- |"
	echo "| **Run** | [${GITHUB_RUN_ID}](${RUN_URL})${job_url:+, [${job_name} log](${job_url})} |"
	echo "| **Commit** | [\`${SHA:0:7}\`](${COMMIT_URL}) on \`main\` |"
	echo "| **Logs** | [\`gpu-e2e-logs\`](${RUN_URL}#artifacts), kept for 7 days: the test output with Kepler's logs, Kepler's GPU metrics every 3 s and \`nvidia-smi\` power every second |"
	echo
	if [ -n "$excerpt" ]; then
		echo "<details open><summary><strong>End of the log of <code>${failed_step:-${job_name}}</code></strong></summary>"
		echo
		echo '```text'
		printf '%s\n' "$excerpt"
		echo '```'
		echo
		echo "</details>"
		echo
	fi
	if [ -s "$summary" ]; then
		cat "$summary"
	else
		echo "There is no run summary: the GPU job did not get to write one, for example because the quick check failed, the job was cancelled before it started, or its runner went away. The run page shows why."
	fi
} >"$body"

# An issue body or comment holds at most 65536 characters. Cut at a line, then
# close the code block and the <details> it was in.
if [ "$(wc -c <"$body")" -gt 60000 ]; then
	head -c 55000 "$body" | sed '$d' >"${body}.cut"
	fences=$(grep -c '^```' "${body}.cut" || true)
	opened=$(grep -c '<details' "${body}.cut" || true)
	closed=$(grep -c '</details>' "${body}.cut" || true)
	{
		if [ $((fences % 2)) -eq 1 ]; then echo '```'; fi
		for ((i = closed; i < opened; i++)); do printf '\n</details>\n'; done
		printf '\nThe rest is cut, the full summary is on the [run page](%s).\n' "$RUN_URL"
	} >>"${body}.cut"
	mv "${body}.cut" "$body"
fi

number=$(gh issue list --repo "$GITHUB_REPOSITORY" --state open --search "\"${title}\" in:title" \
	--json number,title --jq "map(select(.title == \"${title}\")) | .[0].number // empty")
if [ -n "$number" ]; then
	{
		echo "$again"
		echo
		cat "$body"
	} >"${work}/comment.md"
	if [ -n "${GPU_E2E_DRY_RUN:-}" ]; then
		echo "Would comment on issue #${number}:"
		cat "${work}/comment.md"
	else
		gh issue comment "$number" --repo "$GITHUB_REPOSITORY" --body-file "${work}/comment.md"
	fi
else
	{
		cat "$body"
		echo
		echo "---"
		echo "Written by the weekly run of the [GPU e2e workflow](${WORKFLOW_URL}). Later failures are added as comments; close this issue once \`main\` passes again."
	} >"${work}/new-issue.md"
	if [ -n "${GPU_E2E_DRY_RUN:-}" ]; then
		echo "Would open the issue \"${title}\":"
		cat "${work}/new-issue.md"
	else
		gh issue create --repo "$GITHUB_REPOSITORY" --title "$title" --body-file "${work}/new-issue.md"
	fi
fi
