#!/bin/bash
# Morfoschools — Build and restart backend only
# Usage: ./scripts/build-backend.sh

set -e
cd "$(dirname "$0")/.."

echo "🔨 Building backend..."
docker compose up -d --build backend

echo ""
echo "✅ Backend rebuilt and restarted."
echo "   http://127.0.0.1:8080/readyz"
echo ""
