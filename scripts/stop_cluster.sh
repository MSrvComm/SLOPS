#!/bin/bash
SCRIPTDIR=$(pwd)
if [[ $SCRIPTDIR == *"SLOPS" ]]
then
    SCRIPTDIR+="/scripts"
fi
$SCRIPTDIR/apps/delete.sh all
K8DIR=$(pwd)
if [[ $K8DIR == *"/scripts" ]]
then
    K8DIR+="/.."
fi
kubectl delete -f $K8DIR/k8s/cluster/kafka-ephemeral.yaml -n slops

kubectl delete -f k8s/controller/svcAccount.yaml
kubectl delete -f k8s/controller/controllerDaemon.yaml