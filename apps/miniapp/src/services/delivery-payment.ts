import { request } from './api'
import { createDeliveryClientRequestId, type Page } from './delivery-merchant'

export type DeliveryPaymentPurpose = 'order_fee' | 'wallet_recharge'
export type DeliveryPaymentProvider = 'wechat' | 'mock'
export type DeliveryPaymentStatus = 'created' | 'prepay_issued' | 'paid' | 'failed' | 'closed'
export type DeliverySettlementSource = 'wallet' | 'wechat' | 'mock'
export type DeliverySettlementStatus = 'paid' | 'refund_pending' | 'refunded'
export type DeliveryRefundStatus = 'pending' | 'succeeded' | 'failed' | 'refund_pending'
export type DeliveryLedgerDirection = 'credit' | 'debit'
export type DeliveryLedgerEntryType = 'recharge' | 'order_fee' | 'refund'

export interface DeliveryWallet {
  walletPublicId: string
  availableBalanceCents: number
  rowVersion: number
  updatedAt: string
}

export interface DeliveryWalletLedger {
  ledgerPublicId: string
  direction: DeliveryLedgerDirection
  entryType: DeliveryLedgerEntryType
  amountCents: number
  balanceBeforeCents: number
  balanceAfterCents: number
  referenceType: string
  referencePublicId: string
  createdAt: string
}

export interface DeliveryWechatPaymentParameters {
  timeStamp: string
  nonceStr: string
  package: string
  signType: string
  paySign: string
}

export interface DeliveryPayment {
  paymentPublicId: string
  purpose: DeliveryPaymentPurpose
  deliveryOrderPublicId?: string
  provider: DeliveryPaymentProvider
  status: DeliveryPaymentStatus
  amountCents: number
  outTradeNo: string
  rowVersion: number
  paidAt?: string
  wechat?: DeliveryWechatPaymentParameters
}

export interface DeliverySettlement {
  settlementPublicId: string
  deliveryOrderPublicId: string
  source: DeliverySettlementSource
  amountCents: number
  status: DeliverySettlementStatus
  paidAt: string
  refundedAt?: string
}

export interface DeliveryRefund {
  refundPublicId: string
  deliveryOrderPublicId: string
  source: DeliverySettlementSource
  amountCents: number
  status: DeliveryRefundStatus
  outRefundNo: string
  failureReason?: string
  createdAt: string
  updatedAt: string
}

export interface DeliveryFundsOrder {
  deliveryOrderPublicId: string
  deliveryFeeCents: number
  status: string
  rowVersion: number
}

export interface DeliveryFundsResponse {
  wallet?: DeliveryWallet
  ledger?: DeliveryWalletLedger
  payment?: DeliveryPayment
  settlement?: DeliverySettlement
  order?: DeliveryFundsOrder
  refund?: DeliveryRefund
}

export interface DeliveryOrderFunds {
  deliveryOrderPublicId: string
  settlement: DeliverySettlement | null
  refund: DeliveryRefund | null
  payment: DeliveryPayment | null
}

const BASE = '/delivery/merchant'
const PAYMENT_POLL_INTERVAL_MS = 1000
const PAYMENT_POLL_ATTEMPTS = 30
const REFUND_POLL_INTERVAL_MS = 1500
const REFUND_POLL_ATTEMPTS = 20

export function createDeliveryFundsRequestId(): string {
  return createDeliveryClientRequestId()
}

export function getDeliveryWallet(): Promise<DeliveryWallet> {
  return request<DeliveryWallet>(`${BASE}/wallet`, { url: '', timeout: 10000 })
}

export function listDeliveryWalletLedger(page = 1, pageSize = 30): Promise<Page<DeliveryWalletLedger>> {
  return request<Page<DeliveryWalletLedger>>(
    `${BASE}/wallet/ledger?page=${page}&pageSize=${pageSize}`,
    { url: '', timeout: 10000 }
  )
}

export function startDeliveryWalletRecharge(
  amountCents: number,
  clientRequestId = createDeliveryFundsRequestId()
): Promise<DeliveryFundsResponse> {
  return request<DeliveryFundsResponse>(`${BASE}/wallet/recharges`, {
    url: '',
    method: 'POST',
    data: { amountCents, clientRequestId },
    timeout: 15000
  })
}

export function getDeliveryPayment(paymentPublicId: string): Promise<DeliveryPayment> {
  return request<DeliveryPayment>(
    `${BASE}/payments/${encodeURIComponent(paymentPublicId)}`,
    { url: '', timeout: 10000 }
  )
}

export function payDeliveryOrderWithWallet(
  deliveryOrderPublicId: string,
  expectedRowVersion: number,
  idempotencyKey = createDeliveryFundsRequestId()
): Promise<DeliveryFundsResponse> {
  return request<DeliveryFundsResponse>(
    `${BASE}/orders/${encodeURIComponent(deliveryOrderPublicId)}/pay-with-wallet`,
    {
      url: '',
      method: 'POST',
      data: { expectedRowVersion, idempotencyKey },
      timeout: 15000
    }
  )
}

export function payDeliveryOrderWithWechat(
  deliveryOrderPublicId: string,
  expectedRowVersion: number,
  clientRequestId = createDeliveryFundsRequestId()
): Promise<DeliveryFundsResponse> {
  return request<DeliveryFundsResponse>(
    `${BASE}/orders/${encodeURIComponent(deliveryOrderPublicId)}/pay-with-wechat`,
    {
      url: '',
      method: 'POST',
      data: { expectedRowVersion, clientRequestId },
      timeout: 15000
    }
  )
}

export function requestDeliveryOrderRefund(
  deliveryOrderPublicId: string,
  expectedRowVersion: number,
  idempotencyKey = createDeliveryFundsRequestId()
): Promise<DeliveryFundsResponse> {
  return request<DeliveryFundsResponse>(
    `${BASE}/orders/${encodeURIComponent(deliveryOrderPublicId)}/refunds`,
    {
      url: '',
      method: 'POST',
      data: { expectedRowVersion, idempotencyKey },
      timeout: 15000
    }
  )
}

export function getDeliveryOrderFunds(deliveryOrderPublicId: string): Promise<DeliveryOrderFunds> {
  return request<DeliveryOrderFunds>(
    `${BASE}/orders/${encodeURIComponent(deliveryOrderPublicId)}/funds`,
    { url: '', timeout: 10000 }
  )
}

export async function pollDeliveryPayment(paymentPublicId: string): Promise<DeliveryPayment> {
  let latest = await getDeliveryPayment(paymentPublicId)
  for (let attempt = 0; attempt < PAYMENT_POLL_ATTEMPTS; attempt += 1) {
    if (latest.status === 'paid' || latest.status === 'failed' || latest.status === 'closed') {
      return latest
    }
    await delay(PAYMENT_POLL_INTERVAL_MS)
    latest = await getDeliveryPayment(paymentPublicId)
  }
  return latest
}

export async function pollDeliveryRefund(deliveryOrderPublicId: string): Promise<DeliveryOrderFunds> {
  let latest = await getDeliveryOrderFunds(deliveryOrderPublicId)
  for (let attempt = 0; attempt < REFUND_POLL_ATTEMPTS; attempt += 1) {
    if (!latest.refund || latest.refund.status === 'succeeded' || latest.refund.status === 'failed') {
      return latest
    }
    await delay(REFUND_POLL_INTERVAL_MS)
    latest = await getDeliveryOrderFunds(deliveryOrderPublicId)
  }
  return latest
}

function delay(milliseconds: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, milliseconds))
}
