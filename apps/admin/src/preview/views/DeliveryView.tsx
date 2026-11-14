import { useMemo, useRef, useState } from 'react'
import {
  deliveryAdminAPI,
  type DeliveryOrder,
  type OrderFunds
} from '../../services/delivery-admin'
import {
  DELIVERY_PAGE_SIZE,
  cancellableOrderStatuses,
  conflictMessage,
  deliveryErrorMessage,
  errorText,
  formatDateTime,
  formatMoney,
  isDeliveryConflict,
  merchantName,
  orderStatusLabel,
  orderStatusTone,
  paymentStatusLabel,
  reassignableOrderStatuses,
  refundStatusLabel,
  riderName,
  selectableOrderStatuses,
  settlementSourceLabel,
  settlementStatusLabel,
  shortId,
  useRemoteData
} from '../delivery-hooks'
import { FeeSection, FundsSection, type FeeView, type FundView } from './DeliveryFundsViews'
import { DispatchSection } from './DispatchViews'
import {
  Button,
  Drawer,
  EmptyState,
  Field,
  Modal,
  PageHeader,
  Panel,
  Pagination,
  SearchBox,
  SelectField,
  StatCard,
  StatusPill,
  usePreviewToast
} from '../ui'

type BatchAction = 'reassign' | 'cancel'

const STATUS_FILTERS: Array<[string, string]> = [
  ['', '全部状态'],
  ['pending_fee_payment', '待付配送费'],
  ['payment_failed', '支付失败'],
  ['ready_for_claim', '等待接单'],
  ['claimed', '已接单'],
  ['to_pickup', '前往取货'],
  ['picked_up', '已取货'],
  ['delivering', '配送中'],
  ['delivered', '已送达'],
  ['cancelled', '已取消']
]

const STAGES: Array<[string, keyof DeliveryOrder]> = [
  ['进入抢单池', 'readyForClaimAt'],
  ['已接单', 'claimedAt'],
  ['前往取货', 'toPickupAt'],
  ['已取货', 'pickedUpAt'],
  ['开始配送', 'deliveringAt'],
  ['已送达', 'deliveredAt'],
  ['支付失败', 'paymentFailedAt'],
  ['已取消', 'cancelledAt']
]

export function DeliveryView() {
  const { notify } = usePreviewToast()
  const [view, setView] = useState<'overview' | 'dispatch' | 'funds' | 'fee'>('overview')
  const [dispatchView, setDispatchView] = useState<'riders' | 'merchants'>('riders')
  const [fundView, setFundView] = useState<FundView>('merchant')
  const [feeView, setFeeView] = useState<FeeView>('default')

  const [statusFilter, setStatusFilter] = useState('')
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [batchAction, setBatchAction] = useState<BatchAction | null>(null)
  const [batchRider, setBatchRider] = useState('')
  const [batchReason, setBatchReason] = useState('')
  const [batchError, setBatchError] = useState('')
  const [batchBusy, setBatchBusy] = useState(false)

  const [detailId, setDetailId] = useState<string | null>(null)
  const [detail, setDetail] = useState<OrderFunds | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState('')
  const detailEpoch = useRef(0)

  // 订单列表分页与状态筛选都走真实接口；搜索只过滤当前页，不伪造跨页结果。
  const ordersState = useRemoteData(async () => {
    const result = await deliveryAdminAPI.orders({
      status: statusFilter || undefined,
      page,
      pageSize: DELIVERY_PAGE_SIZE
    })
    if (result.items.length === 0 && result.total > 0 && page > 1) {
      setPage(Math.max(1, Math.ceil(result.total / result.pageSize)))
    }
    return result
  }, [statusFilter, page])
  const summaryState = useRemoteData(() => deliveryAdminAPI.summary(), [])
  // 改派候选人只列接单中的骑手，避免把订单交给休息中或已封禁的骑手。
  const ridersState = useRemoteData(() => deliveryAdminAPI.riders({ status: 'active', pageSize: 100 }), [])

  const orders = ordersState.data?.items ?? []
  const summary = summaryState.data
  const candidates = ridersState.data?.items ?? []
  const selectedOrders = orders.filter(order => selectedIds.includes(order.deliveryOrderPublicId))

  const visibleOrders = useMemo(() => {
    const normalized = query.trim().toLowerCase()
    if (!normalized) return orders
    return orders.filter(order => {
      const haystack = [
        order.deliveryOrderPublicId,
        order.merchantRequestId || '',
        order.pickupAddress,
        order.deliveryAddress,
        order.pickupContactName,
        order.pickupContactPhone,
        order.recipientName,
        order.recipientPhone,
        `#${order.merchantUserId}`,
        order.riderUserId ? `#${order.riderUserId}` : '待分配'
      ].join(' ').toLowerCase()
      return haystack.includes(normalized)
    })
  }, [orders, query])

  const selectableOnPage = visibleOrders.filter(order => selectableOrderStatuses.has(order.status))
  const allOnPageSelected = selectableOnPage.length > 0
    && selectableOnPage.every(order => selectedIds.includes(order.deliveryOrderPublicId))

  const toggleRow = (id: string, checked: boolean) => setSelectedIds(current =>
    checked ? [...current, id] : current.filter(item => item !== id))

  const resetSelection = () => setSelectedIds([])

  const openDetail = async (id: string) => {
    const version = ++detailEpoch.current
    setDetailId(id)
    setDetail(null)
    setDetailError('')
    setDetailLoading(true)
    try {
      const value = await deliveryAdminAPI.orderFunds(id)
      if (detailEpoch.current === version) setDetail(value)
    } catch (reason) {
      if (detailEpoch.current === version) setDetailError(deliveryErrorMessage(reason))
    } finally {
      if (detailEpoch.current === version) setDetailLoading(false)
    }
  }

  const closeDetail = () => {
    detailEpoch.current += 1
    setDetailId(null)
    setDetail(null)
    setDetailError('')
  }

  const openBatch = (action: BatchAction, ids: string[]) => {
    setSelectedIds(ids)
    setBatchRider(candidates[0] ? String(candidates[0].riderUserId) : '')
    setBatchReason('')
    setBatchError(action === 'reassign' && candidates.length === 0
      ? (ridersState.loading ? '正在加载接单中的骑手，请稍候' : '当前没有接单中的骑手，无法改派')
      : '')
    setBatchAction(action)
  }

  const refreshAfterWrite = async () => {
    await ordersState.reload()
    await summaryState.reload()
    await ridersState.reload()
    if (detailId) {
      try {
        const value = await deliveryAdminAPI.orderFunds(detailId)
        setDetail(value)
      } catch {
        // 详情刷新失败不覆盖列表结果，用户可重新打开详情。
      }
    }
  }

  const submitBatch = async () => {
    if (!batchAction) return
    if (!batchReason.trim()) {
      setBatchError(batchAction === 'reassign' ? '请填写改派原因' : '请填写取消原因')
      return
    }
    const targetRiderUserId = Number(batchRider)
    if (batchAction === 'reassign' && (!targetRiderUserId || !candidates.some(row => row.riderUserId === targetRiderUserId))) {
      setBatchError('请选择接单中的骑手')
      return
    }
    setBatchBusy(true)
    setBatchError('')
    const failures: string[] = []
    let ok = 0
    for (const order of selectedOrders) {
      const id = order.deliveryOrderPublicId
      const applicable = batchAction === 'reassign'
        ? reassignableOrderStatuses.has(order.status)
        : cancellableOrderStatuses.has(order.status)
      if (!applicable) {
        failures.push(`${id}：当前状态不可${batchAction === 'reassign' ? '改派' : '取消'}`)
        continue
      }
      try {
        if (batchAction === 'reassign') {
          await deliveryAdminAPI.reassignOrder(id, targetRiderUserId, order.rowVersion, batchReason.trim())
        } else {
          await deliveryAdminAPI.cancelOrder(id, order.rowVersion, batchReason.trim())
        }
        ok += 1
      } catch (reason) {
        failures.push(`${id}：${isDeliveryConflict(reason) ? conflictMessage : errorText(reason)}`)
      }
    }
    setBatchBusy(false)
    await refreshAfterWrite()
    if (failures.length === 0) {
      notify(batchAction === 'reassign'
        ? `已改派 ${ok} 笔订单给 ${riderName(targetRiderUserId, candidates.find(row => row.riderUserId === targetRiderUserId)?.realName)}`
        : `已取消 ${ok} 笔订单`, batchAction === 'reassign' ? 'success' : 'info')
      resetSelection()
      setBatchAction(null)
      setBatchError('')
    } else {
      notify(`已完成 ${ok} 笔，${failures.length} 笔失败`, 'danger')
      // 失败项保留选中，方便调整后重试；成功项从选择中剔除。
      const succeeded = selectedOrders
        .filter(order => !failures.some(failure => failure.startsWith(order.deliveryOrderPublicId)))
        .map(order => order.deliveryOrderPublicId)
      setSelectedIds(current => current.filter(id => !succeeded.includes(id)))
      setBatchError(failures.join('；'))
    }
  }

  const summaryMeta = summary
    ? `商家钱包 ${formatMoney(summary.merchantWalletCents)} · 骑手钱包 ${formatMoney(summary.riderWalletCents)} · 待清账骑手报酬 ${formatMoney(summary.pendingRiderFeeCents)} · 已登记线下清账 ${formatMoney(summary.paidOfflineCents)} · 平台配送费收入 ${formatMoney(summary.platformFeeIncomeCents)} · 已取消 ${summary.cancelled} 单`
    : ''

  return <div>
    <PageHeader
      eyebrow="履约服务"
      title="配送与结算"
      description="订单、派单与资金全部来自真实后台：状态筛选、分页与写操作都以服务端数据为准。"
    />

    <div className="preview-subtabs" role="tablist" aria-label="配送模块">
      {([['overview', '订单与概览'], ['dispatch', '派单管理'], ['funds', '资金与结算'], ['fee', '费率配置']] as const).map(([key, label]) => <button
        key={key}
        type="button"
        role="tab"
        aria-selected={view === key}
        className={view === key ? 'active' : ''}
        onClick={() => setView(key)}
      >{label}</button>)}
    </div>

    {view === 'overview' && <><section className="preview-stat-grid">
      <StatCard
        label="正在配送"
        value={summary ? summary.inProgress : '—'}
        meta={summary ? `履约中 · 待付配送费 ${summary.pendingFeePayment} 单` : '实时履约中'}
        tone="accent"
        unit="单"
      />
      <StatCard
        label="等待接单"
        value={summary ? summary.readyForClaim : '—'}
        meta={summary ? `可抢单 · 今日订单 ${summary.todayOrders} 单` : '需要分配或广播'}
        tone="amber"
        unit="单"
      />
      <StatCard
        label="今日已完成"
        value={summary ? summary.todayDelivered : '—'}
        meta={summary ? `今日已送达 · 累计 ${summary.delivered} 单` : '可进入结算核对'}
        tone="mint"
        unit="单"
      />
      <StatCard
        label="今日配送费"
        value={summary ? formatMoney(summary.todayDeliveryFeeCents) : '—'}
        meta={summary ? `平台抽成 ${formatMoney(summary.todayPlatformFeeCents)}` : '含配送费与平台抽成'}
        tone="ink"
      />
    </section>

    {summaryMeta && <div className="preview-toolbar"><span className="preview-toolbar__hint">{summaryMeta}</span></div>}
    {summaryState.error && <div className="preview-form-error" role="alert">{summaryState.error}</div>}

    <Panel
      eyebrow="ORDER FLOW"
      title="订单工作队列"
      description="点击订单查看完整履约与资金信息；状态筛选与服务端分页来自真实接口，搜索仅过滤当前页。"
      actions={<SearchBox value={query} onChange={value => { setQuery(value); resetSelection() }} placeholder="搜索订单号、商家/骑手 ID、地址" />}
    >
      <div className="preview-toolbar">
        <SelectField label="状态" value={statusFilter} onChange={event => { setStatusFilter(event.target.value); setPage(1); resetSelection() }}>
          {STATUS_FILTERS.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
        </SelectField>
        <span className="preview-toolbar__hint">
          {ordersState.loading ? '正在加载…' : `共 ${ordersState.data?.total ?? 0} 条订单${query.trim() ? ` · 本页匹配 ${visibleOrders.length} 条` : ''}`}
        </span>
      </div>

      {ordersState.error && <div className="preview-form-error" role="alert">{ordersState.error}</div>}

      {selectedIds.length > 0 && <div className="preview-batch-bar">
        <span>已选 {selectedIds.length} 笔订单</span>
        <div className="preview-row-actions">
          <Button size="sm" variant="primary" onClick={() => openBatch('reassign', selectedIds)}>批量改派骑手</Button>
          <Button size="sm" variant="danger" onClick={() => openBatch('cancel', selectedIds)}>批量取消</Button>
          <Button size="sm" onClick={resetSelection}>清除选择</Button>
        </div>
      </div>}

      {!ordersState.loading && visibleOrders.length === 0
        ? <EmptyState
          title="没有匹配的订单"
          description={query.trim() ? '当前页没有匹配项，可翻页或清空搜索。' : '调整状态筛选，或查看全部状态。'}
          action={<Button onClick={() => { setStatusFilter(''); setQuery(''); setPage(1); resetSelection() }}>清除筛选</Button>}
        />
        : <div className="preview-table-wrap">
          <table className="preview-table preview-table--delivery" aria-label="配送订单列表">
            <thead><tr>
              <th className="preview-table__check">
                <input
                  type="checkbox"
                  aria-label="选择本页可操作订单"
                  checked={allOnPageSelected}
                  onChange={event => setSelectedIds(event.target.checked
                    ? selectableOnPage.map(order => order.deliveryOrderPublicId)
                    : [])}
                />
              </th>
              <th>订单</th><th>取送到</th><th>骑手</th><th>金额</th><th>状态</th><th>进入时间</th><th />
            </tr></thead>
            <tbody>{visibleOrders.map(order => {
              const id = order.deliveryOrderPublicId
              const canReassign = reassignableOrderStatuses.has(order.status)
              const canCancel = cancellableOrderStatuses.has(order.status)
              return <tr key={id}>
                <td className="preview-table__check">
                  {selectableOrderStatuses.has(order.status) && <input
                    type="checkbox"
                    aria-label={`选择订单 ${id}`}
                    checked={selectedIds.includes(id)}
                    onChange={event => toggleRow(id, event.target.checked)}
                  />}
                </td>
                <td><button type="button" className="preview-table__primary" onClick={() => void openDetail(id)}>{shortId(id)}<small>商家 #{order.merchantUserId}{order.merchantRequestId ? ` · 请求号 ${shortId(order.merchantRequestId, 6, 4)}` : ''}</small></button></td>
                <td><strong>{order.pickupAddress}</strong><span>→ {order.deliveryAddress}</span></td>
                <td>{order.riderUserId ? `骑手 #${order.riderUserId}` : <StatusPill tone="warning">待分配</StatusPill>}</td>
                <td><strong>{formatMoney(order.deliveryFeeCents)}</strong><span className="preview-table__fee">骑手 {formatMoney(order.riderFeeCents)} · 平台 {formatMoney(order.platformCutCents)}</span></td>
                <td><StatusPill tone={orderStatusTone(order.status)}>{orderStatusLabel(order.status)}</StatusPill></td>
                <td>{formatDateTime(order.createdAt)}</td>
                <td><div className="preview-row-actions">
                  <Button size="sm" variant="ghost" onClick={() => void openDetail(id)}>详情</Button>
                  {canReassign && <Button size="sm" variant="ghost" onClick={() => openBatch('reassign', [id])}>改派</Button>}
                  {canCancel && <Button size="sm" variant="danger" onClick={() => openBatch('cancel', [id])}>取消</Button>}
                </div></td>
              </tr>
            })}</tbody>
          </table>
        </div>}
      <Pagination
        page={ordersState.data?.page ?? page}
        pageSize={ordersState.data?.pageSize ?? DELIVERY_PAGE_SIZE}
        total={ordersState.data?.total ?? 0}
        onChange={next => { setPage(next); resetSelection() }}
        label="配送订单分页"
      />
    </Panel>
    </>}

    {view === 'dispatch' && <DispatchSection value={dispatchView} onChange={setDispatchView} />}
    {view === 'funds' && <FundsSection value={fundView} onChange={setFundView} />}
    {view === 'fee' && <FeeSection value={feeView} onChange={setFeeView} />}

    <Modal
      open={Boolean(batchAction)}
      title={batchAction === 'reassign' ? `改派 ${selectedIds.length} 笔订单` : `取消 ${selectedIds.length} 笔订单`}
      description={batchAction === 'reassign' ? '改派后原骑手不再负责该订单，新骑手会收到提醒。仅已接单及之后的在途订单可以改派。' : '取消后订单不再履约，用户会看到取消原因。已取货、配送中的订单需走退款流程，不能直接取消。'}
      onClose={() => { if (!batchBusy) { setBatchAction(null); setBatchError('') } }}
      footer={<>
        <Button disabled={batchBusy} onClick={() => { setBatchAction(null); setBatchError('') }}>返回</Button>
        <Button
          variant={batchAction === 'cancel' ? 'danger' : 'primary'}
          disabled={batchBusy}
          onClick={() => void submitBatch()}
        >{batchBusy ? '处理中…' : batchAction === 'reassign' ? '确认改派' : '确认取消'}</Button>
      </>}
    >
      <div className="preview-form-stack">
        {batchAction === 'reassign' && <SelectField label="新骑手" value={batchRider} onChange={event => { setBatchRider(event.target.value); setBatchError('') }}>
          {candidates.length === 0
            ? <option value="">当前没有接单中的骑手</option>
            : candidates.map(rider => <option key={rider.riderUserId} value={rider.riderUserId}>
              {riderName(rider.riderUserId, rider.realName)}（在途 {rider.inFlight} 单）
            </option>)}
        </SelectField>}
        <Field
          label={batchAction === 'reassign' ? '改派原因' : '取消原因'}
          required
          value={batchReason}
          onChange={event => { setBatchReason(event.target.value); setBatchError('') }}
          placeholder={batchAction === 'reassign' ? '例如：原骑手车辆故障' : '例如：用户要求取消'}
        />
        {batchError && <div className="preview-form-error">{batchError}</div>}
        <div className="preview-batch-list">{selectedIds.map(id => <span key={id}>{shortId(id)}</span>)}</div>
      </div>
    </Modal>

    <Drawer
      open={Boolean(detailId)}
      onClose={closeDetail}
      title={detailId ? `配送单 ${detailId}` : '订单详情'}
      description={detail ? `${orderStatusLabel(detail.order.status)} · ${merchantName(detail.order.merchantUserId)} · ${formatDateTime(detail.order.createdAt)}` : undefined}
      footer={detail && <>
        <Button onClick={closeDetail}>关闭</Button>
        {reassignableOrderStatuses.has(detail.order.status) && <Button variant="primary" onClick={() => {
          closeDetail()
          openBatch('reassign', [detail.order.deliveryOrderPublicId])
        }}>改派骑手</Button>}
        {cancellableOrderStatuses.has(detail.order.status) && <Button variant="danger" onClick={() => {
          closeDetail()
          openBatch('cancel', [detail.order.deliveryOrderPublicId])
        }}>取消订单</Button>}
      </>}
    >
      {detailLoading && !detail && <p className="preview-toolbar__hint">正在加载订单资金明细…</p>}
      {detailError && <div className="preview-form-error" role="alert">{detailError}</div>}
      {detail && <OrderDetailContent value={detail} />}
    </Drawer>
  </div>
}

function OrderDetailContent({ value }: { value: OrderFunds }) {
  const order = value.order
  const stages = STAGES
    .map(([label, key]) => [label, order[key] as string | null | undefined] as const)
    .filter(([, at]) => Boolean(at))
  const processDone = [true, Boolean(order.claimedAt), Boolean(order.pickedUpAt), Boolean(order.deliveringAt), Boolean(order.deliveredAt)]
  return <div className="preview-detail-stack">
    <div className="preview-detail-banner">
      <div><span>当前状态</span><strong>{orderStatusLabel(order.status)}</strong></div>
      <StatusPill tone={orderStatusTone(order.status)}>{orderStatusLabel(order.status)}</StatusPill>
    </div>
    <div className="preview-route-card">
      <div><span>取</span><strong>{order.pickupAddress}</strong><small>取件地点</small></div>
      <i />
      <div><span>送</span><strong>{order.deliveryAddress}</strong><small>送达地点</small></div>
    </div>
    <div className="preview-kv-grid">
      <div><span>所属商家</span><strong>{merchantName(order.merchantUserId)}</strong></div>
      <div><span>当前骑手</span><strong>{order.riderUserId ? `骑手 #${order.riderUserId}` : '待分配'}</strong></div>
      <div><span>配送费</span><strong>{formatMoney(order.deliveryFeeCents)}</strong></div>
      <div><span>骑手报酬</span><strong>{formatMoney(order.riderFeeCents)}</strong></div>
      <div><span>平台抽成</span><strong>{formatMoney(order.platformCutCents)}</strong></div>
      <div><span>商家请求号</span><strong>{order.merchantRequestId || '—'}</strong></div>
      <div><span>取货联系人</span><strong>{order.pickupContactName || '—'}</strong></div>
      <div><span>取货电话</span><strong>{order.pickupContactPhone || '—'}</strong></div>
      <div><span>收货人</span><strong>{order.recipientName || '—'}</strong></div>
      <div><span>收货电话</span><strong>{order.recipientPhone || '—'}</strong></div>
      <div><span>创建时间</span><strong>{formatDateTime(order.createdAt)}</strong></div>
      <div><span>更新时间</span><strong>{formatDateTime(order.updatedAt)}</strong></div>
    </div>
    {order.cancelReason && <div className="preview-note-block"><span>取消原因</span><p>{order.cancelReason}</p></div>}
    <div className="preview-note-block">
      <span>用户备注</span>
      <p>{order.remark || '无'}</p>
    </div>
    <div className="preview-kv-grid">
      <div><span>支付</span><strong>{value.payment ? `${paymentStatusLabel(value.payment.status)} · ${formatMoney(value.payment.amountCents)}` : '—'}</strong></div>
      <div><span>支付时间</span><strong>{formatDateTime(value.payment?.paidAt)}</strong></div>
      <div><span>结算</span><strong>{value.settlement ? `${settlementStatusLabel(value.settlement.status)} · ${formatMoney(value.settlement.amountCents)}` : '—'}</strong></div>
      <div><span>结算渠道</span><strong>{value.settlement ? settlementSourceLabel(value.settlement.source) : '—'}</strong></div>
      <div><span>退款</span><strong>{value.refund ? `${refundStatusLabel(value.refund.status)} · ${formatMoney(value.refund.amountCents)}` : '—'}</strong></div>
      <div><span>退款失败原因</span><strong>{value.refund?.failureReason || '—'}</strong></div>
    </div>
    <div className="preview-note-block">
      <span>阶段时间</span>
      {stages.length === 0
        ? <p>暂无节点时间</p>
        : stages.map(([label, at]) => <p key={label}>{label} · {formatDateTime(at)}</p>)}
    </div>
    {order.status !== 'cancelled' && <div className="preview-process-track">
      {['进入队列', '接单', '取货', '配送中', '已送达'].map((step, index) => <div key={step} className={processDone[index] ? 'is-done' : ''}>
        <span>{processDone[index] ? '✓' : index + 1}</span><small>{step}</small>
      </div>)}
    </div>}
  </div>
}
