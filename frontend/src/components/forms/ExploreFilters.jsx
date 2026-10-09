import { RULE_OPERATORS, ruleNeedsValue } from '../../lib/ruleOperators'
import AutocompleteInput from '../AutocompleteInput'
import { TrashIcon } from '../icons'
import { IconButton, SecondaryButton, compactInputClassName } from '../ui'

function emptyExploreFilter() {
  return { field: '', operator: 'eq', value: '' }
}

function PlusIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M12 5v14M5 12h14" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" />
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

  function addRow() {
    onChange([...filters, emptyExploreFilter()])
  }

  if (filters.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-[var(--border)] px-4 py-8 text-center">
        <p className="text-sm text-[var(--text-muted)]">
          No extra filters yet. Job rules still apply on source.
        </p>
        <div className="mt-4 flex justify-center">
          <SecondaryButton type="button" onClick={addRow}>
            <PlusIcon />
            Add filter
          </SecondaryButton>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-3">
      <div className="overflow-x-auto rounded-lg border border-[var(--border)]">
        <table className="w-full min-w-[480px] border-collapse text-left text-sm">
          <thead>
            <tr className="border-b border-[var(--border)] bg-[var(--bg-elevated)]/80">
              <th className="px-3 py-2 font-mono text-[10px] font-semibold tracking-[0.12em] text-[var(--text-muted)] uppercase">
                Field
              </th>
              <th className="w-28 px-3 py-2 font-mono text-[10px] font-semibold tracking-[0.12em] text-[var(--text-muted)] uppercase">
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
                      className={compactInputClassName}
                      options={fieldOptions}
                      value={row.field}
                      onFocus={onNeedFields}
                      onChange={(e) => updateRow(index, { field: e.target.value })}
                      placeholder="status"
                    />
                  </td>
                  <td className="px-3 py-2">
                    <select
                      className={compactInputClassName}
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
                        className={compactInputClassName}
                        value={row.value}
                        onChange={(e) => updateRow(index, { value: e.target.value })}
                        placeholder={
                          row.operator === 'in' || row.operator === 'not_in'
                            ? 'a, b, c'
                            : row.operator === 'like'
                              ? 'omar'
                              : '123'
                        }
                        title={
                          row.operator === 'like'
                            ? 'Matches anywhere in the value. Add % to write your own pattern, for example %omar.'
                            : undefined
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

      <SecondaryButton type="button" onClick={addRow}>
        <PlusIcon />
        Add filter
      </SecondaryButton>
    </div>
  )
}
