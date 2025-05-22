#!/usr/bin/env bash

set -eu -o pipefail

grafana() {
	local endpoint="$1"
	shift

	http --auth admin:admin ":23000/api/$endpoint" "$@"

}

search() {
	grafana search "$@"
}

dashboards() {
	search query==% | jq -r '.[] | "\(.uid): \(.title)"'
}

download() {
	local uid="$1"
	grafana dashboards/uid/"$uid" | jq '.dashboard' | tee "$uid.json"
}

main() {
	local dashboard=""
	dashboard=$(dashboards | fzf) || true

	if [ -z "$dashboard" ]; then
		echo "ℹ️ No dashboard selected"
		return 0
	fi

	local uid
	uid=$(echo "$dashboard" | cut -d: -f1)
	download "$uid"
}

main "$@"
