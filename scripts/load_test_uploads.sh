#!/usr/bin/env bash
# Load-tests photo upload against several pre-created events (avoids
# hitting MaxPhotosPerEvent, and avoids creating an event inline on
# every request, which made earlier runs flaky).
# Usage: TOKEN=... PHOTO=/path/to/real.jpg ./scripts/load_test_uploads.sh
set -euo pipefail
: "${TOKEN:?set TOKEN first}"
: "${PHOTO:?set PHOTO to a real jpg path first}"
BASE="${BASE:-http://localhost:8080}"
DURATION="${DURATION:-30}"
NUM_EVENTS="${NUM_EVENTS:-5}"
CONCURRENCY="${CONCURRENCY:-5}"

echo "Creating $NUM_EVENTS events..."
EVENT_IDS=()
for i in $(seq 1 "$NUM_EVENTS"); do
  EID=$(curl -s -X POST "$BASE/events" -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" -d "{\"name\":\"Load $i\",\"location\":\"X\"}" \
    | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)
  if [ -z "$EID" ]; then
    echo "failed to create event $i — aborting" >&2
    exit 1
  fi
  EVENT_IDS+=("$EID")
done
echo "Events: ${EVENT_IDS[*]}"

END=$((SECONDS + DURATION))
i=0
while [ $SECONDS -lt "$END" ]; do
  EID=${EVENT_IDS[$((i % NUM_EVENTS))]}
  curl -s -o /dev/null -w "%{http_code}\n" -X POST "$BASE/events/$EID/photos" \
    -H "Authorization: Bearer $TOKEN" -F "file=@$PHOTO" &
  i=$((i + 1))
  [ $((i % CONCURRENCY)) -eq 0 ] && wait
done
wait
echo "total requests: $i"
