export default function SyncProgress({ log, className = '' }) {
  const synced = log?.rows_synced ?? 0
  const total = log?.rows_total ?? 0
  const pct = total > 0 ? Math.min(100, Math.round((synced / total) * 100)) : 0
  const remaining = Math.max(0, total - synced)

  return (
    <div className={className}>
      <div className="mb-1.5 flex items-center justify-between gap-3 text-xs text-[var(--text-muted)]">
        <span className="font-mono">
          {synced.toLocaleString()} / {total.toLocaleString()} rows
        </span>
        <span>{remaining.toLocaleString()} remaining</span>
      </div>
      <div
        className="h-1.5 overflow-hidden rounded-full bg-[var(--bg-elevated)]"
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={pct}
        aria-label="Sync progress"
      >
        <div className="h-full rounded-full bg-[var(--accent)]" style={{ width: `${pct}%` }} />
      </div>
    </div>
  )
}
