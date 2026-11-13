package ids

import (
	"strings"
	"testing"
	"time"
)

func TestNewFormat(t *testing.T) {
	id := New(Geofence)
	if !strings.HasPrefix(id, "geo_") || len(id) != len("geo_")+26 {
		t.Errorf("unexpected id %q", id)
	}
}

func TestNewIsUnique(t *testing.T) {
	seen := make(map[string]bool)
	for range 10000 {
		id := New(Vehicle)
		if seen[id] {
			t.Fatalf("duplicate id %s", id)
		}
		seen[id] = true
	}
}

func TestNewSortsByTime(t *testing.T) {
	first := New(Violation)
	time.Sleep(2 * time.Millisecond)
	second := New(Violation)
	if first >= second {
		t.Errorf("expected %s < %s", first, second)
	}
}
