import { useAdminStore } from '../store'

const apiBase = '/api/v1'

export class HTTPError extends Error {
  constructor(readonly status: number, message: string) {
    super(message)
    this.name = 'HTTPError'
  }
}

type Envelope<T> = { code: number; message: string; data?: T }
type ErrorFactory = (status: number, message: string) => Error

const defaultErrorFactory: ErrorFactory = (status, message) => new HTTPError(status, message)

export async function adminRequest<T>(
  path: string,
  init: RequestInit = {},
  errorFactory: ErrorFactory = defaultErrorFactory
): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body !== undefined && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  const method = (init.method || 'GET').toUpperCase()
  if (method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS') {
    const csrfToken = useAdminStore.getState().csrfToken
    if (csrfToken) headers.set('X-CSRF-Token', csrfToken)
  }

  let response: Response
  try {
    response = await fetch(`${apiBase}${path}`, {
      ...init,
      credentials: 'same-origin',
      headers
    })
  } catch {
    throw errorFactory(0, '网络连接失败，请稍后重试')
  }

  let value: unknown
  try {
    value = await response.json()
  } catch {
    throw errorFactory(response.status, response.ok ? '服务返回格式无效' : `HTTP ${response.status}`)
  }
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw errorFactory(response.status, '服务返回格式无效')
  }
  const body = value as Partial<Envelope<T>>
  if (typeof body.code !== 'number' || typeof body.message !== 'string') {
    throw errorFactory(response.status, '服务返回格式无效')
  }
  if (!response.ok || body.code !== 0) {
    if (response.status === 401) useAdminStore.getState().setUnauthenticated()
    const status = response.ok && body.code > 0 ? body.code : response.status
    throw errorFactory(status, body.message || `HTTP ${status}`)
  }
  if (!('data' in body)) throw errorFactory(response.status, '服务返回格式无效')
  return body.data as T
}
