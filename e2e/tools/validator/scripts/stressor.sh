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

# stepwise load curve: each step is 20 seconds
declare -a load_curve_stepwise=(
	0:20
	20:20
	40:20
	60:20
	80:20
	100:20
	80:20
	60:20
	40:20
	20:20
	0:20
)

# default load curve: varying durations
declare -a load_curve_default=(
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

declare -a load_curve_regression=(
	0:15
	10:30
	25:30
	50:30
	75:30
	50:30
	25:30
	10:30
	0:15

)

main() {
	local total_time=0
	local repeats=5
	local curve_type="default"
	local cooldown_time=5
	local warmup_time=5

	while getopts "t:r:c:d:w:" opt; do
		case $opt in
		t) total_time=$OPTARG ;;
		c) curve_type=$OPTARG ;;
		r) repeats=$OPTARG ;;
		w) warmup_time=$OPTARG ;;
		d) cooldown_time=$OPTARG ;;
		*)
			echo "Usage: $0 [-t total_time_in_seconds] [-w warmup_time_in_seconds] [-c cooldown_time_in_seconds] [-r repeats] [-c curve_type]"
			exit 1
			;;
		esac
	done

	# Select load curve based on curve_type
	local -a load_curve
	case $curve_type in
	"default") load_curve=("${load_curve_default[@]}") ;;
	"stepwise") load_curve=("${load_curve_stepwise[@]}") ;;
	"regression") load_curve=("${load_curve_regression[@]}") ;;
	*)
		echo "Invalid curve type. Use 'default' or 'stepwise or regression'" >&2
		exit 1
		;;
	esac

	local cpus
	cpus=$(nproc)

	# calculate the total duration of one cycle of the load curve
	local total_cycle_time=0
	for x in "${load_curve[@]}"; do
		local time="${x##*:}"
		total_cycle_time=$((total_cycle_time + time))
	done

	# calculate the repeats if total_time is provided
	if [ "$total_time" -gt 0 ]; then
		repeats=$((total_time / total_cycle_time))
	fi

	echo "Total time: $total_time seconds, Repeats: $repeats, Curve type: $curve_type"

	# sleep so that first run and the second run look the same
	echo "Warmup .."
	run stress-ng --cpu "$cpus" --cpu-method ackermann --cpu-load 0 --timeout "$warmup_time"

	for i in $(seq 1 "$repeats"); do
		echo "Running: $i/$repeats"
		for x in "${load_curve[@]}"; do
			local load="${x%%:*}"
			local time="${x##*:}s"
			run stress-ng --cpu "$cpus" --cpu-method ackermann --cpu-load "$load" --timeout "$time"
		done
		# sleep so that the next run looks the same
		echo "Cooldown .."
		run stress-ng --cpu "$cpus" --cpu-method ackermann --cpu-load 0 --timeout "$cooldown_time"
	done
}

main "$@"
