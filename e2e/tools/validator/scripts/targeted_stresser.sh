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

usage() {
    echo "Usage: $0 -g <general_mode> -r <cpu_range> -c <cpus> -d <mount_dir> -t <time_interval_log_name> -l <load_curve> -n <iterations>"
    echo "  -g <general_mode> If set, <cpu_range> and <cpus> are ignored."
    echo "  -r <cpu_range>   CPU range for stress-ng taskset (Default: '15')"
    echo "  -c <cpus>    Number of CPUs to use for stress-ng (Default: '1')"
    echo "  -d <mount_dir>   Directory to mount for logging (Default: '/tmp')"
    echo "  -t <time_interval_log_name> Filename for start and end time file log (Default: 'time_interval.log')"
    echo "  -l <load_curve>  Load curve as a comma-separated list (Default: '0:5,50:20,75:20,100:20,75:20,50:20')"
    echo "  -n <iterations> Number of times to iterate the Load curve (Default: '1')"
    exit 1
}

main() {

    set_general_mode=false
    DEFAULT_CPU_RANGE="15"
    DEFAULT_CPUS="1"
    DEFAULT_MOUNT_DIR="/tmp"
    DEFAULT_LOAD_CURVE_STR="0:5,50:20,75:20,100:20,75:20,50:20"
    DEFAULT_TIME_INTERVAL_LOG_NAME="time_interval.log"
    DEFAULT_ITERATIONS="1"

    # Parse command-line options
    while getopts "g:r:c:d:t:l:n:" opt; do
        case "$opt" in
            g) set_general_mode=true ;;
            r) cpu_range="$OPTARG" ;;
            c) cpus="$OPTARG" ;;
            d) mount_dir="$OPTARG" ;;
            t) time_interval_log_name="$OPTARG" ;;
            l) load_curve_str="$OPTARG" ;;
            n) iterations="$OPTARG" ;;
            *) usage ;;
        esac
    done

    cpu_range="${cpu_range:-$DEFAULT_CPU_RANGE}"
    cpus="${cpus:-$DEFAULT_CPUS}"
    mount_dir="${mount_dir:-$DEFAULT_MOUNT_DIR}"
    time_interval_log_name="${time_interval_log_name:-$DEFAULT_TIME_INTERVAL_LOG_NAME}"
    load_curve_str="${load_curve_str:-$DEFAULT_LOAD_CURVE_STR}"
    iterations="${iterations:-$DEFAULT_ITERATIONS}"

    IFS=',' read -r -a load_curve <<< "$load_curve_str"

    TIME_INTERVAL_LOG="${mount_dir}/${time_interval_log_name}"

    > "$TIME_INTERVAL_LOG"

    start_time=$(date +%s)
    echo "Stress Start Time: $start_time" >> "$TIME_INTERVAL_LOG"

    for i in $(seq 1 "$iterations"); do
        echo "Running $i/$iterations"
        for x in "${load_curve[@]}"; do
            local load="${x%%:*}"
            local time="${x##*:}s"
            if $set_general_mode; then
                run stress-ng --cpu "$cpus" --cpu-method ackermann --cpu-load "$load" --timeout "$time"
            else
                run taskset -c "$cpu_range" stress-ng --cpu "$cpus" --cpu-method ackermann --cpu-load "$load" --timeout "$time"
            fi
        done
    done 

    end_time=$(date +%s)
    echo "Stress End Time: $end_time" >> "$TIME_INTERVAL_LOG"
}

main "$@"