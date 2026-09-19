import { request } from './client'

export function listSyncJobs() {
  return request('/sync-jobs')
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
