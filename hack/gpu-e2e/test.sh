#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2025 The Kepler Authors
# SPDX-License-Identifier: Apache-2.0
#
# Tests the GPU e2e workflow scripts without a GPU or the GitHub API.
#
# usage: test.sh            run the tests
#        UPDATE=1 test.sh   rewrite the expected summaries, testdata/*.want
#
# summary.sh runs on each test output in testdata/ and must write exactly the
# matching testdata/<case>.want, the Markdown it is expected to write. target.sh, status.sh and issue.sh run against a
# fake gh that answers from environment variables and logs what it is asked to
# post. nvidia-smi is replaced by one that fails, so no real GPU is read.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
data="${here}/testdata"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

failures=0
fail() {
	echo "FAIL: $*"
	failures=$((failures + 1))
}
pass() {
	echo "ok:   $*"
}

# Fake tools, first on PATH.
bin="${work}/bin"
mkdir -p "$bin"
cat >"${bin}/nvidia-smi" <<'EOF'
#!/usr/bin/env bash
exit 9
EOF
cat >"${bin}/gh" <<'EOF'
#!/usr/bin/env bash
# Answers the calls the scripts make, from FAKE_* variables. Writes are logged
# to FAKE_GH_LOG instead of being sent.
echo "gh $*" >>"$FAKE_GH_LOG"
case "$*" in
"api repos/"*"/pulls/"*) echo "${FAKE_PR_STATE} ${FAKE_PR_HEAD}" ;;
"api repos/"*"/commits/"*"/statuses"*) echo "${FAKE_LATEST_STATUS_URL:-}" ;;
"api repos/"*"/statuses/"*) ;;
"api repos/"*"/actions/runs/"*"/jobs")
	if [ -n "${FAKE_JOBS_JSON:-}" ]; then echo "$FAKE_JOBS_JSON"; else echo '{"jobs": []}'; fi
	;;
"issue list"*) echo "${FAKE_OPEN_ISSUE:-}" ;;
*) echo "fake gh: unexpected call: $*" >&2; exit 1 ;;
esac
EOF
cat >"${bin}/curl" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "${FAKE_JOB_LOG:-}"
EOF
chmod +x "${bin}"/*
export PATH="${bin}:${PATH}"
export FAKE_GH_LOG="${work}/gh.log"

export GITHUB_SERVER_URL=https://github.com
export GITHUB_REPOSITORY=sustainable-computing-io/kepler
export GITHUB_API_URL=https://api.github.com
export GITHUB_RUN_ID=1000
export GITHUB_REF_NAME=main
export GITHUB_SHA=1111111111111111111111111111111111111111
export GH_TOKEN=fake
unset GITHUB_OUTPUT GPU_E2E_EXIT_CODE

# --- summary.sh: one case per test output, with the test binary's exit code
# ("-" for none, as when the run was cut short or the script runs locally).
while read -r name code samples; do
	[ -n "$name" ] || continue
	got="${work}/${name}.md"
	(
		export GPU_E2E_TARGET="PR #1234"
		export GPU_E2E_SHA=0123456789abcdef0123456789abcdef01234567
		export GPU_E2E_RUNNER=oracle-vm-gpu-a10-1
		export GPU_E2E_ARTIFACT=gpu-e2e-logs
		if [ "$code" != - ]; then export GPU_E2E_EXIT_CODE="$code"; fi
		args=("${data}/${name}.log")
		if [ "$samples" != - ]; then args+=("${data}/${samples}"); fi
		bash "${here}/summary.sh" "${args[@]}" </dev/null
	) >"$got"
	if [ -n "${UPDATE:-}" ]; then
		cp "$got" "${data}/${name}.want"
		pass "summary ${name} (updated)"
	elif diff -u "${data}/${name}.want" "$got" >"${work}/diff"; then
		pass "summary ${name}"
	else
		fail "summary ${name} differs from testdata/${name}.want:"
		cat "${work}/diff"
	fi
done <<'EOF'
passed 0 power.csv
failed 1 -
skipped 0 -
interrupted - -
race - -
notests 0 -
EOF

# A missing test output means the tests never started.
if grep -qxF '## ⚠️ GPU e2e did not run' <<<"$(bash "${here}/summary.sh" "${work}/missing.log")"; then
	pass "summary without test output"
else
	fail "summary without test output"
fi

# --- target.sh
expect_target() {
	local name=$1 want_code=$2 want=$3
	local out code=0
	out=$(bash "${here}/target.sh" 2>&1) || code=$?
	if [ "$code" = "$want_code" ] && grep -qF -- "$want" <<<"$out"; then
		pass "target ${name}"
	else
		fail "target ${name}: exit ${code}, want ${want_code} and \"${want}\", got:"
		echo "$out"
	fi
}
export FAKE_PR_STATE=open FAKE_PR_HEAD=abcdef0123456789abcdef0123456789abcdef01
PR="" REVIEWED_SHA="" expect_target "without a PR tests the branch" 0 "target=main"
PR=12x REVIEWED_SHA=abcdef0 expect_target "rejects a bad PR number" 1 "digits only"
PR=12 REVIEWED_SHA="" expect_target "requires the reviewed commit" 1 "Give the commit of PR #12 you reviewed"
PR=12 REVIEWED_SHA=xyz1234 expect_target "rejects a bad commit" 1 "Give the commit of PR #12 you reviewed"
PR=12 REVIEWED_SHA=ABCDEF0 expect_target "accepts the reviewed head" 0 "sha=abcdef0123456789abcdef0123456789abcdef01"
PR=12 REVIEWED_SHA=1234567 expect_target "stops when the PR moved" 1 "moved from 1234567"
FAKE_PR_STATE=closed PR=12 REVIEWED_SHA=abcdef0 expect_target "stops when the PR is closed" 1 "is closed, not open"

# --- status.sh
expect_status() {
	local name=$1 want=$2
	shift 2
	: >"$FAKE_GH_LOG"
	local out
	out=$(SHA=abcdef0123456789abcdef0123456789abcdef01 RUN_URL=https://example.test/runs/1000 \
		bash "${here}/status.sh" "$@" 2>&1) || true
	if grep -qF -- "$want" "$FAKE_GH_LOG" <(echo "$out"); then
		pass "status ${name}"
	else
		fail "status ${name}: want \"${want}\", got:"
		echo "$out"
		cat "$FAKE_GH_LOG"
	fi
}
expect_status "pending" "state=pending" pending
expect_status "counts passed and skipped tests" "description=6 GPU tests passed, 1 skipped on the CNCF A10 runner" final success 6 1
expect_status "failure" "state=failure" final failure
expect_status "cancelled is an error" "state=error" final cancelled
FAKE_LATEST_STATUS_URL=https://github.com/sustainable-computing-io/kepler/actions/runs/999 \
	expect_status "replaces an older run's status" "state=success" final success 7 0
FAKE_LATEST_STATUS_URL=https://github.com/sustainable-computing-io/kepler/actions/runs/1001 \
	expect_status "leaves a newer run's status alone" "owns the GPU e2e status" final success 7 0
if grep -qF -- "-f state=" "$FAKE_GH_LOG"; then
	fail "status for a newer run was overwritten"
fi

# --- issue.sh, in dry-run mode
expect_issue() {
	local name=$1 want=$2
	local out
	out=$(GPU_E2E_DRY_RUN=1 SHA=abcdef0123456789abcdef0123456789abcdef01 \
		RUN_URL=https://example.test/runs/1000 COMMIT_URL=https://example.test/commit \
		WORKFLOW_URL=https://example.test/gpu-e2e.yaml bash "${here}/issue.sh" "${work}/failed.md" 2>&1) || true
	if grep -qF -- "$want" <<<"$out"; then
		pass "issue ${name}"
	else
		fail "issue ${name}: want \"${want}\", got:"
		echo "$out"
	fi
}
cp "${data}/failed.want" "${work}/failed.md"
GPU_RESULT=failure expect_issue "opens a new issue" 'Would open the issue "Weekly GPU e2e run failed"'
GPU_RESULT=failure expect_issue "carries the failing test" "Why <code>TestGPUPowerMatchesNvidiaSMI</code> failed"
GPU_RESULT=failure FAKE_OPEN_ISSUE=77 expect_issue "comments on the open issue" "Would comment on issue #77"
GPU_RESULT=cancelled FAKE_OPEN_ISSUE=77 expect_issue "says when a run did not finish" "Did not finish this time"
: >"${work}/failed.md"
GPU_RESULT=failure FAKE_JOBS_JSON='{"jobs": [{"id": 5, "name": "GPU tests on the A10", "html_url": "https://example.test/job/5", "steps": [{"name": "Build Kepler and the e2e test binary", "conclusion": "failure"}]}]}' \
	FAKE_JOB_LOG=$'##[group]Run make\nmake test-e2e\n##[endgroup]\nundefined: foo\n##[error]Process completed with exit code 2.' \
	expect_issue "shows the end of the failed step's log" "undefined: foo"

echo
if [ "$failures" -gt 0 ]; then
	echo "${failures} check(s) failed"
	exit 1
fi
echo "all checks passed"
