#!/bin/bash

# get NUM_SERVICES from commandline arg provided - default to 4
NUM_SERVICES=${1}
if [[ ${NUM_SERVICES} == '' ]]; then
    NUM_SERVICES=4
fi

# kill existing services
for i in $(seq 1 ${NUM_SERVICES}); do
    port_num=$((7000 + ${i}))
    fuser -k ${port_num}/tcp
done

# build service
go build

for i in $(seq 1 ${NUM_SERVICES}); do
    # build node_address flags
    port_num=$((7000 + ${i}))
    cmd="./server -port=${port_num}"
    for j in $(seq 1 ${NUM_SERVICES}); do
        if [[ ${j} == ${i} ]]; then
            continue
        fi

        port_num=$((7000 + ${j}))
        cmd="${cmd} -node_address=\":${port_num}\""
    done

    # create service
    eval ${cmd} &
done

echo ''