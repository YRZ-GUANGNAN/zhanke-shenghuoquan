import { request } from './api'
import type { DeliveryStatus, Page, StageTimestamps } from './delivery-merchant'
import type { SourcePlatform } from '../utils/delivery-order-source'

export type RiderStatus = 'active' | 'resting' | 'suspended'
export type RiderAssignedStatusGroup = 'pickup' | 'delivery' | 'completed'

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
  sourcePlatform?: SourcePlatform
  pickupCode?: string
  routeSummary: string
  deliveryFeeCents: number
  status: DeliveryStatus
  rowVersion: number
  createdAt: string
}

export interface AssignedDeliveryOrder extends StageTimestamps {
  deliveryOrderPublicId: string
  sourcePlatform?: SourcePlatform
  pickupCode?: string
  sourceOrderId?: string
  sourceShopId?: string
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

export function listAssignedDeliveryOrders(
  page = 1,
  pageSize = 50,
  statusGroup?: RiderAssignedStatusGroup
): Promise<Page<AssignedDeliveryOrder>> {
  const groupQuery = statusGroup ? `&statusGroup=${encodeURIComponent(statusGroup)}` : ''
  return request<Page<AssignedDeliveryOrder>>(`${BASE}/orders?page=${page}&pageSize=${pageSize}${groupQuery}`, {
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

export function markPickedUp(
  order: AssignedDeliveryOrder,
  proofAssetPublicId: string
): Promise<AssignedDeliveryOrder> {
  return proofCommand(order.deliveryOrderPublicId, 'picked-up', order.rowVersion, proofAssetPublicId)
}

export function startDelivery(order: AssignedDeliveryOrder): Promise<AssignedDeliveryOrder> {
  return riderCommand(order.deliveryOrderPublicId, 'delivering', order.rowVersion)
}

export function markDelivered(
  order: AssignedDeliveryOrder,
  proofAssetPublicId: string
): Promise<AssignedDeliveryOrder> {
  return proofCommand(order.deliveryOrderPublicId, 'delivered', order.rowVersion, proofAssetPublicId)
}

function proofCommand(
  deliveryOrderPublicId: string,
  action: 'picked-up' | 'delivered',
  expectedRowVersion: number,
  proofAssetPublicId: string
): Promise<AssignedDeliveryOrder> {
  return request<AssignedDeliveryOrder>(
    `${BASE}/orders/${encodeURIComponent(deliveryOrderPublicId)}/${action}`,
    {
      url: '',
      method: 'PATCH',
      data: { expectedRowVersion, proofAssetPublicId },
      timeout: 20000
    }
  )
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
