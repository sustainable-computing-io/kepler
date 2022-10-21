#!/usr/bin/env bash
#
# This file is part of the Kepler project
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at

#     http://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Copyright 2022 The Kepler Contributors
#

if [ -z "$CLUSTER_PROVIDER" ]; then
    echo 'You must define a cluster provider (e.g. CLUSTER_PROVIDER="kind")'
    exit 1
fi

version=$(kubectl version --short | grep 'Client Version' | sed 's/.*v//g' | cut -b -4)
if [ 1 -eq "$(echo "${version} < 1.21" | bc)" ]
then
    echo "You need to update your kubectl version to 1.21+ to support kustomize"
    exit 1
fi

source cluster-up/cluster/$CLUSTER_PROVIDER/common.sh
up
