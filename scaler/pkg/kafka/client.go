package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/lag"
)

type LagFetcher struct {
	client        *kafka.Client
	topic         string
	consumerGroup string
}

func NewLagFetcher(brokers string, topic, consumerGroup string) *LagFetcher {
	return &LagFetcher{
		client: &kafka.Client{
			Addr: kafka.TCP(brokers),
		},
		topic:         topic,
		consumerGroup: consumerGroup,
	}
}

// Topic returns the topic being monitored.
func (f *LagFetcher) Topic() string {
	return f.topic
}

// ConsumerGroup returns the consumer group being monitored.
func (f *LagFetcher) ConsumerGroup() string {
	return f.consumerGroup
}

func (f *LagFetcher) FetchLag(ctx context.Context) ([]lag.LagSample, error) {
	now := time.Now()

	// Discover partitions via Metadata
	metaResp, err := f.client.Metadata(ctx, &kafka.MetadataRequest{
		Addr:   f.client.Addr,
		Topics: []string{f.topic},
	})
	if err != nil {
		return nil, fmt.Errorf("metadata request failed: %w", err)
	}

	if len(metaResp.Topics) == 0 {
		return nil, fmt.Errorf("topic %s not found", f.topic)
	}

	topicMeta := metaResp.Topics[0]
	if topicMeta.Error != nil {
		return nil, fmt.Errorf("topic metadata error: %w", topicMeta.Error)
	}

	partitions := topicMeta.Partitions

	// Get high water marks (latest offsets)
	offsetRequests := make(map[string][]kafka.OffsetRequest)
	for _, p := range partitions {
		offsetRequests[f.topic] = append(offsetRequests[f.topic], kafka.OffsetRequest{
			Partition: p.ID,
			Timestamp: -1, // latest offset
		})
	}

	listResp, err := f.client.ListOffsets(ctx, &kafka.ListOffsetsRequest{
		Addr:   f.client.Addr,
		Topics: offsetRequests,
	})
	if err != nil {
		return nil, fmt.Errorf("list offsets failed: %w", err)
	}

	endOffsets := make(map[int]int64)
	for _, po := range listResp.Topics[f.topic] {
		if po.Error != nil {
			return nil, fmt.Errorf("offset error for partition %d: %w", po.Partition, po.Error)
		}
		endOffsets[po.Partition] = po.LastOffset
	}

	// Get committed consumer group offsets
	coordinatorResp, err := f.client.FindCoordinator(ctx, &kafka.FindCoordinatorRequest{
		Addr:    f.client.Addr,
		Key:     f.consumerGroup,
		KeyType: kafka.CoordinatorKeyTypeConsumer,
	})
	if err != nil {
		return nil, fmt.Errorf("coordinator lookup failed: %w", err)
	}
	if coordinatorResp.Error != nil {
		return nil, fmt.Errorf("coordinator lookup failed: %w", coordinatorResp.Error)
	}

	coordinatorAddr := kafka.TCP(fmt.Sprintf("%s:%d", coordinatorResp.Coordinator.Host, coordinatorResp.Coordinator.Port))

	topicPartitions := make(map[string][]int)
	for _, p := range partitions {
		topicPartitions[f.topic] = append(topicPartitions[f.topic], p.ID)
	}

	fetchResp, err := f.client.OffsetFetch(ctx, &kafka.OffsetFetchRequest{
		Addr:    coordinatorAddr,
		GroupID: f.consumerGroup,
		Topics:  topicPartitions,
	})
	if err != nil {
		return nil, fmt.Errorf("offset fetch failed: %w", err)
	}

	committedOffsets := make(map[int]int64)
	for _, po := range fetchResp.Topics[f.topic] {
		if po.Error != nil {
			return nil, fmt.Errorf("committed offset error for partition %d: %w", po.Partition, po.Error)
		}
		committed := po.CommittedOffset
		if committed < 0 {
			committed = 0
		}
		committedOffsets[po.Partition] = committed
	}

	// Calculate lag per partition
	var samples []lag.LagSample
	for _, p := range partitions {
		endOffset := endOffsets[p.ID]
		committed := committedOffsets[p.ID]
		lagValue := endOffset - committed
		if lagValue < 0 {
			lagValue = 0
		}

		samples = append(samples, lag.LagSample{
			Timestamp: now,
			Topic:     f.topic,
			Partition: p.ID,
			Lag:       lagValue,
			Offset:    committed,
			EndOffset: endOffset,
		})
	}

	return samples, nil
}

// FetchConsumerGroupSize returns the number of active members in the consumer group.
func (f *LagFetcher) FetchConsumerGroupSize(ctx context.Context) (int, error) {
	coordinatorResp, err := f.client.FindCoordinator(ctx, &kafka.FindCoordinatorRequest{
		Addr:    f.client.Addr,
		Key:     f.consumerGroup,
		KeyType: kafka.CoordinatorKeyTypeConsumer,
	})
	if err != nil {
		return 0, fmt.Errorf("coordinator lookup failed: %w", err)
	}
	if coordinatorResp.Error != nil {
		return 0, fmt.Errorf("coordinator lookup failed: %w", coordinatorResp.Error)
	}

	coordinatorAddr := kafka.TCP(fmt.Sprintf("%s:%d", coordinatorResp.Coordinator.Host, coordinatorResp.Coordinator.Port))

	resp, err := f.client.DescribeGroups(ctx, &kafka.DescribeGroupsRequest{
		Addr:     coordinatorAddr,
		GroupIDs: []string{f.consumerGroup},
	})
	if err != nil {
		return 0, fmt.Errorf("describe groups failed: %w", err)
	}

	if len(resp.Groups) == 0 {
		return 0, nil
	}

	group := resp.Groups[0]
	if group.Error != nil {
		return 0, fmt.Errorf("describe group error: %w", group.Error)
	}

	return len(group.Members), nil
}
