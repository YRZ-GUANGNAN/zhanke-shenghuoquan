import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import {
  DeliveryAPIError,
  deliveryAdminAPI,
  type DeliveryOrder,
  type DeliveryQuery,
  type DeliverySummary,
  type FundingSourceSummary,
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
  type RiderPayoutQueue,
  type RiderWallet,
  type RiderWithdrawal,
  type RiderWithdrawalProcessing,
  type RiderWithdrawalAction,
  type RiderWithdrawalCommand,
  type RiderAvailabilityConfig
} from './services/delivery-admin'
import { statusLabel, useAdminDialog } from './components'

type View = 'summary' | 'funding-sources' | 'orders' | 'fee' | 'overrides' | 'merchant-wallets' | 'rider-wallets' | 'earnings' | 'payout-queue' | 'withdrawals' | 'refunds'

const pageSize = 20
const emptyPage = <T,>(): Page<T> => ({ items: [], page: 1, pageSize, total: 0 })
const conflictMessage = '数据已被他人修改，请刷新最新数据后重试'
const money = (value?: number) => `¥${((value || 0) / 100).toFixed(2)}`
const time = (value?: string | null) => value ? new Date(value).toLocaleString('zh-CN') : '—'
const errorMessage = (reason: unknown) => reason instanceof Error ? reason.message : String(reason)
const isConflict = (reason: unknown) => reason instanceof DeliveryAPIError && reason.status === 409

const statusOptions: Array<[string, string]> = [
  ['pending_fee_payment', '待付配送费'],
  ['ready_for_claim', '等待接单'],
  ['claimed', '已接单'],
  ['to_pickup', '前往取货'],
  ['picked_up', '已取货'],
  ['delivering', '配送中'],
  ['delivered', '已送达'],
  ['cancelled', '已取消'],
  ['payment_failed', '支付失败']
]

export function DeliveryConsole() {
  const [view, setView] = useState<View>('summary')
  const tabs: Array<[View, string]> = [
    ['summary', '配送概览'],
    ['funding-sources', '资金核对'],
    ['orders', '配送订单'],
    ['fee', '默认运费'],
    ['overrides', '商家专属运费'],
    ['merchant-wallets', '商家钱包'],
    ['rider-wallets', '骑手钱包'],
    ['earnings', '骑手报酬'],
    ['payout-queue', '待打款'],
    ['withdrawals', '骑手提现'],
    ['refunds', '退款记录']
  ]
  return <div className="delivery-console">
    <DeliveryCapabilityNotice />
    <section className="panel delivery-tabs" aria-label="配送后台模块">
      {tabs.map(([key, label]) => <button key={key} className={view === key ? 'active' : ''} onClick={() => setView(key)}>{label}</button>)}
    </section>
    {view === 'summary' && <SummaryPanel />}
    {view === 'funding-sources' && <FundingSourcesPanel />}
    {view === 'orders' && <OrdersPanel />}
    {view === 'fee' && <FeeConfigPanel />}
    {view === 'overrides' && <MerchantOverridesPanel />}
    {view === 'merchant-wallets' && <MerchantWalletsPanel />}
    {view === 'rider-wallets' && <RiderWalletsPanel />}
    {view === 'earnings' && <EarningsPanel />}
    {view === 'payout-queue' && <RiderPayoutQueuePanel />}
    {view === 'withdrawals' && <RiderWithdrawalsPanel />}
    {view === 'refunds' && <RefundsPanel />}
  </div>
}

function DeliveryCapabilityNotice() {
  return <aside className="delivery-capability-notice" role="status">
    <strong>资金说明</strong>
    <span>平台不会自动打款给骑手。骑手申请提现后，需要你线下完成转账，再回到后台登记付款凭证。当前退款与部分订单操作暂未开放，钱包和记录仍可随时查看。</span>
  </aside>
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
  return <span className={`delivery-badge delivery-badge--${value}`}>{statusLabel(value)}</span>
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
  return <Panel title="配送与结算概览" actions={<button className="delivery-secondary" onClick={() => void load()} disabled={loading}>{loading ? '加载中…' : '刷新'}</button>}>
    <ErrorNotice error={error} refresh={() => void load()} />
    {!value ? <p>加载中…</p> : <section className="delivery-summary-grid">{metrics.map(([label, metric]) => <article key={label}><span>{label}</span><strong>{metric}</strong></article>)}</section>}
  </Panel>
}

function FundingSourcesPanel() {
  const [value, setValue] = useState<FundingSourceSummary | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try { setValue(await deliveryAdminAPI.fundingSources()) } catch (reason) { setError(errorMessage(reason)) } finally { setLoading(false) }
  }, [])
  useEffect(() => { void load() }, [load])
  const directionText: Record<string, string> = {
    balanced: '账目一致',
    untracked_wallet_balance: '钱包存在未登记余额',
    source_exceeds_wallet: '来源金额高于钱包余额',
    data_error: '来源数据异常'
  }
  const metrics: Array<[string, string]> = value ? [
    ['钱包余额', money(value.walletBalanceCents)],
    ['可核实来源', money(value.knownAvailableCents)],
    ['其中已核验', money(value.verifiedAvailableCents)],
    ['待核验 / 历史未知', money(value.unverifiedAvailableCents)],
    ['差异', money(value.coverageGapCents)],
    ['支付待核实', String(value.paymentVerificationPending)],
    ['退款待核实', String(value.refundVerificationPending)],
    ['支付错误码', String(value.paymentVerificationErrorCount)],
    ['退款错误码', String(value.refundVerificationErrorCount)]
  ] : []
  return <Panel title="钱包资金来源核对" actions={<button className="delivery-secondary" onClick={() => void load()} disabled={loading}>{loading ? '加载中…' : '刷新'}</button>}>
    <ErrorNotice error={error} refresh={() => void load()} />
    {!value ? <p>加载中…</p> : !value.fundingTablesAvailable ? <div className="delivery-notice"><strong>资金来源数据暂不可用</strong><span>当前只显示已有钱包余额，历史余额尚未核对。</span></div> : <>
      <div className="delivery-summary-grid">{metrics.map(([label, metric]) => <article key={label}><span>{label}</span><strong>{metric}</strong></article>)}</div>
      <p className="delivery-description">核对结果：{directionText[value.coverageDirection] || '需人工核对'} · 入账批次 {value.fundingLots} · 已分配 {value.fundingAllocations}</p>
      <p className="delivery-description">此页仅供对账参考，不能修改支付或退款状态。</p>
    </>}
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
      <label>状态<select value={status} onChange={event => { setPage(1); setStatus(event.target.value) }}><option value="">全部</option>{statusOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
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
      <Fact label="配送费" value={money(order.deliveryFeeCents)} />
      <Fact label="骑手报酬" value={money(order.riderFeeCents)} />
      <Fact label="平台抽成" value={money(order.platformCutCents)} />
      <Fact label="取货信息" value={`${order.pickupContactName} / ${order.pickupContactPhone} / ${order.pickupAddress}`} />
      <Fact label="收货信息" value={`${order.recipientName} / ${order.recipientPhone} / ${order.deliveryAddress}`} />
    </div>
    <p className="delivery-description">{order.remark || '无备注'}</p>
    <h4>资金明细</h4>
    <div className="detail-grid">
      <Fact label="结算" value={value.settlement ? `${value.settlement.status} / ${money(value.settlement.amountCents)}` : '—'} />
      <Fact label="支付" value={value.payment ? `${value.payment.status} / ${money(value.payment.amountCents)}` : '—'} />
      <Fact label="退款" value={value.refund ? `${value.refund.status} / ${money(value.refund.amountCents)}` : '—'} />
      <Fact label="支付核实" value={value.payment ? `${value.payment.providerStatus || '—'} / ${value.payment.errorCode || '无错误码'} / ${value.payment.verifyAttemptCount} 次` : '—'} />
      <Fact label="退款核实" value={value.refund ? `${value.refund.providerStatus || '—'} / ${value.refund.errorCode || '无错误码'} / ${value.refund.verifyAttemptCount} 次` : '—'} />
    </div>
    <h4>阶段时间</h4>
    <div className="delivery-timeline">{stages.map(([label, at]) => <div key={label}><span>{label}</span><strong>{time(at)}</strong></div>)}</div>
  </Detail>
}

function FeeConfigPanel() {
  const dialog = useAdminDialog()
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
    if (!await dialog.confirm({ title: '更新全局配送费', description: `新订单将使用配送费 ${money(Number(deliveryFee))}、平台抽成 ${money(Number(platformCut))}。已提交的订单仍按下单时的金额计算。`, confirmLabel: '保存配置' })) return
    setLoading(true); setError(''); setConflict(false)
    try {
      await deliveryAdminAPI.updateFeeConfig(Number(deliveryFee), Number(platformCut), value.rowVersion)
      await load()
    } catch (reason) {
      if (isConflict(reason)) { setConflict(true); setError(conflictMessage) } else setError(errorMessage(reason))
    } finally { setLoading(false) }
  }
  return <>
  <Panel title="全局配送费配置">
    <ErrorNotice error={error} conflict={conflict} refresh={() => void load()} />
    {value ? <div className="delivery-form">
      <label>配送费（分）<input type="number" min="1" value={deliveryFee} onChange={event => setDeliveryFee(event.target.value)} /></label>
      <label>平台抽成（分）<input type="number" min="0" value={platformCut} onChange={event => setPlatformCut(event.target.value)} /></label>
            <Fact label="更新时间" value={time(value.updatedAt)} />
      <button disabled={loading} onClick={() => void submit()}>{loading ? '保存中…' : '保存配置'}</button>
      <button className="delivery-secondary" disabled={loading} onClick={() => void load()}>刷新</button>
    </div> : <p>加载中…</p>}
  </Panel>
  <RiderAvailabilityConfigPanel />
  </>
}

function RiderAvailabilityConfigPanel() {
  // The admin API owns the authenticated session. Keep this panel isolated to
  // its mount lifecycle so the wallet test harness and account logout cannot
  // inject a second store dependency into the delivery console.
  const authKey = 'delivery-rider-capacity'
  const currentAuth = useRef(authKey)
  currentAuth.current = authKey
  const [value, setValue] = useState<RiderAvailabilityConfig | null>(null)
  const [limit, setLimit] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const epoch = useRef(0)
  const mounted = useRef(true)
  const locked = useRef(false)
  const load = useCallback(async () => {
    if (locked.current) return
    const owner = currentAuth.current
    const version = ++epoch.current
    const current = () => mounted.current && currentAuth.current === owner && epoch.current === version
    setError('')
    setBusy(true)
    setValue(null)
    try {
      const next = await deliveryAdminAPI.riderAvailabilityConfig()
      if (current()) {
        setValue(next)
        if (next.enabled) setLimit(String(next.maxInFlightPerRider))
      }
    } catch (reason) { if (current()) setError(errorMessage(reason)) }
    finally { if (current()) setBusy(false) }
  }, [])
  useEffect(() => {
    mounted.current = true
    locked.current = false
    void load()
    return () => { mounted.current = false; epoch.current++ }
  }, [load, authKey])
  const save = async () => {
    if (!value || !value.enabled || locked.current || busy) return
    const next = Number(limit)
    if (!Number.isSafeInteger(next) || next < 1 || next > 100) { setError('接单容量必须是 1–100 的整数'); return }
    const owner = currentAuth.current
    const version = ++epoch.current
    const current = () => mounted.current && currentAuth.current === owner && epoch.current === version
    locked.current = true
    setBusy(true); setError('')
    try {
      const result = await deliveryAdminAPI.updateRiderAvailabilityConfig(next, value.rowVersion)
      if (current()) {
        setValue(result)
        if (result.enabled) setLimit(String(result.maxInFlightPerRider))
      }
    } catch {
      if (!current()) return
      // Refresh authoritative state after conflict or an unknown PATCH result.
      // Never replay a write with a freshly fetched version automatically.
      setValue(null)
      try {
        const result = await deliveryAdminAPI.riderAvailabilityConfig()
        if (current()) {
          setValue(result)
          if (result.enabled) setLimit(String(result.maxInFlightPerRider))
          setError('已同步最新配置，请核对；本次不会自动重试保存')
        }
      } catch { if (current()) setError('保存结果待核实，请刷新配置后再操作') }
    } finally { if (current()) { locked.current = false; setBusy(false) } }
  }
  return <Panel title="骑手接单容量">
    <p className="delivery-description">只影响主动“开始接单”的骑手；停止接单不会清除已有配送。接单时系统会自动核对人数上限。</p>
    <ErrorNotice error={error} refresh={() => void load()} />
    {value ? value.enabled ? <div className="delivery-form">
      <label>每位骑手最多进行中订单<input type="number" min="1" max="100" disabled={busy} value={limit} onChange={event => setLimit(event.target.value)} /></label>
      <Fact label="新单广播" value={value.enabled ? '已启用' : '已关闭'} />
            <button disabled={busy} onClick={() => void save()}>{busy ? '保存中…' : '保存容量'}</button>
      <button className="delivery-secondary" disabled={busy} onClick={() => void load()}>刷新</button>
    </div> : <p>新单广播尚未开启，开启后可在这里调整人数上限。</p> : <p>{busy ? '加载中…' : '容量配置暂未读取'}<button disabled={busy} onClick={() => void load()}>重新加载</button></p>}
  </Panel>
}

function MerchantOverridesPanel() {
  const dialog = useAdminDialog()
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
    if (!await dialog.confirm({ title: '保存商家特殊价', description: `确认保存商家 #${merchant} 的特殊配送费 ${money(Number(fee))}、平台抽成 ${money(Number(cut))}？仅影响后续新订单。`, confirmLabel: '保存配置' })) return
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
    if (!await dialog.confirm({ title: '停用商家特殊价', description: `停用商家 #${row.merchantUserId} 的特殊价后，新订单将恢复使用全局配置。`, confirmLabel: '确认停用', tone: 'danger' })) return
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
      <label>状态<select value={status} onChange={event => { setPage(1); setStatus(event.target.value) }}><option value="">全部</option><option value="active">生效中</option><option value="disabled">已停用</option></select></label>
      <label>配送费（分）<input value={fee} inputMode="numeric" onChange={event => setFee(event.target.value)} /></label>
      <label>平台抽成（分）<input value={cut} inputMode="numeric" onChange={event => setCut(event.target.value)} /></label>
      <label>版本号<input value={expectedRowVersion} inputMode="numeric" placeholder="修改已有商家时填写，防止覆盖他人改动" onChange={event => setExpectedRowVersion(event.target.value)} /></label>
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
  const dialog = useAdminDialog()
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
  const operationKeys = useRef(new Map<string, { key: string; note: string }>())
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
    const pending = operationKeys.current.get(row.earningPublicId)
    const note = pending?.note ?? await dialog.prompt({ title: '登记线下清账', description: `将骑手 #${row.riderUserId} 的 ${money(row.riderFeeCents)} 登记为已线下支付。本操作不会发起真实打款。`, inputLabel: '清账备注', placeholder: '例如：微信已付截图编号 / 日期', requiredMessage: '清账备注不能为空', confirmLabel: '确认已支付' })
    if (note === null) return
    setBusy(row.earningPublicId); setError(''); setConflict(false)
    try {
      const operation = pending || { key: newIdempotencyKey(), note }
      operationKeys.current.set(row.earningPublicId, operation)
      const updated = await deliveryAdminAPI.payOffline(row.earningPublicId, row.rowVersion, operation.key, operation.note)
      operationKeys.current.delete(row.earningPublicId)
      setResult(updated)
      setDetail(updated.earning)
      await load()
    } catch (reason) {
      if (isConflict(reason)) { setConflict(true); setError(conflictMessage) } else setError(errorMessage(reason))
    } finally { setBusy('') }
  }
  return <Panel title="骑手报酬清账">
    <p className="delivery-capability-notice delivery-capability-notice--inline"><strong>线下清账</strong><span>登记只用于记录已完成的线下支付，需填写可追溯备注；不会调用微信支付或其他自动打款服务。</span></p>
    <div className="delivery-controls"><label>状态<select value={status} onChange={event => { setPage(1); setStatus(event.target.value) }}><option value="">全部</option><option value="pending">待结算</option><option value="settled">已结算</option><option value="paid_offline">已登记结款</option></select></label><label>骑手用户 ID<input value={riderUserId} inputMode="numeric" onChange={event => { setPage(1); setRiderUserId(event.target.value) }} /></label><button onClick={() => void load()}>刷新</button></div>
    <ErrorNotice error={error} conflict={conflict} refresh={() => void load()} />
    <div className="table-scroll"><table><thead><tr><th>报酬</th><th>配送单</th><th>骑手</th><th>金额</th><th>状态</th><th>版本</th><th>操作</th></tr></thead><tbody>{data.items.map(row => <tr key={row.earningPublicId}>
      <td><button className="link-button" onClick={() => void inspect(row.earningPublicId)}>{short(row.earningPublicId)}</button></td>
      <td>{short(row.deliveryOrderPublicId)}</td><td>#{row.riderUserId}</td><td>{money(row.riderFeeCents)}<span>平台抽成 {money(row.platformCutCents)}</span></td><td><Badge value={row.settlementStatus} /></td><td>v{row.rowVersion}</td>
      <td>{row.settlementStatus !== 'paid_offline' ? <button disabled={busy === row.earningPublicId} onClick={() => void pay(row)}>{busy === row.earningPublicId ? '登记中…' : '登记线下已付'}</button> : <span>已清账</span>}</td>
    </tr>)}</tbody></table></div>
    <Pagination value={data} setPage={setPage} />
    {detail && <Detail title={`报酬 ${detail.earningPublicId}`} close={() => { setDetail(null); setResult(null) }}><div className="detail-grid"><Fact label="骑手" value={`#${detail.riderUserId}`} /><Fact label="配送单" value={detail.deliveryOrderPublicId} /><Fact label="骑手报酬" value={money(detail.riderFeeCents)} /><Fact label="状态" value={<Badge value={detail.settlementStatus} />} /><Fact label="线下支付时间" value={time(detail.paidOfflineAt)} /></div>{result && <p className="delivery-description">结款已登记：钱包余额 {money(result.wallet.availableBalanceCents)}，流水号 {short(result.ledger.ledgerPublicId)}，结款记录 {short(result.payout.payoutPublicId)}。</p>}</Detail>}
    <h3 className="delivery-subtitle">最近清账登记</h3>
    <div className="table-scroll"><table><thead><tr><th>登记</th><th>报酬</th><th>骑手</th><th>金额</th><th>操作人</th><th>时间</th></tr></thead><tbody>{payouts.items.map(row => <tr key={row.payoutPublicId}><td>{short(row.payoutPublicId)}</td><td>{short(row.earningPublicId)}</td><td>#{row.riderUserId}</td><td>{money(row.amountCents)}</td><td>#{row.operatorUserId}</td><td>{time(row.paidOfflineAt)}</td></tr>)}</tbody></table></div>
  </Panel>
}

function RiderPayoutQueuePanel() {
  const dialog = useAdminDialog()
  const [data, setData] = useState<Page<RiderPayoutQueue>>(emptyPage())
  const [status, setStatus] = useState('')
  const [page, setPage] = useState(1)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState('')
  const operationKeys = useRef(new Map<string, { key: string; reason: string }>())
  const load = useCallback(async () => {
    setError('')
    try { setData(await deliveryAdminAPI.riderPayoutQueue({ status, page, pageSize })) } catch (reason) { setError(errorMessage(reason)) }
  }, [status, page])
  useEffect(() => { void load() }, [load])
  const cancel = async (row: RiderPayoutQueue) => {
    const pending = operationKeys.current.get(row.payoutQueuePublicId)
    const reason = pending?.reason ?? await dialog.prompt({
      title: '取消待打款队列',
      description: '取消只会释放平台内部占用，不会发起微信转账。请填写后续处理原因。',
      inputLabel: '取消原因',
      placeholder: '例如：改走后台线下结款',
      initialValue: '改走后台线下结款',
      requiredMessage: '取消原因不能为空',
      confirmLabel: '确认取消',
      tone: 'danger',
    })
    if (!reason?.trim()) return
    setBusy(row.payoutQueuePublicId); setError('')
    try {
      const operation = pending || { key: newIdempotencyKey(), reason: reason.trim() }
      operationKeys.current.set(row.payoutQueuePublicId, operation)
      await deliveryAdminAPI.cancelRiderPayoutQueue(row.payoutQueuePublicId, row.rowVersion, operation.key, operation.reason)
      operationKeys.current.delete(row.payoutQueuePublicId)
      await load()
    }
    catch (value) { setError(errorMessage(value)) }
    finally { setBusy('') }
  }
  return <Panel title="平台待打款队列">
    <p className="delivery-capability-notice delivery-capability-notice--inline"><strong>内部队列</strong><span>当前仅占用已核验收益，未调用微信转账；只能取消排队，不能标记到账或批准打款。</span></p>
    <div className="delivery-controls"><label>状态<select value={status} onChange={event => { setPage(1); setStatus(event.target.value) }}><option value="">全部</option><option value="queued">已排队</option><option value="blocked">已暂停</option><option value="cancelled">已取消</option></select></label><button onClick={() => void load()}>刷新</button></div>
    <ErrorNotice error={error} refresh={() => void load()} />
    <div className="table-scroll"><table><thead><tr><th>队列</th><th>收益/配送单</th><th>骑手</th><th>金额</th><th>来源/状态</th><th>占用</th><th>操作</th></tr></thead><tbody>{data.items.map(row => <tr key={row.payoutQueuePublicId}><td>{short(row.payoutQueuePublicId)}<span>v{row.rowVersion}</span></td><td>{short(row.earningPublicId)}<span>{short(row.deliveryOrderPublicId)}</span></td><td>#{row.riderUserId}</td><td>{money(row.amountCents)}</td><td><Badge value={row.status} /><span>{row.reasonCode || '已核验'}</span></td><td>{money(row.reservedCents)}</td><td>{row.status !== 'cancelled' ? <button disabled={busy === row.payoutQueuePublicId} onClick={() => void cancel(row)}>{busy === row.payoutQueuePublicId ? '取消中…' : '取消排队'}</button> : <span>已释放</span>}</td></tr>)}</tbody></table></div>
    <Pagination value={data} setPage={setPage} />
  </Panel>
}

function RiderWithdrawalsPanel() {
  const dialog = useAdminDialog()
  const [withdrawalWritesReady, setWritesReady] = useState(false)
  const [data, setData] = useState<Page<RiderWithdrawal>>(emptyPage())
  const [detail, setDetail] = useState<RiderWithdrawalProcessing | null>(null)
  const [status, setStatus] = useState('processing')
  const [riderUserId, setRiderUserId] = useState('')
  const [page, setPage] = useState(1)
  const [error, setError] = useState('')
  const [conflict, setConflict] = useState(false)
  const [busy, setBusy] = useState('')
  const pending = useRef(new Map<string, { action: RiderWithdrawalAction; command: RiderWithdrawalCommand }>())
  const operationLock = useRef(false)
  const requestVersion = useRef(0)
  const detailVersion = useRef(0)
  const mounted = useRef(true)
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; requestVersion.current++; detailVersion.current++ } }, [])
  const load = useCallback(async () => {
    const version = ++requestVersion.current
    setWritesReady(false); setError(''); setConflict(false)
    try {
      const [result, capability] = await Promise.all([
        deliveryAdminAPI.riderWithdrawals({ status, riderUserId, page, pageSize }),
        deliveryAdminAPI.riderWithdrawalCapabilities()
      ])
      if (mounted.current && version === requestVersion.current) {
        setData(result)
        setWritesReady(capability.withdrawalEnabled === true && capability.mode === 'manual' && capability.claimRequired === true)
      }
    } catch (reason) { if (mounted.current && version === requestVersion.current) setError(errorMessage(reason)) }
  }, [status, riderUserId, page])
  const activeLoad = useRef(load)
  activeLoad.current = load
  useEffect(() => { void load(); return () => { requestVersion.current++ } }, [load])
  const inspect = async (id: string) => {
    const version = ++detailVersion.current
    setDetail(null)
    try {
      const result = await deliveryAdminAPI.riderWithdrawalProcessing(id)
      if (mounted.current && version === detailVersion.current) setDetail(result)
    } catch (cause) { if (mounted.current && version === detailVersion.current) setError(errorMessage(cause)) }
  }
  const run = async (item: RiderWithdrawalProcessing, action: RiderWithdrawalAction) => {
    if (!withdrawalWritesReady || operationLock.current || !mounted.current) return
    const row = item.withdrawal
    operationLock.current = true
    setBusy(row.withdrawPublicId); setError(''); setConflict(false)
    try {
      let operation = pending.current.get(row.withdrawPublicId)
      if (operation && operation.action !== action) {
        setError('原操作结果尚未确认，只能重试原操作或刷新核对。请勿重复转账。'); return
      }
      if (!operation) {
        const command: RiderWithdrawalCommand = { expectedRowVersion: row.rowVersion, idempotencyKey: newIdempotencyKey() }
        if (action === 'claim') {
          if (!await dialog.confirm({ title: '领取提现处理权', description: '领取成功后仅你可处理此申请。领取不发起转账；刷新或超时不会自动释放。请先核对骑手、金额与收款信息。', confirmLabel: '领取处理' })) return
          command.claimKey = newIdempotencyKey()
        } else {
          if (action !== 'reject') command.claimKey = item.claimKey
          const resolvesPaid = action === 'approve' || action === 'resolve_paid'
          const resolvesUnpaid = action === 'release' || action === 'resolve_unpaid'
          const value = (await dialog.prompt({
            title: resolvesPaid ? (action === 'resolve_paid' ? '已有付款凭证，登记已打款' : '登记已实际打款') : resolvesUnpaid ? (action === 'resolve_unpaid' ? '确认未转账并退回余额' : '确认未转账并解除处理') : action === 'exception' ? '保存异常对账证据' : '拒绝申请并退回余额',
            description: resolvesPaid ? (action === 'resolve_paid' ? '仅登记此前已经完成的人工转账，不会再次转账；请填写原付款凭证。' : '仅登记已完成的人工转账，不会调用微信转账。结果未知请返回保存异常；登记超时不要重转。')
              : resolvesUnpaid ? '只有明确确认从未执行转账时才能释放占用并回充一次；已付款或结果未知均不可使用。'
              : action === 'exception' ? '保留资金占用和处理权，禁止回充或再次转账。请写明已付款或结果未知的原因。'
              : '只适用于未被领取、确定未实际打款的申请。退回余额仅执行一次。',
            inputLabel: resolvesPaid ? '原付款凭证编号' : '具体原因',
            requiredMessage: '必须填写后才能提交', confirmLabel: action === 'release' ? '确认未转账并解除' : action === 'resolve_unpaid' ? '确认未转账并退回' : '确认提交'
          }))?.trim()
          if (!value || !mounted.current) return
          if (value.length > (resolvesPaid ? 128 : 256)) { setError('填写内容过长。'); return }
          if (resolvesPaid) command.paymentReference = value
          else command.reason = value
          if (resolvesUnpaid) command.confirmedUnpaid = true
          if (action === 'exception') {
            const receipt = (await dialog.prompt({ title: '保留付款凭证', description: '已转账请填写凭证编号；尚无法确定请填写“结果未知”，不要再次转账。', inputLabel: '凭证编号或结果未知', requiredMessage: '请说明付款状态', confirmLabel: '保存证据并保持占用' }))?.trim()
            if (!receipt || !mounted.current) return
            if (receipt.length > 128) { setError('凭证编号过长。'); return }
            command.paymentReference = receipt
          }
        }
        if (!mounted.current) return
        operation = { action, command }
        pending.current.set(row.withdrawPublicId, operation)
      }
      await deliveryAdminAPI.processRiderWithdrawal(row.withdrawPublicId, operation.action, operation.command)
      pending.current.delete(row.withdrawPublicId)
      if (mounted.current) { await activeLoad.current(); await inspect(row.withdrawPublicId) }
    } catch (cause) {
      if (!mounted.current) return
      // Definitive rejection has not changed money. On timeout/5xx retain the
      // exact original command so retry cannot invent a second operation.
      if (cause instanceof DeliveryAPIError && [400, 403, 404, 409, 422].includes(cause.status)) {
        pending.current.delete(row.withdrawPublicId)
        await inspect(row.withdrawPublicId)
      }
      if (isConflict(cause)) setConflict(true)
      setError(errorMessage(cause) + '；如已付款或结果未知，请勿重复转账或退回余额。')
    } finally { operationLock.current = false; if (mounted.current) setBusy('') }
  }
  const processingLabel = (state: RiderWithdrawalProcessing['status']) => ({
    unclaimed: '未领取', released: '已解除，可重新领取', active: '处理中（处理权不自动过期）',
    exception: '异常对账中（保持资金占用）', completed: '已登记打款'
  })[state]
  const open = detail && detail.withdrawal.status === 'processing'
  const unclaimed = detail && ['unclaimed', 'released'].includes(detail.status)
  const owned = detail?.ownedByMe && detail.status === 'active'
  const exceptionOwned = detail?.ownedByMe && detail.status === 'exception'
  const pendingOperation = detail ? pending.current.get(detail.withdrawal.withdrawPublicId) : undefined
  return <Panel title="骑手提现处理" actions={<button className="delivery-secondary" onClick={() => void load()}>刷新</button>}>
    <p className="delivery-capability-notice delivery-capability-notice--inline"><strong>{withdrawalWritesReady ? '可受理申请 · 人工打款' : '提现登记暂未开放'}</strong><span>{withdrawalWritesReady ? '先领取处理权，再线下转账给骑手，最后登记付款凭证。转账结果不确定时请保留凭证，不要重复转账。' : '当前只能查看，请勿据此付款。'}</span></p>
    <div className="delivery-controls"><label>状态<select value={status} onChange={event => { setPage(1); setStatus(event.target.value) }}><option value="">全部</option><option value="processing">提现中</option><option value="paid">已登记打款</option><option value="rejected">已退回</option></select></label><label>骑手用户 ID<input value={riderUserId} inputMode="numeric" onChange={event => { setPage(1); setRiderUserId(event.target.value) }} /></label></div>
    <ErrorNotice error={error} conflict={conflict} refresh={() => void load()} />
    <div className="table-scroll"><table><thead><tr><th>提现申请</th><th>骑手</th><th>金额</th><th>状态</th><th>版本</th><th>创建时间</th><th>操作</th></tr></thead><tbody>{data.items.map(row => <tr key={row.withdrawPublicId}><td>{short(row.withdrawPublicId)}</td><td>#{row.riderUserId}</td><td>{money(row.amountCents)}</td><td>{row.status === 'paid' ? '已登记打款' : row.status === 'processing' ? '提现中' : '已退回余额'}</td><td>v{row.rowVersion}</td><td>{time(row.createdAt)}</td><td><button disabled={Boolean(busy)} onClick={() => void inspect(row.withdrawPublicId)}>查看与处理</button></td></tr>)}</tbody></table></div>
    <Pagination value={data} setPage={setPage} />
    {detail && <Detail title={`提现 ${detail.withdrawal.withdrawPublicId}`} close={() => { detailVersion.current++; setDetail(null) }}>
      <div className="detail-grid"><Fact label="骑手" value={`#${detail.withdrawal.riderUserId}`} /><Fact label="金额" value={money(detail.withdrawal.amountCents)} /><Fact label="状态" value={processingLabel(detail.status)} /><Fact label="处理人" value={detail.operatorUserId ? `#${detail.operatorUserId}${detail.ownedByMe ? '（本人）' : ''}` : '—'} /><Fact label="付款凭证" value={detail.paymentReference || '—'} /><Fact label="处理说明" value={detail.reason || detail.withdrawal.reason || '—'} /></div>
      {error && <p role="alert">{error}</p>}
      <div className="takeout-row-actions">
        <button disabled={Boolean(busy)} onClick={() => void inspect(detail.withdrawal.withdrawPublicId)}>重新核对处理状态</button>
        {pendingOperation ? <button disabled={!withdrawalWritesReady || Boolean(busy)} onClick={() => void run(detail, pendingOperation.action)}>重试原操作（不要再次转账）</button> : open && <>
          {unclaimed && <><button disabled={!withdrawalWritesReady || Boolean(busy)} onClick={() => void run(detail, 'claim')}>领取处理</button><button className="danger" disabled={!withdrawalWritesReady || Boolean(busy)} onClick={() => void run(detail, 'reject')}>拒绝并退回余额</button></>}
          {owned && <><button disabled={!withdrawalWritesReady || Boolean(busy)} onClick={() => void run(detail, 'approve')}>登记已实际打款</button><button disabled={!withdrawalWritesReady || Boolean(busy)} onClick={() => void run(detail, 'release')}>未转账，解除处理</button><button className="danger" disabled={!withdrawalWritesReady || Boolean(busy)} onClick={() => void run(detail, 'exception')}>结果未知 / 异常留证</button></>}
          {exceptionOwned && <><button disabled={!withdrawalWritesReady || Boolean(busy)} onClick={() => void run(detail, 'resolve_paid')}>已有凭证，登记已打款</button><button className="danger" disabled={!withdrawalWritesReady || Boolean(busy)} onClick={() => void run(detail, 'resolve_unpaid')}>确认未转账，退回余额</button></>}
          {detail.status === 'exception' && <p>已保留凭证和资金占用，需人工对账；本页不允许再次打款或退回余额。</p>}
        </>}
      </div>
    </Detail>}
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
  return <Panel title="退款记录" actions={<button className="delivery-secondary" onClick={() => void load()}>刷新</button>}>
    <div className="delivery-controls"><label>状态<select value={status} onChange={event => { setPage(1); setStatus(event.target.value) }}><option value="">全部</option><option value="pending">处理中</option><option value="succeeded">已退款</option><option value="failed">退款失败</option><option value="retry_pending">等待重试</option></select></label><label>商家用户 ID<input value={merchantUserId} inputMode="numeric" onChange={event => { setPage(1); setMerchantUserId(event.target.value) }} /></label></div>
    <ErrorNotice error={error} refresh={() => void load()} />
    <div className="table-scroll"><table><thead><tr><th>退款</th><th>配送单</th><th>商家</th><th>金额</th><th>状态</th><th>时间</th></tr></thead><tbody>{data.items.map(row => <tr key={row.refundPublicId}><td><button className="link-button" onClick={() => void inspect(row.refundPublicId)}>{short(row.refundPublicId)}</button><span>{row.outRefundNo}</span></td><td>{short(row.deliveryOrderPublicId)}</td><td>#{row.merchantUserId}</td><td>{money(row.amountCents)}</td><td><Badge value={row.status} /></td><td>{time(row.updatedAt)}</td></tr>)}</tbody></table></div>
    <Pagination value={data} setPage={setPage} />
    {detail && <Detail title={`退款 ${detail.refundPublicId}`} close={() => setDetail(null)}><div className="detail-grid"><Fact label="状态" value={<Badge value={detail.status} />} /><Fact label="配送单" value={detail.deliveryOrderPublicId} /><Fact label="商家" value={`#${detail.merchantUserId}`} /><Fact label="金额" value={money(detail.amountCents)} /><Fact label="退款单号" value={detail.outRefundNo} /><Fact label="失败原因" value={detail.failureReason || '—'} /><Fact label="成功时间" value={time(detail.succeededAt)} /></div><p className="delivery-description">退款记录仅用于核对状态，后台不会在此页面直接发起或伪造退款。</p></Detail>}
  </Panel>
}

function localToRFC3339(value: string) {
  return value ? new Date(value).toISOString() : ''
}

function short(value: string) {
  return value.length > 18 ? `${value.slice(0, 8)}…${value.slice(-6)}` : value
}

function newIdempotencyKey() {
  const alphabet = '0123456789ABCDEFGHJKMNPQRSTVWXYZ'
  let value = Math.floor(Date.now() / 1000)
  let result = ''
  for (let index = 0; index < 10; index += 1) { result = alphabet[value % 32] + result; value = Math.floor(value / 32) }
  for (let index = 0; index < 16; index += 1) result += alphabet[Math.floor(Math.random() * alphabet.length)]
  return result
}
