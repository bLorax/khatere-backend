#!/usr/bin/env bash
# Runs an upload load in the background and captures CPU/heap/goroutine
# profiles while it's active. Requires the app's pprof server (started
# in main.go) to be reachable inside the container on 127.0.0.1:6060,
# and the app container to be named "khatere".
# Usage: TOKEN=... PHOTO=/path/to/real.jpg ./scripts/capture_profiles.sh
set -euo pipefail
: "${TOKEN:?set TOKEN first}"
: "${PHOTO:?set PHOTO to a real jpg path first}"
SERVICE="${SERVICE:-app}"
CONTAINER_NAME="${CONTAINER_NAME:-khatere}"
OUTDIR="profiles/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$OUTDIR"

DURATION=35 TOKEN="$TOKEN" PHOTO="$PHOTO" ./scripts/load_test_uploads.sh > "$OUTDIR/upload_results.txt" 2>&1 &
LOAD_PID=$!

sleep 3
docker compose exec "$SERVICE" wget -qO /app/cpu.prof "http://127.0.0.1:6060/debug/pprof/profile?seconds=25"
docker compose exec "$SERVICE" wget -qO /app/heap.prof "http://127.0.0.1:6060/debug/pprof/heap"
docker compose exec "$SERVICE" wget -qO /app/goroutine.prof "http://127.0.0.1:6060/debug/pprof/goroutine"

wait "$LOAD_PID"

docker cp "$CONTAINER_NAME:/app/cpu.prof" "$OUTDIR/cpu.prof"
docker cp "$CONTAINER_NAME:/app/heap.prof" "$OUTDIR/heap.prof"
docker cp "$CONTAINER_NAME:/app/goroutine.prof" "$OUTDIR/goroutine.prof"

echo "Profiles saved to $OUTDIR"
echo "Analyze with: go tool pprof -top $OUTDIR/cpu.prof"
