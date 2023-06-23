#!/bin/bash
BASEDIR=$(pwd)
if [[ $BASEDIR == *"scripts" ]]
then
    BASEDIR+="/.."
fi

kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.9.1/cert-manager.yaml
kubectl wait pods -n cert-manager -l app=webhook --for condition=Ready
kubectl create namespace observability
kubectl create -f https://github.com/jaegertracing/jaeger-operator/releases/download/v1.40.0/jaeger-operator.yaml -n observability
kubectl wait pods -n observability -l name=jaeger-operator --for condition=Ready
kubectl apply -f k8s/cluster/jaeger.yaml -n slops

kubectl apply -f k8s/controller/svcAccount.yaml
kubectl apply -f k8s/controller/controllerDaemon.yaml

kubectl create -f 'https://strimzi.io/install/latest?namespace=slops' -n slops
kubectl wait pods -n slops -l name=strimzi-cluster-operator --for condition=Ready
kubectl apply -f $BASEDIR/k8s/cluster/kafka-ephemeral.yaml -n slops