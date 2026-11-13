# AGENTS.md — Working Rules for AI Coding Agents

> Read this before touching any file in this repo. It applies to Claude Code, Cursor, Copilot, or any other agent.
> Companion docs: `README.md` (overview, architecture diagram) and `SETUP.md` (setup, API guide, behaviour and design decisions).

---

## 1. What this project is

A **geofencing and vehicle-tracking system** built for the MapUp Full-Stack Developer assessment.

- **Geofence**: a virtual boundary (a polygon drawn on a map). The system tracks when vehicles cross it.
- Users create geofences, register vehicles, and send location updates. When a vehicle **enters or exits** a geofence that has a matching alert rule (or enters a restricted zone), connected browsers get a **real-time alert**.

| Part        | Tech                                                                                     |
| ----------- | ---------------------------------------------------------------------------------------- |
| Backend API | Go (stdlib `net/http`, Go 1.22+ path patterns), `pgx` for Postgres                       |
| Database    | PostgreSQL 16 + PostGIS 3.4                                                              |
| Real-time   | WebSocket at `/ws/alerts` (`gorilla/websocket`)                                          |
| Frontend    | React + Vite + TypeScript, Tailwind, react-leaflet + leaflet-geoman, TanStack Query      |
| Infra       | Multi-stage Docker images, Docker Compose, API + Postgres on Render, frontend on Vercel  |

---

## 2. Hard constraints (never break these)

1. **API contract is frozen.** Paths, HTTP methods, JSON field names and response shapes must match the assessment brief, as documented in the API testing guide in `SETUP.md`. Do not rename, nest, camelCase, or add required fields.
   - `POST /geofences`, `GET /geofences?category=`
   - `POST /vehicles`, `GET /vehicles`
   - `POST /vehicles/location`, `GET /vehicles/location/{vehicle_id}`
   - `POST /alerts/configure`, `GET /alerts?geofence_id=&vehicle_id=`
   - `GET /violations/history?vehicle_id=&geofence_id=&start_date=&end_date=&limit=`
   - `GET /ws/alerts` (WebSocket upgrade)
2. **`time_ns` on every JSON response, including errors.** It is a **string** of nanoseconds (`"1234567"`), measured from the start of the handler to just before writing. Always go through the shared `respond` / `respondError` helpers; never call `json.NewEncoder(w)` directly in a handler.
3. **Coordinate order.** The API speaks `[latitude, longitude]`. PostGIS and GeoJSON speak `(longitude latitude)`. Conversion happens **only** in `internal/geo`. Never swap inline anywhere else.
4. **Alert delivery must not block the HTTP response.** The location update commits its transaction, hands alerts to the hub through a buffered channel, and returns. No network writes to WebSocket clients inside the request path.
5. **Every alert is persisted** (table `alert_events`) in the same transaction as its crossing, regardless of live delivery.
6. **Auth and rate limiting are optional and OFF by default** (`API_KEY`, `RATE_LIMIT_RPS`). Turning them on by default would break plain API calls.
7. **Readability.** Keep code small and explainable: readable functions with names that explain intent, no large opaque blocks. For non-obvious logic (state diff, locking, hub), add a 1–3 line comment saying _why_, not _what_.

---

## 3. Repository layout

```
geofence-alert-system/
├── README.md  SETUP.md  AGENTS.md
├── Makefile                    # single entry point for common tasks (`make help`)
├── docker-compose.yml          # db (PostGIS) + api + web (nginx serving the built frontend)
├── render.yaml                 # Render blueprint: API from the Docker Hub image + managed Postgres
├── .env.example
├── backend/
│   ├── cmd/api/main.go         # wiring only: config → db → migrations → hub → router → server
│   ├── internal/
│   │   ├── config/             # env parsing (PORT, DATABASE_URL, CORS_ORIGINS, API_KEY, RATE_LIMIT_RPS, ...)
│   │   ├── httpapi/            # router, thin handlers, validation, middleware (CORS, recover, logging, optional auth/rate limit), respond helpers
│   │   ├── geo/                # polygon/point validation, lat/lng ⇄ WKT/GeoJSON conversion
│   │   ├── store/              # Postgres queries, one file per aggregate; embedded-migration runner
│   │   ├── tracking/           # location-update use case: lock vehicle, containment, diff old/new sets, events, alerts
│   │   ├── alerts/             # WebSocket hub, client pumps, alert payload
│   │   └── ids/                # prefixed ID generation (geo_, veh_, alert_, viol_, evt_)
│   ├── migrations/             # numbered .sql files, embedded with go:embed and applied on startup
│   ├── scripts/                # simulate.sh (demo traffic), smoke.sh (end-to-end check of every endpoint + WebSocket)
│   └── Dockerfile              # multi-stage → static binary on distroless, non-root
└── frontend/
    ├── Dockerfile  nginx.conf  vercel.json
    └── src/
        ├── api/                # typed fetch client, one function per endpoint; base URL from VITE_API_URL
        ├── hooks/              # useAlertsSocket (reconnect with backoff + jitter), TanStack Query hooks
        ├── context/            # live alerts, map mode, toasts
        ├── components/         # AlertFeed, ConnectionPill, shared UI; map/ (MapView, GeofenceLayer, VehicleLayer, DrawControl)
        ├── lib/                # formatting and geometry helpers
        └── pages/              # Geofences, Vehicles, AlertRules, History
```

Dependency direction: `httpapi → tracking/alerts → store/geo`. `store` never imports `httpapi`. Handlers stay thin: parse → validate → call service → respond.

---

## 4. Commands

Use the Makefile; `make help` lists every target. Add a target whenever you add a new recurring command, and keep recipes thin (they call the same commands you'd type by hand).

| Target | Does |
|---|---|
| `make up` / `make down` | Build and start db + api + web (UI on :3000, API on :8080) / stop them (keeps data) |
| `make logs` | Follow API logs |
| `make db` + `make run` | Start only the database, then run the API locally with variables from `.env` |
| `make test` | `go test ./... -race` (always run with `-race` before committing concurrency code) |
| `make vet` / `make fmt` / `make fmt-check` | `go vet`, `gofmt -w`, fail if anything needs formatting |
| `make check` | fmt-check + vet + test + frontend build. Must pass before every commit |
| `make build` | Build the API binary into `backend/bin/` |
| `make fe-install` / `make fe-dev` / `make fe-build` | `npm ci` / Vite dev server on :5173 / production build |
| `make simulate VEHICLE=veh_... [GEOFENCE=geo_...]` | Drive a vehicle across a geofence (demo traffic) |
| `make smoke [API_URL=...]` | End-to-end check of every endpoint, error cases and the WebSocket, local or deployed |
| `make docker-build` / `make docker-push` | Build the backend image / build amd64+arm64 and push `$(IMAGE):$(TAG)` |
| `make psql` | `psql` inside the db container |
| `make reset-db CONFIRM=1` | `docker compose down -v` (deletes all data) |

Makefile rules: every target in `.PHONY`; recipe lines start with a **tab**; overridable variables use `?=`; `.env` is loaded with `-include .env` + `export`; it must keep working with the **GNU Make 3.81 that ships with macOS** (no `.ONESHELL`, chain steps with `cd backend && ...` on one line).

Environment variables (document any new one in `SETUP.md` and `.env.example`):
`PORT`, `DATABASE_URL`, `CORS_ORIGINS` (comma list), `API_KEY` (empty = auth off), `RATE_LIMIT_RPS` (0 = off), `RESTRICTED_ZONE_DEFAULT_ALERT` (default `true`), frontend `VITE_API_URL`, `VITE_WS_URL`, `VITE_API_KEY`.

---

## 5. Coding conventions

### Go

- Return errors, don't panic. Wrap with context: `fmt.Errorf("insert geofence: %w", err)`.
- Map domain errors to status codes in one place (`httpapi/errors.go`): validation → 400, not found → 404, duplicate → 409, everything else → 500 (log the detail, return a generic message).
- Error body shape: `{"error": "<human message>", "time_ns": "..."}`; validation errors may add `"details": [...]`.
- Pass `context.Context` from the request into every DB call.
- No global mutable state except the hub, which owns its client set in a single goroutine.
- The location update runs in one transaction and locks the vehicle row with `SELECT ... FOR UPDATE` (see "Behaviour and design decisions" in `SETUP.md`).
- Table-driven tests for: polygon validation, coordinate conversion, old/new set diff, rule matching.

### SQL

- Geofence column: `geometry(Polygon, 4326)` with a **GiST index**.
- Containment uses `ST_Covers(boundary, point)` (boundary points count as inside — documented decision).
- Validate shapes with `ST_IsValidReason` before insert; reject invalid polygons with a 400 and a plain-language reason.
- All timestamps `TIMESTAMPTZ`, stored and returned in UTC RFC 3339.
- Never change the statements of an applied migration; add a new numbered file instead.

### React

- Server state via TanStack Query; invalidate the relevant query after a mutation.
- Live alert feed in local state, capped, newest first.
- Every API call shows a loading state and a human-readable error.
- Memoize map layers (`React.memo`, `useMemo` for derived polygon arrays) so a new alert doesn't re-render every polygon.
- WebSocket hook: reconnect with exponential backoff + jitter, refetch on reconnect, show a connection status pill.

---

## 6. Definition of done for any task

- [ ] Matches the API contract (diff your JSON against the examples in `SETUP.md`).
- [ ] `time_ns` present on success **and** error paths.
- [ ] `make check` passes; `make smoke` passes for anything touching the API.
- [ ] New env vars documented in `SETUP.md` and `.env.example`.
- [ ] Short comment on any non-obvious decision; user-visible behaviour decisions go in `SETUP.md` → "Behaviour and design decisions".
- [ ] For a large change, summarise it in 3–5 bullets in the commit message.

## 7. Things to ask the human before doing

- Changing the DB schema (new migration) once the backend is deployed.
- Adding any new dependency heavier than a small library.
- Anything touching collaborator access, Docker Hub publishing, or deployment settings.
- Deleting files.

## 8. Commit style

`area: imperative summary` — e.g. `tracking: detect exits by diffing previous geofence set`. Small commits; one concern each. Never commit `.env`.
