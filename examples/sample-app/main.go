package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
)

func main() {
	mode := flag.String("mode", "consumer", "run mode: 'consumer' or 'producer'")
	flag.Parse()

	switch *mode {
	case "consumer":
		runConsumer()
	case "producer":
		runProducer()
	default:
		log.Fatalf("unknown mode: %s (use 'consumer' or 'producer')", *mode)
	}
}

func runConsumer() {
	brokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	topic := getEnv("KAFKA_TOPIC", "test-topic")
	groupID := getEnv("KAFKA_GROUP_ID", "sample-consumer-group")

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        strings.Split(brokers, ","),
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       10e3,
		MaxBytes:       10e6,
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, stopping consumer...")
		cancel()
	}()

	messageCount := 0
	for {
		select {
		case <-ctx.Done():
			log.Println("Consumer stopped")
			return
		default:
				msg, err := reader.ReadMessage(ctx)

			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("Error reading message: %v", err)
				time.Sleep(time.Second)
				continue
			}

			messageCount++
			/*
			log.Printf("[Message %d] Received from partition %d at offset %d",
				messageCount, msg.Partition, msg.Offset)
			log.Printf("[Message %d] Key: %s, Value: %s",
				messageCount, string(msg.Key), string(msg.Value))
			*/

			//log.Printf("[Message %d] Processing message (will take 100 mseconds)...", messageCount)
			time.Sleep(100 * time.Millisecond)

			log.Printf("[Message %d] Processing complete, message acknowledged (latency: %d)", messageCount, time.Since(msg.Time).Milliseconds())
		}
	}
}

func runProducer() {
	brokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	topic := getEnv("KAFKA_TOPIC", "test-topic")
	port := getEnv("PORT", "8080")

	writer := &kafka.Writer{
		Addr:         kafka.TCP(strings.Split(brokers, ",")...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
	}

	defer func() {
		if err := writer.Close(); err != nil {
			log.Printf("Failed to close writer: %v", err)
		}
	}()

	log.Printf("Starting producer API...")
	log.Printf("Brokers: %s", brokers)
	log.Printf("Topic: %s", topic)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /produce", produceHandler(writer))

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down producer API...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	log.Printf("Listening on :%s", port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}
}

type produceRequest struct {
	Count       int    `json:"count"`
	MessageSize int    `json:"messageSize"`
	Key         string `json:"key"`
}

type produceResponse struct {
	Produced int    `json:"produced"`
	Message  string `json:"message"`
}

func produceHandler(writer *kafka.Writer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req produceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
			return
		}

		if req.Count <= 0 {
			req.Count = 1
		}
		if req.MessageSize <= 0 {
			req.MessageSize = 64
		}

		payload := strings.Repeat("x", req.MessageSize)

		messages := make([]kafka.Message, req.Count)
		for i := range messages {
			key := req.Key
			if key == "" {
				key = fmt.Sprintf("load-%d", i)
			}
			messages[i] = kafka.Message{
				Key:   []byte(key),
				Value: []byte(fmt.Sprintf(`{"seq":%d,"payload":"%s"}`, i, payload)),
			}
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		if err := writer.WriteMessages(ctx, messages...); err != nil {
			log.Printf("Failed to produce messages: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		log.Printf("Produced %d messages (size=%d each)", req.Count, req.MessageSize)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(produceResponse{
			Produced: req.Count,
			Message:  fmt.Sprintf("produced %d messages", req.Count),
		})
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
