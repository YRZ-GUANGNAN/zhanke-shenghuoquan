import Taro from '../../services/taro'

export interface PendingWalletRecharge {
  accountId: string
  amountCents: number
  clientRequestId: string
  paymentPublicId?: string
  cancelRequested?: boolean
  createdAt: number
}

function key(accountId: string) { return `delivery.wallet.recharge.${accountId}` }

export function readPendingWalletRecharge(accountId: string): PendingWalletRecharge | null {
  if (!accountId) return null
  const value = Taro.getStorageSync<PendingWalletRecharge>(key(accountId))
  return value && value.accountId === accountId && typeof value.clientRequestId === 'string' && value.clientRequestId.length > 0
    && Number.isSafeInteger(value.amountCents) && value.amountCents > 0
    && Number.isFinite(value.createdAt) && value.createdAt > 0
    && (value.cancelRequested === undefined || typeof value.cancelRequested === 'boolean')
    && (value.paymentPublicId === undefined || (typeof value.paymentPublicId === 'string' && value.paymentPublicId.length > 0)) ? value : null
}

export function savePendingWalletRecharge(value: PendingWalletRecharge) { Taro.setStorageSync(key(value.accountId), value) }
export function clearPendingWalletRecharge(accountId: string) { if (accountId) Taro.removeStorageSync(key(accountId)) }
