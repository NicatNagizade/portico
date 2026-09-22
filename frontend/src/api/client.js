const API_BASE = import.meta.env.VITE_API_BASE || '/api'

export class ApiError extends Error {
  constructor(message, status) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function parseError(response) {
  try {
    const data = await response.json()
    return data?.error || response.statusText || 'Request failed'
  } catch {
    return response.statusText || 'Request failed'
  }
}

export async function request(path, options = {}) {
  const { method = 'GET', body, headers = {}, cache } = options
  const init = {
    method,
    headers: { ...headers },
  }
  if (cache) init.cache = cache

  if (body !== undefined) {
    init.headers['Content-Type'] = 'application/json'
    init.body = JSON.stringify(body)
  }

  const response = await fetch(`${API_BASE}${path}`, init)

  if (response.status === 204) {
    return null
  }

  if (!response.ok) {
    throw new ApiError(await parseError(response), response.status)
  }

  const contentType = response.headers.get('content-type') || ''
  if (!contentType.includes('application/json')) {
    return null
  }

  return response.json()
}

/** POST/GET a binary download and save it via a temporary anchor. */
export async function download(path, { method = 'GET', body, filename } = {}) {
  const init = { method, headers: {} }
  if (body !== undefined) {
    init.headers['Content-Type'] = 'application/json'
    init.body = JSON.stringify(body)
  }
  const response = await fetch(`${API_BASE}${path}`, init)
  if (!response.ok) {
    throw new ApiError(await parseError(response), response.status)
  }
  const disposition = response.headers.get('Content-Disposition') || ''
  const match = disposition.match(/filename="([^"]+)"/)
  const name = match?.[1] || filename || 'download'
  const url = URL.createObjectURL(await response.blob())
  const a = document.createElement('a')
  a.href = url
  a.download = name
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}
