#!/usr/bin/env bash

set -eu -o pipefail

trap exit_all INT
exit_all() {
	pkill -P $$
}

run() {
	echo "❯ $*"
	"$@"
	echo "      ‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾"
}

main() {

	local cpus
	cpus=$(nproc)

	# load and time
	local -a load_curve=(
		0:5
		10:20
		25:20
		50:20
		75:20
		100:30
		75:20
		50:20
		25:20
		10:20
		0:5
	)

	# sleep 5  so that first run and the second run look the same
	echo "Warmup .."
	run stress-ng --cpu "$cpus" --cpu-method ackermann --cpu-load 0 --timeout 5

	for i in $(seq 1 5); do
		echo "Running: $i/5"
		for x in "${load_curve[@]}"; do
			local load="${x%%:*}"
			local time="${x##*:}s"
			run stress-ng --cpu "$cpus" --cpu-method ackermann --cpu-load "$load" --timeout "$time"
		done
	done
}

main "$@"
