#!/bin/bash
BASEDIR=$(pwd)
if [[ $BASEDIR == *"apps" ]]
then
    BASEDIR+="/../.."
elif [[ $BASEDIR == *"scripts" ]]
then
    BASEDIR+="/.."
fi

if [[ $1 == "all" ]]
then
    kubectl apply -f $BASEDIR/k8s/gateway/deployment.yaml
    kubectl apply -f $BASEDIR/k8s/consumer/deployment.yaml
    kubectl apply -f $BASEDIR/k8s/producer/configmap.yaml
    kubectl apply -f $BASEDIR/k8s/producer/deployment.yaml
elif [[ $1 == "producer" ]]
then
    kubectl apply -f $BASEDIR/k8s/producer/configmap.yaml
    kubectl apply -f $BASEDIR/k8s/producer/deployment.yaml
elif [[ $1 == "gateway" ]]
then
    kubectl apply -f $BASEDIR/k8s/gateway/deployment.yaml
else
    kubectl apply -f $BASEDIR/k8s/$1/deployment.yaml
fi