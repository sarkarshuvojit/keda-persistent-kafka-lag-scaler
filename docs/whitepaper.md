# **Whitepaper: Persistent Kafka Consumer Lag-Based Scaler for KEDA**

## **Abstract**

Kubernetes Event-Driven Autoscaling (KEDA) currently supports event-source based scaling for many message systems, including Apache Kafka. Traditionally, KEDA’s Kafka scaler uses static thresholds (e.g., message counts per partition) to scale consumers. However, static thresholds are brittle and often lead to under-scaling, over-scaling, or oscillations.

This whitepaper proposes a **lag-based KEDA scaler**—one that scales based on **persistent consumer lag over time** rather than snapshot thresholds. We argue that **persistent lag is a better indicator of consumer pressure** than momentary metrics, aligning scaling decisions with real workload pressure and real Service Level Objectives (SLOs).

---

## **Table of Contents**

1. Background
2. Problem Statement
3. Why Static Thresholds Are Insufficient
4. Persistent Lag as a Better Scaling Signal
5. Lag-Based Scaler Design
6. Architecture and Flow
7. Example Implementation Logic
8. Evaluation and Benefits
9. Conclusion

---

## **1. Background**

Apache Kafka is a high-throughput distributed streaming platform. Consumer groups read events from topics and partitions. The lag between the **latest offset** and the **consumer’s committed offset** is a key metric describing consumer health.

Kubernetes Event-Driven Autoscaling (KEDA) enables event-driven scale-to-zero or scale-based workloads in Kubernetes. KEDA’s built-in scalers typically use static metrics (e.g., number of messages, age of messages) to decide scaling events.

Static threshold-based policies are simple but often don’t reflect real consumer pressure, especially under variable throughput and non-uniform workloads.

---

## **2. Problem Statement**

In traditional KEDA scaling:

* Static thresholds (e.g., 1000 unprocessed messages) trigger scale-up
* Even if lag is temporary and catches up quickly, autoscaler reacts
* Under heavy throughput spikes, static thresholds can under-react until lag grows dangerously high
* Scaling frequently oscillates (flapping)

Example static policy:

```yaml
spec:
  triggers:
  - type: kafka
    metadata:
      topic: my-topic
      lagThreshold: "1000"
```

But **snapshot-based lag is an instantaneous metric**. It does not reflect:

* how long lag has existed
* whether the cluster can catch up
* whether lag is trending up or down

Thus, scaling based on “current lag” can lead to wasteful or insufficient scaling.

---

## **3. Why Static Thresholds Are Insufficient**

### **3.1 Snapshot Metrics Are Volatile**

Kafka lag can fluctuate rapidly within seconds:

* Temporary surge in messages → lag spikes
* Consumer catches up → lag drops back
* Static threshold triggers scale-up even if no sustained backlog

This causes **unnecessary scaling events**.

### **3.2 Workload Diversity**

Some workloads have high variance:

| Time  | Throughput | Lag     |
| ----- | ---------- | ------- |
| 12:00 | 10k msg/s  | 15k lag |
| 12:05 | 50 msg/s   | 500 lag |

A static threshold of 1k would scale up at both times—even though only one represents actual lag pressure.

### **3.3 Metrics Aren’t SLO-Aligned**

Kafka lag itself is a proxy for delivering real service goals — e.g., latency from publish to processing.

A one-time snapshot doesn’t map directly to a **real user-impacting measure**. For many systems, customers care about **persistent lag** (e.g., sustained delays) more than brief spikes.

---

## **4. Persistent Lag as a Better Scaling Signal**

Instead of looking at one-off lag values, we define **persistent lag** as:

> Lag that *exceeds a threshold continuously* for a configured duration.

Example:

* **Lag Threshold:** 500
* **Sustain Time Window:** 2 minutes

Only if lag > 500 for 2+ minutes do we scale up.

This has key benefits:

* Ignores transient lag spikes
* Captures real bottlenecks where consumer throughput < producer throughput
* Reflects continuous pressure rather than momentary spikes
* Reduces scale flapping

This approach also aligns with the analysis in *Consumer Throughput: Kafka Time-Lag as an Uber Metric for SLO Monitoring* (Deby Roth), which emphasizes viewing lag over time rather than snapshots.

**Key idea:**

> *A lag snapshot at one point in time says little about future pressure. Persistent lag signals true imbalance in consumer processing.*

---

## **5. Lag-Based Scaler Design**

### **5.1 Metrics Collection**

Collect lag per partition at a given interval (e.g., 10s).

Store recent samples in a sliding window:

| Timestamp | Partition | Lag |
| --------- | --------- | --- |
| t-120s    | p1        | 300 |
| t-110s    | p1        | 450 |
| t-100s    | p1        | 550 |
| …         | …         | …   |

### **5.2 Evaluation Logic**

At each scaler tick:

1. Compute if **lag > threshold** for a configured **duration**
2. If yes, signal scale up
3. Otherwise, observe:

* If lag decreasing consistently → no scale
* If lag persistent high → scale
* If lag sustained low → scale down

### **5.3 Smoothing and Hysteresis**

To avoid jitter:

* Use moving averages or median metrics
* Add **scale-up interval** and **cool-down period**

---

## **6. Architecture & Flow**

![Image](https://imgix.datadoghq.com/img/dashboard/dashboard-header-kafka.png)

![Image](https://i.sstatic.net/G4adJ.png)

![Image](https://keda.sh/img/keda-arch.png)

![Image](https://www.opcito.com/sites/default/files/inline-images/Kubernetes_Autoscaling_Infographic.jpg)

### **6.1 Components**

1. **Metrics Scraper**
   Pulls consumer lag metrics from Kafka cluster

2. **Sliding Window Evaluator**
   Maintains buffer of recent lag samples

3. **Lag Sampler Logic**
   Determines whether lag is persistent

4. **KEDA Scaler Adapter**
   Integrates with KEDA’s webhook scaler

---

## **7. Example Implementation Logic**

Pseudo-logic for persistent lag evaluator:

```go
func shouldScaleUp(samples []LagSample, threshold int64, sustain time.Duration) bool {
  // filter samples above threshold
  high := filter(samples, func(s LagSample) bool {
    return s.Lag > threshold
  })

  // check if high lag persisted for sustain duration
  if len(high) == 0 {
    return false
  }

  oldestTime := high[0].Time
  newestTime := high[len(high)-1].Time

  return newestTime.Sub(oldestTime) >= sustain
}
```

Applied in KEDA:

```yaml
spec:
  triggers:
    type: external
    metadata:
      scalerAddress: http://lag-scaler.default.svc.cluster.local
      lagThreshold: "500"
      sustainSeconds: "120"
```

---

## **8. Evaluation & Benefits**

### **8.1 Higher Stability**

By monitoring persistent lag:

* Reduces false scaling events
* Only scales when there’s actual processing backlog

### **8.2 Better SLO Alignment**

Lag directly maps to user-visible delay:

* Persistent lag = processing bottleneck
* No lag = consumers keeping up

### **8.3 Cost Efficiency**

Reduces:

* Unnecessary pods
* Over-provisioning due to temporary spikes

---

## **9. Conclusion**

Static thresholds for Kafka consumer scaling in KEDA are simple but misaligned with real workload dynamics. A **persistent lag-based scaler** offers:

✔ More robust decision making
✔ Better alignment with real consumer pressure
✔ Reduced flapping and over-scaling
✔ Cost-effective scaling

This model favors **temporal persistence of lag**—the true indicator of insufficient consumer processing—over instantaneous metrics. It aligns autoscaling decisions with real operational goals and key performance signals, improving both efficiency and reliability.

