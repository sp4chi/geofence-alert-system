// Typed API client: one function per endpoint. Every request goes through `request`, which turns
// network failures and API error bodies ({ error, details }) into an ApiError with a readable message.
import type {
  AlertRule,
  Geofence,
  HistoryFilters,
  LocationUpdate,
  LocationUpdateResult,
  NewAlertRule,
  NewGeofence,
  NewVehicle,
  Vehicle,
  VehicleLocation,
  Violation,
} from './types'

// `||` (not `??`) so an empty value, e.g. `VITE_WS_URL=` in a Docker build arg, falls back too.
export const API_URL = (import.meta.env.VITE_API_URL || 'http://localhost:8080').replace(/\/$/, '')
export const WS_URL = import.meta.env.VITE_WS_URL || `${API_URL.replace(/^http/, 'ws')}/ws/alerts` // http→ws, https→wss

export class ApiError extends Error {
  status: number
  details: string[]

  constructor(message: string, status: number, details: string[] = []) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.details = details
  }
}

async function request<T>(method: 'GET' | 'POST', path: string, body?: unknown): Promise<T> {
  let response: Response
  try {
    response = await fetch(API_URL + path, {
      method,
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    throw new ApiError(`Can't reach the API at ${API_URL}. Check that the backend is running.`, 0)
  }

  const data = await response.json().catch(() => null)
  if (!response.ok) {
    throw new ApiError(data?.error ?? `Request failed with status ${response.status}`, response.status, data?.details ?? [])
  }
  return data as T
}

/** Builds "?a=1&b=2" from the defined, non-empty values. */
function queryString(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') search.set(key, String(value))
  }
  const text = search.toString()
  return text ? `?${text}` : ''
}

export const api = {
  createGeofence: (body: NewGeofence) =>
    request<{ id: string; name: string; status: string }>('POST', '/geofences', body),

  listGeofences: (category?: string) =>
    request<{ geofences: Geofence[] }>('GET', `/geofences${queryString({ category })}`).then((r) => r.geofences),

  createVehicle: (body: NewVehicle) =>
    request<{ id: string; vehicle_number: string; status: string }>('POST', '/vehicles', body),

  listVehicles: () => request<{ vehicles: Vehicle[] }>('GET', '/vehicles').then((r) => r.vehicles),

  updateLocation: (body: LocationUpdate) => request<LocationUpdateResult>('POST', '/vehicles/location', body),

  getVehicleLocation: (vehicleId: string) =>
    request<VehicleLocation>('GET', `/vehicles/location/${encodeURIComponent(vehicleId)}`),

  configureAlert: (body: NewAlertRule) => request<AlertRule>('POST', '/alerts/configure', body),

  listAlertRules: (filters: { geofence_id?: string; vehicle_id?: string }) =>
    request<{ alerts: AlertRule[] }>('GET', `/alerts${queryString(filters)}`).then((r) => r.alerts),

  violationHistory: (filters: HistoryFilters) =>
    request<{ violations: Violation[]; total_count: number }>('GET', `/violations/history${queryString({ ...filters })}`),
}
