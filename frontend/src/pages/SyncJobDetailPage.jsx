import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { deleteSyncJob, getSyncJob, startSyncJob } from '../api/syncJobs'
import ConfirmDialog from '../components/ConfirmDialog'
import {
  ErrorBanner,
  IconButton,
  iconButtonClass,
  LoadingState,
  MetaChip,
  PageHeader,
  Panel,
  TypeChip,
} from '../components/ui'
import { formatDate } from '../lib/format'
import { ruleNeedsValue, ruleOperatorLabel } from '../lib/ruleOperators'
import { parseConfig } from '../lib/connectionTypes'

function Icon({ children }) {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      {children}
    </svg>
  )
}

function BackIcon() {
  return (
    <Icon>
      <path
        d="M15 6 9 12l6 6"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
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

export default function SyncJobDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [job, setJob] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [running, setRunning] = useState(false)
  const [pendingDelete, setPendingDelete] = useState(false)
  const [deleting, setDeleting] = useState(false)

  async function load() {
    setLoading(true)
    setError('')
    try {
      const data = await getSyncJob(id)
      setJob(data)
    } catch (err) {
      setError(err.message || 'Failed to load sync job')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [id])

  async function handleRun() {
    setRunning(true)
    setError('')
    try {
      const log = await startSyncJob(id)
      navigate(`/sync-logs/${log.id}`)
    } catch (err) {
      setError(err.message || 'Failed to start sync job')
      setRunning(false)
    }
  }

  async function confirmDelete() {
    setDeleting(true)
    setError('')
    try {
      await deleteSyncJob(id)
      navigate('/sync-jobs')
    } catch (err) {
      setError(err.message || 'Failed to delete sync job')
      setDeleting(false)
    }
  }

  if (loading) {
    return (
      <div>
        <PageHeader eyebrow="Sync jobs" title="Loading…" />
        <LoadingState rows={4} />
      </div>
    )
  }

  if (!job) {
    return (
      <div>
        <PageHeader eyebrow="Sync jobs" title="Sync job" />
        <ErrorBanner message={error || 'Sync job not found'} />
      </div>
    )
  }

  const config = parseConfig(job.config)

  return (
    <div>
      <PageHeader
        eyebrow={`Job #${job.id}`}
        title={job.name}
        description="Inspect the pipeline, override fields and relations, then run a full destination reload."
        actions={
          <div className="inline-flex items-center gap-px rounded-lg border border-[var(--border)] bg-[var(--bg-elevated)] p-0.5 shadow-[var(--shadow-sm)]">
            <Link to="/sync-jobs" title="Back" aria-label="Back to sync jobs" className={iconButtonClass()}>
              <BackIcon />
            </Link>
            <Link
              to={`/sync-jobs/${id}/edit`}
              title="Edit"
              aria-label={`Edit ${job.name}`}
              className={iconButtonClass()}
            >
              <EditIcon />
            </Link>
            <Link
              to={`/sync-logs?sync_job_id=${id}`}
              title="View logs"
              aria-label={`Logs for ${job.name}`}
              className={iconButtonClass()}
            >
              <LogsIcon />
            </Link>
            <IconButton
              label={running ? 'Starting' : 'Run sync'}
              tone="accent"
              onClick={handleRun}
              disabled={running}
            >
              {running ? <SpinnerIcon /> : <PlayIcon />}
            </IconButton>
            <IconButton label="Delete" tone="danger" onClick={() => setPendingDelete(true)} disabled={running}>
              <DeleteIcon />
            </IconButton>
          </div>
        }
      />

      <ErrorBanner message={error} />

      <div className="animate-fade-up mb-4 overflow-hidden rounded-[var(--radius)] border border-[var(--border)] bg-[var(--surface)] shadow-[var(--shadow-md)]">
        <div className="grid gap-0 md:grid-cols-[1fr_auto_1fr]">
          <div className="bg-gradient-to-br from-[var(--mysql-soft)] to-[var(--surface)] p-5">
            <p className="font-mono text-[10px] tracking-[0.18em] text-[var(--text-muted)] uppercase">
              Source
            </p>
            <p className="mt-2 text-lg font-semibold">
              {job.source_connection?.name || `#${job.source_connection_id}`}
            </p>
            <div className="mt-2 flex flex-wrap items-center gap-2">
              {job.source_connection?.type ? <TypeChip type={job.source_connection.type} /> : null}
              <MetaChip>{job.source_table}</MetaChip>
            </div>
          </div>
          <div className="flex items-center justify-center bg-[var(--bg-elevated)] px-4 py-3 md:py-0">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-[var(--accent)] text-white shadow-[var(--shadow-sm)]">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path
                  d="M5 12h12M13 6l6 6-6 6"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </div>
          </div>
          <div className="bg-gradient-to-bl from-[var(--typesense-soft)] to-[var(--surface)] p-5">
            <p className="font-mono text-[10px] tracking-[0.18em] text-[var(--text-muted)] uppercase">
              Destination
            </p>
            <p className="mt-2 text-lg font-semibold">
              {job.destination_connection?.name || `#${job.destination_connection_id}`}
            </p>
            <div className="mt-2 flex flex-wrap items-center gap-2">
              {job.destination_connection?.type ? (
                <TypeChip type={job.destination_connection.type} />
              ) : null}
              <MetaChip>{job.destination_table}</MetaChip>
            </div>
          </div>
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Panel title="Runtime" className="animate-fade-up stagger-1">
          <dl className="space-y-3 text-sm">
            <div className="flex justify-between gap-4 border-b border-[var(--border)] pb-3">
              <dt className="text-[var(--text-muted)]">Chunk size</dt>
              <dd className="font-mono font-medium">{job.chunk_size}</dd>
            </div>
            <div className="flex justify-between gap-4 border-b border-[var(--border)] pb-3">
              <dt className="text-[var(--text-muted)]">Workers</dt>
              <dd className="font-mono font-medium">{job.workers}</dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-[var(--text-muted)]">Updated</dt>
              <dd>{formatDate(job.updated_at)}</dd>
            </div>
          </dl>
        </Panel>

        <Panel
          title="Destination config"
          description="Opaque JSON interpreted by the destination connector."
          className="animate-fade-up stagger-2"
        >
          {Object.keys(config).length === 0 ? (
            <p className="text-sm text-[var(--text-muted)]">Using connector defaults.</p>
          ) : (
            <pre className="overflow-x-auto rounded-xl bg-[var(--bg-elevated)] p-4 font-mono text-xs leading-relaxed text-[var(--text)]">
              {JSON.stringify(config, null, 2)}
            </pre>
          )}
        </Panel>

        <Panel
          title="Filter rules"
          description={`${(job.rules || []).length} rule(s) — AND'd before import`}
          className="animate-fade-up stagger-3 lg:col-span-2"
        >
          {(job.rules || []).length === 0 ? (
            <p className="text-sm text-[var(--text-muted)]">No filters — all source rows are imported.</p>
          ) : (
            <ul className="divide-y divide-[var(--border)]">
              {job.rules.map((rule) => (
                <li key={rule.id} className="flex flex-wrap items-center justify-between gap-2 py-3">
                  <div className="flex flex-wrap items-center gap-2 font-mono text-xs">
                    <span className="rounded-md bg-[var(--bg-elevated)] px-2 py-1">{rule.field}</span>
                    <span className="text-[var(--text-muted)]">{ruleOperatorLabel(rule.operator)}</span>
                    {ruleNeedsValue(rule.operator) ? (
                      <span className="rounded-md bg-[var(--accent-soft)] px-2 py-1 text-[var(--accent-ink)]">
                        {rule.value}
                      </span>
                    ) : null}
                  </div>
                  <MetaChip>{rule.active === false ? 'inactive' : 'active'}</MetaChip>
                </li>
              ))}
            </ul>
          )}
        </Panel>

        <Panel
          title="Field overrides"
          description={`${(job.fields || []).length} root field(s)`}
          className="animate-fade-up stagger-4 lg:col-span-2"
        >
          {(job.fields || []).length === 0 ? (
            <p className="text-sm text-[var(--text-muted)]">No field overrides — columns pass through.</p>
          ) : (
            <ul className="divide-y divide-[var(--border)]">
              {job.fields.map((field) => (
                <li key={field.id} className="flex flex-wrap items-center justify-between gap-2 py-3">
                  <div className="flex flex-wrap items-center gap-2 font-mono text-xs">
                    <span className="rounded-md bg-[var(--bg-elevated)] px-2 py-1">{field.source_name}</span>
                    {field.destination_name ? (
                      <>
                        <span className="text-[var(--text-muted)]">→</span>
                        <span className="rounded-md bg-[var(--accent-soft)] px-2 py-1 text-[var(--accent-ink)]">
                          {field.destination_name}
                        </span>
                      </>
                    ) : null}
                    {field.destination_type ? <MetaChip>{field.destination_type}</MetaChip> : null}
                  </div>
                  <MetaChip>{field.active === false ? 'inactive' : 'active'}</MetaChip>
                </li>
              ))}
            </ul>
          )}
        </Panel>

        <Panel
          title="Relations"
          description={`${(job.relations || []).length} configured`}
          className="animate-fade-up stagger-5 lg:col-span-2"
        >
          {(job.relations || []).length === 0 ? (
            <p className="text-sm text-[var(--text-muted)]">No relations configured.</p>
          ) : (
            <ul className="space-y-3">
              {job.relations.map((relation) => {
                const relConfig = parseConfig(relation.config)
                const parent = (job.relations || []).find((r) => r.id === relation.parent_id)
                return (
                  <li
                    key={relation.id}
                    className="rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)]/60 p-4"
                  >
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <span className="font-semibold">{relation.name}</span>
                      <MetaChip>{relation.active === false ? 'inactive' : 'active'}</MetaChip>
                    </div>
                    <p className="mt-2 font-mono text-xs leading-relaxed text-[var(--text-muted)]">
                      {relation.type} · table={relation.table}
                      {relConfig.pivot_table ? ` · pivot=${relConfig.pivot_table}` : ''}
                      {relation.foreign_key ? ` · fk=${relation.foreign_key}` : ''}
                      {relation.related_key ? ` · rk=${relation.related_key}` : ''}
                      {parent ? ` · parent=${parent.name}` : ''}
                    </p>
                    {(relation.fields || []).length > 0 ? (
                      <ul className="mt-3 space-y-1 border-t border-[var(--border)] pt-3">
                        {relation.fields.map((field) => (
                          <li key={field.id} className="font-mono text-xs text-[var(--text-muted)]">
                            {field.source_name}
                            {field.destination_name ? ` → ${field.destination_name}` : ''}
                            {field.destination_type ? ` (${field.destination_type})` : ''}
                          </li>
                        ))}
                      </ul>
                    ) : null}
                  </li>
                )
              })}
            </ul>
          )}
        </Panel>
      </div>

      <ConfirmDialog
        open={pendingDelete}
        title="Delete sync job"
        message={`Delete “${job.name}”?`}
        onCancel={() => setPendingDelete(false)}
        onConfirm={confirmDelete}
        busy={deleting}
      />
    </div>
  )
}
