import { useCallback, useEffect, useRef, useState } from 'react'
import { DeliveryAPIError, type RiderAdminRow } from '../services/delivery-admin'
import type { Tone } from './data'

// 配送视图组共用的数据加载、金额时间格式化与状态映射。
// 真实接口只返回英文字段值，这里统一翻译，避免各面板各写一套。

export const DELIVERY_PAGE_SIZE = 8

export const formatMoney = (cents?: number | null) => `¥${((cents ?? 0) / 100).toFixed(2)}`

export const formatDateTime = (value?: string | null) => {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString('zh-CN')
}

export const shortId = (value: string, head = 8, tail = 6) =>
  value.length > head + tail + 1 ? `${value.slice(0, head)}…${value.slice(-tail)}` : value

export const conflictMessage = '数据已被他人修改，请刷新后重试'

export const isDeliveryConflict = (reason: unknown) =>
  reason instanceof DeliveryAPIError && reason.status === 409

export const errorText = (reason: unknown) => (reason instanceof Error ? reason.message : String(reason))

export const deliveryErrorMessage = (reason: unknown) =>
  isDeliveryConflict(reason) ? conflictMessage : errorText(reason)

// 写操作幂等键：优先用 crypto.randomUUID，非安全上下文下退回时间戳随机串。
export function newOperationKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}-${Math.random().toString(36).slice(2, 12)}`
}

// 通用远程数据 hook：deps 变化时重新加载；返回 reload 供写操作后刷新。
export function useRemoteData<T>(loader: () => Promise<T>, deps: ReadonlyArray<unknown>) {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const loaderRef = useRef(loader)
  loaderRef.current = loader
  const epoch = useRef(0)
  const reload = useCallback(async () => {
    const version = ++epoch.current
    setLoading(true)
    setError('')
    try {
      const value = await loaderRef.current()
      if (epoch.current === version) setData(value)
    } catch (reason) {
      if (epoch.current === version) setError(deliveryErrorMessage(reason))
    } finally {
      if (epoch.current === version) setLoading(false)
    }
  }, [])
  useEffect(() => {
    void reload()
    return () => { epoch.current += 1 }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps)
  return { data, error, loading, reload }
}

export function useDebouncedValue<T>(value: T, delay = 300) {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(value), delay)
    return () => window.clearTimeout(timer)
  }, [value, delay])
  return debounced
}

/* ===== 配送订单状态 ===== */

export const orderStatusLabels: Record<string, string> = {
  pending_fee_payment: '待付配送费',
  payment_failed: '支付失败',
  ready_for_claim: '等待接单',
  claimed: '已接单',
  to_pickup: '前往取货',
  picked_up: '已取货',
  delivering: '配送中',
  delivered: '已送达',
  cancelled: '已取消'
}

const orderStatusTones: Record<string, Tone> = {
  pending_fee_payment: 'warning',
  payment_failed: 'danger',
  ready_for_claim: 'warning',
  claimed: 'info',
  to_pickup: 'info',
  picked_up: 'info',
  delivering: 'info',
  delivered: 'success',
  cancelled: 'neutral'
}

export const orderStatusLabel = (status: string) => orderStatusLabels[status] || status || '未知状态'
export const orderStatusTone = (status: string): Tone => orderStatusTones[status] || 'neutral'

// 与后端管理员命令保持一致的可用状态集合：
// 已取货、配送中的订单不能直接取消，需要走退款流程。
export const reassignableOrderStatuses = new Set(['claimed', 'to_pickup', 'picked_up', 'delivering'])
export const cancellableOrderStatuses = new Set(['pending_fee_payment', 'payment_failed', 'ready_for_claim', 'claimed', 'to_pickup'])
export const selectableOrderStatuses = new Set([...reassignableOrderStatuses, ...cancellableOrderStatuses])

/* ===== 骑手状态 ===== */

export const riderStatusLabels: Record<RiderAdminRow['status'], string> = {
  active: '接单中',
  resting: '休息中',
  suspended: '已封禁'
}

const riderStatusTones: Record<RiderAdminRow['status'], Tone> = {
  active: 'success',
  resting: 'neutral',
  suspended: 'danger'
}

export const riderStatusLabel = (status: string) =>
  riderStatusLabels[status as RiderAdminRow['status']] || status
export const riderStatusTone = (status: string): Tone =>
  riderStatusTones[status as RiderAdminRow['status']] || 'neutral'
export const dutyStateLabel = (state: string) => state === 'on_duty' ? '值班中' : '未值班'
export const riderName = (id: number, realName?: string) => realName?.trim() ? realName : `骑手 #${id}`
export const merchantName = (id: number) => `商家 #${id}`

/* ===== 资金状态 ===== */

export const earningStatusLabels: Record<string, string> = {
  pending: '待结算',
  settled: '已结算',
  paid_offline: '已登记结款'
}

const earningStatusTones: Record<string, Tone> = {
  pending: 'warning',
  settled: 'success',
  paid_offline: 'success'
}

export const earningStatusLabel = (status: string) => earningStatusLabels[status] || status
export const earningStatusTone = (status: string): Tone => earningStatusTones[status] || 'neutral'

export const payoutQueueStatusLabels: Record<string, string> = {
  queued: '已排队',
  blocked: '已暂停',
  cancelled: '已取消'
}

const payoutQueueStatusTones: Record<string, Tone> = {
  queued: 'warning',
  blocked: 'danger',
  cancelled: 'neutral'
}

export const payoutQueueStatusLabel = (status: string) => payoutQueueStatusLabels[status] || status
export const payoutQueueStatusTone = (status: string): Tone => payoutQueueStatusTones[status] || 'neutral'

export const withdrawalStatusLabels: Record<string, string> = {
  processing: '提现中',
  paid: '已登记打款',
  rejected: '已退回'
}

const withdrawalStatusTones: Record<string, Tone> = {
  processing: 'warning',
  paid: 'success',
  rejected: 'danger'
}

export const withdrawalStatusLabel = (status: string) => withdrawalStatusLabels[status] || status
export const withdrawalStatusTone = (status: string): Tone => withdrawalStatusTones[status] || 'neutral'

export const withdrawalProcessingLabels: Record<string, string> = {
  unclaimed: '未领取',
  released: '已解除，可重新领取',
  active: '处理中（处理权不自动过期）',
  exception: '异常对账中（保持资金占用）',
  completed: '已登记打款'
}

export const refundStatusLabels: Record<string, string> = {
  pending: '处理中',
  succeeded: '已退款',
  failed: '退款失败',
  refund_pending: '等待重试'
}

const refundStatusTones: Record<string, Tone> = {
  pending: 'warning',
  succeeded: 'success',
  failed: 'danger',
  refund_pending: 'warning'
}

export const refundStatusLabel = (status: string) => refundStatusLabels[status] || status
export const refundStatusTone = (status: string): Tone => refundStatusTones[status] || 'neutral'

export const paymentStatusLabels: Record<string, string> = {
  created: '已创建',
  prepay_issued: '已发起支付',
  paid: '已支付',
  failed: '支付失败',
  closed: '已关闭'
}
export const paymentStatusLabel = (status: string) => paymentStatusLabels[status] || status

export const settlementStatusLabels: Record<string, string> = {
  paid: '已支付',
  refund_pending: '退款处理中',
  refunded: '已退款'
}
export const settlementStatusLabel = (status: string) => settlementStatusLabels[status] || status

export const settlementSourceLabels: Record<string, string> = {
  wallet: '商家钱包',
  wechat: '微信支付',
  mock: '模拟通道'
}
export const settlementSourceLabel = (source: string) => settlementSourceLabels[source] || source

export const feeStatusLabels: Record<string, string> = {
  active: '生效中',
  disabled: '已停用'
}

const feeStatusTones: Record<string, Tone> = {
  active: 'success',
  disabled: 'neutral'
}

export const feeStatusLabel = (status: string) => feeStatusLabels[status] || status
export const feeStatusTone = (status: string): Tone => feeStatusTones[status] || 'neutral'

export const ledgerDirectionLabel = (direction: string) =>
  direction === 'credit' ? '收入' : direction === 'debit' ? '支出' : direction

export const ledgerEntryTypeLabels: Record<string, string> = {
  recharge: '充值',
  order_fee: '订单配送费',
  refund: '退款',
  income: '报酬入账',
  adjustment: '调整',
  payout: '线下打款'
}
export const ledgerEntryTypeLabel = (entryType: string) => ledgerEntryTypeLabels[entryType] || entryType

export const ledgerReferenceLabels: Record<string, string> = {
  delivery_order: '配送单',
  payment: '支付',
  refund: '退款',
  rider_withdrawal: '提现'
}
export const ledgerReferenceLabel = (referenceType: string) =>
  ledgerReferenceLabels[referenceType] || referenceType || '—'
