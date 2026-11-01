import Taro from '../../services/taro'

export interface PendingRiderWithdrawal { accountId: string; amountCents: number; idempotencyKey: string }
const key = (accountId: string) => `delivery.rider.withdrawal.${accountId}`

export function parseWithdrawalCents(value: string): number | null {
  if (!/^(?:0|[1-9]\d{0,5})(?:\.\d{1,2})?$/.test(value.trim())) return null
  const [yuan, fraction = ''] = value.trim().split('.')
  const cents = Number(yuan) * 100 + Number(fraction.padEnd(2, '0'))
  return Number.isSafeInteger(cents) && cents > 0 ? cents : null
}

export function readPendingRiderWithdrawal(accountId: string): PendingRiderWithdrawal | null {
  let value: PendingRiderWithdrawal
  try { value = Taro.getStorageSync<PendingRiderWithdrawal>(key(accountId)) } catch { return null }
  return value && value.accountId === accountId && Number.isSafeInteger(value.amountCents) && value.amountCents > 0
    && /^[0-7][0-9A-HJKMNP-TV-Z]{25}$/.test(value.idempotencyKey) ? value : null
}
export function savePendingRiderWithdrawal(value: PendingRiderWithdrawal) { Taro.setStorageSync(key(value.accountId), value) }
export function clearPendingRiderWithdrawal(accountId: string) { Taro.removeStorageSync(key(accountId)) }
