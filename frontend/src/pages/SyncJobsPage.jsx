import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { deleteSyncJob, listSyncJobs, runSyncJob } from '../api/syncJobs'
import ConfirmDialog from '../components/ConfirmDialog'
import {
  EmptyState,
  ErrorBanner,
  IconButton,
  iconButtonClass,
  ListRow,
  ListStack,
  LoadingState,
  MetaChip,
  PageHeader,
  PrimaryButton,
  SuccessBanner,
} from '../components/ui'
import { formatDate } from '../lib/destinationTypes'

function Arrow() {
  return (
    <span className="mx-1 inline-flex text-[var(--text-muted)]" aria-hidden="true">
      →
    </span>
  )
}

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
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [pendingDelete, setPendingDelete] = useState(null)
  const [deleting, setDeleting] = useState(false)
  const [runningId, setRunningId] = useState(null)
  const [runMessage, setRunMessage] = useState('')

  async function load() {
    setLoading(true)
    setError('')
    try {
      const data = await listSyncJobs()
      setItems(data || [])
    } catch (err) {
      setError(err.message || 'Failed to load sync jobs')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function handleRun(job) {
    setRunningId(job.id)
    setRunMessage('')
    setError('')
    try {
      const log = await runSyncJob(job.id)
      setRunMessage(`Sync finished with status “${log.status}” (log #${log.id}).`)
    } catch (err) {
      setError(err.message || 'Failed to run sync job')
    } finally {
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
      <SuccessBanner message={runMessage} />

      {loading ? (
        <LoadingState />
      ) : items.length === 0 ? (
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
        <ListStack>
          {items.map((item, index) => (
            <ListRow key={item.id} className={`stagger-${Math.min(index + 1, 5)}`}>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <Link
                    to={`/sync-jobs/${item.id}`}
                    className="text-base font-semibold text-[var(--text)] transition-colors hover:text-[var(--accent)]"
                  >
                    {item.name}
                  </Link>
                  <MetaChip>#{item.id}</MetaChip>
                </div>
                <div className="mt-2 flex flex-wrap items-center gap-x-1 gap-y-1 text-sm text-[var(--text-muted)]">
                  <span className="font-medium text-[var(--text)]">
                    {item.source_connection?.name || `#${item.source_connection_id}`}
                  </span>
                  <span className="font-mono text-xs">/{item.source_table}</span>
                  <Arrow />
                  <span className="font-medium text-[var(--text)]">
                    {item.destination_connection?.name || `#${item.destination_connection_id}`}
                  </span>
                  <span className="font-mono text-xs">/{item.destination_table}</span>
                </div>
                <p className="mt-2 text-xs text-[var(--text-muted)]">
                  Updated {formatDate(item.updated_at)}
                </p>
              </div>
              <div className="flex shrink-0 items-center gap-px self-start rounded-lg border border-[var(--border)] bg-[var(--bg-elevated)] p-0.5 sm:self-center">
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
            </ListRow>
          ))}
        </ListStack>
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
