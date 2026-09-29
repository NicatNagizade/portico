import { useState } from 'react'
import { suggestExplore } from '../../api/exploreAI'
import { Dialog, PrimaryButton, SecondaryButton, inputClassName } from '../ui'

/** Dialog: describe filters/fields in plain language; applies AI result on success. */
export default function ExploreAISuggest({
  open,
  onClose,
  jobId,
  filterFields,
  fields,
  onApply,
}) {
  const [prompt, setPrompt] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function submit() {
    const text = prompt.trim()
    if (!text || !jobId) return
    setLoading(true)
    setError('')
    try {
      const data = await suggestExplore(jobId, {
        prompt: text,
        filterFields,
        fields,
      })
      onApply({
        filters: Array.isArray(data?.filters) ? data.filters : [],
        fields: data?.fields === undefined ? null : data.fields,
      })
      setPrompt('')
      onClose()
    } catch (err) {
      setError(err.message || 'AI suggest failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog
      open={open}
      title="Ask AI"
      description="Describe the filters and columns you want. Existing filters and field picks will be replaced."
      onClose={() => {
        if (!loading) onClose()
      }}
      size="md"
      footer={
        <>
          <SecondaryButton type="button" disabled={loading} onClick={onClose}>
            Cancel
          </SecondaryButton>
          <PrimaryButton
            type="button"
            disabled={loading || !prompt.trim() || !jobId}
            onClick={submit}
          >
            {loading ? 'Thinking…' : 'Apply'}
          </PrimaryButton>
        </>
      }
    >
      <div className="space-y-3">
        <textarea
          className={`${inputClassName} min-h-[120px] resize-y`}
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          placeholder='e.g. status is active and name contains ann; show id, name, email'
          disabled={loading}
          aria-label="AI prompt for filters and fields"
        />
        {error ? (
          <p className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-800">
            {error}
          </p>
        ) : (
          <p className="text-xs text-[var(--text-muted)]">
            Uses your configured OpenAI-compatible model. Field names come from this job’s schema.
          </p>
        )}
      </div>
    </Dialog>
  )
}
