import Taro from '../../services/taro'

export interface PendingWalletRecharge {
  accountId: string
  amountCents: number
  clientRequestId: string
  paymentPublicId?: string
  createdAt: number
}

function key(accountId: string) { return `delivery.wallet.recharge.${accountId}` }

export function readPendingWalletRecharge(accountId: string): PendingWalletRecharge | null {
  if (!accountId) return null
  const value = Taro.getStorageSync<PendingWalletRecharge>(key(accountId))
  return value && value.accountId === accountId && value.clientRequestId ? value : null
}

export function savePendingWalletRecharge(value: PendingWalletRecharge) { Taro.setStorageSync(key(value.accountId), value) }
export function clearPendingWalletRecharge(accountId: string) { if (accountId) Taro.removeStorageSync(key(accountId)) }
