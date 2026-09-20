import { request } from './client'

export function listSyncLogs(syncJobId) {
  const query =
    syncJobId !== undefined && syncJobId !== null && syncJobId !== ''
      ? `?sync_job_id=${encodeURIComponent(syncJobId)}`
      : ''
  return request(`/sync-logs${query}`, { cache: 'no-store' })
}

export function getSyncLog(id) {
  return request(`/sync-logs/${id}`)
}
