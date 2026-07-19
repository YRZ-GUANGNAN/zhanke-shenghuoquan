const placeholderAppIDs = new Set(['legacy-missing-appid', 'wx0000000000000000'])
const miniProgramAppIDPattern = /^wx[a-zA-Z0-9]{16}$/
const miniProgramSchemePattern = /^[A-Za-z][A-Za-z0-9+.-]*:/
const unsafeUnicodePattern = /[\u0000-\u001f\u007f-\u009f\u00ad\u061c\u200b-\u200f\u2028-\u202e\u2060-\u206f\ufeff\ufff9-\ufffb]/
const pathSemanticQueryTokens = new Set(['redirect', 'path', 'url', 'target', 'page', 'route', 'destination', 'jump', 'link'])
const compactPathSemanticQueryKeys = new Set([
  'redirecturl', 'redirectpath', 'targeturl', 'targetpath', 'pageurl', 'pagepath',
  'routeurl', 'routepath', 'destinationurl', 'destinationpath', 'jumpurl', 'jumppath',
  'linkurl', 'linkpath', 'nexturl', 'nextpath', 'returnurl', 'returnpath'
])

function validMiniProgramAppID(value) {
  if (typeof value !== 'string') return false
  const normalized = value.trim()
  return miniProgramAppIDPattern.test(normalized) && !placeholderAppIDs.has(normalized)
}

function decodeComponent(value, plusAsSpace = false) {
  try {
    return decodeURIComponent(plusAsSpace ? value.replace(/\+/g, ' ') : value)
  } catch {
    return null
  }
}

function unsafePlainQueryComponent(value) {
  return unsafeUnicodePattern.test(value) ||
    value.includes('#')
}

function pathSemanticQueryKey(value) {
  const compact = value.replace(/[^A-Za-z0-9]/g, '').toLowerCase()
  if (compactPathSemanticQueryKeys.has(compact)) return true
  const tokens = value
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .split(/[^A-Za-z0-9]+/)
    .filter(Boolean)
    .map((token) => token.toLowerCase())
  return tokens.some((token) => pathSemanticQueryTokens.has(token))
}

function validDecodedPathname(value) {
  if (!value || value.trim() !== value || value.startsWith('/')) return false
  if (unsafeUnicodePattern.test(value) || miniProgramSchemePattern.test(value)) return false
  if (value.includes('//') || value.includes('\\') || value.includes('#') || value.includes('?') || value.includes('%')) return false
  return !value.split('/').some((segment) => segment === '..')
}

function validDecodedRelativeMiniProgramPath(value) {
  if (!value || Array.from(value).length > 240 || value.includes('%')) return false
  const querySeparator = value.indexOf('?')
  const path = querySeparator >= 0 ? value.slice(0, querySeparator) : value
  const query = querySeparator >= 0 ? value.slice(querySeparator + 1) : null
  return validDecodedPathname(path) && (query === null || validMiniProgramQuery(query, true))
}

function validMiniProgramQuery(rawQuery, alreadyDecoded = false) {
  if (rawQuery.includes(';')) return false
  return rawQuery.split('&').every((pair) => {
    const separator = pair.indexOf('=')
    const rawKey = separator >= 0 ? pair.slice(0, separator) : pair
    const rawValue = separator >= 0 ? pair.slice(separator + 1) : ''
    const key = alreadyDecoded ? rawKey : decodeComponent(rawKey, true)
    const value = alreadyDecoded ? rawValue : decodeComponent(rawValue, true)
    if (key === null || value === null || unsafePlainQueryComponent(key) || unsafePlainQueryComponent(value)) return false
    return !pathSemanticQueryKey(key) || validDecodedRelativeMiniProgramPath(value)
  })
}

function validMiniProgramPath(value) {
  if (typeof value !== 'string' || unsafeUnicodePattern.test(value)) return false
  const normalized = value.trim()
  if (!normalized || Array.from(normalized).length > 240 || normalized.startsWith('/')) return false

  const querySeparator = normalized.indexOf('?')
  const rawPath = querySeparator >= 0 ? normalized.slice(0, querySeparator) : normalized
  const rawQuery = querySeparator >= 0 ? normalized.slice(querySeparator + 1) : null
  const decodedPath = decodeComponent(rawPath)
  if (decodedPath === null || !validDecodedPathname(decodedPath)) return false

  return rawQuery === null || validMiniProgramQuery(rawQuery)
}

module.exports = {
  validMiniProgramAppID,
  validMiniProgramPath
}
