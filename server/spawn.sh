#!/bin/bash

# get NUM_SERVICES from commandline arg provided - default to 4
NUM_SERVICES=${1}
if [[ ${NUM_SERVICES} == '' ]]; then
    NUM_SERVICES=4
fi


# kill existing services
for i in $(seq 1 ${NUM_SERVICES}); do
    fuser -k 700${i}/tcp
done

# build service
go build

for i in $(seq 1 ${NUM_SERVICES}); do
    # build node_address flags
    cmd="./server -port=700${i}"
    for j in $(seq 1 ${NUM_SERVICES}); do
        if [[ "${j}" == "${i}" ]]; then
            continue
        fi

        cmd="${cmd} -node_address=\":700${j}\""
    done

    # create service
    echo ''
    eval ${cmd} &
done
