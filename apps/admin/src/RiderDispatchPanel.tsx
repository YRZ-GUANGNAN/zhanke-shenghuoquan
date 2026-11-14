import { useCallback, useEffect, useState } from 'react'
import {
  DeliveryAPIError,
  deliveryAdminAPI,
  type DeliveryOrder,
  type Page,
  type RiderAdminRow
} from './services/delivery-admin'
import { AdminBadge, AdminPagination, AdminPanel, statusLabel, useAdminDialog } from './components'

const pageSize = 20
const emptyPage = <T,>(): Page<T> => ({ items: [], page: 1, pageSize, total: 0 })
const conflictMessage = '数据已被他人修改，请刷新最新数据后重试'
const money = (value?: number) => `¥${((value || 0) / 100).toFixed(2)}`
const time = (value?: string | null) => value ? new Date(value).toLocaleString('zh-CN') : '—'
const errorMessage = (reason: unknown) => reason instanceof Error ? reason.message : String(reason)
const isConflict = (reason: unknown) => reason instanceof DeliveryAPIError && reason.status === 409

const riderStatusLabels: Record<RiderAdminRow['status'], string> = {
  active: '接单中',
  resting: '休息中',
  suspended: '已封禁'
}
const riderStatusTones: Record<RiderAdminRow['status'], 'success' | 'neutral' | 'danger'> = {
  active: 'success',
  resting: 'neutral',
  suspended: 'danger'
}

export function RiderDispatchPanel() {
  const dialog = useAdminDialog()
  const [data, setData] = useState<Page<RiderAdminRow>>(emptyPage())
  const [status, setStatus] = useState('')
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState('')
  const [focusRider, setFocusRider] = useState<RiderAdminRow | null>(null)

  const load = useCallback(async () => {
    setError('')
    try { setData(await deliveryAdminAPI.riders({ status, query: query.trim(), page, pageSize })) }
    catch (reason) { setError(errorMessage(reason)) }
  }, [status, query, page])
  useEffect(() => { void load() }, [load])

  const changeStatus = async (row: RiderAdminRow, next: RiderAdminRow['status'], title: string, description: string) => {
    const reason = await dialog.prompt({
      title,
      description,
      inputLabel: '操作原因',
      placeholder: '例如：骑手申请休息一天 / 多次配送超时',
      requiredMessage: '请填写操作原因，操作会记录在案',
      confirmLabel: `确认${title.replace(/^改|^恢复|^封禁|^解封/, '') || title}`,
      tone: next === 'suspended' ? 'danger' : 'default'
    })
    if (!reason?.trim()) return
    setBusy(String(row.riderUserId)); setError(''); setNotice('')
    try {
      await deliveryAdminAPI.setRiderStatus(row.riderUserId, next, row.rowVersion, reason.trim())
      setNotice(`骑手 ${row.realName || `#${row.riderUserId}`} 已${title.replace(/^确认/, '')}`)
      if (focusRider?.riderUserId === row.riderUserId) setFocusRider(null)
      await load()
    } catch (cause) {
      setError(isConflict(cause) ? conflictMessage : errorMessage(cause))
    } finally { setBusy('') }
  }

  const activeCandidates = data.items.filter(row => row.status === 'active')

  return <div>
    <AdminPanel title="骑手派单" description="查看骑手在途与今日数据，调整接单状态，并把在途订单改派给其他骑手。暂停或封禁不影响骑手已经接下的订单。">
      <div className="delivery-controls">
        <label>状态
          <select value={status} onChange={event => { setPage(1); setStatus(event.target.value) }}>
            <option value="">全部</option>
            <option value="active">接单中</option>
            <option value="resting">休息中</option>
            <option value="suspended">已封禁</option>
          </select>
        </label>
        <label>搜索<input value={query} onChange={event => { setPage(1); setQuery(event.target.value) }} placeholder="姓名或手机号" /></label>
        <button type="button" onClick={() => void load()}>刷新</button>
      </div>
      {error && <p className="review-feedback review-feedback--error" role="alert">{error}</p>}
      {notice && <p className="review-feedback review-feedback--success" role="status">{notice}</p>}
      <div className="table-scroll"><table>
        <thead><tr><th>骑手</th><th>状态</th><th>在途</th><th>今日单量</th><th>今日收入</th><th>更新时间</th><th>操作</th></tr></thead>
        <tbody>{data.items.map(row => <tr key={row.riderUserId}>
          <td><strong>{row.realName || `骑手 #${row.riderUserId}`}</strong><span>{row.phone || '未登记手机号'} · #{row.riderUserId}</span></td>
          <td>
            <AdminBadge tone={riderStatusTones[row.status]}>{riderStatusLabels[row.status] || row.status}</AdminBadge>
            <span>{row.dutyState === 'on_duty' ? '值班中' : '未值班'}</span>
          </td>
          <td>{row.inFlight}</td>
          <td>{row.todayOrders}</td>
          <td>{money(row.todayIncomeCents)}</td>
          <td>{time(row.updatedAt)}</td>
          <td><div className="delivery-row-actions">
            <button type="button" onClick={() => setFocusRider(focusRider?.riderUserId === row.riderUserId ? null : row)}>
              {focusRider?.riderUserId === row.riderUserId ? '收起订单' : '查看订单'}
            </button>
            {row.status === 'active' && <button type="button" disabled={busy === String(row.riderUserId)} onClick={() => void changeStatus(row, 'resting', '改为休息中', `确认让骑手 ${row.realName || `#${row.riderUserId}`} 暂停接单？已接订单不受影响。`)}>暂停接单</button>}
            {row.status === 'resting' && <button type="button" disabled={busy === String(row.riderUserId)} onClick={() => void changeStatus(row, 'active', '恢复接单', `确认让骑手 ${row.realName || `#${row.riderUserId}`} 恢复接单？`)}>恢复接单</button>}
            {row.status !== 'suspended' && <button type="button" className="danger" disabled={busy === String(row.riderUserId)} onClick={() => void changeStatus(row, 'suspended', '封禁骑手', `封禁后骑手 ${row.realName || `#${row.riderUserId}`} 不能再接新单，已接订单不受影响。解封后恢复为休息中。`)}>封禁</button>}
            {row.status === 'suspended' && <button type="button" disabled={busy === String(row.riderUserId)} onClick={() => void changeStatus(row, 'resting', '解封骑手', `确认解除骑手 ${row.realName || `#${row.riderUserId}`} 的封禁？解封后为休息中，需要再次恢复接单。`)}>解封</button>}
          </div></td>
        </tr>)}</tbody>
      </table></div>
      {data.items.length === 0 && <p className="placeholder">当前没有匹配的骑手</p>}
      <AdminPagination page={data.page} pageSize={data.pageSize} total={data.total} onPageChange={setPage} />
    </AdminPanel>
    {focusRider && <RiderOrdersPanel
      rider={focusRider}
      candidates={activeCandidates.filter(row => row.riderUserId !== focusRider.riderUserId)}
      onChanged={() => void load()}
    />}
  </div>
}

function RiderOrdersPanel({ rider, candidates, onChanged }: { rider: RiderAdminRow; candidates: RiderAdminRow[]; onChanged: () => void }) {
  const dialog = useAdminDialog()
  const [orders, setOrders] = useState<DeliveryOrder[]>([])
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState('')

  const load = useCallback(async () => {
    setError('')
    try { setOrders(await deliveryAdminAPI.riderOrders(rider.riderUserId)) }
    catch (cause) { setError(errorMessage(cause)) }
  }, [rider.riderUserId])
  useEffect(() => { void load() }, [load])

  const reassign = async (order: DeliveryOrder, targetRiderUserId: number) => {
    const target = candidates.find(row => row.riderUserId === targetRiderUserId)
    if (!target) { setError('请选择接单中的骑手'); return }
    const reason = await dialog.prompt({
      title: '改派订单',
      description: `把配送单 ${order.deliveryOrderPublicId} 从 ${rider.realName || `#${rider.riderUserId}`} 改派给 ${target.realName || `#${target.riderUserId}`}。原骑手不再负责该订单。`,
      inputLabel: '改派原因',
      placeholder: '例如：原骑手车辆故障',
      requiredMessage: '请填写改派原因',
      confirmLabel: '确认改派'
    })
    if (!reason?.trim()) return
    setBusy(order.deliveryOrderPublicId); setError(''); setNotice('')
    try {
      await deliveryAdminAPI.reassignOrder(order.deliveryOrderPublicId, target.riderUserId, order.rowVersion, reason.trim())
      setNotice(`配送单 ${order.deliveryOrderPublicId} 已改派`)
      await load(); onChanged()
    } catch (cause) { setError(isConflict(cause) ? conflictMessage : errorMessage(cause)) }
    finally { setBusy('') }
  }

  const cancel = async (order: DeliveryOrder) => {
    const reason = await dialog.prompt({
      title: '取消订单',
      description: `取消配送单 ${order.deliveryOrderPublicId} 后订单不再履约。已取货的订单不能直接取消，需要走退款流程。`,
      inputLabel: '取消原因',
      placeholder: '例如：用户要求取消',
      requiredMessage: '请填写取消原因',
      confirmLabel: '确认取消',
      tone: 'danger'
    })
    if (!reason?.trim()) return
    setBusy(order.deliveryOrderPublicId); setError(''); setNotice('')
    try {
      await deliveryAdminAPI.cancelOrder(order.deliveryOrderPublicId, order.rowVersion, reason.trim())
      setNotice(`配送单 ${order.deliveryOrderPublicId} 已取消`)
      await load(); onChanged()
    } catch (cause) { setError(isConflict(cause) ? conflictMessage : errorMessage(cause)) }
    finally { setBusy('') }
  }

  return <AdminPanel title={`${rider.realName || `骑手 #${rider.riderUserId}`} 的在途订单`} actions={<button type="button" className="delivery-secondary" onClick={() => void load()}>刷新</button>}>
    {error && <p className="review-feedback review-feedback--error" role="alert">{error}</p>}
    {notice && <p className="review-feedback review-feedback--success" role="status">{notice}</p>}
    {orders.length === 0
      ? <p className="placeholder">该骑手没有在途订单</p>
      : <div className="table-scroll"><table>
        <thead><tr><th>配送单</th><th>取送到</th><th>金额</th><th>状态</th><th>进入时间</th><th>操作</th></tr></thead>
        <tbody>{orders.map(order => <tr key={order.deliveryOrderPublicId}>
          <td>{order.deliveryOrderPublicId}<span>商家 #{order.merchantUserId}</span></td>
          <td>{order.pickupAddress}<span>→ {order.deliveryAddress}</span></td>
          <td>{money(order.deliveryFeeCents)}<span>骑手 {money(order.riderFeeCents)}</span></td>
          <td><AdminBadge tone="info">{statusLabel(order.status)}</AdminBadge></td>
          <td>{time(order.readyForClaimAt || order.claimedAt || order.createdAt)}</td>
          <td><div className="delivery-row-actions">
            {candidates.length > 0 && <select
              aria-label={`为 ${order.deliveryOrderPublicId} 选择新骑手`}
              defaultValue=""
              disabled={busy === order.deliveryOrderPublicId}
              onChange={event => {
                const value = Number(event.target.value)
                event.target.value = ''
                if (value) void reassign(order, value)
              }}
            >
              <option value="">改派给…</option>
              {candidates.map(row => <option key={row.riderUserId} value={row.riderUserId}>{row.realName || `#${row.riderUserId}`}（在途 {row.inFlight} 单）</option>)}
            </select>}
            <button type="button" className="danger" disabled={busy === order.deliveryOrderPublicId} onClick={() => void cancel(order)}>取消订单</button>
          </div></td>
        </tr>)}</tbody>
      </table></div>}
  </AdminPanel>
}
