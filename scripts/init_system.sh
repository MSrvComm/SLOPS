#!/bin/bash
kubectl create ns slops
./start_cluster.sh
./perms.sh
./apps/deploy.sh all