package lag

import "time"

type LagSample struct {
	Timestamp time.Time
	Topic     string
	Partition int
	Lag       int64
	Offset    int64
	EndOffset int64
}
