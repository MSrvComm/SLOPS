#!/bin/bash
vanilla=${vanilla:-true}
while [ $# -gt 0 ]; do
    if [[ $1 == "--"* ]]; then
        v="${1/--/}"
        declare "$v"="$2"
        shift   
    fi
    shift
done
./scripts/apps/delete.sh all
sleep 5
./scripts/kafka/delete.sh
./scripts/kafka/create.sh 10
./scripts/apps/deploy.sh all