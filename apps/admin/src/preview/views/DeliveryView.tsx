import { useMemo, useState } from 'react'
import { formatMoney, formatTime, type DeliveryOrder } from '../data'
import { usePreviewStore } from '../store'
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
  SearchBox,
  Segmented,
  SelectField,
  Pagination,
  StatCard,
  StatusPill,
  usePagedList,
  usePreviewToast
} from '../ui'

type StatusFilter = '全部' | DeliveryOrder['status']

const nextStatus: Partial<Record<DeliveryOrder['status'], DeliveryOrder['status']>> = {
  待接单: '配送中',
  配送中: '待确认',
  待确认: '已完成'
}

export function DeliveryView() {
  const orders = usePreviewStore(state => state.orders)
  const updateOrderStatus = usePreviewStore(state => state.updateOrderStatus)
  const assignRider = usePreviewStore(state => state.assignRider)
  const [status, setStatus] = useState<StatusFilter>('全部')
  const [query, setQuery] = useState('')
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [riderDraft, setRiderDraft] = useState('周师傅')
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [batchAction, setBatchAction] = useState<'reassign' | 'cancel' | null>(null)
  const [batchRider, setBatchRider] = useState('周师傅')
  const [batchReason, setBatchReason] = useState('')
  const [batchError, setBatchError] = useState('')
  const riders = usePreviewStore(state => state.riders)
  const reassignOrders = usePreviewStore(state => state.reassignOrders)
  const cancelOrders = usePreviewStore(state => state.cancelOrders)
  const [view, setView] = useState<'overview' | 'dispatch' | 'funds' | 'fee'>('overview')
  const [dispatchView, setDispatchView] = useState<'riders' | 'merchants'>('riders')
  const [fundView, setFundView] = useState<FundView>('merchant')
  const [feeView, setFeeView] = useState<FeeView>('default')
  const { notify } = usePreviewToast()

  const filtered = useMemo(() => orders.filter(order => {
    const matchesStatus = status === '全部' || order.status === status
    const normalized = query.trim().toLowerCase()
    const matchesQuery = !normalized || [order.id, order.customer, order.rider, order.pickup, order.dropoff]
      .some(value => value.toLowerCase().includes(normalized))
    return matchesStatus && matchesQuery
  }), [orders, query, status])

  const paged = usePagedList(filtered, 8)
  const selected = orders.find(order => order.id === selectedId) || null
  const totals = useMemo(() => ({
    active: orders.filter(order => order.status === '配送中').length,
    waiting: orders.filter(order => order.status === '待接单').length,
    complete: orders.filter(order => order.status === '已完成').length,
    amount: orders.filter(order => order.status === '已完成').reduce((total, order) => total + order.amountCents, 0)
  }), [orders])

  const advance = (order: DeliveryOrder) => {
    const next = nextStatus[order.status]
    if (!next) {
      notify('该订单当前不能继续推进', 'info')
      return
    }
    if (order.rider === '待分配' && next === '配送中') {
      notify('请先分配骑手，再推进订单', 'danger')
      return
    }
    updateOrderStatus(order.id, next)
    notify(`订单 ${order.id} 已更新为“${next}”`)
  }

  return <div>
    <PageHeader
      eyebrow="履约服务"
      title="配送与结算"
      description="从订单进入、骑手分配到完成确认，每一步都给出明确状态和下一步动作。"
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
      <StatCard label="正在配送" value={totals.active} meta="实时履约中" tone="accent" unit="单" />
      <StatCard label="等待接单" value={totals.waiting} meta="需要分配或广播" tone="amber" unit="单" />
      <StatCard label="今日已完成" value={totals.complete} meta="可进入结算核对" tone="mint" unit="单" />
      <StatCard label="已完成交易额" value={formatMoney(totals.amount)} meta="含配送费与商品金额" tone="ink" />
    </section>

    <Panel
      eyebrow="ORDER FLOW"
      title="订单工作队列"
      description="点击订单查看完整履约信息；状态推进只修改当前本地预览数据。"
      actions={<SearchBox value={query} onChange={value => { setQuery(value); setSelectedIds([]) }} placeholder="搜索订单号或骑手" />}
    >
      <div className="preview-toolbar">
        <Segmented
          value={status}
          onChange={value => { setStatus(value); setSelectedIds([]) }}
          options={[
            { value: '全部', label: '全部', badge: orders.length },
            { value: '待接单', label: '待接单', badge: orders.filter(item => item.status === '待接单').length },
            { value: '配送中', label: '配送中', badge: orders.filter(item => item.status === '配送中').length },
            { value: '待确认', label: '待确认' },
            { value: '已完成', label: '已完成' }
          ]}
        />
        <span className="preview-toolbar__hint">显示 {filtered.length} 条订单</span>
      </div>

      {selectedIds.length > 0 && <div className="preview-batch-bar">
        <span>已选 {selectedIds.length} 笔订单</span>
        <div className="preview-row-actions">
          <Button size="sm" variant="primary" onClick={() => { setBatchRider('周师傅'); setBatchAction('reassign') }}>批量改派骑手</Button>
          <Button size="sm" variant="danger" onClick={() => { setBatchReason(''); setBatchError(''); setBatchAction('cancel') }}>批量取消</Button>
          <Button size="sm" onClick={() => setSelectedIds([])}>清除选择</Button>
        </div>
      </div>}

      {filtered.length === 0
        ? <EmptyState title="没有匹配的订单" description="调整筛选条件，或查看全部订单。" action={<Button onClick={() => { setStatus('全部'); setQuery('') }}>清除筛选</Button>} />
        : <div className="preview-table-wrap">
          <table className="preview-table preview-table--delivery" aria-label="配送订单列表">
            <thead><tr>
              <th className="preview-table__check">
                <input
                  type="checkbox"
                  aria-label="选择本页全部订单"
                  checked={paged.slice.length > 0 && paged.slice.every(order => selectedIds.includes(order.id))}
                  onChange={event => setSelectedIds(event.target.checked ? paged.slice.map(order => order.id) : [])}
                />
              </th>
              <th>订单</th><th>取送到</th><th>骑手</th><th>金额</th><th>状态</th><th>进入时间</th><th />
            </tr></thead>
            <tbody>{paged.slice.map(order => <tr key={order.id}>
              <td className="preview-table__check">
                <input
                  type="checkbox"
                  aria-label={`选择订单 ${order.id}`}
                  checked={selectedIds.includes(order.id)}
                  onChange={event => setSelectedIds(current => event.target.checked ? [...current, order.id] : current.filter(id => id !== order.id))}
                />
              </td>
              <td><button type="button" className="preview-table__primary" onClick={() => { setSelectedId(order.id); setRiderDraft(order.rider === '待分配' ? '周师傅' : order.rider) }}>{order.id}<small>{order.customer}</small></button></td>
              <td><strong>{order.pickup}</strong><span>→ {order.dropoff}</span></td>
              <td>{order.rider === '待分配' ? <StatusPill tone="warning">待分配</StatusPill> : order.rider}</td>
              <td><strong>{formatMoney(order.amountCents)}</strong><span className="preview-table__fee">平台抽成 {formatMoney(order.platformFeeCents)}</span></td>
              <td><StatusPill>{order.status}</StatusPill></td>
              <td>{formatTime(order.createdAt)}</td>
              <td><div className="preview-row-actions">
                <Button size="sm" variant="ghost" onClick={() => { setSelectedId(order.id); setRiderDraft(order.rider === '待分配' ? '周师傅' : order.rider) }}>详情</Button>
                {(order.status === '待接单' || order.status === '配送中') && <Button size="sm" variant="ghost" onClick={() => { setSelectedIds([order.id]); setBatchRider(order.rider === '待分配' ? '周师傅' : order.rider); setBatchAction('reassign') }}>改派</Button>}
                {order.status !== '已完成' && order.status !== '已取消' && <Button size="sm" variant="danger" onClick={() => { setSelectedIds([order.id]); setBatchReason(''); setBatchError(''); setBatchAction('cancel') }}>取消</Button>}
              </div></td>
            </tr>)}</tbody>
          </table>
        </div>}
      <Pagination page={paged.page} pageSize={paged.pageSize} total={paged.total} onChange={page => { paged.setPage(page); setSelectedIds([]) }} label="配送订单分页" />
    </Panel>
    </>}

    {view === 'dispatch' && <DispatchSection value={dispatchView} onChange={setDispatchView} />}
    {view === 'funds' && <FundsSection value={fundView} onChange={setFundView} />}
    {view === 'fee' && <FeeSection value={feeView} onChange={setFeeView} />}

    <Modal
      open={Boolean(batchAction)}
      title={batchAction === 'reassign' ? `改派 ${selectedIds.length} 笔订单` : `取消 ${selectedIds.length} 笔订单`}
      description={batchAction === 'reassign' ? '改派后原骑手不再负责该订单，新骑手会收到提醒。' : '取消后订单不再履约，用户会看到取消原因。'}
      onClose={() => setBatchAction(null)}
      footer={<>
        <Button onClick={() => setBatchAction(null)}>返回</Button>
        <Button
          variant={batchAction === 'cancel' ? 'danger' : 'primary'}
          onClick={() => {
            if (batchAction === 'reassign') {
              reassignOrders(selectedIds, batchRider)
              notify(`已把 ${selectedIds.length} 笔订单改派给 ${batchRider}`)
            } else {
              if (!batchReason.trim()) { setBatchError('请填写取消原因'); return }
              cancelOrders(selectedIds, batchReason.trim())
              notify(`已取消 ${selectedIds.length} 笔订单`, 'info')
            }
            setSelectedIds([])
            setBatchAction(null)
            setBatchError('')
          }}
        >{batchAction === 'reassign' ? '确认改派' : '确认取消'}</Button>
      </>}
    >
      <div className="preview-form-stack">
        {batchAction === 'reassign'
          ? <SelectField label="新骑手" value={batchRider} onChange={event => setBatchRider(event.target.value)}>
            {riders.filter(rider => rider.state === '接单中').map(rider => <option key={rider.id} value={rider.name}>{rider.name}（在途 {rider.inFlight} 单）</option>)}
          </SelectField>
          : <Field label="取消原因" required value={batchReason} onChange={event => { setBatchReason(event.target.value); setBatchError('') }} placeholder="例如：用户要求取消" />}
        {batchError && <div className="preview-form-error">{batchError}</div>}
        <div className="preview-batch-list">{selectedIds.map(id => <span key={id}>{id}</span>)}</div>
      </div>
    </Modal>

    <Drawer
      open={Boolean(selected)}
      onClose={() => setSelectedId(null)}
      title={selected?.id || '订单详情'}
      description={selected ? `${selected.customer} · ${formatTime(selected.createdAt)}` : undefined}
      footer={selected && <>
        <Button onClick={() => setSelectedId(null)}>关闭</Button>
        {selected.status === '待接单' && <Button variant="primary" onClick={() => {
          assignRider(selected.id, riderDraft)
          notify(`已分配骑手 ${riderDraft}`)
        }}>确认分配</Button>}
        {selected.status !== '待接单' && selected.status !== '已完成' && selected.status !== '已取消' && <Button variant="primary" onClick={() => advance(selected)}>推进到“{nextStatus[selected.status]}”</Button>}
        {selected.status !== '已完成' && selected.status !== '已取消' && <Button onClick={() => { setSelectedId(null); setSelectedIds([selected.id]); setBatchRider('周师傅'); setBatchAction('reassign') }}>改派骑手</Button>}
        {selected.status !== '已完成' && selected.status !== '已取消' && <Button variant="danger" onClick={() => { setSelectedId(null); setSelectedIds([selected.id]); setBatchReason(''); setBatchError(''); setBatchAction('cancel') }}>取消订单</Button>}
      </>}
    >
      {selected && <div className="preview-detail-stack">
        <div className="preview-detail-banner">
          <div><span>当前状态</span><strong>{selected.status}</strong></div>
          <StatusPill>{selected.status}</StatusPill>
        </div>
        <div className="preview-route-card">
          <div><span>取</span><strong>{selected.pickup}</strong><small>取件地点</small></div>
          <i />
          <div><span>送</span><strong>{selected.dropoff}</strong><small>送达地点</small></div>
        </div>
        <div className="preview-kv-grid">
          <div><span>下单用户</span><strong>{selected.customer}</strong></div>
          <div><span>收货电话</span><strong>{selected.recipientPhone || '—'}</strong></div>
          <div><span>所属商家</span><strong>{selected.merchant || '—'}</strong></div>
          <div><span>订单来源</span><strong>{selected.source || '—'}</strong></div>
          <div><span>当前骑手</span><strong>{selected.rider}</strong></div>
          <div><span>订单金额</span><strong>{formatMoney(selected.amountCents)}</strong></div>
          <div><span>商品金额</span><strong>{formatMoney(selected.goodsCents ?? 0)}</strong></div>
          <div><span>平台服务收入</span><strong>{formatMoney(selected.platformFeeCents)}</strong></div>
          <div><span>骑手报酬</span><strong>{formatMoney(selected.riderFeeCents ?? 0)}</strong></div>
        </div>
        {selected.cancelReason && <div className="preview-note-block"><span>取消原因</span><p>{selected.cancelReason}</p></div>}
        {selected.status === '待接单' && <SelectField label="选择骑手" value={riderDraft} onChange={event => setRiderDraft(event.target.value)}>
          <option>周师傅</option>
          <option>黄师傅</option>
          <option>李师傅</option>
        </SelectField>}
        <div className="preview-note-block">
          <span>用户备注</span>
          <p>{selected.note}</p>
        </div>
        <div className="preview-note-block">
          <span>时间线</span>
          {(selected.timeline || []).map(event => <p key={`${event.label}-${event.at}`}>{event.label} · {formatTime(event.at)}</p>)}
        </div>
        <div className="preview-process-track">
          {['进入队列', '接单', '配送中', '待确认', '已完成'].map((step, index) => {
            const order = ['待接单', '配送中', '待确认', '已完成']
            const activeIndex = order.indexOf(selected.status)
            const done = selected.status === '已完成' || index <= activeIndex
            return <div key={step} className={done ? 'is-done' : ''}><span>{done ? '✓' : index + 1}</span><small>{step}</small></div>
          })}
        </div>
      </div>}
    </Drawer>
  </div>
}
