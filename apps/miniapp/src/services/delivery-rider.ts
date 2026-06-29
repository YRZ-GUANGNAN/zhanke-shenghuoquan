import { request } from './api'
import type { DeliveryStatus, Page, StageTimestamps } from './delivery-merchant'

export type RiderStatus = 'active' | 'resting' | 'suspended'

export interface DeliveryRider {
  riderUserId: number
  status: RiderStatus
  realName: string
  phone: string
  rowVersion: number
}

export interface DeliveryRiderMe {
  registered: boolean
  message?: string
  rider?: DeliveryRider
}

export interface ClaimableDeliveryOrder {
  deliveryOrderPublicId: string
  routeSummary: string
  deliveryFeeCents: number
  status: DeliveryStatus
  rowVersion: number
  createdAt: string
}

export interface AssignedDeliveryOrder extends StageTimestamps {
  deliveryOrderPublicId: string
  merchantUserId: number
  riderUserId: number
  pickupContactName: string
  pickupContactPhone: string
  pickupAddress: string
  recipientName: string
  recipientPhone: string
  deliveryAddress: string
  remark: string
  deliveryFeeCents: number
  status: DeliveryStatus
  rowVersion: number
  createdAt: string
  updatedAt: string
}

const BASE = '/delivery/rider'

export function getDeliveryRiderMe(): Promise<DeliveryRiderMe> {
  return request<DeliveryRiderMe>(`${BASE}/me`, { url: '', timeout: 10000 })
}

export function listClaimableDeliveryOrders(page = 1, pageSize = 50): Promise<Page<ClaimableDeliveryOrder>> {
  return request<Page<ClaimableDeliveryOrder>>(
    `${BASE}/orders/claimable?page=${page}&pageSize=${pageSize}`,
    { url: '', timeout: 10000 }
  )
}

export function claimDeliveryOrder(order: ClaimableDeliveryOrder): Promise<AssignedDeliveryOrder> {
  return riderCommand(order.deliveryOrderPublicId, 'claim', order.rowVersion, 'POST')
}

export function listAssignedDeliveryOrders(page = 1, pageSize = 50): Promise<Page<AssignedDeliveryOrder>> {
  return request<Page<AssignedDeliveryOrder>>(`${BASE}/orders?page=${page}&pageSize=${pageSize}`, {
    url: '',
    timeout: 10000
  })
}

export function getAssignedDeliveryOrder(deliveryOrderPublicId: string): Promise<AssignedDeliveryOrder> {
  return request<AssignedDeliveryOrder>(
    `${BASE}/orders/${encodeURIComponent(deliveryOrderPublicId)}`,
    { url: '', timeout: 10000 }
  )
}

export function startToPickup(order: AssignedDeliveryOrder): Promise<AssignedDeliveryOrder> {
  return riderCommand(order.deliveryOrderPublicId, 'to-pickup', order.rowVersion)
}

export function markPickedUp(order: AssignedDeliveryOrder): Promise<AssignedDeliveryOrder> {
  return riderCommand(order.deliveryOrderPublicId, 'picked-up', order.rowVersion)
}

export function startDelivery(order: AssignedDeliveryOrder): Promise<AssignedDeliveryOrder> {
  return riderCommand(order.deliveryOrderPublicId, 'delivering', order.rowVersion)
}

export function markDelivered(order: AssignedDeliveryOrder): Promise<AssignedDeliveryOrder> {
  return riderCommand(order.deliveryOrderPublicId, 'delivered', order.rowVersion)
}

export function releaseDeliveryOrder(order: AssignedDeliveryOrder): Promise<AssignedDeliveryOrder> {
  return request<AssignedDeliveryOrder>(
    `${BASE}/orders/${encodeURIComponent(order.deliveryOrderPublicId)}/release`,
    {
      url: '',
      method: 'PATCH',
      data: { expectedStatus: order.status, expectedRowVersion: order.rowVersion },
      timeout: 10000
    }
  )
}

function riderCommand(
  deliveryOrderPublicId: string,
  action: string,
  expectedRowVersion: number,
  method: 'POST' | 'PATCH' = 'PATCH'
): Promise<AssignedDeliveryOrder> {
  return request<AssignedDeliveryOrder>(
    `${BASE}/orders/${encodeURIComponent(deliveryOrderPublicId)}/${action}`,
    {
      url: '',
      method,
      data: { expectedRowVersion },
      timeout: 10000
    }
  )
}
