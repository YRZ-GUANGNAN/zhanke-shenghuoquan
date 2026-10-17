// Demo photographs are packaged assets, never production upload URLs.
export function resolveApiResourceUrl(value: string, _apiBase: string) {
  // Chat and profile components render a blank avatar while identity loads.
  if (!value.trim()) return ''
  if (!/^\/demo-media\/[a-z-]+\.jpg$/.test(value)) throw new Error('非演示媒体已阻止')
  return value
}
