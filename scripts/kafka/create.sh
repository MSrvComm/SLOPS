#!/bin/bash
kubectl exec -it ordergo-kafka-0 -n slops -- bin/kafka-topics.sh --bootstrap-server ordergo-kafka-bootstrap:9092 --create --replication-factor 2 --partitions $1 --topic OrderGo