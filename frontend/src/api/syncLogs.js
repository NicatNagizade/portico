import { request } from './client'
import { buildQuery, normalizePage } from './pagination'

export function listSyncLogs({ syncJobId, page = 1, pageSize = 20 } = {}) {
  return request(
    `/sync-logs${buildQuery({ sync_job_id: syncJobId, page, page_size: pageSize })}`,
    { cache: 'no-store' },
  ).then(normalizePage)
}

export function getSyncLog(id) {
  return request(`/sync-logs/${id}`, { cache: 'no-store' })
}

export function stopSyncLog(id) {
  return request(`/sync-logs/${id}/stop`, { method: 'POST' })
}
