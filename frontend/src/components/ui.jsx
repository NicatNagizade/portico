export function PageHeader({ eyebrow, title, description, actions }) {
  return (
    <div className="animate-fade-up mb-8 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
      <div className="min-w-0">
        {eyebrow ? (
          <p className="mb-2 font-mono text-[11px] tracking-[0.2em] text-[var(--text-muted)] uppercase">
            {eyebrow}
          </p>
        ) : null}
        <h2 className="text-3xl font-semibold tracking-tight text-[var(--text)] sm:text-[2rem]">
          {title}
        </h2>
        {description ? (
          <p className="mt-2 max-w-2xl text-[15px] leading-relaxed text-[var(--text-muted)]">
            {description}
          </p>
        ) : null}
      </div>
      {actions ? <div className="flex shrink-0 flex-wrap gap-2">{actions}</div> : null}
    </div>
  )
}

export function ErrorBanner({ message }) {
  if (!message) return null
  return (
    <div className="animate-fade-in mb-5 flex gap-3 rounded-[var(--radius)] border border-[var(--danger)]/25 bg-[var(--danger-soft)] px-4 py-3 text-sm text-[var(--danger)] shadow-[var(--shadow-sm)]">
      <span className="mt-0.5 inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-[var(--danger)] text-[10px] font-bold text-white">
        !
      </span>
      <p>{message}</p>
    </div>
  )
}

export function SuccessBanner({ message }) {
  if (!message) return null
  return (
    <div className="animate-fade-in mb-5 rounded-[var(--radius)] border border-[var(--success)]/25 bg-[var(--success-soft)] px-4 py-3 text-sm text-[var(--success)] shadow-[var(--shadow-sm)]">
      {message}
    </div>
  )
}

export function EmptyState({ title, message, action }) {
  return (
    <div className="animate-fade-up rounded-[var(--radius)] border border-dashed border-[var(--border-strong)] bg-[var(--surface)]/80 px-6 py-14 text-center shadow-[var(--shadow-sm)] backdrop-blur-sm">
      <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-2xl bg-[var(--accent-soft)] text-[var(--accent)]">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <path
            d="M12 5v14M5 12h14"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
          />
        </svg>
      </div>
      {title ? <h3 className="text-base font-semibold text-[var(--text)]">{title}</h3> : null}
      <p className="mx-auto mt-2 max-w-md text-sm leading-relaxed text-[var(--text-muted)]">
        {message}
      </p>
      {action ? <div className="mt-5 flex justify-center">{action}</div> : null}
    </div>
  )
}

export function LoadingState({ rows = 4 }) {
  return (
    <div className="animate-fade-in space-y-3 rounded-[var(--radius)] border border-[var(--border)] bg-[var(--surface)] p-4 shadow-[var(--shadow-sm)]">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex items-center gap-3">
          <div className="skeleton h-10 w-10 shrink-0 rounded-xl" />
          <div className="flex-1 space-y-2">
            <div className="skeleton h-3 w-1/3" />
            <div className="skeleton h-3 w-2/3" />
          </div>
        </div>
      ))}
    </div>
  )
}

export function PrimaryButton({ children, className = '', ...props }) {
  return (
    <button
      type="button"
      className={[
        'inline-flex items-center justify-center gap-2 rounded-lg bg-[var(--accent)] px-3.5 py-2 text-sm font-semibold text-white shadow-[var(--shadow-sm)] transition-all hover:bg-[var(--accent-hover)] hover:shadow-[var(--shadow-md)] active:scale-[0.98] disabled:pointer-events-none disabled:opacity-50',
        className,
      ].join(' ')}
      {...props}
    >
      {children}
    </button>
  )
}

export function SecondaryButton({ children, className = '', ...props }) {
  return (
    <button
      type="button"
      className={[
        'inline-flex items-center justify-center gap-2 rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3.5 py-2 text-sm font-medium text-[var(--text)] shadow-[var(--shadow-sm)] transition-all hover:border-[var(--border-strong)] hover:bg-[var(--bg-elevated)] active:scale-[0.98] disabled:pointer-events-none disabled:opacity-50',
        className,
      ].join(' ')}
      {...props}
    >
      {children}
    </button>
  )
}

export function DangerButton({ children, className = '', ...props }) {
  return (
    <button
      type="button"
      className={[
        'inline-flex items-center justify-center gap-2 rounded-lg border border-[var(--danger)]/25 bg-[var(--danger-soft)] px-3.5 py-2 text-sm font-medium text-[var(--danger)] transition-all hover:border-[var(--danger)]/40 active:scale-[0.98] disabled:pointer-events-none disabled:opacity-50',
        className,
      ].join(' ')}
      {...props}
    >
      {children}
    </button>
  )
}

export function GhostButton({ children, className = '', ...props }) {
  return (
    <button
      type="button"
      className={[
        'inline-flex items-center justify-center gap-2 rounded-lg px-2.5 py-1.5 text-sm font-medium text-[var(--text-muted)] transition-colors hover:bg-black/5 hover:text-[var(--text)] disabled:opacity-50',
        className,
      ].join(' ')}
      {...props}
    >
      {children}
    </button>
  )
}

export function Field({ label, children, hint }) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-[13px] font-medium text-[var(--text)]">{label}</span>
      {children}
      {hint ? <span className="mt-1.5 block text-xs text-[var(--text-muted)]">{hint}</span> : null}
    </label>
  )
}

export function Panel({ title, description, children, className = '', actions }) {
  return (
    <section
      className={[
        'rounded-[var(--radius)] border border-[var(--border)] bg-[var(--surface)] p-5 shadow-[var(--shadow-sm)]',
        className,
      ].join(' ')}
    >
      {(title || actions) && (
        <div className="mb-4 flex items-start justify-between gap-3">
          <div>
            {title ? <h3 className="text-sm font-semibold tracking-tight text-[var(--text)]">{title}</h3> : null}
            {description ? (
              <p className="mt-1 text-xs leading-relaxed text-[var(--text-muted)]">{description}</p>
            ) : null}
          </div>
          {actions}
        </div>
      )}
      {children}
    </section>
  )
}

export function TypeChip({ type }) {
  const styles = {
    mysql: { bg: 'var(--mysql-soft)', color: 'var(--mysql)' },
    postgres: { bg: 'var(--postgres-soft)', color: 'var(--postgres)' },
    typesense: { bg: 'var(--typesense-soft)', color: 'var(--typesense)' },
    mongodb: { bg: 'var(--mongodb-soft)', color: 'var(--mongodb)' },
  }
  const style = styles[type] || {
    bg: 'var(--accent-soft)',
    color: 'var(--text-muted)',
  }

  return (
    <span
      className="inline-flex items-center rounded-md px-2 py-0.5 font-mono text-[11px] font-medium tracking-wide uppercase"
      style={{ background: style.bg, color: style.color }}
    >
      {type || 'unknown'}
    </span>
  )
}

export function MetaChip({ children }) {
  return (
    <span className="inline-flex items-center rounded-md bg-[var(--bg-elevated)] px-2 py-0.5 font-mono text-[11px] text-[var(--text-muted)]">
      {children}
    </span>
  )
}

export const inputClassName =
  'w-full rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3 py-2.5 text-sm text-[var(--text)] shadow-[var(--shadow-sm)] outline-none transition-shadow placeholder:text-[var(--text-muted)]/70 focus:border-[var(--accent)] focus:shadow-[0_0_0_3px_var(--accent-soft)]'

export const tableClassName = 'w-full min-w-[640px] border-collapse text-left text-sm'

export function TableShell({ children }) {
  return (
    <div className="animate-fade-up overflow-hidden rounded-[var(--radius)] border border-[var(--border)] bg-[var(--surface)] shadow-[var(--shadow-md)]">
      <div className="overflow-x-auto">{children}</div>
    </div>
  )
}

export function ListStack({ children }) {
  return <div className="space-y-3">{children}</div>
}

export function ListRow({ children, className = '', style }) {
  return (
    <div
      style={style}
      className={[
        'animate-fade-up group flex flex-col gap-4 rounded-[var(--radius)] border border-[var(--border)] bg-[var(--surface)] p-4 shadow-[var(--shadow-sm)] transition-all hover:-translate-y-0.5 hover:border-[var(--border-strong)] hover:shadow-[var(--shadow-md)] sm:flex-row sm:items-center sm:justify-between',
        className,
      ].join(' ')}
    >
      {children}
    </div>
  )
}
