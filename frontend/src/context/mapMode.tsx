import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import type { LatLng } from '../api/types'

/**
 * The map is shared by every page, but what a click means depends on the page:
 * - draw: the polygon tool is on (Geofences page); draft previews the polygon being typed
 * - pick: a click sets a point (Vehicles page, "set location"); picked shows it
 */
export type MapMode =
  | { kind: 'idle' }
  | { kind: 'draw'; onDraw: (ring: LatLng[]) => void; draft?: LatLng[] }
  | { kind: 'pick'; onPick: (point: LatLng) => void; picked?: LatLng }

const IDLE: MapMode = { kind: 'idle' }

// Value and setter are separate contexts: pages only use the (stable) setter, so changing the
// mode re-renders the map but not the page that set it — which would otherwise loop.
const ModeContext = createContext<MapMode>(IDLE)
const SetModeContext = createContext<(mode: MapMode) => void>(() => {})

export function MapModeProvider({ children }: { children: ReactNode }) {
  const [mode, setMode] = useState<MapMode>(IDLE)
  return (
    <SetModeContext value={setMode}>
      <ModeContext value={mode}>{children}</ModeContext>
    </SetModeContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useMapModeValue = () => useContext(ModeContext)

/**
 * A page calls this to put the map in a mode while it is mounted; it resets to idle on unmount.
 * Pass a memoized object (useMemo) so the effect only re-runs when the mode really changes.
 */
// eslint-disable-next-line react-refresh/only-export-components
export function useMapMode(mode: MapMode) {
  const setMode = useContext(SetModeContext)
  useEffect(() => {
    setMode(mode)
  }, [mode, setMode])
  useEffect(() => () => setMode(IDLE), [setMode])
}
