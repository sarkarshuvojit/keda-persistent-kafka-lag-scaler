package metrics

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/lag"
)

var (
	registry = prometheus.NewRegistry()

	partitionLag = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kafka_lag_scaler_partition_lag",
		Help: "Current lag for a single topic partition.",
	}, []string{"topic", "partition"})

	totalLag = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kafka_lag_scaler_total_lag",
		Help: "Sum of lag across all partitions of a topic.",
	}, []string{"topic"})

	consumerCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kafka_lag_scaler_consumer_count",
		Help: "Number of active members in the consumer group.",
	}, []string{"topic", "consumer_group"})
)

func init() {
	registry.MustRegister(partitionLag, totalLag, consumerCount)
}

// UpdateLag records the latest per-partition lag samples and their total.
func UpdateLag(samples []lag.LagSample) {
	if len(samples) == 0 {
		return
	}

	var total int64
	topic := samples[0].Topic
	for _, s := range samples {
		partitionLag.WithLabelValues(s.Topic, strconv.Itoa(s.Partition)).Set(float64(s.Lag))
		total += s.Lag
	}
	totalLag.WithLabelValues(topic).Set(float64(total))
}

// UpdateConsumerCount records the current number of active consumer group members.
func UpdateConsumerCount(topic, consumerGroup string, count int) {
	consumerCount.WithLabelValues(topic, consumerGroup).Set(float64(count))
}

// Handler returns the HTTP handler that serves the /metrics endpoint.
func Handler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}
