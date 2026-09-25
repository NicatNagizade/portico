import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  exploreSyncJob,
  exportSyncJobCSV,
  getSyncJob,
  listSyncJobs,
} from '../api/syncJobs'
import {
  EmptyState,
  ErrorBanner,
  Field,
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

const PAGE_SIZE = 50
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
  if (!row) return null

  return (
    <div
      className="animate-fade-in fixed inset-0 z-50 flex items-center justify-center bg-[#12181f]/45 p-4 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Row detail"
        className="animate-dialog-in flex max-h-[min(90vh,720px)] w-full max-w-2xl flex-col overflow-hidden rounded-2xl border border-[var(--border)] bg-[var(--surface)] shadow-[var(--shadow-lg)]"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-3 border-b border-[var(--border)] px-5 py-4">
          <div>
            <h2 className="text-lg font-semibold tracking-tight text-[var(--text)]">Row detail</h2>
            <p className="mt-1 text-xs text-[var(--text-muted)]">
              Full values for every column in this row.
            </p>
          </div>
          <GhostButton type="button" onClick={onClose}>
            Close
          </GhostButton>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
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
        </div>
      </div>
    </div>
  )
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

function SideToggle({ value, onChange, disabled }) {
  return (
    <div
      role="group"
      aria-label="Connection side"
      className="inline-flex w-full rounded-lg border border-[var(--border)] bg-[var(--bg-elevated)] p-1 shadow-[var(--shadow-sm)]"
    >
      {[
        { value: 'source', label: 'Source' },
        { value: 'destination', label: 'Destination' },
      ].map((option) => {
        const active = value === option.value
        return (
          <button
            key={option.value}
            type="button"
            disabled={disabled}
            aria-pressed={active}
            onClick={() => onChange(option.value)}
            className={[
              'flex-1 rounded-md px-3 py-2 text-sm font-medium transition-all',
              active
                ? 'bg-[var(--surface)] text-[var(--text)] shadow-[var(--shadow-sm)]'
                : 'text-[var(--text-muted)] hover:text-[var(--text)]',
              disabled ? 'opacity-50' : '',
            ].join(' ')}
          >
            {option.label}
          </button>
        )
      })}
    </div>
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

export default function ExplorePage() {
  const [jobs, setJobs] = useState([])
  const [jobsLoading, setJobsLoading] = useState(true)
  const [jobId, setJobId] = useState('')
  const [job, setJob] = useState(null)
  const [jobLoading, setJobLoading] = useState(false)
  const [side, setSide] = useState('source')
  const [preview, setPreview] = useState(resetPreviewState)
  const [queryLoading, setQueryLoading] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [error, setError] = useState('')
  const [selectedRow, setSelectedRow] = useState(null)

  useEffect(() => {
    let cancelled = false
    async function loadJobs() {
      setJobsLoading(true)
      setError('')
      try {
        const data = await listSyncJobs({ page: 1, pageSize: JOB_LIST_SIZE })
        if (!cancelled) setJobs(data?.items || [])
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
      setSelectedRow(null)
      return
    }
    let cancelled = false
    async function loadJob() {
      setJobLoading(true)
      setError('')
      setPreview(resetPreviewState())
      setSelectedRow(null)
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

  async function runQuery(nextPage = 1, nextSortBy = preview.sortBy, nextSortDir = preview.sortDir) {
    if (!jobId) return
    setQueryLoading(true)
    setError('')
    try {
      const data = await exploreSyncJob(jobId, {
        side,
        page: nextPage,
        pageSize: PAGE_SIZE,
        sortBy: nextSortBy,
        sortDir: nextSortDir,
      })
      const size = data?.page_size || PAGE_SIZE
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
      await exportSyncJobCSV(jobId, { side })
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
  }

  function toggleSort(col) {
    let nextBy = col
    let nextDir = 'asc'
    if (preview.sortBy === col) {
      if (preview.sortDir === 'asc') nextDir = 'desc'
      else {
        nextBy = ''
        nextDir = 'asc'
      }
    }
    runQuery(1, nextBy, nextDir)
  }

  const { page, rows, columns, sortBy, sortDir, total, totalPages, hasRun } = preview
  const busy = queryLoading || exporting
  const ruleCount = job?.rules?.length ?? 0
  const fieldCount = job?.fields?.length ?? 0
  const relationCount = job?.relations?.length ?? 0

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
          <form
            className="animate-fade-up rounded-[var(--radius)] border border-[var(--border)] bg-[var(--surface)] p-4 shadow-[var(--shadow-sm)]"
            onSubmit={(e) => {
              e.preventDefault()
              runQuery(1)
            }}
          >
            <div className="flex flex-col gap-4 lg:flex-row lg:items-end">
              <div className="min-w-0 flex-1">
                <Field label="Sync job">
                  <select
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
                </Field>
              </div>

              <div className="w-full lg:w-64">
                <span className="mb-1.5 block text-[13px] font-medium text-[var(--text)]">
                  Side
                </span>
                <SideToggle value={side} onChange={changeSide} disabled={!jobId} />
              </div>

              <div className="flex shrink-0 flex-wrap gap-2 lg:pb-0.5">
                <PrimaryButton type="submit" disabled={!jobId || busy}>
                  <PreviewIcon />
                  {queryLoading ? 'Loading…' : 'Preview'}
                </PrimaryButton>
                <SecondaryButton
                  type="button"
                  disabled={!jobId || busy}
                  onClick={handleExport}
                >
                  <DownloadIcon />
                  {exporting ? 'Downloading…' : 'Download CSV'}
                </SecondaryButton>
              </div>
            </div>

            {jobLoading ? (
              <p className="mt-4 border-t border-[var(--border)] pt-4 text-sm text-[var(--text-muted)]">
                Loading job details…
              </p>
            ) : job ? (
              <div className="mt-4 flex flex-wrap items-center gap-2 border-t border-[var(--border)] pt-4">
                <MetaChip>
                  {job.source_table} → {job.destination_table}
                </MetaChip>
                <MetaChip>
                  {ruleCount} rule{ruleCount === 1 ? '' : 's'}
                </MetaChip>
                <MetaChip>
                  {fieldCount} field{fieldCount === 1 ? '' : 's'}
                </MetaChip>
                <MetaChip>
                  {relationCount} relation{relationCount === 1 ? '' : 's'}
                </MetaChip>
                <MetaChip>
                  {side === 'source'
                    ? 'Mapped source rows'
                    : 'Stored destination documents'}
                </MetaChip>
                <Link
                  to={`/sync-jobs/${job.id}`}
                  className="ml-auto inline-flex items-center gap-1.5 text-sm font-medium text-[var(--accent)] hover:underline"
                >
                  Open job
                  <ExternalLinkIcon />
                </Link>
              </div>
            ) : null}
          </form>

          {!jobId ? (
            <EmptyState
              title="Select a sync job"
              message="Pick a job above, choose source or destination, then preview rows."
            />
          ) : !hasRun && !queryLoading ? (
            <EmptyState
              title="Ready to preview"
              message={`Load ${side} rows for “${job?.name || 'this job'}” to inspect the mapped result set.`}
            />
          ) : null}

          {queryLoading && (!hasRun || rows.length === 0) ? <LoadingState rows={4} /> : null}

          {hasRun && !queryLoading && rows.length === 0 ? (
            <EmptyState title="No rows" message="Nothing matched for this job and side." />
          ) : null}

          {hasRun && rows.length > 0 ? (
            <div className="space-y-3">
              <div>
                <h3 className="text-sm font-semibold tracking-tight text-[var(--text)]">
                  Results
                </h3>
                <p className="mt-0.5 text-xs text-[var(--text-muted)]">
                  {total.toLocaleString()} row{total === 1 ? '' : 's'} from {side}
                  {columns.length ? ` · ${columns.length} columns` : ''}
                  {sortBy ? ` · sorted by ${sortBy} ${sortDir}` : ''}
                  {' · '}click a row for full values
                  {queryLoading ? ' · refreshing…' : ''}
                </p>
              </div>

              <TableShell
                footer={
                  <Pagination
                    page={page}
                    totalPages={totalPages}
                    total={total}
                    pageSize={PAGE_SIZE}
                    onPageChange={(next) => runQuery(next)}
                    disabled={queryLoading}
                  />
                }
              >
                <table
                  className="table-fixed border-collapse text-left text-sm"
                  style={{ width: Math.max(columns.length, 1) * 240 }}
                >
                  <colgroup>
                    {columns.map((col) => (
                      <col key={col} style={{ width: 240 }} />
                    ))}
                  </colgroup>
                  <thead>
                    <tr>
                      {columns.map((col) => {
                        const active = sortBy === col
                        return (
                          <Th key={col} className="max-w-[240px]">
                            <button
                              type="button"
                              className="block w-full truncate text-left uppercase hover:text-[var(--text)]"
                              title={`Sort by ${col}`}
                              onClick={() => toggleSort(col)}
                              disabled={queryLoading}
                            >
                              {col}
                              {active ? (sortDir === 'asc' ? ' ↑' : ' ↓') : ''}
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
                        className="cursor-pointer transition-colors hover:bg-[var(--accent-soft)]/40"
                        onClick={() => setSelectedRow(row)}
                      >
                        {columns.map((col) => {
                          const text = cellDisplay(row[col])
                          return (
                            <Td key={col} className="max-w-[240px] overflow-hidden">
                              <span className="block truncate font-mono text-xs" title={text}>
                                {text}
                              </span>
                            </Td>
                          )
                        })}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </TableShell>
            </div>
          ) : null}

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
