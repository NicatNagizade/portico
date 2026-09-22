import { download, request } from './client'
import { buildQuery, normalizePage } from './pagination'

export function listSyncJobs({ page = 1, pageSize = 20 } = {}) {
  return request(`/sync-jobs${buildQuery({ page, page_size: pageSize })}`).then(normalizePage)
}

export function getSyncJob(id) {
  return request(`/sync-jobs/${id}`)
}

export function createSyncJob(input) {
  return request('/sync-jobs', { method: 'POST', body: input })
}

export function updateSyncJob(id, input) {
  return request(`/sync-jobs/${id}`, { method: 'PUT', body: input })
}

export function deleteSyncJob(id) {
  return request(`/sync-jobs/${id}`, { method: 'DELETE' })
}

export function runSyncJob(id) {
  return request(`/sync-jobs/${id}/run`, { method: 'POST' })
}

export function startSyncJob(id) {
  return request(`/sync-jobs/${id}/start`, { method: 'POST' })
}

export function exploreSyncJob(id, { side, page = 1, pageSize = 50, sortBy = '', sortDir = '' } = {}) {
  const body = { side, page, page_size: pageSize }
  if (sortBy) {
    body.sort_by = sortBy
    body.sort_dir = sortDir || 'asc'
  }
  return request(`/sync-jobs/${id}/explore`, {
    method: 'POST',
    body,
  })
}

export function exportSyncJobCSV(id, { side } = {}) {
  return download(`/sync-jobs/${id}/explore/export`, {
    method: 'POST',
    body: { side },
    filename: `explore-${side}.csv`,
  })
}
