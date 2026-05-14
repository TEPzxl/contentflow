#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE="deployments/docker-compose.yaml"

case "${1:-up}" in
  up)
    docker compose -f "${COMPOSE_FILE}" up -d --build
    ;;
  down)
    docker compose -f "${COMPOSE_FILE}" down
    ;;
  restart)
    docker compose -f "${COMPOSE_FILE}" down
    docker compose -f "${COMPOSE_FILE}" up -d --build
    ;;
  logs)
    docker compose -f "${COMPOSE_FILE}" logs -f "${@:2}"
    ;;
  ps)
    docker compose -f "${COMPOSE_FILE}" ps
    ;;
  *)
    echo "Usage: $0 [up|down|restart|logs|ps]" >&2
    exit 1
    ;;
esac
