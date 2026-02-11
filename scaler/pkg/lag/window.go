package lag

import (
	"sync"
	"time"
)

type SlidingWindow struct {
	mu             sync.RWMutex
	samples        []LagSample
	windowDuration time.Duration
}

func NewSlidingWindow(windowSize int, samplingInterval time.Duration) *SlidingWindow {
	return &SlidingWindow{
		windowDuration: time.Duration(windowSize) * samplingInterval,
	}
}

func (w *SlidingWindow) Add(samples ...LagSample) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.samples = append(w.samples, samples...)
	w.evict()
}

func (w *SlidingWindow) Snapshot() []LagSample {
	w.mu.RLock()
	defer w.mu.RUnlock()

	out := make([]LagSample, len(w.samples))
	copy(out, w.samples)
	return out
}

func (w *SlidingWindow) Len() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.samples)
}

func (w *SlidingWindow) evict() {
	cutoff := time.Now().Add(-w.windowDuration)
	i := 0
	for i < len(w.samples) && w.samples[i].Timestamp.Before(cutoff) {
		i++
	}
	if i > 0 {
		w.samples = w.samples[i:]
	}
}
