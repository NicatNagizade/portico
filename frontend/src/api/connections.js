import { request } from './client'

export function listConnections() {
  return request('/connections')
}

export function getConnection(id) {
  return request(`/connections/${id}`)
}

export function createConnection(input) {
  return request('/connections', { method: 'POST', body: input })
}

export function updateConnection(id, input) {
  return request(`/connections/${id}`, { method: 'PUT', body: input })
}

export function deleteConnection(id) {
  return request(`/connections/${id}`, { method: 'DELETE' })
}

export function listConnectionTables(id) {
  return request(`/connections/${id}/tables`).then((data) => data?.tables || [])
}

export function listConnectionColumns(id, table) {
  const query = new URLSearchParams({ table })
  return request(`/connections/${id}/columns?${query}`).then((data) => data?.columns || [])
}

export function checkConnection(input) {
  return request('/connections/check', { method: 'POST', body: input })
}
