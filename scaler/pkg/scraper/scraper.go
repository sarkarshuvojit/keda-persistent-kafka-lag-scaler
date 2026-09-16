package scraper

import (
	"context"
	"log"
	"time"

	"github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/kafka"
	"github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/lag"
	"github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/metrics"
)

type MetricsScraper struct {
	fetcher  *kafka.LagFetcher
	window   *lag.SlidingWindow
	interval time.Duration
}

func New(fetcher *kafka.LagFetcher, window *lag.SlidingWindow, interval time.Duration) *MetricsScraper {
	return &MetricsScraper{
		fetcher:  fetcher,
		window:   window,
		interval: interval,
	}
}

func (s *MetricsScraper) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Fetch immediately on start
	s.fetch(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("Metrics scraper stopped")
			return
		case <-ticker.C:
			s.fetch(ctx)
		}
	}
}

func (s *MetricsScraper) fetch(ctx context.Context) {
	samples, err := s.fetcher.FetchLag(ctx)
	if err != nil {
		log.Printf("Error fetching lag: %v", err)
		return
	}

	s.window.Add(samples...)
	metrics.UpdateLag(samples)
	log.Printf("Collected %d lag samples (window size: %d)", len(samples), s.window.Len())

	count, err := s.fetcher.FetchConsumerGroupSize(ctx)
	if err != nil {
		log.Printf("Error fetching consumer group size: %v", err)
		return
	}
	metrics.UpdateConsumerCount(s.fetcher.Topic(), s.fetcher.ConsumerGroup(), count)
}
