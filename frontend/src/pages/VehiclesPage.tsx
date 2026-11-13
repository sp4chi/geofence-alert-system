import { useCallback, useMemo, useState, type FormEvent } from 'react'
import type { LatLng, LocationUpdateResult, Vehicle, VehicleLocation } from '../api/types'
import { Button, CategoryBadge, EmptyState, ErrorNotice, Field, Input, Panel, Spinner } from '../components/ui'
import { useMapMode, type MapMode } from '../context/mapMode'
import { useToast } from '../context/toast'
import { useCreateVehicle, useUpdateLocation, useVehicleLocations, useVehicles } from '../hooks/queries'
import { formatCoordinate, nowIso, timeAgo } from '../lib/format'

export function VehiclesPage() {
  const vehicles = useVehicles()
  const locations = useVehicleLocations(vehicles.data)
  const [selectedId, setSelectedId] = useState<string>()
  const selected = vehicles.data?.find((v) => v.id === selectedId)

  return (
    <div className="space-y-4">
      {selected ? (
        <LocationUpdateForm key={selected.id} vehicle={selected} onClose={() => setSelectedId(undefined)} />
      ) : (
        <RegisterVehicleForm />
      )}

      <Panel title="Vehicles" action={vehicles.isFetching && <Spinner />}>
        {vehicles.isPending ? (
          <Spinner label="Loading vehicles…" />
        ) : vehicles.error ? (
          <ErrorNotice error={vehicles.error} title="Couldn't load vehicles" />
        ) : vehicles.data.length === 0 ? (
          <EmptyState>No vehicles yet — register one above.</EmptyState>
        ) : (
          <ul className="divide-y divide-slate-100">
            {vehicles.data.map((vehicle) => (
              <VehicleRow
                key={vehicle.id}
                vehicle={vehicle}
                location={locations[vehicle.id]}
                selected={vehicle.id === selectedId}
                onSelect={() => setSelectedId(vehicle.id)}
              />
            ))}
          </ul>
        )}
      </Panel>
    </div>
  )
}

function VehicleRow({
  vehicle,
  location,
  selected,
  onSelect,
}: {
  vehicle: Vehicle
  location?: VehicleLocation
  selected: boolean
  onSelect: () => void
}) {
  const position = location?.current_location
  return (
    <li className={`py-2.5 ${selected ? '-mx-2 rounded-md bg-slate-100 px-2' : ''}`}>
      <div className="flex items-start justify-between gap-2">
        <div>
          <p className="text-sm font-semibold">{vehicle.vehicle_number}</p>
          <p className="text-xs text-slate-600">
            {vehicle.driver_name} · {vehicle.vehicle_type} · {vehicle.phone}
          </p>
        </div>
        <Button variant="secondary" onClick={onSelect} className="shrink-0 !px-2 !py-1 text-xs">
          Set location
        </Button>
      </div>
      <p className="mt-1 text-xs text-slate-500">
        {position
          ? `${formatCoordinate(position.latitude)}, ${formatCoordinate(position.longitude)} · ${timeAgo(position.timestamp)}`
          : location
            ? 'No location reported yet'
            : 'Loading location…'}
      </p>
      {location && location.current_geofences.length > 0 && (
        <div className="mt-1 flex flex-wrap gap-1">
          {location.current_geofences.map((g) => (
            <span key={g.geofence_id} title={g.geofence_name}>
              <CategoryBadge category={g.category} />
              <span className="ml-1 text-xs text-slate-700">{g.geofence_name}</span>
            </span>
          ))}
        </div>
      )}
    </li>
  )
}

const EMPTY_VEHICLE = { vehicle_number: '', driver_name: '', vehicle_type: '', phone: '' }

function RegisterVehicleForm() {
  const [form, setForm] = useState(EMPTY_VEHICLE)
  const createVehicle = useCreateVehicle()
  const toast = useToast()
  const set = (field: keyof typeof EMPTY_VEHICLE) => (e: { target: { value: string } }) =>
    setForm((current) => ({ ...current, [field]: e.target.value }))

  const submit = (event: FormEvent) => {
    event.preventDefault()
    createVehicle.mutate(form, {
      onSuccess: (created) => {
        toast({ tone: 'success', title: `Vehicle ${created.vehicle_number} registered` })
        setForm(EMPTY_VEHICLE)
        createVehicle.reset()
      },
    })
  }

  return (
    <Panel title="Register vehicle">
      <form onSubmit={submit} className="grid grid-cols-2 gap-3">
        <Field label="Vehicle number">
          <Input value={form.vehicle_number} onChange={set('vehicle_number')} placeholder="KA-01-AB-1234" required />
        </Field>
        <Field label="Vehicle type">
          <Input value={form.vehicle_type} onChange={set('vehicle_type')} placeholder="truck" list="vehicle-types" required />
          <datalist id="vehicle-types">
            {['truck', 'van', 'car', 'motorcycle', 'bus'].map((t) => (
              <option key={t} value={t} />
            ))}
          </datalist>
        </Field>
        <Field label="Driver name">
          <Input value={form.driver_name} onChange={set('driver_name')} placeholder="John Doe" required />
        </Field>
        <Field label="Phone">
          <Input value={form.phone} onChange={set('phone')} placeholder="+1234567890" type="tel" required />
        </Field>
        <div className="col-span-2 space-y-3">
          <ErrorNotice error={createVehicle.error} />
          <Button type="submit" loading={createVehicle.isPending} className="w-full">
            Register vehicle
          </Button>
        </div>
      </form>
    </Panel>
  )
}

function LocationUpdateForm({ vehicle, onClose }: { vehicle: Vehicle; onClose: () => void }) {
  const [latitude, setLatitude] = useState('')
  const [longitude, setLongitude] = useState('')
  const [timestamp, setTimestamp] = useState('') // empty = "now" at submit time
  const [result, setResult] = useState<LocationUpdateResult>()
  const updateLocation = useUpdateLocation()

  const lat = Number.parseFloat(latitude)
  const lng = Number.parseFloat(longitude)
  // Memoized on the two numbers so the map mode only changes when the point really moves.
  const picked = useMemo(
    () => (Number.isFinite(lat) && Number.isFinite(lng) ? ([lat, lng] as LatLng) : undefined),
    [lat, lng],
  )

  const handlePick = useCallback(([pickedLat, pickedLng]: LatLng) => {
    setLatitude(String(pickedLat))
    setLongitude(String(pickedLng))
  }, [])
  const mapMode = useMemo<MapMode>(() => ({ kind: 'pick', onPick: handlePick, picked }), [handlePick, picked])
  useMapMode(mapMode)

  const submit = (event: FormEvent) => {
    event.preventDefault()
    if (!picked) return
    updateLocation.mutate(
      { vehicle_id: vehicle.id, latitude: picked[0], longitude: picked[1], timestamp: timestamp.trim() || nowIso() },
      { onSuccess: setResult },
    )
  }

  return (
    <Panel
      title={`Update location · ${vehicle.vehicle_number}`}
      action={
        <Button variant="ghost" onClick={onClose} className="!px-2 !py-1 text-xs">
          Close
        </Button>
      }
    >
      <form onSubmit={submit} className="grid grid-cols-2 gap-3">
        <p className="col-span-2 text-xs text-slate-600">Click the map to fill in the position, or type it.</p>
        <Field label="Latitude">
          <Input value={latitude} onChange={(e) => setLatitude(e.target.value)} inputMode="decimal" placeholder="37.7849" required />
        </Field>
        <Field label="Longitude">
          <Input value={longitude} onChange={(e) => setLongitude(e.target.value)} inputMode="decimal" placeholder="-122.4194" required />
        </Field>
        <div className="col-span-2">
          <Field label="Timestamp (ISO 8601)" hint="Leave empty to use the current time.">
            <Input value={timestamp} onChange={(e) => setTimestamp(e.target.value)} placeholder={nowIso()} />
          </Field>
        </div>
        <div className="col-span-2 space-y-3">
          <ErrorNotice error={updateLocation.error} />
          <Button type="submit" loading={updateLocation.isPending} disabled={!picked} className="w-full">
            Send location update
          </Button>
          {result && (
            <div className="rounded-md bg-slate-50 p-3 text-sm" role="status">
              <p className="font-medium">
                {result.current_geofences.length
                  ? `Inside ${result.current_geofences.length} geofence${result.current_geofences.length > 1 ? 's' : ''}:`
                  : 'Not inside any geofence.'}
              </p>
              <ul className="mt-1 list-disc pl-5 text-slate-700">
                {result.current_geofences.map((g) => (
                  <li key={g.geofence_id}>{g.geofence_name}</li>
                ))}
              </ul>
            </div>
          )}
        </div>
      </form>
    </Panel>
  )
}
