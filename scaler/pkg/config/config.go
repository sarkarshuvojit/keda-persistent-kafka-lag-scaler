package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type ScalerConfig struct {
	BootstrapServers string
	Topic            string
	ConsumerGroup    string
	LagThreshold     int64
	SustainDuration  time.Duration
	SamplingInterval time.Duration
	WindowSize       int
}

func ParseFromMetadata(metadata map[string]string) (*ScalerConfig, error) {
	cfg := &ScalerConfig{
		LagThreshold:     500,
		SustainDuration:  120 * time.Second,
		SamplingInterval: 10 * time.Second,
		WindowSize:       30,
	}

	cfg.BootstrapServers = getMetadataOrEnv(metadata, "bootstrapServers", "KAFKA_BROKERS", "localhost:9092")
	cfg.Topic = getMetadataOrEnv(metadata, "topic", "KAFKA_TOPIC", "")
	cfg.ConsumerGroup = getMetadataOrEnv(metadata, "consumerGroup", "KAFKA_GROUP_ID", "")

	if cfg.Topic == "" {
		return nil, fmt.Errorf("topic is required")
	}
	if cfg.ConsumerGroup == "" {
		return nil, fmt.Errorf("consumerGroup is required")
	}

	if v, ok := metadata["lagThreshold"]; ok {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid lagThreshold: %w", err)
		}
		cfg.LagThreshold = n
	} else if v := os.Getenv("LAG_THRESHOLD"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid LAG_THRESHOLD: %w", err)
		}
		cfg.LagThreshold = n
	}

	if v, ok := metadata["sustainSeconds"]; ok {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid sustainSeconds: %w", err)
		}
		cfg.SustainDuration = time.Duration(n) * time.Second
	} else if v := os.Getenv("SUSTAIN_SECONDS"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid SUSTAIN_SECONDS: %w", err)
		}
		cfg.SustainDuration = time.Duration(n) * time.Second
	}

	if v, ok := metadata["samplingInterval"]; ok {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid samplingInterval: %w", err)
		}
		cfg.SamplingInterval = time.Duration(n) * time.Second
	} else if v := os.Getenv("SAMPLING_INTERVAL"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid SAMPLING_INTERVAL: %w", err)
		}
		cfg.SamplingInterval = time.Duration(n) * time.Second
	}

	if v, ok := metadata["windowSize"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid windowSize: %w", err)
		}
		cfg.WindowSize = n
	} else if v := os.Getenv("WINDOW_SIZE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid WINDOW_SIZE: %w", err)
		}
		cfg.WindowSize = n
	}

	return cfg, nil
}

func ParseFromEnv() (*ScalerConfig, error) {
	return ParseFromMetadata(nil)
}

func getMetadataOrEnv(metadata map[string]string, key, envKey, defaultVal string) string {
	if metadata != nil {
		if v, ok := metadata[key]; ok && v != "" {
			return v
		}
	}
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return defaultVal
}
