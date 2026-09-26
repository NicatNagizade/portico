import { useMemo, useState } from 'react'
import { parseConfig } from '../../lib/connectionTypes'
import { ruleNeedsValue } from '../../lib/ruleOperators'
import { columnNames, fieldsFromSourceColumns } from '../../lib/sourceColumns'
import useConnectionSchema from '../../hooks/useConnectionSchema'
import AutocompleteInput from '../AutocompleteInput'
import { Field, IconButton, PrimaryButton, Toggle, inputClassName } from '../ui'
import FieldEditor from './FieldEditor'
import RelationEditor from './RelationEditor'
import RuleEditor from './RuleEditor'

const DEFAULT_CHUNK_SIZE = 500
const DEFAULT_WORKERS = 2

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

function mapFieldValue(v) {
  return {
    id: v.id,
    source_value: v.source_value || '',
    destination_value: v.destination_value || '',
  }
}

function mapField(f) {
  return {
    id: f.id,
    source_name: f.source_name || '',
    destination_name: f.destination_name || '',
    destination_type: f.destination_type || '',
    active: f.active !== false,
    values: (f.values || []).map(mapFieldValue),
  }
}

function fieldValuesPayload(values = []) {
  return values
    .filter((v) => String(v.source_value ?? '').trim() && String(v.destination_value ?? '').trim())
    .map((v) => ({
      id: v.id && v.id > 0 ? v.id : undefined,
      source_value: String(v.source_value).trim(),
      destination_value: String(v.destination_value).trim(),
    }))
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

function mapRelation(r) {
  return {
    id: r.id,
    name: r.name || '',
    type: r.type || 'has_many',
    table: r.table || '',
    pivot_table: pivotFromConfig(r.config),
    foreign_key: r.foreign_key || '',
    related_key: r.related_key || '',
    active: r.active !== false,
    fields: (r.fields || []).map(mapField),
    relations: (r.relations || []).map(mapRelation),
  }
}

function fieldPayload(f) {
  return {
    id: f.id && f.id > 0 ? f.id : undefined,
    source_name: f.source_name.trim(),
    destination_name: f.destination_name?.trim() || undefined,
    destination_type: f.destination_type || undefined,
    active: f.active !== false,
    values: fieldValuesPayload(f.values),
  }
}

function relationPayload(r) {
  const out = {
    name: r.name.trim(),
    type: r.type,
    table: r.table.trim(),
    foreign_key: r.foreign_key?.trim() || undefined,
    related_key: r.related_key?.trim() || undefined,
    active: r.active !== false,
  }
  // Only real DB ids — UI temp ids are negative and rejected by the API.
  if (r.id && r.id > 0) {
    out.id = r.id
  }
  if (r.type === 'belongs_to_many' && r.pivot_table?.trim()) {
    out.config = { pivot_table: r.pivot_table.trim() }
  }
  const fields = (r.fields || []).filter((f) => f.source_name.trim()).map(fieldPayload)
  if (fields.length) {
    out.fields = fields
  }
  const kids = (r.relations || [])
    .filter((child) => child.name.trim() && child.table.trim())
    .map(relationPayload)
  if (kids.length) {
    out.relations = kids
  }
  return out
}

function keepRelations(relations) {
  return relations
    .filter((r) => r.name.trim() && r.table.trim())
    .map((r) => ({
      ...r,
      relations: keepRelations(r.relations || []),
    }))
}

function ChevronToggleIcon({ open }) {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
      className={`transition-transform ${open ? 'rotate-180' : ''}`}
    >
      <path
        d="M6 9l6 6 6-6"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}

function ButtonPanel({ title, description, open, onToggle, children, className = '' }) {
  return (
    <div
      className={[
        'rounded-lg border border-[var(--border)] bg-[var(--surface)]/60 p-4',
        className,
      ].join(' ')}
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <h4 className="text-sm font-semibold text-[var(--text)]">{title}</h4>
          {description ? (
            <p className="text-xs text-[var(--text-muted)]">{description}</p>
          ) : null}
        </div>
        <IconButton
          label={open ? `Hide ${title}` : `Show ${title}`}
          onClick={() => onToggle(!open)}
        >
          <ChevronToggleIcon open={open} />
        </IconButton>
      </div>
      {open ? <div className="mt-4 border-t border-[var(--border)] pt-4">{children}</div> : null}
    </div>
  )
}

function AdvancedSettings({ open, onToggle, summary, children }) {
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)]/70 p-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-[var(--text)]">Advanced settings</h3>
          <p className="text-xs text-[var(--text-muted)]">
            {open
              ? 'Destination config, filters, field overrides, and relations.'
              : summary || 'Destination config, filters, field overrides, and relations.'}
          </p>
        </div>
        <IconButton
          label={open ? 'Hide advanced settings' : 'Show advanced settings'}
          onClick={() => onToggle(!open)}
        >
          <ChevronToggleIcon open={open} />
        </IconButton>
      </div>
      {open ? (
        <div className="mt-4 space-y-3 border-t border-[var(--border)] pt-4">{children}</div>
      ) : null}
    </div>
  )
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
  const [chunkSize, setChunkSize] = useState(initial?.chunk_size ?? DEFAULT_CHUNK_SIZE)
  const [workers, setWorkers] = useState(initial?.workers ?? DEFAULT_WORKERS)
  const [config, setConfig] = useState(() => buildInitialConfig(initial?.config))
  const [fields, setFields] = useState(() => (initial?.fields || []).map(mapField))
  const [rules, setRules] = useState(() => (initial?.rules || []).map(mapRule))
  const [relations, setRelations] = useState(() =>
    (initial?.relations || []).map(mapRelation),
  )

  const initialConfig = parseConfig(initial?.config)
  const [showAdvanced, setShowAdvanced] = useState(
    () =>
      (initial?.rules || []).length > 0 ||
      (initial?.fields || []).length > 0 ||
      (initial?.relations || []).length > 0 ||
      Boolean(initialConfig.default_sorting_field) ||
      (Array.isArray(initialConfig.symbols_to_index) && initialConfig.symbols_to_index.length > 0) ||
      (Array.isArray(initialConfig.token_separators) && initialConfig.token_separators.length > 0),
  )
  const [showDestConfig, setShowDestConfig] = useState(
    () =>
      Boolean(initialConfig.default_sorting_field) ||
      (Array.isArray(initialConfig.symbols_to_index) && initialConfig.symbols_to_index.length > 0) ||
      (Array.isArray(initialConfig.token_separators) && initialConfig.token_separators.length > 0) ||
      initialConfig.enable_nested_fields === false,
  )
  const [showRules, setShowRules] = useState(() => (initial?.rules || []).length > 0)
  const [showFields, setShowFields] = useState(() => (initial?.fields || []).length > 0)
  const [showRelations, setShowRelations] = useState(() => (initial?.relations || []).length > 0)
  const [showTypesenseAdvanced, setShowTypesenseAdvanced] = useState(() => {
    return Boolean(
      (Array.isArray(initialConfig.symbols_to_index) && initialConfig.symbols_to_index.length) ||
        (typeof initialConfig.symbols_to_index === 'string' &&
          initialConfig.symbols_to_index.trim()) ||
        (Array.isArray(initialConfig.token_separators) &&
          initialConfig.token_separators.length) ||
        (typeof initialConfig.token_separators === 'string' &&
          initialConfig.token_separators.trim()),
    )
  })

  const { tables, columnsByTable, ensureTables, ensureColumns } =
    useConnectionSchema(sourceConnectionId)

  const sourceConnections = connections
  const destinationConnections = connections

  const destinationType = useMemo(() => {
    const selected = connections.find((c) => String(c.id) === String(destinationConnectionId))
    return selected?.type || ''
  }, [connections, destinationConnectionId])

  const sourceColumns = columnsByTable[sourceTable.trim()] || []
  const sourceColumnNames = columnNames(sourceColumns)

  const mappingSummary = useMemo(() => {
    const parts = []
    const ruleCount = rules.filter((r) => r.field.trim()).length
    const fieldCount = fields.filter((f) => f.source_name.trim()).length
    const relationCount = relations.filter((r) => r.name.trim() && r.table.trim()).length
    if (destinationType === 'typesense') parts.push('destination config')
    if (ruleCount) parts.push(`${ruleCount} rule${ruleCount === 1 ? '' : 's'}`)
    if (fieldCount) parts.push(`${fieldCount} field${fieldCount === 1 ? '' : 's'}`)
    if (relationCount) parts.push(`${relationCount} relation${relationCount === 1 ? '' : 's'}`)
    return parts.length ? parts.join(', ') : ''
  }, [rules, fields, relations, destinationType])

  async function autofillFields() {
    const columns = await ensureColumns(sourceTable)
    if (!columns.length) return
    setFields((prev) => fieldsFromSourceColumns(columns, prev))
  }

  function handleSubmit(event) {
    event.preventDefault()

    const payloadConfig = {}
    if (destinationType === 'typesense') {
      if (config.default_sorting_field.trim()) {
        payloadConfig.default_sorting_field = config.default_sorting_field.trim()
      }
      payloadConfig.enable_nested_fields = config.enable_nested_fields
      if (showTypesenseAdvanced) {
        const symbols = fromCsv(config.symbols_to_index)
        const tokens = fromCsv(config.token_separators)
        if (symbols.length) payloadConfig.symbols_to_index = symbols
        if (tokens.length) payloadConfig.token_separators = tokens
      }
    }

    const keptRelations = keepRelations(relations)
    const relationPayloadList = keptRelations.map(relationPayload)

    const fieldPayloadList = fields
      .filter((f) => f.source_name.trim())
      .map(fieldPayload)

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
      chunk_size: Number(chunkSize) || DEFAULT_CHUNK_SIZE,
      workers: Number(workers) || DEFAULT_WORKERS,
      config: payloadConfig,
      fields: fieldPayloadList,
      rules: rulePayload,
      relations: relationPayloadList,
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

      <AdvancedSettings
        open={showAdvanced}
        onToggle={setShowAdvanced}
        summary={mappingSummary}
      >
        {destinationType === 'typesense' ? (
          <ButtonPanel
            title="Destination config"
            description="Optional Typesense options."
            open={showDestConfig}
            onToggle={setShowDestConfig}
          >
            <div className="space-y-4">
              <Field
                label="Default sorting field"
                hint="int32/float/int64 — id uses id_int"
              >
                <AutocompleteInput
                  options={sourceColumnNames}
                  value={config.default_sorting_field}
                  onFocus={() => ensureColumns(sourceTable)}
                  onChange={(e) =>
                    setConfig((prev) => ({ ...prev, default_sorting_field: e.target.value }))
                  }
                />
              </Field>
              <Toggle
                checked={config.enable_nested_fields}
                onChange={(on) => setConfig((prev) => ({ ...prev, enable_nested_fields: on }))}
                label="Enable nested fields"
                description="Keep nested objects and arrays in the Typesense schema."
              />
              <Toggle
                checked={showTypesenseAdvanced}
                onChange={(on) => {
                  setShowTypesenseAdvanced(on)
                  if (!on) {
                    setConfig((prev) => ({
                      ...prev,
                      symbols_to_index: '',
                      token_separators: '',
                    }))
                  }
                }}
                label="Custom indexing symbols"
                description="Override symbols to index and token separators."
              />
              {showTypesenseAdvanced ? (
                <div className="grid gap-4 sm:grid-cols-2">
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
                </div>
              ) : null}
            </div>
          </ButtonPanel>
        ) : null}

        <ButtonPanel
          title="Filter rules"
          description="Only import rows that match all active rules."
          open={showRules}
          onToggle={setShowRules}
        >
          <RuleEditor
            rules={rules}
            onChange={setRules}
            sourceColumns={sourceColumnNames}
            onNeedSourceColumns={() => ensureColumns(sourceTable)}
            hideHeader
          />
        </ButtonPanel>

        <ButtonPanel
          title="Field overrides"
          description="Rename columns, set types, map values, or exclude fields."
          open={showFields}
          onToggle={setShowFields}
        >
          <FieldEditor
            fields={fields}
            onChange={setFields}
            sourceColumns={sourceColumns}
            onNeedSourceColumns={() => ensureColumns(sourceTable)}
            onAutofill={sourceTable.trim() ? autofillFields : undefined}
            hideHeader
          />
        </ButtonPanel>

        <ButtonPanel
          title="Relations"
          description="Nest related rows on the destination document."
          open={showRelations}
          onToggle={setShowRelations}
        >
          <RelationEditor
            relations={relations}
            onChange={setRelations}
            tables={tables}
            columnsByTable={columnsByTable}
            sourceTable={sourceTable}
            onNeedTables={ensureTables}
            onNeedColumns={ensureColumns}
            hideHeader
          />
        </ButtonPanel>
      </AdvancedSettings>

      <div className="flex justify-end border-t border-[var(--border)] pt-5">
        <PrimaryButton type="submit" disabled={busy}>
          {busy ? 'Saving…' : submitLabel}
        </PrimaryButton>
      </div>
    </form>
  )
}
