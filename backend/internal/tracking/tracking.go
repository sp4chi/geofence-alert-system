// Package tracking is the location-update use case: given a new GPS point, work out which
// geofences the vehicle entered or exited and record it all atomically.
package tracking

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"geofencing/internal/alerts"
	"geofencing/internal/geo"
	"geofencing/internal/ids"
	"geofencing/internal/store"
)

const (
	EventEntry = "entry"
	EventExit  = "exit"

	restrictedZone = "restricted_zone"
)

// Publisher delivers alerts to live clients. Publish must not block (the hub's does not).
type Publisher interface {
	Publish(alerts.Message)
}

type Service struct {
	store     *store.Store
	publisher Publisher
	// restrictedDefault: entering a restricted_zone alerts even when no rule is configured.
	restrictedDefault bool
}

func New(st *store.Store, publisher Publisher, restrictedDefault bool) *Service {
	return &Service{store: st, publisher: publisher, restrictedDefault: restrictedDefault}
}

type Update struct {
	VehicleID  string
	Position   geo.Point
	RecordedAt time.Time
}

type Result struct {
	Vehicle store.LockedVehicle
	// Applied is false when the point was older than the last accepted one: it is stored in
	// history but does not move the vehicle or create events.
	Applied bool
	// Current is the set of geofences the vehicle is inside after this update.
	Current []store.GeofenceRef
	// Events lists every crossing this update caused.
	Events []store.GeofenceEvent
	// Alerts are the crossings that matched a rule (or the restricted default). They are stored
	// in the transaction and published only after it commits.
	Alerts []alerts.Message
}

// UpdateLocation runs the whole update in one transaction (see DESIGN.md §2.1):
// lock vehicle → store point → containment → diff with previous set → events → new state → commit.
func (s *Service) UpdateLocation(ctx context.Context, u Update) (Result, error) {
	// Postgres keeps microseconds; truncate now so comparisons with last_seen_at are exact.
	u.RecordedAt = u.RecordedAt.UTC().Truncate(time.Microsecond)

	var result Result
	err := s.store.InTx(ctx, func(tx *store.Tx) error {
		vehicle, err := tx.LockVehicle(ctx, u.VehicleID)
		if err != nil {
			return err
		}
		result = Result{Vehicle: vehicle}

		// Always keep the raw point, even a late one, so the movement history is complete.
		if err := tx.InsertLocation(ctx, u.VehicleID, u.Position, u.RecordedAt); err != nil {
			return err
		}

		previous, err := tx.CurrentGeofences(ctx, u.VehicleID)
		if err != nil {
			return err
		}

		// GPS devices buffer and resend. A point older than the last accepted one must not
		// change state, or it would create fake exits/entries. Report the unchanged state.
		if isStale(vehicle.LastSeenAt, u.RecordedAt) {
			result.Current = previous
			return nil
		}

		current, err := tx.GeofencesCovering(ctx, u.Position)
		if err != nil {
			return err
		}
		entered, exited := diff(previous, current)

		// Exits first: on a jump from A to B the history reads "left A, entered B".
		for _, g := range exited {
			if err := s.recordCrossing(ctx, tx, &result, u, g, EventExit); err != nil {
				return err
			}
			if err := tx.ExitGeofence(ctx, u.VehicleID, g.ID); err != nil {
				return err
			}
		}
		for _, g := range entered {
			if err := s.recordCrossing(ctx, tx, &result, u, g, EventEntry); err != nil {
				return err
			}
			if err := tx.EnterGeofence(ctx, u.VehicleID, g.ID, u.RecordedAt); err != nil {
				return err
			}
		}

		if err := tx.SetLastLocation(ctx, u.VehicleID, u.Position, u.RecordedAt); err != nil {
			return err
		}
		result.Applied = true
		result.Current = current
		return nil
	})
	if err != nil {
		return Result{}, fmt.Errorf("update location of %s: %w", u.VehicleID, err)
	}

	// Only after commit: if the transaction had rolled back, these crossings never happened
	// and nobody must be told about them. Publish is non-blocking, so the response isn't delayed.
	for _, msg := range result.Alerts {
		s.publisher.Publish(msg)
	}
	return result, nil
}

// recordCrossing stores one entry/exit event and, if it should alert, the alert with its payload.
func (s *Service) recordCrossing(ctx context.Context, tx *store.Tx, result *Result, u Update, g store.GeofenceRef, eventType string) error {
	event := store.GeofenceEvent{
		ID:         ids.New(ids.Violation),
		VehicleID:  u.VehicleID,
		GeofenceID: g.ID,
		EventType:  eventType,
		Location:   u.Position,
		OccurredAt: u.RecordedAt,
	}
	if err := tx.InsertEvent(ctx, event); err != nil {
		return err
	}
	result.Events = append(result.Events, event)

	ruleID, err := tx.MatchingRule(ctx, g.ID, u.VehicleID, eventType)
	if err != nil {
		return err
	}
	if !shouldAlert(ruleID != nil, eventType, g.Category, s.restrictedDefault) {
		return nil
	}

	msg := alerts.NewMessage(ids.New(ids.AlertEvent), event, result.Vehicle, g)
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encode alert payload: %w", err)
	}
	if err := tx.InsertAlertEvent(ctx, store.AlertEvent{
		ID:              msg.EventID,
		RuleID:          ruleID,
		GeofenceEventID: event.ID,
		Payload:         payload,
	}); err != nil {
		return err
	}
	result.Alerts = append(result.Alerts, msg)
	return nil
}

// shouldAlert: a matching rule always alerts. Without one, only an entry into a restricted zone
// does, and only while the restricted-zone default is switched on.
func shouldAlert(ruleMatched bool, eventType, category string, restrictedDefault bool) bool {
	if ruleMatched {
		return true
	}
	return restrictedDefault && eventType == EventEntry && category == restrictedZone
}

// isStale reports whether a point is older than the last accepted one.
// An equal timestamp is not stale: a resend of the same point is harmless (the diff is empty).
func isStale(lastSeenAt *time.Time, recordedAt time.Time) bool {
	return lastSeenAt != nil && recordedAt.Before(*lastSeenAt)
}

// diff compares where the vehicle was with where it is now:
// entered = current − previous, exited = previous − current. Output keeps the input order.
// On the first-ever update previous is empty, so every containing geofence counts as an entry.
func diff(previous, current []store.GeofenceRef) (entered, exited []store.GeofenceRef) {
	wasInside := make(map[string]bool, len(previous))
	for _, g := range previous {
		wasInside[g.ID] = true
	}
	isInside := make(map[string]bool, len(current))
	for _, g := range current {
		isInside[g.ID] = true
	}

	for _, g := range current {
		if !wasInside[g.ID] {
			entered = append(entered, g)
		}
	}
	for _, g := range previous {
		if !isInside[g.ID] {
			exited = append(exited, g)
		}
	}
	return entered, exited
}
