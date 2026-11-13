import type { LatLng as LeafletLatLng } from 'leaflet'
import type { LatLng } from '../api/types'

/** Round to 6 decimals (~10 cm) so drawn coordinates stay readable in the JSON box. */
export const roundCoordinate = (value: number) => Math.round(value * 1e6) / 1e6

/** Leaflet point → API [lat, lng], with longitude wrapped into [-180, 180] if the map was panned around the world. */
export function toApiPoint(point: LeafletLatLng): LatLng {
  const wrapped = point.wrap()
  return [roundCoordinate(wrapped.lat), roundCoordinate(wrapped.lng)]
}

/** Leaflet rings are open; the API wants the first point repeated at the end. */
export function toClosedRing(points: LeafletLatLng[]): LatLng[] {
  const ring = points.map(toApiPoint)
  if (ring.length > 0) ring.push([...ring[0]] as LatLng)
  return ring
}
