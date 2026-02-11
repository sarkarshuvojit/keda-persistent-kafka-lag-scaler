package server

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/config"
	pb "github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/externalscaler"
	"github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/lag"
)

type ExternalScalerServer struct {
	pb.UnimplementedExternalScalerServer
	window *lag.SlidingWindow
	config *config.ScalerConfig
}

func New(window *lag.SlidingWindow, cfg *config.ScalerConfig) *ExternalScalerServer {
	return &ExternalScalerServer{
		window: window,
		config: cfg,
	}
}

func (s *ExternalScalerServer) IsActive(ctx context.Context, ref *pb.ScaledObjectRef) (*pb.IsActiveResponse, error) {
	result := s.evaluate()
	log.Printf("IsActive: persistent=%v, totalLag=%d", result.Persistent, result.TotalCurrentLag)
	return &pb.IsActiveResponse{
		Result: result.Persistent,
	}, nil
}

func (s *ExternalScalerServer) StreamIsActive(ref *pb.ScaledObjectRef, stream pb.ExternalScaler_StreamIsActiveServer) error {
	ticker := time.NewTicker(s.config.SamplingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
			result := s.evaluate()
			log.Printf("StreamIsActive: persistent=%v, totalLag=%d", result.Persistent, result.TotalCurrentLag)
			err := stream.Send(&pb.IsActiveResponse{
				Result: result.Persistent,
			})
			if err != nil {
				return fmt.Errorf("error sending stream: %w", err)
			}
		}
	}
}

func (s *ExternalScalerServer) GetMetricSpec(ctx context.Context, ref *pb.ScaledObjectRef) (*pb.GetMetricSpecResponse, error) {
	return &pb.GetMetricSpecResponse{
		MetricSpecs: []*pb.MetricSpec{
			{
				MetricName: "persistent_kafka_lag",
				TargetSize: s.config.LagThreshold,
			},
		},
	}, nil
}

func (s *ExternalScalerServer) GetMetrics(ctx context.Context, req *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error) {
	result := s.evaluate()

	var metricValue int64
	if result.Persistent {
		metricValue = result.TotalCurrentLag
	}

	log.Printf("GetMetrics: persistent=%v, metricValue=%d", result.Persistent, metricValue)
	return &pb.GetMetricsResponse{
		MetricValues: []*pb.MetricValue{
			{
				MetricName:  "persistent_kafka_lag",
				MetricValue: metricValue,
			},
		},
	}, nil
}

func (s *ExternalScalerServer) evaluate() lag.EvaluationResult {
	samples := s.window.Snapshot()
	return lag.EvaluatePersistence(samples, s.config.LagThreshold, s.config.SustainDuration)
}
