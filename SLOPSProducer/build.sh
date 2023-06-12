#!/bin/bash
go mod tidy
docker build -t ratnadeepb/slops-producer:latest .
docker push ratnadeepb/slops-producer:latest