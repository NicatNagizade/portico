import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  createConnection,
  getConnection,
  updateConnection,
} from '../api/connections'
import ConnectionForm from '../components/forms/ConnectionForm'
import { ErrorBanner, LoadingState, PageHeader, Panel, SecondaryButton } from '../components/ui'

export default function ConnectionFormPage() {
  const { id } = useParams()
  const isEdit = Boolean(id)
  const navigate = useNavigate()
  const [initial, setInitial] = useState(null)
  const [loading, setLoading] = useState(isEdit)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!isEdit) return
    let active = true
    setLoading(true)
    getConnection(id)
      .then((data) => {
        if (active) setInitial(data)
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
  }, [id, isEdit])

  async function handleSubmit(payload) {
    setBusy(true)
    setError('')
    try {
      if (isEdit) {
        await updateConnection(id, payload)
      } else {
        await createConnection(payload)
      }
      navigate('/connections')
    } catch (err) {
      setError(err.message || 'Failed to save connection')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="Connections"
        title={isEdit ? 'Edit connection' : 'New connection'}
        description={
          isEdit
            ? 'Update connection credentials and connector settings.'
            : 'Add a source or destination Portico can talk to.'
        }
        actions={
          <Link to="/connections">
            <SecondaryButton>Back to list</SecondaryButton>
          </Link>
        }
      />

      <ErrorBanner message={error} />

      {loading ? (
        <LoadingState rows={3} />
      ) : (
        <Panel className="animate-fade-up">
          <ConnectionForm
            key={initial?.id || 'new'}
            initial={initial}
            onSubmit={handleSubmit}
            busy={busy}
            submitLabel={isEdit ? 'Save changes' : 'Create connection'}
          />
        </Panel>
      )}
    </div>
  )
}
