import { isSyncLogRunning } from '../lib/syncLogStatus'

export default function SyncProgress({ log, className = '', indeterminate = false }) {
  const synced = log?.rows_synced ?? 0
  const total = log?.rows_total ?? 0
  const isIndeterminate =
    indeterminate || (total <= 0 && (isSyncLogRunning(log?.status) || !log))
  const pct = !isIndeterminate && total > 0 ? Math.min(100, Math.round((synced / total) * 100)) : 0
  const remaining = Math.max(0, total - synced)

  return (
    <div className={className}>
      <div className="mb-1.5 flex items-center justify-between gap-3 text-xs text-[var(--text-muted)]">
        {isIndeterminate ? (
          <span>Syncing…</span>
        ) : (
          <>
            <span className="font-mono">
              {synced.toLocaleString()} / {total.toLocaleString()} rows
            </span>
            <span>{remaining.toLocaleString()} remaining</span>
          </>
        )}
      </div>
      <div
        className="h-1.5 overflow-hidden rounded-full bg-[var(--bg-elevated)]"
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={isIndeterminate ? undefined : pct}
        aria-label="Sync progress"
      >
        {isIndeterminate ? (
          <div className="progress-indeterminate h-full w-1/3 rounded-full bg-[var(--accent)]" />
        ) : (
          <div
            className="h-full rounded-full bg-[var(--accent)] transition-[width] duration-300 ease-out"
            style={{ width: `${pct}%` }}
          />
        )}
      </div>
    </div>
  )
}
