-- One rule per (geofence, vehicle-or-all, event type). COALESCE is needed because NULLs never
-- collide in a UNIQUE index, so two "all vehicles" rules would otherwise both be allowed.
CREATE UNIQUE INDEX alert_rules_unique_idx
    ON alert_rules (geofence_id, COALESCE(vehicle_id, ''), event_type);

-- Live delivery is best-effort and in-memory (clients backfill from history on reconnect);
-- counting deliveries would need a DB write per broadcast, so the column is dropped.
ALTER TABLE alert_events DROP COLUMN delivered_count;
