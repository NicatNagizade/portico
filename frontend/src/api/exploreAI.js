import { request } from './client'

/** Ask the LLM to fill explore filters + visible fields from a natural-language prompt. */
export function suggestExplore(id, { prompt, filterFields = [], fields = [] } = {}) {
  return request(`/sync-jobs/${id}/explore/suggest`, {
    method: 'POST',
    body: {
      prompt,
      filter_fields: filterFields,
      fields,
    },
  })
}
