import { useEffect, useRef } from 'react'
import { CircleMarker, MapContainer, Polygon, TileLayer, useMap, useMapEvents } from 'react-leaflet'
import type { Geofence, LatLng, Vehicle } from '../../api/types'
import { useMapModeValue } from '../../context/mapMode'
import { useGeofences, useVehicleLocations, useVehicles } from '../../hooks/queries'
import { CATEGORY_COLORS, CATEGORY_LABELS } from '../../lib/format'
import { CATEGORIES } from '../../api/types'
import { toApiPoint } from '../../lib/geometry'
import { DrawControl } from './DrawControl'
import { GeofenceLayer } from './GeofenceLayer'
import { VehicleLayer } from './VehicleLayer'

const DEFAULT_CENTER: LatLng = [20.5937, 78.9629] // India; replaced by the geofences' bounds once loaded
const NO_GEOFENCES: Geofence[] = []
const NO_VEHICLES: Vehicle[] = []

export function MapView({ visible = true }: { visible?: boolean }) {
  const { data: geofences = NO_GEOFENCES } = useGeofences()
  const { data: vehicles = NO_VEHICLES } = useVehicles()
  const locations = useVehicleLocations(vehicles)
  const mode = useMapModeValue()

  return (
    <div className="relative h-full w-full">
      <MapContainer center={DEFAULT_CENTER} zoom={5} className="h-full w-full" worldCopyJump>
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
          url="https://tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        <GeofenceLayer geofences={geofences} />
        <VehicleLayer vehicles={vehicles} locations={locations} />
        <FitToGeofencesOnce geofences={geofences} />
        <ResizeWhenShown visible={visible} />

        {mode.kind === 'draw' && <DrawControl onDraw={mode.onDraw} />}
        {mode.kind === 'draw' && mode.draft && mode.draft.length > 1 && (
          <Polygon positions={mode.draft} pathOptions={{ color: '#0f172a', dashArray: '6 6', weight: 2, fillOpacity: 0.1 }} />
        )}

        {mode.kind === 'pick' && <PickOnClick onPick={mode.onPick} />}
        {mode.kind === 'pick' && mode.picked && (
          <CircleMarker center={mode.picked} radius={9} pathOptions={{ color: '#0f172a', dashArray: '3 3', fillOpacity: 0.15 }} />
        )}
      </MapContainer>

      {mode.kind !== 'idle' && (
        <p className="pointer-events-none absolute top-3 right-3 left-14 z-[500] mx-auto w-fit rounded-full bg-slate-900/85 px-3 py-1 text-center text-xs font-medium text-white shadow">
          {mode.kind === 'draw'
            ? 'Draw a polygon or rectangle with the tools on the left'
            : 'Click the map to set the vehicle location'}
        </p>
      )}
      <Legend />
    </div>
  )
}

/** Zoom to all geofences the first time they load, then leave the user's view alone. */
function FitToGeofencesOnce({ geofences }: { geofences: Geofence[] }) {
  const map = useMap()
  const fitted = useRef(false)
  useEffect(() => {
    if (fitted.current || geofences.length === 0) return
    fitted.current = true
    map.fitBounds(geofences.flatMap((g) => g.coordinates), { padding: [24, 24], maxZoom: 14 })
  }, [geofences, map])
  return null
}

/** Leaflet measures its container once; after being hidden (History page) it must re-measure. */
function ResizeWhenShown({ visible }: { visible: boolean }) {
  const map = useMap()
  useEffect(() => {
    if (visible) map.invalidateSize()
  }, [visible, map])
  return null
}

function PickOnClick({ onPick }: { onPick: (point: LatLng) => void }) {
  const map = useMapEvents({
    click(event) {
      onPick(toApiPoint(event.latlng))
    },
  })
  useEffect(() => {
    const container = map.getContainer()
    container.style.cursor = 'crosshair'
    return () => {
      container.style.cursor = ''
    }
  }, [map])
  return null
}

function Legend() {
  return (
    <div className="absolute bottom-6 left-2 z-[500] hidden rounded-md bg-white/95 p-2 text-xs shadow sm:block">
      {CATEGORIES.map((category) => (
        <p key={category} className="flex items-center gap-1.5">
          <span className="size-2.5 rounded-sm" style={{ backgroundColor: CATEGORY_COLORS[category] }} aria-hidden />
          {CATEGORY_LABELS[category]}
        </p>
      ))}
      <p className="mt-1 flex items-center gap-1.5 border-t border-slate-200 pt-1">
        <span className="size-2.5 rounded-full bg-slate-900" aria-hidden /> Vehicle
        <span className="ml-1 size-2.5 rounded-full bg-red-600" aria-hidden /> in restricted
      </p>
    </div>
  )
}
