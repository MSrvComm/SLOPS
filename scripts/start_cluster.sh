#!/bin/bash
BASEDIR=$(pwd)
if [[ $BASEDIR == *"scripts" ]]
then
    BASEDIR+="/.."
fi

kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.9.1/cert-manager.yaml
kubectl create namespace observability
kubectl create -f https://github.com/jaegertracing/jaeger-operator/releases/download/v1.40.0/jaeger-operator.yaml -n observability
kubectl apply -f k8s/cluster/jaeger.yaml -n slops
kubectl apply -f $BASEDIR/k8s/cluster/kafka-ephemeral.yaml -n slops