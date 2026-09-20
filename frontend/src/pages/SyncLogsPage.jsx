import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { listSyncLogs } from '../api/syncLogs'
import StatusBadge from '../components/StatusBadge'
import SyncProgress from '../components/SyncProgress'
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

function RefreshIcon({ spinning }) {
  return (
    <svg
      className={spinning ? 'animate-spin' : undefined}
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
    >
      <path
        d="M20 12a8 8 0 1 1-2.3-5.7"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
      />
      <path
        d="M20 4v5h-5"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}

export default function SyncLogsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const filterJobId = searchParams.get('sync_job_id') || ''
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState('')
  const [draftFilter, setDraftFilter] = useState(filterJobId)
  const requestId = useRef(0)

  useEffect(() => {
    setDraftFilter(filterJobId)
  }, [filterJobId])

  const load = useCallback(async () => {
    const id = ++requestId.current
    setLoading(true)
    setRefreshing(true)
    setError('')
    try {
      const data = await listSyncLogs(filterJobId || undefined)
      if (requestId.current !== id) return
      setItems(data || [])
    } catch (err) {
      if (requestId.current !== id) return
      setError(err.message || 'Failed to load sync logs')
    } finally {
      if (requestId.current === id) {
        setLoading(false)
        setRefreshing(false)
      }
    }
  }, [filterJobId])

  useEffect(() => {
    load()
  }, [load])

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
        actions={
          <SecondaryButton
            onClick={load}
            disabled={loading || refreshing}
            aria-busy={refreshing}
          >
            <RefreshIcon spinning={refreshing} />
            Refresh
          </SecondaryButton>
        }
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

      {loading || refreshing ? (
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
                    {formatDuration(item.duration_ms)}
                  </p>
                  <SyncProgress log={item} className="mt-3 max-w-sm" />
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
