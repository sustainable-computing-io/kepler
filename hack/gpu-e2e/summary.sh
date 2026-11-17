#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2025 The Kepler Authors
# SPDX-License-Identifier: Apache-2.0
#
# Writes the GPU e2e run summary as Markdown, like hack/reports/profiling.sh
# does for profiling: the result, the GPU, each test's result and the end of
# the test output, with a link to the run's logs.
#
# usage: summary.sh <test-output>
#
# Environment:
#   GPU_E2E_OUTCOME  outcome of the test step: success, failure, cancelled or skipped
#   GPU_E2E_TARGET   what was tested, such as "PR #1234" or "main"
#   GPU_E2E_SHA      commit tested
#   RUN_URL          link of the run
set -eu -o pipefail

log=${1:?usage: summary.sh <test-output>}
outcome=${GPU_E2E_OUTCOME:-unknown}

case "$outcome" in
success) echo "## ✅ GPU e2e passed" ;;
failure) echo "## ❌ GPU e2e failed" ;;
*) echo "## ⚠️ GPU e2e did not finish (${outcome})" ;;
esac
echo ""
echo "Tested ${GPU_E2E_TARGET:-unknown} at \`${GPU_E2E_SHA:-unknown}\` on the CNCF A10 runner."
echo ""
# nvidia-smi can hang on a broken driver, so the lookup has a time limit.
gpu=$(timeout 10 nvidia-smi --query-gpu=name,driver_version --format=csv,noheader 2>/dev/null || true)
echo "GPU and driver: ${gpu:-unknown}"
echo ""

if [[ ! -s "$log" ]]; then
	echo "❌ The tests did not run. The run's logs show the step that failed."
else
	results=$(grep -E '^--- (PASS|FAIL|SKIP): TestGPU' "$log" | sed -E 's/^--- ([A-Z]+): /- \1 /' || true)
	echo "${results:-No GPU test finished.}"
	echo ""
	echo "<details>"
	echo "<summary>📄 End of the test output</summary>"
	echo ""
	echo "\`\`\`"
	tail -n 200 "$log"
	echo "\`\`\`"
	echo "</details>"
fi
echo ""
echo "📜 Full logs: [Actions run](${RUN_URL:-})"
