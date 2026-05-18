#!/bin/bash
# Morfoschools — Restart frontend (picks up file changes)
# Usage: ./scripts/build-frontend.sh

set -e
cd "$(dirname "$0")/.."

echo "🔨 Restarting frontend..."
docker compose restart frontend

echo ""
echo "✅ Frontend restarted."
echo "   http://127.0.0.1:1666"
echo ""
