# SMALOPS: State Migration Aware Load Switching in Order Preserving Systems

## Branches

- `main`: basic SMALOPS. Rebalances at discrete intervals to allow the system to settle down after each rebalancing.

- `main-tolerance`: Rebalances at discrete intervals to allow the system to settle down after each rebalancing, but does not rebalance if load imbalance is not beyond a threshold.

- `state-tracking`: Rebalances continuously, almost - there is small sleep time to prevent busy looping. Tracks migration timestamp for each flow and ignores recently migrated flows.

- `state-tracking-tol`: Same as `state-tracking` but skips rebalancing if load imbalance is within a threshold.

- `unordered`: holds the code for testing with messages where no ordering is required and has a single producer i.e. producers do not talk about which keys are assigned to which partition and consumer process each message as they arrive.

- `msgset`: implements message set based synchronization. Messages belonging to the cold keys are load balanced with the random partitioner, while hot keys are load balanced using a least weight of two random choices algorithm.

## SLOPSClient

This is an open loop client that generates keys according to a zipf distribution with configurable parameters. These keys are then sent to the [producer](#slopsproducer).

The generated keys are of the format `location_<number>`.

## SLOPSProducer

The SLOPS producer creates Kafka events and sends them to Kafka after marking them with Jaeger spans.

The producer can be configured in different ways using environment variables.
- `VANILLA`: decides whether the producer uses the SLOPS algorithms or the vanilla Kafka ones.
- `P2C`: should the producer use power-of-two-random-choices (P2C) to assign flows to partitions.
- `LOSSY`: should it use lossy counting or count every message explicitly.

## SLOPSConsumer

This consumer gets the messages from Kafka and extracts the Jaeger span while "processing" the message for a configured amount of time.

## Deploying Jaeger

[How to deploy Jaeger](https://www.jaegertracing.io/docs/1.40/operator/)</br>
[Tracing Kafka Records](https://newrelic.com/blog/how-to-relic/distributed-tracing-with-kafka)

### Install the Cert Manager

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.9.1/cert-manager.yaml
```

### Install the Jaeger Operator

```bash
kubectl create namespace observability
kubectl create -f https://github.com/jaegertracing/jaeger-operator/releases/download/v1.40.0/jaeger-operator.yaml -n observability
kubectl apply -f k8s/cluster/jaeger.yaml
```

**Forwarding the Jaeger Trace port on Kuberenetes master node**

```bash
kubectl port-forward -n slops svc/jaeger-trace-query 16686:16686
```

**Forwarding the Jaeger Trace port over SSH**

```bash
ssh -L 16686:localhost:16686 node0
```

## Kafka Commands

```bash
kubectl exec -it ordergo-kafka-0 -n slops -- bin/kafka-topics.sh --bootstrap-server ordergo-kafka-bootstrap:9092 --list
```

```bash
kubectl exec -it ordergo-kafka-0 -n slops -- bin/kafka-topics.sh --bootstrap-server ordergo-kafka-bootstrap:9092 --describe --topic OrderGo
```

```bash
kubectl exec -it ordergo-kafka-0 -n slops -- bin/kafka-topics.sh --bootstrap-server ordergo-kafka-bootstrap:9092 --delete --topic OrderGo
```

```bash
kubectl exec -it ordergo-kafka-0 -n slops -- bin/kafka-topics.sh --bootstrap-server ordergo-kafka-bootstrap:9092 --create --replication-factor 2 --partitions 10 --topic OrderGo
```

```bash
kubectl exec -it ordergo-kafka-0 -n slops -- bin/kafka-topics.sh --bootstrap-server ordergo-kafka-bootstrap:9092 --alter --partitions 10 --topic OrderGo
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

## Automatic Build and Deployment

The build and deployment scripts are located in the [`scripts`](./scripts/) folders while the Kubernetes deployment files are located in the [`k8s`](./k8s/) folder.

Furthermore, the `Kafka` related scripts are in [`kafka`](./scripts/kafka/) subfolder and the application related scripts are in the [`apps`](./scripts/apps/) subfolder.

The kafka topic creation [script](./scripts/kafka/create.sh) takes a single parameter indicating number of partitions to be created. The apps [`deploy`](./scripts/apps/deploy.sh) and [`delete`](./scripts/apps/delete.sh) scripts take a parameter to indicate whether to shutdown a specific application or all the applications.

Starting and stopping the main cluster can be done through the [start](./scripts/start_cluster.sh) and [stop](./scripts/stop_cluster.sh) scripts.

The system initialization [script](./scripts/init_system.sh) does the following:
- Create the `slops` namespace.
- Deploy the Kafka cluster.
- Deploy the applications.

After that the client can be run with the [run](./scripts/apps/run.sh) script. The run script accepts the following arguments:
- The request rate (Default 200).
- The total number of requests to be made (Default 1000).
- The number of keys to choose from (Default 2800).

Usage:
```bash
./SLOPSClient/SLOPSClient --rate 200 --iter 1000 --keys 2800
```

### Repeating the experiment

Generally, it involves redeploying the apps and recreating the kafka topics. The [`redeploy`](./scripts/redeploy.sh) script does this automatically. It accepts a `--vanilla bool` argument that sets the `VANILLA` environment argument in the producer's deployment yaml. This in turn tells the producer whether we want to use the SLOPS method or just the vanilla system.

```bash
. scripts/redeploy.sh
```

## Checking Processing Times By Partition

Filter data in the Jaeger UI by the partition number tags (under service `consumer`):

```None
messaging.kafka.partition=<partition_number>
```

In order to filter by key, use the tag (under service `producer`):

```None
producer.key=<key>
```

The same information is available with the tag:

```None
consumer.key=<key>
```

Filtering results for `key` and `partition` can be done with:

```None
consumer.key=<key> message_bus.destination=<partition_number>
```

### Getting the data from Jaeger

```Python
import requests
url = f"http://localhost:16686/api/traces?service={service}&loopback={hours}h&prettyPrint=true&limit={limit}"
requests.get(url)
```

where `service` is the service for which we are attempting to download the traces, `hours` is how far back in hours we should look and `limit` is the number of traces we want to download.
