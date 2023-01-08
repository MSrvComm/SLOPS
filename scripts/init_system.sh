#!/bin/bash
kubectl create ns slops

BASEDIR=$(pwd)
if [[ $BASEDIR == *"SLOPS" ]]
then
    BASEDIR+="/scripts"
fi

$BASEDIR/start_cluster.sh
$BASEDIR/perms.sh
kubectl wait kafka -n slops --for condition=Ready ordergo
$BASEDIR/apps/deploy.sh all