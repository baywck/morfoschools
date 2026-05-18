#!/bin/bash
# Morfoschools — Build and restart everything
# Usage: ./scripts/build-all.sh

set -e
cd "$(dirname "$0")/.."

echo "🔨 Building all services..."

# Copy env if not exists
if [ ! -f .env ]; then
  cp .env.example .env
  echo "📋 Created .env from .env.example"
fi

docker compose up -d --build

echo ""
echo "✅ All services rebuilt and running."
echo ""
echo "   Frontend:  http://127.0.0.1:1666"
echo "   Backend:   http://127.0.0.1:8080"
echo ""
