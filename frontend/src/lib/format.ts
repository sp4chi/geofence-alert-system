import type { Category } from '../api/types'

export const CATEGORY_LABELS: Record<Category, string> = {
  delivery_zone: 'Delivery zone',
  restricted_zone: 'Restricted zone',
  toll_zone: 'Toll zone',
  customer_area: 'Customer area',
}

/** Map + badge colours per category. Restricted is red because it is the one operators watch. */
export const CATEGORY_COLORS: Record<Category, string> = {
  delivery_zone: '#2563eb',
  restricted_zone: '#dc2626',
  toll_zone: '#d97706',
  customer_area: '#059669',
}

const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' })
const timeOnly = new Intl.DateTimeFormat(undefined, { timeStyle: 'medium' })

export const formatDateTime = (iso: string) => dateTime.format(new Date(iso))
export const formatTime = (iso: string) => timeOnly.format(new Date(iso))

export function timeAgo(iso: string, now = Date.now()): string {
  const seconds = Math.round((now - new Date(iso).getTime()) / 1000)
  if (seconds < 60) return 'just now'
  if (seconds < 3600) return `${Math.floor(seconds / 60)} min ago`
  if (seconds < 86_400) return `${Math.floor(seconds / 3600)} h ago`
  return formatDateTime(iso)
}

export const formatCoordinate = (value: number) => value.toFixed(5)

/** RFC 3339 "now" in UTC without milliseconds, e.g. 2025-01-15T10:35:00Z. */
export const nowIso = () => new Date().toISOString().replace(/\.\d{3}Z$/, 'Z')

/** Convert a <input type="datetime-local"> value (local time, no zone) to UTC RFC 3339. */
export function localInputToIso(value: string): string | undefined {
  if (!value) return undefined
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString().replace(/\.\d{3}Z$/, 'Z')
}
