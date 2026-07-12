#!/usr/bin/env bash
set -euo pipefail

if [ "${1:-}" != "--confirm" ]; then
  echo "Refusing to reset local infrastructure without --confirm." >&2
  exit 1
fi

LOCAL_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$LOCAL_ROOT"

PROJECT_NAME="bbs-local"
if [ -f .env ]; then
  ENV_PROJECT_NAME="$(grep -E '^COMPOSE_PROJECT_NAME=' .env | tail -n 1 | cut -d '=' -f 2- || true)"
  if [ -n "$ENV_PROJECT_NAME" ]; then
    PROJECT_NAME="$ENV_PROJECT_NAME"
  fi
fi

if [ -z "$PROJECT_NAME" ] || [ "$PROJECT_NAME" != "bbs-local" ]; then
  echo "Refusing to delete volumes for unexpected Compose project: '$PROJECT_NAME'." >&2
  exit 1
fi

echo "Volumes currently owned by $PROJECT_NAME:"
docker volume ls --filter "label=com.docker.compose.project=$PROJECT_NAME" --format "  {{.Name}}"

echo "Stopping services and deleting bbs-local volumes..."
docker compose down -v --remove-orphans
echo "Reset complete."
