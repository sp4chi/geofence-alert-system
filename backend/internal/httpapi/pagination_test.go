package httpapi

import (
	"encoding/json"
	"net/url"
	"testing"

	"geofencing/internal/store"
)

func TestParsePage(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		defaultLimit int
		want         store.Page
		wantProblems []string
	}{
		{name: "lists return everything by default", query: "", defaultLimit: 0, want: store.Page{}},
		{name: "history default", query: "", defaultLimit: 50, want: store.Page{Limit: 50}},
		{name: "limit and offset", query: "limit=10&offset=20", want: store.Page{Limit: 10, Offset: 20}},
		{name: "offset alone", query: "offset=5", want: store.Page{Offset: 5}},
		{name: "clamped to 500", query: "limit=9999", want: store.Page{Limit: 500}},
		{name: "zero limit", query: "limit=0", wantProblems: []string{"positive integer"}},
		{name: "both invalid", query: "limit=x&offset=-2", wantProblems: []string{"positive integer", "non-negative"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, _ := url.ParseQuery(tt.query)
			page, problems := parsePage(query, tt.defaultLimit)
			assertProblems(t, problems, tt.wantProblems)
			if tt.wantProblems == nil && page != tt.want {
				t.Errorf("page = %+v, want %+v", page, tt.want)
			}
		})
	}
}

// Without ?limit/?offset the list responses must keep exactly the README shape (no total_count).
func TestTotalCountOnlyWhenPaginated(t *testing.T) {
	plain, _ := json.Marshal(listVehiclesResponse{Vehicles: []vehicleJSON{}, TotalCount: totalIfPaginated(store.Page{}, 3)})
	if string(plain) != `{"vehicles":[]}` {
		t.Errorf("unpaginated = %s", plain)
	}
	paged, _ := json.Marshal(listVehiclesResponse{Vehicles: []vehicleJSON{}, TotalCount: totalIfPaginated(store.Page{Limit: 2}, 3)})
	if string(paged) != `{"vehicles":[],"total_count":3}` {
		t.Errorf("paginated = %s", paged)
	}
}
