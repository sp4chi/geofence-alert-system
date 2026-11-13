# Setup & usage guide

Geofencing and real-time vehicle alert system: a Go API backed by PostgreSQL + PostGIS, live alerts over WebSocket, and a React dashboard with an interactive map.

- [Quick start (Docker Compose)](#quick-start-docker-compose)
- [Prerequisites](#prerequisites)
- [Running the pieces locally](#running-the-pieces-locally)
- [Configuration](#configuration)
- [Make targets](#make-targets)
- [API testing guide](#api-testing-guide)
- [Frontend usage guide](#frontend-usage-guide)
- [Architecture overview](#architecture-overview)
- [Behaviour and design decisions](#behaviour-and-design-decisions)
- [Deployment](#deployment)

---

## Quick start (Docker Compose)

```bash
git clone <repo-url> geofence-alert-system && cd geofence-alert-system
docker compose up --build        # or: make up   (runs in the background)
```

| Service | URL | What it is |
|---|---|---|
| `web` | http://localhost:3000 | React dashboard (static build served by nginx) |
| `api` | http://localhost:8080 | Go REST API + WebSocket at `ws://localhost:8080/ws/alerts` |
| `db` | `localhost:5432` (user/password/db: `geofence`) | PostgreSQL 16 + PostGIS 3.4 |

The API creates the PostGIS extension and all tables on startup (embedded migrations), so there is nothing else to run. Check it with:

```bash
curl http://localhost:8080/healthz
# {"status":"ok","database":"ok","time_ns":"812345"}
```

Ports already in use? Override them: `API_HOST_PORT=9080 WEB_HOST_PORT=4000 DB_HOST_PORT=6432 docker compose up --build`.

> **Apple Silicon:** the official PostGIS image is amd64-only, so Compose runs it under emulation (`platform: linux/amd64`). It works, just a little slower to start.

---

## Prerequisites

| For | You need |
|---|---|
| Running everything | Docker Desktop (or Docker Engine) with Compose v2 |
| Backend development | Go 1.27+ |
| Frontend development | Node.js 22+ and npm |
| Scripts (`smoke.sh`, `simulate.sh`) | `curl`, `python3`; Node 22+ for the WebSocket check in `smoke.sh` |
| `make` targets (optional) | GNU Make (the 3.81 shipped with macOS works) |

---

## Running the pieces locally

Useful when changing code: run the database in Docker and the API / UI natively.

```bash
cp .env.example .env          # local settings, loaded by `make run`
make db                       # PostGIS only, on localhost:5432
make run                      # API on :8080 (go run ./cmd/api with .env)

make fe-install               # npm ci in frontend/
make fe-dev                   # Vite dev server on http://localhost:5173
```

Before committing: `make check` (gofmt check, `go vet`, `go test ./... -race`, frontend build).

---

## Configuration

All configuration is environment variables; the same image runs locally and in production.

**Backend (`backend/`)**

| Variable | Default | Meaning |
|---|---|---|
| `PORT` | `8080` | HTTP port (Render injects its own) |
| `DATABASE_URL` | — (required) | Postgres connection string, e.g. `postgres://geofence:geofence@localhost:5432/geofence?sslmode=disable` |
| `CORS_ORIGINS` | empty | Comma-separated browser origins allowed to call the API **and** open the WebSocket, e.g. `https://app.vercel.app`. `*` allows any. Requests without an `Origin` header (curl, scripts) are always allowed |
| `RESTRICTED_ZONE_DEFAULT_ALERT` | `true` | Entering a `restricted_zone` raises an alert even without a configured rule |
| `API_KEY` | empty (off) | When set, every endpoint except `/healthz` requires `X-API-Key: <key>` (or `Authorization: Bearer <key>`); the WebSocket also accepts `?api_key=<key>`. Missing/wrong key → 401 |
| `RATE_LIMIT_RPS` | `0` (off) | Per-client limit on `POST /vehicles/location` in requests per second (token bucket, bursts up to 2× the rate). Over the limit → 429 with `Retry-After` |

**Frontend (`frontend/`, read at build time)**

| Variable | Default | Meaning |
|---|---|---|
| `VITE_API_URL` | `http://localhost:8080` | Base URL of the API |
| `VITE_WS_URL` | derived from `VITE_API_URL` (`http→ws`, `https→wss`) + `/ws/alerts` | WebSocket URL |
| `VITE_API_KEY` | empty | Only when the API has `API_KEY` set; sent with every request. It is visible in the browser bundle |

**Compose host ports:** `DB_HOST_PORT` (5432), `API_HOST_PORT` (8080), `WEB_HOST_PORT` (3000).

**Optional hardening in Compose:** `API_KEY=demo RATE_LIMIT_RPS=5 docker compose up --build` turns on both (the UI is built with the same key).

---

## Make targets

`make` (or `make help`) lists them:

```text
  help          List all targets
  up            Build and start db + api + web (UI on :3000) in the background
  down          Stop all containers (keeps data)
  logs          Follow API logs
  db            Start only the database (for `make run`)
  run           Run the API locally with variables from .env
  test          Run Go tests with the race detector
  vet           Run go vet
  fmt           Format Go code
  fmt-check     Fail if any Go file needs formatting
  build         Build the API binary into backend/bin/
  check         Everything that must pass before a commit
  docker-build  Build the backend image for this machine as $(IMAGE):$(TAG)
  docker-push   Build amd64+arm64 and push $(IMAGE):$(TAG) and :latest to Docker Hub
  psql          Open psql inside the db container
  reset-db      Delete all data (docker compose down -v); needs CONFIRM=1
  simulate      Drive VEHICLE across a geofence: make simulate VEHICLE=veh_... [GEOFENCE=geo_...] [API_URL=...]
  smoke         Test every endpoint + WebSocket against API_URL (default local)
  fe-install    Install frontend dependencies (npm ci)
  fe-dev        Start the frontend dev server
  fe-build      Build the frontend
```

---

## API testing guide

Every response, success or error, includes `time_ns`: the handler's execution time in nanoseconds, as a string. Errors look like `{"error": "...", "details": ["..."], "time_ns": "..."}`.

Set the base URL once (use your deployed URL to test production):

```bash
API=http://localhost:8080
```

### Automated end-to-end check

```bash
make smoke                                  # local
make smoke API_URL=https://<your-api-host>  # deployed (also checks wss://)
```

`backend/scripts/smoke.sh` calls all nine endpoints plus error cases, checks every status code and `time_ns`, drives a vehicle in and out of a fresh restricted zone, and asserts that the history and the WebSocket alerts are right.

### 1. Create a geofence: `POST /geofences`

```bash
curl -s -X POST $API/geofences -H 'Content-Type: application/json' -d '{
  "name": "Downtown Delivery Zone",
  "description": "Main delivery area for downtown customers",
  "coordinates": [[37.7749,-122.4194],[37.7849,-122.4194],[37.7849,-122.4094],[37.7749,-122.4094],[37.7749,-122.4194]],
  "category": "delivery_zone"
}'
# 201 {"id":"geo_06gx...","name":"Downtown Delivery Zone","status":"active","time_ns":"3101234"}
```

Coordinates are `[latitude, longitude]`; the ring must be closed (first = last), have ≥ 4 points (3 distinct), be in range, and not cross itself. Every problem is reported:

```bash
curl -s -X POST $API/geofences -H 'Content-Type: application/json' \
  -d '{"name":"Bow-tie","coordinates":[[0,0],[1,1],[1,0],[0,1],[0,0]],"category":"toll_zone"}'
# 400 {"error":"invalid geofence","details":["polygon edges cross each other (self-intersecting shape)"],"time_ns":"..."}
```

Save a restricted zone to use below:

```bash
GEO=$(curl -s -X POST $API/geofences -H 'Content-Type: application/json' -d '{
  "name":"Airport No-Go","category":"restricted_zone",
  "coordinates":[[12.95,77.66],[12.97,77.66],[12.97,77.69],[12.95,77.69],[12.95,77.66]]
}' | python3 -c 'import sys,json; print(json.load(sys.stdin)["id"])')
```

### 2. List geofences: `GET /geofences?category=`

```bash
curl -s "$API/geofences?category=restricted_zone"
# 200 {"geofences":[{"id":"geo_...","name":"Airport No-Go","description":"","coordinates":[[12.95,77.66],...],"category":"restricted_zone","created_at":"2026-11-13T10:30:00Z"}],"time_ns":"..."}
```

### 3. Register a vehicle: `POST /vehicles`

```bash
VEH=$(curl -s -X POST $API/vehicles -H 'Content-Type: application/json' -d '{
  "vehicle_number":"KA-01-AB-1234","driver_name":"John Doe","vehicle_type":"truck","phone":"+1234567890"
}' | tee /dev/stderr | python3 -c 'import sys,json; print(json.load(sys.stdin)["id"])')
# 201 {"id":"veh_06gx...","vehicle_number":"KA-01-AB-1234","status":"active","time_ns":"..."}
```

The same number again (in any letter case) returns `409 {"error":"vehicle_number \"KA-01-AB-1234\" is already registered", ...}`.

### 4. List vehicles: `GET /vehicles`

```bash
curl -s $API/vehicles
# 200 {"vehicles":[{"id":"veh_...","vehicle_number":"KA-01-AB-1234","driver_name":"John Doe","vehicle_type":"truck","phone":"+1234567890","status":"active","created_at":"..."}],"time_ns":"..."}
```

### 5. Configure an alert rule: `POST /alerts/configure`

```bash
curl -s -X POST $API/alerts/configure -H 'Content-Type: application/json' \
  -d "{\"geofence_id\":\"$GEO\",\"vehicle_id\":\"$VEH\",\"event_type\":\"exit\"}"
# 201 {"alert_id":"alert_...","geofence_id":"geo_...","vehicle_id":"veh_...","event_type":"exit","status":"active","time_ns":"..."}
```

Omit `vehicle_id` for a rule that applies to all vehicles (`"vehicle_id": null` in responses). `event_type` is `entry`, `exit` or `both`. Unknown IDs → 404; an identical rule → 409.

### 6. List alert rules: `GET /alerts?geofence_id=&vehicle_id=`

```bash
curl -s "$API/alerts?geofence_id=$GEO"
# 200 {"alerts":[{"alert_id":"alert_...","geofence_id":"geo_...","geofence_name":"Airport No-Go","vehicle_id":"veh_...","vehicle_number":"KA-01-AB-1234","event_type":"exit","status":"active","created_at":"..."}],"time_ns":"..."}
```

### 7. Watch live alerts: `GET /ws/alerts` (WebSocket)

In a second terminal, before moving the vehicle:

```bash
# with websocat (brew install websocat)
websocat ws://localhost:8080/ws/alerts

# or with Node 22+ (no install needed)
node -e 'const ws=new WebSocket(process.argv[1]); ws.onmessage=e=>console.log(e.data)' ws://localhost:8080/ws/alerts
```

Each alert is pushed as:

```json
{"event_id":"evt_...","event_type":"entry","timestamp":"2026-11-13T10:35:00Z",
 "vehicle":{"vehicle_id":"veh_...","vehicle_number":"KA-01-AB-1234","driver_name":"John Doe"},
 "geofence":{"geofence_id":"geo_...","geofence_name":"Airport No-Go","category":"restricted_zone"},
 "location":{"latitude":12.96,"longitude":77.675}}
```

### 8. Send location updates: `POST /vehicles/location`

```bash
now() { date -u +%Y-%m-%dT%H:%M:%SZ; }
loc() { curl -s -X POST $API/vehicles/location -H 'Content-Type: application/json' \
  -d "{\"vehicle_id\":\"$VEH\",\"latitude\":$1,\"longitude\":$2,\"timestamp\":\"$(now)\"}"; echo; }

loc 12.94 77.675   # outside
loc 12.96 77.675   # inside → entry (alert: restricted-zone default)
# 200 {"vehicle_id":"veh_...","location_updated":true,"current_geofences":[{"geofence_id":"geo_...","geofence_name":"Airport No-Go","status":"inside"}],"time_ns":"..."}
loc 12.94 77.675   # outside → exit (alert: the exit rule from step 5)
```

Or let the script drive a vehicle straight across a zone: `make simulate VEHICLE=$VEH GEOFENCE=$GEO`.

### 9. Current location: `GET /vehicles/location/{vehicle_id}`

```bash
curl -s $API/vehicles/location/$VEH
# 200 {"vehicle_id":"veh_...","vehicle_number":"KA-01-AB-1234","current_location":{"latitude":12.94,"longitude":77.675,"timestamp":"..."},"current_geofences":[],"time_ns":"..."}
```

`current_location` is `null` until the first update; an unknown ID returns 404.

### 10. Entry/exit history: `GET /violations/history`

```bash
curl -s "$API/violations/history?vehicle_id=$VEH&limit=10"
# 200 {"violations":[{"id":"viol_...","vehicle_id":"veh_...","vehicle_number":"KA-01-AB-1234","geofence_id":"geo_...","geofence_name":"Airport No-Go","event_type":"exit","latitude":12.94,"longitude":77.675,"timestamp":"..."}, ...],"total_count":2,"time_ns":"..."}
```

Filters combine with AND: `vehicle_id`, `geofence_id`, `start_date`, `end_date` (RFC 3339, inclusive). `limit` defaults to 50; values above 500 are clamped to 500. `offset` is supported for paging; `total_count` counts all matches.

### Pagination on the other lists

`GET /geofences`, `GET /vehicles` and `GET /alerts` also accept optional `limit` (1–500, larger values clamped) and `offset`. Without them they return every item in exactly the README shape. With them they return that page plus `total_count`:

```bash
curl -s "$API/vehicles?limit=2&offset=0"
# 200 {"vehicles":[{...},{...}],"total_count":15,"time_ns":"..."}
```

### Optional API key and rate limit

Both are off unless configured (see [Configuration](#configuration)):

```bash
# API_KEY=demo
curl -s $API/vehicles                          # 401 {"error":"missing or invalid API key: send it in the X-API-Key header","time_ns":"..."}
curl -s -H 'X-API-Key: demo' $API/vehicles     # 200
websocat "ws://localhost:8080/ws/alerts?api_key=demo"

# RATE_LIMIT_RPS=2 → bursts of 4, then:
# 429 Retry-After: 1  {"error":"rate limit exceeded: too many location updates, retry shortly","time_ns":"..."}
```

---

## Frontend usage guide

Open http://localhost:3000 (Docker) or http://localhost:5173 (`make fe-dev`). The map stays on screen; the left panel switches between sections, and live alerts appear on the right (below the page on phones). The pill in the header shows whether the live connection is up (**Live**) or recovering (**Reconnecting…**).

1. **Geofences**: draw a zone with the polygon or rectangle tool on the map's left edge (click points, then click the first point to finish), or paste `[latitude, longitude]` pairs into the coordinates box. The dashed outline is exactly what will be saved. Pick a name and category and create it. The list below filters by category; zones are coloured by category on the map (restricted = red).
2. **Vehicles**: register a vehicle. Each one shows its last position and the zones it is in. Press **Set location**, then click the map (or type the position) and send. Leave the timestamp empty to use the current time. The result lists the geofences the vehicle is now inside.
3. **Alert rules**: choose a geofence, optionally a single vehicle (otherwise all vehicles), and entry / exit / both. Entering a restricted zone always alerts, even without a rule.
4. **Live alerts**: each alert raises a toast and is added to the feed with vehicle, driver, zone, category, time and position. Markers turn red while a vehicle is inside a restricted zone. Open the app in two tabs to see alerts arrive in both.
5. **History**: filter entry/exit events by vehicle, geofence and date range, and page through them. Filters are kept in the URL, so a filtered view can be bookmarked or shared.

---

## Architecture overview

```text
Browser (React, Vercel)            Go API (Docker, Render)                         PostgreSQL + PostGIS
───────────────────────            ──────────────────────────────────────────      ────────────────────
pages + TanStack Query ──REST──▶   middleware (log · recover · CORS)
                                   handlers (validate, time_ns) ──▶ tracking ──tx──▶ geofences (GiST)
                                                                   │  lock vehicle    vehicle_locations
useAlertsSocket ◀──wss /ws/alerts──  alerts hub ◀─ buffered chan ──┘  ST_Covers       vehicle_geofence_state
  (backoff + jitter)                (after commit, non-blocking)      diff old/new    geofence_events
                                                                      rules → alert   alert_rules · alert_events
```

**Backend (`backend/`)**: one Go binary with clear packages:

| Package | Responsibility |
|---|---|
| `cmd/api` | Wiring only: config → DB → migrations → hub → router → server, graceful shutdown |
| `internal/httpapi` | Routes (Go 1.22 patterns), thin handlers, validation, JSON + `time_ns` helpers, middleware, error → status mapping |
| `internal/tracking` | The location-update use case: one transaction that locks the vehicle row, finds covering geofences, diffs against the stored set, records events and alerts |
| `internal/alerts` | Alert message format and the WebSocket hub (single owner goroutine, per-client read/write pumps, ping/pong) |
| `internal/store` | All SQL (pgx); embedded migrations under an advisory lock |
| `internal/geo` | Polygon validation and the only `[lat, lng]` ⇄ PostGIS `(lng lat)` conversions |
| `internal/ids` | Prefixed, time-sortable IDs (`geo_`, `veh_`, `alert_`, `viol_`, `evt_`) |

**Location update path:** `BEGIN` → `SELECT … FOR UPDATE` on the vehicle → store the point → `ST_Covers(boundary, point)` (GiST-indexed) → entered = new − old, exited = old − new → insert `geofence_events` → match rules, insert `alert_events` → update `vehicle_geofence_state` → `COMMIT` → hand alerts to the hub without blocking → respond.

**Frontend (`frontend/`)**: React 19 + Vite + TypeScript + Tailwind. `api/` holds the typed client (one function per endpoint); server state is in TanStack Query; one app-wide WebSocket hook feeds toasts and the alert list; one shared Leaflet map (react-leaflet + leaflet-geoman for drawing) whose behaviour each page switches between idle, draw and pick.

---

## Behaviour and design decisions

| Topic | Behaviour | Why |
|---|---|---|
| Point on a boundary | Counts as **inside** (`ST_Covers`) | For restricted zones a false alert is safer than a missed one |
| First location of a vehicle | Every zone it is in counts as an **entry** | "Vehicle appeared inside a restricted zone" is what an operator wants to know |
| Restricted zones | Entry alerts even without a rule (`RESTRICTED_ZONE_DEFAULT_ALERT=true`) | The brief stresses restricted zones; switchable |
| Several rules match one crossing | **One** alert per crossing; the vehicle-specific rule is recorded | Clients would otherwise receive identical duplicates |
| Concurrent updates for one vehicle | Serialized by a row lock | Two updates can't both see the same previous state and double-fire |
| Late (older) timestamps | Stored in history, but don't change state or create events; response still `location_updated: true` | GPS devices buffer and resend; old points must not cause fake exits |
| Timestamps > 5 min in the future | Rejected (400) | One bad device clock would otherwise freeze that vehicle's state |
| Overlapping zones / jump between zones | One event per zone; a jump gives exit(s) then entry in one update | Exact set difference |
| Zone created while a vehicle is already inside | Entry fires on the vehicle's next update | State changes only on location updates |
| Alert delivery | Stored in the same transaction, pushed **after commit** through a buffered channel; the HTTP response never waits on clients | A rolled-back update must not alert; slow clients can't slow the API |
| Slow WebSocket client | Disconnected when its buffer fills; the UI reconnects and refetches history | One slow tab must not stall everyone |
| WebSocket origin | Checked against `CORS_ORIGINS` (CORS itself doesn't apply to WebSockets) | Stops other websites reading the alert stream |
| `POST` create endpoints | Return **201 Created** | Standard status for creation (the brief doesn't specify) |
| `vehicle_number` | Unique, case-insensitive | `ka-01` and `KA-01` are the same plate |
| History `limit` | Default 50, values > 500 clamped to 500; `offset` added | Matches the brief; offset enables paging |
| Pagination on other lists | Optional `limit`/`offset`; `total_count` only when paginating | Unpaginated responses keep exactly the README shape |
| API key / rate limit | Off by default; limit is per client IP (first `X-Forwarded-For` entry behind a proxy) on `POST /vehicles/location` only | Evaluator calls must work unchanged; location updates are the high-volume endpoint. Without a trusted proxy the IP header is spoofable, so the limit is about fairness, not security |

**Known limitations:** GPS jitter right on a boundary can flap in/out (fix: hysteresis); live delivery is in-memory, so an alert is stored but not pushed if the process dies between commit and send (fix: outbox pattern); one API instance per hub (fix: Redis/NATS pub-sub for multiple instances); no geofence update/delete endpoints (not in the brief); polygons crossing the antimeridian (±180°) are not supported.

---

## Deployment

**Backend image (Docker Hub):** [`sp4chi/mapup-geofencing-api`](https://hub.docker.com/r/sp4chi/mapup-geofencing-api), built for `linux/amd64` and `linux/arm64`:

```bash
docker pull sp4chi/mapup-geofencing-api:1.0
make docker-push TAG=1.1    # build both architectures and push (needs docker login)
```

**Backend (Render):** `render.yaml` describes the web service (the Docker Hub image, health check `/healthz`) and a Render Postgres 16 database; PostGIS is enabled by the API's first migration. Set `CORS_ORIGINS` to the frontend's URL. Free instances sleep after 15 minutes idle, so the first request after that takes ~50 s.

**Frontend (Vercel):** import the repository with **Root Directory** `frontend` (framework preset Vite; build `npm run build`, output `dist`) and set:

```text
VITE_API_URL=https://<your-api-host>
VITE_WS_URL=wss://<your-api-host>/ws/alerts     # optional; derived from VITE_API_URL if unset
```

`frontend/vercel.json` rewrites every route to `index.html` so deep links like `/history?vehicle_id=…` work. Vite fingerprints asset file names, so each deploy is picked up as soon as the page is reloaded.

**Verify a deployment:** `make smoke API_URL=https://<your-api-host>`.
