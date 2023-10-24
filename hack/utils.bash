#
# Copyright 2023.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

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
