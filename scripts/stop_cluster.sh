#!/bin/bash
BASEDIR=$(pwd)
if [[ $BASEDIR == *"SLOPS" ]]
then
    BASEDIR+="/scripts"
fi
$BASEDIR/apps/delete.sh all
kubectl delete -f $BASEDIR/k8s/cluster/kafka-ephemeral.yaml -n slops