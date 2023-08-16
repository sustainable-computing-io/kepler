#!/bin/bash

echo "copy data files"

DATAPATH="/var/lib/kepler/data/"
mkdir -p ${DATAPATH}

cp ../data/normalized_cpu_arch.csv ${DATAPATH}

mkdir -p /var/lib/kepler/data/
cp ../data/model_weight/AbsPowerModel.json ${DATAPATH}/AbsPowerModel.json
cp ../data/model_weight/DynPowerModel.json ${DATAPATH}/DynPowerModel.json