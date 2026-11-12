# AGENT.md — Working Rules for AI Coding Agents

> Read this before touching any file in this repo. It applies to Claude Code, Cursor, Copilot, or any other agent.
> Companion docs: `DESIGN.md` (architecture, trade-offs) and `ROADMAP.md` (schedule and checklist).

---

## 1. What this project is

A **geofencing and vehicle-tracking system** built for the MapUp Full-Stack Developer assessment.

- **Geofence**: a virtual boundary (a polygon drawn on a map). The system tracks when vehicles cross it.
- Users create geofences, register vehicles, and send location updates. When a vehicle **enters or exits** a geofence that has a matching alert rule, connected browsers get a **real-time alert**.

| Part        | Tech                                                                                                |
| ----------- | --------------------------------------------------------------------------------------------------- |
| Backend API | Go (stdlib `net/http`, Go 1.22+ path patterns), `pgx` for Postgres                                  |
| Database    | PostgreSQL 16 + PostGIS 3                                                                           |
| Real-time   | WebSocket at `/ws/alerts` (`gorilla/websocket`)                                                     |
| Frontend    | React + Vite + TypeScript, Tailwind, react-leaflet + leaflet-geoman, TanStack Query                 |
| Infra       | Docker (multi-stage build), Docker Compose, backend on a Docker host with HTTPS, frontend on Vercel |

Working end-to-end beats feature-complete. Do not start nice-to-haves until every item in "must-have" in `ROADMAP.md` is green.

---

## 2. Hard constraints (never break these)

1. **API contract is frozen.** Paths, HTTP methods, JSON field names and response shapes must match `README.md` exactly. The evaluators test these strictly. Do not rename, nest, camelCase, or add required fields.
   - `POST /geofences`, `GET /geofences?category=`
   - `POST /vehicles`, `GET /vehicles`
   - `POST /vehicles/location`, `GET /vehicles/location/{vehicle_id}`
   - `POST /alerts/configure`, `GET /alerts?geofence_id=&vehicle_id=`
   - `GET /violations/history?vehicle_id=&geofence_id=&start_date=&end_date=&limit=`
   - `GET /ws/alerts` (WebSocket upgrade)
2. **`time_ns` on every JSON response — including errors.** It is a **string** of nanoseconds (`"1234567"`), measured from the start of the handler to just before writing. Always go through the shared `respond` / `respondError` helpers; never call `json.NewEncoder(w)` directly in a handler.
3. **Coordinate order.** The API speaks `[latitude, longitude]`. PostGIS and GeoJSON speak `(longitude latitude)`. Conversion happens **only** in `internal/geo` (one function each way). Never swap inline anywhere else.
4. **Alert delivery must not block the HTTP response.** The location handler commits the DB transaction, hands events to the hub through a buffered channel, and returns. No network writes to WebSocket clients inside the request path.
5. **Every alert is persisted** (table `alert_events`) before or regardless of delivery.
6. **Auth and rate limiting are optional and OFF by default** (env flags). Turning them on by default would break the evaluator's test calls.
7. **Originality.** The candidate must be able to explain every line in an interview. Do not paste large opaque blocks. Prefer small, readable functions with names that explain intent. When you write non-obvious logic (state diff, locking, hub), add a 1–3 line comment saying _why_, not _what_.

---

## 3. Repository layout

```
mapup/
├── AGENT.md  ROADMAP.md  SETUP.md  README.md
├── docker-compose.yml          # postgis + api (+ optional frontend dev)
├── backend/
│   ├── cmd/api/main.go         # wiring only: config → db → migrations → hub → router → server
│   ├── internal/
│   │   ├── config/             # env parsing (PORT, DATABASE_URL, CORS_ORIGINS, API_KEY, RATE_LIMIT_*)
│   │   ├── httpapi/            # router, handlers, middleware (CORS, recover, logging, optional auth/ratelimit), respond helpers
│   │   ├── geo/                # validation of polygons/points, lat/lng ⇄ WKT/GeoJSON conversion
│   │   ├── store/              # Postgres queries (one file per aggregate: geofences, vehicles, locations, rules, events)
│   │   ├── tracking/           # location-update use case: lock vehicle, containment, diff old/new sets, write events
│   │   ├── alerts/             # rule matching + WebSocket hub + client pumps
│   │   └── ids/                # prefixed ID generation (geo_, veh_, alert_, viol_, evt_)
│   ├── migrations/             # numbered .sql files, embedded with go:embed and applied on startup
│   ├── Dockerfile              # multi-stage → small final image
│   └── scripts/simulate.sh     # drives a vehicle across a geofence for the demo
└── frontend/
    └── src/
        ├── api/                # typed fetch client, one function per endpoint; base URL from VITE_API_URL
        ├── hooks/              # useAlertsSocket (reconnect with backoff), query hooks
        ├── components/map/     # MapView, GeofenceLayer, VehicleLayer, DrawControl
        ├── components/         # forms, tables, AlertFeed, toasts
        └── pages/              # Dashboard, Geofences, Vehicles, Alerts, History
```

Dependency direction: `httpapi → tracking/alerts → store/geo`. `store` never imports `httpapi`. Handlers stay thin: parse → validate → call service → respond.

---

## 4. Commands

```bash
# whole stack locally
docker compose up --build

# backend only (needs DB from compose)
cd backend && go run ./cmd/api
go test ./... -race          # ALWAYS run with -race before committing concurrency code
go vet ./...

# frontend
cd frontend && npm install && npm run dev
npm run build && npm run lint

# demo traffic
./backend/scripts/simulate.sh <vehicle_id>
```

Environment variables (document any new one in `SETUP.md` and `.env.example`):
`PORT`, `DATABASE_URL`, `CORS_ORIGINS` (comma list), `API_KEY` (empty = auth off), `RATE_LIMIT_RPS` (0 = off), `RESTRICTED_ZONE_DEFAULT_ALERT` (default `true`), frontend `VITE_API_URL`, `VITE_WS_URL`.

---

## 5. Coding conventions

### Go

- Return errors, don't panic. Wrap with context: `fmt.Errorf("insert geofence: %w", err)`.
- Map domain errors to status codes in one place: validation → 400, not found → 404, duplicate `vehicle_number` → 409, everything else → 500 (log the detail, return a generic message).
- Error body shape: `{"error": "<human message>", "time_ns": "..."}`; validation errors may add `"details": [...]`.
- Pass `context.Context` from the request into every DB call.
- No global mutable state except the hub (which owns its own lock/channels).
- Use transactions for the location update (see DESIGN.md §5). Lock the vehicle row with `SELECT ... FOR UPDATE`.
- Table-driven tests for: polygon validation, coordinate conversion, old/new set diff, rule matching.

### SQL

- Geofence column: `geometry(Polygon, 4326)` with a **GiST index**.
- Containment uses `ST_Covers(boundary, point)` (boundary points count as inside — documented decision).
- Validate shapes with `ST_IsValid` on insert; reject self-intersecting polygons with 400.
- All timestamps `TIMESTAMPTZ`, stored and returned in UTC RFC 3339.

### React

- Server state via TanStack Query; invalidate the relevant query after a mutation.
- Live alert feed in local state, capped (e.g. last 100), newest first.
- Every API call shows a loading state and a human-readable error.
- Memoize map layers (`React.memo`, `useMemo` for derived polygon arrays) so a new alert doesn't re-render every polygon.
- WebSocket hook: reconnect with exponential backoff + jitter, show a connection status pill.

---

## 6. Definition of done for any task

- [ ] Matches the README contract (diff your JSON against the README example).
- [ ] `time_ns` present on success **and** error paths.
- [ ] `go test ./... -race` and `go vet` pass; `npm run build` passes.
- [ ] New env vars documented.
- [ ] Short comment on any non-obvious decision; bigger decisions go in `DESIGN.md`.
- [ ] Candidate can explain it — if the change is large, summarise it in 3–5 bullets in the PR/commit message.

## 7. Things to ask the human before doing

- Changing the DB schema after the backend is deployed.
- Adding any new dependency heavier than a small library.
- Anything touching collaborator access, Docker Hub publishing, or deployment settings.
- Deleting files.

## 8. Commit style

`area: imperative summary` — e.g. `tracking: detect exits by diffing previous geofence set`. Small commits; one concern each. Never commit `.env`.
