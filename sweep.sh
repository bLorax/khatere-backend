#!/usr/bin/env bash
# Full endpoint sweep for khatere-backend. Generates a brand-new pair of
# users every run, so it's safe to run repeatedly with no manual cleanup.
# Produces real traffic to look at in Jaeger and Grafana afterward.
#
# Usage:
#   ./sweep.sh              # run once
#   REPEAT=10 ./sweep.sh     # run the whole sweep 10 times in a row,
#                             # useful to get a visible rate/latency graph
#                             # in Grafana instead of a single data point

set -e

BASE="${BASE:-http://localhost:8080}"
JAEGER_URL="${JAEGER_URL:-http://localhost:16686}"
GRAFANA_URL="${GRAFANA_URL:-http://localhost:3000}"
REPEAT="${REPEAT:-1}"

# A tiny 50x50 JPEG, embedded so the script needs no external file.
TEST_JPG="$(mktemp /tmp/khatere-test-XXXX.jpg)"
base64 -d > "$TEST_JPG" <<'B64'
/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/2wBDAQkJCQwLDBgNDRgyIRwhMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjL/wAARCAAyADIDASIAAhEBAxEB/8QAHwAAAQUBAQEBAQEAAAAAAAAAAAECAwQFBgcICQoL/8QAtRAAAgEDAwIEAwUFBAQAAAF9AQIDAAQRBRIhMUEGE1FhByJxFDKBkaEII0KxwRVS0fAkM2JyggkKFhcYGRolJicoKSo0NTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqDhIWGh4iJipKTlJWWl5iZmqKjpKWmp6ipqrKztLW2t7i5usLDxMXGx8jJytLT1NXW19jZ2uHi4+Tl5ufo6erx8vP09fb3+Pn6/8QAHwEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoL/8QAtREAAgECBAQDBAcFBAQAAQJ3AAECAxEEBSExBhJBUQdhcRMiMoEIFEKRobHBCSMzUvAVYnLRChYkNOEl8RcYGRomJygpKjU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6goOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4uPk5ebn6Onq8vP09fb3+Pn6/9oADAMBAAIRAxEAPwDp6KKK9s8IKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooA/9k=
B64

cleanup() { rm -f "$TEST_JPG"; }
trap cleanup EXIT

run_sweep() {
  local run_id="$1"
  local suffix
  suffix="$(date +%s)$RANDOM"

  echo "=========================================="
  echo "Sweep run $run_id — user suffix: $suffix"
  echo "=========================================="

  # --- Auth ---
  echo "--- register ---"
  curl -s -X POST "$BASE/register" -d "{\"username\":\"alice$suffix\",\"email\":\"alice$suffix@test.com\",\"password\":\"secret123\"}"
  echo
  curl -s -X POST "$BASE/register" -d "{\"username\":\"bob$suffix\",\"email\":\"bob$suffix@test.com\",\"password\":\"secret123\"}"
  echo

  ALICE_JSON=$(curl -s -X POST "$BASE/login" -d "{\"identifier\":\"alice$suffix\",\"password\":\"secret123\"}")
  ALICE_TOKEN=$(echo "$ALICE_JSON" | grep -o '"token":"[^"]*' | cut -d'"' -f4)

  BOB_JSON=$(curl -s -X POST "$BASE/login" -d "{\"identifier\":\"bob$suffix\",\"password\":\"secret123\"}")
  BOB_TOKEN=$(echo "$BOB_JSON" | grep -o '"token":"[^"]*' | cut -d'"' -f4)
  BOB_ID=$(echo "$BOB_JSON" | grep -o '"id":"[^"]*' | cut -d'"' -f4)

  # --- User ---
  echo "--- user search ---"
  curl -s "$BASE/users/search?q=bob$suffix" -H "Authorization: Bearer $ALICE_TOKEN"
  echo
  echo "--- get user ---"
  curl -s "$BASE/users/$BOB_ID" -H "Authorization: Bearer $ALICE_TOKEN"
  echo

  # --- Event ---
  echo "--- create event ---"
  EVENT_JSON=$(curl -s -X POST "$BASE/events" \
    -H "Authorization: Bearer $ALICE_TOKEN" \
    -d "{\"name\":\"Sweep Event $suffix\",\"location\":\"Test City\"}")
  echo "$EVENT_JSON"
  EVENT_ID=$(echo "$EVENT_JSON" | grep -o '"id":"[^"]*' | cut -d'"' -f4)

  echo "--- list events ---"
  curl -s "$BASE/events" -H "Authorization: Bearer $ALICE_TOKEN" > /dev/null
  echo "(ok)"

  echo "--- get event, call 1 (cache miss expected) ---"
  curl -s "$BASE/events/$EVENT_ID" -H "Authorization: Bearer $ALICE_TOKEN" > /dev/null
  echo "--- get event, call 2 (cache hit expected) ---"
  curl -s "$BASE/events/$EVENT_ID" -H "Authorization: Bearer $ALICE_TOKEN" > /dev/null
  echo "(ok)"

  echo "--- tag bob ---"
  MEMBER_JSON=$(curl -s -X POST "$BASE/events/$EVENT_ID/members" \
    -H "Authorization: Bearer $ALICE_TOKEN" \
    -d "{\"user_id\":\"$BOB_ID\"}")
  echo "$MEMBER_JSON"
  MEMBER_ID=$(echo "$MEMBER_JSON" | grep -o '"id":"[^"]*' | cut -d'"' -f4)

  echo "--- get event as bob before approval (expect 404) ---"
  curl -s -o /dev/null -w '%{http_code}\n' "$BASE/events/$EVENT_ID" -H "Authorization: Bearer $BOB_TOKEN"

  echo "--- bob approves the tag ---"
  curl -s -X POST "$BASE/event-members/$MEMBER_ID/approve" -H "Authorization: Bearer $BOB_TOKEN"
  echo

  echo "--- get event as bob after approval (expect 200, cache invalidated) ---"
  curl -s -o /dev/null -w '%{http_code}\n' "$BASE/events/$EVENT_ID" -H "Authorization: Bearer $BOB_TOKEN"

  # --- Notification ---
  # Kafka delivery + consumer processing + the Postgres insert all happen
  # off the request path, asynchronously. A single immediate check can
  # race ahead of that pipeline, so poll briefly instead of checking once.
  echo "--- alice's notifications ---"
  NOTIF_JSON="{\"results\":[]}"
  for attempt in $(seq 1 10); do
    NOTIF_JSON=$(curl -s "$BASE/notifications" -H "Authorization: Bearer $ALICE_TOKEN")
    if echo "$NOTIF_JSON" | grep -q '"id":"'; then
      break
    fi
    sleep 0.3
  done
  echo "$NOTIF_JSON"
  NOTIF_ID=$(echo "$NOTIF_JSON" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)

  echo "--- mark it read ---"
  if [ -n "$NOTIF_ID" ]; then
    curl -s -o /dev/null -w '%{http_code}\n' -X POST "$BASE/notifications/$NOTIF_ID/read" -H "Authorization: Bearer $ALICE_TOKEN"
  else
    echo "skipped — no notification id available after polling"
  fi

  # --- Photo ---
  echo "--- bob uploads a photo ---"
  curl -s -X POST "$BASE/events/$EVENT_ID/photos" \
    -H "Authorization: Bearer $BOB_TOKEN" \
    -F "file=@$TEST_JPG"
  echo

  echo "--- get event (expect photos + thumbnail_url) ---"
  curl -s "$BASE/events/$EVENT_ID" -H "Authorization: Bearer $ALICE_TOKEN"
  echo

  # --- Gallery ---
  echo "--- alice's gallery ---"
  curl -s "$BASE/gallery" -H "Authorization: Bearer $ALICE_TOKEN"
  echo

  # --- Cleanup path ---
  echo "--- bob removes his membership ---"
  curl -s -o /dev/null -w '%{http_code}\n' -X DELETE "$BASE/event-members/$MEMBER_ID" -H "Authorization: Bearer $BOB_TOKEN"

  # --- Rate limit check (login only — safe, resets after the window) ---
  echo "--- one extra failed login, just to generate a rate-limit data point ---"
  curl -s -o /dev/null -w '%{http_code}\n' -X POST "$BASE/login" -d "{\"identifier\":\"alice$suffix\",\"password\":\"wrong-password\"}"

  echo "Sweep run $run_id done."
  echo
}

for i in $(seq 1 "$REPEAT"); do
  run_sweep "$i"
done

echo "=========================================="
echo "All $REPEAT sweep(s) done."
echo "Jaeger:   $JAEGER_URL   (service: khatere-http)"
echo "Grafana:  $GRAFANA_URL  (dashboard: Khatere Overview)"
echo "Prometheus targets: http://localhost:9090/targets"
echo "=========================================="
