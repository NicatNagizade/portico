import { DESTINATION_TYPES } from '../../lib/destinationTypes'
import AutocompleteInput from '../AutocompleteInput'
import { IconButton, SecondaryButton } from '../ui'

const compactInput =
  'w-full rounded-md border border-[var(--border)] bg-[var(--surface)] px-2.5 py-1.5 text-sm text-[var(--text)] outline-none placeholder:text-[var(--text-muted)]/70 focus:border-[var(--accent)] focus:shadow-[0_0_0_2px_var(--accent-soft)]'

function emptyField() {
  return {
    id: undefined,
    source_name: '',
    destination_name: '',
    destination_type: '',
    active: true,
  }
}

function TrashIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M4 7h16M9 7V5h6v2M8 7l1 12h6l1-12"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}

export default function FieldEditor({
  fields,
  onChange,
  sourceColumns = [],
  onNeedSourceColumns,
  title = 'Field overrides',
  description = 'Rename columns, set destination types, or exclude fields. Leave empty to pass through all source columns.',
  sectionNumber = '03',
  compact = false,
}) {
  function updateRow(index, patch) {
    onChange(fields.map((row, i) => (i === index ? { ...row, ...patch } : row)))
  }

  function removeRow(index) {
    onChange(fields.filter((_, i) => i !== index))
  }

  return (
    <div className="space-y-3">
      <div className="flex items-start justify-between gap-3">
        {compact ? (
          <p className="pt-1 text-xs font-medium text-[var(--text-muted)]">{title}</p>
        ) : (
          <div className="flex items-start gap-2">
            <span className="mt-0.5 flex h-6 w-6 items-center justify-center rounded-md bg-[var(--accent)] font-mono text-[10px] font-bold text-white">
              {sectionNumber}
            </span>
            <div>
              <h3 className="text-sm font-semibold text-[var(--text)]">{title}</h3>
              <p className="text-xs text-[var(--text-muted)]">{description}</p>
            </div>
          </div>
        )}
        <SecondaryButton type="button" onClick={() => onChange([...fields, emptyField()])}>
          Add field
        </SecondaryButton>
      </div>

      {fields.length === 0 ? (
        <p className="rounded-lg border border-dashed border-[var(--border)] px-3 py-4 text-center text-sm text-[var(--text-muted)]">
          No overrides — columns pass through unchanged.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-[var(--border)]">
          <table className="w-full min-w-[640px] border-collapse text-left text-sm">
            <thead>
              <tr className="border-b border-[var(--border)] bg-[var(--bg-elevated)]/80">
                <th className="px-3 py-2 font-mono text-[10px] font-semibold tracking-[0.12em] text-[var(--text-muted)] uppercase">
                  Source
                </th>
                <th className="px-3 py-2 font-mono text-[10px] font-semibold tracking-[0.12em] text-[var(--text-muted)] uppercase">
                  Destination
                </th>
                <th className="w-36 px-3 py-2 font-mono text-[10px] font-semibold tracking-[0.12em] text-[var(--text-muted)] uppercase">
                  Type
                </th>
                <th className="w-16 px-3 py-2 text-center font-mono text-[10px] font-semibold tracking-[0.12em] text-[var(--text-muted)] uppercase">
                  On
                </th>
                <th className="w-10 px-2 py-2" />
              </tr>
            </thead>
            <tbody>
              {fields.map((row, index) => (
                <tr
                  key={row.id ?? `new-${index}`}
                  className="border-b border-[var(--border)] last:border-b-0"
                >
                  <td className="px-2 py-1.5">
                    <AutocompleteInput
                      className={compactInput}
                      options={sourceColumns}
                      value={row.source_name}
                      onFocus={onNeedSourceColumns}
                      onChange={(e) => updateRow(index, { source_name: e.target.value })}
                      required
                      placeholder="column"
                    />
                  </td>
                  <td className="px-2 py-1.5">
                    <input
                      className={compactInput}
                      value={row.destination_name || ''}
                      onChange={(e) => updateRow(index, { destination_name: e.target.value })}
                      placeholder="same as source"
                    />
                  </td>
                  <td className="px-2 py-1.5">
                    <select
                      className={compactInput}
                      value={row.destination_type || ''}
                      onChange={(e) => updateRow(index, { destination_type: e.target.value })}
                    >
                      {DESTINATION_TYPES.map((opt) => (
                        <option key={opt.value || 'default'} value={opt.value}>
                          {opt.label}
                        </option>
                      ))}
                    </select>
                  </td>
                  <td className="px-2 py-1.5 text-center align-middle">
                    <input
                      type="checkbox"
                      title="Active"
                      aria-label="Active"
                      className="h-4 w-4 accent-[var(--accent)]"
                      checked={row.active !== false}
                      onChange={(e) => updateRow(index, { active: e.target.checked })}
                    />
                  </td>
                  <td className="px-1 py-1.5 text-center align-middle">
                    <IconButton label="Remove field" tone="danger" onClick={() => removeRow(index)}>
                      <TrashIcon />
                    </IconButton>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
