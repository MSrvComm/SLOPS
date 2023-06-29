#!/bin/bash
go mod tidy
docker build -t ratnadeepb/slops-producer-tolerance:latest .
docker push ratnadeepb/slops-producer-tolerance:latest