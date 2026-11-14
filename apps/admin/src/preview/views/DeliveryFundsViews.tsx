import { useState } from 'react'
import { formatMoney, formatTime } from '../data'
import { usePreviewStore } from '../store'
import { Button, EmptyState, Field, Modal, Panel, StatusPill, usePreviewToast } from '../ui'

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
  const merchantWallets = usePreviewStore(state => state.merchantWallets)
  const riderWallets = usePreviewStore(state => state.riderWallets)
  const earnings = usePreviewStore(state => state.riderEarnings)
  const payoutQueue = usePreviewStore(state => state.payoutQueue)
  const withdrawals = usePreviewStore(state => state.withdrawals)
  const refunds = usePreviewStore(state => state.refunds)
  const funding = usePreviewStore(state => state.fundingSources)
  const payWithdrawal = usePreviewStore(state => state.payWithdrawal)
  const rejectWithdrawal = usePreviewStore(state => state.rejectWithdrawal)
  const markPayoutPaid = usePreviewStore(state => state.markPayoutPaid)
  const [pending, setPending] = useState<string | null>(null)
  const [reference, setReference] = useState('')
  const { notify } = usePreviewToast()

  const submitPay = () => {
    if (!pending || !reference.trim()) return
    payWithdrawal(pending, reference.trim())
    notify('已登记打款，请保留线下转账凭证')
    setPending(null)
    setReference('')
  }

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

    {value === 'merchant' && <Panel eyebrow="MERCHANT WALLET" title="商家钱包" description="余额来自商家充值，用于支付配送费；冻结金额是已下单未完成的部分。">
      <div className="preview-table-wrap"><table className="preview-table" aria-label="商家钱包列表">
        <thead><tr><th>商家</th><th>可用余额</th><th>冻结</th><th>更新时间</th></tr></thead>
        <tbody>{merchantWallets.map(item => <tr key={item.merchantId}>
          <td><strong>{item.name}</strong><span>{item.merchantId}</span></td>
          <td><strong>{formatMoney(item.balanceCents)}</strong></td>
          <td>{formatMoney(item.frozenCents)}</td>
          <td>{formatTime(item.updatedAt)}</td>
        </tr>)}</tbody>
      </table></div>
    </Panel>}

    {value === 'rider' && <Panel eyebrow="RIDER WALLET" title="骑手钱包" description="可用余额可申请提现；待结金额是已产生但尚未结算的报酬。">
      <div className="preview-table-wrap"><table className="preview-table" aria-label="骑手钱包列表">
        <thead><tr><th>骑手</th><th>可用余额</th><th>待结</th><th>更新时间</th></tr></thead>
        <tbody>{riderWallets.map(item => <tr key={item.riderId}>
          <td><strong>{item.name}</strong><span>{item.riderId}</span></td>
          <td><strong>{formatMoney(item.balanceCents)}</strong></td>
          <td>{formatMoney(item.pendingCents)}</td>
          <td>{formatTime(item.updatedAt)}</td>
        </tr>)}</tbody>
      </table></div>
    </Panel>}

    {value === 'earnings' && <Panel eyebrow="RIDER EARNING" title="骑手报酬" description="每笔配送完成后的报酬记录，次日 00:00 自动结算到骑手钱包。">
      <div className="preview-table-wrap"><table className="preview-table" aria-label="骑手报酬列表">
        <thead><tr><th>记录</th><th>骑手</th><th>配送单</th><th>金额</th><th>状态</th><th>产生时间</th></tr></thead>
        <tbody>{earnings.map(item => <tr key={item.id}>
          <td><strong>{item.id}</strong></td>
          <td>{item.rider}</td>
          <td>{item.orderId}</td>
          <td><strong>{formatMoney(item.amountCents)}</strong></td>
          <td><StatusPill>{item.status}</StatusPill></td>
          <td>{formatTime(item.createdAt)}</td>
        </tr>)}</tbody>
      </table></div>
    </Panel>}

    {value === 'payout' && <Panel eyebrow="PAYOUT QUEUE" title="待打款" description="骑手已申请、等待线下转账的报酬。登记后才会从队列移除。">
      {payoutQueue.length === 0
        ? <EmptyState title="没有待打款记录" description="骑手申请提现后会出现在这里。" />
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="待打款列表">
          <thead><tr><th>编号</th><th>骑手</th><th>金额</th><th>状态</th><th>申请时间</th><th /></tr></thead>
          <tbody>{payoutQueue.map(item => <tr key={item.id}>
            <td><strong>{item.id}</strong></td>
            <td>{item.rider}</td>
            <td><strong>{formatMoney(item.amountCents)}</strong></td>
            <td><StatusPill>{item.status}</StatusPill></td>
            <td>{formatTime(item.requestedAt)}</td>
            <td>{item.status === '排队中' && <Button size="sm" variant="ghost" onClick={() => { markPayoutPaid(item.id); notify('已标记为已打款') }}>标记已打款</Button>}</td>
          </tr>)}</tbody>
        </table></div>}
    </Panel>}

    {value === 'withdrawals' && <Panel eyebrow="WITHDRAWAL" title="骑手提现" description="平台不会自动打款：先线下转账，再回到这里登记付款凭证。">
      <div className="preview-capability-note">当前为人工打款流程，登记后不代表微信到账；请保留转账回单以便核对。</div>
      <div className="preview-table-wrap"><table className="preview-table" aria-label="骑手提现列表">
        <thead><tr><th>申请</th><th>骑手</th><th>金额</th><th>状态</th><th>凭证 / 说明</th><th>申请时间</th><th /></tr></thead>
        <tbody>{withdrawals.map(item => <tr key={item.id}>
          <td><strong>{item.id}</strong></td>
          <td>{item.rider}</td>
          <td><strong>{formatMoney(item.amountCents)}</strong></td>
          <td><StatusPill>{item.status}</StatusPill></td>
          <td>{item.reference || item.note || '—'}</td>
          <td>{formatTime(item.createdAt)}</td>
          <td>{item.status === '待处理' && <div className="preview-row-actions">
            <Button size="sm" variant="primary" onClick={() => { setPending(item.id); setReference('') }}>登记打款</Button>
            <Button size="sm" variant="danger" onClick={() => { rejectWithdrawal(item.id, '收款信息有误'); notify('已退回申请', 'info') }}>退回</Button>
          </div>}</td>
        </tr>)}</tbody>
      </table></div>
    </Panel>}

    {value === 'refunds' && <Panel eyebrow="REFUND" title="退款记录" description="退款由支付通道处理，这里只查看结果，不能直接发起或伪造。">
      <div className="preview-table-wrap"><table className="preview-table" aria-label="退款记录列表">
        <thead><tr><th>退款单</th><th>配送单</th><th>金额</th><th>结果</th><th>原因</th><th>时间</th></tr></thead>
        <tbody>{refunds.map(item => <tr key={item.id}>
          <td><strong>{item.id}</strong></td>
          <td>{item.orderId}</td>
          <td><strong>{formatMoney(item.amountCents)}</strong></td>
          <td><StatusPill>{item.status}</StatusPill></td>
          <td>{item.reason}</td>
          <td>{formatTime(item.createdAt)}</td>
        </tr>)}</tbody>
      </table></div>
    </Panel>}

    {value === 'funding' && <Panel eyebrow="FUNDING" title="资金核对" description="预期金额与实际金额逐项对比，差异项需要人工排查。">
      <div className="preview-table-wrap"><table className="preview-table" aria-label="资金来源核对">
        <thead><tr><th>来源</th><th>预期</th><th>实际</th><th>差异</th><th>结论</th></tr></thead>
        <tbody>{funding.map(item => {
          const diff = item.actualCents - item.expectedCents
          return <tr key={item.name}>
            <td><strong>{item.name}</strong></td>
            <td>{formatMoney(item.expectedCents)}</td>
            <td>{formatMoney(item.actualCents)}</td>
            <td>{diff === 0 ? '—' : formatMoney(diff)}</td>
            <td><StatusPill tone={diff === 0 ? 'success' : 'danger'}>{diff === 0 ? '已一致' : '需核对'}</StatusPill></td>
          </tr>
        })}</tbody>
      </table></div>
    </Panel>}

    <Modal
      open={Boolean(pending)}
      title="登记打款凭证"
      description="请先完成线下转账，再填写可核对的凭证信息。"
      onClose={() => setPending(null)}
      footer={<>
        <Button onClick={() => setPending(null)}>取消</Button>
        <Button variant="primary" disabled={!reference.trim()} onClick={submitPay}>确认已打款</Button>
      </>}
    >
      <div className="preview-form-stack">
        <Field
          label="付款凭证"
          required
          value={reference}
          onChange={event => setReference(event.target.value)}
          placeholder="例如：转账单号或回单编号"
          hint="凭证会保存在提现记录中，便于事后核对。"
        />
      </div>
    </Modal>
  </>
}

export function FeeSection({ value, onChange }: { value: FeeView; onChange: (value: FeeView) => void }) {
  const feeConfig = usePreviewStore(state => state.feeConfig)
  const overrides = usePreviewStore(state => state.merchantOverrides)
  const updateFeeConfig = usePreviewStore(state => state.updateFeeConfig)
  const toggleOverride = usePreviewStore(state => state.toggleMerchantOverride)
  const [deliveryFee, setDeliveryFee] = useState(String(feeConfig.deliveryFeeCents / 100))
  const [platformCut, setPlatformCut] = useState(String(feeConfig.platformCutCents / 100))
  const [error, setError] = useState('')
  const { notify } = usePreviewToast()

  const save = () => {
    const deliveryCents = Math.round(Number(deliveryFee) * 100)
    const cutCents = Math.round(Number(platformCut) * 100)
    if (!Number.isFinite(deliveryCents) || deliveryCents < 0 || !Number.isFinite(cutCents) || cutCents < 0) {
      setError('请填写有效的金额')
      return
    }
    if (cutCents > deliveryCents) {
      setError('平台抽成不能高于配送费')
      return
    }
    setError('')
    updateFeeConfig(deliveryCents, cutCents)
    notify('默认运费已保存，仅影响之后新建的订单')
  }

  return <>
    <div className="preview-subtabs" role="tablist" aria-label="费率模块">
      {([['default', '默认运费'], ['override', '商家专属运费']] as const).map(([key, label]) => <button
        key={key}
        type="button"
        role="tab"
        aria-selected={value === key}
        className={value === key ? 'active' : ''}
        onClick={() => onChange(key)}
      >{label}</button>)}
    </div>

    {value === 'default' && <Panel eyebrow="DEFAULT FEE" title="默认运费" description="新订单使用这里的默认值；已提交的订单按下单时的金额计算。">
      <div className="preview-fee-form">
        <Field label="配送费（元）" required value={deliveryFee} onChange={event => setDeliveryFee(event.target.value)} />
        <Field label="平台抽成（元）" required value={platformCut} onChange={event => setPlatformCut(event.target.value)} />
        <div className="preview-fee-form__meta">版本 {feeConfig.rowVersion} · 最近更新 {formatTime(feeConfig.updatedAt)}</div>
      </div>
      {error && <div className="preview-form-error">{error}</div>}
      <div className="preview-fee-form__actions">
        <Button variant="primary" onClick={save}>保存配置</Button>
        <Button onClick={() => {
          setDeliveryFee(String(feeConfig.deliveryFeeCents / 100))
          setPlatformCut(String(feeConfig.platformCutCents / 100))
          setError('')
        }}>恢复已保存值</Button>
      </div>
    </Panel>}

    {value === 'override' && <Panel eyebrow="MERCHANT OVERRIDE" title="商家专属运费" description="为个别商家覆盖默认运费；停用后立即回到默认值。">
      {overrides.length === 0
        ? <EmptyState title="没有商家专属配置" description="默认运费适用于所有商家。" />
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="商家专属运费列表">
          <thead><tr><th>商家</th><th>配送费</th><th>平台抽成</th><th>状态</th><th /></tr></thead>
          <tbody>{overrides.map(item => <tr key={item.merchantId}>
            <td><strong>{item.name}</strong><span>{item.merchantId}</span></td>
            <td>{formatMoney(item.deliveryFeeCents)}</td>
            <td>{formatMoney(item.platformCutCents)}</td>
            <td><StatusPill>{item.enabled ? '已启用' : '已停用'}</StatusPill></td>
            <td><Button size="sm" variant="ghost" onClick={() => {
              toggleOverride(item.merchantId)
              notify(item.enabled ? '已停用该商家专属运费' : '已启用该商家专属运费', 'info')
            }}>{item.enabled ? '停用' : '启用'}</Button></td>
          </tr>)}</tbody>
        </table></div>}
    </Panel>}
  </>
}
