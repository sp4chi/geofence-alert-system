// Package alerts builds alert messages and delivers them to WebSocket clients through a hub.
package alerts

import (
	"time"

	"geofencing/internal/store"
)

// Message is exactly the WebSocket payload from the README.
type Message struct {
	EventID   string       `json:"event_id"`
	EventType string       `json:"event_type"`
	Timestamp string       `json:"timestamp"`
	Vehicle   VehicleInfo  `json:"vehicle"`
	Geofence  GeofenceInfo `json:"geofence"`
	Location  LocationInfo `json:"location"`
}

type VehicleInfo struct {
	VehicleID     string `json:"vehicle_id"`
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
}

type GeofenceInfo struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Category     string `json:"category"`
}

type LocationInfo struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// NewMessage assembles the payload for one crossing. eventID is the alert's own "evt_" ID.
func NewMessage(eventID string, event store.GeofenceEvent, vehicle store.LockedVehicle, geofence store.GeofenceRef) Message {
	return Message{
		EventID:   eventID,
		EventType: event.EventType,
		Timestamp: event.OccurredAt.UTC().Format(time.RFC3339),
		Vehicle: VehicleInfo{
			VehicleID:     vehicle.ID,
			VehicleNumber: vehicle.VehicleNumber,
			DriverName:    vehicle.DriverName,
		},
		Geofence: GeofenceInfo{
			GeofenceID:   geofence.ID,
			GeofenceName: geofence.Name,
			Category:     geofence.Category,
		},
		Location: LocationInfo{Latitude: event.Location.Lat, Longitude: event.Location.Lng},
	}
}
