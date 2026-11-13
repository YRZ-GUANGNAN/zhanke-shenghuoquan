import { adminRequest } from './http'

const base = '/admin/delivery'

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

export type RiderAvailabilityConfig = { enabled: false } | {
  enabled: true
  maxInFlightPerRider: number
  rowVersion: number
  updatedAt: string
}

export function parseRiderAvailabilityConfig(value: unknown): RiderAvailabilityConfig {
  if (!value || typeof value !== 'object') throw new Error('骑手容量配置暂不可用')
  const row = value as Record<string, unknown>
  if (row.enabled === false) return { enabled: false }
  if (typeof row.enabled !== 'boolean' || typeof row.maxInFlightPerRider !== 'number'
    || !Number.isSafeInteger(row.maxInFlightPerRider) || row.maxInFlightPerRider < 1 || row.maxInFlightPerRider > 100
    || typeof row.rowVersion !== 'number' || !Number.isSafeInteger(row.rowVersion) || row.rowVersion < 1
    || typeof row.updatedAt !== 'string') throw new Error('骑手容量配置数据无效，请刷新')
  return row as unknown as RiderAvailabilityConfig
}

export interface RiderPayoutQueue {
  payoutQueuePublicId: string
  earningPublicId: string
  deliveryOrderPublicId: string
  riderUserId: number
  amountCents: number
  currency: string
  status: 'blocked' | 'queued' | 'cancelled'
  reasonCode?: string
  rowVersion: number
  reservedCents: number
  createdAt: string
  updatedAt: string
  cancelledAt?: string | null
}

export interface RiderPayoutQueueEvent {
  eventPublicId: string
  eventType: string
  actorType: string
  operatorUserId?: number
  reasonCode?: string
  reason?: string
  createdAt: string
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
  todayDeliveryFeeCents: number
  todayPlatformFeeCents: number
  merchantWalletCents: number
  riderWalletCents: number
  pendingRiderFeeCents: number
  paidOfflineCents: number
  platformFeeIncomeCents: number
}

export interface NotificationChannel {
  channel: 'in_app' | 'wechat_subscribe' | 'wechat_official' | 'wecom'
  label: string
  enabled: boolean
  pending: number
  accepted: number
  retryWait: number
  blocked: number
  deadletter: number
}

export interface NotificationChannelOverview {
  checkedAt: string
  sendingEnabled: boolean
  audience: 'test' | 'all'
  channels: NotificationChannel[]
  officialAccount: {
    enabled: boolean
    templateConfigured: boolean
    unknown: number
    following: number
    notFollowing: number
  }
  wecom: {
    merchantConfigured: boolean
    riderConfigured: boolean
  }
}

export interface FundingSourceSummary {
  fundingTablesAvailable: boolean
  fundingLots: number
  fundingAllocations: number
  walletBalanceCents: number
  knownAvailableCents: number
  verifiedAvailableCents: number
  unverifiedAvailableCents: number
  coverageGapCents: number
  coverageDirection: 'balanced' | 'untracked_wallet_balance' | 'source_exceeds_wallet' | 'data_error' | string
  paymentVerificationPending: number
  paymentVerificationErrorCount: number
  refundVerificationPending: number
  refundVerificationErrorCount: number
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
  providerStatus?: string
  nextVerifyAt?: string
  verifyAttemptCount: number
  errorCode?: string
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
  providerStatus?: string
  nextVerifyAt?: string
  verifyAttemptCount: number
  errorCode?: string
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

export interface RiderWithdrawal {
  withdrawPublicId: string
  riderUserId: number
  amountCents: number
  status: 'processing' | 'paid' | 'rejected'
  reason?: string
  rowVersion: number
  processedAt?: string
  createdAt: string
  updatedAt: string
}

export interface RiderWithdrawalProcessing {
  withdrawal: RiderWithdrawal
  status: 'unclaimed' | 'active' | 'released' | 'completed' | 'exception'
  ownedByMe: boolean
  claimKey: string
  operatorUserId: number
  reason: string
  paymentReference: string
}

export type RiderWithdrawalAction = 'claim' | 'release' | 'approve' | 'reject' | 'exception' | 'resolve_paid' | 'resolve_unpaid'
export interface RiderWithdrawalCommand {
  expectedRowVersion: number
  idempotencyKey: string
  claimKey?: string
  reason?: string
  paymentReference?: string
  confirmedUnpaid?: boolean
}

async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  return adminRequest<T>(`${base}${path}`, init, (status, message) => new DeliveryAPIError(status, message))
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
  fundingSources: () => api<FundingSourceSummary>('/funding-sources'),
  orders: (query: DeliveryQuery = {}) => api<Page<DeliveryOrder>>(`/orders${queryString(query)}`),
  order: (id: string) => api<DeliveryOrder>(`/orders/${encodeURIComponent(id)}`),
  orderFunds: (id: string) => api<OrderFunds>(`/orders/${encodeURIComponent(id)}/funds`),
  feeConfig: () => api<FeeConfig>('/fee-config'),
  riderAvailabilityConfig: async () => parseRiderAvailabilityConfig(await api<unknown>('/rider-config')),
  updateRiderAvailabilityConfig: async (maxInFlightPerRider: number, expectedRowVersion: number) => {
    if (!Number.isSafeInteger(maxInFlightPerRider) || maxInFlightPerRider < 1 || maxInFlightPerRider > 100
      || !Number.isSafeInteger(expectedRowVersion) || expectedRowVersion < 1) throw new Error('骑手容量参数无效')
    return parseRiderAvailabilityConfig(await api<unknown>('/rider-config', {
      ...body({ maxInFlightPerRider, expectedRowVersion }), method: 'PATCH'
    }))
  },
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
  riderPayoutQueue: (query: DeliveryQuery = {}) => api<Page<RiderPayoutQueue>>(`/rider-payout-queue${queryString(query)}`),
  riderPayoutQueueDetail: (id: string) => api<RiderPayoutQueue>(`/rider-payout-queue/${encodeURIComponent(id)}`),
  riderPayoutQueueEvents: (id: string, query: DeliveryQuery = {}) => api<Page<RiderPayoutQueueEvent>>(`/rider-payout-queue/${encodeURIComponent(id)}/events${queryString(query)}`),
  cancelRiderPayoutQueue: (id: string, expectedRowVersion: number, idempotencyKey: string, reason: string) =>
    api<RiderPayoutQueue>(`/rider-payout-queue/${encodeURIComponent(id)}/cancel`, {
      ...body({ expectedRowVersion, idempotencyKey, reason }), method: 'POST'
    }),
  riderWithdrawals: (query: DeliveryQuery = {}) => api<Page<RiderWithdrawal>>(`/rider-withdrawals${queryString(query)}`),
  riderWithdrawalCapabilities: () => api<{ withdrawalEnabled: boolean; mode: 'manual'; claimRequired: boolean }>('/rider-withdrawal-capabilities'),
  riderWithdrawalProcessing: (id: string) => api<RiderWithdrawalProcessing>(`/rider-withdrawals/${encodeURIComponent(id)}/processing`),
  processRiderWithdrawal: (id: string, action: RiderWithdrawalAction, command: RiderWithdrawalCommand) => {
    const route = action === 'resolve_paid' ? 'resolve-paid' : action === 'resolve_unpaid' ? 'resolve-unpaid' : action
    return api<RiderWithdrawal>(`/rider-withdrawals/${encodeURIComponent(id)}/${route}`, { ...body(command), method: 'POST' })
  },
  approveRiderWithdrawal: (id: string, expectedRowVersion: number, idempotencyKey: string, paymentReference: string, claimKey: string) =>
    api<RiderWithdrawal>(`/rider-withdrawals/${encodeURIComponent(id)}/approve`, {
      ...body({ expectedRowVersion, idempotencyKey, paymentReference, claimKey }), method: 'POST'
    }),
  rejectRiderWithdrawal: (id: string, expectedRowVersion: number, idempotencyKey: string, reason: string) =>
    api<RiderWithdrawal>(`/rider-withdrawals/${encodeURIComponent(id)}/reject`, {
      ...body({ expectedRowVersion, idempotencyKey, reason }), method: 'POST'
    }),
  refunds: (query: DeliveryQuery = {}) => api<Page<Refund>>(`/refunds${queryString(query)}`),
  refund: (id: string) => api<Refund>(`/refunds/${encodeURIComponent(id)}`),
  notificationChannels: () => api<NotificationChannelOverview>('/notification-channels'),
  testWecomGroup: (group: 'merchant_ops' | 'rider_ops') =>
    api<{ accepted: boolean; group: string }>('/notification-channels/wecom/test', {
      ...body({ group }), method: 'POST'
    })
}
