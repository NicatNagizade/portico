import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { parsePage, parsePageSize } from '../api/pagination'
import { listSyncJobs } from '../api/syncJobs'
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
import { SYNC_LOG_STATUS_OPTIONS } from '../lib/syncLogStatus'

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

const STATUS_VALUES = new Set(SYNC_LOG_STATUS_OPTIONS.map((option) => option.value))

function parseStatus(raw) {
  return STATUS_VALUES.has(raw) ? raw : ''
}

function parseDay(raw) {
  if (!raw || !/^\d{4}-\d{2}-\d{2}$/.test(raw)) return ''
  const [year, month, day] = raw.split('-').map(Number)
  const date = new Date(year, month - 1, day)
  if (
    date.getFullYear() !== year ||
    date.getMonth() !== month - 1 ||
    date.getDate() !== day
  ) {
    return ''
  }
  return raw
}

/** Local calendar day as an ISO instant. `end` is the start of the next day. */
function dayInstant(day, end) {
  const [year, month, date] = day.split('-').map(Number)
  return new Date(year, month - 1, date + (end ? 1 : 0)).toISOString()
}

function buildParams({ syncJobId, status, from, to, page, pageSize }) {
  const next = {}
  if (syncJobId) next.sync_job_id = syncJobId
  if (status) next.status = status
  if (from) next.from = from
  if (to) next.to = to
  if (page > 1) next.page = String(page)
  if (pageSize !== 20) next.page_size = String(pageSize)
  return next
}

function emptyMessage({ jobName, jobId, status, from, to }) {
  const parts = []
  if (jobName) parts.push(jobName)
  else if (jobId) parts.push(`job #${jobId}`)
  if (status) parts.push(status)
  if (from || to) {
    parts.push([from && `from ${from}`, to && `through ${to}`].filter(Boolean).join(' '))
  }
  if (parts.length === 0) return 'Run a sync job to see history here.'
  return `Nothing recorded for ${parts.join(', ')}.`
}

export default function SyncLogsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const filterJobId = searchParams.get('sync_job_id') || ''
  const filterStatus = parseStatus(searchParams.get('status'))
  const filterFrom = parseDay(searchParams.get('from'))
  const filterTo = parseDay(searchParams.get('to'))
  const page = parsePage(searchParams.get('page'))
  const pageSize = parsePageSize(searchParams.get('page_size'), { options: PAGE_SIZE_OPTIONS })
  const [items, setItems] = useState([])
  const [jobs, setJobs] = useState([])
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(0)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState('')
  const requestId = useRef(0)
  const filters = { syncJobId: filterJobId, status: filterStatus, from: filterFrom, to: filterTo }

  useEffect(() => {
    listSyncJobs({ page: 1, pageSize: 100 })
      .then((data) => setJobs(data?.items || []))
      .catch(() => setJobs([]))
  }, [])

  const load = useCallback(async () => {
    const id = ++requestId.current
    setLoading(true)
    setRefreshing(true)
    setError('')
    try {
      const data = await listSyncLogs({
        syncJobId: filterJobId || undefined,
        status: filterStatus || undefined,
        from: filterFrom ? dayInstant(filterFrom, false) : undefined,
        to: filterTo ? dayInstant(filterTo, true) : undefined,
        page,
        pageSize,
      })
      if (requestId.current !== id) return
      setItems(data?.items || [])
      setTotal(data?.total ?? 0)
      setTotalPages(data?.total_pages ?? 0)
      if (data?.total_pages > 0 && page > data.total_pages) {
        setSearchParams(
          buildParams({
            syncJobId: filterJobId,
            status: filterStatus,
            from: filterFrom,
            to: filterTo,
            page: data.total_pages,
            pageSize,
          }),
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
  }, [filterJobId, filterStatus, filterFrom, filterTo, page, pageSize, setSearchParams])

  useEffect(() => {
    load()
  }, [load])

  function setFilters(patch) {
    setSearchParams(buildParams({ ...filters, ...patch, page: 1, pageSize }))
  }

  function setPage(next) {
    setSearchParams(buildParams({ ...filters, page: next, pageSize }))
  }

  function setPageSize(next) {
    setSearchParams(buildParams({ ...filters, page: 1, pageSize: next }))
  }

  const jobOptions = [...jobs].sort((a, b) => a.name.localeCompare(b.name))
  const selectedJob = jobOptions.find((job) => String(job.id) === filterJobId)
  const hasFilters = Boolean(filterJobId || filterStatus || filterFrom || filterTo)

  return (
    <div>
      <PageHeader
        eyebrow="History"
        title="Sync logs"
        description="Every run leaves a trail. Filter by job, status, or the day it started."
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

      <div className="animate-fade-up mb-5 flex flex-wrap items-end gap-3 rounded-[var(--radius)] border border-[var(--border)] bg-[var(--surface)] p-4 shadow-[var(--shadow-sm)]">
        <div className="w-64">
          <Field label="Job">
            <select
              className={inputClassName}
              value={filterJobId}
              onChange={(e) => setFilters({ syncJobId: e.target.value })}
            >
              <option value="">All jobs</option>
              {filterJobId && !selectedJob ? (
                <option value={filterJobId}>Job #{filterJobId}</option>
              ) : null}
              {jobOptions.map((job) => (
                <option key={job.id} value={String(job.id)}>
                  {job.name}
                </option>
              ))}
            </select>
          </Field>
        </div>
        <div className="w-40">
          <Field label="Status">
            <select
              className={inputClassName}
              value={filterStatus}
              onChange={(e) => setFilters({ status: e.target.value })}
            >
              <option value="">All statuses</option>
              {SYNC_LOG_STATUS_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </Field>
        </div>
        <div className="w-40">
          <Field label="Started from">
            <input
              type="date"
              className={inputClassName}
              value={filterFrom}
              max={filterTo || undefined}
              onChange={(e) => setFilters({ from: e.target.value })}
            />
          </Field>
        </div>
        <div className="w-40">
          <Field label="Started through">
            <input
              type="date"
              className={inputClassName}
              value={filterTo}
              min={filterFrom || undefined}
              onChange={(e) => setFilters({ to: e.target.value })}
            />
          </Field>
        </div>
        {hasFilters ? (
          <SecondaryButton type="button" onClick={() => setFilters({ syncJobId: '', status: '', from: '', to: '' })}>
            Clear
          </SecondaryButton>
        ) : null}
      </div>

      <ErrorBanner message={error} />

      {loading && items.length === 0 ? (
        <LoadingState />
      ) : !loading && items.length === 0 ? (
        <EmptyState
          title="No sync logs"
          message={emptyMessage({
            jobName: selectedJob?.name,
            jobId: filterJobId,
            status: filterStatus,
            from: filterFrom,
            to: filterTo,
          })}
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
