import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { listSyncLogs } from '../api/syncLogs'
import StatusBadge from '../components/StatusBadge'
import {
  EmptyState,
  ErrorBanner,
  Field,
  ListRow,
  ListStack,
  LoadingState,
  MetaChip,
  PageHeader,
  SecondaryButton,
  inputClassName,
} from '../components/ui'
import { formatDate, formatDuration } from '../lib/destinationTypes'

export default function SyncLogsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const filterJobId = searchParams.get('sync_job_id') || ''
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [draftFilter, setDraftFilter] = useState(filterJobId)

  useEffect(() => {
    setDraftFilter(filterJobId)
  }, [filterJobId])

  useEffect(() => {
    let active = true
    setLoading(true)
    setError('')
    listSyncLogs(filterJobId || undefined)
      .then((data) => {
        if (active) setItems(data || [])
      })
      .catch((err) => {
        if (active) setError(err.message || 'Failed to load sync logs')
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [filterJobId])

  function applyFilter(event) {
    event.preventDefault()
    const next = draftFilter.trim()
    if (next) {
      setSearchParams({ sync_job_id: next })
    } else {
      setSearchParams({})
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="History"
        title="Sync logs"
        description="Every run leaves a trail — filter by job to debug a single pipeline."
      />

      <form
        onSubmit={applyFilter}
        className="animate-fade-up mb-5 flex flex-wrap items-end gap-3 rounded-[var(--radius)] border border-[var(--border)] bg-[var(--surface)] p-4 shadow-[var(--shadow-sm)]"
      >
        <div className="w-52">
          <Field label="Sync job ID">
            <input
              className={inputClassName}
              value={draftFilter}
              onChange={(e) => setDraftFilter(e.target.value)}
              placeholder="e.g. 1"
            />
          </Field>
        </div>
        <SecondaryButton type="submit">Apply filter</SecondaryButton>
        {filterJobId ? (
          <SecondaryButton
            type="button"
            onClick={() => {
              setDraftFilter('')
              setSearchParams({})
            }}
          >
            Clear
          </SecondaryButton>
        ) : null}
      </form>

      <ErrorBanner message={error} />

      {loading ? (
        <LoadingState />
      ) : items.length === 0 ? (
        <EmptyState
          title="No sync logs"
          message={
            filterJobId
              ? `Nothing recorded for job #${filterJobId} yet.`
              : 'Run a sync job to see history here.'
          }
        />
      ) : (
        <ListStack>
          {items.map((item, index) => (
            <ListRow key={item.id} className={`stagger-${Math.min(index + 1, 5)}`}>
              <div className="flex min-w-0 items-start gap-3">
                <div className="pt-0.5">
                  <StatusBadge status={item.status} />
                </div>
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="font-semibold text-[var(--text)]">
                      {item.sync_job?.name || `Job #${item.sync_job_id}`}
                    </h3>
                    <MetaChip>log #{item.id}</MetaChip>
                  </div>
                  <p className="mt-1 text-sm text-[var(--text-muted)]">
                    {formatDate(item.started_at)}
                    <span className="mx-2 text-[var(--border-strong)]">·</span>
                    {item.rows_synced ?? '—'}
                    {item.rows_total != null ? ` / ${item.rows_total}` : ''} rows
                    <span className="mx-2 text-[var(--border-strong)]">·</span>
                    {formatDuration(item.duration_ms)}
                  </p>
                  {item.message ? (
                    <p className="mt-2 line-clamp-2 max-w-2xl text-xs text-[var(--text-muted)]">
                      {item.message}
                    </p>
                  ) : null}
                </div>
              </div>
              <Link to={`/sync-logs/${item.id}`}>
                <SecondaryButton>Details</SecondaryButton>
              </Link>
            </ListRow>
          ))}
        </ListStack>
      )}
    </div>
  )
}
