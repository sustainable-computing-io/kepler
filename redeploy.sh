#!/bin/bash
make build-manifest OPTS="CI_DEPLOY DEBUG_DEPLOY"
#!/bin/bash
make image-prune
make build_image
img=$(docker images | grep quay.io/sustainable_computing_io/kepler | awk 'NR==1{ print $3 }')
docker tag "$img" localhost:5001/kepler:devel
docker push localhost:5001/kepler:devel
kubectl delete -f _output/generated-manifest/; kubectl apply -f _output/generated-manifest
