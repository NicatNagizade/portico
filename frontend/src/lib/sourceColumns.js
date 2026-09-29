/** Column names for autocomplete datalists. Accepts [{name,type}] or legacy string[]. */
export function columnNames(columns = []) {
  return columns
    .map((col) => (typeof col === 'string' ? col : col?.name))
    .filter(Boolean)
}

/**
 * Top-level column names for the explore Fields picker / CSV column selection.
 * Does not flatten nested relation fields (those stay as whole relations).
 * Skips dotted names (e.g. Typesense posts.title) if a schema still exposes them.
 * Explore Filters should use flattenColumnNames instead.
 */
export function selectableColumnNames(columns = []) {
  return columnNames(columns).filter((name) => !String(name).includes('.'))
}

/**
 * Flatten nested schema columns into dotted paths (e.g. posts.title).
 * Used by explore filters when the side connection is a document store.
 */
export function flattenColumnNames(columns = [], prefix = '') {
  const out = []
  for (const col of columns) {
    const name = typeof col === 'string' ? col : col?.name
    if (!name) continue
    const path = prefix ? `${prefix}.${name}` : name
    out.push(path)
    const nested = typeof col === 'object' && Array.isArray(col.columns) ? col.columns : []
    if (nested.length) {
      out.push(...flattenColumnNames(nested, path))
    }
  }
  return out
}

/**
 * Build field override rows from source columns.
 * Keeps an existing row's id / destination_name / active when source_name matches.
 * Uses top-level columns only (nested relation fields stay under Relations).
 */
export function fieldsFromSourceColumns(columns = [], existing = []) {
  const bySource = new Map(
    existing.filter((f) => f.source_name?.trim()).map((f) => [f.source_name.trim(), f]),
  )

  return columns
    .map((col) => {
      const name = typeof col === 'string' ? col : col?.name
      if (!name || String(name).includes('.')) return null
      const type = typeof col === 'string' ? '' : col?.type || ''
      const prev = bySource.get(name)
      return {
        id: prev?.id,
        source_name: name,
        destination_name: prev?.destination_name || name,
        destination_type: type || prev?.destination_type || '',
        active: prev?.active !== false,
        values: prev?.values || [],
      }
    })
    .filter(Boolean)
}
