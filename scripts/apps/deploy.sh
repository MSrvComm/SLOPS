#!/bin/bash
if [ $1 == "all" ]
then
    kubectl apply -f ../../k8s/tracer/deployment.yaml
    kubectl apply -f ../../k8s/consumer/deployment.yaml
    kubectl apply -f ../../k8s/producer/configmap.yaml
    kubectl apply -f ../../k8s/producer/deployment.yaml
elif [ $1 == "producer" ]
then
    kubectl apply -f ../../k8s/producer/configmap.yaml
    kubectl apply -f ../../k8s/producer/deployment.yaml
else
    kubectl apply -f ../../k8s/$1/deployment.yaml
fi