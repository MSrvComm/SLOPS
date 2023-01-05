#!/bin/bash
kubectl create ns slops

BASEDIR=$(pwd)
if [[ $BASEDIR == *"scripts" ]]
then
    BASEDIR+="/.."
fi

$BASEDIR/start_cluster.sh
$BASEDIR/perms.sh
$BASEDIR/apps/deploy.sh all