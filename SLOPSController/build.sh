#!/bin/bash
go mod tidy
docker build -t ratnadeepb/slops-controller:latest .
docker push ratnadeepb/slops-controller:latest