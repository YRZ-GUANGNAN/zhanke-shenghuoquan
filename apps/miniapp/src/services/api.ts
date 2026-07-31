import Taro, { type RequestOptions } from './taro'

declare const TARO_APP_API_BASE: string | undefined

const DEFAULT_API_BASE = 'https://zhanduiwang.top/api/v1'

export const API_BASE = (
  typeof TARO_APP_API_BASE !== 'undefined' && TARO_APP_API_BASE
    ? TARO_APP_API_BASE
    : DEFAULT_API_BASE
).replace(/\/$/, '')

export class ApiError extends Error {
  readonly status: number
  readonly code: number
	readonly data?: unknown

  constructor(message: string, status: number, code = status, data?: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
		this.data = data
  }
}

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError
}

export async function request<T>(url: string, options: RequestOptions = { url: '' }): Promise<T> {
  if (!API_BASE) {
    throw new ApiError('API base is not configured', 0, -1)
  }

  const token = Taro.getStorageSync<string>('domainA.token')
  let response
  try {
    response = await Taro.request({
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
  } catch (error) {
    const message = error instanceof Error ? error.message : 'Network request failed'
    throw new ApiError(message, 0, -1)
  }

  const body = response.data as { data?: T; message?: string; code?: number }
  if (response.statusCode >= 400 || (body?.code ?? 0) !== 0) {
    throw new ApiError(
      body?.message || `Request failed: ${response.statusCode}`,
      response.statusCode,
      body?.code ?? response.statusCode,
			body?.data
    )
  }
  if (body && typeof body === 'object' && 'data' in body) {
    return body.data as T
  }
  return response.data as T
}
