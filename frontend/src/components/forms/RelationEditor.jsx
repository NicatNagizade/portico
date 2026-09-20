import { RELATION_TYPES } from '../../lib/destinationTypes'
import AutocompleteInput from '../AutocompleteInput'
import { GhostButton, SecondaryButton, inputClassName } from '../ui'
import FieldEditor from './FieldEditor'

let nextTempId = -1

function allocTempId() {
  const id = nextTempId
  nextTempId -= 1
  return id
}

function emptyRelation() {
  return {
    id: allocTempId(),
    name: '',
    type: 'has_many',
    table: '',
    pivot_table: '',
    foreign_key: '',
    related_key: '',
    parent_id: '',
    active: true,
    fields: [],
  }
}

export default function RelationEditor({
  relations,
  onChange,
  tables = [],
  columnsByTable = {},
  sourceTable = '',
  onNeedTables,
  onNeedColumns,
}) {
  function updateRow(index, patch) {
    onChange(relations.map((row, i) => (i === index ? { ...row, ...patch } : row)))
  }

  function removeRow(index) {
    const removed = relations[index]
    onChange(
      relations
        .filter((_, i) => i !== index)
        .map((row) =>
          String(row.parent_id) === String(removed.id) ? { ...row, parent_id: '' } : row,
        ),
    )
  }

  function parentTableFor(row) {
    if (row.parent_id === '' || row.parent_id == null) {
      return sourceTable
    }
    const parent = relations.find((r) => String(r.id) === String(row.parent_id))
    return parent?.table || sourceTable
  }

  return (
    <div className="space-y-4">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-2">
          <span className="mt-0.5 flex h-6 w-6 items-center justify-center rounded-md bg-[var(--accent)] font-mono text-[10px] font-bold text-white">
            04
          </span>
          <div>
            <h3 className="text-sm font-semibold text-[var(--text)]">Relations</h3>
            <p className="text-xs text-[var(--text-muted)]">
              Enrich documents with related rows. Nest via parent relation. belongs_to_many stores
              pivot_table in relation config.
            </p>
          </div>
        </div>
        <SecondaryButton type="button" onClick={() => onChange([...relations, emptyRelation()])}>
          Add relation
        </SecondaryButton>
      </div>

      {relations.length === 0 ? (
        <p className="rounded-xl border border-dashed border-[var(--border)] px-4 py-6 text-center text-sm text-[var(--text-muted)]">
          No relations — destination docs stay flat.
        </p>
      ) : (
        <div className="space-y-3">
          {relations.map((row, index) => {
            const isBelongsToMany = row.type === 'belongs_to_many'
            const isBelongsTo = row.type === 'belongs_to'
            const relatedColumns = columnsByTable[row.table] || []
            const pivotColumns = columnsByTable[row.pivot_table] || []
            const parentTable = parentTableFor(row)
            const parentColumns = columnsByTable[parentTable] || []
            const fkTable = isBelongsToMany ? row.pivot_table : isBelongsTo ? parentTable : row.table
            const fkColumns = isBelongsToMany
              ? pivotColumns
              : isBelongsTo
                ? parentColumns
                : relatedColumns
            const parentOptions = relations.filter((r) => r.id !== row.id && r.name)

            return (
              <div
                key={row.id}
                className="space-y-4 rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4 shadow-[var(--shadow-sm)]"
              >
                <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                  <label className="block text-xs">
                    <span className="mb-1.5 block text-[var(--text-muted)]">Name</span>
                    <input
                      className={inputClassName}
                      value={row.name}
                      onChange={(e) => updateRow(index, { name: e.target.value })}
                      required
                    />
                  </label>
                  <label className="block text-xs">
                    <span className="mb-1.5 block text-[var(--text-muted)]">Type</span>
                    <select
                      className={inputClassName}
                      value={row.type}
                      onChange={(e) => updateRow(index, { type: e.target.value })}
                    >
                      {RELATION_TYPES.map((opt) => (
                        <option key={opt.value} value={opt.value}>
                          {opt.label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="block text-xs">
                    <span className="mb-1.5 block text-[var(--text-muted)]">Related table</span>
                    <AutocompleteInput
                      options={tables}
                      value={row.table}
                      onFocus={onNeedTables}
                      onChange={(e) => updateRow(index, { table: e.target.value })}
                      required
                    />
                  </label>
                  {isBelongsToMany && (
                    <label className="block text-xs">
                      <span className="mb-1.5 block text-[var(--text-muted)]">Pivot table</span>
                      <AutocompleteInput
                        options={tables}
                        value={row.pivot_table || ''}
                        onFocus={onNeedTables}
                        onChange={(e) => updateRow(index, { pivot_table: e.target.value })}
                        required
                      />
                    </label>
                  )}
                  <label className="block text-xs">
                    <span className="mb-1.5 block text-[var(--text-muted)]">Foreign key</span>
                    <AutocompleteInput
                      options={fkColumns}
                      value={row.foreign_key || ''}
                      onFocus={() => fkTable && onNeedColumns?.(fkTable)}
                      onChange={(e) => updateRow(index, { foreign_key: e.target.value })}
                      placeholder="auto"
                    />
                  </label>
                  {isBelongsToMany && (
                    <label className="block text-xs">
                      <span className="mb-1.5 block text-[var(--text-muted)]">Related key</span>
                      <AutocompleteInput
                        options={pivotColumns}
                        value={row.related_key || ''}
                        onFocus={() => row.pivot_table && onNeedColumns?.(row.pivot_table)}
                        onChange={(e) => updateRow(index, { related_key: e.target.value })}
                        placeholder="auto"
                      />
                    </label>
                  )}
                  <label className="block text-xs">
                    <span className="mb-1.5 block text-[var(--text-muted)]">Parent relation</span>
                    <select
                      className={inputClassName}
                      value={row.parent_id === '' || row.parent_id == null ? '' : String(row.parent_id)}
                      onChange={(e) =>
                        updateRow(index, {
                          parent_id: e.target.value === '' ? '' : Number(e.target.value),
                        })
                      }
                    >
                      <option value="">root (none)</option>
                      {parentOptions.map((r) => (
                        <option key={r.id} value={r.id}>
                          {r.name}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      className="h-4 w-4 accent-[var(--accent)]"
                      checked={row.active !== false}
                      onChange={(e) => updateRow(index, { active: e.target.checked })}
                    />
                    Active
                  </label>
                  <div className="flex items-center">
                    <GhostButton type="button" onClick={() => removeRow(index)}>
                      Remove
                    </GhostButton>
                  </div>
                </div>

                <div className="rounded-lg border border-dashed border-[var(--border)] bg-[var(--bg-elevated)]/50 p-3">
                  <FieldEditor
                    compact
                    title="Relation field overrides"
                    fields={row.fields || []}
                    onChange={(fields) => updateRow(index, { fields })}
                    sourceColumns={relatedColumns}
                    onNeedSourceColumns={() => onNeedColumns?.(row.table)}
                  />
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
