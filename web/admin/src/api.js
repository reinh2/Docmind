const BASE = import.meta.env.VITE_API_BASE || '/api'

// api performs an authenticated JSON request against the admin API. It attaches the
// bearer token, unwraps a 204 to null, and surfaces the server's error message on failure.
export function api(token, path, options = {}) {
  const headers = new Headers(options.headers || {})
  headers.set('Authorization', `Bearer ${token}`)
  return fetch(`${BASE}${path}`, { ...options, headers }).then(async response => {
    if (!response.ok) {
      const body = await response.json().catch(() => ({}))
      throw new Error(body.error || `Request failed (${response.status})`)
    }
    return response.status === 204 ? null : response.json()
  })
}

export const formatDate = value => new Date(value).toLocaleString()
