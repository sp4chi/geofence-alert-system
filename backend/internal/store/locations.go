package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"geofencing/internal/geo"
)

// querier is what both the pool and a transaction can do, so read helpers work inside or outside a tx.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// GeofenceRef is the short form of a geofence used in location responses and events.
type GeofenceRef struct {
	ID       string
	Name     string
	Category string
}

// LockedVehicle is a vehicle row held with SELECT ... FOR UPDATE for the rest of the transaction.
type LockedVehicle struct {
	ID            string
	VehicleNumber string
	DriverName    string
	LastSeenAt    *time.Time // nil until the first location update
}

// GeofenceEvent is one boundary crossing: a vehicle entered or exited a geofence.
type GeofenceEvent struct {
	ID         string
	VehicleID  string
	GeofenceID string
	EventType  string // "entry" or "exit"
	Location   geo.Point
	OccurredAt time.Time
}

// Tx is a transaction exposing only the queries the location-update use case needs.
type Tx struct {
	tx pgx.Tx
}

// InTx runs fn in a transaction: commit if fn returns nil, roll back otherwise.
func (s *Store) InTx(ctx context.Context, fn func(*Tx) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(&Tx{tx: tx})
	})
}

// LockVehicle loads the vehicle and locks its row until commit. A second update for the same
// vehicle waits here, so two concurrent updates can never both see the same "previous" state.
func (t *Tx) LockVehicle(ctx context.Context, vehicleID string) (LockedVehicle, error) {
	var v LockedVehicle
	err := t.tx.QueryRow(ctx, `
		SELECT id, vehicle_number, driver_name, last_seen_at
		FROM vehicles WHERE id = $1
		FOR UPDATE`, vehicleID,
	).Scan(&v.ID, &v.VehicleNumber, &v.DriverName, &v.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return LockedVehicle{}, vehicleNotFound(vehicleID)
	}
	if err != nil {
		return LockedVehicle{}, wrap("lock vehicle", err)
	}
	return v, nil
}

// InsertLocation appends the raw position to the history table.
func (t *Tx) InsertLocation(ctx context.Context, vehicleID string, p geo.Point, recordedAt time.Time) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO vehicle_locations (vehicle_id, location, recorded_at)
		VALUES ($1, ST_GeomFromText($2, 4326), $3)`,
		vehicleID, geo.PointWKT(p), recordedAt)
	return wrapIf("insert location", err)
}

// GeofencesCovering returns every active geofence that contains p, boundary included (ST_Covers).
// The GiST index on boundary narrows candidates by bounding box before the exact test.
func (t *Tx) GeofencesCovering(ctx context.Context, p geo.Point) ([]GeofenceRef, error) {
	return queryGeofenceRefs(ctx, t.tx, `
		SELECT id, name, category
		FROM geofences
		WHERE status = 'active' AND ST_Covers(boundary, ST_GeomFromText($1, 4326))
		ORDER BY id`, geo.PointWKT(p))
}

// CurrentGeofences returns the geofences the vehicle is recorded as being inside.
func (t *Tx) CurrentGeofences(ctx context.Context, vehicleID string) ([]GeofenceRef, error) {
	return currentGeofences(ctx, t.tx, vehicleID)
}

// InsertEvent records one crossing in geofence_events (the source for /violations/history).
func (t *Tx) InsertEvent(ctx context.Context, e GeofenceEvent) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO geofence_events (id, vehicle_id, geofence_id, event_type, location, occurred_at)
		VALUES ($1, $2, $3, $4, ST_GeomFromText($5, 4326), $6)`,
		e.ID, e.VehicleID, e.GeofenceID, e.EventType, geo.PointWKT(e.Location), e.OccurredAt)
	return wrapIf("insert geofence event", err)
}

// EnterGeofence adds the geofence to the vehicle's current set.
func (t *Tx) EnterGeofence(ctx context.Context, vehicleID, geofenceID string, at time.Time) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO vehicle_geofence_state (vehicle_id, geofence_id, entered_at)
		VALUES ($1, $2, $3)`, vehicleID, geofenceID, at)
	return wrapIf("enter geofence", err)
}

// ExitGeofence removes the geofence from the vehicle's current set.
func (t *Tx) ExitGeofence(ctx context.Context, vehicleID, geofenceID string) error {
	_, err := t.tx.Exec(ctx, `
		DELETE FROM vehicle_geofence_state WHERE vehicle_id = $1 AND geofence_id = $2`,
		vehicleID, geofenceID)
	return wrapIf("exit geofence", err)
}

// SetLastLocation stores the latest accepted position on the vehicle row.
func (t *Tx) SetLastLocation(ctx context.Context, vehicleID string, p geo.Point, at time.Time) error {
	_, err := t.tx.Exec(ctx, `
		UPDATE vehicles SET last_location = ST_GeomFromText($2, 4326), last_seen_at = $3
		WHERE id = $1`, vehicleID, geo.PointWKT(p), at)
	return wrapIf("set last location", err)
}

// VehicleLocation is the answer to GET /vehicles/location/{vehicle_id}.
type VehicleLocation struct {
	VehicleID     string
	VehicleNumber string
	Location      *geo.Point // nil until the first location update
	LastSeenAt    *time.Time
	Geofences     []GeofenceRef
}

// GetVehicleLocation reads the vehicle's latest accepted position and current geofences.
func (s *Store) GetVehicleLocation(ctx context.Context, vehicleID string) (VehicleLocation, error) {
	out := VehicleLocation{VehicleID: vehicleID}
	var location []byte
	err := s.pool.QueryRow(ctx, `
		SELECT vehicle_number, ST_AsGeoJSON(last_location), last_seen_at
		FROM vehicles WHERE id = $1`, vehicleID,
	).Scan(&out.VehicleNumber, &location, &out.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return VehicleLocation{}, vehicleNotFound(vehicleID)
	}
	if err != nil {
		return VehicleLocation{}, wrap("get vehicle location", err)
	}

	if location != nil {
		point, err := geo.PointFromGeoJSON(location)
		if err != nil {
			return VehicleLocation{}, wrap("decode last location", err)
		}
		out.Location = &point
	}

	out.Geofences, err = currentGeofences(ctx, s.pool, vehicleID)
	if err != nil {
		return VehicleLocation{}, err
	}
	return out, nil
}

func currentGeofences(ctx context.Context, q querier, vehicleID string) ([]GeofenceRef, error) {
	return queryGeofenceRefs(ctx, q, `
		SELECT g.id, g.name, g.category
		FROM vehicle_geofence_state s
		JOIN geofences g ON g.id = s.geofence_id
		WHERE s.vehicle_id = $1
		ORDER BY g.id`, vehicleID)
}

func queryGeofenceRefs(ctx context.Context, q querier, sql string, args ...any) ([]GeofenceRef, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, wrap("query geofences", err)
	}
	refs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (GeofenceRef, error) {
		var g GeofenceRef
		err := row.Scan(&g.ID, &g.Name, &g.Category)
		return g, err
	})
	if err != nil {
		return nil, wrap("scan geofences", err)
	}
	if refs == nil {
		refs = []GeofenceRef{} // API returns [] rather than null
	}
	return refs, nil
}

func vehicleNotFound(vehicleID string) error {
	return &domainError{kind: ErrNotFound, message: fmt.Sprintf("vehicle %q not found", vehicleID)}
}

func wrapIf(action string, err error) error {
	if err == nil {
		return nil
	}
	return wrap(action, err)
}
