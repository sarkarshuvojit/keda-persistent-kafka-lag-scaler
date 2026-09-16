#!/usr/bin/env bash
#
# Stands up the full example environment on minikube: Kafka, kafbat UI,
# Prometheus, Grafana, the sample producer/consumer, and the
# persistent lag scaler wired to KEDA. See scaler/README.md for the manual
# step-by-step this script automates.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARTITIONS=10
TOPIC="test-topic"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --partitions)
      PARTITIONS="$2"
      shift 2
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

echo "==> Checking minikube status"
if ! minikube status >/dev/null 2>&1; then
  echo "minikube is not running. Start it first with: minikube start" >&2
  exit 1
fi

echo "==> Pointing Docker CLI at minikube's daemon"
eval "$(minikube docker-env)"

echo "==> Building sample-app image (kpkls:latest)"
docker build -t kpkls:latest "$ROOT_DIR/examples/sample-app"

echo "==> Building scaler image (kpkls-scaler:latest)"
docker build -t kpkls-scaler:latest "$ROOT_DIR/scaler"

echo "==> Applying infra (kafka, kafbat, prometheus, grafana)"
kubectl apply -f "$ROOT_DIR/examples/k8s/infra/"

echo "==> Waiting for Kafka to be ready"
kubectl rollout status deploy/kafka --timeout=120s

echo "==> Creating topic '$TOPIC' with $PARTITIONS partition(s)"
kubectl exec deploy/kafka -- /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --create --if-not-exists \
  --topic "$TOPIC" --partitions "$PARTITIONS" --replication-factor 1

echo "==> Deploying consumer, producer, and lag-scaler"
kubectl apply \
  -f "$ROOT_DIR/examples/k8s/deploy/deployment.yaml" \
  -f "$ROOT_DIR/examples/k8s/deploy/producer.yaml" \
  -f "$ROOT_DIR/examples/k8s/deploy/lag-scaler.yaml"

echo "==> Applying persistent ScaledObject"
kubectl delete -f "$ROOT_DIR/examples/k8s/scalers/basic/scaledobject.yaml" --ignore-not-found
kubectl apply -f "$ROOT_DIR/examples/k8s/scalers/persistent/scaledobject.yaml"

echo "==> Current pod status"
kubectl get pods

echo
echo "==> Environment is up."
echo "On the Docker driver, 'minikube service --url' opens a long-lived tunnel and blocks,"
echo "so open these in a separate terminal (each stays running to serve traffic):"
echo "  minikube service kafbat --url"
echo "  minikube service grafana --url"
echo
echo "Generate load with:"
echo "  kubectl run curl-load --rm -it --restart=Never --image=curlimages/curl -- \\"
echo "    curl -s -X POST http://kafka-producer/produce \\"
echo "    -H 'Content-Type: application/json' -d '{\"count\":5000,\"messageSize\":64}'"
