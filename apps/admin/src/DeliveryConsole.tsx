import { useCallback, useEffect, useState, type ReactNode } from 'react'
import {
  DeliveryAPIError,
  deliveryAdminAPI,
  type DeliveryOrder,
  type DeliveryQuery,
  type DeliverySummary,
  type FeeConfig,
  type MerchantFeeOverride,
  type MerchantLedger,
  type MerchantWallet,
  type OrderFunds,
  type Page,
  type Refund,
  type RiderEarning,
  type RiderLedger,
  type RiderPayout,
  type RiderPayoutResult,
  type RiderWallet
} from './services/delivery-admin'

type View = 'summary' | 'orders' | 'fee' | 'overrides' | 'merchant-wallets' | 'rider-wallets' | 'earnings' | 'refunds'

const pageSize = 20
const emptyPage = <T,>(): Page<T> => ({ items: [], page: 1, pageSize, total: 0 })
const conflictMessage = '数据已被他人修改，请刷新最新数据后重试'
const money = (value?: number) => `¥${((value || 0) / 100).toFixed(2)}`
const time = (value?: string | null) => value ? new Date(value).toLocaleString('zh-CN') : '—'
const errorMessage = (reason: unknown) => reason instanceof Error ? reason.message : String(reason)
const isConflict = (reason: unknown) => reason instanceof DeliveryAPIError && reason.status === 409

const statusOptions = [
  'pending_fee_payment',
  'ready_for_claim',
  'claimed',
  'to_pickup',
  'picked_up',
  'delivering',
  'delivered',
  'cancelled',
  'payment_failed'
]

export function DeliveryConsole() {
  const [view, setView] = useState<View>('summary')
  const tabs: Array<[View, string]> = [
    ['summary', '配送总览'],
    ['orders', '订单管理'],
    ['fee', '配送费配置'],
    ['overrides', '商家特殊价'],
    ['merchant-wallets', '商家钱包'],
    ['rider-wallets', '骑手钱包'],
    ['earnings', '报酬清账'],
    ['refunds', '退款记录']
  ]
  return <div className="delivery-console">
    <section className="panel delivery-tabs" aria-label="配送后台模块">
      {tabs.map(([key, label]) => <button key={key} className={view === key ? 'active' : ''} onClick={() => setView(key)}>{label}</button>)}
    </section>
    {view === 'summary' && <SummaryPanel />}
    {view === 'orders' && <OrdersPanel />}
    {view === 'fee' && <FeeConfigPanel />}
    {view === 'overrides' && <MerchantOverridesPanel />}
    {view === 'merchant-wallets' && <MerchantWalletsPanel />}
    {view === 'rider-wallets' && <RiderWalletsPanel />}
    {view === 'earnings' && <EarningsPanel />}
    {view === 'refunds' && <RefundsPanel />}
  </div>
}

function Panel({ title, actions, children }: { title: string; actions?: ReactNode; children: ReactNode }) {
  return <section className="panel delivery-panel"><header className="delivery-header"><h2>{title}</h2>{actions}</header>{children}</section>
}

function ErrorNotice({ error, conflict, refresh }: { error: string; conflict?: boolean; refresh: () => void }) {
  if (!error) return null
  return <div className={`delivery-notice ${conflict ? 'delivery-notice--conflict' : ''}`} role="alert">
    <strong>{conflict ? conflictMessage : error}</strong>
    {conflict && <button onClick={refresh}>刷新最新数据</button>}
  </div>
}

function Badge({ value }: { value: string }) {
  return <span className={`delivery-badge delivery-badge--${value}`}>{value}</span>
}

function Fact({ label, value }: { label: string; value: ReactNode }) {
  return <div className="delivery-fact"><span>{label}</span><strong>{value}</strong></div>
}

function Pagination({ value, setPage }: { value: Page<unknown>; setPage: (page: number) => void }) {
  const pages = Math.max(1, Math.ceil(value.total / value.pageSize))
  return <div className="delivery-pagination">
    <button disabled={value.page <= 1} onClick={() => setPage(value.page - 1)}>上一页</button>
    <span>第 {value.page} / {pages} 页，共 {value.total} 条</span>
    <button disabled={value.page >= pages} onClick={() => setPage(value.page + 1)}>下一页</button>
  </div>
}

function Detail({ title, close, children }: { title: string; close: () => void; children: ReactNode }) {
  return <aside className="delivery-detail"><header><h3>{title}</h3><button onClick={close}>关闭</button></header>{children}</aside>
}

function SummaryPanel() {
  const [value, setValue] = useState<DeliverySummary | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try { setValue(await deliveryAdminAPI.summary()) } catch (reason) { setError(errorMessage(reason)) } finally { setLoading(false) }
  }, [])
  useEffect(() => { void load() }, [load])
  const metrics: Array<[string, string]> = value ? [
    ['订单总数', String(value.orders)],
    ['今日订单', String(value.todayOrders)],
    ['待付配送费', String(value.pendingFeePayment)],
    ['可抢订单', String(value.readyForClaim)],
    ['履约中', String(value.inProgress)],
    ['已完成', String(value.delivered)],
    ['平台收入', money(value.platformFeeIncomeCents)],
    ['待清账骑手报酬', money(value.pendingRiderFeeCents)],
    ['已登记线下清账', money(value.paidOfflineCents)],
    ['商家钱包余额', money(value.merchantWalletCents)],
    ['骑手钱包余额', money(value.riderWalletCents)]
  ] : []
  return <Panel title="配送运营总览" actions={<button className="delivery-secondary" onClick={() => void load()} disabled={loading}>{loading ? '加载中…' : '刷新'}</button>}>
    <ErrorNotice error={error} refresh={() => void load()} />
    {!value ? <p>加载中…</p> : <section className="delivery-summary-grid">{metrics.map(([label, metric]) => <article key={label}><span>{label}</span><strong>{metric}</strong></article>)}</section>}
  </Panel>
}

function OrdersPanel() {
  const [data, setData] = useState<Page<DeliveryOrder>>(emptyPage())
  const [detail, setDetail] = useState<OrderFunds | null>(null)
  const [status, setStatus] = useState('')
  const [merchantUserId, setMerchantUserId] = useState('')
  const [riderUserId, setRiderUserId] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [page, setPage] = useState(1)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const query = useCallback((): DeliveryQuery => ({
    status,
    merchantUserId,
    riderUserId,
    from: localToRFC3339(from),
    to: localToRFC3339(to),
    page,
    pageSize
  }), [status, merchantUserId, riderUserId, from, to, page])
  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try { setData(await deliveryAdminAPI.orders(query())) } catch (reason) { setError(errorMessage(reason)) } finally { setLoading(false) }
  }, [query])
  useEffect(() => { void load() }, [load])
  const inspect = async (id: string) => {
    setError('')
    try { setDetail(await deliveryAdminAPI.orderFunds(id)) } catch (reason) { setError(errorMessage(reason)) }
  }
  const clear = () => { setStatus(''); setMerchantUserId(''); setRiderUserId(''); setFrom(''); setTo(''); setPage(1) }
  return <Panel title="配送订单管理" actions={<button className="delivery-secondary" disabled={loading} onClick={() => void load()}>{loading ? '加载中…' : '刷新'}</button>}>
    <div className="delivery-controls">
      <label>状态<select value={status} onChange={event => { setPage(1); setStatus(event.target.value) }}><option value="">全部</option>{statusOptions.map(item => <option key={item}>{item}</option>)}</select></label>
      <label>商家用户 ID<input value={merchantUserId} inputMode="numeric" onChange={event => { setPage(1); setMerchantUserId(event.target.value) }} /></label>
      <label>骑手用户 ID<input value={riderUserId} inputMode="numeric" onChange={event => { setPage(1); setRiderUserId(event.target.value) }} /></label>
      <label>开始时间<input type="datetime-local" value={from} onChange={event => { setPage(1); setFrom(event.target.value) }} /></label>
      <label>结束时间<input type="datetime-local" value={to} onChange={event => { setPage(1); setTo(event.target.value) }} /></label>
      <button onClick={() => void load()} disabled={loading}>查询</button>
      <button className="delivery-secondary" onClick={clear}>清空</button>
    </div>
    <ErrorNotice error={error} refresh={() => void load()} />
    <div className="table-scroll"><table><thead><tr><th>配送单</th><th>商家 / 骑手</th><th>地址</th><th>金额</th><th>状态</th><th>创建时间</th></tr></thead><tbody>{data.items.map(row => <tr key={row.deliveryOrderPublicId}>
      <td><button className="link-button" onClick={() => void inspect(row.deliveryOrderPublicId)}>{short(row.deliveryOrderPublicId)}</button><span>{row.merchantRequestId || '无商家请求号'}</span></td>
      <td>商家 #{row.merchantUserId}<span>骑手 {row.riderUserId ? `#${row.riderUserId}` : '未分配'}</span></td>
      <td>{row.deliveryAddress}<span>{row.recipientName} / {row.recipientPhone}</span></td>
      <td>{money(row.deliveryFeeCents)}<span>骑手 {money(row.riderFeeCents)} / 平台 {money(row.platformCutCents)}</span></td>
      <td><Badge value={row.status} /></td>
      <td>{time(row.createdAt)}</td>
    </tr>)}</tbody></table></div>
    <Pagination value={data} setPage={setPage} />
    {detail && <OrderDetailPanel value={detail} close={() => setDetail(null)} />}
  </Panel>
}

function OrderDetailPanel({ value, close }: { value: OrderFunds; close: () => void }) {
  const order = value.order
  const stages: Array<[string, string | null | undefined]> = [
    ['进入抢单池', order.readyForClaimAt],
    ['已抢单', order.claimedAt],
    ['前往取货', order.toPickupAt],
    ['已取货', order.pickedUpAt],
    ['配送中', order.deliveringAt],
    ['已送达', order.deliveredAt],
    ['支付失败', order.paymentFailedAt],
    ['已取消', order.cancelledAt]
  ]
  return <Detail title={`配送单 ${order.deliveryOrderPublicId}`} close={close}>
    <div className="detail-grid">
      <Fact label="状态" value={<Badge value={order.status} />} />
      <Fact label="商家" value={`#${order.merchantUserId}`} />
      <Fact label="骑手" value={order.riderUserId ? `#${order.riderUserId}` : '未分配'} />
      <Fact label="配送费快照" value={money(order.deliveryFeeCents)} />
      <Fact label="骑手报酬快照" value={money(order.riderFeeCents)} />
      <Fact label="平台抽成快照" value={money(order.platformCutCents)} />
      <Fact label="取货信息" value={`${order.pickupContactName} / ${order.pickupContactPhone} / ${order.pickupAddress}`} />
      <Fact label="收货信息" value={`${order.recipientName} / ${order.recipientPhone} / ${order.deliveryAddress}`} />
      <Fact label="Row Version" value={`v${order.rowVersion}`} />
    </div>
    <p className="delivery-description">{order.remark || '无备注'}</p>
    <h4>资金快照</h4>
    <div className="detail-grid">
      <Fact label="结算" value={value.settlement ? `${value.settlement.status} / ${money(value.settlement.amountCents)}` : '—'} />
      <Fact label="支付" value={value.payment ? `${value.payment.status} / ${money(value.payment.amountCents)}` : '—'} />
      <Fact label="退款" value={value.refund ? `${value.refund.status} / ${money(value.refund.amountCents)}` : '—'} />
    </div>
    <h4>阶段时间</h4>
    <div className="delivery-timeline">{stages.map(([label, at]) => <div key={label}><span>{label}</span><strong>{time(at)}</strong></div>)}</div>
  </Detail>
}

function FeeConfigPanel() {
  const [value, setValue] = useState<FeeConfig | null>(null)
  const [deliveryFee, setDeliveryFee] = useState('')
  const [platformCut, setPlatformCut] = useState('')
  const [error, setError] = useState('')
  const [conflict, setConflict] = useState(false)
  const [loading, setLoading] = useState(false)
  const load = useCallback(async () => {
    setError('')
    setConflict(false)
    try {
      const latest = await deliveryAdminAPI.feeConfig()
      setValue(latest)
      setDeliveryFee(String(latest.deliveryFeeCents))
      setPlatformCut(String(latest.platformCutCents))
    } catch (reason) { setError(errorMessage(reason)) }
  }, [])
  useEffect(() => { void load() }, [load])
  const submit = async () => {
    if (!value) return
    if (!window.confirm('确认更新全局配送费配置？新配送单会使用新配置，旧单快照不会改变。')) return
    setLoading(true); setError(''); setConflict(false)
    try {
      await deliveryAdminAPI.updateFeeConfig(Number(deliveryFee), Number(platformCut), value.rowVersion)
      await load()
    } catch (reason) {
      if (isConflict(reason)) { setConflict(true); setError(conflictMessage) } else setError(errorMessage(reason))
    } finally { setLoading(false) }
  }
  return <Panel title="全局配送费配置">
    <ErrorNotice error={error} conflict={conflict} refresh={() => void load()} />
    {value ? <div className="delivery-form">
      <label>配送费（分）<input type="number" min="1" value={deliveryFee} onChange={event => setDeliveryFee(event.target.value)} /></label>
      <label>平台抽成（分）<input type="number" min="0" value={platformCut} onChange={event => setPlatformCut(event.target.value)} /></label>
      <Fact label="当前 Row Version" value={`v${value.rowVersion}`} />
      <Fact label="更新时间" value={time(value.updatedAt)} />
      <button disabled={loading} onClick={() => void submit()}>{loading ? '保存中…' : '保存配置'}</button>
      <button className="delivery-secondary" disabled={loading} onClick={() => void load()}>刷新</button>
    </div> : <p>加载中…</p>}
  </Panel>
}

function MerchantOverridesPanel() {
  const [data, setData] = useState<Page<MerchantFeeOverride>>(emptyPage())
  const [merchantUserId, setMerchantUserId] = useState('')
  const [status, setStatus] = useState('')
  const [fee, setFee] = useState('')
  const [cut, setCut] = useState('')
  const [expectedRowVersion, setExpectedRowVersion] = useState('')
  const [page, setPage] = useState(1)
  const [error, setError] = useState('')
  const [conflict, setConflict] = useState(false)
  const [busy, setBusy] = useState('')
  const load = useCallback(async () => {
    setError(''); setConflict(false)
    try { setData(await deliveryAdminAPI.merchantFeeOverrides({ merchantUserId, status, page, pageSize })) } catch (reason) { setError(errorMessage(reason)) }
  }, [merchantUserId, status, page])
  useEffect(() => { void load() }, [load])
  const edit = (row: MerchantFeeOverride) => {
    setMerchantUserId(String(row.merchantUserId))
    setFee(String(row.deliveryFeeCents))
    setCut(String(row.platformCutCents))
    setExpectedRowVersion(String(row.rowVersion))
  }
  const upsert = async () => {
    const merchant = Number(merchantUserId)
    if (!merchant || !fee || !cut) { setError('请填写商家用户 ID、配送费和平台抽成'); return }
    if (!window.confirm(`确认保存商家 #${merchant} 的特殊价配置？`)) return
    setBusy('upsert'); setError(''); setConflict(false)
    try {
      await deliveryAdminAPI.upsertMerchantFeeOverride(merchant, Number(fee), Number(cut), expectedRowVersion ? Number(expectedRowVersion) : undefined)
      setExpectedRowVersion('')
      await load()
    } catch (reason) {
      if (isConflict(reason)) { setConflict(true); setError(conflictMessage) } else setError(errorMessage(reason))
    } finally { setBusy('') }
  }
  const disable = async (row: MerchantFeeOverride) => {
    if (!window.confirm(`确认停用商家 #${row.merchantUserId} 的特殊价？停用后新单回到全局配置。`)) return
    setBusy(`disable-${row.merchantUserId}`); setError(''); setConflict(false)
    try {
      await deliveryAdminAPI.disableMerchantFeeOverride(row.merchantUserId, row.rowVersion)
      await load()
    } catch (reason) {
      if (isConflict(reason)) { setConflict(true); setError(conflictMessage) } else setError(errorMessage(reason))
    } finally { setBusy('') }
  }
  return <Panel title="商家特殊配送费 / 抽成">
    <ErrorNotice error={error} conflict={conflict} refresh={() => void load()} />
    <div className="delivery-form delivery-form--wide">
      <label>商家用户 ID<input value={merchantUserId} inputMode="numeric" onChange={event => { setPage(1); setMerchantUserId(event.target.value) }} /></label>
      <label>状态<select value={status} onChange={event => { setPage(1); setStatus(event.target.value) }}><option value="">全部</option><option value="active">active</option><option value="disabled">disabled</option></select></label>
      <label>配送费（分）<input value={fee} inputMode="numeric" onChange={event => setFee(event.target.value)} /></label>
      <label>平台抽成（分）<input value={cut} inputMode="numeric" onChange={event => setCut(event.target.value)} /></label>
      <label>Expected Row Version（更新时必填）<input value={expectedRowVersion} inputMode="numeric" onChange={event => setExpectedRowVersion(event.target.value)} /></label>
      <button disabled={Boolean(busy)} onClick={() => void upsert()}>{busy === 'upsert' ? '保存中…' : '保存 / 新增'}</button>
      <button className="delivery-secondary" onClick={() => { setFee(''); setCut(''); setExpectedRowVersion('') }}>清空编辑</button>
    </div>
    <div className="table-scroll"><table><thead><tr><th>商家</th><th>配送费</th><th>平台抽成</th><th>状态</th><th>版本</th><th>操作</th></tr></thead><tbody>{data.items.map(row => <tr key={row.merchantUserId}>
      <td>#{row.merchantUserId}</td><td>{money(row.deliveryFeeCents)}</td><td>{money(row.platformCutCents)}</td><td><Badge value={row.status} /></td><td>v{row.rowVersion}<span>{time(row.updatedAt)}</span></td>
      <td><div className="delivery-row-actions"><button onClick={() => edit(row)}>编辑</button>{row.status !== 'disabled' && <button className="danger" disabled={busy === `disable-${row.merchantUserId}`} onClick={() => void disable(row)}>{busy === `disable-${row.merchantUserId}` ? '停用中…' : '停用'}</button>}</div></td>
    </tr>)}</tbody></table></div>
    <Pagination value={data} setPage={setPage} />
  </Panel>
}

function MerchantWalletsPanel() {
  const [data, setData] = useState<Page<MerchantWallet>>(emptyPage())
  const [ledger, setLedger] = useState<Page<MerchantLedger> | null>(null)
  const [selected, setSelected] = useState<MerchantWallet | null>(null)
  const [merchantUserId, setMerchantUserId] = useState('')
  const [page, setPage] = useState(1)
  const [error, setError] = useState('')
  const load = useCallback(async () => {
    setError('')
    try { setData(await deliveryAdminAPI.merchantWallets({ merchantUserId, page, pageSize })) } catch (reason) { setError(errorMessage(reason)) }
  }, [merchantUserId, page])
  useEffect(() => { void load() }, [load])
  const inspect = async (row: MerchantWallet) => {
    setSelected(row); setError('')
    try { setLedger(await deliveryAdminAPI.merchantLedger(row.merchantUserId, { page: 1, pageSize: 20 })) } catch (reason) { setError(errorMessage(reason)) }
  }
  return <Panel title="商家钱包">
    <WalletFilter label="商家用户 ID" value={merchantUserId} setValue={value => { setPage(1); setMerchantUserId(value) }} refresh={load} />
    <ErrorNotice error={error} refresh={() => void load()} />
    <div className="table-scroll"><table><thead><tr><th>商家</th><th>钱包</th><th>余额</th><th>版本</th><th>更新时间</th></tr></thead><tbody>{data.items.map(row => <tr key={row.walletPublicId}><td><button className="link-button" onClick={() => void inspect(row)}>#{row.merchantUserId}</button></td><td>{short(row.walletPublicId)}</td><td>{money(row.availableBalanceCents)}</td><td>v{row.rowVersion}</td><td>{time(row.updatedAt)}</td></tr>)}</tbody></table></div>
    <Pagination value={data} setPage={setPage} />
    {selected && ledger && <LedgerDetail title={`商家 #${selected.merchantUserId} 钱包流水`} close={() => { setSelected(null); setLedger(null) }} rows={ledger.items} />}
  </Panel>
}

function RiderWalletsPanel() {
  const [data, setData] = useState<Page<RiderWallet>>(emptyPage())
  const [ledger, setLedger] = useState<Page<RiderLedger> | null>(null)
  const [selected, setSelected] = useState<RiderWallet | null>(null)
  const [riderUserId, setRiderUserId] = useState('')
  const [page, setPage] = useState(1)
  const [error, setError] = useState('')
  const load = useCallback(async () => {
    setError('')
    try { setData(await deliveryAdminAPI.riderWallets({ riderUserId, page, pageSize })) } catch (reason) { setError(errorMessage(reason)) }
  }, [riderUserId, page])
  useEffect(() => { void load() }, [load])
  const inspect = async (row: RiderWallet) => {
    setSelected(row); setError('')
    try { setLedger(await deliveryAdminAPI.riderLedger(row.riderUserId, { page: 1, pageSize: 20 })) } catch (reason) { setError(errorMessage(reason)) }
  }
  return <Panel title="骑手钱包">
    <WalletFilter label="骑手用户 ID" value={riderUserId} setValue={value => { setPage(1); setRiderUserId(value) }} refresh={load} />
    <ErrorNotice error={error} refresh={() => void load()} />
    <div className="table-scroll"><table><thead><tr><th>骑手</th><th>钱包</th><th>余额</th><th>版本</th><th>更新时间</th></tr></thead><tbody>{data.items.map(row => <tr key={row.walletPublicId}><td><button className="link-button" onClick={() => void inspect(row)}>#{row.riderUserId}</button></td><td>{short(row.walletPublicId)}</td><td>{money(row.availableBalanceCents)}</td><td>v{row.rowVersion}</td><td>{time(row.updatedAt)}</td></tr>)}</tbody></table></div>
    <Pagination value={data} setPage={setPage} />
    {selected && ledger && <LedgerDetail title={`骑手 #${selected.riderUserId} 钱包流水`} close={() => { setSelected(null); setLedger(null) }} rows={ledger.items} />}
  </Panel>
}

function WalletFilter({ label, value, setValue, refresh }: { label: string; value: string; setValue: (value: string) => void; refresh: () => Promise<void> }) {
  return <div className="delivery-controls"><label>{label}<input value={value} inputMode="numeric" onChange={event => setValue(event.target.value)} /></label><button onClick={() => void refresh()}>查询</button><button className="delivery-secondary" onClick={() => setValue('')}>清空</button></div>
}

function LedgerDetail({ title, close, rows }: { title: string; close: () => void; rows: Array<MerchantLedger | RiderLedger> }) {
  return <Detail title={title} close={close}>
    <div className="table-scroll"><table><thead><tr><th>流水</th><th>方向/类型</th><th>金额</th><th>余额变化</th><th>引用</th><th>时间</th></tr></thead><tbody>{rows.map(row => <tr key={row.ledgerPublicId}><td>{short(row.ledgerPublicId)}</td><td>{row.direction}<span>{row.entryType}</span></td><td>{money(row.amountCents)}</td><td>{money(row.balanceBeforeCents)} → {money(row.balanceAfterCents)}</td><td>{row.referenceType}<span>{short(row.referencePublicId || '—')}</span></td><td>{time(row.createdAt)}</td></tr>)}</tbody></table></div>
  </Detail>
}

function EarningsPanel() {
  const [data, setData] = useState<Page<RiderEarning>>(emptyPage())
  const [payouts, setPayouts] = useState<Page<RiderPayout>>(emptyPage())
  const [detail, setDetail] = useState<RiderEarning | null>(null)
  const [result, setResult] = useState<RiderPayoutResult | null>(null)
  const [status, setStatus] = useState('pending')
  const [riderUserId, setRiderUserId] = useState('')
  const [page, setPage] = useState(1)
  const [error, setError] = useState('')
  const [conflict, setConflict] = useState(false)
  const [busy, setBusy] = useState('')
  const load = useCallback(async () => {
    setError(''); setConflict(false)
    try {
      setData(await deliveryAdminAPI.riderEarnings({ status, riderUserId, page, pageSize }))
      setPayouts(await deliveryAdminAPI.riderPayouts({ riderUserId, page: 1, pageSize: 10 }))
    } catch (reason) { setError(errorMessage(reason)) }
  }, [status, riderUserId, page])
  useEffect(() => { void load() }, [load])
  const inspect = async (id: string) => {
    setError('')
    try { setDetail(await deliveryAdminAPI.riderEarning(id)) } catch (reason) { setError(errorMessage(reason)) }
  }
  const pay = async (row: RiderEarning) => {
    const note = window.prompt('请输入线下清账备注（例如：微信已付截图编号/日期）')?.trim()
    if (note === undefined) return
    if (!window.confirm(`确认登记骑手 #${row.riderUserId} 的 ${money(row.riderFeeCents)} 已线下支付？\n\n注意：此操作仅登记线下已付，不会真实打款。`)) return
    setBusy(row.earningPublicId); setError(''); setConflict(false)
    try {
      const updated = await deliveryAdminAPI.payOffline(row.earningPublicId, row.rowVersion, newIdempotencyKey(row.earningPublicId), note)
      setResult(updated)
      setDetail(updated.earning)
      await load()
    } catch (reason) {
      if (isConflict(reason)) { setConflict(true); setError(conflictMessage) } else setError(errorMessage(reason))
    } finally { setBusy('') }
  }
  return <Panel title="骑手报酬清账">
    <div className="delivery-controls"><label>状态<select value={status} onChange={event => { setPage(1); setStatus(event.target.value) }}><option value="">全部</option><option value="pending">pending</option><option value="settled">settled</option><option value="paid_offline">paid_offline</option></select></label><label>骑手用户 ID<input value={riderUserId} inputMode="numeric" onChange={event => { setPage(1); setRiderUserId(event.target.value) }} /></label><button onClick={() => void load()}>刷新</button></div>
    <ErrorNotice error={error} conflict={conflict} refresh={() => void load()} />
    <div className="table-scroll"><table><thead><tr><th>报酬</th><th>配送单</th><th>骑手</th><th>金额</th><th>状态</th><th>版本</th><th>操作</th></tr></thead><tbody>{data.items.map(row => <tr key={row.earningPublicId}>
      <td><button className="link-button" onClick={() => void inspect(row.earningPublicId)}>{short(row.earningPublicId)}</button></td>
      <td>{short(row.deliveryOrderPublicId)}</td><td>#{row.riderUserId}</td><td>{money(row.riderFeeCents)}<span>平台抽成 {money(row.platformCutCents)}</span></td><td><Badge value={row.settlementStatus} /></td><td>v{row.rowVersion}</td>
      <td>{row.settlementStatus !== 'paid_offline' ? <button disabled={busy === row.earningPublicId} onClick={() => void pay(row)}>{busy === row.earningPublicId ? '登记中…' : '登记线下已付'}</button> : <span>已清账</span>}</td>
    </tr>)}</tbody></table></div>
    <Pagination value={data} setPage={setPage} />
    {detail && <Detail title={`报酬 ${detail.earningPublicId}`} close={() => { setDetail(null); setResult(null) }}><div className="detail-grid"><Fact label="骑手" value={`#${detail.riderUserId}`} /><Fact label="配送单" value={detail.deliveryOrderPublicId} /><Fact label="骑手报酬" value={money(detail.riderFeeCents)} /><Fact label="状态" value={<Badge value={detail.settlementStatus} />} /><Fact label="Row Version" value={`v${detail.rowVersion}`} /><Fact label="线下支付时间" value={time(detail.paidOfflineAt)} /></div>{result && <p className="delivery-description">后端已返回清账结果：钱包余额 {money(result.wallet.availableBalanceCents)}，流水 {short(result.ledger.ledgerPublicId)}，登记记录 {short(result.payout.payoutPublicId)}。</p>}</Detail>}
    <h3 className="delivery-subtitle">最近清账登记</h3>
    <div className="table-scroll"><table><thead><tr><th>登记</th><th>报酬</th><th>骑手</th><th>金额</th><th>操作人</th><th>时间</th></tr></thead><tbody>{payouts.items.map(row => <tr key={row.payoutPublicId}><td>{short(row.payoutPublicId)}</td><td>{short(row.earningPublicId)}</td><td>#{row.riderUserId}</td><td>{money(row.amountCents)}</td><td>#{row.operatorUserId}</td><td>{time(row.paidOfflineAt)}</td></tr>)}</tbody></table></div>
  </Panel>
}

function RefundsPanel() {
  const [data, setData] = useState<Page<Refund>>(emptyPage())
  const [detail, setDetail] = useState<Refund | null>(null)
  const [status, setStatus] = useState('')
  const [merchantUserId, setMerchantUserId] = useState('')
  const [page, setPage] = useState(1)
  const [error, setError] = useState('')
  const load = useCallback(async () => {
    setError('')
    try { setData(await deliveryAdminAPI.refunds({ status, merchantUserId, page, pageSize })) } catch (reason) { setError(errorMessage(reason)) }
  }, [status, merchantUserId, page])
  useEffect(() => { void load() }, [load])
  const inspect = async (id: string) => {
    setError('')
    try { setDetail(await deliveryAdminAPI.refund(id)) } catch (reason) { setError(errorMessage(reason)) }
  }
  return <Panel title="退款记录（只读）" actions={<button className="delivery-secondary" onClick={() => void load()}>刷新</button>}>
    <div className="delivery-controls"><label>状态<select value={status} onChange={event => { setPage(1); setStatus(event.target.value) }}><option value="">全部</option><option value="pending">pending</option><option value="succeeded">succeeded</option><option value="failed">failed</option><option value="retry_pending">retry_pending</option></select></label><label>商家用户 ID<input value={merchantUserId} inputMode="numeric" onChange={event => { setPage(1); setMerchantUserId(event.target.value) }} /></label></div>
    <ErrorNotice error={error} refresh={() => void load()} />
    <div className="table-scroll"><table><thead><tr><th>退款</th><th>配送单</th><th>商家</th><th>金额</th><th>状态</th><th>时间</th></tr></thead><tbody>{data.items.map(row => <tr key={row.refundPublicId}><td><button className="link-button" onClick={() => void inspect(row.refundPublicId)}>{short(row.refundPublicId)}</button><span>{row.outRefundNo}</span></td><td>{short(row.deliveryOrderPublicId)}</td><td>#{row.merchantUserId}</td><td>{money(row.amountCents)}</td><td><Badge value={row.status} /></td><td>{time(row.updatedAt)}</td></tr>)}</tbody></table></div>
    <Pagination value={data} setPage={setPage} />
    {detail && <Detail title={`退款 ${detail.refundPublicId}`} close={() => setDetail(null)}><div className="detail-grid"><Fact label="状态" value={<Badge value={detail.status} />} /><Fact label="配送单" value={detail.deliveryOrderPublicId} /><Fact label="商家" value={`#${detail.merchantUserId}`} /><Fact label="金额" value={money(detail.amountCents)} /><Fact label="退款单号" value={detail.outRefundNo} /><Fact label="失败原因" value={detail.failureReason || '—'} /><Fact label="Row Version" value={`v${detail.rowVersion}`} /><Fact label="成功时间" value={time(detail.succeededAt)} /></div><p className="delivery-description">D-6c 仅查看退款状态，不在前端发起或伪造退款动作。</p></Detail>}
  </Panel>
}

function localToRFC3339(value: string) {
  return value ? new Date(value).toISOString() : ''
}

function short(value: string) {
  return value.length > 18 ? `${value.slice(0, 8)}…${value.slice(-6)}` : value
}

function newIdempotencyKey(seed: string) {
  const randomUUID = (globalThis.crypto as Crypto & { randomUUID?: () => string } | undefined)?.randomUUID
  const random = randomUUID ? randomUUID.call(globalThis.crypto) : `${Date.now()}-${Math.random().toString(36).slice(2)}`
  return `d6c-${seed}-${random}`.slice(0, 128)
}
