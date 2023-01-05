#!/bin/bash
if [ $1 == "all" ]
then
    kubectl delete -f ../../k8s/tracer/deployment.yaml
    kubectl delete -f ../../k8s/consumer/deployment.yaml
    kubectl delete -f ../../k8s/producer/deployment.yaml
else
    kubectl delete -f ../../k8s/$1/deployment.yaml
fi