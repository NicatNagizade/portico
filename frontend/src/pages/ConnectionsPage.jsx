import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { deleteConnection, listConnections } from '../api/connections'
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
  TypeChip,
} from '../components/ui'
import { formatDate } from '../lib/format'

function Icon({ children }) {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      {children}
    </svg>
  )
}

function CreateIcon() {
  return (
    <Icon>
      <path d="M12 5v14M5 12h14" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" />
    </Icon>
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

export default function ConnectionsPage() {
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [pendingDelete, setPendingDelete] = useState(null)
  const [deleting, setDeleting] = useState(false)

  async function load() {
    setLoading(true)
    setError('')
    try {
      const data = await listConnections()
      setItems(data || [])
    } catch (err) {
      setError(err.message || 'Failed to load connections')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function confirmDelete() {
    if (!pendingDelete) return
    setDeleting(true)
    setError('')
    try {
      await deleteConnection(pendingDelete.id)
      setPendingDelete(null)
      await load()
    } catch (err) {
      setError(err.message || 'Failed to delete connection')
    } finally {
      setDeleting(false)
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="Data plane"
        title="Connections"
        description="Connect MySQL, Postgres, SQLite, Typesense, MongoDB, or Redis. Every connection can be used as a source or a destination."
        actions={
          <Link to="/connections/new">
            <PrimaryButton>
              <CreateIcon /> New connection
            </PrimaryButton>
          </Link>
        }
      />

      <ErrorBanner message={error} />

      {loading ? (
        <LoadingState />
      ) : items.length === 0 ? (
        <EmptyState
          title="No connections yet"
          message="Create a connection, then use it as a source or destination in a sync job."
          action={
            <Link to="/connections/new">
              <PrimaryButton>
                <CreateIcon /> Create first connection
              </PrimaryButton>
            </Link>
          }
        />
      ) : (
        <ListStack>
          {items.map((item, index) => (
            <ListRow
              key={item.id}
              className={`stagger-${Math.min(index + 1, 5)}`}
            >
              <div className="flex min-w-0 items-start gap-3">
                <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-[var(--bg-elevated)] font-mono text-xs font-semibold text-[var(--text-muted)]">
                  #{item.id}
                </div>
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="truncate text-base font-semibold text-[var(--text)]">
                      {item.name}
                    </h3>
                    <TypeChip type={item.type} />
                  </div>
                  <p className="mt-1 text-sm text-[var(--text-muted)]">
                    Created {formatDate(item.created_at)}
                  </p>
                </div>
              </div>
              <div className="flex flex-wrap items-center gap-2 sm:justify-end">
                <MetaChip>updated {formatDate(item.updated_at)}</MetaChip>
                <div className="inline-flex items-center gap-px rounded-lg border border-[var(--border)] bg-[var(--bg-elevated)] p-0.5">
                  <Link
                    to={`/connections/${item.id}`}
                    title="View"
                    aria-label={`View ${item.name}`}
                    className={iconButtonClass()}
                  >
                    <OpenIcon />
                  </Link>
                  <Link
                    to={`/connections/${item.id}/edit`}
                    title="Edit"
                    aria-label={`Edit ${item.name}`}
                    className={iconButtonClass()}
                  >
                    <EditIcon />
                  </Link>
                  <IconButton
                    label="Delete"
                    tone="danger"
                    onClick={() => setPendingDelete(item)}
                  >
                    <DeleteIcon />
                  </IconButton>
                </div>
              </div>
            </ListRow>
          ))}
        </ListStack>
      )}

      <ConfirmDialog
        open={Boolean(pendingDelete)}
        title="Delete connection"
        message={
          pendingDelete
            ? `Delete “${pendingDelete.name}”? Sync jobs that reference it may fail.`
            : ''
        }
        onCancel={() => setPendingDelete(null)}
        onConfirm={confirmDelete}
        busy={deleting}
      />
    </div>
  )
}
