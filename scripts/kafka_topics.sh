#!/usr/bin/env bash
set -euo pipefail

KAFKA_CONTAINER="${KAFKA_CONTAINER:-contentflow_kafka}"

topics=(
  "collection.requested"
  "collection.completed"
  "collection.failed"
  "collection.dlq"
)

for topic in "${topics[@]}"; do
  docker exec "${KAFKA_CONTAINER}" kafka-topics.sh \
    --bootstrap-server localhost:9092 \
    --create \
    --if-not-exists \
    --topic "${topic}" \
    --partitions 3 \
    --replication-factor 1
done

docker exec "${KAFKA_CONTAINER}" kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --list
