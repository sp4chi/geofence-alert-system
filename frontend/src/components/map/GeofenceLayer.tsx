import { memo } from 'react'
import { Polygon, Tooltip } from 'react-leaflet'
import type { Geofence } from '../../api/types'
import { CATEGORY_COLORS, CATEGORY_LABELS } from '../../lib/format'

/**
 * All geofence polygons. memo + a stable array from TanStack Query means a new live alert (which
 * re-renders other parts of the app) doesn't re-render every polygon.
 */
export const GeofenceLayer = memo(function GeofenceLayer({ geofences }: { geofences: Geofence[] }) {
  return (
    <>
      {geofences.map((geofence) => (
        <Polygon
          key={geofence.id}
          positions={geofence.coordinates}
          pathOptions={{
            color: CATEGORY_COLORS[geofence.category],
            weight: 2,
            fillOpacity: geofence.category === 'restricted_zone' ? 0.22 : 0.12,
          }}
        >
          <Tooltip sticky>
            <strong>{geofence.name}</strong> · {CATEGORY_LABELS[geofence.category]}
          </Tooltip>
        </Polygon>
      ))}
    </>
  )
})
