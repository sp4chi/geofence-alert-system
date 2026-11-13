-- Initial schema. See "Behaviour and design decisions" in SETUP.md for the reasoning behind the tables and indexes.
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE geofences (
    id          TEXT PRIMARY KEY,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    category    TEXT        NOT NULL
        CHECK (category IN ('delivery_zone', 'restricted_zone', 'toll_zone', 'customer_area')),
    -- Safety net: the API checks ST_IsValid first to give a friendly 400, this guarantees it.
    boundary    geometry(Polygon, 4326) NOT NULL CHECK (ST_IsValid(boundary)),
    status      TEXT        NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- GiST stores each polygon's bounding box in a tree, so "which geofences contain this point"
-- only runs the exact test on the few candidates whose box contains it.
CREATE INDEX geofences_boundary_gist ON geofences USING GIST (boundary);
CREATE INDEX geofences_category_idx ON geofences (category);

CREATE TABLE vehicles (
    id             TEXT PRIMARY KEY,
    vehicle_number TEXT        NOT NULL,
    driver_name    TEXT        NOT NULL,
    vehicle_type   TEXT        NOT NULL,
    phone          TEXT        NOT NULL,
    status         TEXT        NOT NULL DEFAULT 'active',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Latest accepted position, so GET /vehicles/location/{id} is a single-row read.
    last_location  geometry(Point, 4326),
    last_seen_at   TIMESTAMPTZ
);
-- Case-insensitive: "ka-01-ab-1234" and "KA-01-AB-1234" are the same plate.
CREATE UNIQUE INDEX vehicles_vehicle_number_key ON vehicles (upper(vehicle_number));

-- Append-only raw history of every reported position.
CREATE TABLE vehicle_locations (
    id          BIGSERIAL PRIMARY KEY,
    vehicle_id  TEXT        NOT NULL REFERENCES vehicles (id),
    location    geometry(Point, 4326) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX vehicle_locations_vehicle_time_idx ON vehicle_locations (vehicle_id, recorded_at DESC);

-- Which geofences each vehicle is inside right now. Entry/exit = diff against this set.
CREATE TABLE vehicle_geofence_state (
    vehicle_id  TEXT        NOT NULL REFERENCES vehicles (id),
    geofence_id TEXT        NOT NULL REFERENCES geofences (id),
    entered_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (vehicle_id, geofence_id)
);

CREATE TABLE alert_rules (
    id          TEXT PRIMARY KEY,
    geofence_id TEXT        NOT NULL REFERENCES geofences (id),
    vehicle_id  TEXT REFERENCES vehicles (id), -- NULL = applies to every vehicle
    event_type  TEXT        NOT NULL CHECK (event_type IN ('entry', 'exit', 'both')),
    status      TEXT        NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX alert_rules_geofence_idx ON alert_rules (geofence_id);
CREATE INDEX alert_rules_vehicle_idx ON alert_rules (vehicle_id);

-- Every boundary crossing (a fact), whether or not any rule matched. Feeds /violations/history.
CREATE TABLE geofence_events (
    id          TEXT PRIMARY KEY,
    vehicle_id  TEXT        NOT NULL REFERENCES vehicles (id),
    geofence_id TEXT        NOT NULL REFERENCES geofences (id),
    event_type  TEXT        NOT NULL CHECK (event_type IN ('entry', 'exit')),
    location    geometry(Point, 4326) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX geofence_events_vehicle_time_idx ON geofence_events (vehicle_id, occurred_at DESC);
CREATE INDEX geofence_events_geofence_time_idx ON geofence_events (geofence_id, occurred_at DESC);
CREATE INDEX geofence_events_time_idx ON geofence_events (occurred_at DESC);

-- Crossings that matched a rule (or the restricted-zone default) and were pushed to clients.
CREATE TABLE alert_events (
    id                TEXT PRIMARY KEY,
    rule_id           TEXT REFERENCES alert_rules (id), -- NULL when raised by the restricted-zone default
    -- UNIQUE: one alert per crossing, even if several rules match it.
    geofence_event_id TEXT        NOT NULL UNIQUE REFERENCES geofence_events (id),
    payload           JSONB       NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_count   INT         NOT NULL DEFAULT 0
);
CREATE INDEX alert_events_created_idx ON alert_events (created_at DESC);
