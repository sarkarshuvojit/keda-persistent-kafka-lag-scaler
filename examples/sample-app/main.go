package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
)

func main() {
	// Configuration from environment variables with defaults
	brokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	topic := getEnv("KAFKA_TOPIC", "test-topic")
	groupID := getEnv("KAFKA_GROUP_ID", "sample-consumer-group")

	// Create a new Kafka reader (consumer)
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{brokers},
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset,
	})

	defer func() {
		if err := reader.Close(); err != nil {
			log.Printf("Failed to close reader: %v", err)
		}
	}()

	log.Printf("Starting Kafka consumer...")
	log.Printf("Brokers: %s", brokers)
	log.Printf("Topic: %s", topic)
	log.Printf("Group ID: %s", groupID)

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, stopping consumer...")
		cancel()
	}()

	// Start consuming messages
	messageCount := 0
	for {
		select {
		case <-ctx.Done():
			log.Println("Consumer stopped")
			return
		default:
			// Read message with timeout
			readCtx, readCancel := context.WithTimeout(ctx, 10*time.Second)
			msg, err := reader.ReadMessage(readCtx)
			readCancel()

			if err != nil {
				if ctx.Err() != nil {
					// Context was cancelled, exit gracefully
					return
				}
				log.Printf("Error reading message: %v", err)
				time.Sleep(time.Second)
				continue
			}

			messageCount++
			log.Printf("[Message %d] Received message from partition %d at offset %d",
				messageCount, msg.Partition, msg.Offset)
			log.Printf("[Message %d] Key: %s, Value: %s",
				messageCount, string(msg.Key), string(msg.Value))

			// Simulate processing time (10 seconds)
			log.Printf("[Message %d] Processing message (will take 10 seconds)...", messageCount)
			time.Sleep(10 * time.Second)

			log.Printf("[Message %d] Processing complete, message acknowledged", messageCount)
			// Note: With the kafka-go library and CommitInterval set,
			// messages are automatically committed periodically.
			// The ReadMessage call marks the message as "read" and it will be committed
			// based on the CommitInterval.
		}
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
