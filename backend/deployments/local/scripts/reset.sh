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
  echo "Refusing to reset an unexpected Compose project: '$PROJECT_NAME'." >&2
  exit 1
fi

MAILPIT_CONTAINER_IDS="$(docker compose ps --all --quiet mailpit)"
if [ -z "$MAILPIT_CONTAINER_IDS" ]; then
  echo "BBS Mailpit is not present; no container was removed."
else
  echo "Stopping and removing only the BBS Mailpit container..."
  docker compose rm --stop --force mailpit
fi
echo "Reset complete."
