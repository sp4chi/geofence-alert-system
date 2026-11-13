package store

import (
	"context"
	"time"

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

// ListGeofences returns all geofences, newest first; category "" means no filter.
func (s *Store) ListGeofences(ctx context.Context, category string) ([]Geofence, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, description, category, ST_AsGeoJSON(boundary), status, created_at
		FROM geofences
		WHERE $1 = '' OR category = $1
		ORDER BY created_at DESC, id DESC`, category)
	if err != nil {
		return nil, wrap("list geofences", err)
	}
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
		if g.Boundary, err = geo.PolygonFromGeoJSON(boundary); err != nil {
			return nil, wrap("decode boundary of "+g.ID, err)
		}
		geofences = append(geofences, g)
	}
	if err := rows.Err(); err != nil {
		return nil, wrap("list geofences", err)
	}
	return geofences, nil
}
