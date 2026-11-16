import { useCallback, useEffect, useRef, useState } from 'react'
import {
  deliveryAdminAPI,
  type FeeConfig,
  type FundingSourceSummary,
  type MerchantFeeOverride,
  type MerchantLedger,
  type MerchantWallet,
  type Page,
  type Refund,
  type RiderEarning,
  type RiderLedger,
  type RiderPayout,
  type RiderPayoutQueue,
  type RiderWallet,
  type RiderWithdrawal,
  type RiderWithdrawalCommand,
  type RiderWithdrawalProcessing
} from '../../services/delivery-admin'
import {
  DELIVERY_PAGE_SIZE,
  deliveryErrorMessage,
  earningStatusLabel,
  earningStatusTone,
  feeStatusLabel,
  feeStatusTone,
  conflictMessage,
  isDeliveryConflict,
  formatDateTime,
  formatMoney,
  ledgerDirectionLabel,
  ledgerEntryTypeLabel,
  ledgerReferenceLabel,
  newOperationKey,
  payoutQueueStatusLabel,
  payoutQueueStatusTone,
  refundStatusLabel,
  refundStatusTone,
  settlementSourceLabel,
  settlementStatusLabel,
  shortId,
  useRemoteData,
  withdrawalProcessingLabels,
  withdrawalStatusLabel,
  withdrawalStatusTone
} from '../delivery-hooks'
import { Button, Drawer, EmptyState, Field, Modal, Panel, Pagination, SelectField, StatusPill, usePreviewToast } from '../ui'

export type FundView = 'merchant' | 'rider' | 'earnings' | 'payout' | 'withdrawals' | 'refunds' | 'funding'
export type FeeView = 'default' | 'override'

const FUND_TABS: Array<[FundView, string]> = [
  ['merchant', '商家钱包'],
  ['rider', '骑手钱包'],
  ['earnings', '骑手报酬'],
  ['payout', '待打款'],
  ['withdrawals', '骑手提现'],
  ['refunds', '退款记录'],
  ['funding', '资金核对']
]

export function FundsSection({ value, onChange }: { value: FundView; onChange: (value: FundView) => void }) {
  return <>
    <div className="preview-subtabs" role="tablist" aria-label="资金模块">
      {FUND_TABS.map(([key, label]) => <button
        key={key}
        type="button"
        role="tab"
        aria-selected={value === key}
        className={value === key ? 'active' : ''}
        onClick={() => onChange(key)}
      >{label}</button>)}
    </div>

    {value === 'merchant' && <MerchantWalletsPanel />}
    {value === 'rider' && <RiderWalletsPanel />}
    {value === 'earnings' && <EarningsPanel />}
    {value === 'payout' && <PayoutQueuePanel />}
    {value === 'withdrawals' && <WithdrawalsPanel />}
    {value === 'refunds' && <RefundsPanel />}
    {value === 'funding' && <FundingPanel />}
  </>
}

/* ===== 通用：钱包流水抽屉 ===== */

function LedgerDrawer({ title, loading, error, page, onClose }: {
  title: string
  loading: boolean
  error: string
  page: Page<MerchantLedger | RiderLedger> | null
  onClose: () => void
}) {
  return <Drawer open title={title} onClose={onClose} footer={<Button onClick={onClose}>关闭</Button>}>
    {loading && !page && <p className="preview-toolbar__hint">正在加载钱包流水…</p>}
    {error && <div className="preview-form-error" role="alert">{error}</div>}
    {page && (page.items.length === 0
      ? <EmptyState title="没有流水记录" description="该钱包还没有资金变动。" />
      : <div className="preview-table-wrap"><table className="preview-table" aria-label="钱包流水">
        <thead><tr><th>流水</th><th>方向 / 类型</th><th>金额</th><th>余额变化</th><th>引用</th><th>时间</th></tr></thead>
        <tbody>{page.items.map(row => <tr key={row.ledgerPublicId}>
          <td><strong>{shortId(row.ledgerPublicId)}</strong></td>
          <td>{ledgerDirectionLabel(row.direction)}<span>{ledgerEntryTypeLabel(row.entryType)}</span></td>
          <td><strong>{formatMoney(row.amountCents)}</strong></td>
          <td>{formatMoney(row.balanceBeforeCents)} → {formatMoney(row.balanceAfterCents)}</td>
          <td>{ledgerReferenceLabel(row.referenceType)}<span>{shortId(row.referencePublicId || '—')}</span></td>
          <td>{formatDateTime(row.createdAt)}</td>
        </tr>)}</tbody>
      </table></div>)}
    {page && page.total > page.items.length && <p className="preview-toolbar__hint">仅展示最近 {page.items.length} 条，共 {page.total} 条流水。</p>}
  </Drawer>
}

/* ===== 商家钱包 ===== */

function MerchantWalletsPanel() {
  const [filterInput, setFilterInput] = useState('')
  const [filter, setFilter] = useState('')
  const [filterError, setFilterError] = useState('')
  const [page, setPage] = useState(1)
  const [ledgerUserId, setLedgerUserId] = useState<number | null>(null)
  const [ledgerPage, setLedgerPage] = useState<Page<MerchantLedger> | null>(null)
  const [ledgerLoading, setLedgerLoading] = useState(false)
  const [ledgerError, setLedgerError] = useState('')
  const ledgerEpoch = useRef(0)

  const walletsState = useRemoteData(async () => {
    const result = await deliveryAdminAPI.merchantWallets({
      merchantUserId: filter || undefined,
      page,
      pageSize: DELIVERY_PAGE_SIZE
    })
    if (result.items.length === 0 && result.total > 0 && page > 1) {
      setPage(Math.max(1, Math.ceil(result.total / result.pageSize)))
    }
    return result
  }, [filter, page])

  const applyFilter = () => {
    const trimmed = filterInput.trim()
    if (trimmed && (!/^\d+$/.test(trimmed) || Number(trimmed) <= 0)) {
      setFilterError('请输入有效的商家用户 ID（正整数）')
      return
    }
    setFilterError('')
    setPage(1)
    setFilter(trimmed)
  }

  const openLedger = async (row: MerchantWallet) => {
    const version = ++ledgerEpoch.current
    setLedgerUserId(row.merchantUserId)
    setLedgerPage(null)
    setLedgerError('')
    setLedgerLoading(true)
    try {
      const value = await deliveryAdminAPI.merchantLedger(row.merchantUserId, { page: 1, pageSize: 20 })
      if (ledgerEpoch.current === version) setLedgerPage(value)
    } catch (reason) {
      if (ledgerEpoch.current === version) setLedgerError(deliveryErrorMessage(reason))
    } finally {
      if (ledgerEpoch.current === version) setLedgerLoading(false)
    }
  }

  const wallets = walletsState.data?.items ?? []
  return <>
    <Panel eyebrow="MERCHANT WALLET" title="商家钱包" description="余额来自商家充值，用于支付配送费；点商家编号可查看真实钱包流水。">
      <div className="preview-toolbar">
        <Field
          label="商家用户 ID"
          value={filterInput}
          inputMode="numeric"
          onChange={event => { setFilterInput(event.target.value); setFilterError('') }}
          placeholder="按商家用户 ID 筛选"
          onKeyDown={event => { if (event.key === 'Enter') applyFilter() }}
        />
        <Button variant="primary" onClick={applyFilter}>查询</Button>
        <Button onClick={() => { setFilterInput(''); setFilter(''); setFilterError(''); setPage(1) }}>清空</Button>
        <span className="preview-toolbar__hint">{walletsState.loading ? '正在加载…' : `共 ${walletsState.data?.total ?? 0} 个商家钱包`}</span>
      </div>
      {filterError && <div className="preview-form-error" role="alert">{filterError}</div>}
      {walletsState.error && <div className="preview-form-error" role="alert">{walletsState.error}</div>}
      {!walletsState.loading && wallets.length === 0
        ? <EmptyState title="没有商家钱包" description="商家完成充值后会在这里出现。" />
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="商家钱包列表">
          <thead><tr><th>商家</th><th>钱包</th><th>可用余额</th><th>版本</th><th>更新时间</th></tr></thead>
          <tbody>{wallets.map(item => <tr key={item.walletPublicId} className="is-clickable" onClick={() => void openLedger(item)}>
            <td><button type="button" className="preview-table__primary" onClick={event => { event.stopPropagation(); void openLedger(item) }}>商家 #{item.merchantUserId}<small>查看流水</small></button></td>
            <td>{shortId(item.walletPublicId)}</td>
            <td><strong>{formatMoney(item.availableBalanceCents)}</strong></td>
            <td>v{item.rowVersion}</td>
            <td>{formatDateTime(item.updatedAt)}</td>
          </tr>)}</tbody>
        </table></div>}
      <Pagination
        page={walletsState.data?.page ?? page}
        pageSize={walletsState.data?.pageSize ?? DELIVERY_PAGE_SIZE}
        total={walletsState.data?.total ?? 0}
        onChange={setPage}
        label="商家钱包分页"
      />
    </Panel>
    {ledgerUserId !== null && <LedgerDrawer
      title={`商家 #${ledgerUserId} 钱包流水`}
      loading={ledgerLoading}
      error={ledgerError}
      page={ledgerPage}
      onClose={() => { ledgerEpoch.current += 1; setLedgerUserId(null); setLedgerPage(null); setLedgerError('') }}
    />}
  </>
}

/* ===== 骑手钱包 ===== */

function RiderWalletsPanel() {
  const [filterInput, setFilterInput] = useState('')
  const [filter, setFilter] = useState('')
  const [filterError, setFilterError] = useState('')
  const [page, setPage] = useState(1)
  const [ledgerUserId, setLedgerUserId] = useState<number | null>(null)
  const [ledgerPage, setLedgerPage] = useState<Page<RiderLedger> | null>(null)
  const [ledgerLoading, setLedgerLoading] = useState(false)
  const [ledgerError, setLedgerError] = useState('')
  const ledgerEpoch = useRef(0)

  const walletsState = useRemoteData(async () => {
    const result = await deliveryAdminAPI.riderWallets({
      riderUserId: filter || undefined,
      page,
      pageSize: DELIVERY_PAGE_SIZE
    })
    if (result.items.length === 0 && result.total > 0 && page > 1) {
      setPage(Math.max(1, Math.ceil(result.total / result.pageSize)))
    }
    return result
  }, [filter, page])

  const applyFilter = () => {
    const trimmed = filterInput.trim()
    if (trimmed && (!/^\d+$/.test(trimmed) || Number(trimmed) <= 0)) {
      setFilterError('请输入有效的骑手用户 ID（正整数）')
      return
    }
    setFilterError('')
    setPage(1)
    setFilter(filter)
  }

  const openLedger = async (row: RiderWallet) => {
    const version = ++ledgerEpoch.current
    setLedgerUserId(row.riderUserId)
    setLedgerPage(null)
    setLedgerError('')
    setLedgerLoading(true)
    try {
      const value = await deliveryAdminAPI.riderLedger(row.riderUserId, { page: 1, pageSize: 20 })
      if (ledgerEpoch.current === version) setLedgerPage(value)
    } catch (reason) {
      if (ledgerEpoch.current === version) setLedgerError(deliveryErrorMessage(reason))
    } finally {
      if (ledgerEpoch.current === version) setLedgerLoading(false)
    }
  }

  const wallets = walletsState.data?.items ?? []
  return <>
    <Panel eyebrow="RIDER WALLET" title="骑手钱包" description="可用余额可申请提现；点骑手编号可查看真实钱包流水。">
      <div className="preview-toolbar">
        <Field
          label="骑手用户 ID"
          value={filterInput}
          inputMode="numeric"
          onChange={event => { setFilterInput(event.target.value); setFilterError('') }}
          placeholder="按骑手用户 ID 筛选"
          onKeyDown={event => { if (event.key === 'Enter') applyFilter() }}
        />
        <Button variant="primary" onClick={applyFilter}>查询</Button>
        <Button onClick={() => { setFilterInput(''); setFilter(''); setFilterError(''); setPage(1) }}>清空</Button>
        <span className="preview-toolbar__hint">{walletsState.loading ? '正在加载…' : `共 ${walletsState.data?.total ?? 0} 个骑手钱包`}</span>
      </div>
      {filterError && <div className="preview-form-error" role="alert">{filterError}</div>}
      {walletsState.error && <div className="preview-form-error" role="alert">{walletsState.error}</div>}
      {!walletsState.loading && wallets.length === 0
        ? <EmptyState title="没有骑手钱包" description="骑手完成配送产生报酬后会在这里出现。" />
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="骑手钱包列表">
          <thead><tr><th>骑手</th><th>钱包</th><th>可用余额</th><th>版本</th><th>更新时间</th></tr></thead>
          <tbody>{wallets.map(item => <tr key={item.walletPublicId} className="is-clickable" onClick={() => void openLedger(item)}>
            <td><button type="button" className="preview-table__primary" onClick={event => { event.stopPropagation(); void openLedger(item) }}>骑手 #{item.riderUserId}<small>查看流水</small></button></td>
            <td>{shortId(item.walletPublicId)}</td>
            <td><strong>{formatMoney(item.availableBalanceCents)}</strong></td>
            <td>v{item.rowVersion}</td>
            <td>{formatDateTime(item.updatedAt)}</td>
          </tr>)}</tbody>
        </table></div>}
      <Pagination
        page={walletsState.data?.page ?? page}
        pageSize={walletsState.data?.pageSize ?? DELIVERY_PAGE_SIZE}
        total={walletsState.data?.total ?? 0}
        onChange={setPage}
        label="骑手钱包分页"
      />
    </Panel>
    {ledgerUserId !== null && <LedgerDrawer
      title={`骑手 #${ledgerUserId} 钱包流水`}
      loading={ledgerLoading}
      error={ledgerError}
      page={ledgerPage}
      onClose={() => { ledgerEpoch.current += 1; setLedgerUserId(null); setLedgerPage(null); setLedgerError('') }}
    />}
  </>
}

/* ===== 骑手报酬 ===== */

const EARNING_STATUS_OPTIONS: Array<[string, string]> = [
  ['', '全部状态'],
  ['pending', '待结算'],
  ['settled', '已结算'],
  ['paid_offline', '已登记结款']
]

function EarningsPanel() {
  const [status, setStatus] = useState('')
  const [riderInput, setRiderInput] = useState('')
  const [riderFilter, setRiderFilter] = useState('')
  const [filterError, setFilterError] = useState('')
  const [page, setPage] = useState(1)
  const [detail, setDetail] = useState<RiderEarning | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState('')
  const detailEpoch = useRef(0)

  const earningsState = useRemoteData(async () => {
    const result = await deliveryAdminAPI.riderEarnings({
      status: status || undefined,
      riderUserId: riderFilter || undefined,
      page,
      pageSize: DELIVERY_PAGE_SIZE
    })
    if (result.items.length === 0 && result.total > 0 && page > 1) {
      setPage(Math.max(1, Math.ceil(result.total / result.pageSize)))
    }
    return result
  }, [status, riderFilter, page])
  const payoutsState = useRemoteData(
    () => deliveryAdminAPI.riderPayouts({ riderUserId: riderFilter || undefined, page: 1, pageSize: 10 }),
    [riderFilter]
  )

  const applyRiderFilter = () => {
    const trimmed = riderInput.trim()
    if (trimmed && (!/^\d+$/.test(trimmed) || Number(trimmed) <= 0)) {
      setFilterError('请输入有效的骑手用户 ID（正整数）')
      return
    }
    setFilterError('')
    setPage(1)
    setRiderFilter(trimmed)
  }

  const openDetail = async (id: string) => {
    const version = ++detailEpoch.current
    setDetail(null)
    setDetailError('')
    setDetailLoading(true)
    try {
      const value = await deliveryAdminAPI.riderEarning(id)
      if (detailEpoch.current === version) setDetail(value)
    } catch (reason) {
      if (detailEpoch.current === version) setDetailError(deliveryErrorMessage(reason))
    } finally {
      if (detailEpoch.current === version) setDetailLoading(false)
    }
  }

  const earnings = earningsState.data?.items ?? []
  const payouts = payoutsState.data?.items ?? []
  return <>
    <Panel eyebrow="RIDER EARNING" title="骑手报酬" description="每笔配送完成后的报酬记录，状态与金额直接来自真实结算数据。">
      <div className="preview-toolbar">
        <SelectField label="状态" value={status} onChange={event => { setStatus(event.target.value); setPage(1) }}>
          {EARNING_STATUS_OPTIONS.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
        </SelectField>
        <Field
          label="骑手用户 ID"
          value={riderInput}
          inputMode="numeric"
          onChange={event => { setRiderInput(event.target.value); setFilterError('') }}
          placeholder="按骑手用户 ID 筛选"
          onKeyDown={event => { if (event.key === 'Enter') applyRiderFilter() }}
        />
        <Button variant="primary" onClick={applyRiderFilter}>查询</Button>
        <span className="preview-toolbar__hint">{earningsState.loading ? '正在加载…' : `共 ${earningsState.data?.total ?? 0} 条报酬记录`}</span>
      </div>
      {filterError && <div className="preview-form-error" role="alert">{filterError}</div>}
      {earningsState.error && <div className="preview-form-error" role="alert">{earningsState.error}</div>}
      {!earningsState.loading && earnings.length === 0
        ? <EmptyState title="没有报酬记录" description="调整筛选条件，或等待骑手完成配送。" />
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="骑手报酬列表">
          <thead><tr><th>记录</th><th>骑手</th><th>配送单</th><th>金额</th><th>状态</th><th>产生时间</th></tr></thead>
          <tbody>{earnings.map(item => <tr key={item.earningPublicId} className="is-clickable" onClick={() => void openDetail(item.earningPublicId)}>
            <td><button type="button" className="preview-table__primary" onClick={event => { event.stopPropagation(); void openDetail(item.earningPublicId) }}>{shortId(item.earningPublicId)}<small>查看详情</small></button></td>
            <td>骑手 #{item.riderUserId}</td>
            <td>{shortId(item.deliveryOrderPublicId)}</td>
            <td><strong>{formatMoney(item.riderFeeCents)}</strong><span className="preview-table__fee">平台抽成 {formatMoney(item.platformCutCents)}</span></td>
            <td><StatusPill tone={earningStatusTone(item.settlementStatus)}>{earningStatusLabel(item.settlementStatus)}</StatusPill></td>
            <td>{formatDateTime(item.createdAt)}</td>
          </tr>)}</tbody>
        </table></div>}
      <Pagination
        page={earningsState.data?.page ?? page}
        pageSize={earningsState.data?.pageSize ?? DELIVERY_PAGE_SIZE}
        total={earningsState.data?.total ?? 0}
        onChange={setPage}
        label="骑手报酬分页"
      />

      <p className="preview-toolbar__hint" style={{ marginTop: 16 }}>最近清账登记（平台不会自动打款，仅记录人工线下结款）</p>
      {payoutsState.error && <div className="preview-form-error" role="alert">{payoutsState.error}</div>}
      {payouts.length === 0
        ? <p className="preview-toolbar__hint">暂无清账登记</p>
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="最近清账登记">
          <thead><tr><th>登记</th><th>报酬</th><th>骑手</th><th>金额</th><th>操作人</th><th>线下支付时间</th></tr></thead>
          <tbody>{payouts.map((row: RiderPayout) => <tr key={row.payoutPublicId}>
            <td>{shortId(row.payoutPublicId)}</td>
            <td>{shortId(row.earningPublicId)}</td>
            <td>骑手 #{row.riderUserId}</td>
            <td><strong>{formatMoney(row.amountCents)}</strong></td>
            <td>#{row.operatorUserId}</td>
            <td>{formatDateTime(row.paidOfflineAt)}</td>
          </tr>)}</tbody>
        </table></div>}
    </Panel>

    <Drawer
      open={detail !== null || detailLoading || Boolean(detailError)}
      title={detail ? `报酬 ${shortId(detail.earningPublicId)}` : '报酬详情'}
      description={detail ? `骑手 #${detail.riderUserId} · ${earningStatusLabel(detail.settlementStatus)}` : undefined}
      onClose={() => { detailEpoch.current += 1; setDetail(null); setDetailError(''); setDetailLoading(false) }}
      footer={<Button onClick={() => { detailEpoch.current += 1; setDetail(null); setDetailError(''); setDetailLoading(false) }}>关闭</Button>}
    >
      {detailLoading && !detail && <p className="preview-toolbar__hint">正在加载报酬详情…</p>}
      {detailError && <div className="preview-form-error" role="alert">{detailError}</div>}
      {detail && <div className="preview-kv-grid">
        <div><span>骑手</span><strong>骑手 #{detail.riderUserId}</strong></div>
        <div><span>配送单</span><strong>{detail.deliveryOrderPublicId}</strong></div>
        <div><span>配送费</span><strong>{formatMoney(detail.deliveryFeeCents)}</strong></div>
        <div><span>平台抽成</span><strong>{formatMoney(detail.platformCutCents)}</strong></div>
        <div><span>骑手报酬</span><strong>{formatMoney(detail.riderFeeCents)}</strong></div>
        <div><span>结算状态</span><strong>{earningStatusLabel(detail.settlementStatus)}</strong></div>
        <div><span>线下支付时间</span><strong>{formatDateTime(detail.paidOfflineAt)}</strong></div>
        <div><span>版本</span><strong>v{detail.rowVersion}</strong></div>
        <div><span>产生时间</span><strong>{formatDateTime(detail.createdAt)}</strong></div>
        <div><span>更新时间</span><strong>{formatDateTime(detail.updatedAt)}</strong></div>
      </div>}
    </Drawer>
  </>
}

/* ===== 待打款队列 ===== */

const PAYOUT_STATUS_OPTIONS: Array<[string, string]> = [
  ['', '全部状态'],
  ['queued', '已排队'],
  ['blocked', '已暂停'],
  ['cancelled', '已取消']
]

function PayoutQueuePanel() {
  const { notify } = usePreviewToast()
  const [status, setStatus] = useState('')
  const [page, setPage] = useState(1)
  const [cancelTarget, setCancelTarget] = useState<RiderPayoutQueue | null>(null)
  const [reason, setReason] = useState('')
  const [formError, setFormError] = useState('')
  const [busy, setBusy] = useState(false)

  const queueState = useRemoteData(async () => {
    const result = await deliveryAdminAPI.riderPayoutQueue({ status: status || undefined, page, pageSize: DELIVERY_PAGE_SIZE })
    if (result.items.length === 0 && result.total > 0 && page > 1) {
      setPage(Math.max(1, Math.ceil(result.total / result.pageSize)))
    }
    return result
  }, [status, page])

  const submitCancel = async () => {
    const target = cancelTarget
    if (!target) return
    if (!reason.trim()) {
      setFormError('请填写取消原因')
      return
    }
    setBusy(true)
    setFormError('')
    try {
      await deliveryAdminAPI.cancelRiderPayoutQueue(target.payoutQueuePublicId, target.rowVersion, newOperationKey(), reason.trim())
      notify('已取消待打款排队，释放内部资金占用')
      setCancelTarget(null)
      setReason('')
      await queueState.reload()
    } catch (cause) {
      setFormError(deliveryErrorMessage(cause))
    } finally {
      setBusy(false)
    }
  }

  const items = queueState.data?.items ?? []
  return <>
    <Panel eyebrow="PAYOUT QUEUE" title="待打款" description="平台内部的待打款队列：只占用已核验收益，不会发起真实转账；只能取消排队，不能在此标记到账。">
      <div className="preview-capability-note">取消排队只释放平台内部资金占用，不会发起微信转账；请填写后续处理原因（例如改走线下结款）。</div>
      <div className="preview-toolbar">
        <SelectField label="状态" value={status} onChange={event => { setStatus(event.target.value); setPage(1) }}>
          {PAYOUT_STATUS_OPTIONS.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
        </SelectField>
        <span className="preview-toolbar__hint">{queueState.loading ? '正在加载…' : `共 ${queueState.data?.total ?? 0} 条队列记录`}</span>
      </div>
      {queueState.error && <div className="preview-form-error" role="alert">{queueState.error}</div>}
      {!queueState.loading && items.length === 0
        ? <EmptyState title="没有待打款记录" description="骑手报酬进入打款队列后会出现在这里。" />
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="待打款列表">
          <thead><tr><th>队列</th><th>骑手</th><th>金额</th><th>状态</th><th>占用</th><th>申请时间</th><th /></tr></thead>
          <tbody>{items.map(item => <tr key={item.payoutQueuePublicId}>
            <td><strong>{shortId(item.payoutQueuePublicId)}</strong><span>报酬 {shortId(item.earningPublicId)} · 配送单 {shortId(item.deliveryOrderPublicId)} · v{item.rowVersion}</span></td>
            <td>骑手 #{item.riderUserId}</td>
            <td><strong>{formatMoney(item.amountCents)}</strong></td>
            <td><StatusPill tone={payoutQueueStatusTone(item.status)}>{payoutQueueStatusLabel(item.status)}</StatusPill><span className="preview-table__fee">{item.reasonCode || (item.status === 'blocked' ? '资金占用未释放' : '已核验收益')}</span></td>
            <td>{formatMoney(item.reservedCents)}</td>
            <td>{formatDateTime(item.createdAt)}</td>
            <td onClick={event => event.stopPropagation()}>{item.status !== 'cancelled' && <Button size="sm" variant="ghost" onClick={() => { setCancelTarget(item); setReason(''); setFormError('') }}>取消排队</Button>}</td>
          </tr>)}</tbody>
        </table></div>}
      <Pagination
        page={queueState.data?.page ?? page}
        pageSize={queueState.data?.pageSize ?? DELIVERY_PAGE_SIZE}
        total={queueState.data?.total ?? 0}
        onChange={setPage}
        label="待打款分页"
      />
    </Panel>

    <Modal
      open={Boolean(cancelTarget)}
      title="取消待打款排队"
      description={cancelTarget ? `队列 ${shortId(cancelTarget.payoutQueuePublicId)} · 骑手 #${cancelTarget.riderUserId} · ${formatMoney(cancelTarget.amountCents)}` : undefined}
      onClose={() => { if (!busy) { setCancelTarget(null); setFormError('') } }}
      footer={<>
        <Button disabled={busy} onClick={() => { setCancelTarget(null); setFormError('') }}>返回</Button>
        <Button variant="danger" disabled={busy} onClick={() => void submitCancel()}>{busy ? '取消中…' : '确认取消排队'}</Button>
      </>}
    >
      <div className="preview-form-stack">
        <div className="preview-capability-note">取消只会释放平台内部占用，不会发起微信转账。请填写后续处理原因。</div>
        <Field
          label="取消原因"
          required
          value={reason}
          onChange={event => { setReason(event.target.value); setFormError('') }}
          placeholder="例如：改走后台线下结款"
        />
        {formError && <div className="preview-form-error">{formError}</div>}
      </div>
    </Modal>
  </>
}

/* ===== 骑手提现 ===== */

const WITHDRAWAL_STATUS_OPTIONS: Array<[string, string]> = [
  ['', '全部状态'],
  ['processing', '提现中'],
  ['paid', '已登记打款'],
  ['rejected', '已退回']
]

type WithdrawalMode = 'claim' | 'approve' | 'reject' | 'release' | 'exception' | 'resolve_paid' | 'resolve_unpaid'

const withdrawalModeTitles: Record<WithdrawalMode, string> = {
  claim: '领取处理权',
  approve: '登记已实际打款',
  reject: '拒绝申请并退回余额',
  release: '确认未转账并解除处理',
  exception: '结果未知 / 异常留证',
  resolve_paid: '已有付款凭证，登记已打款',
  resolve_unpaid: '确认未转账，退回余额'
}

const withdrawalNeedsReason = (mode: WithdrawalMode) =>
  mode === 'reject' || mode === 'release' || mode === 'exception' || mode === 'resolve_unpaid'
const withdrawalNeedsReference = (mode: WithdrawalMode) =>
  mode === 'approve' || mode === 'exception' || mode === 'resolve_paid'

function WithdrawalsPanel() {
  const [status, setStatus] = useState('')
  const [riderInput, setRiderInput] = useState('')
  const [riderFilter, setRiderFilter] = useState('')
  const [filterError, setFilterError] = useState('')
  const [page, setPage] = useState(1)
  const [processingId, setProcessingId] = useState<string | null>(null)

  const withdrawalsState = useRemoteData(async () => {
    const result = await deliveryAdminAPI.riderWithdrawals({
      status: status || undefined,
      riderUserId: riderFilter || undefined,
      page,
      pageSize: DELIVERY_PAGE_SIZE
    })
    if (result.items.length === 0 && result.total > 0 && page > 1) {
      setPage(Math.max(1, Math.ceil(result.total / result.pageSize)))
    }
    return result
  }, [status, riderFilter, page])
  const capabilityState = useRemoteData(() => deliveryAdminAPI.riderWithdrawalCapabilities(), [])

  const capability = capabilityState.data
  const canWrite = capability?.withdrawalEnabled === true && capability.mode === 'manual' && capability.claimRequired === true

  const applyRiderFilter = () => {
    const trimmed = riderInput.trim()
    if (trimmed && (!/^\d+$/.test(trimmed) || Number(trimmed) <= 0)) {
      setFilterError('请输入有效的骑手用户 ID（正整数）')
      return
    }
    setFilterError('')
    setPage(1)
    setRiderFilter(trimmed)
  }

  const items = withdrawalsState.data?.items ?? []
  return <>
    <Panel eyebrow="WITHDRAWAL" title="骑手提现" description="人工打款流程：先领取处理权，再线下转账给骑手，最后回到这里登记付款凭证；登记不代表微信自动到账。">
      <div className="preview-capability-note">
        {canWrite
          ? '当前为人工打款流程：领取处理权后线下转账，再登记付款凭证。转账结果不确定时请保留凭证，不要重复转账。'
          : capabilityState.loading
            ? '正在读取提现能力…'
            : '提现登记暂未开放，当前只能查看，请勿据此付款。'}
      </div>
      <div className="preview-toolbar">
        <SelectField label="状态" value={status} onChange={event => { setStatus(event.target.value); setPage(1) }}>
          {WITHDRAWAL_STATUS_OPTIONS.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
        </SelectField>
        <Field
          label="骑手用户 ID"
          value={riderInput}
          inputMode="numeric"
          onChange={event => { setRiderInput(event.target.value); setFilterError('') }}
          placeholder="按骑手用户 ID 筛选"
          onKeyDown={event => { if (event.key === 'Enter') applyRiderFilter() }}
        />
        <Button variant="primary" onClick={applyRiderFilter}>查询</Button>
        <span className="preview-toolbar__hint">{withdrawalsState.loading ? '正在加载…' : `共 ${withdrawalsState.data?.total ?? 0} 条提现申请`}</span>
      </div>
      {filterError && <div className="preview-form-error" role="alert">{filterError}</div>}
      {capabilityState.error && <div className="preview-form-error" role="alert">{capabilityState.error}</div>}
      {withdrawalsState.error && <div className="preview-form-error" role="alert">{withdrawalsState.error}</div>}
      {!withdrawalsState.loading && items.length === 0
        ? <EmptyState title="没有提现申请" description="骑手申请提现后会出现在这里。" />
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="骑手提现列表">
          <thead><tr><th>申请</th><th>骑手</th><th>金额</th><th>状态</th><th>说明</th><th>申请时间</th><th /></tr></thead>
          <tbody>{items.map((item: RiderWithdrawal) => <tr key={item.withdrawPublicId} className="is-clickable" onClick={() => setProcessingId(item.withdrawPublicId)}>
            <td><strong>{shortId(item.withdrawPublicId)}</strong><span>v{item.rowVersion}</span></td>
            <td>骑手 #{item.riderUserId}</td>
            <td><strong>{formatMoney(item.amountCents)}</strong></td>
            <td><StatusPill tone={withdrawalStatusTone(item.status)}>{withdrawalStatusLabel(item.status)}</StatusPill></td>
            <td>{item.reason || '—'}</td>
            <td>{formatDateTime(item.createdAt)}</td>
            <td onClick={event => event.stopPropagation()}>{item.status === 'processing'
              ? <Button size="sm" variant="ghost" onClick={() => setProcessingId(item.withdrawPublicId)}>查看与处理</Button>
              : <Button size="sm" variant="ghost" onClick={() => setProcessingId(item.withdrawPublicId)}>查看详情</Button>}</td>
          </tr>)}</tbody>
        </table></div>}
      <Pagination
        page={withdrawalsState.data?.page ?? page}
        pageSize={withdrawalsState.data?.pageSize ?? DELIVERY_PAGE_SIZE}
        total={withdrawalsState.data?.total ?? 0}
        onChange={setPage}
        label="骑手提现分页"
      />
    </Panel>

    {processingId && <WithdrawalProcessModal
      withdrawalId={processingId}
      canWrite={canWrite}
      onClose={() => setProcessingId(null)}
      onChanged={() => withdrawalsState.reload()}
    />}
  </>
}

function WithdrawalProcessModal({ withdrawalId, canWrite, onClose, onChanged }: {
  withdrawalId: string
  canWrite: boolean
  onClose: () => void
  onChanged: () => void | Promise<void>
}) {
  const { notify } = usePreviewToast()
  const [detail, setDetail] = useState<RiderWithdrawalProcessing | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [mode, setMode] = useState<WithdrawalMode | null>(null)
  const [reason, setReason] = useState('')
  const [reference, setReference] = useState('')
  const [formError, setFormError] = useState('')
  const [busy, setBusy] = useState(false)
  const epoch = useRef(0)

  const load = useCallback(async () => {
    const version = ++epoch.current
    setLoading(true)
    setLoadError('')
    try {
      const value = await deliveryAdminAPI.riderWithdrawalProcessing(withdrawalId)
      if (epoch.current === version) setDetail(value)
    } catch (cause) {
      if (epoch.current === version) setLoadError(deliveryErrorMessage(cause))
    } finally {
      if (epoch.current === version) setLoading(false)
    }
  }, [withdrawalId])

  useEffect(() => { void load() }, [load])

  // 每个转账类动作在一次打开期间使用同一个幂等键：失败重试时复用，
  // 只有成功或重新进入模式才更新，避免"响应丢失后重试换新键"导致去重失效。
  const commandKeys = useRef<Map<WithdrawalMode, { key: string; claimKey: string }>>(new Map())
  const commandKeyFor = (next: WithdrawalMode) => {
    let entry = commandKeys.current.get(next)
    if (!entry) {
      entry = { key: newOperationKey(), claimKey: newOperationKey() }
      commandKeys.current.set(next, entry)
    }
    return entry
  }
  const startMode = (next: WithdrawalMode) => {
    commandKeys.current.delete(next)
    setMode(next)
    setReason('')
    setReference('')
    setFormError('')
  }

  const submit = async () => {
    const target = detail
    if (!target || !mode) return
    const row = target.withdrawal
    const trimmedReason = reason.trim()
    const trimmedReference = reference.trim()
    if (withdrawalNeedsReference(mode) && !trimmedReference) {
      setFormError('请填写付款凭证编号')
      return
    }
    if (withdrawalNeedsReason(mode) && !trimmedReason) {
      setFormError('请填写具体原因')
      return
    }
    // 幂等键在本次操作打开期间固定；失败重试复用同一键让服务端去重。
    const keys = commandKeyFor(mode)
    const command: RiderWithdrawalCommand = {
      expectedRowVersion: row.rowVersion,
      idempotencyKey: keys.key
    }
    if (mode === 'claim') command.claimKey = keys.claimKey
    else if (mode !== 'reject') command.claimKey = target.claimKey
    if (withdrawalNeedsReason(mode)) command.reason = trimmedReason
    if (withdrawalNeedsReference(mode)) command.paymentReference = trimmedReference
    if (mode === 'resolve_unpaid') command.confirmedUnpaid = true
    setBusy(true)
    setFormError('')
    try {
      if (mode === 'approve') {
        await deliveryAdminAPI.approveRiderWithdrawal(row.withdrawPublicId, row.rowVersion, command.idempotencyKey, trimmedReference, target.claimKey)
      } else if (mode === 'reject') {
        await deliveryAdminAPI.rejectRiderWithdrawal(row.withdrawPublicId, row.rowVersion, command.idempotencyKey, trimmedReason)
      } else {
        await deliveryAdminAPI.processRiderWithdrawal(row.withdrawPublicId, mode, command)
      }
      notify(`提现申请已执行：${withdrawalModeTitles[mode]}`)
      commandKeys.current.delete(mode)
      setMode(null)
      setReason('')
      setReference('')
      await load()
      await onChanged()
    } catch (cause) {
      setFormError(deliveryErrorMessage(cause) + (withdrawalNeedsReference(mode) ? '；如已付款或结果未知，请勿重复转账。' : ''))
    } finally {
      setBusy(false)
    }
  }

  const withdrawal = detail?.withdrawal
  const open = withdrawal?.status === 'processing'
  const claimable = detail !== null && (detail.status === 'unclaimed' || detail.status === 'released')
  const owned = detail?.ownedByMe === true && detail.status === 'active'
  const exceptionOwned = detail?.ownedByMe === true && detail.status === 'exception'
  const readOnlyNote = !open
    ? (detail ? '该提现申请已处理完成，仅可查看。' : '')
    : !canWrite
      ? '提现登记暂未开放，当前只能查看。'
      : detail?.status === 'exception' && !exceptionOwned
        ? '异常对账中（保持资金占用），需人工核对，本页不允许再次打款或退回余额。'
        : detail?.status === 'active' && !owned
          ? '该申请由其他管理员处理中，不能代替操作。'
          : ''

  return <Modal
    open
    title={`提现 ${shortId(withdrawalId)}`}
    description={withdrawal ? `骑手 #${withdrawal.riderUserId} · ${formatMoney(withdrawal.amountCents)} · ${withdrawalStatusLabel(withdrawal.status)}` : undefined}
    width="lg"
    onClose={() => { if (!busy) onClose() }}
    footer={<>
      <Button disabled={busy} onClick={onClose}>关闭</Button>
      <Button disabled={busy || loading || Boolean(mode)} onClick={() => void load()}>重新核对</Button>
    </>}
  >
    {loading && !detail && <p className="preview-toolbar__hint">正在加载提现处理详情…</p>}
    {loadError && <div className="preview-form-error" role="alert">{loadError}</div>}
    {detail && withdrawal && <>
      <div className="preview-kv-grid">
        <div><span>骑手</span><strong>骑手 #{withdrawal.riderUserId}</strong></div>
        <div><span>金额</span><strong>{formatMoney(withdrawal.amountCents)}</strong></div>
        <div><span>提现状态</span><strong>{withdrawalStatusLabel(withdrawal.status)}</strong></div>
        <div><span>处理状态</span><strong>{withdrawalProcessingLabels[detail.status] || detail.status}</strong></div>
        <div><span>处理人</span><strong>{detail.operatorUserId ? `#${detail.operatorUserId}${detail.ownedByMe ? '（本人）' : ''}` : '—'}</strong></div>
        <div><span>付款凭证</span><strong>{detail.paymentReference || '—'}</strong></div>
        <div><span>处理说明</span><strong>{detail.reason || withdrawal.reason || '—'}</strong></div>
        <div><span>申请时间</span><strong>{formatDateTime(withdrawal.createdAt)}</strong></div>
      </div>

      {readOnlyNote && <div className="preview-capability-note">{readOnlyNote}</div>}

      {mode ? <div className="preview-form-stack" style={{ marginTop: 12 }}>
        <strong>{withdrawalModeTitles[mode]}</strong>
        {withdrawalNeedsReference(mode) && <Field
          label={mode === 'exception' ? '付款凭证编号（或“结果未知”）' : '付款凭证编号'}
          required
          value={reference}
          onChange={event => { setReference(event.target.value); setFormError('') }}
          placeholder="仅登记已完成的线下转账，不会再次转账"
          hint={mode === 'resolve_paid' ? '用于登记此前已经完成的人工转账，请填写原付款凭证。' : '请填写可核对的转账凭证编号。'}
        />}
        {withdrawalNeedsReason(mode) && <Field
          label="具体原因"
          required
          value={reason}
          onChange={event => { setReason(event.target.value); setFormError('') }}
          placeholder={mode === 'reject' ? '例如：收款信息有误' : mode === 'resolve_unpaid' ? '说明为何确认未转账' : '说明处理情况'}
        />}
        {formError && <div className="preview-form-error">{formError}</div>}
        <div className="preview-row-actions">
          <Button disabled={busy} onClick={() => { setMode(null); setFormError('') }}>返回</Button>
          <Button variant={mode === 'reject' || mode === 'resolve_unpaid' ? 'danger' : 'primary'} disabled={busy} onClick={() => void submit()}>
            {busy ? '提交中…' : `确认${withdrawalModeTitles[mode]}`}
          </Button>
        </div>
      </div> : canWrite && open && <div className="preview-row-actions" style={{ marginTop: 12 }}>
        {claimable && <>
          <Button variant="primary" onClick={() => startMode('claim')}>领取处理权</Button>
          <Button variant="danger" onClick={() => startMode('reject')}>拒绝并退回余额</Button>
        </>}
        {owned && <>
          <Button variant="primary" onClick={() => startMode('approve')}>登记已实际打款</Button>
          <Button onClick={() => startMode('release')}>未转账，解除处理</Button>
          <Button variant="danger" onClick={() => startMode('exception')}>结果未知 / 异常留证</Button>
        </>}
        {exceptionOwned && <>
          <Button variant="primary" onClick={() => startMode('resolve_paid')}>已有凭证，登记已打款</Button>
          <Button variant="danger" onClick={() => startMode('resolve_unpaid')}>确认未转账，退回余额</Button>
        </>}
      </div>}
    </>}
  </Modal>
}

/* ===== 退款记录 ===== */

const REFUND_STATUS_OPTIONS: Array<[string, string]> = [
  ['', '全部状态'],
  ['pending', '处理中'],
  ['succeeded', '已退款'],
  ['failed', '退款失败'],
  ['refund_pending', '等待重试']
]

function RefundsPanel() {
  const [status, setStatus] = useState('')
  const [merchantInput, setMerchantInput] = useState('')
  const [merchantFilter, setMerchantFilter] = useState('')
  const [filterError, setFilterError] = useState('')
  const [page, setPage] = useState(1)
  const [detail, setDetail] = useState<Refund | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState('')
  const detailEpoch = useRef(0)

  const refundsState = useRemoteData(async () => {
    const result = await deliveryAdminAPI.refunds({
      status: status || undefined,
      merchantUserId: merchantFilter || undefined,
      page,
      pageSize: DELIVERY_PAGE_SIZE
    })
    if (result.items.length === 0 && result.total > 0 && page > 1) {
      setPage(Math.max(1, Math.ceil(result.total / result.pageSize)))
    }
    return result
  }, [status, merchantFilter, page])

  const applyMerchantFilter = () => {
    const trimmed = merchantInput.trim()
    if (trimmed && (!/^\d+$/.test(trimmed) || Number(trimmed) <= 0)) {
      setFilterError('请输入有效的商家用户 ID（正整数）')
      return
    }
    setFilterError('')
    setPage(1)
    setMerchantFilter(trimmed)
  }

  const openDetail = async (id: string) => {
    const version = ++detailEpoch.current
    setDetail(null)
    setDetailError('')
    setDetailLoading(true)
    try {
      const value = await deliveryAdminAPI.refund(id)
      if (detailEpoch.current === version) setDetail(value)
    } catch (reason) {
      if (detailEpoch.current === version) setDetailError(deliveryErrorMessage(reason))
    } finally {
      if (detailEpoch.current === version) setDetailLoading(false)
    }
  }

  const refunds = refundsState.data?.items ?? []
  return <>
    <Panel eyebrow="REFUND" title="退款记录" description="退款由支付通道处理，这里只查看真实结果，不能直接发起或伪造退款。">
      <div className="preview-toolbar">
        <SelectField label="状态" value={status} onChange={event => { setStatus(event.target.value); setPage(1) }}>
          {REFUND_STATUS_OPTIONS.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
        </SelectField>
        <Field
          label="商家用户 ID"
          value={merchantInput}
          inputMode="numeric"
          onChange={event => { setMerchantInput(event.target.value); setFilterError('') }}
          placeholder="按商家用户 ID 筛选"
          onKeyDown={event => { if (event.key === 'Enter') applyMerchantFilter() }}
        />
        <Button variant="primary" onClick={applyMerchantFilter}>查询</Button>
        <span className="preview-toolbar__hint">{refundsState.loading ? '正在加载…' : `共 ${refundsState.data?.total ?? 0} 条退款记录`}</span>
      </div>
      {filterError && <div className="preview-form-error" role="alert">{filterError}</div>}
      {refundsState.error && <div className="preview-form-error" role="alert">{refundsState.error}</div>}
      {!refundsState.loading && refunds.length === 0
        ? <EmptyState title="没有退款记录" description="支付通道产生退款后会出现在这里。" />
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="退款记录列表">
          <thead><tr><th>退款单</th><th>配送单</th><th>商家</th><th>金额</th><th>结果</th><th>原因</th><th>更新时间</th></tr></thead>
          <tbody>{refunds.map(item => <tr key={item.refundPublicId} className="is-clickable" onClick={() => void openDetail(item.refundPublicId)}>
            <td><button type="button" className="preview-table__primary" onClick={event => { event.stopPropagation(); void openDetail(item.refundPublicId) }}>{shortId(item.refundPublicId)}<small>{item.outRefundNo}</small></button></td>
            <td>{shortId(item.deliveryOrderPublicId)}</td>
            <td>商家 #{item.merchantUserId}</td>
            <td><strong>{formatMoney(item.amountCents)}</strong></td>
            <td><StatusPill tone={refundStatusTone(item.status)}>{refundStatusLabel(item.status)}</StatusPill></td>
            <td>{item.failureReason || '—'}</td>
            <td>{formatDateTime(item.updatedAt)}</td>
          </tr>)}</tbody>
        </table></div>}
      <Pagination
        page={refundsState.data?.page ?? page}
        pageSize={refundsState.data?.pageSize ?? DELIVERY_PAGE_SIZE}
        total={refundsState.data?.total ?? 0}
        onChange={setPage}
        label="退款记录分页"
      />
    </Panel>

    <Drawer
      open={detail !== null || detailLoading || Boolean(detailError)}
      title={detail ? `退款 ${shortId(detail.refundPublicId)}` : '退款详情'}
      description={detail ? `配送单 ${shortId(detail.deliveryOrderPublicId)} · 商家 #${detail.merchantUserId}` : undefined}
      onClose={() => { detailEpoch.current += 1; setDetail(null); setDetailError(''); setDetailLoading(false) }}
      footer={<Button onClick={() => { detailEpoch.current += 1; setDetail(null); setDetailError(''); setDetailLoading(false) }}>关闭</Button>}
    >
      {detailLoading && !detail && <p className="preview-toolbar__hint">正在加载退款详情…</p>}
      {detailError && <div className="preview-form-error" role="alert">{detailError}</div>}
      {detail && <div className="preview-detail-stack">
        <div className="preview-detail-banner">
          <div><span>退款结果</span><strong>{refundStatusLabel(detail.status)}</strong></div>
          <StatusPill tone={refundStatusTone(detail.status)}>{refundStatusLabel(detail.status)}</StatusPill>
        </div>
        <div className="preview-kv-grid">
          <div><span>退款金额</span><strong>{formatMoney(detail.amountCents)}</strong></div>
          <div><span>退款单号</span><strong>{detail.outRefundNo}</strong></div>
          <div><span>结算单</span><strong>{shortId(detail.settlementPublicId)}</strong></div>
          <div><span>结算来源</span><strong>{settlementSourceLabel(detail.source)}</strong></div>
          <div><span>通道状态</span><strong>{detail.providerStatus || '—'}</strong></div>
          <div><span>错误码</span><strong>{detail.errorCode || '—'}</strong></div>
          <div><span>失败原因</span><strong>{detail.failureReason || '—'}</strong></div>
          <div><span>核实次数</span><strong>{detail.verifyAttemptCount}</strong></div>
          <div><span>创建时间</span><strong>{formatDateTime(detail.createdAt)}</strong></div>
          <div><span>更新时间</span><strong>{formatDateTime(detail.updatedAt)}</strong></div>
          <div><span>成功时间</span><strong>{formatDateTime(detail.succeededAt)}</strong></div>
          <div><span>版本</span><strong>v{detail.rowVersion}</strong></div>
        </div>
        <div className="preview-capability-note">退款记录仅用于核对状态，后台不会在此页面直接发起或伪造退款。</div>
      </div>}
    </Drawer>
  </>
}

/* ===== 资金核对 ===== */

const coverageDirectionText: Record<string, string> = {
  balanced: '账目一致',
  untracked_wallet_balance: '钱包存在未登记余额',
  source_exceeds_wallet: '来源金额高于钱包余额',
  data_error: '来源数据异常'
}

function FundingPanel() {
  const fundingState = useRemoteData(() => deliveryAdminAPI.fundingSources(), [])
  const value: FundingSourceSummary | null = fundingState.data
  const balanced = value?.coverageDirection === 'balanced'
  return <Panel eyebrow="FUNDING" title="资金核对" description="钱包余额与已核实来源逐项对比，差异项需要人工排查；本页只读，不会修改支付或退款状态。">
    {fundingState.error && <div className="preview-form-error" role="alert">{fundingState.error}</div>}
    {fundingState.loading && !value && <p className="preview-toolbar__hint">正在加载资金核对数据…</p>}
    {value && <>
      {!value.fundingTablesAvailable && <div className="preview-capability-note">资金来源明细表暂不可用：当前只显示已有钱包余额，历史余额尚未核对。</div>}
      <div className="preview-table-wrap"><table className="preview-table" aria-label="资金核对">
        <thead><tr><th>核对项</th><th>金额</th><th>说明</th></tr></thead>
        <tbody>
          <tr><td><strong>钱包余额</strong></td><td><strong>{formatMoney(value.walletBalanceCents)}</strong></td><td>商家钱包 + 骑手钱包实际余额</td></tr>
          <tr><td><strong>可核实来源</strong></td><td>{formatMoney(value.knownAvailableCents)}</td><td>入账批次 {value.fundingLots} · 已分配 {value.fundingAllocations}</td></tr>
          <tr><td><strong>其中已核验</strong></td><td>{formatMoney(value.verifiedAvailableCents)}</td><td>资金来源已与支付凭证核对</td></tr>
          <tr><td><strong>待核验 / 历史未知</strong></td><td>{formatMoney(value.unverifiedAvailableCents)}</td><td>尚未完成核验的资金来源</td></tr>
          <tr><td><strong>差异</strong></td><td>{formatMoney(value.coverageGapCents)}</td><td>覆盖缺口，需人工排查</td></tr>
        </tbody>
      </table></div>
      <div className="preview-toolbar" style={{ marginTop: 12 }}>
        <span className="preview-toolbar__hint">
          核对结论：{coverageDirectionText[value.coverageDirection] || '需人工核对'} · 支付待核实 {value.paymentVerificationPending} 笔 · 退款待核实 {value.refundVerificationPending} 笔 · 支付错误码 {value.paymentVerificationErrorCount} · 退款错误码 {value.refundVerificationErrorCount}
        </span>
        <StatusPill tone={balanced ? 'success' : 'danger'}>{balanced ? '已一致' : '需核对'}</StatusPill>
      </div>
    </>}
  </Panel>
}

/* ===== 费率配置 ===== */

export function FeeSection({ value, onChange }: { value: FeeView; onChange: (value: FeeView) => void }) {
  return <>
    <div className="preview-subtabs" role="tablist" aria-label="费率模块">
      {([['default', '默认配送费'], ['override', '商家专属配送费']] as const).map(([key, label]) => <button
        key={key}
        type="button"
        role="tab"
        aria-selected={value === key}
        className={value === key ? 'active' : ''}
        onClick={() => onChange(key)}
      >{label}</button>)}
    </div>

    {value === 'default' && <DefaultFeePanel />}
    {value === 'override' && <MerchantOverridePanel />}
  </>
}

function DefaultFeePanel() {
  const { notify } = usePreviewToast()
  const feeState = useRemoteData(() => deliveryAdminAPI.feeConfig(), [])
  const [deliveryFee, setDeliveryFee] = useState('')
  const [platformCut, setPlatformCut] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const config: FeeConfig | null = feeState.data

  useEffect(() => {
    if (!config) return
    setDeliveryFee(String(config.deliveryFeeCents / 100))
    setPlatformCut(String(config.platformCutCents / 100))
    setError('')
  }, [config])

  const save = async () => {
    if (!config) return
    const deliveryCents = Math.round(Number(deliveryFee) * 100)
    const cutCents = Math.round(Number(platformCut) * 100)
    if (!Number.isFinite(deliveryCents) || deliveryCents <= 0 || !Number.isFinite(cutCents) || cutCents < 0) {
      setError('请填写有效的金额（配送费需大于 0）')
      return
    }
    if (cutCents >= deliveryCents) {
      setError('平台抽成必须低于配送费')
      return
    }
    setSaving(true)
    setError('')
    try {
      await deliveryAdminAPI.updateFeeConfig(deliveryCents, cutCents, config.rowVersion)
      notify('默认配送费已保存，仅影响之后新建的订单')
      await feeState.reload()
    } catch (reason) {
      if (isDeliveryConflict(reason)) {
        // 冲突说明行版本已过期：重新载入配置（同步输入框），用 toast 保留提示，
        // 因为 [config] 变化时表单 effect 会清空局部 error。
        await feeState.reload()
        notify(conflictMessage + '，已载入最新配置', 'danger')
      } else {
        setError(deliveryErrorMessage(reason))
      }
    } finally {
      setSaving(false)
    }
  }

  return <Panel eyebrow="DEFAULT FEE" title="默认配送费" description="新订单使用这里的默认值；已提交的订单按下单时的金额计算。保存会携带当前行版本，避免覆盖他人修改。">
    {feeState.error && <div className="preview-form-error" role="alert">{feeState.error}</div>}
    {feeState.loading && !config && <p className="preview-toolbar__hint">正在加载费率配置…</p>}
    {config && <>
      <div className="preview-fee-form">
        <Field label="配送费（元）" required value={deliveryFee} onChange={event => { setDeliveryFee(event.target.value); setError('') }} />
        <Field label="平台抽成（元）" required value={platformCut} onChange={event => { setPlatformCut(event.target.value); setError('') }} />
        <div className="preview-fee-form__meta">配置范围 {config.configScope === 'global' ? '全局' : config.configScope} · 版本 {config.rowVersion} · 最近更新 {formatDateTime(config.updatedAt)}</div>
      </div>
      {error && <div className="preview-form-error">{error}</div>}
      <div className="preview-fee-form__actions">
        <Button variant="primary" disabled={saving} onClick={() => void save()}>{saving ? '保存中…' : '保存配置'}</Button>
        <Button disabled={saving} onClick={() => {
          setDeliveryFee(String(config.deliveryFeeCents / 100))
          setPlatformCut(String(config.platformCutCents / 100))
          setError('')
        }}>恢复已保存值</Button>
      </div>
    </>}
  </Panel>
}

const OVERRIDE_STATUS_OPTIONS: Array<[string, string]> = [
  ['', '全部状态'],
  ['active', '生效中'],
  ['disabled', '已停用']
]

function MerchantOverridePanel() {
  const { notify } = usePreviewToast()
  const [statusFilter, setStatusFilter] = useState('')
  const [page, setPage] = useState(1)
  const [merchantInput, setMerchantInput] = useState('')
  const [feeInput, setFeeInput] = useState('')
  const [cutInput, setCutInput] = useState('')
  const [expectedVersion, setExpectedVersion] = useState('')
  const [formError, setFormError] = useState('')
  const [saving, setSaving] = useState(false)
  const [disableTarget, setDisableTarget] = useState<MerchantFeeOverride | null>(null)
  const [disableError, setDisableError] = useState('')
  const [disableBusy, setDisableBusy] = useState(false)

  const overridesState = useRemoteData(async () => {
    const result = await deliveryAdminAPI.merchantFeeOverrides({
      status: statusFilter || undefined,
      page,
      pageSize: DELIVERY_PAGE_SIZE
    })
    if (result.items.length === 0 && result.total > 0 && page > 1) {
      setPage(Math.max(1, Math.ceil(result.total / result.pageSize)))
    }
    return result
  }, [statusFilter, page])

  const clearForm = () => {
    setMerchantInput('')
    setFeeInput('')
    setCutInput('')
    setExpectedVersion('')
    setFormError('')
  }

  const editRow = (row: MerchantFeeOverride) => {
    setMerchantInput(String(row.merchantUserId))
    setFeeInput(String(row.deliveryFeeCents / 100))
    setCutInput(String(row.platformCutCents / 100))
    setExpectedVersion(String(row.rowVersion))
    setFormError('')
  }

  const save = async () => {
    const merchantUserId = Number(merchantInput.trim())
    const deliveryCents = Math.round(Number(feeInput) * 100)
    const cutCents = Math.round(Number(cutInput) * 100)
    if (!Number.isSafeInteger(merchantUserId) || merchantUserId <= 0) {
      setFormError('请填写有效的商家用户 ID（正整数）')
      return
    }
    if (!feeInput.trim() || !Number.isFinite(deliveryCents) || deliveryCents <= 0
      || !cutInput.trim() || !Number.isFinite(cutCents) || cutCents < 0) {
      setFormError('请填写有效的配送费与平台抽成（元），配送费需大于 0')
      return
    }
    if (cutCents >= deliveryCents) {
      setFormError('平台抽成必须低于配送费')
      return
    }
    const expected = expectedVersion.trim() ? Number(expectedVersion.trim()) : undefined
    if (expected !== undefined && (!Number.isSafeInteger(expected) || expected <= 0)) {
      setFormError('版本号必须是正整数')
      return
    }
    setSaving(true)
    setFormError('')
    try {
      await deliveryAdminAPI.upsertMerchantFeeOverride(merchantUserId, deliveryCents, cutCents, expected)
      notify(expected ? `已更新商家 #${merchantUserId} 的专属配送费` : `已新增商家 #${merchantUserId} 的专属配送费`)
      clearForm()
      await overridesState.reload()
    } catch (cause) {
      setFormError(deliveryErrorMessage(cause))
    } finally {
      setSaving(false)
    }
  }

  const submitDisable = async () => {
    const target = disableTarget
    if (!target) return
    setDisableBusy(true)
    setDisableError('')
    try {
      await deliveryAdminAPI.disableMerchantFeeOverride(target.merchantUserId, target.rowVersion)
      notify(`已停用商家 #${target.merchantUserId} 的专属配送费`, 'info')
      setDisableTarget(null)
      await overridesState.reload()
    } catch (cause) {
      if (isDeliveryConflict(cause)) {
        // 弹窗里的行版本已失效：关闭弹窗、刷新列表，让用户基于最新数据重新操作。
        setDisableTarget(null)
        await overridesState.reload()
        notify('该专属费率已被他人修改，列表已刷新，请重新打开停用', 'danger')
      } else {
        setDisableError(deliveryErrorMessage(cause))
      }
    } finally {
      setDisableBusy(false)
    }
  }

  const overrides = overridesState.data?.items ?? []
  return <>
    <Panel eyebrow="MERCHANT OVERRIDE" title="商家专属配送费" description="为个别商家覆盖默认配送费；停用后立即回到默认值。修改已有商家时必须携带版本号。">
      <div className="preview-fee-form">
        <Field label="商家用户 ID" required value={merchantInput} inputMode="numeric" onChange={event => { setMerchantInput(event.target.value); setFormError('') }} placeholder="例如：1001" />
        <Field label="配送费（元）" required value={feeInput} onChange={event => { setFeeInput(event.target.value); setFormError('') }} />
        <Field label="平台抽成（元）" required value={cutInput} onChange={event => { setCutInput(event.target.value); setFormError('') }} />
        <Field label="版本号" value={expectedVersion} inputMode="numeric" onChange={event => { setExpectedVersion(event.target.value); setFormError('') }} placeholder="修改已有商家时填写" hint="新增商家可留空；编辑已有配置时必须填写当前版本。" />
      </div>
      {formError && <div className="preview-form-error">{formError}</div>}
      <div className="preview-fee-form__actions">
        <Button variant="primary" disabled={saving} onClick={() => void save()}>{saving ? '保存中…' : '保存配置'}</Button>
        <Button disabled={saving} onClick={clearForm}>清空编辑</Button>
      </div>

      <div className="preview-toolbar" style={{ marginTop: 16 }}>
        <SelectField label="状态" value={statusFilter} onChange={event => { setStatusFilter(event.target.value); setPage(1) }}>
          {OVERRIDE_STATUS_OPTIONS.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
        </SelectField>
        <span className="preview-toolbar__hint">{overridesState.loading ? '正在加载…' : `共 ${overridesState.data?.total ?? 0} 条专属配置`}</span>
      </div>
      {overridesState.error && <div className="preview-form-error" role="alert">{overridesState.error}</div>}
      {!overridesState.loading && overrides.length === 0
        ? <EmptyState title="没有商家专属配置" description="默认配送费适用于所有商家。" />
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="商家专属配送费列表">
          <thead><tr><th>商家</th><th>配送费</th><th>平台抽成</th><th>状态</th><th>版本 / 更新时间</th><th /></tr></thead>
          <tbody>{overrides.map(item => <tr key={item.merchantUserId}>
            <td><strong>商家 #{item.merchantUserId}</strong></td>
            <td>{formatMoney(item.deliveryFeeCents)}</td>
            <td>{formatMoney(item.platformCutCents)}</td>
            <td><StatusPill tone={feeStatusTone(item.status)}>{feeStatusLabel(item.status)}</StatusPill></td>
            <td>v{item.rowVersion}<span>{formatDateTime(item.updatedAt)}</span></td>
            <td onClick={event => event.stopPropagation()}><div className="preview-row-actions">
              <Button size="sm" variant="ghost" onClick={() => editRow(item)}>编辑</Button>
              {item.status !== 'disabled' && <Button size="sm" variant="danger" onClick={() => { setDisableTarget(item); setDisableError('') }}>停用</Button>}
            </div></td>
          </tr>)}</tbody>
        </table></div>}
      <Pagination
        page={overridesState.data?.page ?? page}
        pageSize={overridesState.data?.pageSize ?? DELIVERY_PAGE_SIZE}
        total={overridesState.data?.total ?? 0}
        onChange={setPage}
        label="商家专属配送费分页"
      />
    </Panel>

    <Modal
      open={Boolean(disableTarget)}
      title="停用商家专属配送费"
      description={disableTarget ? `商家 #${disableTarget.merchantUserId} 将恢复使用默认配送费。` : undefined}
      onClose={() => { if (!disableBusy) { setDisableTarget(null); setDisableError('') } }}
      footer={<>
        <Button disabled={disableBusy} onClick={() => { setDisableTarget(null); setDisableError('') }}>取消</Button>
        <Button variant="danger" disabled={disableBusy} onClick={() => void submitDisable()}>{disableBusy ? '停用中…' : '确认停用'}</Button>
      </>}
    >
      <div className="preview-form-stack">
        <div className="preview-capability-note">停用后，该商家之后新建的订单会立即回到默认配送费；已提交的订单不受影响。</div>
        {disableError && <div className="preview-form-error">{disableError}</div>}
      </div>
    </Modal>
  </>
}
