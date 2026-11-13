import { useLiveAlerts } from '../context/liveAlerts'

const STATES = {
  open: { label: 'Live', dot: 'bg-emerald-500', text: 'text-emerald-800 bg-emerald-50 border-emerald-200' },
  connecting: { label: 'Connecting…', dot: 'bg-amber-500 animate-pulse', text: 'text-amber-800 bg-amber-50 border-amber-200' },
  reconnecting: { label: 'Reconnecting…', dot: 'bg-amber-500 animate-pulse', text: 'text-amber-800 bg-amber-50 border-amber-200' },
}

/** Shows whether live alerts are flowing, so a silent feed is never mistaken for "no alerts". */
export function ConnectionPill() {
  const { status } = useLiveAlerts()
  const state = STATES[status]
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium ${state.text}`}
      role="status"
      title="Real-time alert connection"
    >
      <span className={`size-2 rounded-full ${state.dot}`} aria-hidden />
      {state.label}
    </span>
  )
}
