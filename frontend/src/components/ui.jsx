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

const iconButtonTones = {
  default:
    'text-[var(--text-muted)] hover:bg-[var(--surface)] hover:text-[var(--text)]',
  accent:
    'text-[var(--accent)] hover:bg-[var(--accent-soft)]',
  danger:
    'text-[var(--danger)] hover:bg-[var(--danger-soft)]',
}

export function iconButtonClass(tone = 'default', className = '') {
  return [
    'inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md transition-colors active:scale-[0.96] disabled:pointer-events-none disabled:opacity-50',
    iconButtonTones[tone] || iconButtonTones.default,
    className,
  ].join(' ')
}

export function IconButton({ label, tone = 'default', children, className = '', ...props }) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      className={iconButtonClass(tone, className)}
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

export function Toggle({
  checked,
  onChange,
  label,
  description,
  disabled,
  className = '',
  'aria-label': ariaLabel,
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel || label}
      disabled={disabled}
      onClick={() => onChange?.(!checked)}
      className={[
        'flex items-start gap-3 text-left',
        disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer',
        className,
      ].join(' ')}
    >
      <span
        className={[
          'relative mt-0.5 inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors',
          checked ? 'bg-[var(--accent)]' : 'bg-[var(--border-strong)]',
        ].join(' ')}
      >
        <span
          className={[
            'inline-block h-4 w-4 rounded-full bg-white shadow-sm transition-transform',
            checked ? 'translate-x-4' : 'translate-x-0.5',
          ].join(' ')}
        />
      </span>
      {(label || description) && (
        <span className="min-w-0 flex-1">
          {label ? (
            <span className="block text-sm font-medium text-[var(--text)]">{label}</span>
          ) : null}
          {description ? (
            <span className="mt-0.5 block text-xs text-[var(--text-muted)]">{description}</span>
          ) : null}
        </span>
      )}
    </button>
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
    sqlite: { bg: 'var(--sqlite-soft)', color: 'var(--sqlite)' },
    typesense: { bg: 'var(--typesense-soft)', color: 'var(--typesense)' },
    mongodb: { bg: 'var(--mongodb-soft)', color: 'var(--mongodb)' },
    redis: { bg: 'var(--redis-soft)', color: 'var(--redis)' },
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

export const tableClassName = 'w-full min-w-[720px] border-collapse text-left text-sm'

export function TableShell({ children, footer }) {
  return (
    <div className="animate-fade-up min-w-0 max-w-full overflow-hidden rounded-[var(--radius)] border border-[var(--border)] bg-[var(--surface)] shadow-[var(--shadow-md)]">
      <div className="overflow-x-auto">{children}</div>
      {footer}
    </div>
  )
}

export function Th({ children, className = '' }) {
  return (
    <th
      className={[
        'border-b border-[var(--border)] bg-[var(--bg-elevated)] px-4 py-3 font-mono text-[11px] font-semibold tracking-[0.12em] text-[var(--text-muted)] uppercase',
        className,
      ].join(' ')}
    >
      {children}
    </th>
  )
}

export function Td({ children, className = '' }) {
  return (
    <td className={['border-b border-[var(--border)] px-4 py-3 align-middle text-[var(--text)]', className].join(' ')}>
      {children}
    </td>
  )
}

/** Build a compact page list with ellipses, e.g. [1, '…', 4, 5, 6, '…', 20]. */
export function pageList(current, total) {
  if (total <= 0) return []
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)

  const set = new Set([1, total, current])
  for (let d = 1; d <= 1; d++) {
    set.add(current - d)
    set.add(current + d)
  }
  if (current <= 3) {
    set.add(2)
    set.add(3)
    set.add(4)
  }
  if (current >= total - 2) {
    set.add(total - 1)
    set.add(total - 2)
    set.add(total - 3)
  }

  const sorted = [...set].filter((p) => p >= 1 && p <= total).sort((a, b) => a - b)
  const items = []
  let prev = 0
  for (const p of sorted) {
    if (prev && p - prev > 1) items.push('…')
    items.push(p)
    prev = p
  }
  return items
}

function PaginationNavButton({ label, disabled, onClick, children }) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      disabled={disabled}
      onClick={onClick}
      className="inline-flex h-8 min-w-8 items-center justify-center rounded-md border border-[var(--border)] bg-[var(--surface)] px-2 text-xs font-medium text-[var(--text)] transition-colors hover:border-[var(--border-strong)] hover:bg-[var(--bg-elevated)] disabled:pointer-events-none disabled:opacity-40"
    >
      {children}
    </button>
  )
}

export function Pagination({
  page,
  totalPages,
  total,
  pageSize,
  onPageChange,
  pageSizeOptions,
  onPageSizeChange,
  disabled,
}) {
  if (!total) return null
  const from = (page - 1) * pageSize + 1
  const to = Math.min(page * pageSize, total)
  const pages = Math.max(totalPages, 1)
  const canPrev = page > 1
  const canNext = page < pages
  const showNav = pages > 1
  const sizes = Array.isArray(pageSizeOptions) && pageSizeOptions.length > 0 ? pageSizeOptions : null

  return (
    <div className="flex flex-col gap-3 border-t border-[var(--border)] bg-[var(--bg-elevated)]/60 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex flex-wrap items-center gap-3">
        <p className="text-xs text-[var(--text-muted)]">
          Showing <span className="font-mono text-[var(--text)]">{from}</span>
          –<span className="font-mono text-[var(--text)]">{to}</span> of{' '}
          <span className="font-mono text-[var(--text)]">{total}</span>
        </p>
        {sizes && onPageSizeChange ? (
          <label className="inline-flex items-center gap-2 text-xs text-[var(--text-muted)]">
            <span className="sr-only">Rows per page</span>
            <select
              className="rounded-md border border-[var(--border)] bg-[var(--surface)] px-2 py-1 font-mono text-xs text-[var(--text)] outline-none focus:border-[var(--accent)]"
              value={pageSize}
              disabled={disabled}
              onChange={(e) => onPageSizeChange(Number(e.target.value))}
            >
              {sizes.map((size) => (
                <option key={size} value={size}>
                  {size} / page
                </option>
              ))}
            </select>
          </label>
        ) : null}
      </div>

      {showNav ? (
        <nav className="flex flex-wrap items-center gap-1" aria-label="Pagination">
          <PaginationNavButton
            label="Previous page"
            disabled={disabled || !canPrev}
            onClick={() => onPageChange(page - 1)}
          >
            ‹
          </PaginationNavButton>
          {pageList(page, pages).map((item, i) =>
            item === '…' ? (
              <span
                key={`ellipsis-${i}`}
                className="inline-flex h-8 w-8 items-center justify-center font-mono text-xs text-[var(--text-muted)]"
                aria-hidden="true"
              >
                …
              </span>
            ) : (
              <button
                key={item}
                type="button"
                aria-label={`Page ${item}`}
                aria-current={item === page ? 'page' : undefined}
                disabled={disabled || item === page}
                onClick={() => onPageChange(item)}
                className={[
                  'inline-flex h-8 min-w-8 items-center justify-center rounded-md px-2 font-mono text-xs font-medium transition-colors disabled:pointer-events-none',
                  item === page
                    ? 'bg-[var(--accent)] text-white shadow-[var(--shadow-sm)] disabled:opacity-100'
                    : 'border border-[var(--border)] bg-[var(--surface)] text-[var(--text)] hover:border-[var(--border-strong)] hover:bg-[var(--bg-elevated)] disabled:opacity-40',
                ].join(' ')}
              >
                {item}
              </button>
            ),
          )}
          <PaginationNavButton
            label="Next page"
            disabled={disabled || !canNext}
            onClick={() => onPageChange(page + 1)}
          >
            ›
          </PaginationNavButton>
        </nav>
      ) : null}
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
