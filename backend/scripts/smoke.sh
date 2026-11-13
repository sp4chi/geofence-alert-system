#!/usr/bin/env bash
# End-to-end smoke test of every endpoint (and the WebSocket) against API_URL, local or deployed.
# Each call must return the expected status and a numeric-string time_ns. Stops at the first failure.
#
# Usage: API_URL=https://... smoke.sh      (default http://localhost:8080)
# Needs: curl, python3; node 22+ for the WebSocket check (skipped if missing)
set -euo pipefail

API_URL=${API_URL:-http://localhost:8080}
WS_URL=${WS_URL:-${API_URL/http/ws}/ws/alerts} # http→ws, https→wss

# call METHOD PATH WANT_STATUS [BODY]: prints the response body; exits if status or time_ns is wrong.
call() {
  local method=$1 path=$2 want=$3 body=${4:-} out status
  out=$(curl -sS -X "$method" "$API_URL$path" -H 'Content-Type: application/json' ${body:+-d "$body"} -w $'\n%{http_code}')
  status=${out##*$'\n'}
  out=${out%$'\n'*}
  if [[ $status != "$want" ]]; then
    echo "FAIL $method $path → $status (want $want): $out" >&2
    exit 1
  fi
  if ! python3 -c 'import json, re, sys; t = json.loads(sys.argv[1]).get("time_ns"); sys.exit(not (isinstance(t, str) and re.fullmatch(r"\d+", t)))' "$out"; then
    echo "FAIL $method $path: time_ns missing or not a numeric string: $out" >&2
    exit 1
  fi
  printf 'ok  %-4s %-60s %s\n' "$method" "${path:0:60}" "$status" >&2
  printf '%s' "$out"
}

# get JSON EXPR: evaluate a Python expression against the parsed JSON as d, e.g. 'd["id"]'.
get() { python3 -c 'import json, sys; d = json.loads(sys.argv[1]); print(eval(sys.argv[2]))' "$1" "$2"; }

expect() { # expect LABEL ACTUAL WANTED
  if [[ $2 != "$3" ]]; then echo "FAIL $1: got $2, want $3" >&2; exit 1; fi
  echo "ok  $1 = $2" >&2
}

ago() { python3 -c 'import datetime, sys; print((datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(seconds=int(sys.argv[1]))).strftime("%Y-%m-%dT%H:%M:%SZ"))' "$1"; }

run=$(date +%s)
# A small square in its own spot for this run, so repeated runs never overlap.
lng=$(python3 -c "print(-170 + ($run % 3400) / 10)")
lng2=$(python3 -c "print($lng + 0.01)")
square="[[1.0,$lng],[1.01,$lng],[1.01,$lng2],[1.0,$lng2],[1.0,$lng]]"
mid=$(python3 -c "print($lng + 0.005)")

echo "Smoke testing $API_URL" >&2
call GET /healthz 200 >/dev/null

geofence=$(call POST /geofences 201 "{\"name\":\"Smoke $run\",\"description\":\"smoke test\",\"coordinates\":$square,\"category\":\"restricted_zone\"}")
geofence_id=$(get "$geofence" 'd["id"]')
call POST /geofences 400 '{"name":"Bowtie","coordinates":[[0,0],[1,1],[1,0],[0,1],[0,0]],"category":"toll_zone"}' >/dev/null
listed=$(call GET "/geofences?category=restricted_zone" 200)
expect "new geofence listed" "$(get "$listed" "any(g['id'] == '$geofence_id' for g in d['geofences'])")" True

vehicle=$(call POST /vehicles 201 "{\"vehicle_number\":\"SMOKE-$run\",\"driver_name\":\"Smoke Test\",\"vehicle_type\":\"van\",\"phone\":\"+911234567890\"}")
vehicle_id=$(get "$vehicle" 'd["id"]')
call POST /vehicles 409 "{\"vehicle_number\":\"smoke-$run\",\"driver_name\":\"Dup\",\"vehicle_type\":\"van\",\"phone\":\"+911234567890\"}" >/dev/null
call GET /vehicles 200 >/dev/null

call POST /alerts/configure 201 "{\"geofence_id\":\"$geofence_id\",\"vehicle_id\":\"$vehicle_id\",\"event_type\":\"exit\"}" >/dev/null
rules=$(call GET "/alerts?geofence_id=$geofence_id" 200)
expect "rules for geofence" "$(get "$rules" 'len(d["alerts"])')" 1

# Listen on the WebSocket while the vehicle crosses the zone.
ws_log=$(mktemp)
if command -v node >/dev/null; then
  node -e '
    const ws = new WebSocket(process.argv[1]); const got = [];
    ws.onopen = () => console.error("ok  websocket connected");
    ws.onerror = () => { console.error("FAIL websocket error"); process.exit(1); };
    ws.onmessage = (e) => got.push(JSON.parse(e.data));
    setTimeout(() => { console.log(JSON.stringify(got)); process.exit(0); }, 6000);
  ' "$WS_URL" >"$ws_log" &
  ws_pid=$!
  sleep 2
fi

location() { echo "{\"vehicle_id\":\"$vehicle_id\",\"latitude\":$1,\"longitude\":$mid,\"timestamp\":\"$(ago "$2")\"}"; }
call POST /vehicles/location 200 "$(location 0.99 30)" >/dev/null
inside=$(call POST /vehicles/location 200 "$(location 1.005 20)")
expect "inside after entering" "$(get "$inside" "[g['geofence_id'] for g in d['current_geofences']] == ['$geofence_id']")" True
call POST /vehicles/location 200 "$(location 0.99 10)" >/dev/null
call POST /vehicles/location 404 "{\"vehicle_id\":\"veh_missing\",\"latitude\":1,\"longitude\":1,\"timestamp\":\"$(ago 5)\"}" >/dev/null

current=$(call GET "/vehicles/location/$vehicle_id" 200)
expect "current geofences after leaving" "$(get "$current" 'len(d["current_geofences"])')" 0
call GET /vehicles/location/veh_missing 404 >/dev/null

history=$(call GET "/violations/history?vehicle_id=$vehicle_id&limit=10" 200)
expect "history events (entry, exit)" "$(get "$history" '[v["event_type"] for v in reversed(d["violations"])]')" "['entry', 'exit']"
expect "history total_count" "$(get "$history" 'd["total_count"]')" 2

if [[ -n ${ws_pid:-} ]]; then
  wait "$ws_pid"
  received=$(cat "$ws_log")
  # Entry alerts via the restricted-zone default; exit alerts via the configured rule.
  expect "websocket alerts for this vehicle" \
    "$(get "$received" "[(m['event_type'], m['geofence']['category']) for m in d if m['vehicle']['vehicle_id'] == '$vehicle_id']")" \
    "[('entry', 'restricted_zone'), ('exit', 'restricted_zone')]"
else
  echo "skip websocket check (node not installed)" >&2
fi
rm -f "$ws_log"

echo "ALL CHECKS PASSED against $API_URL" >&2
