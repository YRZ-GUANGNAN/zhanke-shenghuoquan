import { useMemo, useState } from 'react'
import { formatMoney, formatTime, type DeliveryOrder } from '../data'
import { usePreviewStore } from '../store'
import {
  Button,
  Drawer,
  EmptyState,
  PageHeader,
  Panel,
  SearchBox,
  Segmented,
  SelectField,
  StatCard,
  StatusPill,
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
  const { notify } = usePreviewToast()

  const filtered = useMemo(() => orders.filter(order => {
    const matchesStatus = status === '全部' || order.status === status
    const normalized = query.trim().toLowerCase()
    const matchesQuery = !normalized || [order.id, order.customer, order.rider, order.pickup, order.dropoff]
      .some(value => value.toLowerCase().includes(normalized))
    return matchesStatus && matchesQuery
  }), [orders, query, status])

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
      actions={<Button variant="primary" onClick={() => setStatus('待接单')}>查看待分配订单</Button>}
    />

    <section className="preview-stat-grid">
      <StatCard label="正在配送" value={totals.active} meta="实时履约中" tone="accent" marker="配" />
      <StatCard label="等待接单" value={totals.waiting} meta="需要分配或广播" tone="amber" marker="待" />
      <StatCard label="今日已完成" value={totals.complete} meta="可进入结算核对" tone="mint" marker="✓" />
      <StatCard label="已完成交易额" value={formatMoney(totals.amount)} meta="含配送费与商品金额" tone="ink" marker="¥" />
    </section>

    <Panel
      eyebrow="ORDER FLOW"
      title="订单工作队列"
      description="点击订单查看完整履约信息；状态推进只修改当前本地预览数据。"
      actions={<SearchBox value={query} onChange={setQuery} placeholder="搜索订单号、用户、骑手或地点" />}
    >
      <div className="preview-toolbar">
        <Segmented
          value={status}
          onChange={setStatus}
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

      {filtered.length === 0
        ? <EmptyState title="没有匹配的订单" description="调整筛选条件，或查看全部订单。" action={<Button onClick={() => { setStatus('全部'); setQuery('') }}>清除筛选</Button>} />
        : <div className="preview-table-wrap">
          <table className="preview-table preview-table--delivery">
            <thead><tr>
              <th>订单</th><th>取送到</th><th>骑手</th><th>金额</th><th>状态</th><th>进入时间</th><th />
            </tr></thead>
            <tbody>{filtered.map(order => <tr key={order.id}>
              <td><button type="button" className="preview-table__primary" onClick={() => { setSelectedId(order.id); setRiderDraft(order.rider === '待分配' ? '周师傅' : order.rider) }}>{order.id}<small>{order.customer}</small></button></td>
              <td><strong>{order.pickup}</strong><span>→ {order.dropoff}</span></td>
              <td>{order.rider === '待分配' ? <StatusPill tone="warning">待分配</StatusPill> : order.rider}</td>
              <td><strong>{formatMoney(order.amountCents)}</strong><span>平台 {formatMoney(order.platformFeeCents)}</span></td>
              <td><StatusPill>{order.status}</StatusPill></td>
              <td>{formatTime(order.createdAt)}</td>
              <td><Button size="sm" variant="ghost" onClick={() => { setSelectedId(order.id); setRiderDraft(order.rider === '待分配' ? '周师傅' : order.rider) }}>处理</Button></td>
            </tr>)}</tbody>
          </table>
        </div>}
    </Panel>

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
          <div><span>订单金额</span><strong>{formatMoney(selected.amountCents)}</strong></div>
          <div><span>平台服务收入</span><strong>{formatMoney(selected.platformFeeCents)}</strong></div>
          <div><span>当前骑手</span><strong>{selected.rider}</strong></div>
        </div>
        {selected.status === '待接单' && <SelectField label="选择骑手" value={riderDraft} onChange={event => setRiderDraft(event.target.value)}>
          <option>周师傅</option>
          <option>黄师傅</option>
          <option>李师傅</option>
        </SelectField>}
        <div className="preview-note-block">
          <span>用户备注</span>
          <p>{selected.note}</p>
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
