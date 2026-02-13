package lag

import (
	"sync"
	"testing"
	"time"
)

func TestSlidingWindow_AddAndSnapshot(t *testing.T) {
	w := NewSlidingWindow(10, time.Second) // 10s window

	now := time.Now()
	w.Add(LagSample{Timestamp: now, Partition: 0, Lag: 100})
	w.Add(LagSample{Timestamp: now, Partition: 1, Lag: 200})

	snap := w.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(snap))
	}
	if snap[0].Lag != 100 || snap[1].Lag != 200 {
		t.Errorf("unexpected sample values: %+v", snap)
	}
}

func TestSlidingWindow_AddBatch(t *testing.T) {
	w := NewSlidingWindow(10, time.Second)

	now := time.Now()
	w.Add(
		LagSample{Timestamp: now, Partition: 0, Lag: 10},
		LagSample{Timestamp: now, Partition: 1, Lag: 20},
		LagSample{Timestamp: now, Partition: 2, Lag: 30},
	)

	if w.Len() != 3 {
		t.Fatalf("expected 3 samples, got %d", w.Len())
	}
}

func TestSlidingWindow_EvictsOldSamples(t *testing.T) {
	w := NewSlidingWindow(5, time.Second) // 5s window

	now := time.Now()
	// Add samples that are already older than the window
	w.Add(
		LagSample{Timestamp: now.Add(-10 * time.Second), Partition: 0, Lag: 100},
		LagSample{Timestamp: now.Add(-8 * time.Second), Partition: 0, Lag: 200},
	)
	// These should have been evicted on Add
	if w.Len() != 0 {
		t.Fatalf("expected 0 samples after eviction, got %d", w.Len())
	}

	// Add a mix of old and fresh samples
	w.Add(
		LagSample{Timestamp: now.Add(-10 * time.Second), Partition: 0, Lag: 100},
		LagSample{Timestamp: now.Add(-3 * time.Second), Partition: 0, Lag: 200},
		LagSample{Timestamp: now, Partition: 0, Lag: 300},
	)
	// Only the recent two should survive
	snap := w.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(snap))
	}
	if snap[0].Lag != 200 || snap[1].Lag != 300 {
		t.Errorf("unexpected surviving samples: %+v", snap)
	}
}

func TestSlidingWindow_KeepsFreshSamples(t *testing.T) {
	w := NewSlidingWindow(30, time.Second) // 30s window

	now := time.Now()
	for i := range 10 {
		w.Add(LagSample{
			Timestamp: now.Add(time.Duration(-9+i) * time.Second), // -9s to now
			Partition: 0,
			Lag:       int64(i * 100),
		})
	}

	if w.Len() != 10 {
		t.Fatalf("expected all 10 samples retained, got %d", w.Len())
	}
}

func TestSlidingWindow_SnapshotIsACopy(t *testing.T) {
	w := NewSlidingWindow(10, time.Second)

	now := time.Now()
	w.Add(LagSample{Timestamp: now, Partition: 0, Lag: 100})

	snap := w.Snapshot()
	snap[0].Lag = 999

	// Verify the window's internal data wasn't mutated
	snap2 := w.Snapshot()
	if snap2[0].Lag != 100 {
		t.Error("Snapshot should return a copy; modifying it should not affect the window")
	}
}

func TestSlidingWindow_LenMatchesSnapshot(t *testing.T) {
	w := NewSlidingWindow(10, time.Second)

	if w.Len() != 0 {
		t.Fatal("expected 0 for empty window")
	}

	now := time.Now()
	w.Add(
		LagSample{Timestamp: now, Partition: 0, Lag: 1},
		LagSample{Timestamp: now, Partition: 1, Lag: 2},
	)

	if w.Len() != len(w.Snapshot()) {
		t.Errorf("Len() = %d but Snapshot has %d elements", w.Len(), len(w.Snapshot()))
	}
}

func TestSlidingWindow_ConcurrentAccess(t *testing.T) {
	w := NewSlidingWindow(60, time.Second) // large window so nothing is evicted

	now := time.Now()
	var wg sync.WaitGroup

	// 10 goroutines writing concurrently
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range 100 {
				w.Add(LagSample{
					Timestamp: now.Add(time.Duration(j) * time.Millisecond),
					Partition: id,
					Lag:       int64(j),
				})
			}
		}(i)
	}

	// 5 goroutines reading concurrently
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				_ = w.Snapshot()
				_ = w.Len()
			}
		}()
	}

	wg.Wait()

	// All 1000 samples should be present (10 goroutines * 100 each)
	if w.Len() != 1000 {
		t.Errorf("expected 1000 samples, got %d", w.Len())
	}
}
