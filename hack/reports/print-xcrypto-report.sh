#!/usr/bin/env bash
set -eu -o pipefail

trap cleanup EXIT INT TERM

declare BASE_REF=""
declare HEAD_SHA=""
declare REPORT_PATH="/tmp/check-xcrypto-report.txt"
declare SHOW_HELP=false
declare tmp_base_ref=""
declare tmp_head_sha=""

log_and_save() {
	echo "$1" | tee -a "$REPORT_PATH"
}

print_help() {
	cat <<-EOF
		        ⚙️ Options:
				-h|--help       show this help
		        --base-ref      git reference to base of PR
		        --head-sha      git commit SHA of latest commit in PR
		        --report-path   filepath to store the report (ex. /tmp/check-xcrypto-report.txt)
	EOF
}

generate_dependency_report() {
	git archive "$BASE_REF" | tar -x -C "$tmp_base_ref"
	git archive "$HEAD_SHA" | tar -x -C "$tmp_head_sha"

	echo "$tmp_base_ref"
	(
		cd "$tmp_base_ref"
		go mod tidy
		go mod graph >/tmp/deps-base.txt
	)
	(
		cd "$tmp_head_sha"
		go mod tidy
		go mod graph >/tmp/deps-pr.txt
		grep -R "golang.org/x/crypto" . --exclude-dir={vendor,testdata} --include=*.go >/tmp/deps-direct.txt || touch /tmp/deps-direct.txt
	)
	log_and_save "=== Dependency (x/crypto) Report ==="
	log_and_save ""

	# Check dependencies prior to PR
	log_and_save "ℹ️ Current Kepler dependencies on golang.org/x/crypto:"
	if grep -q " golang.org/x/crypto@" /tmp/deps-base.txt; then
		log_and_save "⚠️ Kepler already depends on golang.org/x/crypto through the following:"
		existing_deps=$(grep " golang.org/x/crypto@" /tmp/deps-base.txt | sed 's/^/ 🔹 /')
		log_and_save " 🔹 ${existing_deps//$'\n'/$'\n' 🔹 }"
	else
		log_and_save "✅ No existing dependency on golang.org/x/crypto found in base branch"
	fi

	log_and_save ""

	# Check for new dependencies introduced by PR
	log_and_save "ℹ️ Changes introduced by this PR:"
	if ! grep -q " golang.org/x/crypto@" /tmp/deps-pr.txt; then
		log_and_save "✅ PR does not introduce any new dependencies on golang.org/x/crypto"
	else
		new_deps=$(comm -13 <(sort /tmp/deps-base.txt) <(sort /tmp/deps-pr.txt) | grep " golang.org/x/crypto@" || true)
		if [ -z "$new_deps" ]; then
			log_and_save "❗PR doesn't add new x/crypto dependencies (note it is possible this PR depends on existing x/crypto dependencies)"
		else
			log_and_save "⚠️ PR introduces new dependencies on golang.org/x/crypto:"
			pr_deps=" 🔹 ${new_deps//$'\n'/$'\n' 🔹 }"
			log_and_save "$pr_deps"
			log_and_save ""
		fi
	fi

	log_and_save ""

	# Check for direct imports of x/crypto
	log_and_save "ℹ️ Locate any direct dependencies on golang.org/x/crypto:"
	occurrences=$(</tmp/deps-direct.txt)
	if [ -z "$occurrences" ]; then
		log_and_save "✅ No direct imports of golang.org/x/crypto found"
	else
		log_and_save "⚠️ Discovered direct imports of golang.org/x/crypto:"
		direct_deps=" 🔹 ${occurrences//$'\n'/$'\n' 🔹 }"
		log_and_save "$direct_deps"
	fi

	log_and_save ""
	log_and_save "=== End of Dependency Report ==="

}

parse_args() {
	while [[ -n ${1+xxx} ]]; do
		case "$1" in
		--base-ref)
			BASE_REF="$2"
			shift 2
			;;
		--head-sha)
			HEAD_SHA="$2"
			shift 2
			;;
		--report-path)
			REPORT_PATH="$2"
			shift 2
			;;
		-h | --help)
			SHOW_HELP=true
			return 0
			;;
		*)
			echo "Unknown option: $1"
			print_help
			return 1
			;;
		esac
	done

	return 0
}

cleanup() {
	rm -rf "$tmp_base_ref" "$tmp_head_sha"
}

main() {
	if ! parse_args "$@"; then
		echo "parse args failed"
		exit 1
	fi

	$SHOW_HELP && {
		print_help
		exit 0
	}

	if [[ -z $BASE_REF || -z $HEAD_SHA ]]; then
		echo "Error: --base-ref and --head-sha are required fields"
		print_help
		exit 1
	fi

	tmp_base_ref=$(mktemp -d)
	tmp_head_sha=$(mktemp -d)

	generate_dependency_report
}

main "$@"
