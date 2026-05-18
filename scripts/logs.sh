#!/bin/bash
# Morfoschools — View logs
# Usage: ./scripts/logs.sh [service]
# Examples:
#   ./scripts/logs.sh           # all services
#   ./scripts/logs.sh backend   # backend only
#   ./scripts/logs.sh frontend  # frontend only

set -e
cd "$(dirname "$0")/.."

if [ -n "$1" ]; then
  docker compose logs -f "$1"
else
  docker compose logs -f
fi
