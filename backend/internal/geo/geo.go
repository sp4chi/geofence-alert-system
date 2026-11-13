// Package geo is the only place that knows about both coordinate orders.
// The API speaks [latitude, longitude]; WKT, GeoJSON and PostGIS speak (longitude latitude).
// Every conversion between the two goes through a function in this file — never swap inline elsewhere.
package geo

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Point is a WGS 84 position (SRID 4326).
type Point struct {
	Lat float64
	Lng float64
}

const (
	MinPolygonPoints = 4 // 3 unique corners + the closing point
	MaxPolygonPoints = 10000
)

// ValidatePoint returns a human-readable problem for each out-of-range coordinate.
func ValidatePoint(p Point) []string {
	var problems []string
	if p.Lat < -90 || p.Lat > 90 {
		problems = append(problems, fmt.Sprintf("latitude %v is out of range [-90, 90]", p.Lat))
	}
	if p.Lng < -180 || p.Lng > 180 {
		problems = append(problems, fmt.Sprintf("longitude %v is out of range [-180, 180]", p.Lng))
	}
	return problems
}

// ParsePolygon turns API pairs [[lat, lng], ...] into a closed ring.
// It collects every problem instead of stopping at the first, so a client can fix them all in one go.
// Self-intersection is not checked here; PostGIS ST_IsValid does that (see store.CreateGeofence).
func ParsePolygon(pairs [][]float64) ([]Point, []string) {
	if len(pairs) < MinPolygonPoints {
		return nil, []string{fmt.Sprintf(
			"coordinates must have at least %d points (3 unique + closing point), got %d", MinPolygonPoints, len(pairs))}
	}
	if len(pairs) > MaxPolygonPoints {
		return nil, []string{fmt.Sprintf("coordinates must have at most %d points, got %d", MaxPolygonPoints, len(pairs))}
	}

	var problems []string
	ring := make([]Point, 0, len(pairs))
	for i, pair := range pairs {
		if len(pair) != 2 {
			problems = append(problems, fmt.Sprintf("coordinates[%d] must be [latitude, longitude], got %d numbers", i, len(pair)))
			continue
		}
		p := Point{Lat: pair[0], Lng: pair[1]}
		for _, msg := range ValidatePoint(p) {
			problems = append(problems, fmt.Sprintf("coordinates[%d]: %s", i, msg))
		}
		ring = append(ring, p)
	}
	if len(problems) > 0 {
		return nil, problems
	}

	if ring[0] != ring[len(ring)-1] {
		problems = append(problems, "first and last coordinates must be identical (closed polygon)")
	}
	if countDistinct(ring) < 3 {
		problems = append(problems, "polygon must have at least 3 distinct points")
	} else if allCollinear(ring) {
		// PostGIS would also reject this, but calls it "self-intersection", which confuses users.
		problems = append(problems, "polygon has no area (all points lie on one straight line)")
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return ring, nil
}

func countDistinct(ring []Point) int {
	seen := make(map[Point]struct{}, len(ring))
	for _, p := range ring {
		seen[p] = struct{}{}
	}
	return len(seen)
}

// allCollinear reports whether every point lies on the line through the first two distinct points.
// (A signed-area test is not enough: a symmetric "bow-tie" also has zero net area, but it is a
// self-intersection, which PostGIS reports with the right message.)
func allCollinear(ring []Point) bool {
	a := ring[0]
	var b Point
	for _, p := range ring[1:] {
		if p != a {
			b = p
			break
		}
	}
	for _, p := range ring {
		// Cross product of (b − a) and (p − a) is zero exactly when p is on line ab.
		if (b.Lng-a.Lng)*(p.Lat-a.Lat)-(b.Lat-a.Lat)*(p.Lng-a.Lng) != 0 {
			return false
		}
	}
	return true
}

// Pairs converts a ring back to the API shape [[lat, lng], ...].
func Pairs(ring []Point) [][2]float64 {
	pairs := make([][2]float64, len(ring))
	for i, p := range ring {
		pairs[i] = [2]float64{p.Lat, p.Lng}
	}
	return pairs
}

// PolygonWKT renders a ring as WKT for ST_GeomFromText: "POLYGON((lng lat, lng lat, ...))".
func PolygonWKT(ring []Point) string {
	var b strings.Builder
	b.WriteString("POLYGON((")
	for i, p := range ring {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(lngLat(p))
	}
	b.WriteString("))")
	return b.String()
}

// PointWKT renders a point as WKT for ST_GeomFromText: "POINT(lng lat)".
func PointWKT(p Point) string {
	return "POINT(" + lngLat(p) + ")"
}

func lngLat(p Point) string {
	// 'f' with -1 precision prints the shortest exact representation, so values round-trip unchanged.
	return strconv.FormatFloat(p.Lng, 'f', -1, 64) + " " + strconv.FormatFloat(p.Lat, 'f', -1, 64)
}

// PolygonFromGeoJSON parses ST_AsGeoJSON output for a polygon and returns its outer ring.
// GeoJSON positions are [lng, lat].
func PolygonFromGeoJSON(data []byte) ([]Point, error) {
	var shape struct {
		Type        string         `json:"type"`
		Coordinates [][][2]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal(data, &shape); err != nil {
		return nil, fmt.Errorf("decode polygon geojson: %w", err)
	}
	if shape.Type != "Polygon" || len(shape.Coordinates) == 0 {
		return nil, errors.New("geojson is not a polygon with an outer ring")
	}

	outer := shape.Coordinates[0] // we never store holes, so ring 0 is the whole boundary
	ring := make([]Point, len(outer))
	for i, position := range outer {
		ring[i] = Point{Lat: position[1], Lng: position[0]}
	}
	return ring, nil
}

// PointFromGeoJSON parses ST_AsGeoJSON output for a point. GeoJSON positions are [lng, lat].
func PointFromGeoJSON(data []byte) (Point, error) {
	var shape struct {
		Type        string     `json:"type"`
		Coordinates [2]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal(data, &shape); err != nil {
		return Point{}, fmt.Errorf("decode point geojson: %w", err)
	}
	if shape.Type != "Point" {
		return Point{}, fmt.Errorf("geojson type %q is not a point", shape.Type)
	}
	return Point{Lat: shape.Coordinates[1], Lng: shape.Coordinates[0]}, nil
}
