import { useState } from 'react'
import { RELATION_TYPES } from '../../lib/relationTypes'
import { columnNames, fieldsFromSourceColumns } from '../../lib/sourceColumns'
import AutocompleteInput from '../AutocompleteInput'
import { GhostButton, IconButton, MetaChip, SecondaryButton, Toggle } from '../ui'
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
    active: true,
    fields: [],
    relations: [],
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

function countOwnFields(row) {
  return (row.fields || []).filter((f) => f.source_name?.trim()).length
}

function RelationCard({
  row,
  depth,
  parentTable,
  onChange,
  onRemove,
  tables,
  columnsByTable,
  onNeedTables,
  onNeedColumns,
  expanded,
  setExpanded,
}) {
  const isOpen = expanded.has(String(row.id))
  const isBelongsToMany = row.type === 'belongs_to_many'
  const isBelongsTo = row.type === 'belongs_to'
  const isHasManyOrOne = row.type === 'has_many' || row.type === 'has_one'
  const relatedColumns = columnsByTable[row.table] || []
  const pivotColumns = columnsByTable[row.pivot_table] || []
  const parentColumns = columnsByTable[parentTable] || []
  // FK lives on: pivot (m2m), parent (belongs_to), or related (has_many/has_one).
  const fkTable = isBelongsToMany ? row.pivot_table : isBelongsTo ? parentTable : row.table
  const fkColumns = isBelongsToMany
    ? pivotColumns
    : isBelongsTo
      ? parentColumns
      : relatedColumns
  // Related key lives on: pivot (m2m), parent (has_many/has_one local key), or related (belongs_to owner key).
  const rkTable = isBelongsToMany
    ? row.pivot_table
    : isHasManyOrOne
      ? parentTable
      : row.table
  const rkColumns = isBelongsToMany
    ? pivotColumns
    : isHasManyOrOne
      ? parentColumns
      : relatedColumns
  const fieldCount = countOwnFields(row)
  const children = row.relations || []
  const label = row.name.trim() || 'Untitled relation'
  const [customKeys, setCustomKeys] = useState(
    () => Boolean(row.foreign_key?.trim() || row.related_key?.trim()),
  )

  function toggleExpanded() {
    const key = String(row.id)
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  function patch(next) {
    onChange({ ...row, ...next })
  }

  function updateChild(index, next) {
    patch({
      relations: children.map((child, i) => (i === index ? next : child)),
    })
  }

  function removeChild(index) {
    const removed = children[index]
    setExpanded((prev) => {
      const next = new Set(prev)
      next.delete(String(removed.id))
      return next
    })
    patch({ relations: children.filter((_, i) => i !== index) })
  }

  function addChild() {
    const child = emptyRelation()
    patch({ relations: [...children, child] })
    setExpanded((prev) => new Set(prev).add(String(child.id)))
  }

  return (
    <div className="space-y-2">
      <div
        style={{ marginLeft: depth ? Math.min(depth, 4) * 12 : 0 }}
        className="overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--surface)]"
      >
        <div className="flex items-center gap-2 px-2 py-2 sm:px-3">
          <button
            type="button"
            className="flex min-w-0 flex-1 flex-wrap items-center gap-1.5 rounded-md px-1 py-1 text-left hover:bg-[var(--bg-elevated)]/80"
            onClick={toggleExpanded}
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
          <IconButton label="Remove relation" tone="danger" onClick={onRemove}>
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
                  onChange={(e) => patch({ name: e.target.value })}
                  required
                  placeholder="posts"
                />
              </label>
              <label className="block text-xs">
                <span className="mb-1 block text-[var(--text-muted)]">Type</span>
                <select
                  className={compactInput}
                  value={row.type}
                  onChange={(e) => patch({ type: e.target.value })}
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
                  onChange={(e) => patch({ table: e.target.value })}
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
                    onChange={(e) => patch({ pivot_table: e.target.value })}
                    required
                  />
                </label>
              )}
            </div>

            <div className="space-y-3">
              <Toggle
                checked={row.active !== false}
                onChange={(on) => patch({ active: on })}
                label="Active"
                description="Inactive relations are skipped during sync."
              />
              <Toggle
                checked={customKeys}
                onChange={(on) => {
                  setCustomKeys(on)
                  if (!on) patch({ foreign_key: '', related_key: '' })
                }}
                label="Custom foreign / related keys"
                description="Leave off to infer keys from the relation name."
              />
              {customKeys ? (
                <div className="grid gap-3 sm:grid-cols-2">
                  <label className="block text-xs">
                    <span className="mb-1 block text-[var(--text-muted)]">Foreign key</span>
                    <AutocompleteInput
                      className={compactInput}
                      options={columnNames(fkColumns)}
                      value={row.foreign_key || ''}
                      onFocus={() => fkTable && onNeedColumns?.(fkTable)}
                      onChange={(e) => patch({ foreign_key: e.target.value })}
                      placeholder="auto"
                    />
                  </label>
                  <label className="block text-xs">
                    <span className="mb-1 block text-[var(--text-muted)]">Related key</span>
                    <AutocompleteInput
                      className={compactInput}
                      options={columnNames(rkColumns)}
                      value={row.related_key || ''}
                      onFocus={() => rkTable && onNeedColumns?.(rkTable)}
                      onChange={(e) => patch({ related_key: e.target.value })}
                      placeholder="auto"
                    />
                  </label>
                </div>
              ) : null}
            </div>

            <div className="rounded-lg border border-dashed border-[var(--border)] bg-[var(--surface)]/70 p-3">
              <FieldEditor
                compact
                title="Field overrides on related rows"
                fields={row.fields || []}
                onChange={(fields) => patch({ fields })}
                sourceColumns={relatedColumns}
                onNeedSourceColumns={() => onNeedColumns?.(row.table)}
                onAutofill={
                  row.table?.trim()
                    ? async () => {
                        const columns = (await onNeedColumns?.(row.table)) || []
                        patch({
                          fields: fieldsFromSourceColumns(columns, row.fields || []),
                        })
                      }
                    : undefined
                }
              />
            </div>

            <div className="flex items-center justify-between gap-2">
              <SecondaryButton type="button" onClick={addChild}>
                Add nested relation
              </SecondaryButton>
              <GhostButton type="button" onClick={toggleExpanded}>
                Done
              </GhostButton>
            </div>
          </div>
        ) : null}
      </div>

      {children.map((child, index) => (
        <RelationCard
          key={child.id}
          row={child}
          depth={depth + 1}
          parentTable={row.table || parentTable}
          onChange={(next) => updateChild(index, next)}
          onRemove={() => removeChild(index)}
          tables={tables}
          columnsByTable={columnsByTable}
          onNeedTables={onNeedTables}
          onNeedColumns={onNeedColumns}
          expanded={expanded}
          setExpanded={setExpanded}
        />
      ))}
    </div>
  )
}

export default function RelationEditor({
  relations,
  onChange,
  tables = [],
  columnsByTable = {},
  sourceTable = '',
  onNeedTables,
  onNeedColumns,
  hideHeader = false,
}) {
  const [expanded, setExpanded] = useState(() => new Set())

  function updateRow(index, next) {
    onChange(relations.map((row, i) => (i === index ? next : row)))
  }

  function removeRow(index) {
    const removed = relations[index]
    setExpanded((prev) => {
      const next = new Set(prev)
      next.delete(String(removed.id))
      return next
    })
    onChange(relations.filter((_, i) => i !== index))
  }

  function addRelation() {
    const row = emptyRelation()
    onChange([...relations, row])
    setExpanded((prev) => new Set(prev).add(String(row.id)))
  }

  return (
    <div className="space-y-3">
      <div
        className={[
          'flex items-start gap-3',
          hideHeader ? 'justify-end' : 'justify-between',
        ].join(' ')}
      >
        {hideHeader ? null : (
          <div className="flex items-start gap-2">
            <span className="mt-0.5 flex h-6 w-6 items-center justify-center rounded-md bg-[var(--accent)] font-mono text-[10px] font-bold text-white">
              05
            </span>
            <div>
              <h3 className="text-sm font-semibold text-[var(--text)]">Relations</h3>
              <p className="text-xs text-[var(--text-muted)]">
                Nested related rows on the destination document. Nest children under a relation.
              </p>
            </div>
          </div>
        )}
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
          {relations.map((row, index) => (
            <RelationCard
              key={row.id}
              row={row}
              depth={0}
              parentTable={sourceTable}
              onChange={(next) => updateRow(index, next)}
              onRemove={() => removeRow(index)}
              tables={tables}
              columnsByTable={columnsByTable}
              onNeedTables={onNeedTables}
              onNeedColumns={onNeedColumns}
              expanded={expanded}
              setExpanded={setExpanded}
            />
          ))}
        </div>
      )}
    </div>
  )
}
