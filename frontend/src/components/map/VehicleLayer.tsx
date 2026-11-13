import { memo } from 'react'
import { CircleMarker, Popup, Tooltip } from 'react-leaflet'
import type { Vehicle, VehicleLocation } from '../../api/types'
import { formatCoordinate, timeAgo } from '../../lib/format'

/** One marker per vehicle that has reported a location. Red = currently inside a restricted zone. */
export const VehicleLayer = memo(function VehicleLayer({
  vehicles,
  locations,
}: {
  vehicles: Vehicle[]
  locations: Record<string, VehicleLocation>
}) {
  return (
    <>
      {vehicles.map((vehicle) => {
        const location = locations[vehicle.id]
        const position = location?.current_location
        if (!position) return null

        const inRestricted = location.current_geofences.some((g) => g.category === 'restricted_zone')
        return (
          <CircleMarker
            key={vehicle.id}
            center={[position.latitude, position.longitude]}
            radius={8}
            pathOptions={{
              color: '#ffffff',
              weight: 2,
              fillColor: inRestricted ? '#dc2626' : '#0f172a',
              fillOpacity: 1,
            }}
          >
            <Tooltip direction="top" offset={[0, -8]} permanent className="!px-1.5 !py-0.5 !text-xs !font-semibold">
              {vehicle.vehicle_number}
            </Tooltip>
            <Popup>
              <p className="font-semibold">{vehicle.vehicle_number}</p>
              <p>
                {vehicle.driver_name} · {vehicle.vehicle_type}
              </p>
              <p>
                {formatCoordinate(position.latitude)}, {formatCoordinate(position.longitude)} · {timeAgo(position.timestamp)}
              </p>
              <p>
                Inside:{' '}
                {location.current_geofences.length
                  ? location.current_geofences.map((g) => g.geofence_name).join(', ')
                  : 'no geofence'}
              </p>
            </Popup>
          </CircleMarker>
        )
      })}
    </>
  )
})
