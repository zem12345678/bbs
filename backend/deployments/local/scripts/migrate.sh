#!/usr/bin/env bash
set -euo pipefail

BACKEND_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SERVICES_ROOT="$BACKEND_ROOT/services"

services=(
  admin-service
  user-service
  content-service
  comment-service
  credit-service
  file-service
  mall-service
  notification-service
  reaction-service
  chat-service
)

if (($# > 0)); then
  services=("$@")
fi

for service in "${services[@]}"; do
  service_dir="$SERVICES_ROOT/$service"
  if [[ ! -f "$service_dir/go.mod" ]]; then
    echo "Unknown BBS service '$service'." >&2
    exit 1
  fi

  echo "Migrating $service..."
  (
    cd "$service_dir"
    go run ./cmd migrate -c configs/config.yaml
  )
done

echo "Local BBS migrations completed."
