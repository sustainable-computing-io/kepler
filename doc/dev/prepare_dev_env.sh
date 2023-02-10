#!/bin/bash

echo "copy data files"

DATAPATH="/var/lib/kepler/data/"
mkdir -p ${DATAPATH}

cp ../data/normalized_cpu_arch.csv ${DATAPATH}

