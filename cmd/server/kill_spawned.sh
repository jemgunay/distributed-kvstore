#!/bin/bash

# get NUM_SERVICES from commandline arg provided - default to 4
NUM_SERVICES=${1}
if [[ ${NUM_SERVICES} == '' ]]; then
    NUM_SERVICES=4
fi

# kill existing services
for i in $(seq 1 ${NUM_SERVICES}); do
    port_num=$((7000 + ${i}))
    fuser -k ${port_num}/tcp &
done
