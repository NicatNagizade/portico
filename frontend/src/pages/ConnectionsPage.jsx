import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { deleteConnection, listConnections } from '../api/connections'
import ConfirmDialog from '../components/ConfirmDialog'
import {
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
  TypeChip,
} from '../components/ui'
import { formatDate } from '../lib/format'

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
        description="Wire up MySQL or Postgres sources and Typesense destinations Portico will sync between."
        actions={
          <Link to="/connections/new">
            <PrimaryButton>
              <span aria-hidden="true">+</span> New connection
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
          message="Create a source (MySQL/Postgres) and a destination (Typesense) to start syncing tables."
          action={
            <Link to="/connections/new">
              <PrimaryButton>Create first connection</PrimaryButton>
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
                <Link to={`/connections/${item.id}/edit`}>
                  <SecondaryButton>Edit</SecondaryButton>
                </Link>
                <GhostButton onClick={() => setPendingDelete(item)}>Delete</GhostButton>
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
