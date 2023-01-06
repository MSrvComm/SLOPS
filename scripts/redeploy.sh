#!/bin/bash
./scripts/apps/delete.sh all
sleep 5
./scripts/kafka/delete.sh
./scripts/kafka/create.sh 10
./scripts/apps/deploy.sh all