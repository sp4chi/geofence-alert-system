package geo

import (
	"reflect"
	"strings"
	"testing"
)

// README example square in San Francisco, as [lat, lng] pairs.
var square = [][]float64{
	{37.7749, -122.4194},
	{37.7849, -122.4194},
	{37.7849, -122.4094},
	{37.7749, -122.4094},
	{37.7749, -122.4194},
}

func TestParsePolygon(t *testing.T) {
	tests := []struct {
		name        string
		pairs       [][]float64
		wantProblem string // substring of the first problem; "" means valid
	}{
		{"valid square", square, ""},
		{"triangle is the minimum", [][]float64{{0, 0}, {0, 1}, {1, 1}, {0, 0}}, ""},
		{"empty", nil, "at least 4 points"},
		{"three points", [][]float64{{0, 0}, {0, 1}, {0, 0}}, "at least 4 points"},
		{"not closed", [][]float64{{0, 0}, {0, 1}, {1, 1}, {1, 0}}, "closed polygon"},
		{"pair with three numbers", [][]float64{{0, 0, 5}, {0, 1}, {1, 1}, {0, 0}}, "coordinates[0] must be [latitude, longitude]"},
		{"pair with one number", [][]float64{{0, 0}, {0}, {1, 1}, {0, 0}}, "coordinates[1] must be"},
		{"latitude too big", [][]float64{{0, 0}, {91, 1}, {1, 1}, {0, 0}}, "coordinates[1]: latitude 91"},
		{"longitude too small", [][]float64{{0, 0}, {0, -181}, {1, 1}, {0, 0}}, "coordinates[1]: longitude -181"},
		{"boundary values are allowed", [][]float64{{-90, -180}, {90, -180}, {90, 180}, {-90, -180}}, ""},
		{"all points identical", [][]float64{{1, 1}, {1, 1}, {1, 1}, {1, 1}}, "at least 3 distinct points"},
		{"collinear points", [][]float64{{0, 0}, {1, 1}, {2, 2}, {0, 0}}, "no area"},
		{"collinear with repeats", [][]float64{{0, 0}, {0, 0}, {1, 1}, {3, 3}, {0, 0}}, "no area"},
		// Zero net signed area but not a line: left for PostGIS to report as self-intersection.
		{"bow-tie passes Go checks", [][]float64{{10, 10}, {11, 11}, {11, 10}, {10, 11}, {10, 10}}, ""},
		{"only two distinct points", [][]float64{{0, 0}, {1, 1}, {0, 0}, {1, 1}, {0, 0}}, "at least 3 distinct points"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ring, problems := ParsePolygon(tt.pairs)
			if tt.wantProblem == "" {
				if len(problems) > 0 {
					t.Fatalf("unexpected problems: %v", problems)
				}
				if len(ring) != len(tt.pairs) {
					t.Fatalf("ring has %d points, want %d", len(ring), len(tt.pairs))
				}
				return
			}
			if ring != nil {
				t.Errorf("ring should be nil when invalid")
			}
			if len(problems) == 0 || !strings.Contains(problems[0], tt.wantProblem) {
				t.Errorf("problems = %v, want first containing %q", problems, tt.wantProblem)
			}
		})
	}
}

func TestParsePolygonReportsEveryBadPoint(t *testing.T) {
	_, problems := ParsePolygon([][]float64{{0, 0}, {95, 0}, {0, 200}, {0, 0}})
	if len(problems) != 2 {
		t.Errorf("want 2 problems, got %v", problems)
	}
}

func TestPolygonWKTPutsLongitudeFirst(t *testing.T) {
	ring, _ := ParsePolygon(square)
	want := "POLYGON((-122.4194 37.7749, -122.4194 37.7849, -122.4094 37.7849, -122.4094 37.7749, -122.4194 37.7749))"
	if got := PolygonWKT(ring); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestPointWKT(t *testing.T) {
	if got := PointWKT(Point{Lat: 12.5, Lng: 77.25}); got != "POINT(77.25 12.5)" {
		t.Errorf("got %s", got)
	}
}

func TestPolygonFromGeoJSONPutsLatitudeFirst(t *testing.T) {
	// What PostGIS ST_AsGeoJSON returns for the square: positions are [lng, lat].
	geojson := `{"type":"Polygon","coordinates":[[[-122.4194,37.7749],[-122.4194,37.7849],[-122.4094,37.7849],[-122.4094,37.7749],[-122.4194,37.7749]]]}`

	ring, err := PolygonFromGeoJSON([]byte(geojson))
	if err != nil {
		t.Fatal(err)
	}
	got := Pairs(ring)
	for i, pair := range square {
		if got[i] != [2]float64{pair[0], pair[1]} {
			t.Fatalf("point %d = %v, want %v (API order is [lat, lng])", i, got[i], pair)
		}
	}
}

func TestPolygonFromGeoJSONRejectsOtherShapes(t *testing.T) {
	for _, input := range []string{`{"type":"Point","coordinates":[1,2]}`, `{"type":"Polygon","coordinates":[]}`, `not json`} {
		if _, err := PolygonFromGeoJSON([]byte(input)); err == nil {
			t.Errorf("expected error for %s", input)
		}
	}
}

func TestPointFromGeoJSON(t *testing.T) {
	got, err := PointFromGeoJSON([]byte(`{"type":"Point","coordinates":[-122.4194,37.7849]}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := (Point{Lat: 37.7849, Lng: -122.4194}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if _, err := PointFromGeoJSON([]byte(`{"type":"Polygon","coordinates":[]}`)); err == nil {
		t.Error("expected error for non-point")
	}
}
