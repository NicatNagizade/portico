/** Column names for autocomplete datalists. Accepts [{name,type}] or legacy string[]. */
export function columnNames(columns = []) {
  return columns
    .map((col) => (typeof col === 'string' ? col : col?.name))
    .filter(Boolean)
}

/**
 * Build field override rows from source columns.
 * Keeps an existing row's id / destination_name / active when source_name matches.
 */
export function fieldsFromSourceColumns(columns = [], existing = []) {
  const bySource = new Map(
    existing.filter((f) => f.source_name?.trim()).map((f) => [f.source_name.trim(), f]),
  )

  return columns
    .map((col) => {
      const name = typeof col === 'string' ? col : col?.name
      if (!name) return null
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
