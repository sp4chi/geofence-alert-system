import { useQueryClient } from '@tanstack/react-query'
import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react'
import type { AlertMessage } from '../api/types'
import { queryKeys } from '../hooks/queries'
import { useAlertsSocket, type SocketStatus } from '../hooks/useAlertsSocket'
import { CATEGORY_LABELS } from '../lib/format'
import { useToast } from './toast'

const MAX_FEED = 100

interface LiveAlerts {
  status: SocketStatus
  alerts: AlertMessage[]
  clear: () => void
}

const LiveAlertsContext = createContext<LiveAlerts>({ status: 'connecting', alerts: [], clear: () => {} })

/** Owns the single WebSocket for the app: keeps the live feed, raises toasts, refreshes stale data. */
export function LiveAlertsProvider({ children }: { children: ReactNode }) {
  const [alerts, setAlerts] = useState<AlertMessage[]>([])
  const queryClient = useQueryClient()
  const toast = useToast()

  const handleAlert = useCallback(
    (alert: AlertMessage) => {
      // Newest first, capped so a long session can't grow memory without bound.
      setAlerts((current) => [alert, ...current].slice(0, MAX_FEED))

      const verb = alert.event_type === 'entry' ? 'entered' : 'left'
      toast({
        tone: 'alert',
        title: `${alert.vehicle.vehicle_number} ${verb} ${alert.geofence.geofence_name}`,
        body: `${CATEGORY_LABELS[alert.geofence.category]} · driver ${alert.vehicle.driver_name}`,
      })

      // The alert means this vehicle moved and history grew: refetch just those.
      queryClient.invalidateQueries({ queryKey: queryKeys.vehicleLocation(alert.vehicle.vehicle_id) })
      queryClient.invalidateQueries({ queryKey: ['history'] })
    },
    [queryClient, toast],
  )

  // After a reconnect, anything that happened while offline is in the history: refetch it.
  const handleReconnect = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ['history'] })
    queryClient.invalidateQueries({ queryKey: ['vehicle-location'] })
  }, [queryClient])

  const status = useAlertsSocket(handleAlert, handleReconnect)
  const clear = useCallback(() => setAlerts([]), [])
  const value = useMemo(() => ({ status, alerts, clear }), [status, alerts, clear])

  return <LiveAlertsContext value={value}>{children}</LiveAlertsContext>
}

// eslint-disable-next-line react-refresh/only-export-components
export const useLiveAlerts = () => useContext(LiveAlertsContext)
