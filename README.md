# Geofence Alert System

Real-time geofencing and vehicle tracking, built for the MapUp Full-Stack Developer assessment.

Draw zones on a map, register vehicles and stream their positions. The system works out every entry and exit with PostGIS, stores the full history, and pushes alerts to every open dashboard over WebSocket the moment a vehicle crosses into a restricted zone or matches an alert rule.

## Live deployment

| | URL |
|---|---|
| Web app (Vercel) | https://frontend-ivory-delta-38.vercel.app |
| API (Render) | https://geofence-alert-system-ytl8.onrender.com (health: [`/healthz`](https://geofence-alert-system-ytl8.onrender.com/healthz)) |
| WebSocket | `wss://geofence-alert-system-ytl8.onrender.com/ws/alerts` |
| Docker image | [`sp4chi/mapup-geofencing-api`](https://hub.docker.com/r/sp4chi/mapup-geofencing-api) |

The API runs on Render's free tier, which sleeps after 15 minutes without traffic: the first request after that can take about a minute while it wakes up.

## Run it

```bash
docker compose up --build
```

Then open http://localhost:3000 (UI), with the API at http://localhost:8080. Everything else (local development, configuration, curl examples for every endpoint, the WebSocket, design decisions and deployment) is in **[SETUP.md](SETUP.md)**.

## Stack

| Part      | Tech                                                                                 |
| --------- | ------------------------------------------------------------------------------------ |
| API       | Go (stdlib `net/http`), pgx, gorilla/websocket                                       |
| Database  | PostgreSQL 16 + PostGIS 3.4 (GiST-indexed `ST_Covers` containment)                   |
| Real-time | WebSocket hub at `/ws/alerts`, alerts persisted and published after commit           |
| Frontend  | React 19, Vite, TypeScript, Tailwind, react-leaflet + leaflet-geoman, TanStack Query |
| Infra     | Multi-stage Docker images, Docker Compose, Render (API + Postgres), Vercel (UI)      |

## Repository layout

```text
backend/    Go API: cmd/api (wiring), internal/{httpapi,tracking,alerts,store,geo,ids}, migrations/, scripts/
frontend/   React app: src/{api,hooks,context,components,pages}
SETUP.md    Setup, API guide, architecture, decisions
Makefile    Common tasks: `make help`
```

## Architecture

```mermaid
%% Started from https://gitdiagram.com/sp4chi/geofence-alert-system, corrected to match the code
flowchart TD

subgraph group_dashboard["Operator dashboard (React)"]
  node_app["Dashboard shell<br/>[App.tsx]"]
  node_map["Map and drawing<br/>[MapView.tsx]"]
  node_geofence_page["Geofence management<br/>[GeofencesPage.tsx]"]
  node_vehicle_page["Vehicle tracking<br/>[VehiclesPage.tsx]"]
  node_rules_page["Alert rules<br/>[AlertRulesPage.tsx]"]
  node_history_page["Violation history<br/>[HistoryPage.tsx]"]
  node_live_alerts["Live alert feed + toasts<br/>[liveAlerts.tsx]"]
  node_queries["Server-state queries<br/>[queries.ts]"]
  node_socket_hook["WebSocket client, backoff + jitter<br/>[useAlertsSocket.ts]"]
  node_api_client["Typed API client<br/>[client.ts]"]
end

subgraph group_api["HTTP layer (Go)"]
  node_api_routes["Routes + middleware<br/>[router.go]"]
  node_location_handler["POST /vehicles/location<br/>[locations.go]"]
  node_history_handler["GET /violations/history<br/>[httpapi/history.go]"]
  node_ws_handler["GET /ws/alerts upgrade + origin check<br/>[ws.go]"]
end

subgraph group_processing["Tracking and alerts"]
  node_tracking["Location update use case<br/>[tracking.go]"]
  node_geo["Validation + lat/lng ⇄ WKT/GeoJSON<br/>[geo.go]"]
  node_alert_message["Alert payload<br/>[message.go]"]
  node_alert_hub["WebSocket hub<br/>[hub.go]"]
end

subgraph group_state["Persistence"]
  node_store["SQL queries<br/>[store/]"]
  node_history_store["History query<br/>[store/history.go]"]
  node_db[("PostgreSQL + PostGIS<br/>GiST index on boundaries")]
end

node_operator(("Fleet operator"))

node_operator -->|"uses"| node_app
node_app -->|"renders"| node_map
node_app -->|"routes to"| node_geofence_page
node_app -->|"routes to"| node_vehicle_page
node_app -->|"routes to"| node_rules_page
node_app -->|"routes to"| node_history_page
node_app -->|"renders"| node_live_alerts
node_map -->|"loads map data"| node_queries
node_geofence_page -->|"mutates and reads"| node_queries
node_vehicle_page -->|"mutates and reads"| node_queries
node_rules_page -->|"mutates and reads"| node_queries
node_history_page -->|"loads history"| node_queries
node_queries -->|"calls endpoints"| node_api_client
node_api_client -->|"HTTPS + JSON"| node_api_routes

node_api_routes -->|"location update"| node_location_handler
node_location_handler -->|"validated update"| node_tracking
node_tracking -->|"lat/lng ⇄ WKT"| node_geo
node_tracking -->|"one transaction: lock vehicle, ST_Covers,<br/>diff old/new set, events, matched alerts, state"| node_store
node_store --> node_db
node_tracking -->|"builds"| node_alert_message
node_tracking -->|"after commit, non-blocking"| node_alert_hub

node_api_routes -->|"history query"| node_history_handler
node_history_handler --> node_history_store
node_history_store --> node_db

node_live_alerts -->|"opens connection"| node_socket_hook
node_socket_hook -->|"wss /ws/alerts"| node_ws_handler
node_ws_handler -->|"attach client"| node_alert_hub
node_alert_hub -->|"fan-out to every client"| node_socket_hook

click node_app "https://github.com/sp4chi/geofence-alert-system/blob/main/frontend/src/App.tsx"
click node_map "https://github.com/sp4chi/geofence-alert-system/blob/main/frontend/src/components/map/MapView.tsx"
click node_geofence_page "https://github.com/sp4chi/geofence-alert-system/blob/main/frontend/src/pages/GeofencesPage.tsx"
click node_vehicle_page "https://github.com/sp4chi/geofence-alert-system/blob/main/frontend/src/pages/VehiclesPage.tsx"
click node_rules_page "https://github.com/sp4chi/geofence-alert-system/blob/main/frontend/src/pages/AlertRulesPage.tsx"
click node_history_page "https://github.com/sp4chi/geofence-alert-system/blob/main/frontend/src/pages/HistoryPage.tsx"
click node_live_alerts "https://github.com/sp4chi/geofence-alert-system/blob/main/frontend/src/context/liveAlerts.tsx"
click node_queries "https://github.com/sp4chi/geofence-alert-system/blob/main/frontend/src/hooks/queries.ts"
click node_socket_hook "https://github.com/sp4chi/geofence-alert-system/blob/main/frontend/src/hooks/useAlertsSocket.ts"
click node_api_client "https://github.com/sp4chi/geofence-alert-system/blob/main/frontend/src/api/client.ts"
click node_api_routes "https://github.com/sp4chi/geofence-alert-system/blob/main/backend/internal/httpapi/router.go"
click node_location_handler "https://github.com/sp4chi/geofence-alert-system/blob/main/backend/internal/httpapi/locations.go"
click node_history_handler "https://github.com/sp4chi/geofence-alert-system/blob/main/backend/internal/httpapi/history.go"
click node_ws_handler "https://github.com/sp4chi/geofence-alert-system/blob/main/backend/internal/httpapi/ws.go"
click node_tracking "https://github.com/sp4chi/geofence-alert-system/blob/main/backend/internal/tracking/tracking.go"
click node_geo "https://github.com/sp4chi/geofence-alert-system/blob/main/backend/internal/geo/geo.go"
click node_alert_message "https://github.com/sp4chi/geofence-alert-system/blob/main/backend/internal/alerts/message.go"
click node_alert_hub "https://github.com/sp4chi/geofence-alert-system/blob/main/backend/internal/alerts/hub.go"
click node_store "https://github.com/sp4chi/geofence-alert-system/tree/main/backend/internal/store"
click node_history_store "https://github.com/sp4chi/geofence-alert-system/blob/main/backend/internal/store/history.go"
click node_db "https://github.com/sp4chi/geofence-alert-system/blob/main/backend/migrations/001_init.sql"

classDef toneBlue fill:#dbeafe,stroke:#2563eb,stroke-width:1.5px,color:#172554
classDef toneAmber fill:#fef3c7,stroke:#d97706,stroke-width:1.5px,color:#78350f
classDef toneMint fill:#dcfce7,stroke:#16a34a,stroke-width:1.5px,color:#14532d
classDef toneRose fill:#ffe4e6,stroke:#e11d48,stroke-width:1.5px,color:#881337
classDef toneIndigo fill:#e0e7ff,stroke:#4f46e5,stroke-width:1.5px,color:#312e81
class node_app,node_map,node_geofence_page,node_vehicle_page,node_rules_page,node_history_page,node_live_alerts,node_queries,node_socket_hook,node_api_client toneBlue
class node_api_routes,node_location_handler,node_history_handler,node_ws_handler toneAmber
class node_tracking,node_geo,node_alert_hub,node_alert_message toneMint
class node_store,node_history_store,node_db toneRose
class node_operator toneIndigo
```
