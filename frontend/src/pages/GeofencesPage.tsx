import { useCallback, useMemo, useState, type FormEvent } from 'react'
import { CATEGORIES, type Category, type LatLng } from '../api/types'
import { Button, CategoryBadge, EmptyState, ErrorNotice, Field, Input, Panel, Select, Spinner, Textarea } from '../components/ui'
import { useMapMode, type MapMode } from '../context/mapMode'
import { useToast } from '../context/toast'
import { useCreateGeofence, useGeofences } from '../hooks/queries'
import { CATEGORY_LABELS, formatDateTime } from '../lib/format'

export function GeofencesPage() {
  return (
    <div className="space-y-4">
      <CreateGeofenceForm />
      <GeofenceList />
    </div>
  )
}

/** Client-side shape check only; the API does the real validation and its messages are shown as-is. */
function parseCoordinates(text: string): { ring?: LatLng[]; error?: string } {
  if (!text.trim()) return {}
  let value: unknown
  try {
    value = JSON.parse(text)
  } catch {
    return { error: 'Not valid JSON. Expected e.g. [[37.77, -122.42], [37.78, -122.42], ...]' }
  }
  const isPair = (p: unknown) => Array.isArray(p) && p.length === 2 && p.every((n) => typeof n === 'number')
  if (!Array.isArray(value) || !value.every(isPair)) {
    return { error: 'Coordinates must be an array of [latitude, longitude] number pairs' }
  }
  return { ring: value as LatLng[] }
}

const formatRing = (ring: LatLng[]) => `[\n${ring.map(([lat, lng]) => `  [${lat}, ${lng}]`).join(',\n')}\n]`

const isClosed = (ring: LatLng[]) =>
  ring.length > 1 && ring[0][0] === ring[ring.length - 1][0] && ring[0][1] === ring[ring.length - 1][1]

function CreateGeofenceForm() {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [category, setCategory] = useState<Category>('delivery_zone')
  const [coordinatesText, setCoordinatesText] = useState('')
  const parsed = useMemo(() => parseCoordinates(coordinatesText), [coordinatesText])

  const createGeofence = useCreateGeofence()
  const toast = useToast()

  // Drawing on the map fills the text box, so typed and drawn shapes go through one path.
  const handleDraw = useCallback((ring: LatLng[]) => setCoordinatesText(formatRing(ring)), [])
  const mapMode = useMemo<MapMode>(() => ({ kind: 'draw', onDraw: handleDraw, draft: parsed.ring }), [handleDraw, parsed.ring])
  useMapMode(mapMode)

  const closeRing = () => parsed.ring && setCoordinatesText(formatRing([...parsed.ring, parsed.ring[0]]))

  const submit = (event: FormEvent) => {
    event.preventDefault()
    if (!parsed.ring) return
    createGeofence.mutate(
      { name, description, category, coordinates: parsed.ring },
      {
        onSuccess: (created) => {
          toast({ tone: 'success', title: `Geofence “${created.name}” created` })
          setName('')
          setDescription('')
          setCoordinatesText('')
          createGeofence.reset()
        },
      },
    )
  }

  return (
    <Panel title="Create geofence">
      <form onSubmit={submit} className="space-y-3">
        <Field label="Name">
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Downtown Delivery Zone" required />
        </Field>
        <Field label="Description (optional)">
          <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Main delivery area" />
        </Field>
        <Field label="Category">
          <Select value={category} onChange={(e) => setCategory(e.target.value as Category)}>
            {CATEGORIES.map((c) => (
              <option key={c} value={c}>
                {CATEGORY_LABELS[c]}
              </option>
            ))}
          </Select>
        </Field>
        <Field
          label="Coordinates — [latitude, longitude] pairs"
          hint="Draw on the map (tools on the left of the map) or type/paste. First and last point must match."
        >
          <Textarea
            rows={6}
            value={coordinatesText}
            onChange={(e) => setCoordinatesText(e.target.value)}
            placeholder={'[\n  [37.7749, -122.4194],\n  [37.7849, -122.4194],\n  [37.7849, -122.4094],\n  [37.7749, -122.4194]\n]'}
            spellCheck={false}
          />
        </Field>

        {parsed.error && <p className="text-xs text-red-700">{parsed.error}</p>}
        {parsed.ring && !isClosed(parsed.ring) && (
          <p className="flex items-center gap-2 text-xs text-amber-800">
            The polygon isn't closed.
            <button type="button" onClick={closeRing} className="font-semibold underline">
              Close it
            </button>
          </p>
        )}
        {parsed.ring && isClosed(parsed.ring) && (
          <p className="text-xs text-slate-500">{parsed.ring.length - 1} corners · preview shown dashed on the map</p>
        )}

        <ErrorNotice error={createGeofence.error} />
        <Button type="submit" loading={createGeofence.isPending} disabled={!parsed.ring || !name.trim()} className="w-full">
          Create geofence
        </Button>
      </form>
    </Panel>
  )
}

function GeofenceList() {
  const [category, setCategory] = useState<Category | ''>('')
  const geofences = useGeofences(category || undefined)

  return (
    <Panel title="Geofences" action={geofences.isFetching && <Spinner />}>
      <div className="mb-3 flex flex-wrap gap-1.5" role="group" aria-label="Filter by category">
        {(['', ...CATEGORIES] as const).map((c) => (
          <button
            key={c || 'all'}
            type="button"
            onClick={() => setCategory(c)}
            aria-pressed={category === c}
            className={`rounded-full border px-2.5 py-1 text-xs font-medium ${
              category === c ? 'border-slate-900 bg-slate-900 text-white' : 'border-slate-300 bg-white text-slate-700 hover:bg-slate-50'
            }`}
          >
            {c ? CATEGORY_LABELS[c] : 'All'}
          </button>
        ))}
      </div>

      {geofences.isPending ? (
        <Spinner label="Loading geofences…" />
      ) : geofences.error ? (
        <ErrorNotice error={geofences.error} title="Couldn't load geofences" />
      ) : geofences.data.length === 0 ? (
        <EmptyState>{category ? `No ${CATEGORY_LABELS[category].toLowerCase()}s yet.` : 'No geofences yet — draw one above.'}</EmptyState>
      ) : (
        <ul className="divide-y divide-slate-100">
          {geofences.data.map((g) => (
            <li key={g.id} className="py-2">
              <div className="flex items-start justify-between gap-2">
                <p className="text-sm font-medium">{g.name}</p>
                <CategoryBadge category={g.category} />
              </div>
              {g.description && <p className="text-xs text-slate-600">{g.description}</p>}
              <p className="mt-0.5 text-xs text-slate-400">
                {g.coordinates.length - 1} corners · {formatDateTime(g.created_at)} · <code>{g.id}</code>
              </p>
            </li>
          ))}
        </ul>
      )}
    </Panel>
  )
}
