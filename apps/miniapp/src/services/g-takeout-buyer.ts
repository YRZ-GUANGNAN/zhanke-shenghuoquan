import { request } from './api'

export type MerchantStatus = 'pending_review' | 'active' | 'resting' | 'rejected' | 'suspended'
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

export interface Page<T> {
  items: T[]
  page: number
  pageSize: number
  total: number
}

export interface BuyerMerchant {
  merchantPublicId: string
  name: string
  contactPhone: string
  location: string
  businessHours: string
  logoAssetPublicId?: string
  logoContentUrl?: string
  status: MerchantStatus
  createdAt: string
  updatedAt: string
}

export interface BuyerDish {
  dishPublicId: string
  merchantPublicId: string
  categoryPublicId?: string
  name: string
  description: string
  priceCents: number
  imageAssetPublicId?: string
  imageContentUrl?: string
  sortOrder: number
}

export interface BuyerOrder {
  orderPublicId: string
  buyerUserId: number
  merchantPublicId: string
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

export interface BuyerOrderItem {
  dishPublicId: string
  dishNameSnapshot: string
  unitPriceCentsSnapshot: number
  quantity: number
  lineAmountCents: number
}

export interface BuyerOrderDetail {
  order: BuyerOrder
  items: BuyerOrderItem[]
}

export interface CreateBuyerOrderInput {
  merchantPublicId: string
  items: Array<{ dishPublicId: string; quantity: number }>
  address: {
    contactName: string
    contactPhone: string
    addressBuilding: string
    addressRoom: string
  }
  remark: string
}

export interface ResolvedDisplayURL {
  url: string
  expiresInSeconds: number
}

const BASE = '/takeout/buyer'

export function listBuyerMerchants(page = 1, pageSize = 20): Promise<Page<BuyerMerchant>> {
  return request<Page<BuyerMerchant>>(`${BASE}/merchants?page=${page}&pageSize=${pageSize}`, { url: '', timeout: 10000 })
}

export function getBuyerMerchant(merchantPublicId: string): Promise<BuyerMerchant> {
  return request<BuyerMerchant>(`${BASE}/merchants/${encodeURIComponent(merchantPublicId)}`, { url: '', timeout: 10000 })
}

export function listBuyerDishes(merchantPublicId: string, page = 1, pageSize = 100): Promise<Page<BuyerDish>> {
  return request<Page<BuyerDish>>(`${BASE}/merchants/${encodeURIComponent(merchantPublicId)}/dishes?page=${page}&pageSize=${pageSize}`, { url: '', timeout: 10000 })
}

export function createBuyerOrder(input: CreateBuyerOrderInput): Promise<BuyerOrderDetail> {
  return request<BuyerOrderDetail>(`${BASE}/orders`, { url: '', method: 'POST', data: input, timeout: 15000 })
}

export function listBuyerOrders(status?: OrderStatus, page = 1, pageSize = 50): Promise<Page<BuyerOrder>> {
  const params = [`page=${page}`, `pageSize=${pageSize}`]
  if (status) params.push(`status=${encodeURIComponent(status)}`)
  return request<Page<BuyerOrder>>(`${BASE}/orders?${params.join('&')}`, { url: '', timeout: 10000 })
}

export function getBuyerOrderDetail(orderPublicId: string): Promise<BuyerOrderDetail> {
  return request<BuyerOrderDetail>(`${BASE}/orders/${encodeURIComponent(orderPublicId)}`, { url: '', timeout: 10000 })
}

export function resolveDisplayContentURL(contentUrl: string): Promise<ResolvedDisplayURL> {
  const path = displayContentURLToResolvePath(contentUrl)
  return request<ResolvedDisplayURL>(path, { url: '', timeout: 10000 })
}

export function displayContentURLToResolvePath(contentUrl: string): string {
  const clean = contentUrl.trim()
  if (!clean) throw new Error('media display URL is empty')
  const marker = '/miniapp/media/display/'
  const markerIndex = clean.indexOf(marker)
  if (markerIndex < 0) throw new Error('invalid media display URL')
  const coordinate = clean.slice(markerIndex + marker.length)
  const suffix = '/content'
  if (!coordinate.endsWith(suffix)) throw new Error('invalid media display URL')
  return `/miniapp/media/resolve/${coordinate.slice(0, -suffix.length)}`
}

export function centsToYuan(value: number): string {
  return (value / 100).toFixed(value % 100 === 0 ? 0 : 2)
}
