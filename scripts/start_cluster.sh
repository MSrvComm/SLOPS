#!/bin/bash
BASEDIR=$(pwd)
if [[ $BASEDIR == *"scripts" ]]
then
    BASEDIR+="/.."
fi

kubectl apply -f $BASEDIR/k8s/cluster/kafka-ephemeral.yaml -n slops