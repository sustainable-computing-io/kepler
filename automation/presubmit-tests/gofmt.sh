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
# Copyright 2022 The Kepler Contributors.
#

set -e

echo "Checking go format"
sources="pkg/ cmd/"
unformatted=$(gofmt -e -d -s -l $sources)
if [ ! -z "$unformatted" ]; then
    # Some files are not gofmt.
    echo >&2 "The following Go files must be formatted with gofmt:"
    for fn in $unformatted; do
        echo >&2 "  $fn"
    done
    echo >&2 "Please run 'make format'."
    exit 1
fi

exit 0