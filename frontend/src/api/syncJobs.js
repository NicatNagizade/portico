import { request } from './client'
import { normalizePage } from './pagination'

function buildListQuery({ page, pageSize } = {}) {
  const params = new URLSearchParams()
  if (page) params.set('page', String(page))
  if (pageSize) params.set('page_size', String(pageSize))
  const query = params.toString()
  return query ? `?${query}` : ''
}

export function listSyncJobs({ page = 1, pageSize = 20 } = {}) {
  return request(`/sync-jobs${buildListQuery({ page, pageSize })}`).then(normalizePage)
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
