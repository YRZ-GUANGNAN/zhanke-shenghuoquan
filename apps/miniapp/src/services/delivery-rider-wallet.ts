import { request } from './api'
import type { Page } from './delivery-merchant'
import { createDeliveryClientRequestId } from './delivery-merchant'

export type RiderWalletState = 'ready' | 'not_created'
export type RiderEarningStatus = 'pending' | 'settled' | 'paid_offline'
export type RiderPayoutQueueStatus = 'blocked' | 'queued' | 'cancelled'
export type RiderWithdrawalStatus = 'processing' | 'paid' | 'rejected'

export interface RiderWalletSummary {
  withdrawalEnabled: boolean
  withdrawableBalanceCents?: number
  walletState: RiderWalletState
  walletPublicId?: string
  availableBalanceCents: number
  reservedCents: number
  unverifiedCents: number
  // Missing on older servers: do not substitute unverified funds or invent 0.
  pendingSettlementCents?: number
  nextSettlementAt?: string
  updatedAt?: string
}

export interface RiderEarningView {
  earningPublicId: string
  deliveryOrderPublicId: string
  amountCents: number
  settlementStatus: RiderEarningStatus
  settleAt?: string
  settledAt?: string
  createdAt: string
  paidOfflineAt?: string
}

export interface RiderLedgerView {
  ledgerPublicId: string
  direction: 'credit' | 'debit'
  entryType: 'income' | 'adjustment' | 'payout'
  amountCents: number
  balanceBeforeCents: number
  balanceAfterCents: number
  referencePublicId?: string
  createdAt: string
}

export interface RiderPayoutQueueView {
  payoutQueuePublicId: string
  earningPublicId: string
  deliveryOrderPublicId: string
  amountCents: number
  status: RiderPayoutQueueStatus
  reasonCode?: string
  createdAt: string
  updatedAt: string
}

export interface RiderWithdrawalView {
  withdrawPublicId: string
  riderUserId: number
  amountCents: number
  status: RiderWithdrawalStatus
  reason?: string
  rowVersion: number
  processedAt?: string
  createdAt: string
  updatedAt: string
}

const BASE = '/delivery/rider/wallet'

function walletResponseError(): Error { return new Error('资金记录格式异常，请稍后重试') }
function record(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw walletResponseError()
  return value as Record<string, unknown>
}
function cents(value: unknown): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) throw walletResponseError()
  return value
}
function optionalTimestamp(value: unknown): string | undefined {
  if (value === undefined || value === null) return undefined
  // Explicit timezone prevents device-local interpretation of UTC timestamps.
  if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(value) || !Number.isFinite(Date.parse(value))) throw walletResponseError()
  return value
}
function requiredText(value: unknown): string {
  if (typeof value !== 'string' || !value.trim()) throw walletResponseError()
  return value
}
function normalizeWallet(value: unknown): RiderWalletSummary {
  const data = record(value)
  if (data.walletState !== 'ready' && data.walletState !== 'not_created') throw walletResponseError()
  const availableBalanceCents = cents(data.availableBalanceCents)
  const reservedCents = cents(data.reservedCents)
  if (reservedCents > availableBalanceCents) throw walletResponseError()
  const withdrawableBalanceCents = data.withdrawableBalanceCents === undefined || data.withdrawableBalanceCents === null
    ? undefined : cents(data.withdrawableBalanceCents)
  if (withdrawableBalanceCents !== undefined && withdrawableBalanceCents > availableBalanceCents - reservedCents) throw walletResponseError()
  return {
    // Older servers and malformed capability values must never open money writes.
    withdrawalEnabled: data.withdrawalEnabled === true && withdrawableBalanceCents !== undefined,
    withdrawableBalanceCents,
    walletState: data.walletState,
    walletPublicId: data.walletPublicId === undefined ? undefined : requiredText(data.walletPublicId),
    availableBalanceCents, reservedCents, unverifiedCents: cents(data.unverifiedCents),
    pendingSettlementCents: data.pendingSettlementCents === undefined || data.pendingSettlementCents === null
      ? undefined : cents(data.pendingSettlementCents),
    nextSettlementAt: optionalTimestamp(data.nextSettlementAt), updatedAt: optionalTimestamp(data.updatedAt),
  }
}
function normalizeEarning(value: unknown): RiderEarningView {
  const data = record(value)
  if (data.settlementStatus !== 'pending' && data.settlementStatus !== 'settled' && data.settlementStatus !== 'paid_offline') throw walletResponseError()
  const createdAt = optionalTimestamp(data.createdAt)
  if (!createdAt) throw walletResponseError()
  return {
    earningPublicId: requiredText(data.earningPublicId), deliveryOrderPublicId: requiredText(data.deliveryOrderPublicId),
    amountCents: cents(data.amountCents), settlementStatus: data.settlementStatus,
    settleAt: optionalTimestamp(data.settleAt), settledAt: optionalTimestamp(data.settledAt),
    createdAt, paidOfflineAt: optionalTimestamp(data.paidOfflineAt),
  }
}

export function getRiderWallet(): Promise<RiderWalletSummary> {
  return request<unknown>(BASE, { url: '', timeout: 10000 }).then(normalizeWallet)
}

export function listRiderEarnings(page = 1, pageSize = 20): Promise<Page<RiderEarningView>> {
  return request<unknown>(`${BASE}/earnings?page=${page}&pageSize=${pageSize}`, { url: '', timeout: 10000 }).then(value => {
    const data = record(value)
    if (!Array.isArray(data.items)) throw walletResponseError()
    const currentPage = data.page === undefined ? 1 : cents(data.page)
    const size = data.pageSize === undefined ? 20 : cents(data.pageSize)
    const total = data.total === undefined ? data.items.length : cents(data.total)
    if (currentPage < 1 || size < 1) throw walletResponseError()
    return { items: data.items.map(normalizeEarning), page: currentPage, pageSize: size, total }
  })
}

export function listRiderLedger(page = 1, pageSize = 20): Promise<Page<RiderLedgerView>> {
  return request<Page<RiderLedgerView>>(`${BASE}/ledger?page=${page}&pageSize=${pageSize}`, { url: '', timeout: 10000 })
}

export function listRiderPayoutQueue(page = 1, pageSize = 20): Promise<Page<RiderPayoutQueueView>> {
  return request<Page<RiderPayoutQueueView>>(`${BASE}/payout-queue?page=${page}&pageSize=${pageSize}`, { url: '', timeout: 10000 })
}

export function listRiderWithdrawals(page = 1, pageSize = 20): Promise<Page<RiderWithdrawalView>> {
  return request<Page<RiderWithdrawalView>>(`${BASE}/withdrawals?page=${page}&pageSize=${pageSize}`, { url: '', timeout: 10000 })
}

export function createRiderWithdrawal(amountCents: number, idempotencyKey = createDeliveryClientRequestId()): Promise<RiderWithdrawalView> {
  return request<RiderWithdrawalView>(`${BASE}/withdrawals`, {
    url: '', method: 'POST', timeout: 10000,
    data: { amountCents, idempotencyKey }, header: { 'Idempotency-Key': idempotencyKey },
  })
}
