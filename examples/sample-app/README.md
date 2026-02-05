# Sample Kafka Consumer

A simple Golang Kafka consumer that simulates slow message processing by taking 10 seconds to process each message before acknowledging it.

## Features

- Consumes messages from a Kafka topic
- Simulates processing with a 10-second delay
- Automatic message acknowledgment
- Graceful shutdown on SIGTERM/SIGINT
- Configurable via environment variables

## Configuration

The consumer can be configured using the following environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `KAFKA_BROKERS` | Comma-separated list of Kafka broker addresses | `localhost:9092` |
| `KAFKA_TOPIC` | Kafka topic to consume from | `test-topic` |
| `KAFKA_GROUP_ID` | Consumer group ID | `sample-consumer-group` |

## Running Locally

### Prerequisites

- Go 1.24.4 or later
- Running Kafka cluster

### Build and Run

```bash
# Install dependencies
go mod download

# Run the consumer
go run main.go

# Or build and run
go build -o consumer main.go
./consumer
```

### With Custom Configuration

```bash
KAFKA_BROKERS="broker1:9092,broker2:9092" \
KAFKA_TOPIC="my-topic" \
KAFKA_GROUP_ID="my-consumer-group" \
./consumer
```

## Running with Docker

### Build the Docker Image

```bash
docker build -t kafka-slow-consumer:latest .
```

### Run the Container

```bash
docker run -it --rm \
  -e KAFKA_BROKERS="kafka:9092" \
  -e KAFKA_TOPIC="test-topic" \
  -e KAFKA_GROUP_ID="sample-consumer-group" \
  kafka-slow-consumer:latest
```

## Behavior

1. The consumer connects to the specified Kafka broker(s)
2. Joins the specified consumer group
3. For each message received:
   - Logs the message details (partition, offset, key, value)
   - Simulates processing by sleeping for 10 seconds
   - Automatically acknowledges the message (commits offset)
4. Continues processing until interrupted

## Graceful Shutdown

The consumer handles SIGTERM and SIGINT signals gracefully:
- On receiving a shutdown signal, it stops consuming new messages
- Allows the current message processing to complete
- Closes the Kafka connection cleanly
