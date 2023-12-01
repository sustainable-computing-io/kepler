#!/bin/bash

echo "copy data files"

DATAPATH="/var/lib/kepler/data/"
mkdir -p ${DATAPATH}

cp ../data/cpus.yaml ${DATAPATH}

mkdir -p /var/lib/kepler/data/
cp ../data/model_weight/acpi_AbsPowerModel.json ${DATAPATH}/acpi_AbsPowerModel.json
cp ../data/model_weight/acpi_DynPowerModel.json ${DATAPATH}/acpi_DynPowerModel.json
cp ../data/model_weight/rapl_AbsPowerModel.json ${DATAPATH}/rapl_AbsPowerModel.json
cp ../data/model_weight/rapl_DynPowerModel.json ${DATAPATH}/rapl_DynPowerModel.json
