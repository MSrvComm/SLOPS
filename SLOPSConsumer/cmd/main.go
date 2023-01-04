package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/Shopify/sarama"
	"go.opentelemetry.io/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/contrib/instrumentation/github.com/Shopify/sarama/otelsarama"
)

const (
	// kafkaConn = "kafka-service:9092"
	topic = "OrderGo"
	group = "OrderGroup"
)

func main() {
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

	c := Consumer{
		// ready:  make(chan bool),
		// config: config,
	}

	consumer := otelsarama.WrapConsumerGroupHandler(&c)

	ctx, cancel := context.WithCancel(context.Background())
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
			if err := client.Consume(ctx, []string{topic}, consumer); err != nil {
				log.Panicf("Error from consumer: %v", err)
			}

			// check if context was cancelled, signaling that the consumer should stop
			if ctx.Err() != nil {
				return
			}
			// consumer.ready = make(chan bool)
		}
	}()

	// <-consumer.ready // Await till the consumer has been set up
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
	// ready  chan bool
	// config *sarama.Config
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (consumer *Consumer) Setup(sarama.ConsumerGroupSession) error {
	// Mark consumer as ready.
	// close(consumer.ready)
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
			ctx := otel.GetTextMapPropagator().Extract(context.Background(), otelsarama.NewConsumerMessageCarrier(message))

			tr := otel.Tracer("consumer")
			_, span := tr.Start(ctx, "consume message", trace.WithAttributes(
				semconv.MessagingOperationProcess,
			))
			defer span.End()

			key := string(message.Key)
			val := string(message.Value)
			start := time.Now()
			// workhorse
			for {
				if time.Since(start) > time.Duration(svcTm) {
					break
				}
			}
			log.Printf("Container %s processed \"%s\" for key \"%s\"", containerIP, key, val)
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
