import { RULE_OPERATORS, ruleNeedsValue } from '../../lib/ruleOperators'
import AutocompleteInput from '../AutocompleteInput'
import { IconButton, SecondaryButton } from '../ui'

const compactInput =
  'w-full rounded-md border border-[var(--border)] bg-[var(--surface)] px-2.5 py-1.5 text-sm text-[var(--text)] outline-none placeholder:text-[var(--text-muted)]/70 focus:border-[var(--accent)] focus:shadow-[0_0_0_2px_var(--accent-soft)]'

function emptyExploreFilter() {
  return { field: '', operator: 'eq', value: '' }
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

/** Explore-time filter rows. Field input uses AutocompleteInput. */
export default function ExploreFilters({
  filters,
  onChange,
  fieldOptions = [],
  onNeedFields,
}) {
  function updateRow(index, patch) {
    onChange(filters.map((row, i) => (i === index ? { ...row, ...patch } : row)))
  }

  function removeRow(index) {
    onChange(filters.filter((_, i) => i !== index))
  }

  return (
    <div className="space-y-3">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h3 className="text-sm font-semibold text-[var(--text)]">Filters</h3>
          <p className="text-xs text-[var(--text-muted)]">
            Narrow the preview. All filters are AND’d
            {fieldOptions.length ? ' · field names autocomplete from columns' : ''}.
          </p>
        </div>
        <SecondaryButton type="button" onClick={() => onChange([...filters, emptyExploreFilter()])}>
          Add filter
        </SecondaryButton>
      </div>

      {filters.length === 0 ? (
        <p className="rounded-lg border border-dashed border-[var(--border)] px-3 py-3 text-center text-sm text-[var(--text-muted)]">
          No extra filters — job rules still apply on source.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-[var(--border)]">
          <table className="w-full min-w-[480px] border-collapse text-left text-sm">
            <thead>
              <tr className="border-b border-[var(--border)] bg-[var(--bg-elevated)]/80">
                <th className="px-3 py-2 font-mono text-[10px] font-semibold tracking-[0.12em] text-[var(--text-muted)] uppercase">
                  Field
                </th>
                <th className="px-3 py-2 font-mono text-[10px] font-semibold tracking-[0.12em] text-[var(--text-muted)] uppercase">
                  Operator
                </th>
                <th className="px-3 py-2 font-mono text-[10px] font-semibold tracking-[0.12em] text-[var(--text-muted)] uppercase">
                  Value
                </th>
                <th className="w-10 px-2 py-2" />
              </tr>
            </thead>
            <tbody>
              {filters.map((row, index) => {
                const needsValue = ruleNeedsValue(row.operator)
                return (
                  <tr key={index} className="border-b border-[var(--border)] last:border-b-0">
                    <td className="px-3 py-2">
                      <AutocompleteInput
                        required
                        className={compactInput}
                        options={fieldOptions}
                        value={row.field}
                        onFocus={onNeedFields}
                        onChange={(e) => updateRow(index, { field: e.target.value })}
                        placeholder="status"
                      />
                    </td>
                    <td className="px-3 py-2">
                      <select
                        className={compactInput}
                        value={row.operator}
                        onChange={(e) => {
                          const operator = e.target.value
                          updateRow(index, {
                            operator,
                            value: ruleNeedsValue(operator) ? row.value : '',
                          })
                        }}
                      >
                        {RULE_OPERATORS.map((opt) => (
                          <option key={opt.value} value={opt.value}>
                            {opt.label}
                          </option>
                        ))}
                      </select>
                    </td>
                    <td className="px-3 py-2">
                      {needsValue ? (
                        <input
                          required
                          className={compactInput}
                          value={row.value}
                          onChange={(e) => updateRow(index, { value: e.target.value })}
                          placeholder={
                            row.operator === 'in' || row.operator === 'not_in' ? 'a, b, c' : '123'
                          }
                        />
                      ) : (
                        <span className="font-mono text-xs text-[var(--text-muted)]">—</span>
                      )}
                    </td>
                    <td className="px-2 py-2">
                      <IconButton
                        type="button"
                        label="Remove filter"
                        tone="danger"
                        onClick={() => removeRow(index)}
                      >
                        <TrashIcon />
                      </IconButton>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
