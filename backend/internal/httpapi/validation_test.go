package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var validSquare = [][]float64{{0, 0}, {0, 1}, {1, 1}, {1, 0}, {0, 0}}

func TestCreateGeofenceValidate(t *testing.T) {
	tests := []struct {
		name         string
		req          createGeofenceRequest
		wantProblems []string // substrings, in order; nil means valid
	}{
		{
			name: "valid",
			req:  createGeofenceRequest{Name: "Zone", Category: "toll_zone", Coordinates: validSquare},
		},
		{
			name:         "everything missing",
			req:          createGeofenceRequest{},
			wantProblems: []string{"name is required", "category is required", "coordinates is required"},
		},
		{
			name:         "whitespace name",
			req:          createGeofenceRequest{Name: "   ", Category: "toll_zone", Coordinates: validSquare},
			wantProblems: []string{"name is required"},
		},
		{
			name:         "unknown category",
			req:          createGeofenceRequest{Name: "Zone", Category: "parking", Coordinates: validSquare},
			wantProblems: []string{`category "parking" is invalid`},
		},
		{
			name:         "polygon problems are included",
			req:          createGeofenceRequest{Name: "Zone", Category: "toll_zone", Coordinates: [][]float64{{0, 0}, {0, 1}, {1, 1}, {1, 0}}},
			wantProblems: []string{"closed polygon"},
		},
		{
			name:         "name too long",
			req:          createGeofenceRequest{Name: strings.Repeat("a", 201), Category: "toll_zone", Coordinates: validSquare},
			wantProblems: []string{"name must be at most 200"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, problems := tt.req.validate()
			assertProblems(t, problems, tt.wantProblems)
			if tt.wantProblems == nil && len(input.Boundary) != len(tt.req.Coordinates) {
				t.Errorf("boundary not filled: %v", input.Boundary)
			}
		})
	}
}

func TestCreateGeofenceTrimsName(t *testing.T) {
	input, _ := createGeofenceRequest{Name: "  Zone A ", Category: "toll_zone", Coordinates: validSquare}.validate()
	if input.Name != "Zone A" {
		t.Errorf("name = %q", input.Name)
	}
}

func TestCreateVehicleValidate(t *testing.T) {
	valid := createVehicleRequest{VehicleNumber: "KA-01-AB-1234", DriverName: "John Doe", VehicleType: "truck", Phone: "+1234567890"}

	tests := []struct {
		name         string
		mutate       func(*createVehicleRequest)
		wantProblems []string
	}{
		{name: "valid", mutate: func(*createVehicleRequest) {}},
		{
			name:         "all missing",
			mutate:       func(r *createVehicleRequest) { *r = createVehicleRequest{} },
			wantProblems: []string{"vehicle_number is required", "driver_name is required", "vehicle_type is required", "phone is required"},
		},
		{name: "phone with separators", mutate: func(r *createVehicleRequest) { r.Phone = "+91 (98765) 43-210" }},
		{name: "phone with letters", mutate: func(r *createVehicleRequest) { r.Phone = "call me" }, wantProblems: []string{"phone must contain"}},
		{name: "phone too short", mutate: func(r *createVehicleRequest) { r.Phone = "12345" }, wantProblems: []string{"phone must contain"}},
		{name: "plus in the middle", mutate: func(r *createVehicleRequest) { r.Phone = "123+4567890" }, wantProblems: []string{"phone must contain"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := valid
			tt.mutate(&req)
			_, problems := req.validate()
			assertProblems(t, problems, tt.wantProblems)
		})
	}
}

func assertProblems(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("problems = %q, want %d matching %q", got, len(want), want)
	}
	for i := range want {
		if !strings.Contains(got[i], want[i]) {
			t.Errorf("problem[%d] = %q, want it to contain %q", i, got[i], want[i])
		}
	}
}

func TestDecodeJSONErrors(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"empty", "", "request body is empty"},
		{"syntax", `{"name": }`, "not valid JSON"},
		{"truncated", `{"name": "a"`, "ends too early"},
		{"wrong type", `{"name": 5}`, `field "name" must be a string, got number`},
		{"array body", `[1,2]`, "must be a JSON object"},
		{"two objects", `{} {}`, "single JSON object"},
		{"ok", `{"name": "a", "extra": true}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dst createGeofenceRequest
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			err := decodeJSON(httptest.NewRecorder(), req, &dst)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestDescribeGeometryProblem(t *testing.T) {
	if got := describeGeometryProblem("Self-intersection[0.5 0.5]"); !strings.Contains(got, "cross each other") {
		t.Errorf("got %q", got)
	}
}
