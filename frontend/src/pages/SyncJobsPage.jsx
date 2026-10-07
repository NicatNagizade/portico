import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { listConnections } from '../api/connections'
import { parsePage, parsePageSize } from '../api/pagination'
import { deleteSyncJob, listSyncJobs, startSyncJob } from '../api/syncJobs'
import ConfirmDialog from '../components/ConfirmDialog'
import { DeleteIcon, EditIcon, LogsIcon, OpenIcon, PlayIcon, SpinnerIcon } from '../components/icons'
import {
  EmptyState,
  ErrorBanner,
  IconButton,
  iconButtonClass,
  LoadingState,
  MetaChip,
  PageHeader,
  Pagination,
  PrimaryButton,
  TableShell,
  Td,
  Th,
  inputClassName,
  tableClassName,
} from '../components/ui'
import { formatDate } from '../lib/format'

const PAGE_SIZE_OPTIONS = [10, 20, 50]

function buildParams({ connectionId, page, pageSize }) {
  const next = {}
  if (connectionId) next.connection_id = connectionId
  if (page > 1) next.page = String(page)
  if (pageSize !== 20) next.page_size = String(pageSize)
  return next
}

export default function SyncJobsPage() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const filterConnectionId = searchParams.get('connection_id') || ''
  const page = parsePage(searchParams.get('page'))
  const pageSize = parsePageSize(searchParams.get('page_size'), { options: PAGE_SIZE_OPTIONS })
  const [items, setItems] = useState([])
  const [connections, setConnections] = useState([])
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [pendingDelete, setPendingDelete] = useState(null)
  const [deleting, setDeleting] = useState(false)
  const [runningId, setRunningId] = useState(null)

  useEffect(() => {
    listConnections()
      .then((data) => setConnections(data || []))
      .catch(() => setConnections([]))
  }, [])

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const data = await listSyncJobs({
        connectionId: filterConnectionId || undefined,
        page,
        pageSize,
      })
      setItems(data?.items || [])
      setTotal(data?.total ?? 0)
      setTotalPages(data?.total_pages ?? 0)
      if (data?.total_pages > 0 && page > data.total_pages) {
        setSearchParams(
          buildParams({ connectionId: filterConnectionId, page: data.total_pages, pageSize }),
          { replace: true },
        )
      }
    } catch (err) {
      setError(err.message || 'Failed to load sync jobs')
    } finally {
      setLoading(false)
    }
  }, [filterConnectionId, page, pageSize, setSearchParams])

  useEffect(() => {
    load()
  }, [load])

  function setConnectionFilter(connectionId) {
    setSearchParams(buildParams({ connectionId, page: 1, pageSize }))
  }

  function setPage(next) {
    setSearchParams(buildParams({ connectionId: filterConnectionId, page: next, pageSize }))
  }

  function setPageSize(next) {
    setSearchParams(buildParams({ connectionId: filterConnectionId, page: 1, pageSize: next }))
  }

  async function handleRun(job) {
    setRunningId(job.id)
    setError('')
    try {
      const log = await startSyncJob(job.id)
      navigate(`/sync-logs/${log.id}`)
    } catch (err) {
      setError(err.message || 'Failed to start sync job')
      setRunningId(null)
    }
  }

  async function confirmDelete() {
    if (!pendingDelete) return
    setDeleting(true)
    setError('')
    try {
      await deleteSyncJob(pendingDelete.id)
      setPendingDelete(null)
      await load()
    } catch (err) {
      setError(err.message || 'Failed to delete sync job')
    } finally {
      setDeleting(false)
    }
  }

  const filterConnection = connections.find((c) => String(c.id) === filterConnectionId)

  return (
    <div>
      <PageHeader
        eyebrow="Pipelines"
        title="Sync jobs"
        description="Map source tables to destination collections, then run a full reload whenever you need a fresh copy."
        actions={
          <Link to="/sync-jobs/new">
            <PrimaryButton>
              <span aria-hidden="true">+</span> New sync job
            </PrimaryButton>
          </Link>
        }
      />

      <ErrorBanner message={error} />

      <div className="mb-3 flex justify-end">
        <div className="w-56">
          <select
            aria-label="Filter by connection"
            className={inputClassName}
            value={filterConnectionId}
            onChange={(e) => setConnectionFilter(e.target.value)}
          >
            <option value="">All connections</option>
            {connections.map((conn) => (
              <option key={conn.id} value={String(conn.id)}>
                {conn.name} (#{conn.id})
              </option>
            ))}
          </select>
        </div>
      </div>

      {loading && items.length === 0 ? (
        <LoadingState />
      ) : !loading && items.length === 0 ? (
        <EmptyState
          title="No sync jobs yet"
          message={
            filterConnectionId
              ? `No jobs use ${filterConnection?.name || `connection #${filterConnectionId}`} as a source or destination.`
              : 'Create a job after you have at least one source and one destination connection.'
          }
          action={
            filterConnectionId ? null : (
              <Link to="/sync-jobs/new">
                <PrimaryButton>Create sync job</PrimaryButton>
              </Link>
            )
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
              disabled={loading}
            />
          }
        >
          <table className={tableClassName}>
            <thead>
              <tr>
                <Th>Job</Th>
                <Th>Source</Th>
                <Th>Destination</Th>
                <Th>Updated</Th>
                <Th className="text-right">Actions</Th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id} className="transition-colors hover:bg-[var(--bg-elevated)]/70">
                  <Td>
                    <div className="flex flex-wrap items-center gap-2">
                      <Link
                        to={`/sync-jobs/${item.id}`}
                        className="font-semibold text-[var(--text)] transition-colors hover:text-[var(--accent)]"
                      >
                        {item.name}
                      </Link>
                      <MetaChip>#{item.id}</MetaChip>
                    </div>
                  </Td>
                  <Td>
                    <div className="min-w-0">
                      <p className="truncate font-medium">
                        {item.source_connection?.name || `#${item.source_connection_id}`}
                      </p>
                      <p className="mt-0.5 font-mono text-xs text-[var(--text-muted)]">
                        {item.source_table}
                      </p>
                    </div>
                  </Td>
                  <Td>
                    <div className="min-w-0">
                      <p className="truncate font-medium">
                        {item.destination_connection?.name || `#${item.destination_connection_id}`}
                      </p>
                      <p className="mt-0.5 font-mono text-xs text-[var(--text-muted)]">
                        {item.destination_table}
                      </p>
                    </div>
                  </Td>
                  <Td>
                    <span className="text-xs text-[var(--text-muted)] whitespace-nowrap">
                      {formatDate(item.updated_at)}
                    </span>
                  </Td>
                  <Td className="text-right">
                    <div className="inline-flex items-center gap-px rounded-lg border border-[var(--border)] bg-[var(--bg-elevated)] p-0.5">
                      <Link
                        to={`/sync-jobs/${item.id}`}
                        title="Open"
                        aria-label={`Open ${item.name}`}
                        className={iconButtonClass()}
                      >
                        <OpenIcon />
                      </Link>
                      <Link
                        to={`/sync-jobs/${item.id}/edit`}
                        title="Edit"
                        aria-label={`Edit ${item.name}`}
                        className={iconButtonClass()}
                      >
                        <EditIcon />
                      </Link>
                      <IconButton
                        label={runningId === item.id ? 'Running' : 'Run'}
                        tone="accent"
                        onClick={() => handleRun(item)}
                        disabled={runningId === item.id}
                      >
                        {runningId === item.id ? <SpinnerIcon /> : <PlayIcon />}
                      </IconButton>
                      <Link
                        to={`/sync-logs?sync_job_id=${item.id}`}
                        title="Logs"
                        aria-label={`Logs for ${item.name}`}
                        className={iconButtonClass()}
                      >
                        <LogsIcon />
                      </Link>
                      <IconButton
                        label="Delete"
                        tone="danger"
                        onClick={() => setPendingDelete(item)}
                      >
                        <DeleteIcon />
                      </IconButton>
                    </div>
                  </Td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableShell>
      )}

      <ConfirmDialog
        open={Boolean(pendingDelete)}
        title="Delete sync job"
        message={pendingDelete ? `Delete “${pendingDelete.name}”?` : ''}
        onCancel={() => setPendingDelete(null)}
        onConfirm={confirmDelete}
        busy={deleting}
      />
    </div>
  )
}
