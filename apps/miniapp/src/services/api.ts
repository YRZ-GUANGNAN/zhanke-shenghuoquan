import Taro, { type RequestOptions } from './taro'

declare const TARO_APP_API_BASE: string | undefined

const LOCAL_API_BASE = 'http://127.0.0.1:8080/api/v1'

const API_BASE = (
  typeof TARO_APP_API_BASE !== 'undefined' && TARO_APP_API_BASE
    ? TARO_APP_API_BASE
    : LOCAL_API_BASE
).replace(/\/$/, '')

export async function request<T>(url: string, options: RequestOptions = { url: '' }): Promise<T> {
  if (!API_BASE) {
    throw new Error('API base is not configured')
  }

  const token = Taro.getStorageSync<string>('domainA.token')
  const response = await Taro.request({
    url: `${API_BASE}${url}`,
    method: options.method || 'GET',
    data: options.data,
    timeout: options.timeout ?? 3000,
    header: {
      'content-type': 'application/json',
	  ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(options.header || {})
    }
  })

  const body = response.data as { data?: T; message?: string; code?: number }
	if (response.statusCode >= 400 || (body?.code ?? 0) !== 0) {
	  throw new Error(body?.message || `Request failed: ${response.statusCode}`)
	}
  if (body && typeof body === 'object' && 'data' in body) {
    return body.data as T
  }
  return response.data as T
}
