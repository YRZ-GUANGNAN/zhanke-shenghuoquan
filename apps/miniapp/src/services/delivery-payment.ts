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
  cancelRequestedAt?: string
  providerStatus?: string
  nextVerifyAt?: string
  verifyAttemptCount?: number
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
  providerStatus?: string
  nextVerifyAt?: string
  verifyAttemptCount?: number
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

export function deliveryPaymentStatusLabel(value: DeliveryPayment): string {
  if (value.status === 'paid') return '已确认支付'
  if (value.status === 'failed') return '支付失败'
  if (value.status === 'closed') return '支付已关闭'
  if (value.providerStatus === 'USERPAYING') return '微信处理中，等待核实'
  if (value.status === 'prepay_issued') return '支付待核实'
  if (value.status === 'created') return '待发起支付'
  return '支付待核实'
}

export function deliveryRefundStatusLabel(value: DeliveryRefund): string {
  if (value.status === 'succeeded') return '已确认退款'
  if (value.status === 'failed') return '退款失败'
  if (value.providerStatus === 'PROCESSING') return '退款处理中，等待核实'
  if (value.status === 'pending') return '退款已申请，等待核实'
  return '退款待核实'
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

export function closeDeliveryWalletRecharge(paymentPublicId: string): Promise<DeliveryPayment> {
  return request<DeliveryPayment>(
    `${BASE}/wallet/recharges/${encodeURIComponent(paymentPublicId)}/close`,
    { url: '', method: 'POST', timeout: 20000 }
  )
}

export function getDeliveryWalletRechargeByRequest(clientRequestId: string, amountCents: number): Promise<DeliveryPayment | null> {
  return request<DeliveryPayment | null>(
    `${BASE}/wallet/recharges/by-request/${encodeURIComponent(clientRequestId)}?amountCents=${amountCents}`,
    { url: '', timeout: 10000 }
  )
}

export function closeDeliveryWalletRechargeByRequest(clientRequestId: string, amountCents: number): Promise<DeliveryPayment> {
  return request<DeliveryPayment>(`${BASE}/wallet/recharges/close`, {
    url: '', method: 'POST', data: { clientRequestId, amountCents }, timeout: 20000
  })
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

export async function pollDeliveryPayment(paymentPublicId: string, isCurrent: () => boolean = () => true): Promise<DeliveryPayment> {
  if (!isCurrent()) throw new Error('支付核实已中断，请返回原订单查看')
  let latest = await getDeliveryPayment(paymentPublicId)
  for (let attempt = 0; attempt < PAYMENT_POLL_ATTEMPTS; attempt += 1) {
    if (!isCurrent() || latest.status === 'paid' || latest.status === 'failed' || latest.status === 'closed') {
      return latest
    }
    await delay(PAYMENT_POLL_INTERVAL_MS)
    if (!isCurrent()) return latest
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
