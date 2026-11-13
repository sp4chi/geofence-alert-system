import { useState, type FormEvent } from 'react'
import { ALERT_EVENT_TYPES, type AlertEventType } from '../api/types'
import { Button, EmptyState, ErrorNotice, EventBadge, Field, Panel, Select, Spinner } from '../components/ui'
import { useToast } from '../context/toast'
import { useAlertRules, useConfigureAlert, useGeofences, useVehicles } from '../hooks/queries'
import { formatDateTime } from '../lib/format'

const EVENT_LABELS: Record<AlertEventType, string> = {
  entry: 'Entry — vehicle enters the zone',
  exit: 'Exit — vehicle leaves the zone',
  both: 'Both entry and exit',
}

export function AlertRulesPage() {
  const geofences = useGeofences()
  const vehicles = useVehicles()
  const [filterGeofence, setFilterGeofence] = useState('')
  const [filterVehicle, setFilterVehicle] = useState('')
  const rules = useAlertRules({ geofence_id: filterGeofence || undefined, vehicle_id: filterVehicle || undefined })

  return (
    <div className="space-y-4">
      <ConfigureAlertForm />

      <Panel title="Alert rules" action={rules.isFetching && <Spinner />}>
        <div className="mb-3 grid grid-cols-2 gap-2">
          <Select value={filterGeofence} onChange={(e) => setFilterGeofence(e.target.value)} aria-label="Filter by geofence">
            <option value="">All geofences</option>
            {geofences.data?.map((g) => (
              <option key={g.id} value={g.id}>
                {g.name}
              </option>
            ))}
          </Select>
          <Select value={filterVehicle} onChange={(e) => setFilterVehicle(e.target.value)} aria-label="Filter by vehicle">
            <option value="">Any vehicle</option>
            {vehicles.data?.map((v) => (
              <option key={v.id} value={v.id}>
                {v.vehicle_number}
              </option>
            ))}
          </Select>
        </div>

        {rules.isPending ? (
          <Spinner label="Loading rules…" />
        ) : rules.error ? (
          <ErrorNotice error={rules.error} title="Couldn't load alert rules" />
        ) : rules.data.length === 0 ? (
          <EmptyState>No alert rules{filterGeofence || filterVehicle ? ' match these filters' : ' yet'}.</EmptyState>
        ) : (
          <ul className="divide-y divide-slate-100">
            {rules.data.map((rule) => (
              <li key={rule.alert_id} className="py-2">
                <div className="flex items-center justify-between gap-2">
                  <p className="text-sm font-medium">{rule.geofence_name}</p>
                  <EventBadge type={rule.event_type} />
                </div>
                <p className="text-xs text-slate-600">{rule.vehicle_number ?? 'All vehicles'}</p>
                <p className="text-xs text-slate-400">
                  {rule.status} · {formatDateTime(rule.created_at)}
                </p>
              </li>
            ))}
          </ul>
        )}
        <p className="mt-3 text-xs text-slate-500">
          Entering a restricted zone always raises an alert, even without a rule.
        </p>
      </Panel>
    </div>
  )
}

function ConfigureAlertForm() {
  const geofences = useGeofences()
  const vehicles = useVehicles()
  const [geofenceId, setGeofenceId] = useState('')
  const [vehicleId, setVehicleId] = useState('')
  const [eventType, setEventType] = useState<AlertEventType>('entry')
  const configure = useConfigureAlert()
  const toast = useToast()

  const submit = (event: FormEvent) => {
    event.preventDefault()
    configure.mutate(
      { geofence_id: geofenceId, vehicle_id: vehicleId || undefined, event_type: eventType },
      {
        onSuccess: (rule) => {
          toast({ tone: 'success', title: 'Alert rule created', body: `${rule.event_type} on ${geofences.data?.find((g) => g.id === rule.geofence_id)?.name ?? rule.geofence_id}` })
          configure.reset()
        },
      },
    )
  }

  const noGeofences = geofences.data?.length === 0

  return (
    <Panel title="Configure alert">
      <form onSubmit={submit} className="space-y-3">
        <Field label="Geofence">
          <Select value={geofenceId} onChange={(e) => setGeofenceId(e.target.value)} required disabled={geofences.isPending || noGeofences}>
            <option value="" disabled>
              {geofences.isPending ? 'Loading…' : noGeofences ? 'Create a geofence first' : 'Choose a geofence'}
            </option>
            {geofences.data?.map((g) => (
              <option key={g.id} value={g.id}>
                {g.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Vehicle (optional)">
          <Select value={vehicleId} onChange={(e) => setVehicleId(e.target.value)} disabled={vehicles.isPending}>
            <option value="">All vehicles</option>
            {vehicles.data?.map((v) => (
              <option key={v.id} value={v.id}>
                {v.vehicle_number} · {v.driver_name}
              </option>
            ))}
          </Select>
        </Field>
        <fieldset>
          <legend className="mb-1 text-xs font-medium text-slate-700">Alert on</legend>
          <div className="space-y-1">
            {ALERT_EVENT_TYPES.map((type) => (
              <label key={type} className="flex items-center gap-2 text-sm">
                <input type="radio" name="event_type" value={type} checked={eventType === type} onChange={() => setEventType(type)} />
                {EVENT_LABELS[type]}
              </label>
            ))}
          </div>
        </fieldset>
        <ErrorNotice error={configure.error} />
        <Button type="submit" loading={configure.isPending} disabled={!geofenceId} className="w-full">
          Create alert rule
        </Button>
      </form>
    </Panel>
  )
}
