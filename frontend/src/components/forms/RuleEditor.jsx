import { RULE_OPERATORS, ruleNeedsValue } from '../../lib/ruleOperators'
import AutocompleteInput from '../AutocompleteInput'
import { IconButton, SecondaryButton } from '../ui'

const compactInput =
  'w-full rounded-md border border-[var(--border)] bg-[var(--surface)] px-2.5 py-1.5 text-sm text-[var(--text)] outline-none placeholder:text-[var(--text-muted)]/70 focus:border-[var(--accent)] focus:shadow-[0_0_0_2px_var(--accent-soft)]'

function emptyRule() {
  return {
    id: undefined,
    field: '',
    operator: 'eq',
    value: '',
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

export default function RuleEditor({
  rules,
  onChange,
  sourceColumns = [],
  onNeedSourceColumns,
}) {
  function updateRow(index, patch) {
    onChange(rules.map((row, i) => (i === index ? { ...row, ...patch } : row)))
  }

  function removeRow(index) {
    onChange(rules.filter((_, i) => i !== index))
  }

  return (
    <div className="space-y-3">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-2">
          <span className="mt-0.5 flex h-6 w-6 items-center justify-center rounded-md bg-[var(--accent)] font-mono text-[10px] font-bold text-white">
            03
          </span>
          <div>
            <h3 className="text-sm font-semibold text-[var(--text)]">Filter rules</h3>
            <p className="text-xs text-[var(--text-muted)]">
              Only rows matching all active rules are imported. Example: client_id = 123.
            </p>
          </div>
        </div>
        <SecondaryButton type="button" onClick={() => onChange([...rules, emptyRule()])}>
          Add rule
        </SecondaryButton>
      </div>

      {rules.length === 0 ? (
        <p className="rounded-lg border border-dashed border-[var(--border)] px-3 py-4 text-center text-sm text-[var(--text-muted)]">
          No filters — all source rows are imported.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-[var(--border)]">
          <table className="w-full min-w-[560px] border-collapse text-left text-sm">
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
                <th className="px-3 py-2 font-mono text-[10px] font-semibold tracking-[0.12em] text-[var(--text-muted)] uppercase">
                  Active
                </th>
                <th className="w-10 px-2 py-2" />
              </tr>
            </thead>
            <tbody>
              {rules.map((row, index) => {
                const needsValue = ruleNeedsValue(row.operator)
                return (
                  <tr key={row.id ?? `new-${index}`} className="border-b border-[var(--border)] last:border-b-0">
                    <td className="px-3 py-2">
                      <AutocompleteInput
                        required
                        className={compactInput}
                        options={sourceColumns}
                        value={row.field}
                        onFocus={onNeedSourceColumns}
                        onChange={(e) => updateRow(index, { field: e.target.value })}
                        placeholder="client_id"
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
                          placeholder={row.operator === 'in' || row.operator === 'not_in' ? 'a, b, c' : '123'}
                        />
                      ) : (
                        <span className="font-mono text-xs text-[var(--text-muted)]">—</span>
                      )}
                    </td>
                    <td className="px-3 py-2">
                      <input
                        type="checkbox"
                        className="h-4 w-4 accent-[var(--accent)]"
                        checked={row.active !== false}
                        onChange={(e) => updateRow(index, { active: e.target.checked })}
                      />
                    </td>
                    <td className="px-2 py-2">
                      <IconButton type="button" label="Remove rule" tone="danger" onClick={() => removeRow(index)}>
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
