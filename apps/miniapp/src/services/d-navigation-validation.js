const placeholderAppIDs = new Set(['legacy-missing-appid', 'wx0000000000000000'])
const miniProgramAppIDPattern = /^wx[a-zA-Z0-9]{16}$/
const miniProgramSchemePattern = /^[A-Za-z][A-Za-z0-9+.-]*:/
const unsafeUnicodePattern = /[\u0000-\u001f\u007f-\u009f\u00ad\u061c\u200b-\u200f\u2028-\u202e\u2060-\u206f\ufeff\ufff9-\ufffb]/
const nestedPercentEncodingPattern = /%[0-9a-f]{2}/i

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

function unsafeQueryComponent(value) {
  return unsafeUnicodePattern.test(value) ||
    value.includes('//') ||
    value.includes('\\') ||
    value.includes('..') ||
    value.includes('#') ||
    nestedPercentEncodingPattern.test(value)
}

function validMiniProgramQuery(rawQuery) {
  if (rawQuery.includes(';')) return false
  return rawQuery.split('&').every((pair) => {
    const separator = pair.indexOf('=')
    const rawKey = separator >= 0 ? pair.slice(0, separator) : pair
    const rawValue = separator >= 0 ? pair.slice(separator + 1) : ''
    const key = decodeComponent(rawKey, true)
    const value = decodeComponent(rawValue, true)
    return key !== null && value !== null && !unsafeQueryComponent(key) && !unsafeQueryComponent(value)
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
  if (decodedPath === null || !decodedPath || decodedPath.trim() !== decodedPath || decodedPath.startsWith('/')) return false
  if (unsafeUnicodePattern.test(decodedPath) || miniProgramSchemePattern.test(decodedPath)) return false
  if (decodedPath.includes('//') || decodedPath.includes('\\') || decodedPath.includes('#') || decodedPath.includes('?') || decodedPath.includes('%')) return false
  if (decodedPath.split('/').some((segment) => segment === '..')) return false

  return rawQuery === null || validMiniProgramQuery(rawQuery)
}

module.exports = {
  validMiniProgramAppID,
  validMiniProgramPath
}
