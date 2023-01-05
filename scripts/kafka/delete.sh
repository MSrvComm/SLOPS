#!/bin/bash
kubectl exec -it ordergo-kafka-0 -n slops -- bin/kafka-topics.sh --bootstrap-server ordergo-kafka-bootstrap:9092 --delete --topic OrderGo