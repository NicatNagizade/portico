import { request } from './client'
import { normalizePage } from './pagination'

function buildListQuery({ syncJobId, page, pageSize } = {}) {
  const params = new URLSearchParams()
  if (syncJobId !== undefined && syncJobId !== null && syncJobId !== '') {
    params.set('sync_job_id', String(syncJobId))
  }
  if (page) params.set('page', String(page))
  if (pageSize) params.set('page_size', String(pageSize))
  const query = params.toString()
  return query ? `?${query}` : ''
}

export function listSyncLogs({ syncJobId, page = 1, pageSize = 20 } = {}) {
  return request(`/sync-logs${buildListQuery({ syncJobId, page, pageSize })}`, {
    cache: 'no-store',
  }).then(normalizePage)
}

export function getSyncLog(id) {
  return request(`/sync-logs/${id}`, { cache: 'no-store' })
}

export function stopSyncLog(id) {
  return request(`/sync-logs/${id}/stop`, { method: 'POST' })
}
