import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { deleteConnection, getConnection } from '../api/connections'
import ConfirmDialog from '../components/ConfirmDialog'
import { BackIcon, DeleteIcon, EditIcon, Icon } from '../components/icons'
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
import { getConfigFields, parseConfig } from '../lib/connectionTypes'
import { formatDate } from '../lib/format'

function JobsIcon() {
  return (
    <Icon>
      <path
        d="M5 7h14v4H5V7Zm0 6h14v4H5v-4Z"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinejoin="round"
      />
    </Icon>
  )
}

export default function ConnectionDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [item, setItem] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [pendingDelete, setPendingDelete] = useState(false)
  const [deleting, setDeleting] = useState(false)

  useEffect(() => {
    let active = true
    setLoading(true)
    setError('')
    getConnection(id)
      .then((data) => {
        if (active) setItem(data)
      })
      .catch((err) => {
        if (active) setError(err.message || 'Failed to load connection')
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [id])

  async function confirmDelete() {
    setDeleting(true)
    setError('')
    try {
      await deleteConnection(id)
      navigate('/connections')
    } catch (err) {
      setError(err.message || 'Failed to delete connection')
      setPendingDelete(false)
    } finally {
      setDeleting(false)
    }
  }

  if (loading) {
    return (
      <div>
        <PageHeader eyebrow="Connections" title="Connection" />
        <LoadingState rows={3} />
      </div>
    )
  }

  if (!item) {
    return (
      <div>
        <PageHeader eyebrow="Connections" title="Connection" />
        <ErrorBanner message={error || 'Connection not found'} />
      </div>
    )
  }

  const config = parseConfig(item.config)
  const fields = getConfigFields(item.type)
  const entries = fields
    ? fields.map((f) => [f.label, config[f.key]])
    : Object.entries(config)

  return (
    <div>
      <PageHeader
        eyebrow={`Connection #${item.id}`}
        title={item.name}
        description="Inspect this connector and jump to sync jobs that use it as a source or destination."
        actions={
          <div className="inline-flex items-center gap-px rounded-lg border border-[var(--border)] bg-[var(--bg-elevated)] p-0.5 shadow-[var(--shadow-sm)]">
            <Link
              to="/connections"
              title="Back"
              aria-label="Back to connections"
              className={iconButtonClass()}
            >
              <BackIcon />
            </Link>
            <Link
              to={`/connections/${id}/edit`}
              title="Edit"
              aria-label={`Edit ${item.name}`}
              className={iconButtonClass()}
            >
              <EditIcon />
            </Link>
            <Link
              to={`/sync-jobs?connection_id=${id}`}
              title="Sync jobs"
              aria-label={`Sync jobs for ${item.name}`}
              className={iconButtonClass()}
            >
              <JobsIcon />
            </Link>
            <IconButton label="Delete" tone="danger" onClick={() => setPendingDelete(true)}>
              <DeleteIcon />
            </IconButton>
          </div>
        }
      />

      <ErrorBanner message={error} />

      <Panel className="animate-fade-up">
        <div className="flex flex-wrap items-center gap-2">
          <TypeChip type={item.type} />
          <MetaChip>created {formatDate(item.created_at)}</MetaChip>
          <MetaChip>updated {formatDate(item.updated_at)}</MetaChip>
        </div>

        <dl className="mt-5 grid gap-3 sm:grid-cols-2">
          {entries.map(([label, value]) => (
            <div key={label} className="rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)]/60 px-4 py-3">
              <dt className="text-xs font-medium text-[var(--text-muted)]">{label}</dt>
              <dd className="mt-1 break-all font-mono text-sm text-[var(--text)]">
                {value === '' || value == null ? '—' : String(value)}
              </dd>
            </div>
          ))}
        </dl>
      </Panel>

      <ConfirmDialog
        open={pendingDelete}
        title="Delete connection"
        message={`Delete “${item.name}”? Sync jobs that reference it may fail.`}
        onCancel={() => setPendingDelete(false)}
        onConfirm={confirmDelete}
        busy={deleting}
      />
    </div>
  )
}
