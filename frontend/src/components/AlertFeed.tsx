import { useLiveAlerts } from '../context/liveAlerts'
import { formatCoordinate, formatTime } from '../lib/format'
import { CategoryBadge, EventBadge } from './ui'

/** Alerts received over the WebSocket during this session, newest first. */
export function AlertFeed() {
  const { alerts, clear } = useLiveAlerts()

  return (
    <section className="flex min-h-0 flex-col" aria-label="Live alerts">
      <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
        <h2 className="text-sm font-semibold">
          Live alerts {alerts.length > 0 && <span className="text-slate-500">({alerts.length})</span>}
        </h2>
        {alerts.length > 0 && (
          <button type="button" onClick={clear} className="text-xs text-slate-500 hover:text-slate-900">
            Clear
          </button>
        )}
      </div>

      {alerts.length === 0 ? (
        <p className="p-4 text-sm text-slate-500">
          Waiting for alerts. They appear here when a vehicle crosses a geofence with an alert rule, or enters a restricted
          zone.
        </p>
      ) : (
        <ol className="min-h-0 flex-1 divide-y divide-slate-100 overflow-y-auto" aria-live="polite">
          {alerts.map((alert) => (
            <li key={alert.event_id} className="px-4 py-3">
              <div className="flex items-center justify-between gap-2">
                <EventBadge type={alert.event_type} />
                <time className="text-xs text-slate-500" dateTime={alert.timestamp}>
                  {formatTime(alert.timestamp)}
                </time>
              </div>
              <p className="mt-1 text-sm">
                <span className="font-semibold">{alert.vehicle.vehicle_number}</span>{' '}
                {alert.event_type === 'entry' ? 'entered' : 'left'}{' '}
                <span className="font-medium">{alert.geofence.geofence_name}</span>
              </p>
              <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-slate-500">
                <CategoryBadge category={alert.geofence.category} />
                <span>{alert.vehicle.driver_name}</span>
                <span className="font-mono">
                  {formatCoordinate(alert.location.latitude)}, {formatCoordinate(alert.location.longitude)}
                </span>
              </div>
            </li>
          ))}
        </ol>
      )}
    </section>
  )
}
