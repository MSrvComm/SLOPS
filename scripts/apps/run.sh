#!/bin/bash
rate=${rate:-200}
iter=${iter:-1000}
keys=${keys:-2800}
burst=${rate}

PORT=$(kubectl get svc producer -n slops -o go-template='{{range.spec.ports}}{{if .nodePort}}{{.nodePort}}{{"\n"}}{{end}}{{end}}')
../../SLOPSClient/SLOPSClient --rate ${rate} --iter ${iter} --burst ${burst} --keys ${keys} --url http://localhost:$PORT/new