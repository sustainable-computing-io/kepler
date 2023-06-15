#!/bin/bash
set -x
IMAGE_BASE="quay.io/sustainable_computing_io/kepler_kernel_source_images"
for i in "8" "9"; do
    Image="registry.access.redhat.com/ubi${i}/ubi" 
    echo "Building $i"
    # podman doesn't support --build-arg
    # replace ImageName with the actual image name using sed
    sed "s|ImageName|${Image}|g" Dockerfile.ubi > Dockerfile.ubi.${i}

    docker build --build-arg ImageName=${Image} -t ${IMAGE_BASE}:ubi${i}  -f Dockerfile.ubi.${i} .
    docker push ${IMAGE_BASE}:ubi${i}
    rm Dockerfile.ubi.${i}
done
