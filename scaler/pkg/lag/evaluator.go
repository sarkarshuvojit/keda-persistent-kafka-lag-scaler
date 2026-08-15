package lag

import (
	"sort"
	"time"
)

type EvaluationResult struct {
	Persistent      bool
	TotalCurrentLag int64
}

// EvaluatePersistence checks whether the total lag across all partitions has
// exceeded the threshold continuously for at least sustainDuration. Samples
// are grouped by fetch timestamp (all partitions are scraped together in one
// batch) and summed per batch before checking for a persistent stretch.
func EvaluatePersistence(samples []LagSample, threshold int64, sustainDuration time.Duration) EvaluationResult {
	if len(samples) == 0 {
		return EvaluationResult{}
	}

	// Group samples by fetch timestamp and sum lag across partitions per batch
	totalByTimestamp := make(map[time.Time]int64)
	for _, s := range samples {
		totalByTimestamp[s.Timestamp] += s.Lag
	}

	batches := make([]LagSample, 0, len(totalByTimestamp))
	for ts, total := range totalByTimestamp {
		batches = append(batches, LagSample{Timestamp: ts, Lag: total})
	}
	sort.Slice(batches, func(i, j int) bool {
		return batches[i].Timestamp.Before(batches[j].Timestamp)
	})

	var totalCurrentLag int64
	if len(batches) > 0 {
		totalCurrentLag = batches[len(batches)-1].Lag
	}

	return EvaluationResult{
		Persistent:      hasPersistentLag(batches, threshold, sustainDuration),
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
