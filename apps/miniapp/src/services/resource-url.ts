const HTTP_ORIGIN = /^(https?:\/\/[^/]+)/i

export function resolveApiResourceUrl(value: string, apiBase: string): string {
  const candidate = value.trim()
  if (!candidate.startsWith('/') || candidate.startsWith('//')) return value
  const origin = apiBase.match(HTTP_ORIGIN)?.[1]
  return origin ? `${origin}${candidate}` : value
}
