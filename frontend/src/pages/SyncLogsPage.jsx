import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { parsePage, parsePageSize } from '../api/pagination'
import { listSyncLogs } from '../api/syncLogs'
import StatusBadge from '../components/StatusBadge'
import SyncProgress from '../components/SyncProgress'
import {
  EmptyState,
  ErrorBanner,
  Field,
  LoadingState,
  MetaChip,
  PageHeader,
  Pagination,
  SecondaryButton,
  TableShell,
  Td,
  Th,
  inputClassName,
  tableClassName,
} from '../components/ui'
import { formatDate, formatDuration } from '../lib/format'

const PAGE_SIZE_OPTIONS = [10, 20, 50]

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

function buildParams({ syncJobId, page, pageSize }) {
  const next = {}
  if (syncJobId) next.sync_job_id = syncJobId
  if (page > 1) next.page = String(page)
  if (pageSize !== 20) next.page_size = String(pageSize)
  return next
}

export default function SyncLogsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const filterJobId = searchParams.get('sync_job_id') || ''
  const page = parsePage(searchParams.get('page'))
  const pageSize = parsePageSize(searchParams.get('page_size'), { options: PAGE_SIZE_OPTIONS })
  const [items, setItems] = useState([])
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(0)
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
      const data = await listSyncLogs({
        syncJobId: filterJobId || undefined,
        page,
        pageSize,
      })
      if (requestId.current !== id) return
      setItems(data?.items || [])
      setTotal(data?.total ?? 0)
      setTotalPages(data?.total_pages ?? 0)
      if (data?.total_pages > 0 && page > data.total_pages) {
        setSearchParams(
          buildParams({ syncJobId: filterJobId, page: data.total_pages, pageSize }),
          { replace: true },
        )
      }
    } catch (err) {
      if (requestId.current !== id) return
      setError(err.message || 'Failed to load sync logs')
    } finally {
      if (requestId.current === id) {
        setLoading(false)
        setRefreshing(false)
      }
    }
  }, [filterJobId, page, pageSize, setSearchParams])

  useEffect(() => {
    load()
  }, [load])

  function applyFilter(event) {
    event.preventDefault()
    const next = draftFilter.trim()
    setSearchParams(buildParams({ syncJobId: next, page: 1, pageSize }))
  }

  function setPage(next) {
    setSearchParams(buildParams({ syncJobId: filterJobId, page: next, pageSize }))
  }

  function setPageSize(next) {
    setSearchParams(buildParams({ syncJobId: filterJobId, page: 1, pageSize: next }))
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
              setSearchParams(buildParams({ page: 1, pageSize }))
            }}
          >
            Clear
          </SecondaryButton>
        ) : null}
      </form>

      <ErrorBanner message={error} />

      {loading && items.length === 0 ? (
        <LoadingState />
      ) : !loading && items.length === 0 ? (
        <EmptyState
          title="No sync logs"
          message={
            filterJobId
              ? `Nothing recorded for job #${filterJobId} yet.`
              : 'Run a sync job to see history here.'
          }
        />
      ) : (
        <TableShell
          footer={
            <Pagination
              page={page}
              totalPages={totalPages}
              total={total}
              pageSize={pageSize}
              pageSizeOptions={PAGE_SIZE_OPTIONS}
              onPageChange={setPage}
              onPageSizeChange={setPageSize}
              disabled={loading || refreshing}
            />
          }
        >
          <table className={tableClassName}>
            <thead>
              <tr>
                <Th>Status</Th>
                <Th>Job</Th>
                <Th>Started</Th>
                <Th>Duration</Th>
                <Th>Progress</Th>
                <Th className="text-right"> </Th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id} className="transition-colors hover:bg-[var(--bg-elevated)]/70">
                  <Td>
                    <StatusBadge status={item.status} />
                  </Td>
                  <Td>
                    <div className="flex flex-wrap items-center gap-2">
                      <Link
                        to={`/sync-jobs/${item.sync_job_id}`}
                        className="font-semibold text-[var(--text)] transition-colors hover:text-[var(--accent)]"
                      >
                        {item.sync_job?.name || `Job #${item.sync_job_id}`}
                      </Link>
                      <MetaChip>log #{item.id}</MetaChip>
                    </div>
                    {item.message ? (
                      <p className="mt-1 line-clamp-1 max-w-md text-xs text-[var(--text-muted)]">
                        {item.message}
                      </p>
                    ) : null}
                  </Td>
                  <Td>
                    <span className="text-xs text-[var(--text-muted)] whitespace-nowrap">
                      {formatDate(item.started_at)}
                    </span>
                  </Td>
                  <Td>
                    <span className="font-mono text-xs text-[var(--text-muted)]">
                      {formatDuration(item.duration_ms)}
                    </span>
                  </Td>
                  <Td>
                    <SyncProgress log={item} className="min-w-[10rem] max-w-xs" />
                  </Td>
                  <Td className="text-right">
                    <Link to={`/sync-logs/${item.id}`}>
                      <SecondaryButton>Details</SecondaryButton>
                    </Link>
                  </Td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableShell>
      )}
    </div>
  )
}
