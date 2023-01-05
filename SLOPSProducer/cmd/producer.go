package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Shopify/sarama"
)

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
	// kafkaProducer, err := sarama.NewAsyncProducer([]string{os.Getenv("KAFKA_BOOTSTRAP")}, config)
	kafkaProducer, err := sarama.NewAsyncProducer(sysDetails.kafkaBrokers, config)
	if err != nil {
		log.Println("Error creating producer:", err.Error())
	}

	return Producer{
		envVar:        envVar,
		sysDetails:    sysDetails,
		kafkaConfig:   config,
		kafkaProducer: kafkaProducer,
	}
}

func (app *Application) Produce(ctx context.Context, key, msg string, partition ...int) {
	var kmsg *sarama.ProducerMessage
	// Add sequencing.
	seq := fmt.Sprintf("%s-%d", app.producer.envVar.containerIP, app.seq.Next())
	hdrs := []sarama.RecordHeader{
		{
			Key:   []byte("Producer"),
			Value: []byte(app.producer.envVar.containerIP),
		},
		{
			Key:   []byte("Sequence"),
			Value: []byte(seq),
		},
	}
	if len(partition) == 0 {
		kmsg = &sarama.ProducerMessage{
			Topic:   app.producer.sysDetails.kafkaTopic,
			Key:     sarama.StringEncoder(key),
			Value:   sarama.StringEncoder(msg),
			Headers: hdrs,
		}
	} else if len(partition) == 1 {
		kmsg = &sarama.ProducerMessage{
			Topic:     app.producer.sysDetails.kafkaTopic,
			Key:       sarama.StringEncoder(key),
			Value:     sarama.StringEncoder(msg),
			Headers:   hdrs,
			Partition: int32(partition[0]),
		}
	} else {
		log.Println("Partition length:", len(partition))
		return
	}

	app.producer.kafkaProducer.Input() <- kmsg
	// Send the trace info.
	go func(seq string) {
		app.SendSequence(seq)
	}(seq)
	log.Println("message sent")
}
