package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"log"
	"os"
	"time"

	"github.com/Shopify/sarama"
	"go.opentelemetry.io/contrib/instrumentation/github.com/Shopify/sarama/otelsarama"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"go.opentelemetry.io/otel/exporters/jaeger"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
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

func (app *Application) getProdConfig() *sarama.Config {
	// sarama logging to stdout.
	sarama.Logger = app.logger

	// producer config
	config := sarama.NewConfig()
	config.Producer.Retry.Max = 5
	config.Producer.RequiredAcks = sarama.WaitForLocal
	config.Producer.Compression = sarama.CompressionSnappy // Compress messages
	config.Producer.Return.Successes = true
	config.Producer.Flush.Frequency = 500 * time.Millisecond // Flush batches every 500ms
	config.ClientID = os.Getenv("ADDRESS")
	// if app.vanilla {
	// 	config.Producer.Partitioner = sarama.NewRandomPartitioner
	// } else {
	// 	config.Producer.Partitioner = sarama.NewManualPartitioner
	// }
	if !app.vanilla {
		config.Producer.Partitioner = sarama.NewManualPartitioner
	}
	return config
}

// SysDetails will hold const values required to run the system
// instead of defining them as constants.
type SysDetails struct {
	kafkaBrokers []string
	kafkaTopic   string
}

func NewSysDetails() SysDetails {
	return SysDetails{
		// kafkaBrokers: []string{"kafka-service:9092"},
		kafkaBrokers: []string{os.Getenv("KAFKA_BOOTSTRAP")},
		kafkaTopic:   "OrderGo",
	}
}

// EnvVar will hold required environmental variables.
type EnvVar struct {
	containerIP string
}

func NewEnvVar() EnvVar {
	containerIP := os.Getenv("ADDRESS")

	return EnvVar{
		containerIP: containerIP,
	}
}

// Producer holds all the information required to run the Kafka producer.
type Producer struct {
	envVar        EnvVar
	sysDetails    SysDetails
	kafkaConfig   *sarama.Config
	kafkaProducer sarama.AsyncProducer
}

func (app *Application) NewProducer() Producer {
	envVar := NewEnvVar()
	sysDetails := NewSysDetails()

	config := app.getProdConfig()

	kafkaProducer, err := sarama.NewAsyncProducer(sysDetails.kafkaBrokers, config)
	propagators := propagation.TraceContext{}
	// Wrap instrumentation
	kafkaProducer = otelsarama.WrapAsyncProducer(
		config,
		kafkaProducer,
		otelsarama.WithTracerProvider(otel.GetTracerProvider()),
		otelsarama.WithPropagators(propagators),
	)
	if err != nil {
		log.Println("Error creating producer:", err.Error())
	}
	log.Println("propogators:", kafkaProducer)

	return Producer{
		envVar:        envVar,
		sysDetails:    sysDetails,
		kafkaConfig:   config,
		kafkaProducer: kafkaProducer,
	}
}

func (app *Application) Produce(key, msg string, partition ...int32) {
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

	var kmsg *sarama.ProducerMessage

	hdrs := []sarama.RecordHeader{
		{
			Key:   []byte("Producer"),
			Value: []byte(app.producer.envVar.containerIP),
		},
	}

	// When Kafka is used.
	if len(partition) == 0 {
		kmsg = &sarama.ProducerMessage{
			Topic:   app.producer.sysDetails.kafkaTopic,
			Key:     sarama.StringEncoder(key),
			Value:   sarama.StringEncoder(msg),
			Headers: hdrs,
		}
	} else if len(partition) == 1 { // When SLOPS is used.
		// Adding message set header from producer.
		msgset, partitionchanged := app.MsgsetHdrVal(key, partition[0])
		var msgsetHdrVal bytes.Buffer
		enc := gob.NewEncoder(&msgsetHdrVal)
		err := enc.Encode(msgset)
		if err != nil {
			log.Println("Encoding err:", err)
			return
		}
		msgsetHdr := sarama.RecordHeader{
			Key:   []byte("SyncEvent"),
			Value: msgsetHdrVal.Bytes(),
		}
		hdrs = append(hdrs, msgsetHdr)

		// Send a message to the older partition that the message set has ended.
		if partitionchanged {
			kmsg = &sarama.ProducerMessage{
				Topic:     app.producer.sysDetails.kafkaTopic,
				Key:       sarama.StringEncoder(key),
				Value:     sarama.StringEncoder(msg),
				Headers:   hdrs,
				Partition: msgset.SrcPartition,
			}
			log.Printf("Key %s switching to %d from %d\n", key, msgset.DestPartition, msgset.SrcPartition)
		} else {
			kmsg = &sarama.ProducerMessage{
				Topic:     app.producer.sysDetails.kafkaTopic,
				Key:       sarama.StringEncoder(key),
				Value:     sarama.StringEncoder(msg),
				Headers:   hdrs,
				Partition: partition[0],
			}
		}
	} else {
		log.Println("Partition length:", len(partition))
		return
	}

	// Create root span
	tr := tp.Tracer("producer")
	ctx, span := tr.Start(context.Background(), "produce message")
	defer span.End()

	propagators := propagation.TraceContext{}
	propagators.Inject(ctx, otelsarama.NewProducerMessageCarrier(kmsg))
	// Add the key as a Jaeger tag.
	span.SetAttributes(attribute.String("producer.key", key))

	app.producer.kafkaProducer.Input() <- kmsg
	log.Println("message sent on partition:", kmsg.Partition)
}

// Create and send message set header
func (app *Application) MsgsetHdrVal(key string, partition int32) (*MessageSet, bool) {
	var msgset *MessageSet
	partitionChanged := false
	lastmsgset, err := app.messageSets.GetKey(key)
	if err != nil {
		// First message of key.
		// Key is not being tracked.
		msgset = &MessageSet{
			Key:             key,
			SrcPartition:    -1,
			SrcMsgsetIndex:  -1,
			DestPartition:   partition,
			DestMsgsetIndex: 0,
		}
	} else {
		if lastmsgset.DestPartition == partition {
			// If we are still sending to the same partition,
			// then no change required.
			msgset = lastmsgset
		} else {
			// Otherwise, we are now sending to a new partition.
			msgset = &MessageSet{
				Key:             key,
				SrcPartition:    lastmsgset.DestPartition,
				SrcMsgsetIndex:  lastmsgset.DestMsgsetIndex,
				DestPartition:   partition,
				DestMsgsetIndex: lastmsgset.DestMsgsetIndex + 1,
			}
			partitionChanged = true
		}
	}

	if partitionChanged {
		go app.messageSets.AddKey(*msgset)
	}

	return msgset, partitionChanged
}
