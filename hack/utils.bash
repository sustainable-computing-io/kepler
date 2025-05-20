is_fn() {
	[[ $(type -t "$1") == "function" ]]
	return $?
}

header() {
	local title=" 🔆🔆🔆  $*  🔆🔆🔆 "

	local len=40
	if [[ ${#title} -gt $len ]]; then
		len=${#title}
	fi

	echo -e "\n\n  \033[1m${title}\033[0m"
	echo -n "━━━━━"
	printf '━%.0s' $(seq "$len")
	echo "━━━━━━━"

}

info() {
	echo -e " 🔔 $*" >&2
}

err() {
	echo -e " 😱 $*" >&2
}

warn() {
	echo -e "   $*" >&2
}

ok() {
	echo -e "   ✅ $*" >&2
}

skip() {
	echo -e " 🙈 SKIP: $*" >&2
}

fail() {
	echo -e " ❌ FAIL: $*" >&2
}

info_run() {
	echo -e "      $*\n" >&2
}

run() {
	echo -e " ❯ $*\n" >&2
	"$@"
}

die() {
	echo -e "\n ✋ $* "
	echo -e "──────────────────── ⛔️⛔️⛔️ ────────────────────────\n"
	exit 1
}

line() {
	local len="$1"
	local style="${2:-thin}"
	shift

	local ch='─'
	[[ "$style" == 'heavy' ]] && ch="━"

	printf "$ch%.0s" $(seq "$len") >&2
	echo
}
# condition_check <exp> <cond>
# checks if the condition result matches the expected result
condition_check() {
	local exp="$1"
	local cond="$2"
	shift 2
	[[ -n $exp ]] && [[ $("$cond" "$@") == "$exp" ]] && {
		return 0
	}
	return 1
}

# wait_until <max_tries> <delay> <msg> <condition>
# waits for condition to be true for a max of <max_tries> x <delay> seconds
wait_until() {
	local max_tries="$1"
	local delay="$2"
	local msg="$3"
	local condition="$4"
	shift 4

	info "Waiting [$max_tries x ${delay}s] for $msg"
	local tries=0
	local -i ret=1
	echo " ❯ $condition $*" 2>&1
	while [[ $tries -lt $max_tries ]]; do

		$condition "$@" && {
			ret=0
			break
		}

		tries=$((tries + 1))
		echo "   ... [$tries / $max_tries] waiting ($delay secs) - $msg" >&2
		sleep "$delay"
	done

	return $ret
}
