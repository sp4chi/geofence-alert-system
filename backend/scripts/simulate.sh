#!/usr/bin/env bash
# Drive a vehicle straight across a geofence for demos: starts outside, passes through the middle,
# ends outside on the other side, so you see an entry and an exit (and live alerts, if configured).
#
# Usage: simulate.sh <vehicle_id> [geofence_id]
#   geofence_id defaults to the first restricted_zone (or the first geofence if there is none).
# Env:   API_URL (default http://localhost:8080), STEPS (default 8), DELAY seconds (default 1)
# Needs: curl, python3
set -euo pipefail

API_URL=${API_URL:-http://localhost:8080}
STEPS=${STEPS:-8}
DELAY=${DELAY:-1}
VEHICLE_ID=${1:?usage: simulate.sh <vehicle_id> [geofence_id]}
GEOFENCE_ID=${2:-}

# Pick the geofence and plan a west → east path across the middle of its bounding box.
# Prints one "lat lng" per line. API coordinates are [latitude, longitude].
path=$(curl -fsS "$API_URL/geofences" | python3 -c '
import json, sys
wanted, steps = sys.argv[1], int(sys.argv[2])
fences = json.load(sys.stdin)["geofences"]
if not fences:
    sys.exit("no geofences exist yet; create one first")
if wanted:
    matches = [g for g in fences if g["id"] == wanted]
    if not matches:
        sys.exit(f"geofence {wanted} not found")
    fence = matches[0]
else:
    fence = next((g for g in fences if g["category"] == "restricted_zone"), fences[0])
lats = [p[0] for p in fence["coordinates"]]
lngs = [p[1] for p in fence["coordinates"]]
mid_lat = (min(lats) + max(lats)) / 2
margin = (max(lngs) - min(lngs)) * 0.3
start, end = min(lngs) - margin, max(lngs) + margin
print("# %s (%s, %s)" % (fence["name"], fence["id"], fence["category"]), file=sys.stderr)
for i in range(steps):
    print(f"{mid_lat:.6f} {start + (end - start) * i / (steps - 1):.6f}")
' "$GEOFENCE_ID" "$STEPS")

while read -r lat lng; do
  now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  body=$(printf '{"vehicle_id":"%s","latitude":%s,"longitude":%s,"timestamp":"%s"}' "$VEHICLE_ID" "$lat" "$lng" "$now")
  response=$(curl -sS -X POST "$API_URL/vehicles/location" -H 'Content-Type: application/json' -d "$body")
  inside=$(printf '%s' "$response" | python3 -c '
import json, sys
d = json.load(sys.stdin)
print("ERROR: " + d["error"] if "error" in d else ", ".join(g["geofence_name"] for g in d["current_geofences"]) or "-")')
  printf '%s  lat=%-11s lng=%-12s inside: %s\n' "$now" "$lat" "$lng" "$inside"
  sleep "$DELAY"
done <<< "$path"
