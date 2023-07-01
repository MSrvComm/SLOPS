#!/bin/bash
go mod tidy
docker build -t ratnadeepb/slops-producer-state-tol:latest .
docker push ratnadeepb/slops-producer-state-tol:latest