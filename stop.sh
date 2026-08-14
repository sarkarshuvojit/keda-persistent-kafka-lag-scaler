#!/usr/bin/env bash
#
# Tears down everything start.sh stood up on minikube: the persistent
# ScaledObject, consumer/producer/lag-scaler deployments, and infra
# (kafka, kafbat UI, prometheus, perses). Leaves minikube itself running.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==> Checking minikube status"
if ! minikube status >/dev/null 2>&1; then
  echo "minikube is not running, nothing to stop." >&2
  exit 0
fi

echo "==> Deleting persistent ScaledObject"
kubectl delete -f "$ROOT_DIR/examples/k8s/scalers/persistent/scaledobject.yaml" --ignore-not-found

echo "==> Deleting consumer, producer, and lag-scaler"
kubectl delete \
  -f "$ROOT_DIR/examples/k8s/deploy/deployment.yaml" \
  -f "$ROOT_DIR/examples/k8s/deploy/producer.yaml" \
  -f "$ROOT_DIR/examples/k8s/deploy/lag-scaler.yaml" \
  --ignore-not-found

echo "==> Deleting infra (kafka, kafbat, prometheus, perses)"
kubectl delete -f "$ROOT_DIR/examples/k8s/infra/" --ignore-not-found

echo
echo "==> Environment torn down."
echo "minikube is still running. To stop it entirely: minikube stop"
echo "To delete it entirely: minikube delete"
