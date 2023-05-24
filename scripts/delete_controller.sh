#!/bin/bash
kubectl delete -f k8s/controller/svcAccount.yaml
kubectl delete -f k8s/controller/controllerDaemon.yaml