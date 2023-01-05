# SLOPS: Switching Load in Order Preserving Systems

## SLOPSClient

This is an open loop client that generates keys according to a zipf distribution with configurable parameters. These keys are then sent to the [producer](#slopsproducer).

## SLOPSProducer

The SLOPS producer creates Kafka events and sends them to Kafka after marking them with Jaeger spans.

## SLOPSConsumer

This consumer gets the messages from Kafka and extracts the Jaeger span while "processing" the message for a configured amount of time.

## Tracing Infrastucture

Each producer generates a unique sequence for every new request and updates it to separate tracer service. The producer then adds the sequence number to the message generated from the request.

The consumer, upon receiving a message, processes the message and extracts the sequence. The consumer then sends the sequence to the tracer service.

The tracer service opens a trace with a start time upon receiving a sequence from a producer. The tracer keeps the trace open till it receives the same sequence number from the consumer. At this point the tracer closes the trace and saves it as a completed job. A completed job is a trace ID with duration for which the trace was open.

A third API, `/all`, can be used to fetch the entire list of completed jobs.

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
