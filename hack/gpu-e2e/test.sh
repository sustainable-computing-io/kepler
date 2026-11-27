#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2025 The Kepler Authors
# SPDX-License-Identifier: Apache-2.0
#
# Tests the GPU e2e workflow scripts without a GPU or the GitHub API.
#
# usage: test.sh
#
# target.sh and status.sh run against a fake gh that answers from environment
# variables and logs what it is asked to post. nvidia-smi is replaced by one
# that fails, so no real GPU is read.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
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
cat >"${bin}/nvidia-smi" <<'EOS'
#!/usr/bin/env bash
exit 9
EOS
cat >"${bin}/gh" <<'EOS'
#!/usr/bin/env bash
# Answers the calls the scripts make, from FAKE_* variables. Writes are logged
# to FAKE_GH_LOG instead of being sent.
echo "gh $*" >>"$FAKE_GH_LOG"
case "$*" in
"api repos/"*"/pulls/"*) echo "${FAKE_PR_STATE} ${FAKE_PR_HEAD}" ;;
"api repos/"*"/contents/hack/gpu-e2e/"*) [ "${FAKE_PR_HAS_SCRIPTS:-yes}" = yes ] ;;
"api repos/"*"/commits/"*"/statuses"*) echo "${FAKE_LATEST_STATUS_URL:-}" ;;
"api repos/"*"/statuses/"*) ;;
*) echo "fake gh: unexpected call: $*" >&2; exit 1 ;;
esac
EOS
chmod +x "${bin}"/*
export PATH="${bin}:${PATH}"
export FAKE_GH_LOG="${work}/gh.log"

export GITHUB_REPOSITORY=sustainable-computing-io/kepler
export GITHUB_RUN_ID=1000
export GITHUB_REF_NAME=main
export GITHUB_SHA=1111111111111111111111111111111111111111
export GH_TOKEN=fake
unset GITHUB_OUTPUT

# --- summary.sh
expect_summary() {
	local name=$1 outcome=$2 log=$3 want=$4
	local out
	out=$(GPU_E2E_OUTCOME="$outcome" GPU_E2E_TARGET="PR #12" GPU_E2E_SHA=abc RUN_URL=https://example.test/runs/1000 \
		bash "${here}/summary.sh" "$log" 2>&1) || true
	if grep -qF -- "$want" <<<"$out"; then
		pass "summary ${name}"
	else
		fail "summary ${name}: want \"${want}\", got:"
		echo "$out"
	fi
}
printf '%s\n' '=== RUN   TestGPUNodeMetricsPresent' '--- PASS: TestGPUNodeMetricsPresent (2.98s)' \
	'=== RUN   TestGPUIdle' '--- FAIL: TestGPUIdle (1.00s)' 'FAIL' >"${work}/gpu-e2e.log"
expect_summary "passed" success "${work}/gpu-e2e.log" "## ✅ GPU e2e passed"
expect_summary "failed" failure "${work}/gpu-e2e.log" "## ❌ GPU e2e failed"
expect_summary "cancelled" cancelled "${work}/gpu-e2e.log" "## ⚠️ GPU e2e did not finish (cancelled)"
expect_summary "lists each test" failure "${work}/gpu-e2e.log" "- FAIL TestGPUIdle (1.00s)"
expect_summary "without the GPU" success "${work}/gpu-e2e.log" "GPU and driver: unknown"
expect_summary "links the run" failure "${work}/gpu-e2e.log" "[Actions run](https://example.test/runs/1000)"
expect_summary "without test output" skipped "${work}/missing.log" "The tests did not run"

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
head_sha=abcdef0123456789abcdef0123456789abcdef01
export FAKE_PR_STATE=open FAKE_PR_HEAD=$head_sha
PR="" REVIEWED_SHA="" expect_target "without a PR tests the branch" 0 "target=main"
PR=12x REVIEWED_SHA=$head_sha expect_target "rejects a bad PR number" 1 "digits only"
PR=12 REVIEWED_SHA="" expect_target "requires the reviewed commit" 1 "Give the full commit of PR #12 you reviewed"
PR=12 REVIEWED_SHA=abcdef0 expect_target "rejects a short commit" 1 "Give the full commit of PR #12 you reviewed"
PR=12 REVIEWED_SHA=ABCDEF0123456789ABCDEF0123456789ABCDEF01 expect_target "accepts the reviewed head" 0 "sha=${head_sha}"
PR=12 REVIEWED_SHA=abcdef0123456789abcdef0123456789abcdef02 expect_target "stops when the PR moved" 1 "moved from abcdef012345"
FAKE_PR_STATE=closed PR=12 REVIEWED_SHA=$head_sha expect_target "stops when the PR is closed" 1 "is closed, not open"
FAKE_PR_HAS_SCRIPTS=no PR=12 REVIEWED_SHA=$head_sha expect_target "stops when the PR lacks the scripts" 1 "does not have hack/gpu-e2e/ yet"

# --- status.sh
expect_status() {
	local name=$1 want=$2
	shift 2
	: >"$FAKE_GH_LOG"
	local out
	out=$(SHA=$head_sha RUN_URL=https://example.test/runs/1000 \
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

echo
if [ "$failures" -gt 0 ]; then
	echo "${failures} check(s) failed"
	exit 1
fi
echo "all checks passed"
