import { request } from './api'

export type RiderStatus = 'active' | 'resting' | 'suspended'
export type OrderStatus =
  | 'pending_payment'
  | 'paid'
  | 'accepted'
  | 'preparing'
  | 'ready_for_pickup'
  | 'delivering'
  | 'completed'
  | 'cancelled'
  | 'refunded'
export type FulfillmentStatus = 'ASSIGNED' | 'TO_STORE' | 'PICKED' | 'TO_USER' | 'DELIVERED'
export type WithdrawStatus = 'pending' | 'paid' | 'rejected'
export type SettlementStatus = 'pending_settlement' | 'settled'

export interface Page<T> {
  items: T[]
  page: number
  pageSize: number
  total: number
}

export interface Rider {
  riderUserId: number
  status: RiderStatus
  realName: string
  phone: string
  rowVersion: number
  createdAt: string
  updatedAt: string
}

export interface RiderMe {
  registered: boolean
  rider?: Rider
}

export interface RegisterRiderInput {
  realName: string
  phone: string
}

export interface RiderOrder {
  orderPublicId: string
  buyerUserId: number
  merchantPublicId: string
  merchantName?: string
  riderUserId?: number
  status: OrderStatus
  itemsAmountCents: number
  deliveryFeeCents: number
  totalAmountCents: number
  contactName: string
  contactPhone: string
  addressBuilding: string
  addressRoom: string
  remark: string
  paidAt?: string
  acceptedAt?: string
  preparingAt?: string
  readyAt?: string
  deliveringAt?: string
  completedAt?: string
  cancelledAt?: string
  refundedAt?: string
  rowVersion: number
  createdAt: string
  updatedAt: string
}

export interface RiderOrderItem {
  dishPublicId: string
  dishNameSnapshot: string
  unitPriceCentsSnapshot: number
  quantity: number
  lineAmountCents: number
}

export interface RiderOrderDetail {
  order: RiderOrder
  items: RiderOrderItem[]
}

export interface Fulfillment {
  fulfillmentPublicId: string
  orderPublicId: string
  riderUserId: number
  status: FulfillmentStatus
  assignedAt?: string
  toStoreAt?: string
  pickedAt?: string
  toUserAt?: string
  deliveredAt?: string
  rowVersion: number
  createdAt: string
  updatedAt: string
}

export interface FulfillmentDetail {
  fulfillment: Fulfillment
  order: RiderOrder
  items: RiderOrderItem[]
}

export interface Earning {
  earningPublicId: string
  orderPublicId: string
  riderUserId: number
  riderFeeCents: number
  platformCutCents: number
  settlementStatus: SettlementStatus
  settledAt?: string
  rowVersion: number
  createdAt: string
  updatedAt: string
}

export interface Wallet {
  riderUserId: number
  pendingSettlementCents: number
  pendingWithdrawCents: number
  availableWithdrawCents: number
  settledCents: number
  recentEarnings: Earning[]
  recentEarningsPage: number
  recentEarningsPageSize: number
  recentEarningsTotal: number
}

export interface Withdraw {
  withdrawPublicId: string
  riderUserId: number
  amountCents: number
  status: WithdrawStatus
  note: string
  paidAt?: string
  rowVersion: number
  createdAt: string
  updatedAt: string
}

const BASE = '/takeout/rider'

export function getRiderMe(): Promise<RiderMe> {
  return request<RiderMe>(`${BASE}/me`, { url: '', timeout: 10000 })
}

export function registerRider(input: RegisterRiderInput): Promise<Rider> {
  return request<Rider>(`${BASE}/register`, { url: '', method: 'POST', data: compact(input), timeout: 10000 })
}

export function listClaimableOrders(): Promise<Page<RiderOrderDetail>> {
  return request<Page<RiderOrderDetail>>(`${BASE}/claimable-orders`, { url: '', timeout: 10000 })
}

export function claimOrder(order: RiderOrder): Promise<FulfillmentDetail> {
  return request<FulfillmentDetail>(`${BASE}/orders/${encodeURIComponent(order.orderPublicId)}/claim`, {
    url: '',
    method: 'POST',
    data: { expectedRowVersion: order.rowVersion },
    timeout: 10000
  })
}

export function listFulfillments(status?: FulfillmentStatus, page = 1, pageSize = 50): Promise<Page<FulfillmentDetail>> {
  const params = [`page=${page}`, `pageSize=${pageSize}`]
  if (status) params.push(`status=${encodeURIComponent(status)}`)
  return request<Page<FulfillmentDetail>>(`${BASE}/fulfillments?${params.join('&')}`, { url: '', timeout: 10000 })
}

export function getFulfillmentDetail(fulfillmentPublicId: string): Promise<FulfillmentDetail> {
  return request<FulfillmentDetail>(`${BASE}/fulfillments/${encodeURIComponent(fulfillmentPublicId)}`, { url: '', timeout: 10000 })
}

export function startToStore(detail: FulfillmentDetail): Promise<FulfillmentDetail> {
  return fulfillmentAction(detail, 'to-store')
}

export function markPicked(detail: FulfillmentDetail): Promise<FulfillmentDetail> {
  return fulfillmentAction(detail, 'picked')
}

export function startToUser(detail: FulfillmentDetail): Promise<FulfillmentDetail> {
  return fulfillmentAction(detail, 'to-user')
}

export function completeFulfillment(detail: FulfillmentDetail): Promise<FulfillmentDetail> {
  return fulfillmentAction(detail, 'complete')
}

export function getWallet(page = 1, pageSize = 20): Promise<Wallet> {
  return request<Wallet>(`${BASE}/wallet?page=${page}&pageSize=${pageSize}`, { url: '', timeout: 10000 })
}

export function listWithdrawals(status?: WithdrawStatus, page = 1, pageSize = 50): Promise<Page<Withdraw>> {
  const params = [`page=${page}`, `pageSize=${pageSize}`]
  if (status) params.push(`status=${encodeURIComponent(status)}`)
  return request<Page<Withdraw>>(`${BASE}/withdrawals?${params.join('&')}`, { url: '', timeout: 10000 })
}

export function createWithdrawal(amountCents: number): Promise<Withdraw> {
  return request<Withdraw>(`${BASE}/withdrawals`, { url: '', method: 'POST', data: { amountCents }, timeout: 10000 })
}

export function centsToYuan(value: number): string {
  return (value / 100).toFixed(value % 100 === 0 ? 0 : 2)
}

function fulfillmentAction(detail: FulfillmentDetail, action: string): Promise<FulfillmentDetail> {
  return request<FulfillmentDetail>(`${BASE}/fulfillments/${encodeURIComponent(detail.fulfillment.fulfillmentPublicId)}/${action}`, {
    url: '',
    method: 'PATCH',
    data: { expectedRowVersion: detail.fulfillment.rowVersion },
    timeout: 10000
  })
}

function compact(input: object): Record<string, unknown> {
  const result: Record<string, unknown> = {}
  Object.entries(input).forEach(([key, value]) => {
    if (value !== undefined && value !== '') result[key] = value
  })
  return result
}
