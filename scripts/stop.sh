#!/bin/bash
# Morfoschools — Stop all services
# Usage: ./scripts/stop.sh

set -e
cd "$(dirname "$0")/.."

echo "🛑 Stopping Morfoschools..."
docker compose down
echo "✅ All services stopped."
