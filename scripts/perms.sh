#!/bin/bash
BASEDIR=$(pwd)
if [[ $BASEDIR == *"scripts" ]]
then
    BASEDIR+="/.."
fi

kubectl apply -f $BASEDIR/k8s/producer/svcAccount.yaml