#!/bin/bash
./scripts/apps/delete.sh all
kubectl wait --for=delete deployment/consumer -n slops
kubectl wait --for=delete deployment/producer -n slops
./scripts/kafka/delete.sh
./scripts/kafka/create.sh 100
./scripts/apps/deploy.sh all