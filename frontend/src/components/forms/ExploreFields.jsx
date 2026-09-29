import { useState } from 'react'
import { Checkbox, GhostButton, inputClassName } from '../ui'

/** Checkbox list for choosing which explore columns to show. selected=null means all. */
export default function ExploreFields({ options = [], selected, onChange }) {
  const [query, setQuery] = useState('')
  const selectedSet = selected === null ? null : new Set(selected)
  const needle = query.trim().toLowerCase()
  const visible = needle
    ? options.filter((name) => name.toLowerCase().includes(needle))
    : options

  function isChecked(name) {
    return selectedSet === null || selectedSet.has(name)
  }

  function toggle(name, checked) {
    if (selectedSet === null) {
      if (checked) return
      onChange(options.filter((o) => o !== name))
      return
    }
    const next = new Set(selectedSet)
    if (checked) next.add(name)
    else next.delete(name)
    if (next.size === options.length && options.every((o) => next.has(o))) {
      onChange(null)
      return
    }
    onChange(options.filter((o) => next.has(o)))
  }

  if (options.length === 0) {
    return (
      <p className="rounded-lg border border-dashed border-[var(--border)] px-3 py-6 text-center text-sm text-[var(--text-muted)]">
        No fields yet — preview once or wait for schema to load.
      </p>
    )
  }

  const checkedCount = selectedSet === null ? options.length : selectedSet.size

  return (
    <div className="space-y-3">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-xs text-[var(--text-muted)]">
          {checkedCount} of {options.length} selected
          {selectedSet === null ? ' · showing all' : ''}
        </p>
        <div className="flex gap-1">
          <GhostButton type="button" onClick={() => onChange(null)}>
            Select all
          </GhostButton>
          <GhostButton type="button" onClick={() => onChange([])}>
            Clear
          </GhostButton>
        </div>
      </div>

      {options.length > 8 ? (
        <input
          type="search"
          className={inputClassName}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search fields…"
          aria-label="Search fields"
        />
      ) : null}

      {visible.length === 0 ? (
        <p className="rounded-lg border border-dashed border-[var(--border)] px-3 py-4 text-center text-sm text-[var(--text-muted)]">
          No fields match “{query.trim()}”.
        </p>
      ) : (
        <ul className="max-h-[min(50vh,420px)] divide-y divide-[var(--border)] overflow-y-auto rounded-lg border border-[var(--border)]">
          {visible.map((name) => (
            <li key={name}>
              <Checkbox
                className="px-3 py-2.5 hover:bg-[var(--bg-elevated)]"
                checked={isChecked(name)}
                onChange={(checked) => toggle(name, checked)}
                aria-label={name}
                label={
                  <span className="font-mono text-[13px] font-normal tracking-tight text-[var(--text)]">
                    {name}
                  </span>
                }
              />
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
