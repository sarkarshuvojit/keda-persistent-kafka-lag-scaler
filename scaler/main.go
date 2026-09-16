package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	"github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/config"
	pb "github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/externalscaler"
	"github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/kafka"
	"github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/lag"
	"github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/metrics"
	"github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/scraper"
	"github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/server"
)

func main() {
	cfg, err := config.ParseFromEnv()
	if err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}

	log.Printf("Starting persistent Kafka lag scaler")
	log.Printf("  Brokers:          %s", cfg.BootstrapServers)
	log.Printf("  Topic:            %s", cfg.Topic)
	log.Printf("  Consumer Group:   %s", cfg.ConsumerGroup)
	log.Printf("  Lag Threshold:    %d", cfg.LagThreshold)
	log.Printf("  Sustain Duration: %s", cfg.SustainDuration)
	log.Printf("  Sampling Interval:%s", cfg.SamplingInterval)
	log.Printf("  Window Size:      %d", cfg.WindowSize)

	fetcher := kafka.NewLagFetcher(cfg.BootstrapServers, cfg.Topic, cfg.ConsumerGroup)
	window := lag.NewSlidingWindow(cfg.WindowSize, cfg.SamplingInterval)
	scr := scraper.New(fetcher, window, cfg.SamplingInterval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background scraper
	go scr.Run(ctx)

	// Start metrics server
	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "9090"
	}
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", metrics.Handler())
	metricsServer := &http.Server{Addr: ":" + metricsPort, Handler: metricsMux}
	go func() {
		log.Printf("Metrics server listening on :%s", metricsPort)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	// Start gRPC server
	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50051"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterExternalScalerServer(grpcServer, server.New(window, cfg))

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("Received shutdown signal, stopping...")
		cancel()
		grpcServer.GracefulStop()
		metricsServer.Close()
	}()

	log.Printf("gRPC server listening on :%s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
