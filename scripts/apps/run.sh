#!/bin/bash
BASEDIR=$(pwd)
if [[ $BASEDIR == *"apps" ]]
then
    BASEDIR+="/../.."
elif [[ $BASEDIR == *"scripts" ]]
then
    BASEDIR+="/.."
fi

# gw=${gw:-0}
rate=${rate:-1000}
iter=${iter:-300000} # 5 minutes
keys=${keys:-28000000}

while [ $# -gt 0 ]; do
    if [[ $1 == "--"* ]]; then
        v="${1/--/}"
        declare "$v"="$2"
        shift   
    fi
    shift
done

PORT=$(kubectl get svc gateway -n slops -o go-template='{{range.spec.ports}}{{if .nodePort}}{{.nodePort}}{{"\n"}}{{end}}{{end}}')
$BASEDIR/SLOPSClient/SLOPSClient -rate $rate -iter $iter -keys $keys -url http://localhost:$PORT/new

# if [ $gw == 0 ]
# then
#     rate=2000
#     iter=600000
#     PORT=$(kubectl get svc gateway -n slops -o go-template='{{range.spec.ports}}{{if .nodePort}}{{.nodePort}}{{"\n"}}{{end}}{{end}}')
#     $BASEDIR/SLOPSClient/SLOPSClient -rate $rate -iter $iter -keys $keys -url http://localhost:$PORT/new
# else
#     PORT=$(kubectl get svc producer -n slops -o go-template='{{range.spec.ports}}{{if .nodePort}}{{.nodePort}}{{"\n"}}{{end}}{{end}}')
#     $BASEDIR/SLOPSClient/SLOPSClient -rate $rate -iter $iter -keys $keys -url http://localhost:$PORT/new
# fi