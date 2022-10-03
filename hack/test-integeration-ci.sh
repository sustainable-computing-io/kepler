#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
set -o errexit
#set -x

main() {
    #kubectl -n kepler get pod
    #kubectl -n kepler get pods -oname
    #kubectl port-forward $(kubectl -n kepler get pods -oname) 9102:9102 -n kepler -v7 &
    #go test ./e2e/... --race --bench=. -cover --count=1 --vet=all
    build_test_image
    push_test_image
    run_job
}

main()