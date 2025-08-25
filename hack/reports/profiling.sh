#!/usr/bin/env bash

set -eu -o pipefail

PROJECT_ROOT="$(git rev-parse --show-toplevel)"

# config
declare DURATION=${DURATION:-30}
declare KEPLER_DEV_PORT=${KEPLER_DEV_PORT:-28283}
declare KEPLER_LATEST_PORT=${KEPLER_LATEST_PORT:-28284}
declare SHOW_HELP=false

# constants
declare -r PROJECT_ROOT
declare -r TMP_DIR="$PROJECT_ROOT/tmp"
declare -r CPU_PROFILE_DIR="$TMP_DIR/cpu-profile"
declare -r MEM_PROFILE_DIR="$TMP_DIR/mem-profile"
declare -r COMPARE_DIR="$TMP_DIR/compare"

source "$PROJECT_ROOT/hack/utils.bash"

validate() {
	header "Validating"
	command -v pprof >/dev/null 2>&1 || {
		fail "No pprof command found in PATH"
		info "Please install pprof"
		cat <<-EOF
			go install github.com/google/pprof@latest
		EOF
		# NOTE: do not proceed without pprof installed
		return 1
	}
}

parse_args() {
	### while there are args parse them
	while [[ -n "${1+xxx}" ]]; do
		case $1 in
		--help | -h)
			shift
			SHOW_HELP=true
			return 0
			;;
		--dev-port)
			shift
			KEPLER_DEV_PORT="$1"
			shift
			;;
		--latest-port)
			shift
			KEPLER_LATEST_PORT="$1"
			shift
			;;
		--duration)
			shift
			DURATION="$1"
			shift
			;;
		*) return 1 ;; # show usage on everything else
		esac
	done
	return 0
}

print_usage() {
	local scr
	scr="$(basename "$0")"

	read -r -d '' help <<-EOF_HELP || true
		  🔆 Usage:
			  $scr <command> [OPTIONS]
			  $scr  -h | --help

		    📋 Commands:
		      capture        Captures CPU and Memory profile from the Kepler dev and latest compose services
		      compare        Compares two profiles and prints out the top cumulative samples
		      output         Generates formatted profiling report output for GitHub workflow

		    💡 Examples:
		      →  $scr capture --duration 30 --dev-port 28283 --latest-port 28284

		      →  $scr compare

		      →  $scr output

			⚙️ Options:
		      -h | --help             Show this help message
		      --duration              Duration of the profiling session in seconds (default: 30)
		      --dev-port              Port of the kepler dev compose service  (default: 28283)
		      --latest-port           Port of the kepler latest compose service (default: 28284)
	EOF_HELP

	echo -e "$help"
	return 0
}

profile_capture() {
	header "Running CPU Profiling"
	mkdir -p "$CPU_PROFILE_DIR"

	local http_dev_status=0
	local http_latest_status=0

	http_dev_status=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:$KEPLER_DEV_PORT/debug/pprof/")
	http_latest_status=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:$KEPLER_LATEST_PORT/debug/pprof/")

	[[ $http_dev_status -ne 200 || $http_latest_status -ne 200 ]] && {
		fail "Unable to connect to kepler-dev or kepler-latest compose service"
		info "Make sure that kepler-dev and kepler-latest compose services are running"
		return 1
	}

	run pprof -proto -seconds "$DURATION" \
		-output "$CPU_PROFILE_DIR"/profile-dev.pb.gz \
		"http://localhost:$KEPLER_DEV_PORT/debug/pprof/profile" &

	run pprof -proto -seconds "$DURATION" \
		-output "$CPU_PROFILE_DIR"/profile-latest.pb.gz \
		"http://localhost:$KEPLER_LATEST_PORT/debug/pprof/profile" &

	# NOTE: Simulate Prometheus scraping by querying metrics endpoints
	# to capture realistic operational behavior during profiling
	(
		sleep 5
		for i in {1..5}; do
			info "Querying metric endpoints [$i/5]"
			curl -s "http://localhost:$KEPLER_LATEST_PORT/metrics" >/dev/null &
			curl -s "http://localhost:$KEPLER_DEV_PORT/metrics" >/dev/null &
			wait
			sleep 5
			curl -s "http://localhost:$KEPLER_DEV_PORT/metrics" >/dev/null &
			curl -s "http://localhost:$KEPLER_LATEST_PORT/metrics" >/dev/null &
			wait
			sleep 5
		done
	) &
	wait

	header "Running Memory Profiling"

	mkdir -p "$MEM_PROFILE_DIR"

	run pprof -proto -seconds "$DURATION" \
		-output "$MEM_PROFILE_DIR"/profile-dev.pb.gz \
		"http://localhost:$KEPLER_DEV_PORT/debug/pprof/heap" &

	run pprof -proto -seconds "$DURATION" \
		-output "$MEM_PROFILE_DIR"/profile-latest.pb.gz \
		"http://localhost:$KEPLER_LATEST_PORT/debug/pprof/heap" &

	# NOTE: Simulate Prometheus scraping by querying metrics endpoints to
	# capture realistic operational behavior during profiling
	(
		sleep 5
		for i in {1..5}; do
			info "Querying metric endpoints [$i/5]"
			curl -s "http://localhost:$KEPLER_LATEST_PORT/metrics" >/dev/null &
			curl -s "http://localhost:$KEPLER_DEV_PORT/metrics" >/dev/null &
			wait
			sleep 5
			curl -s "http://localhost:$KEPLER_DEV_PORT/metrics" >/dev/null &
			curl -s "http://localhost:$KEPLER_LATEST_PORT/metrics" >/dev/null &
			wait
			sleep 5
		done
	) &
	wait

	return 0
}

profile_compare() {
	header "Comparing CPU Profiles"

	mkdir -p "$COMPARE_DIR"
	info "Fetching top cumulative samples"
	run pprof -top -cum -show "github.com/sustainable-computing-io" -base \
		"$CPU_PROFILE_DIR"/profile-latest.pb.gz "$CPU_PROFILE_DIR"/profile-dev.pb.gz |
		tee "$COMPARE_DIR/top-cpu.txt"

	header "Comparing Memory Profiles"

	info "Fetching top cumulative samples for inuse memory"
	run pprof -top -cum -show "github.com/sustainable-computing-io" -base \
		"$MEM_PROFILE_DIR"/profile-latest.pb.gz "$MEM_PROFILE_DIR"/profile-dev.pb.gz |
		tee "$COMPARE_DIR/top-mem-inuse.txt"

	info "Fetching top cumulative samples for alloc memory"
	run pprof -top -cum -alloc_space -show "github.com/sustainable-computing-io" -base \
		"$MEM_PROFILE_DIR"/profile-latest.pb.gz "$MEM_PROFILE_DIR"/profile-dev.pb.gz |
		tee "$COMPARE_DIR/top-mem-alloc.txt"

	return 0
}

profile_output() {
	# NOTE: Generate the formatted message in Markdown format
	echo "📊 Profiling reports are ready to be viewed"
	echo ""
	echo "⚠️ ***Variability in pprof CPU and Memory profiles***"
	echo "***When comparing pprof profiles of Kepler versions, expect variability in CPU and memory. Focus only on significant, consistent differences.***"
	echo ""

	# CPU Comparison section
	echo "<details>"
	echo "<summary>💻 CPU Comparison with base Kepler</summary>"
	echo ""
	echo "\`\`\`"
	if [[ -f "$COMPARE_DIR/top-cpu.txt" ]]; then
		cat "$COMPARE_DIR/top-cpu.txt"
	else
		echo "❌ CPU comparison data not available"
	fi
	echo "\`\`\`"
	echo "</details>"
	echo ""

	# Memory Comparison (Inuse) section
	echo "<details>"
	echo "<summary>💾 Memory Comparison with base Kepler (Inuse)</summary>"
	echo ""
	echo "\`\`\`"
	if [[ -f "$COMPARE_DIR/top-mem-inuse.txt" ]]; then
		cat "$COMPARE_DIR/top-mem-inuse.txt"
	else
		echo "❌ Memory comparison data not available"
	fi
	echo "\`\`\`"
	echo "</details>"
	echo ""

	# Memory Comparison (Alloc) section
	echo "<details>"
	echo "<summary>💾 Memory Comparison with base Kepler (Alloc)</summary>"
	echo ""
	echo "\`\`\`"
	if [[ -f "$COMPARE_DIR/top-mem-alloc.txt" ]]; then
		cat "$COMPARE_DIR/top-mem-alloc.txt"
	else
		echo "❌ Memory comparison data not available"
	fi
	echo "\`\`\`"
	echo "</details>"
	echo ""

	return 0
}

main() {
	local fn=${1:-''}
	shift

	parse_args "$@" || die "failed to parse args"

	$SHOW_HELP && {
		print_usage
		exit 0
	}

	cd "$PROJECT_ROOT"

	local cmd_fn="profile_$fn"
	if ! is_fn "$cmd_fn"; then
		fail "unknown command: $fn"
		print_usage
		return 1
	fi

	validate || return 1

	$cmd_fn "$@" || return 1

	return 0
}

main "$@"
