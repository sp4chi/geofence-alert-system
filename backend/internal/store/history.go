package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"geofencing/internal/geo"
)

// HistoryFilter narrows /violations/history. Empty / nil fields mean "no filter"; filters combine with AND.
type HistoryFilter struct {
	VehicleID  string
	GeofenceID string
	Start      *time.Time // inclusive
	End        *time.Time // inclusive
	Limit      int
	Offset     int
}

// Violation is one geofence crossing joined with the vehicle and geofence names.
type Violation struct {
	ID            string
	VehicleID     string
	VehicleNumber string
	GeofenceID    string
	GeofenceName  string
	EventType     string
	Location      geo.Point
	OccurredAt    time.Time
}

// ListViolations returns one page of crossings (newest first) and the total count matching the filter.
func (s *Store) ListViolations(ctx context.Context, f HistoryFilter) ([]Violation, int, error) {
	where, args := historyWhere(f)

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM geofence_events e `+where, args...).Scan(&total); err != nil {
		return nil, 0, wrap("count violations", err)
	}

	pageArgs := append(args, f.Limit, f.Offset)
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT e.id, e.vehicle_id, v.vehicle_number, e.geofence_id, g.name,
		       e.event_type, ST_AsGeoJSON(e.location), e.occurred_at
		FROM geofence_events e
		JOIN vehicles v ON v.id = e.vehicle_id
		JOIN geofences g ON g.id = e.geofence_id
		%s
		ORDER BY e.occurred_at DESC, e.id DESC
		LIMIT $%d OFFSET $%d`, where, len(args)+1, len(args)+2), pageArgs...)
	if err != nil {
		return nil, 0, wrap("list violations", err)
	}

	violations, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Violation, error) {
		var (
			v        Violation
			location []byte
		)
		if err := row.Scan(&v.ID, &v.VehicleID, &v.VehicleNumber, &v.GeofenceID, &v.GeofenceName,
			&v.EventType, &location, &v.OccurredAt); err != nil {
			return v, err
		}
		var err error
		v.Location, err = geo.PointFromGeoJSON(location)
		return v, err
	})
	if err != nil {
		return nil, 0, wrap("scan violations", err)
	}
	if violations == nil {
		violations = []Violation{}
	}
	return violations, total, nil
}

// historyWhere builds the WHERE clause shared by the count and the page query.
// Values only ever go in as $n placeholders, never into the SQL text, so this is injection-safe.
func historyWhere(f HistoryFilter) (string, []any) {
	var (
		conditions []string
		args       []any
	)
	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}

	if f.VehicleID != "" {
		add("e.vehicle_id = $%d", f.VehicleID)
	}
	if f.GeofenceID != "" {
		add("e.geofence_id = $%d", f.GeofenceID)
	}
	if f.Start != nil {
		add("e.occurred_at >= $%d", *f.Start)
	}
	if f.End != nil {
		add("e.occurred_at <= $%d", *f.End)
	}

	if len(conditions) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}
