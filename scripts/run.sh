#!/bin/bash
PORT=$(kubectl get svc producer -n slops -o go-template='{{range.spec.ports}}{{if .nodePort}}{{.nodePort}}{{"\n"}}{{end}}{{end}}')
../SLOPSClient/SLOPSClient --url http://localhost:$PORT/new