#!/bin/bash
# Morfoschools — Start all services
# Usage: ./scripts/start.sh

set -e
cd "$(dirname "$0")/.."

echo "🚀 Starting Morfoschools..."

# Copy env if not exists
if [ ! -f .env ]; then
  cp .env.example .env
  echo "📋 Created .env from .env.example"
fi

docker compose up -d

echo ""
echo "✅ All services starting..."
echo ""
echo "   Frontend:  http://127.0.0.1:1666"
echo "   Backend:   http://127.0.0.1:8080"
echo "   PgBouncer: localhost:6432"
echo "   Valkey:    localhost:6399"
echo "   NATS:      localhost:4222"
echo ""
echo "   Logs: docker compose logs -f"
echo ""
