export const DESTINATION_TYPES = [
  { value: '', label: 'Default (pass-through)' },
  { value: 'string', label: 'string' },
  { value: 'int64', label: 'int64' },
  { value: 'float64', label: 'float64' },
  { value: 'bool', label: 'bool' },
  { value: 'object', label: 'object' },
  { value: 'object_array', label: 'object_array' },
]

export const RELATION_TYPES = [
  { value: 'belongs_to_many', label: 'belongs_to_many' },
  { value: 'has_many', label: 'has_many' },
]

export function formatDate(value) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return date.toLocaleString()
}

export function formatDuration(ms) {
  if (ms === null || ms === undefined) return '—'
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(2)} s`
}
