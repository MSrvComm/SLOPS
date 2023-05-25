#!/bin/bash
#!/bin/bash
BASEDIR=$(pwd)
if [[ $BASEDIR == *"apps" ]]
then
    BASEDIR+="/../.."
elif [[ $BASEDIR == *"scripts" ]]
then
    BASEDIR+="/.."
fi

if [[ $1 == "all" ]]
then
    kubectl delete -f $BASEDIR/k8s/consumer/deployment.yaml
    kubectl delete -f $BASEDIR/k8s/producer/configmap.yaml
    kubectl delete -f $BASEDIR/k8s/producer/deployment.yaml
    kubectl delete -f $BASEDIR/k8s/gateway/deployment.yaml
else
    kubectl delete -f $BASEDIR/k8s/$1/deployment.yaml
fi