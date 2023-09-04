#!/usr/bin/env bash
#
# This file is part of the Kepler project
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at

#     http://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Copyright 2022 The Kepler Contributors
#
err() {
    echo -e "$(date -u +%H:%M:%S) 😱 ERROR: $*\n" >&2
}

info() {
    echo -e "$(date -u +%H:%M:%S) 🔔 INFO : $*\n" >&2
}

header() {
    local title="🔆🔆🔆  $*  🔆🔆🔆 "

    local len=40
    if [[ ${#title} -gt $len ]]; then
        len=${#title}
    fi

    echo -e "\n\n  \033[1m${title}\033[0m"
    echo -n "━━━━━"
    printf '━%.0s' $(seq "$len")
    echo "━━━━━━━"

}

die() {
    echo -e "$(date -u +%H:%M:%S) 💀 FATAL: $*\n" >&2
    exit 1
}

run() {
    echo -e " ❯ $*\n"
    "$@"
    ret=$?
    echo -e "        ────────────────────────────────────────────\n"
    return $ret
}

ok() {
    echo -e "    ✅ $*\n" >&2
}

fail() {
    echo -e "    ❌ $*\n" >&2
}

# returns 0 if arg is set to True or true or TRUE else false
# usage: is_set $x && echo "$x is set"
is_set() {
    [[ "$1" =~ true|TRUE|True ]]
}
