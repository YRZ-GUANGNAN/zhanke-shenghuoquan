import { request } from './api'

export type MerchantStatus = 'pending_review' | 'active' | 'resting' | 'rejected' | 'suspended'
export type DishStatus = 'draft' | 'on_shelf' | 'sold_out' | 'off_shelf' | 'removed'
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

export interface Merchant {
  merchantPublicId: string
  ownerUserId: number
  name: string
  contactPhone: string
  location: string
  businessHours: string
  logoAssetPublicId?: string
  status: MerchantStatus
  rowVersion: number
  createdAt: string
  updatedAt: string
}

export interface MerchantMe {
  onboarded: boolean
  merchant?: Merchant
}

export interface ReviewSummary {
  taskId: number
  objectType: string
  objectId: number
  status: string
}

export interface OnboardingResult {
  merchant: Merchant
  review: ReviewSummary
}

export interface Category {
  categoryPublicId: string
  merchantPublicId: string
  name: string
  sortOrder: number
  rowVersion: number
  createdAt: string
  updatedAt: string
}

export interface Dish {
  dishPublicId: string
  merchantPublicId: string
  categoryPublicId?: string
  name: string
  description: string
  priceCents: number
  imageAssetPublicId?: string
  status: DishStatus
  sortOrder: number
  rowVersion: number
  createdAt: string
  updatedAt: string
}

export interface Order {
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
  payNo?: string
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

export interface OrderItem {
  dishPublicId: string
  dishNameSnapshot: string
  unitPriceCentsSnapshot: number
  quantity: number
  lineAmountCents: number
}

export interface OrderDetail {
  order: Order
  items: OrderItem[]
}

export interface OnboardingInput {
  name: string
  contactPhone: string
  location: string
  businessHours: string
  logoAssetPublicId?: string
}

export interface CategoryInput {
  name: string
  sortOrder: number
}

export interface CategoryUpdateInput {
  name?: string
  sortOrder?: number
  expectedRowVersion: number
}

export interface DishInput {
  categoryPublicId?: string
  name: string
  description: string
  priceCents: number
  imageAssetPublicId?: string
  sortOrder: number
  expectedStatus?: DishStatus
  expectedRowVersion?: number
}

const BASE = '/takeout/merchant'

export function getMerchantMe(): Promise<MerchantMe> {
  return request<MerchantMe>(`${BASE}/me`, { url: '', timeout: 10000 })
}

export function submitMerchantOnboarding(input: OnboardingInput): Promise<OnboardingResult> {
  return request<OnboardingResult>(`${BASE}/onboarding`, { url: '', method: 'POST', data: compact(input), timeout: 20000 })
}

export function listCategories(page = 1, pageSize = 100): Promise<Page<Category>> {
  return request<Page<Category>>(`${BASE}/categories?page=${page}&pageSize=${pageSize}`, { url: '', timeout: 10000 })
}

export function createCategory(input: CategoryInput): Promise<Category> {
  return request<Category>(`${BASE}/categories`, { url: '', method: 'POST', data: input, timeout: 10000 })
}

export function updateCategory(categoryPublicId: string, input: CategoryUpdateInput): Promise<Category> {
  return request<Category>(`${BASE}/categories/${categoryPublicId}`, { url: '', method: 'PATCH', data: compact(input), timeout: 10000 })
}

export function listDishes(status?: DishStatus, categoryId?: string, page = 1, pageSize = 100): Promise<Page<Dish>> {
  const params = [`page=${page}`, `pageSize=${pageSize}`]
  if (status) params.push(`status=${encodeURIComponent(status)}`)
  if (categoryId) params.push(`categoryId=${encodeURIComponent(categoryId)}`)
  return request<Page<Dish>>(`${BASE}/dishes?${params.join('&')}`, { url: '', timeout: 10000 })
}

export function createDish(input: DishInput): Promise<Dish> {
  return request<Dish>(`${BASE}/dishes`, { url: '', method: 'POST', data: compact(input), timeout: 15000 })
}

export function updateDish(dishPublicId: string, input: DishInput): Promise<Dish> {
  return request<Dish>(`${BASE}/dishes/${dishPublicId}`, { url: '', method: 'PATCH', data: compact(input), timeout: 15000 })
}

export function putDishOnShelf(dish: Dish): Promise<Dish> {
  return request<Dish>(`${BASE}/dishes/${dish.dishPublicId}/on-shelf`, {
    url: '', method: 'PATCH', data: { expectedStatus: dish.status, expectedRowVersion: dish.rowVersion }, timeout: 10000
  })
}

export function markDishSoldOut(dish: Dish): Promise<Dish> {
  return dishAction(dish, 'sold-out')
}

export function restockDish(dish: Dish): Promise<Dish> {
  return dishAction(dish, 'restock')
}

export function takeDishOffShelf(dish: Dish): Promise<Dish> {
  return request<Dish>(`${BASE}/dishes/${dish.dishPublicId}/off-shelf`, {
    url: '', method: 'PATCH', data: { expectedStatus: dish.status, expectedRowVersion: dish.rowVersion }, timeout: 10000
  })
}

export function removeDish(dish: Dish): Promise<Dish> {
  return dishAction(dish, 'remove')
}

export function listOrders(status?: OrderStatus, page = 1, pageSize = 50): Promise<Page<Order>> {
  const params = [`page=${page}`, `pageSize=${pageSize}`]
  if (status) params.push(`status=${encodeURIComponent(status)}`)
  return request<Page<Order>>(`${BASE}/orders?${params.join('&')}`, { url: '', timeout: 10000 })
}

export function getOrderDetail(orderPublicId: string): Promise<OrderDetail> {
  return request<OrderDetail>(`${BASE}/orders/${orderPublicId}`, { url: '', timeout: 10000 })
}

export function acceptOrder(order: Order): Promise<Order> {
  return orderAction(order, 'accept')
}

export function startPreparing(order: Order): Promise<Order> {
  return orderAction(order, 'start-preparing')
}

export function markOrderReady(order: Order): Promise<Order> {
  return orderAction(order, 'mark-ready')
}

function dishAction(dish: Dish, action: string): Promise<Dish> {
  return request<Dish>(`${BASE}/dishes/${dish.dishPublicId}/${action}`, {
    url: '', method: 'PATCH', data: { expectedRowVersion: dish.rowVersion }, timeout: 10000
  })
}

function orderAction(order: Order, action: string): Promise<Order> {
  return request<Order>(`${BASE}/orders/${order.orderPublicId}/${action}`, {
    url: '', method: 'PATCH', data: { expectedRowVersion: order.rowVersion }, timeout: 10000
  })
}

function compact(input: object): Record<string, unknown> {
  const result: Record<string, unknown> = {}
  Object.entries(input).forEach(([key, value]) => {
    if (value !== undefined && value !== '') result[key] = value
  })
  return result
}
