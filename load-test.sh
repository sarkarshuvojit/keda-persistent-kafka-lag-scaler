#!/usr/bin/env bash
#
# Drives the kafka-producer service through two bursts to demonstrate
# persistent-lag scaling (see scaler/README.md, Step 4):
#
#   Phase 1 (moderate burst): pushes lag slightly above lagThreshold, but the
#   single consumer (~10 msg/s: ReadMessage + 100ms sleep) drains it before
#   sustainSeconds elapses. Expected: lag spikes then drops, no scale-up.
#
#   Phase 2 (huge burst, after a cool-down wait): pushes far more lag than a
#   single consumer can drain within sustainSeconds. Expected: lag stays
#   above threshold long enough for the scaler to mark it persistent, and
#   KEDA scales the consumer up.
#
# Assumes start.sh has already stood up the environment and kafka-producer
# is reachable in-cluster at http://kafka-producer/produce.
set -euo pipefail

MODERATE_COUNT=1000
HUGE_COUNT=8000
MESSAGE_SIZE=64
WAIT_BETWEEN=180
PRODUCER_URL="http://kafka-producer/produce"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --moderate-count)
      MODERATE_COUNT="$2"
      shift 2
      ;;
    --huge-count)
      HUGE_COUNT="$2"
      shift 2
      ;;
    --message-size)
      MESSAGE_SIZE="$2"
      shift 2
      ;;
    --wait-between)
      WAIT_BETWEEN="$2"
      shift 2
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

produce() {
  local count="$1"
  local pod="curl-load-$(date +%s)-${RANDOM}"
  kubectl run "$pod" --rm -i --restart=Never --image=curlimages/curl --quiet -- \
    curl -s -X POST "$PRODUCER_URL" \
    -H 'Content-Type: application/json' \
    -d "{\"count\":${count},\"messageSize\":${MESSAGE_SIZE}}"
  echo
}

echo "==> Phase 1: moderate burst ($MODERATE_COUNT messages, ${MESSAGE_SIZE}b each)"
echo "    Expect: lag spikes just above threshold, drains before sustainSeconds"
echo "    elapses -> no scale-up. Watch with:"
echo "      kubectl logs -l app=lag-scaler -f"
echo "      kubectl get pods -l app=kafka-consumer -w"
produce "$MODERATE_COUNT"

echo
echo "==> Waiting ${WAIT_BETWEEN}s for lag to fully drain and KEDA to cool down..."
sleep "$WAIT_BETWEEN"

echo
echo "==> Phase 2: huge burst ($HUGE_COUNT messages, ${MESSAGE_SIZE}b each)"
echo "    Expect: lag stays above threshold past sustainSeconds -> scale-up triggers."
echo "    Watch with:"
echo "      kubectl get pods -l app=kafka-consumer -w"
produce "$HUGE_COUNT"

echo
echo "==> Done producing. Keep watching pods/logs to observe scale-up and, once"
echo "    drained, the eventual scale-down back to minReplicaCount."
