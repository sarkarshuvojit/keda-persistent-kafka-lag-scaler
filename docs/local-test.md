# Local Test: Persistent Lag Scaling in Action

This documents the local run behind [`docs/test-results-annotated.png`](./test-results-annotated.png), showing the scaler ignoring a transient spike and reacting to a sustained one.

## Setup

Deployed via the manifests under [`examples/k8s/`](../examples/k8s/): Kafka broker, a single consumer, the lag-scaler service, and a `ScaledObject` wired to it (see [`examples/k8s/scalers/persistent/scaledobject.yaml`](../examples/k8s/scalers/persistent/scaledobject.yaml)).

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: kafka-consumer-scaler
spec:
  scaleTargetRef:
    name: kafka-consumer
  pollingInterval: 10
  cooldownPeriod: 30
  minReplicaCount: 1
  maxReplicaCount: 10
  triggers:
    - type: external
      metadata:
        scalerAddress: lag-scaler.default.svc.cluster.local:50051
        topic: test-topic
        consumerGroup: sample-consumer-group
        lagThreshold: "500"
        sustainSeconds: "60"
```

| Parameter          | Value | Meaning in this run                                              |
|---------------------|-------|--------------------------------------------------------------------|
| `lagThreshold`      | 500   | Lag must exceed 500 messages before it's even considered high     |
| `sustainSeconds`    | 60    | High lag must persist continuously for 60s before triggering scale-up |
| `pollingInterval`   | 10    | KEDA asks the scaler for a fresh metric every 10s                 |
| `cooldownPeriod`    | 30    | Wait 30s after the last high-lag reading before scaling back down |
| `minReplicaCount`   | 1     | Consumer never scales below 1 replica                             |
| `maxReplicaCount`   | 10    | Consumer never scales above 10 replicas                           |

## Load pattern

Traffic was generated with [`load-test.sh`](../load-test.sh), which drives the consumer through two bursts:

1. **Phase 1 — moderate burst** (1000 messages, 64 bytes each): pushes lag just above `lagThreshold`, but the single consumer (~10 msg/s) drains it before `sustainSeconds` elapses.
2. **60s wait** for lag to fully drain and KEDA to cool down.
3. **Phase 2 — huge burst** (8000 messages, 64 bytes each): pushes far more lag than one consumer can drain within `sustainSeconds`.

## What the graph shows

Reading [`test-results-annotated.png`](./test-results-annotated.png) left to right:

1. **~10:40** — a small lag spike crosses the dashed threshold line but clears within seconds. Too short-lived to count as "persistent," so the scaler takes no action.
2. **~10:42:30** — Phase 1's moderate burst pushes lag to ~8k, well above threshold.
3. **~10:45** — lag has stayed above threshold long enough (`sustainSeconds: 60`) for the scaler to mark it persistent; consumer count steps from 1 to 10.
4. **~10:46–10:48** — the extra consumers drain the backlog; lag returns to near zero.
5. **~10:48:45** — Phase 2's huge burst hits, but since the consumer is already scaled up from the previous event, lag clears noticeably faster.
6. **~10:54:30** — lag has stayed near zero past `cooldownPeriod`, so KEDA scales the consumer back down to `minReplicaCount`.

The key behavior: a threshold breach alone (step 1) never triggers scaling — only a breach that persists for `sustainSeconds` does (step 3).
