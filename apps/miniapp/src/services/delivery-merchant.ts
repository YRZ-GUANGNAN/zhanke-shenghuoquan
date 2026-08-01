import { request } from './api'

export type DeliveryStatus =
  | 'pending_fee_payment'
  | 'payment_failed'
  | 'ready_for_claim'
  | 'claimed'
  | 'to_pickup'
  | 'picked_up'
  | 'delivering'
  | 'delivered'
  | 'cancelled'

export type MerchantDeliveryStatusGroup = 'pending' | 'delivering' | 'completed'

export interface StageTimestamps {
  readyForClaimAt: string | null
  claimedAt: string | null
  toPickupAt: string | null
  pickedUpAt: string | null
  deliveringAt: string | null
  deliveredAt: string | null
  paymentFailedAt: string | null
  cancelledAt: string | null
}

export interface MerchantDeliveryOrder extends StageTimestamps {
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
  status: DeliveryStatus
  rowVersion: number
  createdAt: string
  updatedAt: string
}

export interface Page<T> {
  items: T[]
  page: number
  pageSize: number
  total: number
}

export interface CreateMerchantDeliveryInput {
  clientRequestId: string
  pickupName: string
  pickupPhone: string
  pickupAddress: string
  recipientName: string
  recipientPhone: string
  deliveryAddress: string
  remark: string
}

const BASE = '/delivery/merchant'
const CROCKFORD = '0123456789ABCDEFGHJKMNPQRSTVWXYZ'

export function createDeliveryClientRequestId(now = Date.now()): string {
  let value = Math.max(0, Math.floor(now))
  let timestamp = ''
  for (let index = 0; index < 10; index += 1) {
    timestamp = CROCKFORD[value % 32] + timestamp
    value = Math.floor(value / 32)
  }
  let random = ''
  for (let index = 0; index < 16; index += 1) {
    random += CROCKFORD[Math.floor(Math.random() * CROCKFORD.length)]
  }
  return `${timestamp}${random}`
}

export function createMerchantDeliveryOrder(input: CreateMerchantDeliveryInput): Promise<MerchantDeliveryOrder> {
  return request<MerchantDeliveryOrder>(`${BASE}/orders`, {
    url: '',
    method: 'POST',
    data: input,
    header: { 'Idempotency-Key': input.clientRequestId },
    timeout: 10000
  })
}

export function listMerchantDeliveryOrders(
  page = 1,
  pageSize = 50,
  statusGroup?: MerchantDeliveryStatusGroup
): Promise<Page<MerchantDeliveryOrder>> {
  const groupQuery = statusGroup ? `&statusGroup=${encodeURIComponent(statusGroup)}` : ''
  return request<Page<MerchantDeliveryOrder>>(`${BASE}/orders?page=${page}&pageSize=${pageSize}${groupQuery}`, {
    url: '',
    timeout: 10000
  })
}

export function getMerchantDeliveryOrder(deliveryOrderPublicId: string): Promise<MerchantDeliveryOrder> {
  return request<MerchantDeliveryOrder>(
    `${BASE}/orders/${encodeURIComponent(deliveryOrderPublicId)}`,
    { url: '', timeout: 10000 }
  )
}

export function cancelMerchantDeliveryOrder(order: MerchantDeliveryOrder): Promise<MerchantDeliveryOrder> {
  return request<MerchantDeliveryOrder>(
    `${BASE}/orders/${encodeURIComponent(order.deliveryOrderPublicId)}/cancel`,
    {
      url: '',
      method: 'PATCH',
      data: { expectedStatus: order.status, expectedRowVersion: order.rowVersion },
      timeout: 10000
    }
  )
}

export function confirmMockDeliveryFee(order: MerchantDeliveryOrder): Promise<MerchantDeliveryOrder> {
  return request<MerchantDeliveryOrder>(
    `${BASE}/orders/${encodeURIComponent(order.deliveryOrderPublicId)}/mock-confirm-fee`,
    {
      url: '',
      method: 'POST',
      data: { expectedRowVersion: order.rowVersion },
      timeout: 10000
    }
  )
}

export function centsToYuan(value: number): string {
  return (value / 100).toFixed(value % 100 === 0 ? 0 : 2)
}
