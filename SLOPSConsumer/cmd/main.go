package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/Shopify/sarama"
	"go.opentelemetry.io/contrib/instrumentation/github.com/Shopify/sarama/otelsarama"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	_ "go.uber.org/automaxprocs"
)

const (
	// kafkaConn = "kafka-service:9092"
	topic = "OrderGo"
	group = "OrderGroup"
)

func TracerProvider() (*sdktrace.TracerProvider, error) {
	url := os.Getenv("TRACER_COLLECTOR")
	// Create Jaeger exporter.
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}
	svcName := os.Getenv("TRACER_NAME")
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(svcName),
			attribute.String("exporter", "jaeger"),
		)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}))
	return tp, nil
}

func main() {
	tp, tperr := TracerProvider()

	if tperr != nil {
		log.Fatal(tperr)
	}

	// Cleanly shutdown and flush telemetry when the application exits.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func(ctx context.Context) {
		// Do not make the application hang when it is shutdown.
		ctx, cancel = context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}(ctx)

	keepRunning := true
	// sarama logging to stdout.
	sarama.Logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Llongfile)
	// consumer config
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.BalanceStrategySticky}
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.ClientID = os.Getenv("ADDRESS")
	// config.Consumer.Fetch.Max = 10
	// config.Consumer.Fetch.Min = 8

	kafkaConn := os.Getenv("KAFKA_BOOTSTRAP")

	consumer := Consumer{ready: make(chan bool)}
	propagators := propagation.TraceContext{}

	handler := otelsarama.WrapConsumerGroupHandler(&consumer, otelsarama.WithPropagators(propagators))

	client, err := sarama.NewConsumerGroup([]string{kafkaConn}, group, config)
	if err != nil {
		log.Panicf("Error creating consumer group client: %v", err)
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			// `Consume` should be called inside an infinite loop, when a
			// server-side rebalance happens, the consumer session will need to be
			// recreated to get the new claims
			if err := client.Consume(ctx, []string{topic}, handler); err != nil {
				log.Panicf("Error from consumer: %v", err)
			}

			// check if context was cancelled, signaling that the consumer should stop
			if ctx.Err() != nil {
				return
			}
			consumer.ready = make(chan bool)
		}
	}()

	<-consumer.ready // Await till the consumer has been set up
	log.Println("Sarama consumer up and running!...")

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)

	for keepRunning {
		select {
		case <-ctx.Done():
			log.Println("terminating: context cancelled")
			keepRunning = false
		case <-sigterm:
			log.Println("terminating: via signal")
			keepRunning = false
		}
	}

	cancel()
	wg.Wait()
	if err = client.Close(); err != nil {
		log.Panicf("Error closing client: %v", err)
	}
}

type Consumer struct {
	ready chan bool
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (consumer *Consumer) Setup(sarama.ConsumerGroupSession) error {
	// Mark the consumer as ready
	close(consumer.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (consumer *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (consumer *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	containerIP := os.Getenv("ADDRESS")

	// env vars
	svcTm, err := strconv.Atoi(os.Getenv("SVC_TIME_MS"))
	if err != nil {
		log.Fatal("Service Time not defined")
	}

	// NOTE:
	// Do not move the code below to a goroutine.
	// The `ConsumeClaim` itself is called within a goroutine, see:
	// https://github.com/Shopify/sarama/blob/main/consumer_group.go#L27-L29
	for {
		select {
		case message := <-claim.Messages():
			printMessage(message, svcTm, containerIP)
			// Commit message
			session.MarkMessage(message, "")
		// Should return when `session.Context()` is done.
		// If not, will raise `ErrRebalanceInProgress` or `read tcp <ip>:<port>: i/o timeout` when kafka rebalance. see:
		// https://github.com/Shopify/sarama/issues/1192
		case <-session.Context().Done():
			return nil
		}
	}
}

func printMessage(msg *sarama.ConsumerMessage, svcTm int, ip string) {
	// Extract tracing info from message
	propagators := propagation.TraceContext{}
	ctx := propagators.Extract(context.Background(), otelsarama.NewConsumerMessageCarrier(msg))
	log.Println("HEADERS:", msg.Headers)
	tr := otel.Tracer("consumer")

	// Create a span.printMessage
	_, span := tr.Start(ctx, "print message")

	defer span.End()

	// Inject current span context, so any further processing can use it to propagate span.
	propagators.Inject(ctx, otelsarama.NewConsumerMessageCarrier(msg))

	// workhorse
	key := string(msg.Key)
	hdrs := msg.Headers
	start := time.Now()
	for {
		if time.Since(start) > time.Duration(svcTm)*time.Microsecond {
			break
		}
	}

	for _, hdr := range hdrs {
		if string(hdr.Key) == "Producer" {
			log.Println("Arrived from producer:", string(hdr.Value))
		}

		// Detect and Handle sync events.
		if string(hdr.Key) == "SyncEvent" {
			dec := gob.NewDecoder(bytes.NewBuffer(hdr.Value))
			var msgset MessageSet
			err := dec.Decode(&msgset)
			if err != nil {
				log.Println("Decoding err:", err)
				return
			}
			// Check if this is the last message of a set.
			if msgset.DestPartition != msg.Partition {
				HandleShiftKey(key)
			}
			// Ignore new streams.
			if msgset.SrcPartition > -1 && msgset.SrcMsgsetIndex > -1 {
				HandleSyncEvent(msgset)
			}
		}
	}

	time.Sleep(time.Millisecond * time.Duration(svcTm))
	// Set any additional attributes that might make sense
	// span.SetAttributes(attribute.String("consumed message at offset",strconv.FormatInt(int64(msg.Offset),10)))
	// span.SetAttributes(attribute.String("consumed message to partition",strconv.FormatInt(int64(msg.Partition),10)))
	span.SetAttributes(attribute.Int64("message_bus.destination", int64(msg.Partition)))
	span.SetAttributes(attribute.String("consumer.key", key))
	log.Printf("Container %s processed key \"%s\" from \"%d\" at offset \"%d\"",
		ip,
		key,
		msg.Partition,
		msg.Offset)
}

// Handle key shift events.
// This basically means that a consumer is being told that a stream (key)
// has started a new message set on a different partition.
func HandleShiftKey(key string) {
	log.Println("Handling Shift Key event for:", key)
}

// Handle sync events.
// This happens when a stream (key) has started a new message set
// on this partition after having shifted from another partition.
// This doesn't get triggered when a new stream is starting.
func HandleSyncEvent(msgset MessageSet) {
	log.Println("Handling Sync Event")
	log.Printf("Key %s is starting message set %d on partition %d shifting from partition %d\n",
		msgset.Key, msgset.DestMsgsetIndex, msgset.DestPartition, msgset.SrcPartition,
	)
}
