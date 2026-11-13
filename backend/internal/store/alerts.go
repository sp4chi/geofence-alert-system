package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"geofencing/internal/ids"
)

type NewAlertRule struct {
	GeofenceID string
	VehicleID  *string // nil = applies to every vehicle
	EventType  string  // entry, exit or both
}

// AlertRule is a rule joined with the names the API shows next to the IDs.
type AlertRule struct {
	ID            string
	GeofenceID    string
	GeofenceName  string
	VehicleID     *string
	VehicleNumber *string
	EventType     string
	Status        string
	CreatedAt     time.Time
}

// CreateAlertRule inserts a rule. Unknown geofence/vehicle → ErrNotFound; identical rule → ErrDuplicate.
// The foreign keys do the existence check, so this is one round trip.
func (s *Store) CreateAlertRule(ctx context.Context, in NewAlertRule) (AlertRule, error) {
	rule := AlertRule{
		ID:         ids.New(ids.AlertRule),
		GeofenceID: in.GeofenceID,
		VehicleID:  in.VehicleID,
		EventType:  in.EventType,
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO alert_rules (id, geofence_id, vehicle_id, event_type)
		VALUES ($1, $2, $3, $4)
		RETURNING status, created_at`,
		rule.ID, rule.GeofenceID, rule.VehicleID, rule.EventType,
	).Scan(&rule.Status, &rule.CreatedAt)

	var pgErr *pgconn.PgError
	switch {
	case err == nil:
		return rule, nil
	case errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "alert_rules_geofence_id_fkey":
		return AlertRule{}, &domainError{kind: ErrNotFound, message: fmt.Sprintf("geofence %q not found", in.GeofenceID)}
	case errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "alert_rules_vehicle_id_fkey":
		return AlertRule{}, vehicleNotFound(*in.VehicleID)
	case isUniqueViolation(err, "alert_rules_unique_idx"):
		return AlertRule{}, &domainError{kind: ErrDuplicate, message: "an identical alert rule already exists"}
	default:
		return AlertRule{}, wrap("insert alert rule", err)
	}
}

// ListAlertRules returns rules newest first. Filters are exact matches; "" means no filter.
// (vehicle_id=X does not include "all vehicles" rules — those have vehicle_id null.)
func (s *Store) ListAlertRules(ctx context.Context, geofenceID, vehicleID string) ([]AlertRule, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.geofence_id, g.name, r.vehicle_id, v.vehicle_number, r.event_type, r.status, r.created_at
		FROM alert_rules r
		JOIN geofences g ON g.id = r.geofence_id
		LEFT JOIN vehicles v ON v.id = r.vehicle_id
		WHERE ($1 = '' OR r.geofence_id = $1)
		  AND ($2 = '' OR r.vehicle_id = $2)
		ORDER BY r.created_at DESC, r.id DESC`, geofenceID, vehicleID)
	if err != nil {
		return nil, wrap("list alert rules", err)
	}
	rules, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AlertRule, error) {
		var r AlertRule
		err := row.Scan(&r.ID, &r.GeofenceID, &r.GeofenceName, &r.VehicleID, &r.VehicleNumber, &r.EventType, &r.Status, &r.CreatedAt)
		return r, err
	})
	if err != nil {
		return nil, wrap("scan alert rules", err)
	}
	if rules == nil {
		rules = []AlertRule{}
	}
	return rules, nil
}

// MatchingRule returns the ID of an active rule for this crossing, or nil if none matches.
// A rule for this specific vehicle wins over an "all vehicles" rule, so the stored rule_id is the most specific.
func (t *Tx) MatchingRule(ctx context.Context, geofenceID, vehicleID, eventType string) (*string, error) {
	var ruleID string
	err := t.tx.QueryRow(ctx, `
		SELECT id FROM alert_rules
		WHERE status = 'active'
		  AND geofence_id = $1
		  AND (vehicle_id = $2 OR vehicle_id IS NULL)
		  AND event_type IN ($3, 'both')
		ORDER BY vehicle_id IS NULL, created_at
		LIMIT 1`, geofenceID, vehicleID, eventType,
	).Scan(&ruleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, wrap("match alert rule", err)
	}
	return &ruleID, nil
}

// AlertEvent is a stored alert: the exact payload pushed to clients, linked to its crossing.
type AlertEvent struct {
	ID              string
	RuleID          *string // nil when raised by the restricted-zone default
	GeofenceEventID string
	Payload         []byte // JSON
}

// InsertAlertEvent stores an alert in the same transaction as its crossing ("store all alerts").
func (t *Tx) InsertAlertEvent(ctx context.Context, a AlertEvent) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO alert_events (id, rule_id, geofence_event_id, payload)
		VALUES ($1, $2, $3, $4)`, a.ID, a.RuleID, a.GeofenceEventID, a.Payload)
	return wrapIf("insert alert event", err)
}
