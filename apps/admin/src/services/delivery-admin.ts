const base = '/api/v1/admin/delivery'

export class DeliveryAPIError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'DeliveryAPIError'
    this.status = status
  }
}

export interface Page<T> {
  items: T[]
  page: number
  pageSize: number
  total: number
}

export interface DeliveryQuery {
  page?: number
  pageSize?: number
  status?: string
  merchantUserId?: string | number
  riderUserId?: string | number
  from?: string
  to?: string
}

export interface DeliverySummary {
  orders: number
  pendingFeePayment: number
  readyForClaim: number
  inProgress: number
  delivered: number
  cancelled: number
  todayOrders: number
  todayDelivered: number
  merchantWalletCents: number
  riderWalletCents: number
  pendingRiderFeeCents: number
  paidOfflineCents: number
  platformFeeIncomeCents: number
}

export interface FeeConfig {
  configScope: string
  deliveryFeeCents: number
  platformCutCents: number
  status: string
  rowVersion: number
  createdAt: string
  updatedAt: string
}

export interface MerchantFeeOverride {
  merchantUserId: number
  deliveryFeeCents: number
  platformCutCents: number
  status: string
  rowVersion: number
  createdAt: string
  updatedAt: string
}

export interface DeliveryOrder {
  deliveryOrderPublicId: string
  merchantRequestId?: string
  merchantUserId: number
  riderUserId?: number
  pickupContactName: string
  pickupContactPhone: string
  pickupAddress: string
  recipientName: string
  recipientPhone: string
  deliveryAddress: string
  remark: string
  deliveryFeeCents: number
  riderFeeCents: number
  platformCutCents: number
  status: string
  rowVersion: number
  readyForClaimAt?: string | null
  claimedAt?: string | null
  toPickupAt?: string | null
  pickedUpAt?: string | null
  deliveringAt?: string | null
  deliveredAt?: string | null
  paymentFailedAt?: string | null
  cancelledAt?: string | null
  createdAt: string
  updatedAt: string
}

export interface MerchantWallet {
  walletPublicId: string
  merchantUserId: number
  availableBalanceCents: number
  rowVersion: number
  createdAt: string
  updatedAt: string
}

export interface MerchantLedger {
  ledgerPublicId: string
  walletPublicId: string
  merchantUserId: number
  direction: string
  entryType: string
  amountCents: number
  balanceBeforeCents: number
  balanceAfterCents: number
  referenceType: string
  referencePublicId: string
  createdAt: string
}

export interface Payment {
  paymentPublicId: string
  merchantUserId: number
  purpose: string
  deliveryOrderPublicId?: string
  provider: string
  status: string
  amountCents: number
  outTradeNo: string
  rowVersion: number
  paidAt?: string
  createdAt: string
  updatedAt: string
}

export interface Settlement {
  settlementPublicId: string
  deliveryOrderPublicId: string
  merchantUserId: number
  source: string
  amountCents: number
  status: string
  paymentPublicId?: string
  ledgerPublicId?: string
  paidAt: string
  refundedAt?: string
  rowVersion: number
}

export interface Refund {
  refundPublicId: string
  settlementPublicId: string
  deliveryOrderPublicId: string
  merchantUserId: number
  source: string
  amountCents: number
  status: string
  outRefundNo: string
  failureReason?: string
  rowVersion: number
  createdAt: string
  updatedAt: string
  succeededAt?: string
}

export interface OrderFunds {
  order: DeliveryOrder
  settlement?: Settlement
  refund?: Refund
  payment?: Payment
}

export interface RiderWallet {
  walletPublicId: string
  riderUserId: number
  availableBalanceCents: number
  rowVersion: number
  createdAt: string
  updatedAt: string
}

export interface RiderLedger {
  ledgerPublicId: string
  walletPublicId: string
  riderUserId: number
  direction: string
  entryType: string
  amountCents: number
  balanceBeforeCents: number
  balanceAfterCents: number
  referenceType: string
  referencePublicId: string
  createdAt: string
}

export interface RiderEarning {
  earningPublicId: string
  deliveryOrderPublicId: string
  riderUserId: number
  deliveryFeeCents: number
  platformCutCents: number
  riderFeeCents: number
  settlementStatus: string
  paidOfflineAt?: string
  rowVersion: number
  createdAt: string
  updatedAt: string
}

export interface RiderPayout {
  payoutPublicId: string
  earningPublicId: string
  deliveryOrderPublicId: string
  riderUserId: number
  operatorUserId: number
  amountCents: number
  idempotencyKey: string
  note: string
  paidOfflineAt: string
  rowVersion: number
  createdAt: string
  updatedAt: string
}

export interface RiderPayoutResult {
  wallet: RiderWallet
  earning: RiderEarning
  ledger: RiderLedger
  payout: RiderPayout
}

async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = localStorage.getItem('domainA.token') || ''
  const response = await fetch(`${base}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(init.headers || {})
    }
  })
  const body = await response.json() as { code: number; message: string; data: T }
  if (!response.ok || body.code !== 0) throw new DeliveryAPIError(response.status, body.message || `HTTP ${response.status}`)
  return body.data
}

function queryString(query: DeliveryQuery = {}) {
  const params = new URLSearchParams()
  Object.entries(query).forEach(([key, value]) => {
    if (value !== undefined && value !== '') params.set(key, String(value))
  })
  const encoded = params.toString()
  return encoded ? `?${encoded}` : ''
}

const body = (value: unknown): RequestInit => ({
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(value)
})

export const deliveryAdminAPI = {
  summary: () => api<DeliverySummary>('/summary'),
  orders: (query: DeliveryQuery = {}) => api<Page<DeliveryOrder>>(`/orders${queryString(query)}`),
  order: (id: string) => api<DeliveryOrder>(`/orders/${encodeURIComponent(id)}`),
  orderFunds: (id: string) => api<OrderFunds>(`/orders/${encodeURIComponent(id)}/funds`),
  feeConfig: () => api<FeeConfig>('/fee-config'),
  updateFeeConfig: (deliveryFeeCents: number, platformCutCents: number, expectedRowVersion: number) =>
    api<FeeConfig>('/fee-config', { ...body({ deliveryFeeCents, platformCutCents, expectedRowVersion }), method: 'PUT' }),
  merchantFeeOverrides: (query: DeliveryQuery = {}) => api<Page<MerchantFeeOverride>>(`/merchant-fee-overrides${queryString(query)}`),
  merchantFeeOverride: (merchantUserId: number) => api<MerchantFeeOverride>(`/merchant-fee-overrides/${merchantUserId}`),
  upsertMerchantFeeOverride: (merchantUserId: number, deliveryFeeCents: number, platformCutCents: number, expectedRowVersion?: number) =>
    api<MerchantFeeOverride>(`/merchant-fee-overrides/${merchantUserId}`, {
      ...body({ deliveryFeeCents, platformCutCents, expectedRowVersion }),
      method: 'PUT'
    }),
  disableMerchantFeeOverride: (merchantUserId: number, expectedRowVersion: number) =>
    api<MerchantFeeOverride>(`/merchant-fee-overrides/${merchantUserId}/disable`, {
      ...body({ expectedRowVersion }),
      method: 'PATCH'
    }),
  merchantWallets: (query: DeliveryQuery = {}) => api<Page<MerchantWallet>>(`/merchant-wallets${queryString(query)}`),
  merchantWallet: (merchantUserId: number) => api<MerchantWallet>(`/merchant-wallets/${merchantUserId}`),
  merchantLedger: (merchantUserId: number, query: DeliveryQuery = {}) => api<Page<MerchantLedger>>(`/merchant-wallets/${merchantUserId}/ledger${queryString(query)}`),
  riderWallets: (query: DeliveryQuery = {}) => api<Page<RiderWallet>>(`/rider-wallets${queryString(query)}`),
  riderWallet: (riderUserId: number) => api<RiderWallet>(`/rider-wallets/${riderUserId}`),
  riderLedger: (riderUserId: number, query: DeliveryQuery = {}) => api<Page<RiderLedger>>(`/rider-wallets/${riderUserId}/ledger${queryString(query)}`),
  riderEarnings: (query: DeliveryQuery = {}) => api<Page<RiderEarning>>(`/rider-earnings${queryString(query)}`),
  riderEarning: (id: string) => api<RiderEarning>(`/rider-earnings/${encodeURIComponent(id)}`),
  payOffline: (id: string, expectedRowVersion: number, idempotencyKey: string, note: string) =>
    api<RiderPayoutResult>(`/rider-earnings/${encodeURIComponent(id)}/pay-offline`, {
      ...body({ expectedRowVersion, idempotencyKey, note }),
      method: 'POST'
    }),
  riderPayouts: (query: DeliveryQuery = {}) => api<Page<RiderPayout>>(`/rider-payouts${queryString(query)}`),
  refunds: (query: DeliveryQuery = {}) => api<Page<Refund>>(`/refunds${queryString(query)}`),
  refund: (id: string) => api<Refund>(`/refunds/${encodeURIComponent(id)}`)
}
