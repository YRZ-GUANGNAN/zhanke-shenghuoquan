export interface ProductPriceParts {
  major: string
  minor: string
  hasMinor: boolean
  display: string
}

/**
 * Keep product prices stable across cards, search results and details.
 * Product.price is already normalized to yuan by the trade service.
 */
export function formatProductPrice(value: number | string | null | undefined): ProductPriceParts {
  const text = String(value ?? '').trim()
  const numeric = typeof value === 'number' ? value : Number(text)
  if (!text || !Number.isFinite(numeric) || numeric < 0 || !Number.isSafeInteger(Math.round(numeric * 100))) {
    return { major: '—', minor: '', hasMinor: false, display: '—' }
  }
  const fixed = (Math.round(numeric * 100) / 100).toFixed(2).replace(/\.00$/, '')
  const [major, minor = ''] = fixed.split('.')
  return { major, minor, hasMinor: Boolean(minor), display: fixed }
}
