#!/bin/bash
go mod tidy
docker build -t ratnadeepb/slops-consumer:latest .
docker push ratnadeepb/slops-consumer:latest