// Types mirror the API contract in the assessment README exactly (snake_case field names).
// Coordinates are [latitude, longitude] — the same order Leaflet uses, so no swapping in the UI.

export type LatLng = [number, number]

export const CATEGORIES = ['delivery_zone', 'restricted_zone', 'toll_zone', 'customer_area'] as const
export type Category = (typeof CATEGORIES)[number]

export interface Geofence {
  id: string
  name: string
  description: string
  coordinates: LatLng[]
  category: Category
  created_at: string
}

export interface NewGeofence {
  name: string
  description: string
  coordinates: LatLng[]
  category: Category
}

export interface Vehicle {
  id: string
  vehicle_number: string
  driver_name: string
  vehicle_type: string
  phone: string
  status: string
  created_at: string
}

export interface NewVehicle {
  vehicle_number: string
  driver_name: string
  vehicle_type: string
  phone: string
}

export interface LocationUpdate {
  vehicle_id: string
  latitude: number
  longitude: number
  timestamp: string
}

export interface LocationUpdateResult {
  vehicle_id: string
  location_updated: boolean
  current_geofences: { geofence_id: string; geofence_name: string; status: string }[]
}

export interface VehicleLocation {
  vehicle_id: string
  vehicle_number: string
  current_location: { latitude: number; longitude: number; timestamp: string } | null
  current_geofences: { geofence_id: string; geofence_name: string; category: Category }[]
}

export const ALERT_EVENT_TYPES = ['entry', 'exit', 'both'] as const
export type AlertEventType = (typeof ALERT_EVENT_TYPES)[number]

export interface NewAlertRule {
  geofence_id: string
  vehicle_id?: string
  event_type: AlertEventType
}

export interface AlertRule {
  alert_id: string
  geofence_id: string
  geofence_name: string
  vehicle_id: string | null // null = all vehicles
  vehicle_number: string | null
  event_type: AlertEventType
  status: string
  created_at: string
}

export interface Violation {
  id: string
  vehicle_id: string
  vehicle_number: string
  geofence_id: string
  geofence_name: string
  event_type: 'entry' | 'exit'
  latitude: number
  longitude: number
  timestamp: string
}

export interface HistoryFilters {
  vehicle_id?: string
  geofence_id?: string
  start_date?: string
  end_date?: string
  limit: number
  offset: number
}

/** The message pushed on /ws/alerts. */
export interface AlertMessage {
  event_id: string
  event_type: 'entry' | 'exit'
  timestamp: string
  vehicle: { vehicle_id: string; vehicle_number: string; driver_name: string }
  geofence: { geofence_id: string; geofence_name: string; category: Category }
  location: { latitude: number; longitude: number }
}
