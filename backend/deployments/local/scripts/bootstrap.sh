#!/usr/bin/env bash
set -euo pipefail

LOCAL_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$LOCAL_ROOT"

load_local_environment() {
  [ -f .env ] || return 0

  while IFS= read -r line || [ -n "$line" ]; do
    line="${line%$'\r'}"
    line="${line#"${line%%[![:space:]]*}"}"
    [ -z "$line" ] && continue
    [[ "$line" == \#* ]] && continue
    if [[ "$line" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]]; then
      export "${BASH_REMATCH[1]}=${BASH_REMATCH[2]}"
    else
      echo "Invalid environment entry in $LOCAL_ROOT/.env: $line" >&2
      exit 1
    fi
  done < .env
}

local_env_value() {
  local name="$1"
  local fallback="${2:-}"
  local value="${!name:-}"
  if [ -z "${value//[[:space:]]/}" ]; then
    printf '%s' "$fallback"
  else
    printf '%s' "$value"
  fi
}

require_local_env_value() {
  local name="$1"
  local value
  value="$(local_env_value "$name")"
  if [ -z "${value//[[:space:]]/}" ]; then
    echo "$name must be set in $LOCAL_ROOT/.env before publishing local Nacos configs." >&2
    exit 1
  fi
  case "$value" in
    *$'\n'*|*$'\r'*)
      echo "$name must not contain newlines." >&2
      exit 1
      ;;
  esac
  printf '%s' "$value"
}

yaml_single_quoted_scalar() {
  local value="$1"
  value="${value//\'/\'\'}"
  printf "'%s'" "$value"
}

resolve_nacos_config() {
  local file="$1"
  local content
  content="$(<"$file")"

  case "$(basename "$file")" in
    bbs-api-gateway.yaml|bbs-file-service.yaml)
      local endpoint bucket access_key secret_key
      endpoint="$(yaml_single_quoted_scalar "$MINIO_ENDPOINT")"
      bucket="$(yaml_single_quoted_scalar "$MINIO_BUCKET")"
      access_key="$(yaml_single_quoted_scalar "$MINIO_ACCESS_KEY")"
      secret_key="$(yaml_single_quoted_scalar "$MINIO_SECRET_KEY")"
      content="${content//__BBS_LOCAL_MINIO_ENDPOINT__/$endpoint}"
      content="${content//__BBS_LOCAL_MINIO_BUCKET__/$bucket}"
      content="${content//__BBS_LOCAL_MINIO_ACCESS_KEY__/$access_key}"
      content="${content//__BBS_LOCAL_MINIO_SECRET_KEY__/$secret_key}"
      if [[ "$content" == *"__BBS_LOCAL_MINIO_"* ]]; then
        echo "Unresolved MinIO placeholder in $file." >&2
        exit 1
      fi
      ;;
  esac

  printf '%s' "$content"
}

FULL=false
EVENTS=false
COMMENTS=false
SEARCH=false
FILES=false
MAIL=false
POSTGRES_HOST="${POSTGRES_HOST:-127.0.0.1}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
POSTGRES_USER="${POSTGRES_USER:-postgres}"
POSTGRES_DATABASE="${POSTGRES_DATABASE:-bbs}"

for arg in "$@"; do
  case "$arg" in
    --full) FULL=true ;;
    --events) EVENTS=true ;;
    --comments) COMMENTS=true ;;
    --search) SEARCH=true ;;
    --files) FILES=true ;;
    --mail) MAIL=true ;;
    *) echo "Unknown argument: $arg" >&2; exit 1 ;;
  esac
done

if [ "$FULL" = true ]; then
  EVENTS=true
  COMMENTS=true
  SEARCH=true
  FILES=true
  MAIL=true
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

wait_http() {
  local url="$1"
  local name="$2"
  local retries="${3:-60}"
  for _ in $(seq 1 "$retries"); do
    if curl -fsS --max-time 2 "$url" >/dev/null; then
      echo "ready: $name ($url)"
      return 0
    fi
    sleep 1
  done
  echo "Timed out waiting for $name at $url" >&2
  exit 1
}

publish_nacos_config() {
  local file="$1"
  local data_id content
  data_id="$(basename "$file")"
  content="$(resolve_nacos_config "$file")"
  curl -fsS -X POST "$NACOS_URL/nacos/v1/cs/configs" \
    --data-urlencode "dataId=$data_id" \
    --data-urlencode "group=$NACOS_GROUP" \
    --data-urlencode "tenant=$NACOS_NAMESPACE" \
    --data-urlencode "type=yaml" \
    --data-urlencode "content=$content" >/dev/null
  echo "nacos config: $data_id"
}

load_local_environment
require_local_env_value MINIO_ENDPOINT >/dev/null
require_local_env_value MINIO_BUCKET >/dev/null
require_local_env_value MINIO_ACCESS_KEY >/dev/null
require_local_env_value MINIO_SECRET_KEY >/dev/null

NACOS_URL="$(local_env_value NACOS_URL http://127.0.0.1:8848)"
NACOS_URL="${NACOS_URL%/}"
if [[ "$NACOS_URL" =~ ^https?://([^/:]+)(:([0-9]+))?/?$ ]]; then
  NACOS_HOST="${BASH_REMATCH[1]}"
  if [ -n "${BASH_REMATCH[3]:-}" ]; then
    NACOS_PORT="${BASH_REMATCH[3]}"
  elif [[ "$NACOS_URL" == https://* ]]; then
    NACOS_PORT=443
  else
    NACOS_PORT=80
  fi
else
  echo "NACOS_URL must be an absolute http or https URL." >&2
  exit 1
fi
NACOS_NAMESPACE="$(local_env_value NACOS_NAMESPACE bbs-local)"
NACOS_GROUP="$(local_env_value NACOS_GROUP BBS_LOCAL)"

echo "Bootstrapping BBS local infrastructure..."

wait_tcp "$POSTGRES_HOST" "$POSTGRES_PORT" PostgreSQL
wait_tcp 127.0.0.1 6379 Redis
wait_tcp 127.0.0.1 2379 etcd
wait_tcp "$NACOS_HOST" "$NACOS_PORT" Nacos

echo "Applying PostgreSQL schemas and local app users..."
if ! [[ "$POSTGRES_DATABASE" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
  echo "POSTGRES_DATABASE must be a simple PostgreSQL identifier: $POSTGRES_DATABASE" >&2
  exit 1
fi

database_exists="$(psql --host "$POSTGRES_HOST" --port "$POSTGRES_PORT" --username "$POSTGRES_USER" --dbname postgres --tuples-only --no-align --command "SELECT 1 FROM pg_database WHERE datname = '$POSTGRES_DATABASE'" | tr -d '[:space:]')"
if [ "$database_exists" != "1" ]; then
  psql --host "$POSTGRES_HOST" --port "$POSTGRES_PORT" --username "$POSTGRES_USER" --dbname postgres --command "CREATE DATABASE \"$POSTGRES_DATABASE\""
  echo "postgres database created: $POSTGRES_DATABASE"
fi

psql --host "$POSTGRES_HOST" --port "$POSTGRES_PORT" --username "$POSTGRES_USER" --dbname "$POSTGRES_DATABASE" --file "$LOCAL_ROOT/postgres/init/001-create-database-and-schemas.sql"

echo "Preparing Nacos namespace/configs..."
curl -fsS -X POST "$NACOS_URL/nacos/v1/console/namespaces" \
  -d "customNamespaceId=$NACOS_NAMESPACE" \
  -d "namespaceName=$NACOS_NAMESPACE" \
  -d "namespaceDesc=BBS local development" >/dev/null || true

find ./nacos/configs -name "*.yaml" -type f | sort | while read -r file; do
  publish_nacos_config "$file"
done

if [ "$EVENTS" = true ]; then
  wait_tcp 127.0.0.1 9092 Kafka
  echo "Kafka is external; bootstrap does not create, delete, or alter its topics."
fi

if [ "$COMMENTS" = true ]; then
  wait_tcp 127.0.0.1 27017 MongoDB
  echo "MongoDB is external; comment-service ensures its required indexes on startup."
fi

if [ "$SEARCH" = true ]; then
  ELASTICSEARCH_URL="$(local_env_value ELASTICSEARCH_URL http://127.0.0.1:9200)"
  ELASTICSEARCH_URL="${ELASTICSEARCH_URL%/}"
  wait_http "$ELASTICSEARCH_URL/_cluster/health" Elasticsearch
  echo "Creating Elasticsearch indices..."
  for file in ./elasticsearch/*.mapping.json; do
    index="$(basename "$file" .mapping.json)"
    if curl -fsS -I "$ELASTICSEARCH_URL/$index" >/dev/null; then
      echo "elasticsearch index exists: $index"
    else
      curl -fsS -X PUT "$ELASTICSEARCH_URL/$index" -H "Content-Type: application/json" --data-binary "@$file" >/dev/null
      echo "elasticsearch index created: $index"
    fi
  done
fi

if [ "$FILES" = true ]; then
  MINIO_ENDPOINT="${MINIO_ENDPOINT%/}"
  wait_http "$MINIO_ENDPOINT/minio/health/live" MinIO
  echo "MinIO is external; bucket '$MINIO_BUCKET' is created on first upload."
fi

if [ "$MAIL" = true ]; then
  wait_tcp 127.0.0.1 1025 Mailpit-SMTP
  wait_tcp 127.0.0.1 8025 Mailpit-HTTP
fi

echo "Local infra bootstrap complete."
