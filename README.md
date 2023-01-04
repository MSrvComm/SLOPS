# SLOPS: Switching Load in Order Preserving Systems

## SLOPSClient

This is an open loop client that generates keys according to a zipf distribution with configurable parameters. These keys are then sent to the [producer](#slopsproducer).

## SLOPSProducer

The SLOPS producer creates Kafka events and sends them to Kafka after marking them with Jaeger spans.

## SLOPSConsumer

This consumer gets the messages from Kafka and extracts the Jaeger span while "processing" the message for a configured amount of time.

## Deploying Jaeger

[How to deploy Jaeger](https://www.jaegertracing.io/docs/1.40/deployment/)

### Install the Cert Manager

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.9.1/cert-manager.yaml
```

### Install Jaeger Operator

```bash
kubectl create namespace observability
kubectl create -f https://github.com/jaegertracing/jaeger-operator/releases/download/v1.40.0/jaeger-operator.yaml -n observability
```

## Kafka Commands

```bash
kubectl exec -it kafka-0 -- kafka-topics.sh --zookeeper zk-cs.default.svc.cluster.local:2181 --list
```

```bash
kubectl exec -it kafka-0 -- kafka-topics.sh --zookeeper zk-cs.default.svc.cluster.local:2181 --describe --topic OrderGo
```

```bash
kubectl exec -it kafka-0 -- kafka-topics.sh --zookeeper zk-cs.default.svc.cluster.local:2181 --delete --topic OrderGo
```

```bash
kubectl exec -it kafka-0 -- kafka-topics.sh --zookeeper zk-cs.default.svc.cluster.local:2181 --create --replication-factor 2 --partitions 10 --topic OrderGo
```

## Strimzi

[Strimzi Documentation](https://strimzi.io/documentation/)

### Install the Operator in NS 'slops'

```bash
kubectl create ns slops
kubectl create -f 'https://strimzi.io/install/latest?namespace=slops' -n slops
```

### Create the cluster

This creates a cluster with 3 brokers and 1 zookeeper.

```bash
kubectl apply -f k8s/cluster/kafka-ephemeral.yaml -n slops
```
