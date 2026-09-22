import { isSyncLogRunning } from '../lib/syncLogStatus'

const STYLES = {
  running: {
    bg: 'var(--running-soft)',
    color: 'var(--running)',
  },
  success: {
    bg: 'var(--success-soft)',
    color: 'var(--success)',
  },
  failed: {
    bg: 'var(--danger-soft)',
    color: 'var(--danger)',
  },
  stopped: {
    bg: 'var(--warning-soft)',
    color: 'var(--warning)',
  },
}

export default function StatusBadge({ status }) {
  const style = STYLES[status] || {
    bg: 'var(--bg-elevated)',
    color: 'var(--text-muted)',
  }

  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 font-mono text-[11px] font-semibold tracking-wide uppercase"
      style={{ background: style.bg, color: style.color }}
    >
      <span
        className="h-1.5 w-1.5 rounded-full"
        style={{
          background: style.color,
          animation: isSyncLogRunning(status) ? 'pulse-dot 1.4s ease-in-out infinite' : undefined,
        }}
      />
      {status || 'unknown'}
    </span>
  )
}
