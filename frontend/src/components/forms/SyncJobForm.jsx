import { useMemo, useState } from 'react'
import { parseConfig } from '../../lib/connectionTypes'
import { ruleNeedsValue } from '../../lib/ruleOperators'
import useConnectionSchema from '../../hooks/useConnectionSchema'
import AutocompleteInput from '../AutocompleteInput'
import { Field, PrimaryButton, inputClassName } from '../ui'
import FieldEditor from './FieldEditor'
import RelationEditor from './RelationEditor'
import RuleEditor from './RuleEditor'

function toArrayCsv(value) {
  if (Array.isArray(value)) return value.join(', ')
  if (typeof value === 'string') return value
  return ''
}

function fromCsv(value) {
  return value
    .split(',')
    .map((part) => part.trim())
    .filter(Boolean)
}

function buildInitialConfig(config) {
  const parsed = parseConfig(config)
  return {
    default_sorting_field: parsed.default_sorting_field || '',
    enable_nested_fields:
      parsed.enable_nested_fields === undefined ? true : Boolean(parsed.enable_nested_fields),
    symbols_to_index: toArrayCsv(parsed.symbols_to_index),
    token_separators: toArrayCsv(parsed.token_separators),
  }
}

function mapField(f) {
  return {
    id: f.id,
    source_name: f.source_name || '',
    destination_name: f.destination_name || '',
    destination_type: f.destination_type || '',
    active: f.active !== false,
  }
}

function mapRule(r) {
  return {
    id: r.id,
    field: r.field || '',
    operator: r.operator || 'eq',
    value: r.value || '',
    active: r.active !== false,
  }
}

function pivotFromConfig(config) {
  const parsed = parseConfig(config)
  return parsed.pivot_table || ''
}

export default function SyncJobForm({
  initial,
  connections = [],
  onSubmit,
  busy,
  submitLabel,
}) {
  const [name, setName] = useState(initial?.name || '')
  const [sourceConnectionId, setSourceConnectionId] = useState(
    initial?.source_connection_id ? String(initial.source_connection_id) : '',
  )
  const [destinationConnectionId, setDestinationConnectionId] = useState(
    initial?.destination_connection_id ? String(initial.destination_connection_id) : '',
  )
  const [sourceTable, setSourceTable] = useState(initial?.source_table || '')
  const [destinationTable, setDestinationTable] = useState(initial?.destination_table || '')
  const [chunkSize, setChunkSize] = useState(initial?.chunk_size ?? 500)
  const [workers, setWorkers] = useState(initial?.workers ?? 2)
  const [config, setConfig] = useState(() => buildInitialConfig(initial?.config))
  const [fields, setFields] = useState(() => (initial?.fields || []).map(mapField))
  const [rules, setRules] = useState(() => (initial?.rules || []).map(mapRule))
  const [relations, setRelations] = useState(() =>
    (initial?.relations || []).map((r) => ({
      id: r.id,
      name: r.name || '',
      type: r.type || 'has_many',
      table: r.table || '',
      pivot_table: pivotFromConfig(r.config),
      foreign_key: r.foreign_key || '',
      related_key: r.related_key || '',
      parent_id: r.parent_id ?? '',
      active: r.active !== false,
      fields: (r.fields || []).map(mapField),
    })),
  )

  const { tables, columnsByTable, ensureTables, ensureColumns } =
    useConnectionSchema(sourceConnectionId)

  const sourceConnections = useMemo(
    () => connections.filter((c) => c.type === 'mysql' || c.type === 'postgres'),
    [connections],
  )
  const destinationConnections = useMemo(
    () => connections.filter((c) => c.type === 'typesense'),
    [connections],
  )

  const destinationType = useMemo(() => {
    const selected = connections.find((c) => String(c.id) === String(destinationConnectionId))
    return selected?.type || ''
  }, [connections, destinationConnectionId])

  const sourceColumns = columnsByTable[sourceTable.trim()] || []

  function handleSubmit(event) {
    event.preventDefault()

    const payloadConfig = {}
    if (destinationType === 'typesense') {
      if (config.default_sorting_field.trim()) {
        payloadConfig.default_sorting_field = config.default_sorting_field.trim()
      }
      payloadConfig.enable_nested_fields = config.enable_nested_fields
      const symbols = fromCsv(config.symbols_to_index)
      const tokens = fromCsv(config.token_separators)
      if (symbols.length) payloadConfig.symbols_to_index = symbols
      if (tokens.length) payloadConfig.token_separators = tokens
    }

    const keptRelations = relations.filter((r) => r.name.trim() && r.table.trim())

    const relationPayload = keptRelations.map((r) => {
      const out = {
        id: r.id,
        name: r.name.trim(),
        type: r.type,
        table: r.table.trim(),
        foreign_key: r.foreign_key?.trim() || undefined,
        related_key: r.related_key?.trim() || undefined,
        active: r.active !== false,
      }
      if (r.parent_id !== '' && r.parent_id != null) {
        out.parent_id = Number(r.parent_id)
      }
      if (r.type === 'belongs_to_many' && r.pivot_table?.trim()) {
        out.config = { pivot_table: r.pivot_table.trim() }
      }
      return out
    })

    const fieldPayload = [
      ...fields
        .filter((f) => f.source_name.trim())
        .map((f) => ({
          id: f.id && f.id > 0 ? f.id : undefined,
          source_name: f.source_name.trim(),
          destination_name: f.destination_name?.trim() || undefined,
          destination_type: f.destination_type || undefined,
          active: f.active !== false,
        })),
      ...keptRelations.flatMap((r) =>
        (r.fields || [])
          .filter((f) => f.source_name.trim())
          .map((f) => ({
            id: f.id && f.id > 0 ? f.id : undefined,
            sync_job_relation_id: r.id,
            source_name: f.source_name.trim(),
            destination_name: f.destination_name?.trim() || undefined,
            destination_type: f.destination_type || undefined,
            active: f.active !== false,
          })),
      ),
    ]

    const rulePayload = rules
      .filter((r) => r.field.trim())
      .map((r) => ({
        id: r.id && r.id > 0 ? r.id : undefined,
        field: r.field.trim(),
        operator: r.operator,
        value: ruleNeedsValue(r.operator) ? r.value ?? '' : '',
        active: r.active !== false,
      }))

    const payload = {
      name: name.trim(),
      source_connection_id: Number(sourceConnectionId),
      source_table: sourceTable.trim(),
      destination_connection_id: Number(destinationConnectionId),
      destination_table: destinationTable.trim(),
      chunk_size: Number(chunkSize) || 500,
      workers: Number(workers) || 2,
      config: payloadConfig,
      fields: fieldPayload,
      rules: rulePayload,
      relations: relationPayload,
    }

    onSubmit(payload)
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)]/70 p-4">
        <div className="mb-4 flex items-center gap-2">
          <span className="flex h-6 w-6 items-center justify-center rounded-md bg-[var(--accent)] font-mono text-[10px] font-bold text-white">
            01
          </span>
          <div>
            <h3 className="text-sm font-semibold text-[var(--text)]">Pipeline</h3>
            <p className="text-xs text-[var(--text-muted)]">Core mapping from source to destination.</p>
          </div>
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="sm:col-span-2">
            <Field label="Name">
              <input
                required
                className={inputClassName}
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </Field>
          </div>
          <Field label="Source connection">
            <select
              required
              className={inputClassName}
              value={sourceConnectionId}
              onChange={(e) => setSourceConnectionId(e.target.value)}
            >
              <option value="">Select source…</option>
              {sourceConnections.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name} ({c.type})
                </option>
              ))}
            </select>
          </Field>
          <Field label="Source table">
            <AutocompleteInput
              required
              options={tables}
              value={sourceTable}
              onFocus={ensureTables}
              onChange={(e) => setSourceTable(e.target.value)}
            />
          </Field>
          <Field label="Destination connection">
            <select
              required
              className={inputClassName}
              value={destinationConnectionId}
              onChange={(e) => setDestinationConnectionId(e.target.value)}
            >
              <option value="">Select destination…</option>
              {destinationConnections.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name} ({c.type})
                </option>
              ))}
            </select>
          </Field>
          <Field label="Destination table">
            <input
              required
              className={inputClassName}
              value={destinationTable}
              onChange={(e) => setDestinationTable(e.target.value)}
            />
          </Field>
          <Field label="Chunk size">
            <input
              type="number"
              min="1"
              className={inputClassName}
              value={chunkSize}
              onChange={(e) => setChunkSize(e.target.value)}
            />
          </Field>
          <Field label="Workers">
            <input
              type="number"
              min="1"
              className={inputClassName}
              value={workers}
              onChange={(e) => setWorkers(e.target.value)}
            />
          </Field>
        </div>
      </div>

      {destinationType === 'typesense' ? (
        <details className="group rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)]/70 open:pb-4">
          <summary className="flex cursor-pointer list-none items-center gap-2 p-4 [&::-webkit-details-marker]:hidden">
            <span className="flex h-6 w-6 items-center justify-center rounded-md bg-[var(--accent)] font-mono text-[10px] font-bold text-white">
              02
            </span>
            <div className="min-w-0 flex-1">
              <h3 className="text-sm font-semibold text-[var(--text)]">Destination config</h3>
              <p className="text-xs text-[var(--text-muted)]">
                Optional Typesense options — expand only if you need them.
              </p>
            </div>
            <span className="font-mono text-[11px] text-[var(--text-muted)] group-open:hidden">
              show
            </span>
            <span className="hidden font-mono text-[11px] text-[var(--text-muted)] group-open:inline">
              hide
            </span>
          </summary>
          <div className="grid gap-4 border-t border-[var(--border)] px-4 pt-4 sm:grid-cols-2">
            <Field label="Default sorting field">
              <AutocompleteInput
                options={sourceColumns}
                value={config.default_sorting_field}
                onFocus={() => ensureColumns(sourceTable)}
                onChange={(e) =>
                  setConfig((prev) => ({ ...prev, default_sorting_field: e.target.value }))
                }
              />
            </Field>
            <Field label="Symbols to index" hint="Comma-separated">
              <input
                className={inputClassName}
                value={config.symbols_to_index}
                onChange={(e) =>
                  setConfig((prev) => ({ ...prev, symbols_to_index: e.target.value }))
                }
              />
            </Field>
            <Field label="Token separators" hint="Comma-separated">
              <input
                className={inputClassName}
                value={config.token_separators}
                onChange={(e) =>
                  setConfig((prev) => ({ ...prev, token_separators: e.target.value }))
                }
              />
            </Field>
            <label className="flex items-center gap-2 pt-7 text-sm">
              <input
                type="checkbox"
                className="h-4 w-4 accent-[var(--accent)]"
                checked={config.enable_nested_fields}
                onChange={(e) =>
                  setConfig((prev) => ({ ...prev, enable_nested_fields: e.target.checked }))
                }
              />
              Enable nested fields
            </label>
          </div>
        </details>
      ) : null}

      <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)]/70 p-4">
        <RuleEditor
          rules={rules}
          onChange={setRules}
          sourceColumns={sourceColumns}
          onNeedSourceColumns={() => ensureColumns(sourceTable)}
        />
      </div>

      <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)]/70 p-4">
        <FieldEditor
          fields={fields}
          onChange={setFields}
          sourceColumns={sourceColumns}
          onNeedSourceColumns={() => ensureColumns(sourceTable)}
        />
      </div>

      <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)]/70 p-4">
        <RelationEditor
          relations={relations}
          onChange={setRelations}
          tables={tables}
          columnsByTable={columnsByTable}
          sourceTable={sourceTable}
          onNeedTables={ensureTables}
          onNeedColumns={ensureColumns}
        />
      </div>

      <div className="flex justify-end border-t border-[var(--border)] pt-5">
        <PrimaryButton type="submit" disabled={busy}>
          {busy ? 'Saving…' : submitLabel}
        </PrimaryButton>
      </div>
    </form>
  )
}
