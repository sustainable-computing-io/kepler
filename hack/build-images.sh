#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

script_dir=$(dirname "$(readlink -f "$0")")

registry="quay.io/sustainable_computing_io"

supported_arches=("linux/amd64" "linux/s390x" "linux/arm64")
supported_attacher=("bcc", "libbpf")

function install_packages() {
	sudo apt install -y qemu-user-static binfmt-support build-essential
}

function setup_docker_experimental() {
	mkdir -p $HOME/.docker
    echo '{
      "experimental": "enabled"
    }' | tee $HOME/.docker/config.json
}

function create_builx_builder () {
    BUILDX_VERSION=$1 ## v0.8.2
    ARCH=$2 ### amd64
    BUILDX_BIN=buildx-${BUILDX_VERSION}.linux-${ARCH}
    wget https://github.com/docker/buildx/releases/download/v0.8.2/${BUILDX_BIN}
	chmod +x ${BUILDX_BIN}
    mkdir -p ~/.docker/cli-plugins
	mv ${BUILDX_BIN} ~/.docker/cli-plugins/docker-buildx

	docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
	
    docker buildx create --name multi-arch --platform "linux/amd64,linux/s390x,linux/arm64" --use
    docker buildx ls
    docker buildx inspect --bootstrap
}

function setup_env_for_arch() {
	case "$1" in
		"linux/amd64")
			kernel_arch="amd64"
			;;
		"linux/s390x")
			kernel_arch="s390x"
			;;
		"linux/arm64")
			kernel_arch="arm64"
			;;
		(*) echo "$1 is not supported" && exit 1
	esac
}

function help() {
	 echo "Usage: $0 setup"
	 echo "       $0 base"
	 echo "       $0 kepler"
	 echo "       $0 help"
}

function setup_docker_buildx() {
	install_packages
	setup_docker_experimental
	create_builx_builder "v0.8.2" "amd64"
}

function build_image_base() {
	image="kepler_base"
	pushd "${script_dir}/.."

	tag=$(date +%Y%m%d%H%M%s)

	for attacher in ${supported_attacher[@]}; do 
		for arch in ${supported_arches[@]}; do
			if [ -e  ./build/Dockerfile.${attacher}.base.${kernel_arch} ];
			then
				setup_env_for_arch "${arch}"

				echo "Building ${image} image for ${arch}"
				docker buildx build \
					-f ./build/Dockerfile.${attacher}.base.${kernel_arch} \
					-t "${registry}/${image}:${kernel_arch}-${attacher}-${tag}" \
					--platform="${arch}" \
					--load \
					.
				docker tag "${registry}/${image}:${kernel_arch}-${attacher}-${tag}" "${registry}/${image}:latest-${kernel_arch}-${attacher}"
				docker push "${registry}/${image}:${kernel_arch}-${attacher}-${tag}"
				docker push "${registry}/${image}:latest-${kernel_arch}-${attacher}"
			fi
		done
	done

	popd

	create_image_manifest_base
}

function create_image_manifest_base () {
	docker manifest create \
		${registry}/${image}:latest \
		${registry}/${image}:latest-amd64-bcc \
		${registry}/${image}:latest-s390x-bcc \
		${registry}/${image}:latest-arm64-bcc \
		${registry}/${image}:latest-amd64-libbpf

	docker manifest push ${registry}/${image}:latest
}

function build_image_kepler() {
	echo "TBD..."
}


if [ $# -eq 0 ];
then
	help 1>&2;
	exit 1
fi


case "$1" in
	"setup")
		echo "setup_docker_buildx..."
		setup_docker_buildx
		;;
	"base")
		echo "build image kepler_base..."
		build_image_base
		;;
	"kepler")
		echo "build image kepler..."
		build_image_kepler
		;;
	"help")
		help; 
		exit 0 ;;
	(*) 
		help 1>&2;
		exit 1 ;;
esac

