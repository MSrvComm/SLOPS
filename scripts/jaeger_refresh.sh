#!/bin/bash
kubectl delete -f k8s/cluster/jaeger.yaml -n slops
kubectl wait --for=delete jaeger/jaeger-trace -n slops
kubectl apply -f k8s/cluster/jaeger.yaml -n slops