#!/bin/bash
go mod tidy
docker build -t ratnadeepb/slops-gateway:latest .
docker push ratnadeepb/slops-gateway:latest