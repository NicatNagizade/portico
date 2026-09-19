import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { deleteSyncJob, listSyncJobs, runSyncJob } from '../api/syncJobs'
import ConfirmDialog from '../components/ConfirmDialog'
import {
  DangerButton,
  EmptyState,
  ErrorBanner,
  GhostButton,
  ListRow,
  ListStack,
  LoadingState,
  MetaChip,
  PageHeader,
  PrimaryButton,
  SecondaryButton,
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
              <div className="flex flex-wrap gap-2 sm:justify-end">
                <Link to={`/sync-jobs/${item.id}`}>
                  <SecondaryButton>Open</SecondaryButton>
                </Link>
                <Link to={`/sync-jobs/${item.id}/edit`}>
                  <SecondaryButton>Edit</SecondaryButton>
                </Link>
                <PrimaryButton
                  onClick={() => handleRun(item)}
                  disabled={runningId === item.id}
                >
                  {runningId === item.id ? 'Running…' : 'Run'}
                </PrimaryButton>
                <Link to={`/sync-logs?sync_job_id=${item.id}`}>
                  <GhostButton>Logs</GhostButton>
                </Link>
                <DangerButton onClick={() => setPendingDelete(item)}>Delete</DangerButton>
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
