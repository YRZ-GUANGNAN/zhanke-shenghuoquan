import { request } from './api'
import type { Page } from './delivery-merchant'
import { createDeliveryClientRequestId } from './delivery-merchant'

export type RiderWalletState = 'ready' | 'not_created'
export type RiderEarningStatus = 'pending' | 'settled' | 'paid_offline'
export type RiderPayoutQueueStatus = 'blocked' | 'queued' | 'cancelled'
export type RiderWithdrawalStatus = 'processing' | 'paid' | 'rejected'

export interface RiderWalletSummary {
  walletState: RiderWalletState
  walletPublicId?: string
  availableBalanceCents: number
  reservedCents: number
  unverifiedCents: number
  updatedAt?: string
}

export interface RiderEarningView {
  earningPublicId: string
  deliveryOrderPublicId: string
  amountCents: number
  settlementStatus: RiderEarningStatus
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

export function getRiderWallet(): Promise<RiderWalletSummary> {
  return request<RiderWalletSummary>(BASE, { url: '', timeout: 10000 })
}

export function listRiderEarnings(page = 1, pageSize = 20): Promise<Page<RiderEarningView>> {
  return request<Page<RiderEarningView>>(`${BASE}/earnings?page=${page}&pageSize=${pageSize}`, { url: '', timeout: 10000 })
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
