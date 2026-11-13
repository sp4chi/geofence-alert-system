import { useSearchParams } from 'react-router'
import type { HistoryFilters } from '../api/types'
import { Button, EmptyState, ErrorNotice, EventBadge, Field, Input, Panel, Select, Spinner } from '../components/ui'
import { useGeofences, useVehicles, useViolationHistory } from '../hooks/queries'
import { formatCoordinate, formatDateTime, localInputToIso } from '../lib/format'

const PAGE_SIZE = 25

/** Filters live in the URL (?vehicle_id=…&page=2), so a filtered view can be shared or bookmarked. */
export function HistoryPage() {
  const [params, setParams] = useSearchParams()
  const geofences = useGeofences()
  const vehicles = useVehicles()

  const page = Math.max(1, Number(params.get('page')) || 1)
  const filters: HistoryFilters = {
    vehicle_id: params.get('vehicle_id') || undefined,
    geofence_id: params.get('geofence_id') || undefined,
    start_date: localInputToIso(params.get('from') ?? ''),
    end_date: localInputToIso(params.get('to') ?? ''),
    limit: PAGE_SIZE,
    offset: (page - 1) * PAGE_SIZE,
  }
  const history = useViolationHistory(filters)

  // Changing a filter goes back to page 1; empty values are removed from the URL.
  const setFilter = (key: string, value: string) =>
    setParams((current) => {
      const next = new URLSearchParams(current)
      if (value) next.set(key, value)
      else next.delete(key)
      if (key !== 'page') next.delete('page')
      return next
    })

  const total = history.data?.total_count ?? 0
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const firstRow = total === 0 ? 0 : filters.offset + 1
  const lastRow = Math.min(filters.offset + PAGE_SIZE, total)

  return (
    <Panel title="Violation history" action={history.isFetching && <Spinner />}>
      <div className="mb-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
        <Field label="Vehicle">
          <Select value={filters.vehicle_id ?? ''} onChange={(e) => setFilter('vehicle_id', e.target.value)}>
            <option value="">All vehicles</option>
            {vehicles.data?.map((v) => (
              <option key={v.id} value={v.id}>
                {v.vehicle_number}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Geofence">
          <Select value={filters.geofence_id ?? ''} onChange={(e) => setFilter('geofence_id', e.target.value)}>
            <option value="">All geofences</option>
            {geofences.data?.map((g) => (
              <option key={g.id} value={g.id}>
                {g.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="From">
          <Input type="datetime-local" value={params.get('from') ?? ''} onChange={(e) => setFilter('from', e.target.value)} />
        </Field>
        <Field label="To">
          <Input type="datetime-local" value={params.get('to') ?? ''} onChange={(e) => setFilter('to', e.target.value)} />
        </Field>
        <div className="flex items-end">
          <Button variant="secondary" onClick={() => setParams({})} disabled={params.size === 0} className="w-full">
            Clear filters
          </Button>
        </div>
      </div>

      {history.isPending ? (
        <Spinner label="Loading history…" />
      ) : history.error ? (
        <ErrorNotice error={history.error} title="Couldn't load history" />
      ) : history.data.violations.length === 0 ? (
        <EmptyState>No entry or exit events match these filters.</EmptyState>
      ) : (
        <>
          <div className="-mx-4 overflow-x-auto">
            <table className="w-full min-w-[640px] text-left text-sm">
              <thead className="border-b border-slate-200 text-xs uppercase tracking-wide text-slate-500">
                <tr>
                  <th className="px-4 py-2 font-medium">Time</th>
                  <th className="px-4 py-2 font-medium">Event</th>
                  <th className="px-4 py-2 font-medium">Vehicle</th>
                  <th className="px-4 py-2 font-medium">Geofence</th>
                  <th className="px-4 py-2 font-medium">Location</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {history.data.violations.map((v) => (
                  <tr key={v.id} className="hover:bg-slate-50">
                    <td className="whitespace-nowrap px-4 py-2 text-slate-600">{formatDateTime(v.timestamp)}</td>
                    <td className="px-4 py-2">
                      <EventBadge type={v.event_type} />
                    </td>
                    <td className="px-4 py-2 font-medium">{v.vehicle_number}</td>
                    <td className="px-4 py-2">{v.geofence_name}</td>
                    <td className="whitespace-nowrap px-4 py-2 font-mono text-xs text-slate-600">
                      {formatCoordinate(v.latitude)}, {formatCoordinate(v.longitude)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <nav className="mt-4 flex items-center justify-between gap-2 text-sm" aria-label="Pagination">
            <p className="text-slate-600">
              {firstRow}–{lastRow} of {total}
            </p>
            <div className="flex items-center gap-2">
              <Button variant="secondary" disabled={page <= 1} onClick={() => setFilter('page', String(page - 1))}>
                Previous
              </Button>
              <span className="text-slate-600">
                Page {page} of {pageCount}
              </span>
              <Button variant="secondary" disabled={page >= pageCount} onClick={() => setFilter('page', String(page + 1))}>
                Next
              </Button>
            </div>
          </nav>
        </>
      )}
    </Panel>
  )
}
