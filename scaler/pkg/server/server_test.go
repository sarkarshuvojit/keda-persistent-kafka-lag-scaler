package server

import (
	"context"
	"testing"
	"time"

	"github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/config"
	pb "github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/externalscaler"
	"github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/lag"
)

func defaultConfig() *config.ScalerConfig {
	return &config.ScalerConfig{
		BootstrapServers: "localhost:9092",
		Topic:            "test-topic",
		ConsumerGroup:    "test-group",
		LagThreshold:     500,
		SustainDuration:  120 * time.Second,
		SamplingInterval: 10 * time.Second,
		WindowSize:       30,
	}
}

func ref() *pb.ScaledObjectRef {
	return &pb.ScaledObjectRef{Name: "test", Namespace: "default"}
}

// simulateScraper mimics what the real scraper does: adds samples to the
// window one tick at a time, just like the production code path.
func simulateScraper(w *lag.SlidingWindow, start time.Time, interval time.Duration, ticks int, partitions int, lagPerPartition int64) {
	for i := range ticks {
		ts := start.Add(time.Duration(i) * interval)
		for p := range partitions {
			w.Add(lag.LagSample{
				Timestamp: ts,
				Partition: p,
				Lag:       lagPerPartition,
				Topic:     "test-topic",
			})
		}
	}
}

func TestIsActive_EmptyWindow(t *testing.T) {
	cfg := defaultConfig()
	w := lag.NewSlidingWindow(cfg.WindowSize, cfg.SamplingInterval)
	srv := New(w, cfg)

	resp, err := srv.IsActive(context.Background(), ref())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result {
		t.Error("expected inactive with empty window")
	}
}

func TestIsActive_LagBelowThreshold(t *testing.T) {
	cfg := defaultConfig()
	w := lag.NewSlidingWindow(cfg.WindowSize, cfg.SamplingInterval)
	srv := New(w, cfg)

	// 3 minutes of lag at 100 (below threshold of 500)
	simulateScraper(w, time.Now().Add(-3*time.Minute), cfg.SamplingInterval, 18, 3, 100)

	resp, err := srv.IsActive(context.Background(), ref())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result {
		t.Error("expected inactive when lag is below threshold")
	}
}

func TestIsActive_HighLagButTooShort(t *testing.T) {
	cfg := defaultConfig()
	w := lag.NewSlidingWindow(cfg.WindowSize, cfg.SamplingInterval)
	srv := New(w, cfg)

	// Only 1 minute of high lag (sustain requires 2 minutes)
	simulateScraper(w, time.Now().Add(-1*time.Minute), cfg.SamplingInterval, 6, 3, 1000)

	resp, err := srv.IsActive(context.Background(), ref())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result {
		t.Error("expected inactive when high lag hasn't persisted long enough")
	}
}

func TestIsActive_PersistentHighLag(t *testing.T) {
	cfg := defaultConfig()
	w := lag.NewSlidingWindow(cfg.WindowSize, cfg.SamplingInterval)
	srv := New(w, cfg)

	// 3 minutes of high lag (sustain requires 2 minutes)
	simulateScraper(w, time.Now().Add(-3*time.Minute), cfg.SamplingInterval, 18, 3, 1000)

	resp, err := srv.IsActive(context.Background(), ref())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Result {
		t.Error("expected ACTIVE when lag has persisted beyond sustain duration")
	}
}

func TestIsActive_LagAtExactThreshold(t *testing.T) {
	cfg := defaultConfig()
	w := lag.NewSlidingWindow(cfg.WindowSize, cfg.SamplingInterval)
	srv := New(w, cfg)

	// 3 minutes of lag at exactly the threshold (500)
	simulateScraper(w, time.Now().Add(-3*time.Minute), cfg.SamplingInterval, 18, 3, 500)

	resp, err := srv.IsActive(context.Background(), ref())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Lag == threshold should count as persistent (>= not >)
	if !resp.Result {
		t.Error("expected ACTIVE when lag equals threshold for sustained period")
	}
}

func TestIsActive_GapBreaksPersistence(t *testing.T) {
	cfg := defaultConfig()
	w := lag.NewSlidingWindow(cfg.WindowSize, cfg.SamplingInterval)
	srv := New(w, cfg)

	now := time.Now()

	// 1 minute of high lag
	simulateScraper(w, now.Add(-3*time.Minute), cfg.SamplingInterval, 6, 3, 1000)

	// Then a dip below threshold at the 70s mark
	for p := range 3 {
		w.Add(lag.LagSample{
			Timestamp: now.Add(-3*time.Minute + 60*time.Second),
			Partition: p, Lag: 100, Topic: "test-topic",
		})
	}

	// Then another minute of high lag
	simulateScraper(w, now.Add(-3*time.Minute+70*time.Second), cfg.SamplingInterval, 6, 3, 1000)

	resp, err := srv.IsActive(context.Background(), ref())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result {
		t.Error("expected inactive when a gap breaks the persistent stretch")
	}
}

func TestGetMetrics_ReturnsLagWhenPersistent(t *testing.T) {
	cfg := defaultConfig()
	w := lag.NewSlidingWindow(cfg.WindowSize, cfg.SamplingInterval)
	srv := New(w, cfg)

	// 3 minutes of lag=1000 across 3 partitions
	simulateScraper(w, time.Now().Add(-3*time.Minute), cfg.SamplingInterval, 18, 3, 1000)

	resp, err := srv.GetMetrics(context.Background(), &pb.GetMetricsRequest{
		ScaledObjectRef: ref(),
		MetricName:      "persistent_kafka_lag",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.MetricValues) != 1 {
		t.Fatalf("expected 1 metric value, got %d", len(resp.MetricValues))
	}

	mv := resp.MetricValues[0]
	if mv.MetricName != "persistent_kafka_lag" {
		t.Errorf("metric name = %s", mv.MetricName)
	}
	// 3 partitions * 1000 lag each = 3000 total
	if mv.MetricValue != 3000 {
		t.Errorf("expected metric value 3000, got %d", mv.MetricValue)
	}
}

func TestGetMetrics_ReturnsZeroWhenNotPersistent(t *testing.T) {
	cfg := defaultConfig()
	w := lag.NewSlidingWindow(cfg.WindowSize, cfg.SamplingInterval)
	srv := New(w, cfg)

	// Short burst — not persistent
	simulateScraper(w, time.Now().Add(-30*time.Second), cfg.SamplingInterval, 3, 3, 1000)

	resp, err := srv.GetMetrics(context.Background(), &pb.GetMetricsRequest{
		ScaledObjectRef: ref(),
		MetricName:      "persistent_kafka_lag",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.MetricValues[0].MetricValue != 0 {
		t.Errorf("expected metric value 0 when not persistent, got %d", resp.MetricValues[0].MetricValue)
	}
}

func TestGetMetricSpec_ReturnsThreshold(t *testing.T) {
	cfg := defaultConfig()
	cfg.LagThreshold = 750
	w := lag.NewSlidingWindow(cfg.WindowSize, cfg.SamplingInterval)
	srv := New(w, cfg)

	resp, err := srv.GetMetricSpec(context.Background(), ref())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.MetricSpecs) != 1 {
		t.Fatalf("expected 1 metric spec, got %d", len(resp.MetricSpecs))
	}
	if resp.MetricSpecs[0].MetricName != "persistent_kafka_lag" {
		t.Errorf("metric name = %s", resp.MetricSpecs[0].MetricName)
	}
	if resp.MetricSpecs[0].TargetSize != 750 {
		t.Errorf("target size = %d, want 750", resp.MetricSpecs[0].TargetSize)
	}
}

func TestIsActive_RealisticScraperSimulation(t *testing.T) {
	// Simulates exactly what happens in production:
	// scraper adds samples every 10s, KEDA polls IsActive periodically
	cfg := defaultConfig()
	w := lag.NewSlidingWindow(cfg.WindowSize, cfg.SamplingInterval)
	srv := New(w, cfg)

	now := time.Now()
	partitions := 3

	// Simulate 3 minutes of scraper ticks with high lag
	for tick := range 19 { // 0s to 180s
		ts := now.Add(-3*time.Minute + time.Duration(tick)*cfg.SamplingInterval)
		for p := range partitions {
			w.Add(lag.LagSample{
				Timestamp: ts,
				Topic:     "test-topic",
				Partition: p,
				Lag:       2000,
				Offset:    int64(tick * 10),
				EndOffset: int64(tick*10 + 2000),
			})
		}
	}

	// Now IsActive should return true
	resp, err := srv.IsActive(context.Background(), ref())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Result {
		snap := w.Snapshot()
		t.Errorf("expected ACTIVE after 3 minutes of high lag; window has %d samples", len(snap))
	}
}
