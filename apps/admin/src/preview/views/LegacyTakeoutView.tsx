import { useCallback, useEffect, useState } from 'react'
import { TakeoutAPIError, takeoutAdminAPI } from '../../services/takeout-admin'
import type { MerchantRow, Page } from '../../types'
import {
  Button,
  EmptyState,
  Modal,
  PageHeader,
  Panel,
  StatCard,
  StatusPill,
  usePreviewToast
} from '../ui'

// 旧版外卖只做存量数据总览：统计来自列表接口 total，商家列表取前 10 条并支持暂停/恢复。
const merchantPageSize = 10
const todayOrderPageSize = 100
const todayOrderPageLimit = 10
const conflictMessage = '数据已被他人修改，请刷新后重试'

const merchantStatusLabels: Record<MerchantRow['status'], string> = {
  pending_review: '待审核',
  active: '营业中',
  resting: '休息中',
  rejected: '已驳回',
  suspended: '已暂停'
}

const merchantStatusTones: Record<MerchantRow['status'], 'success' | 'warning' | 'danger' | 'neutral'> = {
  pending_review: 'warning',
  active: 'success',
  resting: 'neutral',
  rejected: 'danger',
  suspended: 'danger'
}

interface LegacyOverview {
  dishes: number | null
  ordersToday: number | null
  ordersTotal: number | null
  ordersTodayCapped: boolean
  settlements: number | null
  pendingWithdrawals: number | null
}

const emptyOverview: LegacyOverview = {
  dishes: null,
  ordersToday: null,
  ordersTotal: null,
  ordersTodayCapped: false,
  settlements: null,
  pendingWithdrawals: null
}

function errorText(reason: unknown) {
  return reason instanceof Error ? reason.message : String(reason)
}

function formatDateTime(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString('zh-CN')
}

// 接口时间带时区偏移，按浏览器本地日期判断「今天」，不能只看字符串前缀。
function isToday(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return false
  const now = new Date()
  return date.getFullYear() === now.getFullYear()
    && date.getMonth() === now.getMonth()
    && date.getDate() === now.getDate()
}

// 订单列表按创建时间倒序：逐页累计今日订单，翻到非今日订单即停止。
async function countTodayOrders() {
  let page = 1
  let today = 0
  let total = 0
  let capped = false
  for (;;) {
    const result = await takeoutAdminAPI.orders({ page, pageSize: todayOrderPageSize })
    total = result.total
    today += result.items.filter(order => isToday(order.createdAt)).length
    const last = result.items[result.items.length - 1]
    const hasMore = page * todayOrderPageSize < result.total
    if (!last || !hasMore || !isToday(last.createdAt)) break
    if (page >= todayOrderPageLimit) {
      capped = true
      break
    }
    page += 1
  }
  return { today, total, capped }
}

interface PendingMerchantAction {
  merchant: MerchantRow
  action: 'suspend' | 'unsuspend'
}

export function LegacyTakeoutView() {
  const { notify } = usePreviewToast()
  const [merchantPage, setMerchantPage] = useState<Page<MerchantRow> | null>(null)
  const [overview, setOverview] = useState<LegacyOverview>(emptyOverview)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [pendingAction, setPendingAction] = useState<PendingMerchantAction | null>(null)
  const [actionBusy, setActionBusy] = useState(false)
  const [actionError, setActionError] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    const [merchantResult, dishResult, orderResult, settlementResult, withdrawalResult] = await Promise.allSettled([
      takeoutAdminAPI.merchants({ page: 1, pageSize: merchantPageSize }),
      takeoutAdminAPI.dishes({ page: 1, pageSize: 1 }),
      countTodayOrders(),
      takeoutAdminAPI.settlements({ page: 1, pageSize: 1 }),
      takeoutAdminAPI.withdrawals({ status: 'pending', page: 1, pageSize: 1 })
    ])
    if (merchantResult.status === 'fulfilled') setMerchantPage(merchantResult.value)
    setOverview({
      dishes: dishResult.status === 'fulfilled' ? dishResult.value.total : null,
      ordersToday: orderResult.status === 'fulfilled' ? orderResult.value.today : null,
      ordersTotal: orderResult.status === 'fulfilled' ? orderResult.value.total : null,
      ordersTodayCapped: orderResult.status === 'fulfilled' ? orderResult.value.capped : false,
      settlements: settlementResult.status === 'fulfilled' ? settlementResult.value.total : null,
      pendingWithdrawals: withdrawalResult.status === 'fulfilled' ? withdrawalResult.value.total : null
    })
    const failures: string[] = []
    if (merchantResult.status === 'rejected') failures.push(`商家列表：${errorText(merchantResult.reason)}`)
    if (dishResult.status === 'rejected') failures.push(`菜品统计：${errorText(dishResult.reason)}`)
    if (orderResult.status === 'rejected') failures.push(`订单统计：${errorText(orderResult.reason)}`)
    if (settlementResult.status === 'rejected') failures.push(`结算报表：${errorText(settlementResult.reason)}`)
    if (withdrawalResult.status === 'rejected') failures.push(`提现申请：${errorText(withdrawalResult.reason)}`)
    setError(failures.join('；'))
    setLoading(false)
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const openAction = (merchant: MerchantRow, action: PendingMerchantAction['action']) => {
    setActionError('')
    setPendingAction({ merchant, action })
  }

  const submitAction = async () => {
    if (!pendingAction) return
    const { merchant, action } = pendingAction
    setActionBusy(true)
    setActionError('')
    try {
      if (action === 'suspend') {
        await takeoutAdminAPI.suspendMerchant(merchant.merchantPublicId, merchant.status, merchant.rowVersion)
        notify(`已暂停商家「${merchant.name}」`)
      } else {
        await takeoutAdminAPI.unsuspendMerchant(merchant.merchantPublicId, merchant.rowVersion)
        notify(`已恢复商家「${merchant.name}」`)
      }
      setPendingAction(null)
      try {
        setMerchantPage(await takeoutAdminAPI.merchants({ page: 1, pageSize: merchantPageSize }))
      } catch (reason) {
        setError(`商家列表刷新失败：${errorText(reason)}`)
      }
    } catch (reason) {
      // 409 是乐观锁冲突，提示刷新；其他错误留在弹窗内展示。
      if (reason instanceof TakeoutAPIError && reason.status === 409) {
        setActionError(conflictMessage)
      } else {
        setActionError(errorText(reason))
      }
    } finally {
      setActionBusy(false)
    }
  }

  const refreshAfterConflict = async () => {
    setPendingAction(null)
    setActionError('')
    await load()
  }

  const ordersTotalText = overview.ordersTotal ?? '—'
  const settlementsText = overview.settlements === null ? '暂不可用' : `共 ${overview.settlements} 条汇总记录`
  const withdrawalsText = overview.pendingWithdrawals === null ? '暂不可用' : `待处理 ${overview.pendingWithdrawals} 笔`

  return <div>
    <PageHeader
      eyebrow="历史模块"
      title="旧版外卖"
      description="旧版外卖只保留存量商家与历史订单的只读概览；新下单、派单与履约统一走「配送与结算」，避免两套并行业务。"
    />

    <section className="preview-stat-grid">
      <StatCard label="存量商家" value={merchantPage ? merchantPage.total : '—'} meta="全部状态商家" tone="accent" unit="家" />
      <StatCard label="菜品数量" value={overview.dishes ?? '—'} meta="跨商家菜品总数" tone="mint" unit="个" />
      <StatCard
        label="今日订单"
        value={overview.ordersToday ?? '—'}
        meta={overview.ordersTotal === null
          ? '订单统计暂不可用'
          : overview.ordersTodayCapped
            ? `按最近 ${todayOrderPageSize * todayOrderPageLimit} 单统计 · 历史累计 ${ordersTotalText} 单`
            : `历史累计 ${ordersTotalText} 单`}
        tone="amber"
        unit="单"
      />
    </section>

    <Panel
      eyebrow="LEGACY DATA"
      title="外卖商家列表"
      description={`展示前 ${merchantPageSize} 家存量商家，暂停与恢复直接调用真实接口。`}
      actions={<Button size="sm" onClick={() => void load()} disabled={loading}>{loading ? '刷新中…' : '刷新'}</Button>}
    >
      {error && <div className="preview-warning-box" role="alert">
        <strong>数据加载异常</strong>
        <p>{error}</p>
      </div>}

      {!merchantPage
        ? <p>{loading ? '正在加载旧版外卖数据…' : '商家列表加载失败，请点击右上角刷新重试。'}</p>
        : merchantPage.items.length === 0
          ? <EmptyState title="暂无存量商家" description="旧版外卖没有可展示的商家数据。" />
          : <div className="preview-table-wrap">
            <table className="preview-table" aria-label="旧版外卖商家列表">
              <thead><tr><th>商家</th><th>联系方式</th><th>位置 / 营业时间</th><th>状态</th><th>更新时间</th><th>操作</th></tr></thead>
              <tbody>{merchantPage.items.map(merchant => <tr key={merchant.merchantPublicId}>
                <td><strong>{merchant.name}</strong><span>{merchant.merchantPublicId}</span></td>
                <td>{merchant.contactPhone || '—'}</td>
                <td><strong>{merchant.location || '—'}</strong><span>{merchant.businessHours || '—'}</span></td>
                <td><StatusPill tone={merchantStatusTones[merchant.status]}>{merchantStatusLabels[merchant.status]}</StatusPill></td>
                <td>{formatDateTime(merchant.updatedAt)}</td>
                <td><div className="preview-row-actions">
                  {(merchant.status === 'active' || merchant.status === 'resting') && <Button size="sm" variant="ghost" onClick={() => openAction(merchant, 'suspend')}>暂停</Button>}
                  {merchant.status === 'suspended' && <Button size="sm" variant="primary" onClick={() => openAction(merchant, 'unsuspend')}>恢复</Button>}
                </div></td>
              </tr>)}</tbody>
            </table>
          </div>}

      <div className="preview-policy-note">
        <span>迁移说明</span>
        <p>旧版外卖仅保留历史数据入口，结算与提现不在本页展开：结算报表{settlementsText}、提现申请{withdrawalsText}，均可通过对应列表接口继续查看，完整工作台见「配送与结算」。</p>
      </div>
    </Panel>

    <Modal
      open={Boolean(pendingAction)}
      title={pendingAction?.action === 'suspend' ? '暂停商家' : '恢复商家'}
      description={pendingAction ? `${pendingAction.merchant.name} · ${pendingAction.merchant.merchantPublicId}` : undefined}
      onClose={() => {
        if (actionBusy) return
        setPendingAction(null)
        setActionError('')
      }}
      footer={<>
        <Button disabled={actionBusy} onClick={() => { setPendingAction(null); setActionError('') }}>取消</Button>
        {actionError && <Button variant="ghost" onClick={() => void refreshAfterConflict()}>刷新数据</Button>}
        <Button
          variant={pendingAction?.action === 'suspend' ? 'danger' : 'primary'}
          disabled={actionBusy}
          onClick={() => void submitAction()}
        >{actionBusy ? '处理中…' : pendingAction?.action === 'suspend' ? '确认暂停' : '确认恢复'}</Button>
      </>}
    >
      {pendingAction && <div className="preview-form-stack">
        <div>
          当前状态：<StatusPill tone={merchantStatusTones[pendingAction.merchant.status]}>{merchantStatusLabels[pendingAction.merchant.status]}</StatusPill>
        </div>
        <p>{pendingAction.action === 'suspend'
          ? '暂停后该商家将无法继续接单，已产生的订单不受影响。'
          : '恢复后商家会回到营业中状态，可以继续接单。'}</p>
        {actionError && <div className="preview-form-error" role="alert">{actionError}</div>}
      </div>}
    </Modal>
  </div>
}
