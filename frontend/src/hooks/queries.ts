// Server state lives in TanStack Query. Keys are centralised so mutations and live alerts can
// invalidate exactly what changed.
import { keepPreviousData, useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'
import type { HistoryFilters, Vehicle, VehicleLocation } from '../api/types'

export const queryKeys = {
  geofences: (category?: string) => ['geofences', category ?? 'all'] as const,
  vehicles: ['vehicles'] as const,
  vehicleLocation: (vehicleId: string) => ['vehicle-location', vehicleId] as const,
  alertRules: (filters: { geofence_id?: string; vehicle_id?: string }) => ['alert-rules', filters] as const,
  history: (filters: HistoryFilters) => ['history', filters] as const,
}

export function useGeofences(category?: string) {
  return useQuery({ queryKey: queryKeys.geofences(category), queryFn: () => api.listGeofences(category) })
}

export function useVehicles() {
  return useQuery({ queryKey: queryKeys.vehicles, queryFn: api.listVehicles })
}

/**
 * Current location for every vehicle, keyed by vehicle ID. The API has one endpoint per vehicle,
 * so this fans out; polling every 10 s also catches updates sent by other clients (e.g. simulate.sh).
 */
export function useVehicleLocations(vehicles: Vehicle[] | undefined) {
  return useQueries({
    queries: (vehicles ?? []).map((vehicle) => ({
      queryKey: queryKeys.vehicleLocation(vehicle.id),
      queryFn: () => api.getVehicleLocation(vehicle.id),
      refetchInterval: 10_000,
    })),
    combine: combineLocations,
  })
}

// Defined outside the hook so TanStack can keep the combined result stable between renders.
function combineLocations(results: { data?: VehicleLocation }[]): Record<string, VehicleLocation> {
  const byId: Record<string, VehicleLocation> = {}
  for (const result of results) {
    if (result.data) byId[result.data.vehicle_id] = result.data
  }
  return byId
}

export function useAlertRules(filters: { geofence_id?: string; vehicle_id?: string }) {
  return useQuery({ queryKey: queryKeys.alertRules(filters), queryFn: () => api.listAlertRules(filters) })
}

export function useViolationHistory(filters: HistoryFilters) {
  return useQuery({
    queryKey: queryKeys.history(filters),
    queryFn: () => api.violationHistory(filters),
    placeholderData: keepPreviousData, // keep the old page visible while the next one loads
  })
}

export function useCreateGeofence() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: api.createGeofence,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['geofences'] }),
  })
}

export function useCreateVehicle() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: api.createVehicle,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.vehicles }),
  })
}

export function useUpdateLocation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: api.updateLocation,
    onSuccess: (_result, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.vehicleLocation(variables.vehicle_id) })
      queryClient.invalidateQueries({ queryKey: ['history'] })
    },
  })
}

export function useConfigureAlert() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: api.configureAlert,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['alert-rules'] }),
  })
}
