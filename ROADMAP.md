# ROADMAP.md — MapUp Geofencing Assessment

> **Deadline:** 11:59 PM IST, **Fri 13 Nov 2026** · **Target submit:** 10:00 PM Fri (2 h buffer)
> **Rule:** a smaller system that works end to end beats a bigger one that half works. Finish every 🟥 must-have before any 🟨 nice-to-have.
> Details of _why_ live in `DESIGN.md`; agent rules in `AGENT.md`.

Legend: 🟥 must-have · 🟨 nice-to-have / bonus · ⏱ time box · ✅ exit check (don't move on until true)

---

## Phase 0 — Setup

- [ ] 🟥 Private GitHub repo `mapup-geofencing` with `backend/`, `frontend/`, `AGENT.md`, `ROADMAP.md`, `README.md`
- [x] 🟥 `docker-compose.yml` with `postgis/postgis:16-3.4` (healthcheck) + api service
- [x] 🟥 Go module, `cmd/api/main.go`, config from env, `/healthz`
- [x] 🟥 `respond` / `respondError` helpers with `time_ns` (string)
- ✅ `docker compose up` → DB healthy, `curl /healthz` returns JSON with `time_ns`

## Phase 1 — Schema + geofences + vehicles

- [x] 🟥 Migrations: all 7 tables, GiST index on `geofences.boundary`, B-tree indexes from DESIGN §3
- [x] 🟥 `internal/geo`: lat/lng ⇄ PostGIS conversion (one place), polygon validation + table tests
- [x] 🟥 `POST /geofences` (400 on every invalid case, `ST_IsValid`), `GET /geofences?category=`
- [x] 🟥 `POST /vehicles` (409 on duplicate number), `GET /vehicles`
- [x] 🟥 Middleware: recover, request log, CORS allow-list
- ✅ curl each endpoint; JSON shape diffed against README; invalid geofence → clear 400 with `time_ns`

## Phase 2 — Location updates + entry/exit

- [x] 🟥 `POST /vehicles/location`: tx → `FOR UPDATE` → insert location → `ST_Covers` → diff old/new → write `geofence_events` → update state → commit
- [x] 🟥 Handle first update, overlapping zones, late timestamps
- [x] 🟥 `GET /vehicles/location/{vehicle_id}` (404 if unknown)
- [x] 🟥 `GET /violations/history` with all filters, `limit` (default 50, max 500), `offset`, `total_count`
- [x] 🟥 Unit tests: set diff, validation; `go test ./... -race`
- ✅ Scripted run: outside → inside → inside → outside gives exactly 1 entry + 1 exit in history

## Phase 3 — Alert rules + WebSocket

- [x] 🟥 `POST /alerts/configure`, `GET /alerts?geofence_id=&vehicle_id=` (with names joined)
- [x] 🟥 Rule matching: vehicle-specific or all (NULL), entry/exit/both; restricted-zone default; dedupe
- [x] 🟥 Persist `alert_events`; publish **after commit**, non-blocking
- [x] 🟥 Hub + read/write pumps + ping/pong at `/ws/alerts`; message format exactly as README
- ✅ `websocat` in two terminals both receive the same alert; HTTP response time unaffected

> 💤 **Sleep by ~2:30 AM.** A tired demo costs more than a missing bonus.

## Phase 4 — Docker + deploy backend

- [x] 🟥 Multi-stage `Dockerfile` (small final image, non-root), `.dockerignore`
- [x] 🟥 Push image to Docker Hub (`sp4chi/mapup-geofencing-api:1.0` (amd64+arm64)) — public image, private repo
- [x] 🟥 Deploy (Railway/Render/Fly) + Postgres with PostGIS enabled; `CORS_ORIGINS` set; HTTPS/WSS works — Render: https://geofence-alert-system-ytl8.onrender.com (built from repo `backend/`), smoke passes over HTTPS/WSS
- [x] 🟥 `scripts/simulate.sh` drives a vehicle in and out of a zone
- ✅ curl suite + `wss://…/ws/alerts` work against the **deployed** URL — run `make smoke API_URL=https://…`

## Phase 5 — Frontend

- [x] 🟥 Vite + React + TS + Tailwind; typed API client; `VITE_API_URL`/`VITE_WS_URL`
- [x] 🟥 Map: geofence polygons by category colour, vehicle markers
- [x] 🟥 Create geofence: draw on map **and** coordinate input; category filter list
- [x] 🟥 Vehicles: register form, list with current location + geofence status
- [x] 🟥 Location update: click map or inputs; show current geofences
- [x] 🟥 Alert rules: form (geofence, optional vehicle, event type) + list
- [x] 🟥 Live alerts: `useAlertsSocket` with backoff, toasts, feed, connection pill
- [x] 🟥 History: filters (vehicle, geofence, date range) + pagination
- [x] 🟥 Loading + error states everywhere; check at mobile width
- [~] 🟨 Memoized map layers ✔; virtualized history table — not needed (server pagination, 25 rows/page)
- ✅ Two browser tabs both show the toast when simulate.sh crosses a boundary

## Phase 6 — Ship + docs

- [x] 🟥 Deploy frontend to Vercel; deployed FE ↔ deployed BE verified — https://frontend-ivory-delta-38.vercel.app (2 tabs Live, alert toast + feed from a Render crossing)
- [x] 🟥 `SETUP.md`: prerequisites, local setup, Docker Compose, curl examples for all 9 endpoints + WS, frontend usage, architecture summary, documented decisions (boundary, first update, restricted default)
- [x] 🟥 `SETUP.md`: add a "Make targets" section (paste `make help` output) — required by AGENTS.md §4.1
- [x] 🟥 Fresh-clone test: `git clone` into new folder → `docker compose up` works
- [x] 🟨 Bonus (only if all 🟥 green): rate limit on `/vehicles/location` (429 + `time_ns`), optional API key, pagination on all lists, more unit tests — all off by default; verified on/off locally
- ✅ "Before you submit" list from the Prep Guide all ticked

## Phase 7 — Video (record ~45 min, upload unlisted)

- [ ] 🟥 Demo 10–12 min: draw geofence → register vehicle → move in/out → live alert in 2 tabs → history filters → `docker compose up` locally
- [ ] 🟥 Code 12–18 min: structure, `geo` conversion, `ST_Covers` + GiST, tracking diff + row lock, hub, schema, `useAlertsSocket`
- [ ] 🟥 Gaps & roadmap 10–15 min: honest gaps (DESIGN §6) + scaling plan (DESIGN §8) + estimates (below)
- [ ] 🟥 Link in README

## Phase 8 — Submit

- [ ] 🟥 Add all 9 collaborators: vedantp, ajayap, jyotis, tanishqs, naveenn, asijitp, dewanshp, karkuvelpandip, parask (@mapup.ai)
- [ ] 🟥 Docker Hub image pullable; both deployments up
- [ ] 🟥 Google Form submitted ✉️

---

## Cut list (drop in this order if behind)

1. 🟨 API key auth → 2. 🟨 rate limiting → 3. 🟨 virtualized table → 4. drawing tool (keep coordinate input) → 5. vehicle-location map click (keep inputs).
   **Never cut:** exact endpoints, `time_ns`, entry/exit correctness, live alerts in the UI, deployment, SETUP.md, video.

---

## Post-submission roadmap (for the video's "Gaps & Roadmap" section)

| Priority | Item                                                                               | Why                                                                      | Estimate |
| -------- | ---------------------------------------------------------------------------------- | ------------------------------------------------------------------------ | -------- |
| P0       | Hysteresis for GPS jitter (N consecutive readings / buffer distance)               | Stops in-out flapping at edges                                           | 0.5 day  |
| P0       | Outbox pattern for alerts                                                          | Guarantees every stored alert gets published even if the process crashes | 1 day    |
| P0       | Integration tests against real PostGIS (testcontainers) + CI                       | Confidence on SQL + concurrency                                          | 1 day    |
| P1       | Geofence update/delete + recompute state                                           | Real operators edit zones                                                | 1 day    |
| P1       | Auth (JWT) + per-org scoping of geofences/alerts                                   | Multi-tenant                                                             | 2 days   |
| P1       | Keyset pagination + monthly partitions for history                                 | Fast at millions of rows                                                 | 1 day    |
| P1       | Observability: metrics (p95 latency, WS clients, drops), structured logs, tracing  | Run it in production                                                     | 1 day    |
| P2       | In-memory R-tree geofence cache + LISTEN/NOTIFY invalidation                       | Remove heaviest query from hot path                                      | 1.5 days |
| P2       | Redis/NATS pub-sub for multi-instance WebSockets                                   | Horizontal scaling                                                       | 1 day    |
| P2       | Queue-based ingestion partitioned by vehicle_id                                    | 1k+ updates/s with ordering                                              | 2–3 days |
| P3       | Circle geofences, schedules (zone active only at certain hours), dwell-time alerts | Product features                                                         | 2 days   |
| P3       | Alert channels: SMS / email / webhook                                              | Reach drivers and managers                                               | 1–2 days |

---

## Progress log

| When       | Done                               | Blockers / notes |
| ---------- | ---------------------------------- | ---------------- |
| Thu 12 Nov | Plan, AGENT/DESIGN/ROADMAP written |                  |
| Thu 12 Nov | Phase 0: compose (PostGIS amd64, emulated), config, respond helpers, /healthz | GitHub repo not created yet |
| Thu 12 Nov | Phase 1: schema (7 tables) + embedded migrator, geo package, geofence & vehicle endpoints, middleware | Creates return 201 |
| Fri 13 Nov | Phase 2: tracking service (lock → contain → diff → events → state), vehicle location GET, violation history; exit check + overlap/jump/late/concurrency verified | |
| Fri 13 Nov | Makefile (AGENTS §4.1). Phase 3: alert rules API, rule matching + restricted default in the location tx, alert_events, WS hub with pumps/ping; 2 listeners verified, origin check, clean shutdown | Docker Hub user still needed for `IMAGE` |
| Fri 13 Nov | Phase 4: cross-compiling multi-arch Dockerfile, image pushed to Docker Hub, simulate.sh + smoke.sh (`make simulate` / `make smoke`), render.yaml | Render deploy needs the user's dashboard login |
| Fri 13 Nov | Phase 5: React app (map + draw, vehicles + click-to-locate, alert rules, live feed/toasts/pill, history with URL filters + pagination); verified in headless Chrome incl. 390 px; fixed bow-tie message bug in `geo` | Frontend not deployed yet (Phase 6) |
| Fri 13 Nov | Phase 6 (local): web container (nginx) in compose, SETUP.md + README.md, SETUP walkthrough run as written, fresh clone on custom ports passes smoke + UI; fixed CORS when WEB_HOST_PORT changes | Waiting on Render URL + Vercel import; 38+ commits not pushed |
| Fri 13 Nov | Render deploy live: `make smoke` passes over HTTPS + WSS; latest code (bow-tie fix) confirmed; CORS allows localhost:5173 | Add Vercel URL to Render `CORS_ORIGINS` after frontend deploy |
| Fri 13 Nov | Vercel live; CORS fixed on Render; production verified in browser (2 tabs, live alert) | CORS normalization reverted (exact origin set in Render) |
| Fri 13 Nov | Bonus: token-bucket rate limit, optional API key (FE sends VITE_API_KEY), optional limit/offset + total_count on all lists, tests; smoke passes with defaults | Not pushed/deployed yet |
|            |                                    |                  |
