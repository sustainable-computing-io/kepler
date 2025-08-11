#!/usr/bin/env bash

set -eu -o pipefail
trap cleanup INT EXIT

# config
declare DURATION=${DURATION:-30}
declare KEPLER_PORT=${KEPLER_PORT:-28282}

# constants
PROJECT_ROOT="$(git rev-parse --show-toplevel)"
declare -r PROJECT_ROOT
declare -r TMP_DIR="$PROJECT_ROOT/tmp"
declare -r CPU_PROFILE_DIR="$TMP_DIR/cpu-profile"
declare -r MEM_PROFILE_DIR="$TMP_DIR/mem-profile"
declare -r KEPLER_BIN_DIR="$PROJECT_ROOT/bin"
declare -r CPU_HTTP_PORT=29000
declare -r MEM_HTTP_PORT=29001

source "$PROJECT_ROOT/hack/utils.bash"

cleanup() {
	info "Cleaning up ..."
	# Terminate all background jobs (e.g. pprof servers)
	{ jobs -p | xargs -I {} -- pkill -TERM -P {}; } || true
	wait
	sleep 1

	return 0
}

capture_cpu_profile() {
	run go tool pprof -proto -seconds "$DURATION" \
		-output "$CPU_PROFILE_DIR"/profile.pb.gz "$KEPLER_BIN_DIR/kepler" \
		"http://localhost:$KEPLER_PORT/debug/pprof/profile" || return 1

	# Start pprof web server in background
	run go tool pprof --http "localhost:$CPU_HTTP_PORT" --no_browser \
		"$KEPLER_BIN_DIR/kepler" "$CPU_PROFILE_DIR/profile.pb.gz" </dev/null &
	sleep 1
	# Fetch visualizations
	for sample in {cpu,samples}; do
		curl --fail "http://localhost:$CPU_HTTP_PORT/ui/?si=$sample" -o "$CPU_PROFILE_DIR/graph-$sample.html" || return 1
		curl --fail "http://localhost:$CPU_HTTP_PORT/ui/flamegraph?si=$sample" -o "$CPU_PROFILE_DIR/flamegraph-$sample.html" || return 1
		for page in top peek source disasm; do
			curl --fail "http://localhost:$CPU_HTTP_PORT/ui/$page?si=$sample" -o "$CPU_PROFILE_DIR/$page-$sample.html" || return 1
		done
	done

	return 0
}

capture_mem_profile() {
	run go tool pprof -proto -seconds "$DURATION" \
		-output "$MEM_PROFILE_DIR"/profile.pb.gz "$KEPLER_BIN_DIR/kepler" \
		"http://localhost:$KEPLER_PORT/debug/pprof/heap" || return 1

	# Start pprof web server in background
	run go tool pprof --http "localhost:$MEM_HTTP_PORT" --no_browser \
		"$KEPLER_BIN_DIR/kepler" "$MEM_PROFILE_DIR/profile.pb.gz" </dev/null &
	sleep 1
	# Fetch visualizations
	for sample in {alloc,inuse}_{objects,space}; do
		curl --fail "http://localhost:$MEM_HTTP_PORT/ui/?si=$sample" -o "$MEM_PROFILE_DIR/graph-$sample.html" || return 1
		curl --fail "http://localhost:$MEM_HTTP_PORT/ui/flamegraph?si=$sample" -o "$MEM_PROFILE_DIR/flamegraph-$sample.html" || return 1
		for page in top peek source disasm; do
			curl --fail "http://localhost:$MEM_HTTP_PORT/ui/$page?si=$sample" -o "$MEM_PROFILE_DIR/$page-$sample.html" || return 1
		done
	done

	return 0
}

main() {
	cd "$PROJECT_ROOT"
	mkdir -p "${CPU_PROFILE_DIR}"
	mkdir -p "${MEM_PROFILE_DIR}"

	header "Running CPU Profiling"
	capture_cpu_profile || {
		fail "CPU Profiling failed"
		return 1
	}
	header "Running Memory Profiling"
	capture_mem_profile || {
		fail "Memory Profiling failed"
		return 1
	}
}

main "$@"
