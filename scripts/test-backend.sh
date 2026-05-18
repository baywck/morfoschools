#!/bin/bash
# Morfoschools — Run backend tests
# Usage: ./scripts/test-backend.sh

set -e
cd "$(dirname "$0")/../backend"

export PATH=$PATH:/usr/local/go/bin

echo "🧪 Running backend tests..."
go test ./...
echo "✅ All backend tests passed."
