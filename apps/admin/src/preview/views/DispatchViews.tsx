import { useEffect, useState } from 'react'
import {
  deliveryAdminAPI,
  type DeliveryOrder,
  type RiderAdminRow
} from '../../services/delivery-admin'
import {
  DELIVERY_PAGE_SIZE,
  cancellableOrderStatuses,
  conflictMessage,
  deliveryErrorMessage,
  dutyStateLabel,
  errorText,
  formatDateTime,
  formatMoney,
  isDeliveryConflict,
  orderStatusLabel,
  orderStatusTone,
  reassignableOrderStatuses,
  riderName,
  riderStatusLabel,
  riderStatusTone,
  selectableOrderStatuses,
  shortId,
  useDebouncedValue,
  useRemoteData
} from '../delivery-hooks'
import {
  Button,
  EmptyState,
  Field,
  Modal,
  Pagination,
  Panel,
  SearchBox,
  Segmented,
  SelectField,
  StatusPill,
  usePreviewToast
} from '../ui'

type DispatchTab = 'riders' | 'merchants'
type RiderFilter = '全部' | RiderAdminRow['status']

export function DispatchSection({ value, onChange }: { value: DispatchTab; onChange: (value: DispatchTab) => void }) {
  return <>
    <div className="preview-subtabs" role="tablist" aria-label="派单模块">
      {([['riders', '骑手派单'], ['merchants', '订单按商家筛选']] as const).map(([key, label]) => <button
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
  const { notify } = usePreviewToast()
  const [status, setStatus] = useState<RiderFilter>('全部')
  const [search, setSearch] = useState('')
  const debouncedSearch = useDebouncedValue(search)
  const [page, setPage] = useState(1)
  const [focusRider, setFocusRider] = useState<RiderAdminRow | null>(null)

  const [statusTarget, setStatusTarget] = useState<{ rider: RiderAdminRow; next: RiderAdminRow['status'] } | null>(null)
  const [statusReason, setStatusReason] = useState('')
  const [statusError, setStatusError] = useState('')
  const [statusBusy, setStatusBusy] = useState(false)

  const ridersState = useRemoteData(async () => {
    const result = await deliveryAdminAPI.riders({
      status: status === '全部' ? '' : status,
      query: debouncedSearch.trim(),
      page,
      pageSize: DELIVERY_PAGE_SIZE
    })
    if (result.items.length === 0 && result.total > 0 && page > 1) {
      setPage(Math.max(1, Math.ceil(result.total / result.pageSize)))
    }
    return result
  }, [status, debouncedSearch, page])
  // 改派候选人只取接单中的骑手；与真实 riderOrders 改派约束保持一致。
  const candidatesState = useRemoteData(() => deliveryAdminAPI.riders({ status: 'active', pageSize: 100 }), [])

  const riders = ridersState.data?.items ?? []
  const candidates = candidatesState.data?.items ?? []

  const openStatusModal = (rider: RiderAdminRow, next: RiderAdminRow['status']) => {
    setStatusTarget({ rider, next })
    setStatusReason('')
    setStatusError('')
  }

  const statusActionLabel = (target: { rider: RiderAdminRow; next: RiderAdminRow['status'] }) => {
    if (target.next === 'suspended') return '封禁'
    if (target.next === 'active') return '恢复接单'
    return target.rider.status === 'suspended' ? '解封' : '暂停接单'
  }

  const submitStatus = async () => {
    const target = statusTarget
    if (!target) return
    if (!statusReason.trim()) {
      setStatusError('请填写操作原因，操作会记录在案')
      return
    }
    setStatusBusy(true)
    setStatusError('')
    try {
      await deliveryAdminAPI.setRiderStatus(target.rider.riderUserId, target.next, target.rider.rowVersion, statusReason.trim())
      notify(`${riderName(target.rider.riderUserId, target.rider.realName)} 已${statusActionLabel(target)}`)
      setStatusTarget(null)
      setStatusReason('')
      if (focusRider?.riderUserId === target.rider.riderUserId && target.next !== 'active') setFocusRider(null)
      await ridersState.reload()
      await candidatesState.reload()
    } catch (reason) {
      setStatusError(deliveryErrorMessage(reason))
    } finally {
      setStatusBusy(false)
    }
  }

  return <>
    <Panel
      eyebrow="RIDER DESK"
      title="骑手状态"
      description="暂停或封禁后骑手无法登录工作台推进订单，因此有在途订单时需先转出；没有在途订单时方可暂停或封禁。"
      actions={<SearchBox value={search} onChange={value => { setSearch(value); setPage(1) }} placeholder="搜索骑手姓名或手机号" />}
    >
      <div className="preview-toolbar">
        <Segmented
          value={status}
          onChange={value => { setStatus(value); setPage(1) }}
          options={[
            { value: '全部', label: '全部' },
            { value: 'active', label: '接单中' },
            { value: 'resting', label: '休息中' },
            { value: 'suspended', label: '已封禁' }
          ]}
        />
        <span className="preview-toolbar__hint">{ridersState.loading ? '正在加载…' : `共 ${ridersState.data?.total ?? 0} 位骑手`}</span>
      </div>

      {ridersState.error && <div className="preview-form-error" role="alert">{ridersState.error}</div>}

      {!ridersState.loading && riders.length === 0
        ? <EmptyState title="没有匹配的骑手" description="调整筛选条件，或查看全部骑手。" />
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="骑手列表">
          <thead><tr><th>骑手</th><th>状态</th><th>在途</th><th>今日单量</th><th>今日收入</th><th>更新时间</th><th /></tr></thead>
          <tbody>{riders.map(rider => <tr key={rider.riderUserId} className="is-clickable" onClick={() => setFocusRider(rider)}>
            <td><strong>{riderName(rider.riderUserId, rider.realName)}</strong><span>{rider.phone || '未登记手机号'} · 骑手 #{rider.riderUserId}</span></td>
            <td>
              <StatusPill tone={riderStatusTone(rider.status)}>{riderStatusLabel(rider.status)}</StatusPill>
              <span className="preview-table__fee">{dutyStateLabel(rider.dutyState)}</span>
            </td>
            <td>{rider.inFlight}</td>
            <td>{rider.todayOrders}</td>
            <td><strong>{formatMoney(rider.todayIncomeCents)}</strong></td>
            <td>{formatDateTime(rider.updatedAt)}</td>
            <td onClick={event => event.stopPropagation()}><div className="preview-row-actions">
              <Button size="sm" variant="ghost" onClick={() => setFocusRider(focusRider?.riderUserId === rider.riderUserId ? null : rider)}>
                {focusRider?.riderUserId === rider.riderUserId ? '收起订单' : '查看订单'}
              </Button>
              {rider.status === 'active' && <Button size="sm" onClick={() => openStatusModal(rider, 'resting')}>暂停接单</Button>}
              {rider.status === 'resting' && <Button size="sm" variant="primary" onClick={() => openStatusModal(rider, 'active')}>恢复接单</Button>}
              {rider.status !== 'suspended' && <Button size="sm" variant="danger" onClick={() => openStatusModal(rider, 'suspended')}>封禁</Button>}
              {rider.status === 'suspended' && <Button size="sm" onClick={() => openStatusModal(rider, 'resting')}>解封</Button>}
            </div></td>
          </tr>)}</tbody>
        </table></div>}
      <Pagination
        page={ridersState.data?.page ?? page}
        pageSize={ridersState.data?.pageSize ?? DELIVERY_PAGE_SIZE}
        total={ridersState.data?.total ?? 0}
        onChange={next => { setPage(next); setFocusRider(null) }}
        label="骑手列表分页"
      />
    </Panel>

    {focusRider && <RiderOrdersPanel
      rider={focusRider}
      candidates={candidates.filter(row => row.riderUserId !== focusRider.riderUserId)}
      onChanged={async () => {
        await ridersState.reload()
        await candidatesState.reload()
      }}
    />}

    <Modal
      open={Boolean(statusTarget)}
      title={statusTarget ? `${statusActionLabel(statusTarget)} · ${riderName(statusTarget.rider.riderUserId, statusTarget.rider.realName)}` : '调整骑手状态'}
      description={statusTarget ? `骑手 #${statusTarget.rider.riderUserId} · 当前 ${riderStatusLabel(statusTarget.rider.status)}` : undefined}
      onClose={() => { if (!statusBusy) { setStatusTarget(null); setStatusError('') } }}
      footer={<>
        <Button disabled={statusBusy} onClick={() => { setStatusTarget(null); setStatusError('') }}>取消</Button>
        <Button variant={statusTarget?.next === 'suspended' ? 'danger' : 'primary'} disabled={statusBusy} onClick={() => void submitStatus()}>
          {statusBusy ? '提交中…' : `确认${statusTarget ? statusActionLabel(statusTarget) : ''}`}
        </Button>
      </>}
    >
      <div className="preview-form-stack">
        {statusTarget?.next === 'suspended' && <div className="preview-warning-box">
          <strong>封禁后该骑手不能再接新单；若仍有在途订单，需先转出才能封禁</strong>
          <p>解封后恢复为“休息中”，需手动恢复接单。操作原因会记录在案。</p>
        </div>}
        <Field
          label="操作原因"
          required
          value={statusReason}
          onChange={event => { setStatusReason(event.target.value); setStatusError('') }}
          placeholder="例如：骑手申请休息一天 / 多次配送超时"
          hint="原因会随行版本一起提交，便于事后追溯。"
        />
        {statusError && <div className="preview-form-error">{statusError}</div>}
      </div>
    </Modal>
  </>
}

function RiderOrdersPanel({ rider, candidates, onChanged }: {
  rider: RiderAdminRow
  candidates: RiderAdminRow[]
  onChanged: () => void | Promise<void>
}) {
  const ordersState = useRemoteData(() => deliveryAdminAPI.riderOrders(rider.riderUserId), [rider.riderUserId])
  const orders = ordersState.data ?? []
  return <Panel
    eyebrow="RIDER ORDERS"
    title={`${riderName(rider.riderUserId, rider.realName)} 的在途订单`}
    description="可以单笔或批量把订单转给其他接单中的骑手，转出后原骑手不再负责；已取货、配送中的订单不能直接取消。"
  >
    {ordersState.error && <div className="preview-form-error" role="alert">{ordersState.error}</div>}
    {ordersState.loading && !ordersState.data
      ? <p className="preview-toolbar__hint">正在加载在途订单…</p>
      : orders.length === 0
        ? <EmptyState title="该骑手没有在途订单" description="已完成或已取消的订单不在此列出。" />
        : <OrderActionsTable
          orders={orders}
          candidates={candidates}
          onChanged={async () => {
            await ordersState.reload()
            await onChanged()
          }}
        />}
  </Panel>
}

function MerchantsPanel() {
  const [input, setInput] = useState('')
  const [applied, setApplied] = useState<number | null>(null)
  const [inputError, setInputError] = useState('')

  const apply = () => {
    const trimmed = input.trim()
    if (!/^\d+$/.test(trimmed) || Number(trimmed) <= 0) {
      setInputError('请输入有效的商家用户 ID（正整数）')
      return
    }
    setInputError('')
    setApplied(Number(trimmed))
  }

  return <>
    <Panel
      eyebrow="MERCHANT ORDERS"
      title="订单按商家筛选"
      description="真实后台不支持按商家名称聚合；输入商家用户 ID 查看该商家的配送订单，在途订单可以改派给其他接单中骑手或取消。"
    >
      <div className="preview-toolbar">
        <Field
          label="商家用户 ID"
          value={input}
          inputMode="numeric"
          onChange={event => { setInput(event.target.value); setInputError('') }}
          placeholder="例如：1001"
          onKeyDown={event => { if (event.key === 'Enter') apply() }}
        />
        <Button variant="primary" onClick={apply}>查询订单</Button>
        <span className="preview-toolbar__hint">{applied ? `当前筛选：商家 #${applied}` : '输入用户 ID 后查询'}</span>
      </div>
      {inputError && <div className="preview-form-error" role="alert">{inputError}</div>}
      {applied === null
        ? <EmptyState title="输入商家用户 ID 查询订单" description="订单按商家用户 ID 过滤，不做前端聚合汇总。" />
        : <MerchantOrderList key={applied} merchantUserId={applied} />}
    </Panel>
  </>
}

function MerchantOrderList({ merchantUserId }: { merchantUserId: number }) {
  const [page, setPage] = useState(1)
  const ordersState = useRemoteData(async () => {
    const result = await deliveryAdminAPI.orders({ merchantUserId, page, pageSize: DELIVERY_PAGE_SIZE })
    if (result.items.length === 0 && result.total > 0 && page > 1) {
      setPage(Math.max(1, Math.ceil(result.total / result.pageSize)))
    }
    return result
  }, [merchantUserId, page])
  const candidatesState = useRemoteData(() => deliveryAdminAPI.riders({ status: 'active', pageSize: 100 }), [])
  const orders = ordersState.data?.items ?? []
  return <>
    {ordersState.error && <div className="preview-form-error" role="alert">{ordersState.error}</div>}
    <p className="preview-toolbar__hint">
      {ordersState.loading ? '正在加载…' : `商家 #${merchantUserId} · 共 ${ordersState.data?.total ?? 0} 条订单`}
    </p>
    {!ordersState.loading && orders.length === 0
      ? <EmptyState title="该商家没有订单" description="确认商家用户 ID 是否正确，或更换筛选条件。" />
      : <OrderActionsTable
        orders={orders}
        candidates={candidatesState.data?.items ?? []}
        onChanged={() => ordersState.reload()}
      />}
    <Pagination
      page={ordersState.data?.page ?? page}
      pageSize={ordersState.data?.pageSize ?? DELIVERY_PAGE_SIZE}
      total={ordersState.data?.total ?? 0}
      onChange={setPage}
      label="商家订单分页"
    />
  </>
}

// 骑手在途订单与商家订单共用的订单操作表：批量改派/取消逐个调用真实接口并汇总失败项。
function OrderActionsTable({ orders, candidates, onChanged }: {
  orders: DeliveryOrder[]
  candidates: RiderAdminRow[]
  onChanged: () => void | Promise<void>
}) {
  const { notify } = usePreviewToast()
  const [selected, setSelected] = useState<string[]>([])
  // 订单刷新后，之前选中的订单可能已经不在列表里，需要剔除。
  useEffect(() => {
    setSelected(current => current.filter(id => orders.some(order => order.deliveryOrderPublicId === id)))
  }, [orders])
  const [mode, setMode] = useState<'reassign' | 'cancel' | null>(null)
  const [rider, setRider] = useState('')
  const [reason, setReason] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const selectable = orders.filter(order => selectableOrderStatuses.has(order.status))
  const selectedOrders = orders.filter(order => selected.includes(order.deliveryOrderPublicId))
  const allSelected = selectable.length > 0 && selectable.every(order => selected.includes(order.deliveryOrderPublicId))

  const openModal = (next: 'reassign' | 'cancel') => {
    setMode(next)
    setRider(candidates[0] ? String(candidates[0].riderUserId) : '')
    setReason('')
    setError(candidates.length === 0 && next === 'reassign' ? '当前没有接单中的骑手，无法改派' : '')
  }

  const submit = async () => {
    if (!mode) return
    if (!reason.trim()) {
      setError(mode === 'reassign' ? '请填写改派原因' : '请填写取消原因')
      return
    }
    const riderUserId = Number(rider)
    if (mode === 'reassign' && (!riderUserId || !candidates.some(row => row.riderUserId === riderUserId))) {
      setError('请选择接单中的骑手')
      return
    }
    setBusy(true)
    setError('')
    const failures: string[] = []
    let ok = 0
    for (const order of selectedOrders) {
      const id = order.deliveryOrderPublicId
      const applicable = mode === 'reassign'
        ? reassignableOrderStatuses.has(order.status)
        : cancellableOrderStatuses.has(order.status)
      if (!applicable) {
        failures.push(`${id}：当前状态不可${mode === 'reassign' ? '改派' : '取消'}`)
        continue
      }
      try {
        if (mode === 'reassign') {
          await deliveryAdminAPI.reassignOrder(id, riderUserId, order.rowVersion, reason.trim())
        } else {
          await deliveryAdminAPI.cancelOrder(id, order.rowVersion, reason.trim())
        }
        ok += 1
      } catch (cause) {
        failures.push(`${id}：${isDeliveryConflict(cause) ? conflictMessage : errorText(cause)}`)
      }
    }
    setBusy(false)
    await onChanged()
    if (failures.length === 0) {
      notify(mode === 'reassign'
        ? `已把 ${ok} 笔订单转给 ${riderName(riderUserId, candidates.find(row => row.riderUserId === riderUserId)?.realName)}`
        : `已取消 ${ok} 笔订单`, mode === 'reassign' ? 'success' : 'info')
      setSelected([])
      setMode(null)
      setReason('')
    } else {
      notify(`已完成 ${ok} 笔，${failures.length} 笔失败`, 'danger')
      const succeeded = selectedOrders
        .filter(order => !failures.some(failure => failure.startsWith(order.deliveryOrderPublicId)))
        .map(order => order.deliveryOrderPublicId)
      setSelected(current => current.filter(id => !succeeded.includes(id)))
      setError(failures.join('；'))
    }
  }

  return <>
    {selected.length > 0 && <div className="preview-batch-bar">
      <span>已选 {selected.length} 笔</span>
      <div className="preview-row-actions">
        <Button size="sm" variant="primary" onClick={() => openModal('reassign')}>改派给其他骑手</Button>
        <Button size="sm" variant="danger" onClick={() => openModal('cancel')}>取消订单</Button>
        <Button size="sm" onClick={() => setSelected([])}>清除</Button>
      </div>
    </div>}
    <div className="preview-table-wrap"><table className="preview-table" aria-label="订单列表">
      <thead><tr>
        <th className="preview-table__check"><input
          type="checkbox"
          aria-label="选择全部可操作订单"
          checked={allSelected}
          onChange={event => setSelected(event.target.checked
            ? selectable.map(order => order.deliveryOrderPublicId)
            : [])}
        /></th>
        <th>订单</th><th>取送到</th><th>骑手</th><th>状态</th><th>金额</th><th>进入时间</th>
      </tr></thead>
      <tbody>{orders.map(order => {
        const id = order.deliveryOrderPublicId
        return <tr key={id}>
          <td className="preview-table__check">{selectableOrderStatuses.has(order.status) && <input
            type="checkbox"
            aria-label={`选择订单 ${id}`}
            checked={selected.includes(id)}
            onChange={event => setSelected(current => event.target.checked
              ? [...current, id]
              : current.filter(item => item !== id))}
          />}</td>
          <td><strong>{shortId(id)}</strong><span>商家 #{order.merchantUserId}</span></td>
          <td>{order.pickupAddress}<span>→ {order.deliveryAddress}</span></td>
          <td>{order.riderUserId ? `骑手 #${order.riderUserId}` : <StatusPill tone="warning">待分配</StatusPill>}</td>
          <td><StatusPill tone={orderStatusTone(order.status)}>{orderStatusLabel(order.status)}</StatusPill></td>
          <td>{formatMoney(order.deliveryFeeCents)}<span className="preview-table__fee">骑手 {formatMoney(order.riderFeeCents)}</span></td>
          <td>{formatDateTime(order.createdAt)}</td>
        </tr>
      })}</tbody>
    </table></div>

    <Modal
      open={Boolean(mode)}
      title={mode === 'reassign' ? `改派给其他骑手（${selected.length} 笔）` : `取消订单（${selected.length} 笔）`}
      description={mode === 'reassign' ? '改派后原骑手不再负责，新骑手会收到提醒。' : '取消后订单不再履约，用户会看到取消原因。已取货、配送中的订单不能直接取消。'}
      onClose={() => { if (!busy) { setMode(null); setError('') } }}
      footer={<>
        <Button disabled={busy} onClick={() => { setMode(null); setError('') }}>返回</Button>
        <Button variant={mode === 'cancel' ? 'danger' : 'primary'} disabled={busy} onClick={() => void submit()}>
          {busy ? '处理中…' : mode === 'reassign' ? '确认改派' : '确认取消'}
        </Button>
      </>}
    >
      <div className="preview-form-stack">
        {mode === 'reassign' && <SelectField label="新骑手" value={rider} onChange={event => { setRider(event.target.value); setError('') }}>
          {candidates.length === 0
            ? <option value="">当前没有接单中的骑手</option>
            : candidates.map(item => <option key={item.riderUserId} value={item.riderUserId}>
              {riderName(item.riderUserId, item.realName)}（在途 {item.inFlight} 单）
            </option>)}
        </SelectField>}
        <Field
          label={mode === 'reassign' ? '改派原因' : '取消原因'}
          required
          value={reason}
          onChange={event => { setReason(event.target.value); setError('') }}
          placeholder={mode === 'reassign' ? '例如：原骑手车辆故障' : '例如：用户要求取消'}
        />
        {error && <div className="preview-form-error">{error}</div>}
      </div>
    </Modal>
  </>
}
