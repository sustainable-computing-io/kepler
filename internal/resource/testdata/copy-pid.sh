#!/usr/bin/env bash

main() {
	local pid=$1
	shift

	local dest_dir="./procfs/$pid"
	local src_dir="/proc/$pid"
	sudo -v

	mkdir -p "$dest_dir"
	files=(cgroup cmdline comm environ stat)

	for f in "${files[@]}"; do
		# shellcheck disable=SC2024
		# disabled to allow creation of files owned by current user
		sudo cat "$src_dir/$f" >"$dest_dir/$f"
	done

	# create a fake exe symlink
	exe=$(cat "$src_dir/comm")
	touch "./fake-root/usr/bin/$exe"
	chmod +x "./fake-root/usr/bin/$exe"

	ln -s "../../fake-root/usr/bin/$exe" "$dest_dir/exe"

	echo "Copied pid $pid to $dest_dir"
	tree "$dest_dir"

}

main "$@"
