package tracking

import (
	"testing"
	"time"

	"geofencing/internal/store"
)

func refs(ids ...string) []store.GeofenceRef {
	out := make([]store.GeofenceRef, len(ids))
	for i, id := range ids {
		out[i] = store.GeofenceRef{ID: id}
	}
	return out
}

func idsOf(gs []store.GeofenceRef) []string {
	out := []string{}
	for _, g := range gs {
		out = append(out, g.ID)
	}
	return out
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name        string
		previous    []string
		current     []string
		wantEntered []string
		wantExited  []string
	}{
		{"first update inside one zone", nil, []string{"A"}, []string{"A"}, nil},
		{"first update outside everything", nil, nil, nil, nil},
		{"still inside", []string{"A"}, []string{"A"}, nil, nil},
		{"left the zone", []string{"A"}, nil, nil, []string{"A"}},
		{"jump from A to B in one update", []string{"A"}, []string{"B"}, []string{"B"}, []string{"A"}},
		{"entered overlapping zones together", nil, []string{"A", "B"}, []string{"A", "B"}, nil},
		{"left one of two overlapping zones", []string{"A", "B"}, []string{"B"}, nil, []string{"A"}},
		{"mixed", []string{"A", "B", "C"}, []string{"B", "D"}, []string{"D"}, []string{"A", "C"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entered, exited := diff(refs(tt.previous...), refs(tt.current...))
			assertIDs(t, "entered", idsOf(entered), tt.wantEntered)
			assertIDs(t, "exited", idsOf(exited), tt.wantExited)
		})
	}
}

func assertIDs(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s = %v, want %v", label, got, want)
		}
	}
}

func TestShouldAlert(t *testing.T) {
	tests := []struct {
		name              string
		ruleMatched       bool
		eventType         string
		category          string
		restrictedDefault bool
		want              bool
	}{
		{"rule matched on any zone", true, EventExit, "delivery_zone", false, true},
		{"no rule, ordinary zone", false, EventEntry, "delivery_zone", true, false},
		{"no rule, restricted entry, default on", false, EventEntry, "restricted_zone", true, true},
		{"no rule, restricted exit, default on", false, EventExit, "restricted_zone", true, false},
		{"no rule, restricted entry, default off", false, EventEntry, "restricted_zone", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldAlert(tt.ruleMatched, tt.eventType, tt.category, tt.restrictedDefault); got != tt.want {
				t.Errorf("shouldAlert = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsStale(t *testing.T) {
	last := time.Date(2025, 1, 15, 10, 35, 0, 0, time.UTC)

	tests := []struct {
		name       string
		lastSeenAt *time.Time
		recordedAt time.Time
		want       bool
	}{
		{"first ever update", nil, last, false},
		{"newer point", &last, last.Add(time.Second), false},
		{"same timestamp resent", &last, last, false},
		{"older point arrives late", &last, last.Add(-time.Microsecond), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isStale(tt.lastSeenAt, tt.recordedAt); got != tt.want {
				t.Errorf("isStale = %v, want %v", got, tt.want)
			}
		})
	}
}
