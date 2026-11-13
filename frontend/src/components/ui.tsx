// Small shared building blocks so every form, list and error looks and behaves the same.
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from 'react'
import { ApiError } from '../api/client'
import type { Category } from '../api/types'
import { CATEGORY_COLORS, CATEGORY_LABELS } from '../lib/format'

export function Button({
  variant = 'primary',
  loading = false,
  children,
  className = '',
  disabled,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: 'primary' | 'secondary' | 'ghost'; loading?: boolean }) {
  const styles = {
    primary: 'bg-slate-900 text-white hover:bg-slate-700 disabled:bg-slate-400',
    secondary: 'border border-slate-300 bg-white text-slate-800 hover:bg-slate-50 disabled:text-slate-400',
    ghost: 'text-slate-700 hover:bg-slate-200/70 disabled:text-slate-400',
  }[variant]
  return (
    <button
      type="button"
      disabled={disabled || loading}
      className={`inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-slate-900 disabled:cursor-not-allowed ${styles} ${className}`}
      {...props}
    >
      {loading && <Spinner />}
      {children}
    </button>
  )
}

export function Field({ label, hint, children }: { label: string; hint?: ReactNode; children: ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 block text-xs font-medium text-slate-700">{label}</span>
      {children}
      {hint && <span className="mt-1 block text-xs text-slate-500">{hint}</span>}
    </label>
  )
}

const inputStyles =
  'w-full rounded-md border border-slate-300 bg-white px-2.5 py-1.5 text-sm shadow-sm placeholder:text-slate-400 focus:border-slate-500 focus:outline-none focus:ring-2 focus:ring-slate-200'

export const Input = (props: InputHTMLAttributes<HTMLInputElement>) => <input className={inputStyles} {...props} />
export const Select = (props: SelectHTMLAttributes<HTMLSelectElement>) => <select className={inputStyles} {...props} />
export const Textarea = (props: TextareaHTMLAttributes<HTMLTextAreaElement>) => (
  <textarea className={`${inputStyles} font-mono text-xs`} {...props} />
)

export function Spinner({ label }: { label?: string }) {
  return (
    <span className="inline-flex items-center gap-2 text-sm text-slate-500" role="status">
      <span className="size-4 animate-spin rounded-full border-2 border-current border-t-transparent" aria-hidden />
      {label ?? <span className="sr-only">Loading</span>}
    </span>
  )
}

/** Shows an API error's message and any per-field details from the backend's validation. */
export function ErrorNotice({ error, title }: { error: unknown; title?: string }) {
  if (!error) return null
  const message = error instanceof Error ? error.message : 'Something went wrong'
  const details = error instanceof ApiError ? error.details : []
  return (
    <div role="alert" className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-800">
      <p className="font-medium">{title ? `${title}: ${message}` : message}</p>
      {details.length > 0 && (
        <ul className="mt-1 list-disc pl-5">
          {details.map((detail) => (
            <li key={detail}>{detail}</li>
          ))}
        </ul>
      )}
    </div>
  )
}

export function EmptyState({ children }: { children: ReactNode }) {
  return <p className="rounded-md border border-dashed border-slate-300 p-4 text-center text-sm text-slate-500">{children}</p>
}

export function CategoryBadge({ category }: { category: Category }) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-700">
      <span className="size-2 rounded-full" style={{ backgroundColor: CATEGORY_COLORS[category] }} aria-hidden />
      {CATEGORY_LABELS[category]}
    </span>
  )
}

export function EventBadge({ type }: { type: string }) {
  const styles: Record<string, string> = {
    entry: 'bg-red-100 text-red-800',
    exit: 'bg-sky-100 text-sky-800',
    both: 'bg-violet-100 text-violet-800',
  }
  return (
    <span className={`rounded px-1.5 py-0.5 text-xs font-semibold uppercase tracking-wide ${styles[type] ?? 'bg-slate-100'}`}>
      {type}
    </span>
  )
}

export function Panel({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
      <div className="mb-3 flex items-center justify-between gap-2">
        <h2 className="text-sm font-semibold text-slate-900">{title}</h2>
        {action}
      </div>
      {children}
    </section>
  )
}
