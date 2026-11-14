#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2025 The Kepler Authors
# SPDX-License-Identifier: Apache-2.0
#
# Samples GPU power and Kepler's GPU metrics in the background while the GPU
# e2e tests run, so a failure can be read without another GPU run.
#
# usage: sample.sh start <dir>
#        sample.sh stop <dir>
#
# start writes, until stop:
#   <dir>/gpu-power.csv           one "seconds,watts" line per nvidia-smi sample
#   <dir>/kepler-gpu-metrics.txt  Kepler's GPU series every 3 seconds
#
# Optional environment:
#   KEPLER_METRICS_URL  Kepler's metrics endpoint (default: the port set in
#                       test/testdata/e2e-gpu-config.yaml)
set -euo pipefail

usage="usage: sample.sh start|stop <dir>"
action=${1:?$usage}
dir=${2:?$usage}
metrics_url=${KEPLER_METRICS_URL:-http://localhost:28282/metrics}

case "$action" in
start)
	start=$(date +%s)
	(
		while true; do
			# nvidia-smi can hang on a broken driver. A sample that times out is
			# left out rather than written as 0 W.
			if draw=$(timeout 5 nvidia-smi --query-gpu=power.draw --format=csv,noheader,nounits); then
				watts=$(awk '{ s += $1 } END { printf "%.2f", s }' <<<"$draw")
				echo "$(($(date +%s) - start)),${watts}"
			fi
			sleep 1
		done
	) >>"${dir}/gpu-power.csv" 2>/dev/null </dev/null &
	echo $! >"${dir}/gpu-power.pid"

	# Each test starts its own Kepler, so the series restart with every test.
	(
		while true; do
			if metrics=$(curl -sf --max-time 2 "$metrics_url" | grep -E '^kepler_(node|process|container)_gpu_'); then
				echo "# $(date -u +%Y-%m-%dT%H:%M:%SZ), $(($(date +%s) - start)) s"
				printf '%s\n' "$metrics"
			fi
			sleep 3
		done
	) >>"${dir}/kepler-gpu-metrics.txt" 2>/dev/null </dev/null &
	echo $! >"${dir}/kepler-gpu-metrics.pid"
	;;
stop)
	for pid in "${dir}/gpu-power.pid" "${dir}/kepler-gpu-metrics.pid"; do
		if [ -f "$pid" ]; then
			kill "$(cat "$pid")" 2>/dev/null || true
			rm -f "$pid"
		fi
	done
	;;
*)
	echo "$usage" >&2
	exit 2
	;;
esac
