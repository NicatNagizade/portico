/** Normalize list API responses to a page envelope. */
export function normalizePage(data) {
  if (Array.isArray(data)) {
    return {
      items: data,
      page: 1,
      page_size: data.length,
      total: data.length,
      total_pages: data.length > 0 ? 1 : 0,
    }
  }
  return {
    items: data?.items || [],
    page: data?.page ?? 1,
    page_size: data?.page_size ?? 0,
    total: data?.total ?? 0,
    total_pages: data?.total_pages ?? 0,
  }
}

/** Build `?a=1&b=2` from a plain object; omits null/undefined/''. */
export function buildQuery(params = {}) {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') continue
    search.set(key, String(value))
  }
  const query = search.toString()
  return query ? `?${query}` : ''
}

/** Parse a 1-based page from a URL/search string value. */
export function parsePage(raw) {
  const n = Number.parseInt(raw || '1', 10)
  return Number.isFinite(n) && n > 0 ? n : 1
}
