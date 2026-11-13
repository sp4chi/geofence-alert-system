package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"geofencing/internal/geo"
	"geofencing/internal/ids"
)

type Geofence struct {
	ID          string
	Name        string
	Description string
	Category    string
	Boundary    []geo.Point // closed ring, first == last
	Status      string
	CreatedAt   time.Time
}

type NewGeofence struct {
	Name        string
	Description string
	Category    string
	Boundary    []geo.Point
}

// CreateGeofence validates the shape with PostGIS, then inserts it.
func (s *Store) CreateGeofence(ctx context.Context, in NewGeofence) (Geofence, error) {
	wkt := geo.PolygonWKT(in.Boundary)

	// Ask PostGIS first so we can return the exact reason (e.g. self-intersection) as a 400,
	// rather than relying on the table's CHECK constraint, which only says "violates check".
	var reason string
	err := s.pool.QueryRow(ctx, `SELECT ST_IsValidReason(ST_GeomFromText($1, 4326))`, wkt).Scan(&reason)
	if err != nil {
		return Geofence{}, wrap("validate geofence shape", err)
	}
	if reason != "Valid Geometry" {
		return Geofence{}, &InvalidGeometryError{Reason: reason}
	}

	g := Geofence{
		ID:          ids.New(ids.Geofence),
		Name:        in.Name,
		Description: in.Description,
		Category:    in.Category,
		Boundary:    in.Boundary,
	}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO geofences (id, name, description, category, boundary)
		VALUES ($1, $2, $3, $4, ST_GeomFromText($5, 4326))
		RETURNING status, created_at`,
		g.ID, g.Name, g.Description, g.Category, wkt,
	).Scan(&g.Status, &g.CreatedAt)
	if err != nil {
		return Geofence{}, wrap("insert geofence", err)
	}
	return g, nil
}

// ListGeofences returns geofences newest first, plus the total matching count; category "" means
// no filter, and an empty Page means all of them.
func (s *Store) ListGeofences(ctx context.Context, category string, page Page) ([]Geofence, int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, description, category, ST_AsGeoJSON(boundary), status, created_at
		FROM geofences
		WHERE $1 = '' OR category = $1
		ORDER BY created_at DESC, id DESC
		LIMIT NULLIF($2, 0) OFFSET $3`, category, page.Limit, page.Offset)
	if err != nil {
		return nil, 0, wrap("list geofences", err)
	}
	geofences, err := scanGeofences(rows)
	if err != nil {
		return nil, 0, err
	}

	total := len(geofences)
	if page.Paginated() {
		err = s.pool.QueryRow(ctx, `SELECT count(*) FROM geofences WHERE $1 = '' OR category = $1`, category).Scan(&total)
		if err != nil {
			return nil, 0, wrap("count geofences", err)
		}
	}
	return geofences, total, nil
}

func scanGeofences(rows pgx.Rows) ([]Geofence, error) {
	defer rows.Close()

	geofences := []Geofence{} // non-nil so the API returns [] rather than null
	for rows.Next() {
		var (
			g        Geofence
			boundary []byte
		)
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.Category, &boundary, &g.Status, &g.CreatedAt); err != nil {
			return nil, wrap("scan geofence", err)
		}
		ring, err := geo.PolygonFromGeoJSON(boundary)
		if err != nil {
			return nil, wrap("decode boundary of "+g.ID, err)
		}
		g.Boundary = ring
		geofences = append(geofences, g)
	}
	if err := rows.Err(); err != nil {
		return nil, wrap("list geofences", err)
	}
	return geofences, nil
}
