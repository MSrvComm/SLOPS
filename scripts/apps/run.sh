#!/bin/bash
BASEDIR=$(pwd)
if [[ $BASEDIR == *"apps" ]]
then
    BASEDIR+="/../.."
elif [[ $BASEDIR == *"scripts" ]]
then
    BASEDIR+="/.."
fi

rate=${rate:-200}
iter=${iter:-1000}
keys=${keys:-2800}

while [ $# -gt 0 ]; do
    if [[ $1 == "--"* ]]; then
        v="${1/--/}"
        declare "$v"="$2"
        shift   
    fi
    shift
done

PORT=$(kubectl get svc producer -n slops -o go-template='{{range.spec.ports}}{{if .nodePort}}{{.nodePort}}{{"\n"}}{{end}}{{end}}')
$BASEDIR/SLOPSClient/SLOPSClient -rate $rate -iter $iter -keys $keys -url http://localhost:$PORT/new