import { useState } from 'react'
import { RELATION_TYPES } from '../../lib/relationTypes'
import AutocompleteInput from '../AutocompleteInput'
import { GhostButton, IconButton, MetaChip, SecondaryButton } from '../ui'
import FieldEditor from './FieldEditor'

const compactInput =
  'w-full rounded-md border border-[var(--border)] bg-[var(--surface)] px-2.5 py-1.5 text-sm text-[var(--text)] outline-none placeholder:text-[var(--text-muted)]/70 focus:border-[var(--accent)] focus:shadow-[0_0_0_2px_var(--accent-soft)]'

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

function ChevronIcon({ open }) {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
      className={`transition-transform ${open ? 'rotate-90' : ''}`}
    >
      <path
        d="M9 6l6 6-6 6"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}

function depthOf(row, relations) {
  let depth = 0
  let current = row
  const seen = new Set()
  while (current?.parent_id !== '' && current?.parent_id != null) {
    if (seen.has(String(current.id))) break
    seen.add(String(current.id))
    const parent = relations.find((r) => String(r.id) === String(current.parent_id))
    if (!parent) break
    depth += 1
    current = parent
  }
  return depth
}

function parentName(row, relations) {
  if (row.parent_id === '' || row.parent_id == null) return null
  return relations.find((r) => String(r.id) === String(row.parent_id))?.name || null
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
  const [expanded, setExpanded] = useState(() => new Set())

  function updateRow(index, patch) {
    onChange(relations.map((row, i) => (i === index ? { ...row, ...patch } : row)))
  }

  function removeRow(index) {
    const removed = relations[index]
    setExpanded((prev) => {
      const next = new Set(prev)
      next.delete(String(removed.id))
      return next
    })
    onChange(
      relations
        .filter((_, i) => i !== index)
        .map((row) =>
          String(row.parent_id) === String(removed.id) ? { ...row, parent_id: '' } : row,
        ),
    )
  }

  function addRelation() {
    const row = emptyRelation()
    onChange([...relations, row])
    setExpanded((prev) => new Set(prev).add(String(row.id)))
  }

  function toggleExpanded(id) {
    const key = String(id)
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  function parentTableFor(row) {
    if (row.parent_id === '' || row.parent_id == null) {
      return sourceTable
    }
    const parent = relations.find((r) => String(r.id) === String(row.parent_id))
    return parent?.table || sourceTable
  }

  return (
    <div className="space-y-3">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-2">
          <span className="mt-0.5 flex h-6 w-6 items-center justify-center rounded-md bg-[var(--accent)] font-mono text-[10px] font-bold text-white">
            05
          </span>
          <div>
            <h3 className="text-sm font-semibold text-[var(--text)]">Relations</h3>
            <p className="text-xs text-[var(--text-muted)]">
              Nested related rows on the destination document. Collapse cards you are not editing.
            </p>
          </div>
        </div>
        <SecondaryButton type="button" onClick={addRelation}>
          Add relation
        </SecondaryButton>
      </div>

      {relations.length === 0 ? (
        <p className="rounded-lg border border-dashed border-[var(--border)] px-3 py-4 text-center text-sm text-[var(--text-muted)]">
          No relations — destination docs stay flat.
        </p>
      ) : (
        <div className="space-y-2">
          {relations.map((row, index) => {
            const isOpen = expanded.has(String(row.id))
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
            const depth = depthOf(row, relations)
            const nestedUnder = parentName(row, relations)
            const fieldCount = (row.fields || []).filter((f) => f.source_name?.trim()).length
            const label = row.name.trim() || 'Untitled relation'

            return (
              <div
                key={row.id}
                style={{ marginLeft: depth ? Math.min(depth, 4) * 12 : 0 }}
                className="overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--surface)]"
              >
                <div className="flex items-center gap-2 px-2 py-2 sm:px-3">
                  <button
                    type="button"
                    className="flex min-w-0 flex-1 flex-wrap items-center gap-1.5 rounded-md px-1 py-1 text-left hover:bg-[var(--bg-elevated)]/80"
                    onClick={() => toggleExpanded(row.id)}
                    aria-expanded={isOpen}
                  >
                    <span className="text-[var(--text-muted)]">
                      <ChevronIcon open={isOpen} />
                    </span>
                    <span className="max-w-[12rem] truncate text-sm font-medium text-[var(--text)] sm:max-w-none">
                      {label}
                    </span>
                    <MetaChip>{row.type}</MetaChip>
                    {row.table ? <MetaChip>{row.table}</MetaChip> : null}
                    {nestedUnder ? <MetaChip>under {nestedUnder}</MetaChip> : null}
                    {fieldCount > 0 ? (
                      <MetaChip>
                        {fieldCount} field{fieldCount === 1 ? '' : 's'}
                      </MetaChip>
                    ) : null}
                    {row.active === false ? (
                      <span className="rounded-md bg-[var(--danger-soft)] px-2 py-0.5 font-mono text-[11px] text-[var(--danger)]">
                        off
                      </span>
                    ) : null}
                  </button>
                  <IconButton
                    label="Remove relation"
                    tone="danger"
                    onClick={() => removeRow(index)}
                  >
                    <TrashIcon />
                  </IconButton>
                </div>

                {isOpen ? (
                  <div className="space-y-4 border-t border-[var(--border)] bg-[var(--bg-elevated)]/40 px-3 py-3 sm:px-4">
                    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                      <label className="block text-xs">
                        <span className="mb-1 block text-[var(--text-muted)]">Name</span>
                        <input
                          className={compactInput}
                          value={row.name}
                          onChange={(e) => updateRow(index, { name: e.target.value })}
                          required
                          placeholder="posts"
                        />
                      </label>
                      <label className="block text-xs">
                        <span className="mb-1 block text-[var(--text-muted)]">Type</span>
                        <select
                          className={compactInput}
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
                        <span className="mb-1 block text-[var(--text-muted)]">Related table</span>
                        <AutocompleteInput
                          className={compactInput}
                          options={tables}
                          value={row.table}
                          onFocus={onNeedTables}
                          onChange={(e) => updateRow(index, { table: e.target.value })}
                          required
                        />
                      </label>
                      {isBelongsToMany && (
                        <label className="block text-xs">
                          <span className="mb-1 block text-[var(--text-muted)]">Pivot table</span>
                          <AutocompleteInput
                            className={compactInput}
                            options={tables}
                            value={row.pivot_table || ''}
                            onFocus={onNeedTables}
                            onChange={(e) => updateRow(index, { pivot_table: e.target.value })}
                            required
                          />
                        </label>
                      )}
                      <label className="block text-xs">
                        <span className="mb-1 block text-[var(--text-muted)]">Foreign key</span>
                        <AutocompleteInput
                          className={compactInput}
                          options={fkColumns}
                          value={row.foreign_key || ''}
                          onFocus={() => fkTable && onNeedColumns?.(fkTable)}
                          onChange={(e) => updateRow(index, { foreign_key: e.target.value })}
                          placeholder="auto"
                        />
                      </label>
                      {isBelongsToMany && (
                        <label className="block text-xs">
                          <span className="mb-1 block text-[var(--text-muted)]">Related key</span>
                          <AutocompleteInput
                            className={compactInput}
                            options={pivotColumns}
                            value={row.related_key || ''}
                            onFocus={() => row.pivot_table && onNeedColumns?.(row.pivot_table)}
                            onChange={(e) => updateRow(index, { related_key: e.target.value })}
                            placeholder="auto"
                          />
                        </label>
                      )}
                      <label className="block text-xs">
                        <span className="mb-1 block text-[var(--text-muted)]">Parent relation</span>
                        <select
                          className={compactInput}
                          value={
                            row.parent_id === '' || row.parent_id == null
                              ? ''
                              : String(row.parent_id)
                          }
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
                      <label className="flex items-end gap-2 pb-1.5 text-sm">
                        <input
                          type="checkbox"
                          className="h-4 w-4 accent-[var(--accent)]"
                          checked={row.active !== false}
                          onChange={(e) => updateRow(index, { active: e.target.checked })}
                        />
                        Active
                      </label>
                    </div>

                    <div className="rounded-lg border border-dashed border-[var(--border)] bg-[var(--surface)]/70 p-3">
                      <FieldEditor
                        compact
                        title="Field overrides on related rows"
                        fields={row.fields || []}
                        onChange={(fields) => updateRow(index, { fields })}
                        sourceColumns={relatedColumns}
                        onNeedSourceColumns={() => onNeedColumns?.(row.table)}
                      />
                    </div>

                    <div className="flex justify-end">
                      <GhostButton type="button" onClick={() => toggleExpanded(row.id)}>
                        Done
                      </GhostButton>
                    </div>
                  </div>
                ) : null}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
