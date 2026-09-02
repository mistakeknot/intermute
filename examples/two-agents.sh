#!/usr/bin/env bash
# Two-agent walkthrough: reservation, conflict detection, negotiation by
# message, release, re-reservation. Starts its own intermute server against
# a temp database and tears it down on exit, so this is safe to run as-is.
#
#   ./examples/two-agents.sh
#   PORT=7398 ./examples/two-agents.sh   # if the default port is taken
set -euo pipefail

PORT="${PORT:-7399}"
BASE="http://127.0.0.1:${PORT}"
PROJECT="two-agents-demo"
TMPDIR_RUN="$(mktemp -d)"
DB="${TMPDIR_RUN}/x.db"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# Keep the generated dev key out of the repo directory.
export INTERMUTE_KEYS_FILE="${TMPDIR_RUN}/intermute.keys.yaml"

if curl -sf "${BASE}/health" >/dev/null 2>&1; then
  echo "something already answers on ${BASE}; this script wants a fresh server. Set PORT to a free port." >&2
  exit 1
fi

echo "==> starting: go run ./cmd/intermute serve --db $DB --port $PORT"
go run ./cmd/intermute serve --db "$DB" --port "$PORT" &
SERVER_PID=$!
cleanup() {
  # `go run` execs a separate compiled binary as a child process, so killing
  # $SERVER_PID alone leaves the actual server running. Find it by its
  # unique argv (this run's temp DB path + port) and kill that too.
  local child
  child=$(pgrep -f "serve --db ${DB} --port ${PORT}" 2>/dev/null || true)
  if [[ -n "$child" ]]; then
    kill $child >/dev/null 2>&1 || true
  fi
  kill "$SERVER_PID" >/dev/null 2>&1 || true
  wait "$SERVER_PID" 2>/dev/null || true
}
trap cleanup EXIT

echo "==> waiting for health"
up=0
for _ in $(seq 1 100); do
  if curl -sf "${BASE}/health" >/dev/null 2>&1; then
    up=1
    break
  fi
  sleep 0.2
done
if [[ "$up" -ne 1 ]]; then
  echo "server never came up on ${BASE}" >&2
  exit 1
fi
if ! pgrep -f "serve --db ${DB} --port ${PORT}" >/dev/null 2>&1; then
  echo "health answered on ${BASE} but this script's server is not running; another process owns the port" >&2
  exit 1
fi

json_field() {
  # $1 = python expression to index the parsed dict, e.g. "['agent_id']"
  python3 -c "import json,sys; print(json.load(sys.stdin)$1)"
}

echo
echo "==> register alpha"
ALPHA=$(curl -sf -X POST "${BASE}/api/agents" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"alpha\",\"project\":\"${PROJECT}\"}")
echo "$ALPHA"
ALPHA_ID=$(echo "$ALPHA" | json_field "['agent_id']")

echo
echo "==> register beta"
BETA=$(curl -sf -X POST "${BASE}/api/agents" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"beta\",\"project\":\"${PROJECT}\"}")
echo "$BETA"
BETA_ID=$(echo "$BETA" | json_field "['agent_id']")

echo
echo "==> alpha reserves src/**/*.go"
RESERVATION=$(curl -sf -X POST "${BASE}/api/reservations" \
  -H 'Content-Type: application/json' \
  -H "X-Agent-ID: ${ALPHA_ID}" \
  -d "{\"agent_id\":\"${ALPHA_ID}\",\"project\":\"${PROJECT}\",\"path_pattern\":\"src/**/*.go\",\"exclusive\":true,\"reason\":\"refactor\"}")
echo "$RESERVATION"
RESERVATION_ID=$(echo "$RESERVATION" | json_field "['id']")

echo
echo "==> beta checks the same pattern for conflicts"
CHECK=$(curl -sf -G "${BASE}/api/reservations/check" \
  --data-urlencode "project=${PROJECT}" \
  --data-urlencode "pattern=src/**/*.go" \
  --data-urlencode "exclusive=true")
echo "$CHECK"
echo "$CHECK" | python3 -c "
import json, sys
d = json.load(sys.stdin)
conflicts = d.get('conflicts') or []
assert conflicts, 'expected beta to see alpha\'s reservation as a conflict'
assert conflicts[0]['agent_id'] == '${ALPHA_ID}'
print('conflict confirmed: held by', conflicts[0]['held_by'])
"

echo
echo "==> beta messages alpha asking for the reservation"
curl -sf -X POST "${BASE}/api/messages" \
  -H 'Content-Type: application/json' \
  -H "X-Agent-ID: ${BETA_ID}" \
  -d "{\"project\":\"${PROJECT}\",\"from\":\"${BETA_ID}\",\"to\":[\"${ALPHA_ID}\"],\"subject\":\"release request\",\"body\":\"can you release src/**/*.go?\"}"
echo

echo
echo "==> alpha reads its inbox"
curl -sf -G "${BASE}/api/inbox/${ALPHA_ID}" --data-urlencode "project=${PROJECT}"
echo

echo
echo "==> alpha releases the reservation"
curl -sf -X DELETE "${BASE}/api/reservations/${RESERVATION_ID}" \
  -H "X-Agent-ID: ${ALPHA_ID}"
echo "released ${RESERVATION_ID}"

echo
echo "==> beta reserves the now-free pattern"
curl -sf -X POST "${BASE}/api/reservations" \
  -H 'Content-Type: application/json' \
  -H "X-Agent-ID: ${BETA_ID}" \
  -d "{\"agent_id\":\"${BETA_ID}\",\"project\":\"${PROJECT}\",\"path_pattern\":\"src/**/*.go\",\"exclusive\":true,\"reason\":\"refactor\"}"
echo

echo
echo "==> done: alpha and beta coordinated a handoff without stepping on each other"
