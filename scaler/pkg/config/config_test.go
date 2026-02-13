package config

import (
	"testing"
	"time"
)

func TestParseFromMetadata_Defaults(t *testing.T) {
	meta := map[string]string{
		"topic":         "my-topic",
		"consumerGroup": "my-group",
	}

	cfg, err := ParseFromMetadata(meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.LagThreshold != 500 {
		t.Errorf("expected default lagThreshold 500, got %d", cfg.LagThreshold)
	}
	if cfg.SustainDuration != 120*time.Second {
		t.Errorf("expected default sustainDuration 120s, got %s", cfg.SustainDuration)
	}
	if cfg.SamplingInterval != 10*time.Second {
		t.Errorf("expected default samplingInterval 10s, got %s", cfg.SamplingInterval)
	}
	if cfg.WindowSize != 30 {
		t.Errorf("expected default windowSize 30, got %d", cfg.WindowSize)
	}
	if cfg.BootstrapServers != "localhost:9092" {
		t.Errorf("expected default bootstrapServers, got %s", cfg.BootstrapServers)
	}
}

func TestParseFromMetadata_AllFields(t *testing.T) {
	meta := map[string]string{
		"bootstrapServers": "broker1:9092,broker2:9092",
		"topic":            "orders",
		"consumerGroup":    "order-processor",
		"lagThreshold":     "1000",
		"sustainSeconds":   "60",
		"samplingInterval": "5",
		"windowSize":       "50",
	}

	cfg, err := ParseFromMetadata(meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.BootstrapServers != "broker1:9092,broker2:9092" {
		t.Errorf("bootstrapServers = %s", cfg.BootstrapServers)
	}
	if cfg.Topic != "orders" {
		t.Errorf("topic = %s", cfg.Topic)
	}
	if cfg.ConsumerGroup != "order-processor" {
		t.Errorf("consumerGroup = %s", cfg.ConsumerGroup)
	}
	if cfg.LagThreshold != 1000 {
		t.Errorf("lagThreshold = %d", cfg.LagThreshold)
	}
	if cfg.SustainDuration != 60*time.Second {
		t.Errorf("sustainDuration = %s", cfg.SustainDuration)
	}
	if cfg.SamplingInterval != 5*time.Second {
		t.Errorf("samplingInterval = %s", cfg.SamplingInterval)
	}
	if cfg.WindowSize != 50 {
		t.Errorf("windowSize = %d", cfg.WindowSize)
	}
}

func TestParseFromMetadata_MissingTopic(t *testing.T) {
	meta := map[string]string{
		"consumerGroup": "my-group",
	}

	_, err := ParseFromMetadata(meta)
	if err == nil {
		t.Fatal("expected error for missing topic")
	}
	if err.Error() != "topic is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseFromMetadata_MissingConsumerGroup(t *testing.T) {
	meta := map[string]string{
		"topic": "my-topic",
	}

	_, err := ParseFromMetadata(meta)
	if err == nil {
		t.Fatal("expected error for missing consumerGroup")
	}
	if err.Error() != "consumerGroup is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseFromMetadata_InvalidLagThreshold(t *testing.T) {
	meta := map[string]string{
		"topic":         "my-topic",
		"consumerGroup": "my-group",
		"lagThreshold":  "not-a-number",
	}

	_, err := ParseFromMetadata(meta)
	if err == nil {
		t.Fatal("expected error for invalid lagThreshold")
	}
}

func TestParseFromMetadata_InvalidSustainSeconds(t *testing.T) {
	meta := map[string]string{
		"topic":          "my-topic",
		"consumerGroup":  "my-group",
		"sustainSeconds": "abc",
	}

	_, err := ParseFromMetadata(meta)
	if err == nil {
		t.Fatal("expected error for invalid sustainSeconds")
	}
}

func TestParseFromMetadata_InvalidSamplingInterval(t *testing.T) {
	meta := map[string]string{
		"topic":            "my-topic",
		"consumerGroup":    "my-group",
		"samplingInterval": "xyz",
	}

	_, err := ParseFromMetadata(meta)
	if err == nil {
		t.Fatal("expected error for invalid samplingInterval")
	}
}

func TestParseFromMetadata_InvalidWindowSize(t *testing.T) {
	meta := map[string]string{
		"topic":         "my-topic",
		"consumerGroup": "my-group",
		"windowSize":    "big",
	}

	_, err := ParseFromMetadata(meta)
	if err == nil {
		t.Fatal("expected error for invalid windowSize")
	}
}

func TestParseFromMetadata_EnvVarFallback(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "env-broker:9092")
	t.Setenv("KAFKA_TOPIC", "env-topic")
	t.Setenv("KAFKA_GROUP_ID", "env-group")
	t.Setenv("LAG_THRESHOLD", "750")
	t.Setenv("SUSTAIN_SECONDS", "90")
	t.Setenv("SAMPLING_INTERVAL", "15")
	t.Setenv("WINDOW_SIZE", "20")

	// Pass nil metadata — everything comes from env
	cfg, err := ParseFromMetadata(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.BootstrapServers != "env-broker:9092" {
		t.Errorf("bootstrapServers = %s, want env-broker:9092", cfg.BootstrapServers)
	}
	if cfg.Topic != "env-topic" {
		t.Errorf("topic = %s", cfg.Topic)
	}
	if cfg.ConsumerGroup != "env-group" {
		t.Errorf("consumerGroup = %s", cfg.ConsumerGroup)
	}
	if cfg.LagThreshold != 750 {
		t.Errorf("lagThreshold = %d, want 750", cfg.LagThreshold)
	}
	if cfg.SustainDuration != 90*time.Second {
		t.Errorf("sustainDuration = %s, want 1m30s", cfg.SustainDuration)
	}
	if cfg.SamplingInterval != 15*time.Second {
		t.Errorf("samplingInterval = %s, want 15s", cfg.SamplingInterval)
	}
	if cfg.WindowSize != 20 {
		t.Errorf("windowSize = %d, want 20", cfg.WindowSize)
	}
}

func TestParseFromMetadata_MetadataOverridesEnv(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "env-broker:9092")
	t.Setenv("LAG_THRESHOLD", "999")

	meta := map[string]string{
		"bootstrapServers": "meta-broker:9092",
		"topic":            "my-topic",
		"consumerGroup":    "my-group",
		"lagThreshold":     "100",
	}

	cfg, err := ParseFromMetadata(meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.BootstrapServers != "meta-broker:9092" {
		t.Errorf("expected metadata to override env, got %s", cfg.BootstrapServers)
	}
	if cfg.LagThreshold != 100 {
		t.Errorf("expected metadata lagThreshold 100, got %d", cfg.LagThreshold)
	}
}

func TestParseFromMetadata_InvalidEnvVars(t *testing.T) {
	t.Setenv("KAFKA_TOPIC", "env-topic")
	t.Setenv("KAFKA_GROUP_ID", "env-group")
	t.Setenv("LAG_THRESHOLD", "not-a-number")

	_, err := ParseFromMetadata(nil)
	if err == nil {
		t.Fatal("expected error for invalid LAG_THRESHOLD env var")
	}
}
