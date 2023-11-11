#!/bin/bash

# Set image registry as a variable
IMAGE_REGISTRY=${IMAGE_REGISTRY:-quay.io/sustainable_computing_io}

# Detect whether podman or docker exists
if command -v podman &> /dev/null
then
    DOCKER_CMD="podman"
else
    DOCKER_CMD="docker"
fi

# set ARCH variable for base image build, if it is x86_64, then use amd64
ARCH=$(uname -m)
if [ "$ARCH" == "x86_64" ]
then
    ARCH="amd64"
fi

# Set image tag prefixes
BASE_TAG_PREFIX="ubi-9-"
BUILDER_TAG_PREFIX="ubi-9-"
KEPLER_TAG_PREFIX="ubi-9-"

# Set image tag suffixes
BCC_TAG_SUFFIX="bcc-0.26"
LIBBPF_TAG_SUFFIX="libbpf-1.2.0"

# Set image tags
KEPLER_BCC_TAG="latest"
KEPLER_LIBBPF_TAG="latest-libbpf"

# Parse command line options
BUILD_BASE=false
BUILD_BUILDER=false
BUILD_KEPLER=false
PUSH_ONLY=false

function show_usage() {
    echo "Usage: build-images.sh [OPTIONS]"
    echo "Builds Kepler images."
    echo ""
    echo "Options:"
    echo "  -b, --base      Build base image"
    echo "  -r, --builder   Build builder image"
    echo "  -k, --kepler    Build Kepler image"
    echo "  -p, --push      Push images only"
    echo "  -h, --help      Show this help message"
    exit 0
}

# Set Dockerfile variables
BCC_BASE_DOCKERFILE=Dockerfile.bcc.base
BCC_BUILDER_DOCKERFILE=Dockerfile.bcc.builder
BCC_KEPLER_DOCKERFILE=Dockerfile.bcc.kepler
LIBBPF_BASE_DOCKERFILE=Dockerfile.libbpf.base
LIBBPF_BUILDER_DOCKERFILE=Dockerfile.libbpf.builder
LIBBPF_KEPLER_DOCKERFILE=Dockerfile.libbpf.kepler

while [[ $# -gt 0 ]]
do
    key="$1"

    case $key in
        -b|--base)
        BUILD_BASE=true
        ;;
        -r|--builder)
        BUILD_BUILDER=true
        ;;
        -k|--kepler)
        BUILD_KEPLER=true
        ;;
        -h|--help)
        show_usage
        ;;
        -p|--push)
        PUSH_ONLY=true
        ;;
        *)
        echo "Unknown option: $key"
        show_usage
        exit 1
        ;;
    esac
    shift
done

# remove build.log if it exists
if [ -f build.log ]
then
    rm build.log
fi

# Build and push images
if [ "$BUILD_BASE" = true ]
then   
    echo "$DOCKER_CMD build -t $IMAGE_REGISTRY/kepler_base:${BASE_TAG_PREFIX}${BCC_TAG_SUFFIX} -f $BCC_BASE_DOCKERFILE --build-arg ARCH=$ARCH . >> build.log 2>&1"
    $DOCKER_CMD build -t $IMAGE_REGISTRY/kepler_base:${BASE_TAG_PREFIX}${BCC_TAG_SUFFIX} -f $BCC_BASE_DOCKERFILE --build-arg ARCH=$ARCH . >> build.log 2>&1
    echo "$DOCKER_CMD build -t $IMAGE_REGISTRY/kepler_base:${BASE_TAG_PREFIX}${LIBBPF_TAG_SUFFIX} -f $LIBBPF_BASE_DOCKERFILE --build-arg ARCH=$ARCH . >> build.log 2>&1"
    $DOCKER_CMD build -t $IMAGE_REGISTRY/kepler_base:${BASE_TAG_PREFIX}${LIBBPF_TAG_SUFFIX} -f $LIBBPF_BASE_DOCKERFILE --build-arg ARCH=$ARCH . >> build.log 2>&1
fi

if [ "$BUILD_BUILDER" = true ]
then
    echo "$DOCKER_CMD build -t $IMAGE_REGISTRY/kepler_builder:${BUILDER_TAG_PREFIX}${BCC_TAG_SUFFIX} -f $BCC_BUILDER_DOCKERFILE . >> build.log 2>&1"
    $DOCKER_CMD build -t $IMAGE_REGISTRY/kepler_builder:${BUILDER_TAG_PREFIX}${BCC_TAG_SUFFIX} -f $BCC_BUILDER_DOCKERFILE . >> build.log 2>&1
    echo "$DOCKER_CMD build -t $IMAGE_REGISTRY/kepler_builder:${BUILDER_TAG_PREFIX}${LIBBPF_TAG_SUFFIX} -f $LIBBPF_BUILDER_DOCKERFILE . >> build.log 2>&1"
    $DOCKER_CMD build -t $IMAGE_REGISTRY/kepler_builder:${BUILDER_TAG_PREFIX}${LIBBPF_TAG_SUFFIX} -f $LIBBPF_BUILDER_DOCKERFILE . >> build.log 2>&1
fi

if [ "$BUILD_KEPLER" = true ]
then
    echo "$DOCKER_CMD build -t $IMAGE_REGISTRY/kepler:${KEPLER_BCC_TAG} -f $BCC_KEPLER_DOCKERFILE . >> build.log 2>&1"
    $DOCKER_CMD build -t $IMAGE_REGISTRY/kepler:${KEPLER_BCC_TAG} -f $BCC_KEPLER_DOCKERFILE . >> build.log 2>&1
    echo "$DOCKER_CMD build -t $IMAGE_REGISTRY/kepler:${KEPLER_LIBBPF_TAG} -f $LIBBPF_KEPLER_DOCKERFILE . >> build.log 2>&1"
    $DOCKER_CMD build -t $IMAGE_REGISTRY/kepler:${KEPLER_LIBBPF_TAG} -f $LIBBPF_KEPLER_DOCKERFILE . >> build.log 2>&1
fi

# Push images
if [ "$PUSH_ONLY" = true ]
then
    if [ "$BUILD_BASE" = true ]
    then
        echo "$DOCKER_CMD push $IMAGE_REGISTRY/kepler_base:${BASE_TAG_PREFIX}${BCC_TAG_SUFFIX} >> build.log 2>&1"
        $DOCKER_CMD push $IMAGE_REGISTRY/kepler_base:${BASE_TAG_PREFIX}${BCC_TAG_SUFFIX} >> build.log 2>&1
        echo "$DOCKER_CMD push $IMAGE_REGISTRY/kepler_base:${BASE_TAG_PREFIX}${LIBBPF_TAG_SUFFIX} >> build.log 2>&1"
        $DOCKER_CMD push $IMAGE_REGISTRY/kepler_base:${BASE_TAG_PREFIX}${LIBBPF_TAG_SUFFIX} >> build.log 2>&1
    fi

    if [ "$BUILD_BUILDER" = true ]
    then
        echo "$DOCKER_CMD push $IMAGE_REGISTRY/kepler_builder:${BUILDER_TAG_PREFIX}${BCC_TAG_SUFFIX} >> build.log 2>&1"
        $DOCKER_CMD push $IMAGE_REGISTRY/kepler_builder:${BUILDER_TAG_PREFIX}${BCC_TAG_SUFFIX} >> build.log 2>&1
        echo "$DOCKER_CMD push $IMAGE_REGISTRY/kepler_builder:${BUILDER_TAG_PREFIX}${LIBBPF_TAG_SUFFIX} >> build.log 2>&1"
        $DOCKER_CMD push $IMAGE_REGISTRY/kepler_builder:${BUILDER_TAG_PREFIX}${LIBBPF_TAG_SUFFIX} >> build.log 2>&1
    fi

    if [ "$BUILD_KEPLER" = true ]
    then
        echo "$DOCKER_CMD push $IMAGE_REGISTRY/kepler:${KEPLER_BCC_TAG} >> build.log 2>&1"
        $DOCKER_CMD push $IMAGE_REGISTRY/kepler:${KEPLER_BCC_TAG} >> build.log 2>&1
        echo "$DOCKER_CMD push $IMAGE_REGISTRY/kepler:${KEPLER_LIBBPF_TAG} >> build.log 2>&1"
        $DOCKER_CMD push $IMAGE_REGISTRY/kepler:${KEPLER_LIBBPF_TAG} >> build.log 2>&1
    fi
fi
