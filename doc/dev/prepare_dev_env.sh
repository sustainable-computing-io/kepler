#!/bin/bash

echo "copy data files"

DATAPATH="/var/lib/kepler/data/"
mkdir -p ${DATAPATH}

cp ../data/normalized_cpu_arch.csv ${DATAPATH}

mkdir -p /var/lib/kepler/data/
curl -s https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-server/main/tests/test_models/DynComponentModelWeight/CgroupOnly/ScikitMixed/ScikitMixed.json --output /var/lib/kepler/data/ScikitMixed.json
curl -s https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-server/main/tests/test_models/AbsComponentModelWeight/Full/KerasCompWeightFullPipeline/KerasCompWeightFullPipeline.json --output /var/lib/kepler/data/KerasCompWeightFullPipeline.json