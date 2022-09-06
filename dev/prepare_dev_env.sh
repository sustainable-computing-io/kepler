#!/bin/bash

echo "copy data files"

DATAPATH="/var/lib/kepler/data/"
mkdir -p ${DATAPATH}

cp ../data/normalized_cpu_arch.csv ${DATAPATH}
cp ../data/power_data.csv          ${DATAPATH}
cp ../data/power_model.csv         ${DATAPATH}

