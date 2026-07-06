#!/usr/bin/env bash
set -euo pipefail

LOCAL_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$LOCAL_ROOT"

FULL=false
EVENTS=false
COMMENTS=false
SEARCH=false
FILES=false

for arg in "$@"; do
  case "$arg" in
    --full) FULL=true ;;
    --events) EVENTS=true ;;
    --comments) COMMENTS=true ;;
    --search) SEARCH=true ;;
    --files) FILES=true ;;
    *) echo "Unknown argument: $arg" >&2; exit 1 ;;
  esac
done

if [ "$FULL" = true ]; then
  EVENTS=true
  COMMENTS=true
  SEARCH=true
  FILES=true
fi

wait_tcp() {
  local host="$1"
  local port="$2"
  local name="$3"
  local retries="${4:-60}"
  for _ in $(seq 1 "$retries"); do
    if (echo >"/dev/tcp/$host/$port") >/dev/null 2>&1; then
      echo "ready: $name ($host:$port)"
      return 0
    fi
    sleep 1
  done
  echo "Timed out waiting for $name at $host:$port" >&2
  exit 1
}

publish_nacos_config() {
  local file="$1"
  local data_id
  data_id="$(basename "$file")"
  curl -fsS -X POST "http://127.0.0.1:8848/nacos/v1/cs/configs" \
    --data-urlencode "dataId=$data_id" \
    --data-urlencode "group=BBS_LOCAL" \
    --data-urlencode "tenant=bbs-local" \
    --data-urlencode "type=yaml" \
    --data-urlencode "content@$file" >/dev/null
  echo "nacos config: $data_id"
}

echo "Bootstrapping BBS local infrastructure..."

wait_tcp 127.0.0.1 5432 PostgreSQL
wait_tcp 127.0.0.1 6379 Redis
wait_tcp 127.0.0.1 2379 etcd
wait_tcp 127.0.0.1 8848 Nacos

echo "Applying PostgreSQL schemas and local app users..."
docker compose exec -T postgres psql -U postgres -d bbs -f /docker-entrypoint-initdb.d/001-create-database-and-schemas.sql

echo "Preparing Nacos namespace/configs..."
curl -fsS -X POST "http://127.0.0.1:8848/nacos/v1/console/namespaces" \
  -d "customNamespaceId=bbs-local" \
  -d "namespaceName=bbs-local" \
  -d "namespaceDesc=BBS local development" >/dev/null || true

find ./nacos/configs -name "*.yaml" -type f | sort | while read -r file; do
  publish_nacos_config "$file"
done

if [ "$EVENTS" = true ]; then
  wait_tcp 127.0.0.1 9092 Kafka
  echo "Creating Kafka topics..."
  while IFS= read -r topic; do
    [ -z "$topic" ] && continue
    docker compose exec -T kafka kafka-topics.sh --bootstrap-server 127.0.0.1:29092 --create --if-not-exists --topic "$topic" --partitions 1 --replication-factor 1
  done < ./kafka/topics.txt
fi

if [ "$COMMENTS" = true ]; then
  wait_tcp 127.0.0.1 27017 MongoDB
  echo "Creating MongoDB comment indexes..."
  docker compose exec -T mongodb mongosh /docker-entrypoint-initdb.d/001-comments-indexes.js
fi

if [ "$SEARCH" = true ]; then
  wait_tcp 127.0.0.1 9200 Elasticsearch
  echo "Creating Elasticsearch indices..."
  for file in ./elasticsearch/*.mapping.json; do
    index="$(basename "$file" .mapping.json)"
    if curl -fsS -I "http://127.0.0.1:9200/$index" >/dev/null; then
      echo "elasticsearch index exists: $index"
    else
      curl -fsS -X PUT "http://127.0.0.1:9200/$index" -H "Content-Type: application/json" --data-binary "@$file" >/dev/null
      echo "elasticsearch index created: $index"
    fi
  done
fi

if [ "$FILES" = true ]; then
  wait_tcp 127.0.0.1 9000 MinIO
  echo "Creating MinIO bucket..."
  docker run --rm --network bbs-local-net minio/mc:latest sh -c "mc alias set local http://minio:9000 minioadmin minioadmin >/dev/null && mc mb --ignore-existing local/bbs-local"
fi

echo "Local infra bootstrap complete."

