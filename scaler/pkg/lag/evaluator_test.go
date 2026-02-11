package lag

import (
	"testing"
	"time"
)

func TestEvaluatePersistence_NoSamples(t *testing.T) {
	result := EvaluatePersistence(nil, 500, 2*time.Minute)
	if result.Persistent {
		t.Error("expected not persistent with no samples")
	}
	if result.TotalCurrentLag != 0 {
		t.Errorf("expected 0 total lag, got %d", result.TotalCurrentLag)
	}
}

func TestEvaluatePersistence_BelowThreshold(t *testing.T) {
	now := time.Now()
	samples := makeSamples(0, now, 10*time.Second, 20, 100) // lag=100, threshold=500

	result := EvaluatePersistence(samples, 500, 2*time.Minute)
	if result.Persistent {
		t.Error("expected not persistent when lag is below threshold")
	}
}

func TestEvaluatePersistence_ShortStretch(t *testing.T) {
	now := time.Now()
	// 1 minute of high lag, but sustain requires 2 minutes
	samples := makeSamples(0, now, 10*time.Second, 6, 1000)

	result := EvaluatePersistence(samples, 500, 2*time.Minute)
	if result.Persistent {
		t.Error("expected not persistent for short stretch")
	}
}

func TestEvaluatePersistence_ExactDuration(t *testing.T) {
	now := time.Now()
	// Exactly 2 minutes of high lag (13 samples at 10s intervals = 120s from first to last)
	samples := makeSamples(0, now, 10*time.Second, 13, 1000)

	result := EvaluatePersistence(samples, 500, 2*time.Minute)
	if !result.Persistent {
		t.Error("expected persistent at exact sustain duration")
	}
}

func TestEvaluatePersistence_LongStretch(t *testing.T) {
	now := time.Now()
	// 5 minutes of high lag
	samples := makeSamples(0, now, 10*time.Second, 30, 1000)

	result := EvaluatePersistence(samples, 500, 2*time.Minute)
	if !result.Persistent {
		t.Error("expected persistent for long stretch")
	}
	if result.TotalCurrentLag != 1000 {
		t.Errorf("expected total lag 1000, got %d", result.TotalCurrentLag)
	}
}

func TestEvaluatePersistence_GapInMiddle(t *testing.T) {
	now := time.Now()
	// 1 minute high, then 1 sample low, then 1 minute high
	// Neither stretch reaches 2 minutes
	var samples []LagSample

	// First stretch: 6 samples, 50 seconds
	for i := 0; i < 6; i++ {
		samples = append(samples, LagSample{
			Timestamp: now.Add(time.Duration(i) * 10 * time.Second),
			Partition: 0,
			Lag:       1000,
		})
	}
	// Gap: 1 sample below threshold
	samples = append(samples, LagSample{
		Timestamp: now.Add(60 * time.Second),
		Partition: 0,
		Lag:       100,
	})
	// Second stretch: 6 samples, 50 seconds
	for i := 0; i < 6; i++ {
		samples = append(samples, LagSample{
			Timestamp: now.Add(time.Duration(70+i*10) * time.Second),
			Partition: 0,
			Lag:       1000,
		})
	}

	result := EvaluatePersistence(samples, 500, 2*time.Minute)
	if result.Persistent {
		t.Error("expected not persistent when gap breaks the stretch")
	}
}

func TestEvaluatePersistence_MultiPartition(t *testing.T) {
	now := time.Now()

	// Partition 0: short stretch (not persistent)
	p0 := makeSamples(0, now, 10*time.Second, 6, 1000)

	// Partition 1: long stretch (persistent)
	p1 := makeSamples(1, now, 10*time.Second, 15, 800)

	all := append(p0, p1...)
	result := EvaluatePersistence(all, 500, 2*time.Minute)
	if !result.Persistent {
		t.Error("expected persistent when at least one partition has persistent lag")
	}
	// Total current lag = latest from p0 (1000) + latest from p1 (800)
	if result.TotalCurrentLag != 1800 {
		t.Errorf("expected total lag 1800, got %d", result.TotalCurrentLag)
	}
}

func TestEvaluatePersistence_TotalLagFromLatest(t *testing.T) {
	now := time.Now()
	samples := []LagSample{
		{Timestamp: now, Partition: 0, Lag: 500},
		{Timestamp: now.Add(10 * time.Second), Partition: 0, Lag: 200},
		{Timestamp: now, Partition: 1, Lag: 300},
		{Timestamp: now.Add(10 * time.Second), Partition: 1, Lag: 400},
	}

	result := EvaluatePersistence(samples, 500, 2*time.Minute)
	// Latest for p0 = 200, latest for p1 = 400
	if result.TotalCurrentLag != 600 {
		t.Errorf("expected total lag 600, got %d", result.TotalCurrentLag)
	}
}

// makeSamples creates n samples for a given partition with constant lag.
func makeSamples(partition int, start time.Time, interval time.Duration, n int, lag int64) []LagSample {
	samples := make([]LagSample, n)
	for i := range samples {
		samples[i] = LagSample{
			Timestamp: start.Add(time.Duration(i) * interval),
			Partition: partition,
			Lag:       lag,
		}
	}
	return samples
}
