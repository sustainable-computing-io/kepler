#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2025 The Kepler Authors
# SPDX-License-Identifier: Apache-2.0
#
# Writes a Markdown summary of a GPU e2e run, for the GitHub Actions run page.
#
# usage: gpu-e2e-summary.sh <test-output> [<power-samples.csv>]
#
#   <test-output>        output of: kepler-e2e.test -test.v -gpu.real
#   <power-samples.csv>  one "seconds,watts" line per nvidia-smi sample
#
# Optional environment:
#   GPU_E2E_TARGET     what was tested, e.g. "PR #2600" (default: the git ref)
#   GPU_E2E_SHA        commit tested (default: GITHUB_SHA)
#   GPU_E2E_RUNNER     runner label shown in the summary
#   GPU_E2E_EXIT_CODE  exit code of the test binary
#   GPU_E2E_ARTIFACT   name of the artifact holding the full logs
set -euo pipefail

log=${1:?usage: gpu-e2e-summary.sh <test-output> [<power-samples.csv>]}
samples=${2:-}

if [ ! -s "$log" ]; then
	echo "## ⚠️ GPU e2e did not run"
	echo
	echo "There is no test output. An earlier step failed before the tests started; its log shows why."
	exit 0
fi

server=${GITHUB_SERVER_URL:-https://github.com}
repo=${GITHUB_REPOSITORY:-sustainable-computing-io/kepler}
sha=${GPU_E2E_SHA:-${GITHUB_SHA:-}}
target=${GPU_E2E_TARGET:-${GITHUB_REF_NAME:-unknown}}
runner=${GPU_E2E_RUNNER:-${RUNNER_NAME:-unknown}}
exit_code=${GPU_E2E_EXIT_CODE:-}
artifact=${GPU_E2E_ARTIFACT:-}

# describe returns what a test checks, in plain words.
describe() {
	case "$1" in
	TestGPURealHardware) echo "Kepler reads a real NVIDIA GPU that draws power" ;;
	TestGPUPowerMatchesNvidiaSMI) echo "Kepler's GPU watts match \`nvidia-smi\`" ;;
	TestGPUWorkloadAttribution) echo "a GPU load's power goes to its process and container" ;;
	TestGPUNodeMetricsPresent) echo "GPU node metrics are exported" ;;
	TestGPUNodeMetricsLabels) echo "GPU metrics carry the gpu, uuid, name and vendor labels" ;;
	TestGPUNodeMetricsNonNegative) echo "no GPU power value is negative" ;;
	TestGPUNodePowerConservation) echo "total GPU power = active + idle" ;;
	*) echo "" ;;
	esac
}

# first prints the first capture group of an extended regex matched in the log.
first() {
	sed -nE "s/.*$1.*/\\1/p" "$log" | head -n 1
}

# watts formats a number as "12.3 W", or a dash when it is missing.
watts() {
	if [ -n "$1" ]; then
		printf '%.1f W' "$1"
	else
		printf '%s' '-'
	fi
}

# Test results, one "STATUS NAME SECONDS" line per top-level test.
results=$(sed -nE 's/^--- (PASS|FAIL|SKIP): (Test[A-Za-z0-9_]+) \(([0-9.]+)s\)$/\1 \2 \3/p' "$log")
total=0 passed=0 failed=0 skipped=0
if [ -n "$results" ]; then
	total=$(printf '%s\n' "$results" | wc -l | tr -d ' ')
	passed=$(printf '%s\n' "$results" | grep -c '^PASS ' || true)
	failed=$(printf '%s\n' "$results" | grep -c '^FAIL ' || true)
	skipped=$(printf '%s\n' "$results" | grep -c '^SKIP ' || true)
fi

# Skip reasons, one "NAME<TAB>reason" line per skipped test.
skip_reasons=$(awk '
	/^=== RUN   Test/ { test = $3 }
	/Skipping: / && test != "" && !(test in seen) {
		seen[test] = 1
		line = $0
		sub(/.*Skipping: /, "", line)
		print test "\t" line
	}' "$log")

# GPU details from the driver. Best effort: the summary matters most when
# something is broken, so a failing nvidia-smi must not stop it.
gpu_name="" driver="" memory="" cuda=""
if command -v nvidia-smi >/dev/null 2>&1; then
	gpu_info=$(nvidia-smi --query-gpu=name,driver_version,memory.total \
		--format=csv,noheader 2>/dev/null | head -n 1) || true
	IFS=',' read -r gpu_name driver memory <<<"$gpu_info" || true
	gpu_name=$(echo "$gpu_name" | xargs)
	driver=$(echo "$driver" | xargs)
	memory=$(echo "$memory" | xargs)
	cuda=$(nvidia-smi 2>/dev/null | sed -nE 's/.*CUDA Version: ([0-9.]+).*/\1/p' | head -n 1) || true
fi
gpu_name=${gpu_name:-unknown} driver=${driver:-unknown} memory=${memory:-unknown} cuda=${cuda:-unknown}

# Measurements logged by the tests.
idle_smi=$(first 'idle: nvidia-smi=([0-9.]+) W')
idle_total=$(first 'idle: nvidia-smi=[0-9.]+ W, node total=([0-9.]+) W')
idle_idle=$(first 'idle: nvidia-smi=.*, idle=([0-9.]+) W')
idle_active=$(first 'idle: nvidia-smi=.*, active=([0-9.]+) W')
load_container=$(first 'GPU load running: container=([0-9a-f]+) pid=[0-9]+')
load_pid=$(first 'GPU load running: container=[0-9a-f]+ pid=([0-9]+)')
load_smi=$(first 'under load: nvidia-smi=([0-9.]+) W')
load_total=$(first 'under load: .*node total=([0-9.]+) W')
load_idle=$(first 'under load: .*, idle=([0-9.]+) W')
load_active=$(first 'under load: .*, active=([0-9.]+) W')
load_running=$(first 'under load: .*running processes=([0-9.]+) W')
load_watts=$(first 'under load: .*load pid=([0-9.]+) W')

commit_ref="\`${sha:0:7}\`"
if [ -n "$sha" ]; then
	commit_ref="[\`${sha:0:7}\`](${server}/${repo}/commit/${sha})"
fi

# The test binary prints a standalone PASS or FAIL line when it finishes. A
# run that was cancelled or timed out has neither, and may have no exit code.
finished=false
if grep -qE '^(PASS|FAIL)$' "$log"; then
	finished=true
fi

# Heading and one-line verdict.
if [ "$total" -eq 0 ]; then
	echo "## ⚠️ GPU e2e did not report any tests"
	echo
	echo "The test binary exited with code \`${exit_code:-unknown}\` before any test finished. The logs below and in the artifact show why."
elif [ "$failed" -gt 0 ]; then
	echo "## ❌ GPU e2e failed on ${gpu_name}"
	echo
	echo "${failed} of ${total} GPU tests failed on \`${target}\` at ${commit_ref}. Details are below each failing test."
elif [ -n "$exit_code" ] && [ "$exit_code" != 0 ]; then
	echo "## ❌ GPU e2e failed on ${gpu_name}"
	echo
	echo "The test binary exited with code \`${exit_code}\` on \`${target}\` at ${commit_ref} without a failing test, for example after a timeout or a crash. The logs show why."
elif [ "$finished" != true ]; then
	echo "## ⚠️ GPU e2e did not finish on ${gpu_name}"
	echo
	echo "The run on \`${target}\` at ${commit_ref} stopped before the tests finished, for example because it was cancelled. Only the ${total} tests that finished are listed below."
else
	echo "## ✅ GPU e2e passed on ${gpu_name}"
	echo
	ran=$((total - skipped))
	echo "${ran} of ${total} GPU tests passed on \`${target}\` at ${commit_ref}, against the real GPU."
fi
echo

echo "| GPU | Driver | CUDA | Memory | Runner |"
echo "| --- | --- | --- | --- | --- |"
echo "| ${gpu_name} | \`${driver}\` | \`${cuda:-unknown}\` | ${memory} | \`${runner}\` |"
echo

if [ "$total" -gt 0 ]; then
	counts="${passed} passed"
	[ "$failed" -gt 0 ] && counts="${counts}, ${failed} failed"
	[ "$skipped" -gt 0 ] && counts="${counts}, ${skipped} skipped"
	echo "### Tests: ${counts}"
	echo
	echo "| | Test | What it checks | Time |"
	echo "| :---: | --- | --- | ---: |"
	# Failures first, then passes, then skips.
	for status in FAIL PASS SKIP; do
		printf '%s\n' "$results" | while read -r st name secs; do
			[ "$st" = "$status" ] || continue
			case "$st" in
			PASS) icon="✅" ;;
			FAIL) icon="❌" ;;
			*) icon="⏭️" ;;
			esac
			what=$(describe "$name")
			if [ "$st" = SKIP ]; then
				reason=$(printf '%s\n' "$skip_reasons" | awk -F '\t' -v t="$name" '$1 == t { print $2; exit }')
				[ -n "$reason" ] && what="${what:+${what}. }Skipped: ${reason}"
			fi
			printf "| %s | \`%s\` | %s | %.1f s |\n" "$icon" "$name" "$what" "$secs"
		done
	done
	echo
fi

# Why each failing test failed: the assertion and its message, without the
# Kepler start-up lines and stack paths around them.
if [ "$failed" -gt 0 ]; then
	printf '%s\n' "$results" | awk '$1 == "FAIL" { print $2 }' | while read -r name; do
		echo "<details open><summary><strong>Why <code>${name}</code> failed</strong></summary>"
		echo
		echo '```text'
		awk -v t="$name" '
			$0 == "=== RUN   " t { inside = 1; next }
			inside && /^--- (PASS|FAIL|SKIP): / { inside = 0 }
			inside && (/_test\.go:[0-9]+:/ || /^[[:space:]]+\t/) {
				line = $0
				sub(/^[[:space:]]+/, "", line)
				gsub(/\t+/, " ", line)
				if (line ~ /Starting Kepler:|Kepler started on port|^Error Trace:|^Test:|^\//) next
				print line
			}' "$log" | tail -n 30
		echo '```'
		echo
		echo "</details>"
		echo
	done
fi

if [ -n "$idle_smi$load_smi" ]; then
	echo "### Power"
	echo
	echo "| | nvidia-smi | Kepler total | Kepler idle | Kepler active | To processes | To the load |"
	echo "| --- | ---: | ---: | ---: | ---: | ---: | ---: |"
	if [ -n "$idle_smi" ]; then
		echo "| Idle | $(watts "$idle_smi") | $(watts "$idle_total") | $(watts "$idle_idle") | $(watts "$idle_active") | - | - |"
	fi
	if [ -n "$load_smi" ]; then
		echo "| Under load | $(watts "$load_smi") | $(watts "$load_total") | $(watts "$load_idle") | $(watts "$load_active") | $(watts "$load_running") | $(watts "$load_watts") |"
	fi
	echo
	echo "Kepler splits the GPU's power into idle, the lowest power it has seen with no process on the GPU, and active, the rest, which it gives to the processes using the GPU."
	if [ -n "$load_pid" ]; then
		echo "Under load, \`dcgmproftester\` kept the tensor cores busy as pid \`${load_pid}\` in container \`${load_container}\`."
	fi
	echo
fi

# Power over time, from the nvidia-smi samples. The x axis follows the
# timestamps in the samples, not their count: each sample also waits for
# nvidia-smi, so samples are not exactly one second apart.
if [ -n "$samples" ] && [ -s "$samples" ]; then
	chart=$(awk -F ',' '
		NF >= 2 && $1 + 0 == $1 && $2 + 0 == $2 {
			n++
			t[n] = $1
			w[n] = $2
			if ($2 > max) max = $2
		}
		END {
			if (n < 2) exit
			span = t[n] - t[1]
			if (span <= 0) exit
			# Average the samples in each time bucket; an empty bucket keeps
			# the value of the bucket before it.
			buckets = n < 90 ? n : 90
			width = span / buckets
			for (i = 1; i <= n; i++) {
				b = int((t[i] - t[1]) / width)
				if (b >= buckets) b = buckets - 1
				sum[b] += w[i]
				count[b]++
			}
			line = ""
			value = w[1]
			for (b = 0; b < buckets; b++) {
				if (count[b]) value = sum[b] / count[b]
				line = line (b ? ", " : "") sprintf("%.1f", value)
			}
			top = (int(max / 25) + 1) * 25
			if (top - max < 10) top += 25
			printf "x-axis \"seconds\" 0 --> %d\n", span + 0.5
			printf "y-axis \"watts\" 0 --> %d\n", top
			printf "line [%s]\n", line
		}' "$samples")
	if [ -n "$chart" ]; then
		echo "<details><summary><strong>GPU power over the whole run</strong> (nvidia-smi, every second)</summary>"
		echo
		echo '```mermaid'
		echo 'xychart-beta'
		while IFS= read -r chart_line; do
			echo "  ${chart_line}"
		done <<<"$chart"
		echo '```'
		echo
		echo "</details>"
		echo
	fi
fi

logged=$(sed -nE 's/^[[:space:]]+gpu_test\.go:[0-9]+: (GPU .*|idle: .*|under load: .*|Starting GPU load: .*)$/\1/p' "$log")
if [ -n "$logged" ]; then
	echo "<details><summary><strong>Measurements logged by the tests</strong></summary>"
	echo
	echo '```text'
	printf '%s\n' "$logged"
	echo '```'
	echo
	echo "</details>"
	echo
fi

if [ -n "$artifact" ] && [ -n "${GITHUB_RUN_ID:-}" ]; then
	echo "⬇️ The full test output, Kepler's logs and the power samples are in the [\`${artifact}\`](${server}/${repo}/actions/runs/${GITHUB_RUN_ID}#artifacts) artifact."
fi
