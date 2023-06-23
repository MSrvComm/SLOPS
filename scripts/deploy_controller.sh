#!/bin/bash
kubectl apply -f k8s/controller/svcAccount.yaml
kubectl apply -f k8s/controller/controllerDaemon.yaml