import { useEffect, useRef, useState } from 'react'
import { WS_URL } from '../api/client'
import type { AlertMessage } from '../api/types'

export type SocketStatus = 'connecting' | 'open' | 'reconnecting'

const MAX_DELAY_MS = 30_000

/**
 * Exponential backoff with "equal jitter": 1 s, 2 s, 4 s … capped at 30 s, each randomised between
 * half and the full value so many tabs don't all reconnect at the same instant after an outage.
 */
export function reconnectDelay(attempt: number): number {
  const ceiling = Math.min(MAX_DELAY_MS, 1000 * 2 ** attempt)
  return ceiling / 2 + Math.random() * (ceiling / 2)
}

/**
 * Keeps a WebSocket open to /ws/alerts for the lifetime of the component, reconnecting on any drop.
 * onReconnect fires after a reconnect so the caller can refetch whatever it missed while offline.
 */
export function useAlertsSocket(onAlert: (alert: AlertMessage) => void, onReconnect?: () => void): SocketStatus {
  const [status, setStatus] = useState<SocketStatus>('connecting')

  // Refs hold the latest callbacks, so the socket isn't torn down whenever the caller re-renders.
  const onAlertRef = useRef(onAlert)
  const onReconnectRef = useRef(onReconnect)
  useEffect(() => {
    onAlertRef.current = onAlert
    onReconnectRef.current = onReconnect
  })

  useEffect(() => {
    let socket: WebSocket | undefined
    let retryTimer: ReturnType<typeof setTimeout> | undefined
    let attempt = 0
    let hasConnectedBefore = false
    let stopped = false

    const connect = () => {
      socket = new WebSocket(WS_URL)

      socket.onopen = () => {
        attempt = 0
        setStatus('open')
        if (hasConnectedBefore) onReconnectRef.current?.()
        hasConnectedBefore = true
      }

      socket.onmessage = (event) => {
        try {
          onAlertRef.current(JSON.parse(event.data) as AlertMessage)
        } catch {
          console.warn('Ignoring a malformed alert message', event.data)
        }
      }

      // onclose also fires after a failed connection attempt, so this is the single retry path.
      socket.onclose = () => {
        if (stopped) return
        setStatus('reconnecting')
        retryTimer = setTimeout(connect, reconnectDelay(attempt))
        attempt += 1
      }
    }

    connect()
    return () => {
      stopped = true
      clearTimeout(retryTimer)
      socket?.close()
    }
  }, [])

  return status
}
