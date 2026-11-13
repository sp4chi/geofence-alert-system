import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from 'react'

type Tone = 'success' | 'error' | 'alert'

interface Toast {
  id: number
  tone: Tone
  title: string
  body?: string
}

type PushToast = (toast: Omit<Toast, 'id'>) => void

const ToastContext = createContext<PushToast>(() => {})

const TONE_STYLES: Record<Tone, string> = {
  success: 'border-emerald-300 bg-emerald-50 text-emerald-900',
  error: 'border-red-300 bg-red-50 text-red-900',
  alert: 'border-red-500 bg-white text-slate-900 shadow-red-200',
}

const VISIBLE_MS = 6000
const MAX_TOASTS = 4

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const nextId = useRef(0)

  const dismiss = useCallback((id: number) => setToasts((current) => current.filter((t) => t.id !== id)), [])

  const push = useCallback<PushToast>(
    (toast) => {
      const id = ++nextId.current
      // Functional update: several alerts can arrive in one tick; each must see the latest list.
      setToasts((current) => [...current.slice(-(MAX_TOASTS - 1)), { ...toast, id }])
      setTimeout(() => dismiss(id), VISIBLE_MS)
    },
    [dismiss],
  )

  return (
    <ToastContext value={push}>
      {children}
      <div aria-live="polite" className="pointer-events-none fixed inset-x-3 top-14 z-50 flex flex-col items-end gap-2">
        {toasts.map((toast) => (
          <div
            key={toast.id}
            role={toast.tone === 'error' || toast.tone === 'alert' ? 'alert' : 'status'}
            className={`pointer-events-auto w-full max-w-sm rounded-lg border-l-4 border p-3 shadow-lg ${TONE_STYLES[toast.tone]}`}
          >
            <div className="flex items-start justify-between gap-3">
              <p className="text-sm font-semibold">{toast.title}</p>
              <button
                type="button"
                onClick={() => dismiss(toast.id)}
                className="-m-1 rounded p-1 text-slate-500 hover:bg-black/5"
                aria-label="Dismiss notification"
              >
                ✕
              </button>
            </div>
            {toast.body && <p className="mt-1 text-sm text-slate-700">{toast.body}</p>}
          </div>
        ))}
      </div>
    </ToastContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useToast = () => useContext(ToastContext)
