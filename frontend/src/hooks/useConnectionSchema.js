import { useRef, useState } from 'react'
import { listConnectionColumns, listConnectionTables } from '../api/connections'

/**
 * Lazy source-schema helpers for autocomplete.
 * Fetches only when ensureTables / ensureColumns is called (e.g. on focus),
 * and never repeats a successful or in-flight request for the same key.
 */
export default function useConnectionSchema(connectionId) {
  const [state, setState] = useState({
    connectionId: null,
    tables: [],
    tablesLoaded: false,
    columnsByTable: {},
  })
  const pendingTables = useRef(null)
  const pendingColumns = useRef(new Set())

  // Ignore cached data from a previous connection without resetting in an effect.
  const tables = state.connectionId === connectionId ? state.tables : []
  const columnsByTable = state.connectionId === connectionId ? state.columnsByTable : {}
  const tablesLoaded = state.connectionId === connectionId && state.tablesLoaded

  async function ensureTables() {
    if (!connectionId || tablesLoaded || pendingTables.current === connectionId) return

    pendingTables.current = connectionId
    try {
      const names = await listConnectionTables(connectionId)
      setState((prev) => ({
        connectionId,
        tables: names,
        tablesLoaded: true,
        columnsByTable: prev.connectionId === connectionId ? prev.columnsByTable : {},
      }))
    } catch {
      setState((prev) => ({
        connectionId,
        tables: [],
        tablesLoaded: true,
        columnsByTable: prev.connectionId === connectionId ? prev.columnsByTable : {},
      }))
    } finally {
      if (pendingTables.current === connectionId) pendingTables.current = null
    }
  }

  async function ensureColumns(table) {
    const name = table?.trim()
    if (!connectionId || !name) return

    // Skip partial names once we know the real table list.
    if (tablesLoaded && !tables.includes(name)) return

    const cacheKey = `${connectionId}:${name}`
    if (name in columnsByTable || pendingColumns.current.has(cacheKey)) return

    pendingColumns.current.add(cacheKey)
    try {
      const columns = await listConnectionColumns(connectionId, name)
      setState((prev) => mergeColumns(prev, connectionId, name, columns))
    } catch {
      setState((prev) => mergeColumns(prev, connectionId, name, []))
    } finally {
      pendingColumns.current.delete(cacheKey)
    }
  }

  return { tables, columnsByTable, ensureTables, ensureColumns }
}

function mergeColumns(prev, connectionId, name, columns) {
  if (prev.connectionId === connectionId && name in prev.columnsByTable) return prev
  return {
    connectionId,
    tables: prev.connectionId === connectionId ? prev.tables : [],
    tablesLoaded: prev.connectionId === connectionId ? prev.tablesLoaded : false,
    columnsByTable: {
      ...(prev.connectionId === connectionId ? prev.columnsByTable : {}),
      [name]: columns,
    },
  }
}
