import { useEffect, useMemo, useState } from 'react'
import { formatMoney, formatTime, type DeliveryOrder, type RiderItem } from '../data'
import { usePreviewStore } from '../store'
import { Button, EmptyState, Field, Modal, Panel, SearchBox, SelectField, StatusPill, usePreviewToast } from '../ui'

type DispatchTab = 'riders' | 'merchants'

export function DispatchSection({ value, onChange }: { value: DispatchTab; onChange: (value: DispatchTab) => void }) {
  return <>
    <div className="preview-subtabs" role="tablist" aria-label="派单模块">
      {([['riders', '骑手派单'], ['merchants', '商家订单']] as const).map(([key, label]) => <button
        key={key}
        type="button"
        role="tab"
        aria-selected={value === key}
        className={value === key ? 'active' : ''}
        onClick={() => onChange(key)}
      >{label}</button>)}
    </div>
    {value === 'riders' ? <RidersPanel /> : <MerchantsPanel />}
  </>
}

function RidersPanel() {
  const riders = usePreviewStore(state => state.riders)
  const orders = usePreviewStore(state => state.orders)
  const setRiderState = usePreviewStore(state => state.setRiderState)
  const [focusRider, setFocusRider] = useState<string>('')
  const { notify } = usePreviewToast()

  const activeOrders = useMemo(() => orders.filter(order =>
    order.rider === focusRider && order.status !== '已完成' && order.status !== '已取消'
  ), [orders, focusRider])

  return <>
    <Panel eyebrow="RIDER DESK" title="骑手状态" description="暂停接单不影响已接订单；封禁后该骑手不能再接新单。">
      <div className="preview-table-wrap"><table className="preview-table" aria-label="骑手列表">
        <thead><tr><th>骑手</th><th>状态</th><th>在途</th><th>今日单量</th><th>今日收入</th><th>加入时间</th><th /></tr></thead>
        <tbody>{riders.map(rider => <tr key={rider.id}>
          <td><strong>{rider.name}</strong><span>{rider.phone} · {rider.id}</span></td>
          <td><StatusPill tone={rider.state === '接单中' ? 'success' : rider.state === '休息中' ? 'neutral' : 'danger'}>{rider.state}</StatusPill></td>
          <td>{rider.inFlight}</td>
          <td>{rider.todayOrders}</td>
          <td><strong>{formatMoney(rider.todayIncomeCents)}</strong></td>
          <td>{rider.joinedAt}</td>
          <td><div className="preview-row-actions">
            <Button size="sm" variant="ghost" onClick={() => setFocusRider(focusRider === rider.name ? '' : rider.name)}>{focusRider === rider.name ? '收起订单' : '查看订单'}</Button>
            {rider.state === '接单中' && <Button size="sm" onClick={() => { setRiderState(rider.id, '休息中'); notify(`${rider.name} 已暂停接单`, 'info') }}>暂停接单</Button>}
            {rider.state === '休息中' && <Button size="sm" variant="primary" onClick={() => { setRiderState(rider.id, '接单中'); notify(`${rider.name} 已恢复接单`) }}>恢复接单</Button>}
            {rider.state !== '已封禁' && <Button size="sm" variant="danger" onClick={() => { setRiderState(rider.id, '已封禁'); notify(`${rider.name} 已封禁`, 'danger') }}>封禁</Button>}
            {rider.state === '已封禁' && <Button size="sm" onClick={() => { setRiderState(rider.id, '休息中'); notify(`${rider.name} 已解封`, 'info') }}>解封</Button>}
          </div></td>
        </tr>)}</tbody>
      </table></div>
    </Panel>

    {focusRider && <Panel
      eyebrow="RIDER ORDERS"
      title={`${focusRider} 的在途订单`}
      description="可以单笔或批量把订单转给其他骑手，转出后原骑手不再负责。"
    >
      {activeOrders.length === 0
        ? <EmptyState title="该骑手没有在途订单" description="已完成或已取消的订单不在此列出。" />
        : <BulkOrderTable orders={activeOrders} />}
    </Panel>}
  </>
}

function MerchantsPanel() {
  const orders = usePreviewStore(state => state.orders)
  const [query, setQuery] = useState('')
  const [focusMerchant, setFocusMerchant] = useState<string>('')

  const merchants = useMemo(() => {
    const map = new Map<string, { name: string; orders: DeliveryOrder[] }>()
    for (const order of orders) {
      const name = order.merchant || '未归属商家'
      if (!map.has(name)) map.set(name, { name, orders: [] })
      map.get(name)!.orders.push(order)
    }
    return [...map.values()].filter(item => !query.trim() || item.name.includes(query.trim()))
  }, [orders, query])

  const merchantOrders = orders.filter(order => order.merchant === focusMerchant && order.status !== '已完成' && order.status !== '已取消')

  return <>
    <Panel
      eyebrow="MERCHANT ORDERS"
      title="商家订单"
      description="按商家查看订单，可单笔或批量改派骑手、取消订单。"
      actions={<SearchBox value={query} onChange={setQuery} placeholder="搜索商家名称" />}
    >
      <div className="preview-table-wrap"><table className="preview-table" aria-label="商家订单汇总">
        <thead><tr><th>商家</th><th>订单数</th><th>在途</th><th>已完成</th><th>成交额</th><th /></tr></thead>
        <tbody>{merchants.map(merchant => {
          const inFlight = merchant.orders.filter(order => order.status === '待接单' || order.status === '配送中' || order.status === '待确认').length
          const done = merchant.orders.filter(order => order.status === '已完成')
          return <tr key={merchant.name}>
            <td><strong>{merchant.name}</strong></td>
            <td>{merchant.orders.length}</td>
            <td>{inFlight}</td>
            <td>{done.length}</td>
            <td><strong>{formatMoney(done.reduce((sum, order) => sum + order.amountCents, 0))}</strong></td>
            <td><Button size="sm" variant="ghost" onClick={() => setFocusMerchant(focusMerchant === merchant.name ? '' : merchant.name)}>{focusMerchant === merchant.name ? '收起订单' : '查看订单'}</Button></td>
          </tr>
        })}</tbody>
      </table></div>
    </Panel>

    {focusMerchant && <Panel
      eyebrow="MERCHANT ORDER LIST"
      title={`${focusMerchant} 的订单`}
      description="在途订单可以改派给其他骑手或直接取消。"
    >
      {merchantOrders.length === 0
        ? <EmptyState title="该商家没有在途订单" description="已完成或已取消的订单不在此列出。" />
        : <BulkOrderTable orders={merchantOrders} />}
    </Panel>}
  </>
}

function BulkOrderTable({ orders }: { orders: DeliveryOrder[] }) {
  const riders = usePreviewStore(state => state.riders)
  const reassignOrders = usePreviewStore(state => state.reassignOrders)
  const cancelOrders = usePreviewStore(state => state.cancelOrders)
  const [selected, setSelected] = useState<string[]>([])
  // 切换骑手或商家后，之前选中的订单可能已不在列表里，需要剔除。
  useEffect(() => {
    setSelected(current => current.filter(id => orders.some(order => order.id === id)))
  }, [orders])
  const [mode, setMode] = useState<'reassign' | 'cancel' | null>(null)
  const [rider, setRider] = useState('')
  const [reason, setReason] = useState('')
  const [error, setError] = useState('')
  const { notify } = usePreviewToast()

  const submit = () => {
    if (mode === 'reassign') {
      if (!rider) { setError('请选择新骑手'); return }
      reassignOrders(selected, rider)
      notify(`已把 ${selected.length} 笔订单转给 ${rider}`)
    } else if (mode === 'cancel') {
      if (!reason.trim()) { setError('请填写取消原因'); return }
      cancelOrders(selected, reason.trim())
      notify(`已取消 ${selected.length} 笔订单`, 'info')
    }
    setSelected([])
    setMode(null)
    setError('')
  }

  return <>
    {selected.length > 0 && <div className="preview-batch-bar">
      <span>已选 {selected.length} 笔</span>
      <div className="preview-row-actions">
        <Button size="sm" variant="primary" onClick={() => { setMode('reassign'); setRider(riders.find(item => item.state === '接单中')?.name || ''); setError('') }}>转给其他骑手</Button>
        <Button size="sm" variant="danger" onClick={() => { setMode('cancel'); setReason(''); setError('') }}>取消订单</Button>
        <Button size="sm" onClick={() => setSelected([])}>清除</Button>
      </div>
    </div>}
    <div className="preview-table-wrap"><table className="preview-table" aria-label="订单列表">
      <thead><tr>
        <th className="preview-table__check"><input
          type="checkbox"
          aria-label="选择全部"
          checked={orders.length > 0 && orders.every(order => selected.includes(order.id))}
          onChange={event => setSelected(event.target.checked ? orders.map(order => order.id) : [])}
        /></th>
        <th>订单</th><th>取送到</th><th>骑手</th><th>状态</th><th>金额</th><th>进入时间</th>
      </tr></thead>
      <tbody>{orders.map(order => <tr key={order.id}>
        <td className="preview-table__check"><input
          type="checkbox"
          aria-label={`选择 ${order.id}`}
          checked={selected.includes(order.id)}
          onChange={event => setSelected(current => event.target.checked ? [...current, order.id] : current.filter(id => id !== order.id))}
        /></td>
        <td><strong>{order.id}</strong><span>{order.customer}</span></td>
        <td>{order.pickup}<span>→ {order.dropoff}</span></td>
        <td>{order.rider}</td>
        <td><StatusPill>{order.status}</StatusPill></td>
        <td>{formatMoney(order.amountCents)}</td>
        <td>{formatTime(order.createdAt)}</td>
      </tr>)}</tbody>
    </table></div>

    <Modal
      open={Boolean(mode)}
      title={mode === 'reassign' ? `转单给其他骑手（${selected.length} 笔）` : `取消订单（${selected.length} 笔）`}
      description={mode === 'reassign' ? '转出后原骑手不再负责，新骑手会收到提醒。' : '取消后订单不再履约，用户会看到取消原因。'}
      onClose={() => setMode(null)}
      footer={<>
        <Button onClick={() => setMode(null)}>返回</Button>
        <Button variant={mode === 'cancel' ? 'danger' : 'primary'} onClick={submit}>{mode === 'reassign' ? '确认转单' : '确认取消'}</Button>
      </>}
    >
      <div className="preview-form-stack">
        {mode === 'reassign'
          ? <SelectField label="新骑手" value={rider} onChange={event => { setRider(event.target.value); setError('') }}>
            <option value="">请选择</option>
            {riders.filter(item => item.state === '接单中').map(item => <option key={item.id} value={item.name}>{item.name}（在途 {item.inFlight} 单）</option>)}
          </SelectField>
          : <Field label="取消原因" required value={reason} onChange={event => { setReason(event.target.value); setError('') }} placeholder="例如：用户要求取消" />}
        {error && <div className="preview-form-error">{error}</div>}
      </div>
    </Modal>
  </>
}
