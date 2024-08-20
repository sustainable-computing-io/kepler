#!/bin/bash

echo "copy data files"

DATAPATH="/var/lib/kepler/data/"
mkdir -p ${DATAPATH}

cp ../data/cpus.yaml ${DATAPATH}

mkdir -p /var/lib/kepler/data/model_weight
cp ../data/model_weight/acpi_AbsPowerModel.json ${DATAPATH}/model_weight/acpi_AbsPowerModel.json
cp ../data/model_weight/acpi_DynPowerModel.json ${DATAPATH}/model_weight/acpi_DynPowerModel.json
cp ../data/model_weight/intel_rapl_AbsPowerModel.json ${DATAPATH}/model_weight/intel_rapl_AbsPowerModel.json
cp ../data/model_weight/intel_rapl_DynPowerModel.json ${DATAPATH}/model_weight/intel_rapl_DynPowerModel.json
