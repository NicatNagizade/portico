import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getSyncLog, stopSyncLog } from '../api/syncLogs'
import StatusBadge from '../components/StatusBadge'
import SyncProgress from '../components/SyncProgress'
import {
  DangerButton,
  ErrorBanner,
  LoadingState,
  MetaChip,
  PageHeader,
  Panel,
  SecondaryButton,
} from '../components/ui'
import { formatDate, formatDuration } from '../lib/destinationTypes'
import { isSyncLogRunning } from '../lib/syncLogStatus'

const POLL_MS = 750

export default function SyncLogDetailPage() {
  const { id } = useParams()
  const [log, setLog] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [stopping, setStopping] = useState(false)

  useEffect(() => {
    let active = true
    let timer = null

    async function load(initial) {
      if (initial) setLoading(true)
      try {
        const data = await getSyncLog(id)
        if (!active) return
        setLog(data)
        setError('')
        if (isSyncLogRunning(data.status)) {
          timer = setTimeout(() => load(false), POLL_MS)
        }
      } catch (err) {
        if (active) setError(err.message || 'Failed to load sync log')
      } finally {
        if (active && initial) setLoading(false)
      }
    }

    load(true)
    return () => {
      active = false
      if (timer != null) clearTimeout(timer)
    }
  }, [id])

  async function handleStop() {
    setStopping(true)
    setError('')
    try {
      const data = await stopSyncLog(id)
      setLog(data)
    } catch (err) {
      setError(err.message || 'Failed to stop sync')
    } finally {
      setStopping(false)
    }
  }

  if (loading) {
    return (
      <div>
        <PageHeader eyebrow="Sync logs" title="Loading…" />
        <LoadingState rows={3} />
      </div>
    )
  }

  if (!log) {
    return (
      <div>
        <PageHeader eyebrow="Sync logs" title="Sync log" />
        <ErrorBanner message={error || 'Sync log not found'} />
      </div>
    )
  }

  const running = isSyncLogRunning(log.status)

  return (
    <div>
      <PageHeader
        eyebrow="Sync logs"
        title={`Log #${log.id}`}
        description={log.sync_job?.name || `Job #${log.sync_job_id}`}
        actions={
          <>
            <Link to="/sync-logs">
              <SecondaryButton>All logs</SecondaryButton>
            </Link>
            <Link to={`/sync-logs?sync_job_id=${log.sync_job_id}`}>
              <SecondaryButton>Job logs</SecondaryButton>
            </Link>
            <Link to={`/sync-jobs/${log.sync_job_id}`}>
              <SecondaryButton>View job</SecondaryButton>
            </Link>
            {running ? (
              <DangerButton onClick={handleStop} disabled={stopping}>
                {stopping ? 'Stopping…' : 'Stop'}
              </DangerButton>
            ) : null}
          </>
        }
      />

      <ErrorBanner message={error} />

      <div className="animate-fade-up mb-4 rounded-[var(--radius)] border border-[var(--border)] bg-[var(--surface)] p-4 shadow-[var(--shadow-sm)]">
        <div className="mb-4 flex flex-wrap items-center gap-3">
          <StatusBadge status={log.status} />
          <MetaChip>{formatDuration(log.duration_ms)}</MetaChip>
          <MetaChip>started {formatDate(log.started_at)}</MetaChip>
        </div>
        <SyncProgress log={log} />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Panel title="Timing" className="animate-fade-up stagger-1">
          <dl className="space-y-3 text-sm">
            <div className="flex justify-between gap-4 border-b border-[var(--border)] pb-3">
              <dt className="text-[var(--text-muted)]">Rows total</dt>
              <dd className="font-mono font-medium">{log.rows_total ?? '—'}</dd>
            </div>
            <div className="flex justify-between gap-4 border-b border-[var(--border)] pb-3">
              <dt className="text-[var(--text-muted)]">Rows synced</dt>
              <dd className="font-mono font-medium">{log.rows_synced ?? '—'}</dd>
            </div>
            <div className="flex justify-between gap-4 border-b border-[var(--border)] pb-3">
              <dt className="text-[var(--text-muted)]">Started</dt>
              <dd>{formatDate(log.started_at)}</dd>
            </div>
            <div className="flex justify-between gap-4 border-b border-[var(--border)] pb-3">
              <dt className="text-[var(--text-muted)]">Finished</dt>
              <dd>{formatDate(log.finished_at)}</dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-[var(--text-muted)]">Duration</dt>
              <dd className="font-mono font-medium">{formatDuration(log.duration_ms)}</dd>
            </div>
          </dl>
        </Panel>

        <Panel title="Message" className="animate-fade-up stagger-2 lg:col-span-1">
          <pre className="max-h-[420px] overflow-auto whitespace-pre-wrap rounded-xl bg-[var(--bg-elevated)] p-4 font-mono text-xs leading-relaxed text-[var(--text)]">
            {log.message || '—'}
          </pre>
        </Panel>
      </div>
    </div>
  )
}
