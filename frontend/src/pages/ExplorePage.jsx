import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  exploreSyncJob,
  exportSyncJobCSV,
  getSyncJob,
  listSyncJobs,
} from '../api/syncJobs'
import { getHealth } from '../api/health'
import ExploreAISuggest from '../components/forms/ExploreAISuggest'
import ExploreFields from '../components/forms/ExploreFields'
import ExploreFilters from '../components/forms/ExploreFilters'
import {
  Dialog,
  EmptyState,
  ErrorBanner,
  GhostButton,
  LoadingState,
  MetaChip,
  PageHeader,
  Pagination,
  PrimaryButton,
  SecondaryButton,
  TableShell,
  Td,
  Th,
  inputClassName,
} from '../components/ui'
import useConnectionSchema from '../hooks/useConnectionSchema'
import { ruleNeedsValue, ruleOperatorLabel } from '../lib/ruleOperators'
import { flattenColumnNames, selectableColumnNames } from '../lib/sourceColumns'

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100]
const DEFAULT_PAGE_SIZE = 50
const JOB_LIST_SIZE = 100

function cellDisplay(value, pretty = false) {
  if (value === null || value === undefined) return ''
  if (typeof value === 'object') {
    try {
      return JSON.stringify(value, null, pretty ? 2 : 0)
    } catch {
      return String(value)
    }
  }
  return String(value)
}

function RowDetailDialog({ row, columns, onClose }) {
  return (
    <Dialog
      open={!!row}
      title="Row detail"
      description="Full values for every column in this row."
      onClose={onClose}
      size="lg"
    >
      {row ? (
        <dl className="space-y-4">
          {columns.map((col) => {
            const text = cellDisplay(row[col], true)
            return (
              <div key={col}>
                <dt className="font-mono text-[11px] font-semibold tracking-[0.12em] text-[var(--text-muted)] uppercase">
                  {col}
                </dt>
                <dd className="mt-1.5">
                  {text === '' ? (
                    <span className="text-sm text-[var(--text-muted)] italic">empty</span>
                  ) : (
                    <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-words rounded-xl bg-[var(--bg-elevated)] p-3 font-mono text-xs leading-relaxed text-[var(--text)]">
                      {text}
                    </pre>
                  )}
                </dd>
              </div>
            )
          })}
        </dl>
      ) : null}
    </Dialog>
  )
}

function visibleColumns(allColumns, selectedFields) {
  if (selectedFields === null) return allColumns
  const set = new Set(selectedFields)
  return allColumns.filter((col) => set.has(col))
}

function filterChipLabel(f) {
  const op = ruleOperatorLabel(f.operator)
  if (!ruleNeedsValue(f.operator)) return `${f.field} ${op}`
  return `${f.field} ${op} ${f.value}`
}

function PreviewIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="11" cy="11" r="6.25" stroke="currentColor" strokeWidth="1.75" />
      <path d="M16 16l4 4" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" />
    </svg>
  )
}

function DownloadIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M12 4v10m0 0 4-4m-4 4-4-4M5 18h14"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}

function ExternalLinkIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M14 4h6v6M20 4l-9 9M10 5H6a2 2 0 0 0-2 2v11a2 2 0 0 0 2 2h11a2 2 0 0 0 2-2v-4"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}

function FilterIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M4 6h16M7 12h10M10 18h4"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
      />
    </svg>
  )
}

function ColumnsIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M5 5h4v14H5V5Zm10 0h4v14h-4V5ZM11 5h2v14h-2V5Z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
    </svg>
  )
}

function SparkleIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M12 3l1.2 4.8L18 9l-4.8 1.2L12 15l-1.2-4.8L6 9l4.8-1.2L12 3Zm7 10 .7 2.3L22 16l-2.3.7L19 19l-.7-2.3L16 16l2.3-.7L19 13ZM5 14l.6 1.9L7.5 16.5 5.6 17 5 19l-.6-2L2.5 16.5 4.4 15.9 5 14Z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
    </svg>
  )
}

function CloseChipIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M6 6l12 12M18 6L6 18" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </svg>
  )
}

/** Source / destination picker; each card shows what will be browsed. */
function SidePicker({ job, side, onChange }) {
  const ruleCount = job.rules?.length ?? 0
  const relationCount = job.relations?.length ?? 0
  const options = [
    {
      value: 'source',
      label: 'Source',
      connection: job.source_connection?.name || `Connection #${job.source_connection_id}`,
      table: job.source_table,
      hint: `Rows with field mappings applied · ${ruleCount} rule${ruleCount === 1 ? '' : 's'} · ${relationCount} relation${relationCount === 1 ? '' : 's'}`,
    },
    {
      value: 'destination',
      label: 'Destination',
      connection:
        job.destination_connection?.name || `Connection #${job.destination_connection_id}`,
      table: job.destination_table,
      hint: 'Documents exactly as stored after the last sync',
    },
  ]

  return (
    <div
      role="radiogroup"
      aria-label="Data to browse"
      className="grid gap-2 sm:grid-cols-[1fr_auto_1fr] sm:items-stretch"
    >
      {options.map((option, index) => {
        const active = side === option.value
        return (
          <div key={option.value} className="contents">
            {index ? (
              <span
                className="hidden items-center text-[var(--text-muted)] sm:flex"
                aria-hidden="true"
              >
                →
              </span>
            ) : null}
            <button
              type="button"
              role="radio"
              aria-checked={active}
              onClick={() => onChange(option.value)}
              className={[
                'flex min-w-0 items-start gap-3 rounded-xl border p-3 text-left transition-all',
                active
                  ? 'border-[var(--accent)] bg-[var(--accent-soft)]/60 shadow-[0_0_0_3px_var(--accent-soft)]'
                  : 'border-[var(--border)] bg-[var(--surface)] hover:border-[var(--border-strong)] hover:bg-[var(--bg-elevated)]',
              ].join(' ')}
            >
              <span
                className={[
                  'mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full border-2',
                  active ? 'border-[var(--accent)]' : 'border-[var(--border-strong)]',
                ].join(' ')}
                aria-hidden="true"
              >
                {active ? <span className="h-1.5 w-1.5 rounded-full bg-[var(--accent)]" /> : null}
              </span>
              <span className="min-w-0 flex-1">
                <span className="flex flex-wrap items-baseline gap-x-2">
                  <span className="text-sm font-semibold text-[var(--text)]">{option.label}</span>
                  <span className="truncate font-mono text-[11px] text-[var(--text-muted)]">
                    {option.connection} / {option.table}
                  </span>
                </span>
                <span className="mt-1 block text-xs text-[var(--text-muted)]">{option.hint}</span>
              </span>
            </button>
          </div>
        )
      })}
    </div>
  )
}

function TableCellValue({ value }) {
  if (value === null || value === undefined || value === '') {
    return <span className="text-[var(--text-muted)]/70">—</span>
  }
  if (Array.isArray(value)) {
    return (
      <span className="inline-flex rounded-md bg-[var(--accent-soft)] px-2 py-1 font-mono text-[11px] text-[var(--accent-ink)]">
        [{value.length} {value.length === 1 ? 'item' : 'items'}]
      </span>
    )
  }
  if (typeof value === 'object') {
    const count = Object.keys(value).length
    return (
      <span className="inline-flex rounded-md bg-[var(--bg-elevated)] px-2 py-1 font-mono text-[11px] text-[var(--text-muted)]">
        {'{'}
        {count} {count === 1 ? 'field' : 'fields'}
        {'}'}
      </span>
    )
  }
  const text = String(value)
  return (
    <span className="block truncate font-mono text-xs" title={text}>
      {text}
    </span>
  )
}

/** Quiet control that opens a dialog; accent when something is configured. */
function RefineButton({ active, count, icon, label, detail, onClick, disabled }) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className={[
        'inline-flex items-center gap-2 rounded-lg border px-3 py-2 text-sm font-medium transition-all active:scale-[0.98] disabled:pointer-events-none disabled:opacity-50',
        active
          ? 'border-[var(--accent)]/35 bg-[var(--accent-soft)] text-[var(--accent-ink)] shadow-[var(--shadow-sm)]'
          : 'border-[var(--border)] bg-[var(--surface)] text-[var(--text)] shadow-[var(--shadow-sm)] hover:border-[var(--border-strong)] hover:bg-[var(--bg-elevated)]',
      ].join(' ')}
    >
      <span className={active ? 'text-[var(--accent)]' : 'text-[var(--text-muted)]'}>{icon}</span>
      <span>{label}</span>
      {detail ? (
        <span
          className={[
            'rounded-md px-1.5 py-0.5 font-mono text-[11px]',
            active
              ? 'bg-[var(--surface)]/80 text-[var(--accent-ink)]'
              : 'bg-[var(--bg-elevated)] text-[var(--text-muted)]',
          ].join(' ')}
        >
          {detail}
        </span>
      ) : count ? (
        <span className="rounded-md bg-[var(--accent)] px-1.5 py-0.5 font-mono text-[11px] font-semibold text-white">
          {count}
        </span>
      ) : null}
    </button>
  )
}

function resetPreviewState() {
  return {
    page: 1,
    rows: [],
    columns: [],
    sortBy: '',
    sortDir: 'asc',
    total: 0,
    totalPages: 0,
    hasRun: false,
  }
}

function columnsFromRows(rows, preferred = []) {
  const seen = new Set()
  const out = []
  for (const col of preferred) {
    if (!col || seen.has(col)) continue
    seen.add(col)
    out.push(col)
  }
  for (const row of rows) {
    for (const key of Object.keys(row || {})) {
      if (seen.has(key)) continue
      seen.add(key)
      out.push(key)
    }
  }
  return out
}

/** Job field overrides + top-level relation names (whole nested docs, not nested paths). */
function jobColumnOptions(job) {
  const { out, add } = uniqueNames()
  addJobFields(job, add)
  for (const r of job?.relations || []) {
    if (r.active === false) continue
    add(r.name)
  }
  return out
}

/** Filter autocomplete: relation names plus nested relation table columns (posts.title). */
function jobFilterFieldOptions(job, columnsByTable = {}) {
  const { out, add } = uniqueNames()
  addJobFields(job, add)
  function addRelation(rel, prefix) {
    const name = rel.name?.trim()
    if (!name) return
    const path = prefix ? `${prefix}.${name}` : name
    add(path)
    for (const f of rel.fields || []) {
      if (f.active === false) continue
      add(`${path}.${f.destination_name || f.source_name}`)
      add(`${path}.${f.source_name}`)
    }
    const table = rel.table?.trim()
    for (const col of columnsByTable[table] || []) {
      const colName = typeof col === 'string' ? col : col?.name
      if (colName) add(`${path}.${colName}`)
    }
    for (const child of rel.relations || []) {
      if (child.active === false) continue
      addRelation(child, path)
    }
  }
  for (const r of job?.relations || []) {
    if (r.active === false) continue
    addRelation(r, '')
  }
  return out
}

function uniqueNames() {
  const seen = new Set()
  const out = []
  return {
    out,
    add(name) {
      const n = typeof name === 'string' ? name.trim() : name
      if (!n || seen.has(n)) return
      seen.add(n)
      out.push(n)
    },
  }
}

function addJobFields(job, add) {
  for (const f of job?.fields || []) {
    if (f.active === false) continue
    add(f.destination_name || f.source_name)
    add(f.source_name)
  }
}

function relationTables(relations, out = []) {
  for (const r of relations || []) {
    if (r.active === false) continue
    const table = r.table?.trim()
    if (table && !out.includes(table)) out.push(table)
    relationTables(r.relations, out)
  }
  return out
}

function prefetchSchemaColumns(ensureColumns, schemaTable, side, job) {
  if (schemaTable) ensureColumns(schemaTable)
  if (side === 'source' && job) {
    for (const table of relationTables(job.relations)) {
      ensureColumns(table)
    }
  }
}

function isActiveExploreFilter(f) {
  if (!f.field?.trim()) return false
  if (ruleNeedsValue(f.operator) && String(f.value ?? '').trim() === '') return false
  return true
}

/** `like` without `%` matches anywhere in the value. A value that already has `%` is a SQL pattern. */
function likeValue(value) {
  const text = String(value ?? '')
  return text.includes('%') ? text : `%${text}%`
}

function activeExploreFilters(filters) {
  return filters.filter(isActiveExploreFilter).map((f) => ({
    field: f.field.trim(),
    operator: f.operator,
    value: !ruleNeedsValue(f.operator)
      ? ''
      : f.operator === 'like'
        ? likeValue(f.value)
        : String(f.value ?? ''),
  }))
}

/** Indexes in `filters` that are currently active (valid field + value when needed). */
function activeFilterIndexes(filters) {
  const indexes = []
  filters.forEach((f, i) => {
    if (isActiveExploreFilter(f)) indexes.push(i)
  })
  return indexes
}

function mergeUnique(lists, { skipDotted = false } = {}) {
  const { out, add } = uniqueNames()
  for (const list of lists) {
    for (const name of list || []) {
      if (skipDotted && String(name).includes('.')) continue
      add(name)
    }
  }
  return out
}

export default function ExplorePage() {
  const [jobs, setJobs] = useState([])
  const [jobsLoading, setJobsLoading] = useState(true)
  const [jobId, setJobId] = useState('')
  const [job, setJob] = useState(null)
  const [jobLoading, setJobLoading] = useState(false)
  const [side, setSide] = useState('source')
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
  const [filters, setFilters] = useState([])
  const [selectedFields, setSelectedFields] = useState(null) // null = show all
  const [filtersOpen, setFiltersOpen] = useState(false)
  const [fieldsOpen, setFieldsOpen] = useState(false)
  const [aiOpen, setAiOpen] = useState(false)
  const [aiEnabled, setAiEnabled] = useState(false)
  const [preview, setPreview] = useState(resetPreviewState)
  const [queryLoading, setQueryLoading] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [error, setError] = useState('')
  const [selectedRow, setSelectedRow] = useState(null)

  const schemaConnectionId =
    side === 'source' ? job?.source_connection_id : job?.destination_connection_id
  const schemaTable = side === 'source' ? job?.source_table : job?.destination_table
  const { columnsByTable, ensureColumns } = useConnectionSchema(schemaConnectionId)
  const schemaColumns = columnsByTable[schemaTable?.trim()] || []
  // Filters: nested dotted paths (e.g. posts.title). Fields picker: top-level + whole relations only.
  const filterFieldOptions = mergeUnique([
    preview.columns,
    jobFilterFieldOptions(job, columnsByTable),
    flattenColumnNames(schemaColumns),
  ])
  const selectableFields = mergeUnique(
    [preview.columns, jobColumnOptions(job), selectableColumnNames(schemaColumns)],
    { skipDotted: true },
  )

  function clearExploreControls() {
    setSelectedRow(null)
    setFilters([])
    setSelectedFields(null)
    setFiltersOpen(false)
    setFieldsOpen(false)
    setAiOpen(false)
  }

  useEffect(() => {
    let cancelled = false
    async function loadJobs() {
      setJobsLoading(true)
      setError('')
      try {
        const [data, health] = await Promise.all([
          listSyncJobs({ page: 1, pageSize: JOB_LIST_SIZE }),
          getHealth().catch(() => null),
        ])
        if (!cancelled) {
          setJobs(data?.items || [])
          setAiEnabled(!!health?.ai_enabled)
        }
      } catch (err) {
        if (!cancelled) setError(err.message || 'Failed to load sync jobs')
      } finally {
        if (!cancelled) setJobsLoading(false)
      }
    }
    loadJobs()
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (!jobId) {
      setJob(null)
      clearExploreControls()
      return
    }
    let cancelled = false
    async function loadJob() {
      setJobLoading(true)
      setError('')
      setPreview(resetPreviewState())
      clearExploreControls()
      try {
        const data = await getSyncJob(jobId)
        if (!cancelled) setJob(data)
      } catch (err) {
        if (!cancelled) {
          setJob(null)
          setError(err.message || 'Failed to load sync job')
        }
      } finally {
        if (!cancelled) setJobLoading(false)
      }
    }
    loadJob()
    return () => {
      cancelled = true
    }
  }, [jobId])

  // Prefetch side-table schema (+ related tables on source) for filter/field autocomplete.
  useEffect(() => {
    if (!job) return
    prefetchSchemaColumns(ensureColumns, schemaTable, side, job)
  }, [job, schemaTable, side, schemaConnectionId])

  function prefetchFilterFields() {
    prefetchSchemaColumns(ensureColumns, schemaTable, side, job)
  }

  async function runQuery(
    nextPage = 1,
    nextSortBy = preview.sortBy,
    nextSortDir = preview.sortDir,
    nextPageSize = pageSize,
  ) {
    if (!jobId) return
    setQueryLoading(true)
    setError('')
    try {
      const data = await exploreSyncJob(jobId, {
        side,
        page: nextPage,
        pageSize: nextPageSize,
        sortBy: nextSortBy,
        sortDir: nextSortDir,
        filters: activeExploreFilters(filters),
      })
      const size = data?.page_size || nextPageSize
      const rows = data?.rows || []
      const preferred = Array.isArray(data?.columns) ? data.columns : []
      // Keep prior column order on re-query (sort/page) so headers don't jump.
      const sticky = preview.hasRun ? preview.columns : []
      setPreview({
        page: data?.page ?? nextPage,
        rows,
        columns: columnsFromRows(rows, [...sticky, ...preferred]),
        sortBy: data?.sort_by || nextSortBy || '',
        sortDir: data?.sort_dir || nextSortDir || 'asc',
        total: data?.total ?? 0,
        totalPages: Math.ceil((data?.total ?? 0) / size) || 0,
        hasRun: true,
      })
    } catch (err) {
      setError(err.message || 'Failed to explore data')
    } finally {
      setQueryLoading(false)
    }
  }

  async function handleExport() {
    if (!jobId) return
    setExporting(true)
    setError('')
    try {
      await exportSyncJobCSV(jobId, {
        side,
        filters: activeExploreFilters(filters),
        fields: selectedFields,
      })
    } catch (err) {
      setError(err.message || 'Failed to download CSV')
    } finally {
      setExporting(false)
    }
  }

  function changeSide(next) {
    if (next === side) return
    setSide(next)
    setPreview(resetPreviewState())
    setSelectedRow(null)
    setSelectedFields(null)
  }

  function changePageSize(next) {
    setPageSize(next)
    if (preview.hasRun) runQuery(1, preview.sortBy, preview.sortDir, next)
  }

  function toggleSort(col) {
    let nextBy = col
    let nextDir = 'asc'
    if (preview.sortBy === col) {
      if (preview.sortDir === 'asc') nextDir = 'desc'
      else {
        // Clear client sort — API defaults back to primary key ascending.
        nextBy = ''
        nextDir = 'asc'
      }
    }
    runQuery(1, nextBy, nextDir)
  }

  function openFilters() {
    prefetchFilterFields()
    setFiltersOpen(true)
  }

  function openFields() {
    if (schemaTable) ensureColumns(schemaTable)
    setFieldsOpen(true)
  }

  function openAI() {
    prefetchFilterFields()
    if (schemaTable) ensureColumns(schemaTable)
    setAiOpen(true)
  }

  function applyAISuggest({ filters: nextFilters, fields: nextFields }) {
    setFilters(
      (nextFilters || []).map((f) => ({
        field: f.field || '',
        operator: f.operator || 'eq',
        value: f.value ?? '',
      })),
    )
    setSelectedFields(nextFields === undefined ? null : nextFields)
  }

  function removeFilterAt(index) {
    setFilters((prev) => prev.filter((_, i) => i !== index))
  }

  function clearFilters() {
    setFilters([])
  }

  function resetFields() {
    setSelectedFields(null)
  }

  const { page, rows, columns, sortBy, sortDir, total, totalPages, hasRun } = preview
  const displayColumns = visibleColumns(columns, selectedFields)
  const busy = queryLoading || exporting
  const filterIndexes = activeFilterIndexes(filters)
  const filterCount = filterIndexes.length
  const fieldsCustom = selectedFields !== null
  const columnTotal = columns.length || selectableFields.length
  const visibleFieldCount =
    selectedFields === null ? columnTotal : selectedFields.length
  const fieldsDetail = fieldsCustom
    ? `${visibleFieldCount}/${columnTotal || visibleFieldCount}`
    : columnTotal
      ? `all ${columnTotal}`
      : 'all'

  return (
    <div>
      <PageHeader
        eyebrow="Inspect"
        title="Explore"
        description="Preview mapped source rows or stored destination documents for a sync job, then download CSV."
      />

      <ErrorBanner message={error} />

      {jobsLoading ? (
        <LoadingState rows={3} />
      ) : jobs.length === 0 ? (
        <EmptyState
          title="No sync jobs yet"
          message="Create a sync job first, then come back to explore its source or destination data."
          action={
            <Link to="/sync-jobs/new">
              <PrimaryButton>Create sync job</PrimaryButton>
            </Link>
          }
        />
      ) : (
        <div className="space-y-5">
          <div className="animate-fade-up overflow-hidden rounded-[var(--radius)] border border-[var(--border)] bg-[var(--surface)] shadow-[var(--shadow-md)]">
            <div className="space-y-4 p-4 sm:p-5">
              <div>
                <div className="mb-1.5 flex items-center justify-between gap-3">
                  <label htmlFor="explore-job" className="text-[13px] font-medium text-[var(--text)]">
                    Sync job
                  </label>
                  {job ? (
                    <Link
                      to={`/sync-jobs/${job.id}`}
                      className="inline-flex items-center gap-1 text-xs font-medium text-[var(--accent)] hover:underline"
                    >
                      Open job
                      <ExternalLinkIcon />
                    </Link>
                  ) : null}
                </div>
                <select
                  id="explore-job"
                  className={inputClassName}
                  value={jobId}
                  onChange={(e) => setJobId(e.target.value)}
                >
                  <option value="">Select a sync job…</option>
                  {jobs.map((j) => (
                    <option key={j.id} value={String(j.id)}>
                      {j.name}
                    </option>
                  ))}
                </select>
              </div>

              {jobLoading ? (
                <p className="text-sm text-[var(--text-muted)]">Loading job details…</p>
              ) : job ? (
                <div>
                  <p className="mb-1.5 text-[13px] font-medium text-[var(--text)]">Browse</p>
                  <SidePicker job={job} side={side} onChange={changeSide} />
                </div>
              ) : null}
            </div>

            {job ? (
              <div className="border-t border-[var(--border)] bg-[var(--bg-elevated)]/60 px-4 py-3 sm:px-5">
                <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                  <div className="flex flex-wrap gap-2">
                    <RefineButton
                      icon={<FilterIcon />}
                      label="Filters"
                      active={filterCount > 0}
                      count={filterCount || undefined}
                      onClick={openFilters}
                    />
                    <RefineButton
                      icon={<ColumnsIcon />}
                      label="Fields"
                      active={fieldsCustom}
                      detail={fieldsDetail}
                      onClick={openFields}
                    />
                    {aiEnabled ? (
                      <RefineButton icon={<SparkleIcon />} label="Ask AI" onClick={openAI} />
                    ) : null}
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <SecondaryButton type="button" disabled={busy} onClick={handleExport}>
                      <DownloadIcon />
                      {exporting ? 'Downloading…' : 'Export CSV'}
                    </SecondaryButton>
                    <PrimaryButton type="button" disabled={busy} onClick={() => runQuery(1)}>
                      <PreviewIcon />
                      {queryLoading ? 'Loading…' : hasRun ? 'Refresh' : 'Preview'}
                    </PrimaryButton>
                  </div>
                </div>

                {filterCount > 0 || fieldsCustom ? (
                  <div className="mt-3 flex flex-wrap items-center gap-2 border-t border-[var(--border)] pt-3">
                    {filterIndexes.map((index) => {
                      const f = filters[index]
                      const label = filterChipLabel({
                        field: f.field.trim(),
                        operator: f.operator,
                        value: String(f.value ?? ''),
                      })
                      return (
                        <span
                          key={`${index}-${label}`}
                          className="inline-flex max-w-full items-center gap-1.5 rounded-md border border-[var(--accent)]/25 bg-[var(--accent-soft)] px-2 py-1 font-mono text-[11px] text-[var(--accent-ink)]"
                        >
                          <button
                            type="button"
                            className="min-w-0 truncate hover:underline"
                            title="Edit filters"
                            onClick={openFilters}
                          >
                            {label}
                          </button>
                          <button
                            type="button"
                            className="shrink-0 rounded p-0.5 text-[var(--accent)] hover:bg-[var(--surface)]"
                            aria-label={`Remove filter ${label}`}
                            onClick={() => removeFilterAt(index)}
                          >
                            <CloseChipIcon />
                          </button>
                        </span>
                      )
                    })}
                    {fieldsCustom ? (
                      <span className="inline-flex items-center gap-1.5 rounded-md border border-[var(--border)] bg-[var(--surface)] px-2 py-1 font-mono text-[11px] text-[var(--text-muted)]">
                        <button
                          type="button"
                          className="hover:text-[var(--text)] hover:underline"
                          onClick={openFields}
                        >
                          {visibleFieldCount} column{visibleFieldCount === 1 ? '' : 's'}
                        </button>
                        <button
                          type="button"
                          className="shrink-0 rounded p-0.5 hover:bg-[var(--bg-elevated)] hover:text-[var(--text)]"
                          aria-label="Show all fields"
                          onClick={resetFields}
                        >
                          <CloseChipIcon />
                        </button>
                      </span>
                    ) : null}
                    {filterCount > 0 ? (
                      <GhostButton type="button" className="ml-auto text-xs" onClick={clearFilters}>
                        Clear filters
                      </GhostButton>
                    ) : null}
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>

          {!jobId ? (
            <EmptyState
              title="Select a sync job"
              message="Pick a job above, choose source or destination, then preview rows."
            />
          ) : !hasRun && !queryLoading ? (
            <EmptyState
              title="Ready to preview"
              message={`Press Preview to load ${side === 'source' ? 'source rows' : 'destination documents'}. Add filters or pick fields first if you only need part of the data.`}
            />
          ) : null}

          {queryLoading && (!hasRun || rows.length === 0) ? <LoadingState rows={4} /> : null}

          {hasRun && !queryLoading && rows.length === 0 ? (
            <EmptyState title="No rows" message="Nothing matched for this job and side." />
          ) : null}

          {hasRun && rows.length > 0 ? (
            <div className="space-y-3">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
                <div>
                  <p className="font-mono text-[10px] font-semibold tracking-[0.16em] text-[var(--text-muted)] uppercase">
                    Preview
                  </p>
                  <h3 className="mt-1 text-lg font-semibold tracking-tight text-[var(--text)]">
                    {side === 'source' ? 'Source rows' : 'Destination documents'}
                  </h3>
                  <p className="mt-1 text-xs text-[var(--text-muted)]">
                    Click any row to inspect every value
                    {queryLoading ? ' · refreshing…' : ''}
                  </p>
                </div>
                <div className="flex flex-wrap gap-2">
                  <MetaChip>
                    {total.toLocaleString()} row{total === 1 ? '' : 's'}
                  </MetaChip>
                  <MetaChip>
                    {displayColumns.length} column{displayColumns.length === 1 ? '' : 's'}
                    {fieldsCustom && columns.length !== displayColumns.length
                      ? ` of ${columns.length}`
                      : ''}
                  </MetaChip>
                  {sortBy ? <MetaChip>Sorted by {sortBy} {sortDir}</MetaChip> : null}
                </div>
              </div>

              {displayColumns.length === 0 ? (
                <EmptyState
                  title="No fields selected"
                  message="Choose at least one column to show in the table and CSV."
                  action={
                    <SecondaryButton type="button" onClick={openFields}>
                      Choose fields
                    </SecondaryButton>
                  }
                />
              ) : (
                <TableShell
                  footer={
                    <Pagination
                      page={page}
                      totalPages={totalPages}
                      total={total}
                      pageSize={pageSize}
                      pageSizeOptions={PAGE_SIZE_OPTIONS}
                      onPageChange={(next) => runQuery(next)}
                      onPageSizeChange={changePageSize}
                      disabled={queryLoading}
                    />
                  }
                >
                  <table
                    className="w-full table-fixed border-collapse text-left text-sm"
                    style={{ minWidth: Math.max(displayColumns.length, 1) * 160 }}
                  >
                    <thead>
                      <tr>
                        {displayColumns.map((col) => {
                          const active = sortBy === col
                          return (
                            <Th key={col} className="sticky top-0 z-10">
                              <button
                                type="button"
                                className="group flex w-full items-center justify-between gap-2 text-left uppercase hover:text-[var(--text)]"
                                title={`Sort by ${col}`}
                                onClick={() => toggleSort(col)}
                                disabled={queryLoading}
                              >
                                <span className="truncate">{col}</span>
                                <span
                                  className={[
                                    'shrink-0 text-sm tracking-normal',
                                    active
                                      ? 'text-[var(--accent)]'
                                      : 'text-[var(--border-strong)] opacity-0 transition-opacity group-hover:opacity-100',
                                  ].join(' ')}
                                >
                                  {active ? (sortDir === 'asc' ? '↑' : '↓') : '↕'}
                                </span>
                              </button>
                            </Th>
                          )
                        })}
                      </tr>
                    </thead>
                    <tbody>
                      {rows.map((row, idx) => (
                        <tr
                          key={row.id != null ? String(row.id) : idx}
                          className="cursor-pointer transition-colors odd:bg-[var(--surface)] even:bg-[var(--bg-elevated)]/35 hover:bg-[var(--accent-soft)]/55"
                          onClick={() => setSelectedRow(row)}
                        >
                          {displayColumns.map((col) => (
                            <Td key={col} className="overflow-hidden">
                              <TableCellValue value={row[col]} />
                            </Td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </TableShell>
              )}
            </div>
          ) : null}

          <Dialog
            open={filtersOpen}
            title="Filters"
            description="Narrow the preview query. All filters are AND’d. Like matches anywhere unless you add % yourself. Job rules still apply on source."
            onClose={() => setFiltersOpen(false)}
            size="lg"
            footer={
              <>
                {hasRun ? (
                  <SecondaryButton
                    type="button"
                    disabled={busy}
                    onClick={() => {
                      setFiltersOpen(false)
                      runQuery(1)
                    }}
                  >
                    Apply & preview
                  </SecondaryButton>
                ) : null}
                <PrimaryButton type="button" onClick={() => setFiltersOpen(false)}>
                  Done
                </PrimaryButton>
              </>
            }
          >
            <ExploreFilters
              filters={filters}
              onChange={setFilters}
              fieldOptions={filterFieldOptions}
              onNeedFields={prefetchFilterFields}
            />
          </Dialog>

          <Dialog
            open={fieldsOpen}
            title="Fields"
            description="Choose columns for the results table and CSV download. Row detail still shows every value."
            onClose={() => setFieldsOpen(false)}
            footer={
              <PrimaryButton type="button" onClick={() => setFieldsOpen(false)}>
                Done
              </PrimaryButton>
            }
          >
            <ExploreFields
              options={selectableFields}
              selected={selectedFields}
              onChange={setSelectedFields}
            />
          </Dialog>

          <ExploreAISuggest
            open={aiEnabled && aiOpen}
            onClose={() => setAiOpen(false)}
            jobId={jobId}
            filterFields={filterFieldOptions}
            fields={selectableFields}
            onApply={applyAISuggest}
          />

          <RowDetailDialog
            row={selectedRow}
            columns={columns}
            onClose={() => setSelectedRow(null)}
          />
        </div>
      )}
    </div>
  )
}
