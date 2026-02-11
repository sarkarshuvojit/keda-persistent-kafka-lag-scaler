# Persistent Kafka Lag Scaler

A standalone gRPC service that implements [KEDA's External Scaler protocol](https://keda.sh/docs/latest/concepts/external-scalers/) to scale Kafka consumers based on **persistent lag** — lag that exceeds a threshold continuously for a configured duration — rather than reacting to momentary spikes.

## Prerequisites

- Go 1.24.4+
- Docker (or minikube with `eval $(minikube docker-env)`)
- A running Kubernetes cluster with [KEDA](https://keda.sh) installed
- Kafka cluster accessible from the cluster (see `k8s/infra/kafka.yaml`)

## Configuration

| Environment Variable | Metadata Key | Description | Default |
|---|---|---|---|
| `KAFKA_BROKERS` | `bootstrapServers` | Kafka broker addresses | `localhost:9092` |
| `KAFKA_TOPIC` | `topic` | Topic to monitor | *(required)* |
| `KAFKA_GROUP_ID` | `consumerGroup` | Consumer group to track | *(required)* |
| `LAG_THRESHOLD` | `lagThreshold` | Lag count above which a partition is considered "lagging" | `500` |
| `SUSTAIN_SECONDS` | `sustainSeconds` | How long lag must stay above threshold before scaling triggers | `120` |
| `SAMPLING_INTERVAL` | `samplingInterval` | Seconds between each lag poll | `10` |
| `WINDOW_SIZE` | `windowSize` | Number of samples to keep in the sliding window | `30` |
| `GRPC_PORT` | — | Port for the gRPC server | `50051` |

## Step 1: Run Tests

```bash
cd scaler
make test
```

Expected output:

```
ok  github.com/sarkarshuvojit/keda-persistent-kafka-lag-scaler/scaler/pkg/lag  0.5s
```

All 7 evaluator test cases should pass (no samples, below threshold, short stretch, exact duration, long stretch, gap in middle, multi-partition).

## Step 2: Build the Docker Image

If using minikube, first point your Docker CLI to the minikube daemon:

```bash
eval $(minikube docker-env)
```

Then build:

```bash
cd scaler
docker build -t kpkls-scaler:latest .
```

Verify the image exists:

```bash
docker images | grep kpkls-scaler
```

```
kpkls-scaler   latest   abc123def456   5 seconds ago   15MB
```

## Step 3: Deploy the Scaler

Make sure the infrastructure (Kafka) and the consumer app are already running:

```bash
kubectl apply -f k8s/infra/
kubectl apply -f k8s/deploy/deployment.yaml
kubectl apply -f k8s/deploy/producer.yaml
```

Now deploy the scaler:

```bash
kubectl apply -f k8s/deploy/lag-scaler.yaml
```

This creates a Deployment (1 replica) and a ClusterIP Service on port `50051`.

### Verify the scaler is running

```bash
kubectl get pods -l app=lag-scaler
```

```
NAME                          READY   STATUS    RESTARTS   AGE
lag-scaler-5d4f8b7c6f-x9k2p  1/1     Running   0          10s
```

Check the logs to confirm it's polling Kafka:

```bash
kubectl logs -l app=lag-scaler -f
```

You should see output like:

```
Starting persistent Kafka lag scaler
  Brokers:          kafka.default.svc.cluster.local:9092
  Topic:            test-topic
  Consumer Group:   sample-consumer-group
  Lag Threshold:    500
  Sustain Duration: 2m0s
  Sampling Interval:10s
  Window Size:      30
gRPC server listening on :50051
Collected 3 lag samples (window size: 3)
Collected 3 lag samples (window size: 6)
```

### Verify the service is reachable

```bash
kubectl get svc lag-scaler
```

```
NAME         TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)     AGE
lag-scaler   ClusterIP   10.96.xxx.xxx   <none>        50051/TCP   30s
```

## Step 4: Apply the ScaledObject

First, make sure no other ScaledObject is targeting the same consumer deployment. If the basic (threshold-based) scaler is active, remove it first:

```bash
kubectl delete -f k8s/scalers/basic/scaledobject.yaml --ignore-not-found
```

Then apply the persistent scaler:

```bash
kubectl apply -f k8s/scalers/persistent/scaledobject.yaml
```

### Verify KEDA picked it up

```bash
kubectl get scaledobject kafka-consumer-scaler
```

```
NAME                     SCALETARGETREF   MIN   MAX   TRIGGERS   ...   READY   ACTIVE
kafka-consumer-scaler    kafka-consumer   1     5     external   ...   True    False
```

`READY=True` means KEDA is successfully talking to the scaler. `ACTIVE=False` means no persistent lag is detected (expected at rest).

You can also check KEDA's operator logs:

```bash
kubectl logs -l app=keda-operator -n keda --tail=20
```

Look for lines mentioning `kafka-consumer-scaler` without errors.

## Step 5: Test It

### Generate load

Send a burst of messages via the producer:

```bash
kubectl run curl-load --rm -it --restart=Never --image=curlimages/curl -- \
  curl -s -X POST http://kafka-producer/produce \
  -H 'Content-Type: application/json' \
  -d '{"count":5000,"messageSize":64}'
```

### Watch the scaler logs

```bash
kubectl logs -l app=lag-scaler -f
```

You should see:

1. **Lag samples climbing** — `Collected N lag samples` with the window growing
2. **`IsActive: persistent=false`** — lag is above threshold but hasn't persisted long enough yet
3. After ~2 minutes of sustained lag: **`IsActive: persistent=true`** — scaling triggers
4. **`GetMetrics: persistent=true, metricValue=<total_lag>`** — KEDA receives the lag value

### Watch the consumer scale

```bash
kubectl get pods -l app=kafka-consumer -w
```

You should see replicas stay at 1 during the initial burst, and only scale up after the sustain duration (default 2 minutes). Compare this with the threshold-based scaler (`k8s/scalers/basic/scaledobject.yaml`) which would scale up almost immediately.

### After lag clears

Once the consumers drain the backlog, the scaler will report `persistent=false` and `metricValue=0`. After the `cooldownPeriod` (30s), KEDA will scale back down to `minReplicaCount: 1`.

## Running Locally (outside Kubernetes)

For development/debugging, you can run the scaler directly:

```bash
KAFKA_BROKERS="localhost:9092" \
KAFKA_TOPIC="test-topic" \
KAFKA_GROUP_ID="sample-consumer-group" \
LAG_THRESHOLD="500" \
SUSTAIN_SECONDS="120" \
go run main.go
```

The gRPC server will start on `:50051`. You can test it with [grpcurl](https://github.com/fullstorydev/grpcurl):

```bash
grpcurl -plaintext -d '{"name":"test","namespace":"default"}' \
  localhost:50051 externalscaler.ExternalScaler/IsActive
```

```json
{
  "result": false
}
```

## Project Structure

```
scaler/
  main.go                       # Entry point: wires everything, gRPC server, signal handling
  Makefile                      # build, proto-gen, test
  Dockerfile                    # Multi-stage alpine build
  proto/
    externalscaler.proto        # KEDA External Scaler proto (vendored)
  pkg/
    externalscaler/             # Generated protobuf + gRPC Go code
    config/config.go            # ScalerConfig: parse from metadata or env vars
    kafka/client.go             # LagFetcher: per-partition lag via kafka-go Client API
    lag/
      sample.go                 # LagSample type
      window.go                 # SlidingWindow: thread-safe, time-based eviction
      evaluator.go              # EvaluatePersistence: core algorithm
      evaluator_test.go         # Unit tests (7 cases)
    scraper/scraper.go          # Background goroutine: periodic lag collection
    server/server.go            # gRPC ExternalScalerServer (IsActive, StreamIsActive, GetMetricSpec, GetMetrics)
```
