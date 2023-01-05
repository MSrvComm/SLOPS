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
    kubectl apply -f $BASEDIR/k8s/tracer/deployment.yaml
    kubectl apply -f $BASEDIR/k8s/consumer/deployment.yaml
    kubectl apply -f $BASEDIR/k8s/producer/configmap.yaml
    kubectl apply -f $BASEDIR/k8s/producer/deployment.yaml
elif [[ $1 == "producer" ]]
then
    kubectl apply -f $BASEDIR/k8s/producer/configmap.yaml
    kubectl apply -f $BASEDIR/k8s/producer/deployment.yaml
else
    kubectl apply -f $BASEDIR/k8s/$1/deployment.yaml
fi