import { request } from './client'

/** App health / feature flags (e.g. whether Ask AI is configured). */
export function getHealth() {
  return request('/health')
}
