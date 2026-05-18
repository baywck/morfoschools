#!/bin/bash
# Morfoschools — Run frontend typecheck
# Usage: ./scripts/test-frontend.sh

set -e
cd "$(dirname "$0")/../frontend"

echo "🧪 Running frontend typecheck..."
npx tsc --noEmit
echo "✅ Frontend typecheck passed."
