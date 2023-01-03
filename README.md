# SLOPS: Switching Load in Order Preserving Systems

## SLOPSClient

This is an open loop client that generates keys according to a zipf distribution with configurable parameters. These keys are then sent to the [producer](#slopsproducer)

## SLOPSProducer

The SLOPS producer creates Kafka events and sends them to Kafka after marking them with Jaeger spans.

## SLOPSConsumer

This consumer gets the messages from Kafka and extracts the Jaeger span while "processing" the message for a configured amount of time.
