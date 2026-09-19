import { DESTINATION_TYPES } from '../../lib/destinationTypes'
import { GhostButton, SecondaryButton, inputClassName } from '../ui'

function emptyField() {
  return {
    source_name: '',
    destination_name: '',
    destination_type: '',
    active: true,
  }
}

export default function FieldEditor({ fields, onChange }) {
  function updateRow(index, patch) {
    onChange(fields.map((row, i) => (i === index ? { ...row, ...patch } : row)))
  }

  function removeRow(index) {
    onChange(fields.filter((_, i) => i !== index))
  }

  return (
    <div className="space-y-4">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-2">
          <span className="mt-0.5 flex h-6 w-6 items-center justify-center rounded-md bg-[var(--accent)] font-mono text-[10px] font-bold text-white">
            03
          </span>
          <div>
            <h3 className="text-sm font-semibold text-[var(--text)]">Field overrides</h3>
            <p className="text-xs text-[var(--text-muted)]">
              Rename columns, set destination types, or exclude fields.
            </p>
          </div>
        </div>
        <SecondaryButton type="button" onClick={() => onChange([...fields, emptyField()])}>
          Add field
        </SecondaryButton>
      </div>

      {fields.length === 0 ? (
        <p className="rounded-xl border border-dashed border-[var(--border)] px-4 py-6 text-center text-sm text-[var(--text-muted)]">
          No overrides — source columns pass through unchanged.
        </p>
      ) : (
        <div className="space-y-3">
          {fields.map((row, index) => (
            <div
              key={index}
              className="grid gap-3 rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4 shadow-[var(--shadow-sm)] sm:grid-cols-2 lg:grid-cols-5"
            >
              <label className="block text-xs">
                <span className="mb-1.5 block text-[var(--text-muted)]">Source name</span>
                <input
                  className={inputClassName}
                  value={row.source_name}
                  onChange={(e) => updateRow(index, { source_name: e.target.value })}
                  required
                />
              </label>
              <label className="block text-xs">
                <span className="mb-1.5 block text-[var(--text-muted)]">Destination name</span>
                <input
                  className={inputClassName}
                  value={row.destination_name || ''}
                  onChange={(e) => updateRow(index, { destination_name: e.target.value })}
                />
              </label>
              <label className="block text-xs">
                <span className="mb-1.5 block text-[var(--text-muted)]">Destination type</span>
                <select
                  className={inputClassName}
                  value={row.destination_type || ''}
                  onChange={(e) => updateRow(index, { destination_type: e.target.value })}
                >
                  {DESTINATION_TYPES.map((opt) => (
                    <option key={opt.value || 'default'} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </label>
              <label className="flex items-end gap-2 pb-2.5 text-sm">
                <input
                  type="checkbox"
                  className="h-4 w-4 accent-[var(--accent)]"
                  checked={row.active !== false}
                  onChange={(e) => updateRow(index, { active: e.target.checked })}
                />
                Active
              </label>
              <div className="flex items-end">
                <GhostButton type="button" onClick={() => removeRow(index)}>
                  Remove
                </GhostButton>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
