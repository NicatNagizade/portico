import { RELATION_TYPES } from '../../lib/destinationTypes'
import { GhostButton, SecondaryButton, inputClassName } from '../ui'

function emptyRelation() {
  return {
    name: '',
    type: 'belongs_to_many',
    table: '',
    pivot_table: '',
    foreign_key: '',
    related_key: '',
    active: true,
  }
}

export default function RelationEditor({ relations, onChange }) {
  function updateRow(index, patch) {
    onChange(relations.map((row, i) => (i === index ? { ...row, ...patch } : row)))
  }

  function removeRow(index) {
    onChange(relations.filter((_, i) => i !== index))
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
              Enrich documents with related rows via pivot tables.
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
          {relations.map((row, index) => (
            <div
              key={index}
              className="grid gap-3 rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4 shadow-[var(--shadow-sm)] sm:grid-cols-2 lg:grid-cols-3"
            >
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
                <input
                  className={inputClassName}
                  value={row.table}
                  onChange={(e) => updateRow(index, { table: e.target.value })}
                  required
                />
              </label>
              <label className="block text-xs">
                <span className="mb-1.5 block text-[var(--text-muted)]">Pivot table</span>
                <input
                  className={inputClassName}
                  value={row.pivot_table || ''}
                  onChange={(e) => updateRow(index, { pivot_table: e.target.value })}
                />
              </label>
              <label className="block text-xs">
                <span className="mb-1.5 block text-[var(--text-muted)]">Foreign key</span>
                <input
                  className={inputClassName}
                  value={row.foreign_key || ''}
                  onChange={(e) => updateRow(index, { foreign_key: e.target.value })}
                  placeholder="auto"
                />
              </label>
              <label className="block text-xs">
                <span className="mb-1.5 block text-[var(--text-muted)]">Related key</span>
                <input
                  className={inputClassName}
                  value={row.related_key || ''}
                  onChange={(e) => updateRow(index, { related_key: e.target.value })}
                  placeholder="auto"
                />
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
          ))}
        </div>
      )}
    </div>
  )
}
