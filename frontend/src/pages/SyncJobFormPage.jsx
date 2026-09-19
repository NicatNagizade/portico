import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { listConnections } from '../api/connections'
import { createSyncJob, getSyncJob, updateSyncJob } from '../api/syncJobs'
import SyncJobForm from '../components/forms/SyncJobForm'
import { ErrorBanner, LoadingState, PageHeader, Panel, SecondaryButton } from '../components/ui'

export default function SyncJobFormPage() {
  const { id } = useParams()
  const isEdit = Boolean(id)
  const navigate = useNavigate()
  const [initial, setInitial] = useState(null)
  const [connections, setConnections] = useState([])
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true
    setLoading(true)
    Promise.all([listConnections(), isEdit ? getSyncJob(id) : Promise.resolve(null)])
      .then(([conns, job]) => {
        if (!active) return
        setConnections(conns || [])
        setInitial(job)
      })
      .catch((err) => {
        if (active) setError(err.message || 'Failed to load form data')
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [id, isEdit])

  async function handleSubmit(payload) {
    setBusy(true)
    setError('')
    try {
      if (isEdit) {
        await updateSyncJob(id, payload)
        navigate(`/sync-jobs/${id}`)
      } else {
        const created = await createSyncJob(payload)
        navigate(`/sync-jobs/${created.id}`)
      }
    } catch (err) {
      setError(err.message || 'Failed to save sync job')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="Sync jobs"
        title={isEdit ? 'Edit sync job' : 'New sync job'}
        description="Configure tables, field overrides, relations, and destination options."
        actions={
          <Link to={isEdit ? `/sync-jobs/${id}` : '/sync-jobs'}>
            <SecondaryButton>Back</SecondaryButton>
          </Link>
        }
      />

      <ErrorBanner message={error} />

      {loading ? (
        <LoadingState rows={5} />
      ) : (
        <Panel className="animate-fade-up">
          <SyncJobForm
            key={initial?.id || 'new'}
            initial={initial}
            connections={connections}
            onSubmit={handleSubmit}
            busy={busy}
            submitLabel={isEdit ? 'Save changes' : 'Create sync job'}
          />
        </Panel>
      )}
    </div>
  )
}
