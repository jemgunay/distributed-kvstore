#!/bin/bash

# Starts up X node instances each on their own port, and points each node to the other nodes via node_address service
# flags. Existing services on the target port range will be killed first.

# get NUM_SERVICES from commandline arg provided - default to 4
NUM_SERVICES=${1}
if [[ ${NUM_SERVICES} == '' ]]; then
    NUM_SERVICES=3
fi

# build service
go build
if [[ $? != 0 ]]; then
    echo "failed to build"
    exit 1
fi

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
    if [[ ${i} == ${NUM_SERVICES} ]]; then
      eval ${cmd}
    else
      eval ${cmd} &
    fi
done

# wait for all services to identify each other
sleep 5