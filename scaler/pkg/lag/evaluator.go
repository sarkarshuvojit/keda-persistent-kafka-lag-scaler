package lag

import (
	"sort"
	"time"
)

type EvaluationResult struct {
	Persistent      bool
	TotalCurrentLag int64
}

// EvaluatePersistence checks whether lag has exceeded the threshold continuously
// for at least sustainDuration on any partition. It groups samples by partition
// and finds the longest continuous stretch where ALL samples have Lag > threshold.
func EvaluatePersistence(samples []LagSample, threshold int64, sustainDuration time.Duration) EvaluationResult {
	if len(samples) == 0 {
		return EvaluationResult{}
	}

	// Group samples by partition
	byPartition := make(map[int][]LagSample)
	latestByPartition := make(map[int]LagSample)

	for _, s := range samples {
		byPartition[s.Partition] = append(byPartition[s.Partition], s)
		if existing, ok := latestByPartition[s.Partition]; !ok || s.Timestamp.After(existing.Timestamp) {
			latestByPartition[s.Partition] = s
		}
	}

	// Compute total current lag from latest sample per partition
	var totalCurrentLag int64
	for _, s := range latestByPartition {
		totalCurrentLag += s.Lag
	}

	// Check each partition for persistent lag
	persistent := false
	for _, partSamples := range byPartition {
		// Sort by timestamp
		sort.Slice(partSamples, func(i, j int) bool {
			return partSamples[i].Timestamp.Before(partSamples[j].Timestamp)
		})

		if hasPersistentLag(partSamples, threshold, sustainDuration) {
			persistent = true
			break
		}
	}

	return EvaluationResult{
		Persistent:      persistent,
		TotalCurrentLag: totalCurrentLag,
	}
}

// hasPersistentLag checks if there's a continuous stretch of samples above
// the threshold that spans at least sustainDuration.
func hasPersistentLag(samples []LagSample, threshold int64, sustainDuration time.Duration) bool {
	if len(samples) == 0 {
		return false
	}

	var stretchStart time.Time
	inStretch := false

	for _, s := range samples {
		if s.Lag >= threshold {
			if !inStretch {
				stretchStart = s.Timestamp
				inStretch = true
			}
			if s.Timestamp.Sub(stretchStart) >= sustainDuration {
				return true
			}
		} else {
			inStretch = false
		}
	}

	return false
}
