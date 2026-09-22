import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { parsePage } from '../api/pagination'
import { deleteSyncJob, listSyncJobs, startSyncJob } from '../api/syncJobs'
import ConfirmDialog from '../components/ConfirmDialog'
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
  tableClassName,
} from '../components/ui'
import { formatDate } from '../lib/format'

const PAGE_SIZE = 20

function Icon({ children }) {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      {children}
    </svg>
  )
}

function OpenIcon() {
  return (
    <Icon>
      <path
        d="M2.5 12s3.5-6.5 9.5-6.5S21.5 12 21.5 12s-3.5 6.5-9.5 6.5S2.5 12 2.5 12Z"
        stroke="currentColor"
        strokeWidth="1.75"
      />
      <circle cx="12" cy="12" r="2.75" stroke="currentColor" strokeWidth="1.75" />
    </Icon>
  )
}

function EditIcon() {
  return (
    <Icon>
      <path
        d="M4 20h4L18.5 9.5a2.12 2.12 0 0 0-3-3L5 17v3Z"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinejoin="round"
      />
      <path d="m13.5 6.5 3 3" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" />
    </Icon>
  )
}

function PlayIcon() {
  return (
    <Icon>
      <path d="M8 5.5v13l11-6.5-11-6.5Z" fill="currentColor" />
    </Icon>
  )
}

function SpinnerIcon() {
  return (
    <svg
      className="animate-spin"
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="8" stroke="currentColor" strokeWidth="2" opacity="0.35" />
      <path d="M12 4a8 8 0 0 1 8 8" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </svg>
  )
}

function LogsIcon() {
  return (
    <Icon>
      <path d="M5 4h14v16H5V4Z" stroke="currentColor" strokeWidth="1.75" strokeLinejoin="round" />
      <path d="M8 8h8M8 12h8M8 16h5" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" />
    </Icon>
  )
}

function DeleteIcon() {
  return (
    <Icon>
      <path
        d="M4 7h16M9 7V5h6v2M6.5 7l.8 13h9.4l.8-13"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </Icon>
  )
}

export default function SyncJobsPage() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const page = parsePage(searchParams.get('page'))
  const [items, setItems] = useState([])
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [pendingDelete, setPendingDelete] = useState(null)
  const [deleting, setDeleting] = useState(false)
  const [runningId, setRunningId] = useState(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const data = await listSyncJobs({ page, pageSize: PAGE_SIZE })
      setItems(data?.items || [])
      setTotal(data?.total ?? 0)
      setTotalPages(data?.total_pages ?? 0)
      if (data?.total_pages > 0 && page > data.total_pages) {
        setSearchParams({ page: String(data.total_pages) }, { replace: true })
      }
    } catch (err) {
      setError(err.message || 'Failed to load sync jobs')
    } finally {
      setLoading(false)
    }
  }, [page, setSearchParams])

  useEffect(() => {
    load()
  }, [load])

  function setPage(next) {
    setSearchParams(next > 1 ? { page: String(next) } : {})
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

      {loading && items.length === 0 ? (
        <LoadingState />
      ) : !loading && items.length === 0 ? (
        <EmptyState
          title="No sync jobs yet"
          message="Create a job after you have at least one source and one destination connection."
          action={
            <Link to="/sync-jobs/new">
              <PrimaryButton>Create sync job</PrimaryButton>
            </Link>
          }
        />
      ) : (
        <TableShell
          footer={
            <Pagination
              page={page}
              totalPages={totalPages}
              total={total}
              pageSize={PAGE_SIZE}
              onPageChange={setPage}
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
