#!/bin/bash
# Morfoschools — Run all checks (backend tests + frontend typecheck)
# Usage: ./scripts/test-all.sh

set -e
cd "$(dirname "$0")/.."

echo "🧪 Running all checks..."
echo ""

./scripts/test-backend.sh
echo ""
./scripts/test-frontend.sh

echo ""
echo "✅ All checks passed."
