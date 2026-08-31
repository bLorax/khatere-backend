#!/usr/bin/env bash
# Load-tests the two Redis-cached read paths. Needs TOKEN and EVENT_ID
# env vars set first (get them via login/GET /events as usual).
# Usage: TOKEN=... EVENT_ID=... ./scripts/load_test_reads.sh
set -euo pipefail
: "${TOKEN:?set TOKEN first}"
: "${EVENT_ID:?set EVENT_ID first}"
BASE="${BASE:-http://localhost:8080}"

echo "=== GET /events/{id} (membership cache) ==="
hey -z 30s -c 20 -H "Authorization: Bearer $TOKEN" "$BASE/events/$EVENT_ID"

echo "=== GET /gallery (gallery cache) ==="
hey -z 30s -c 20 -H "Authorization: Bearer $TOKEN" "$BASE/gallery"
