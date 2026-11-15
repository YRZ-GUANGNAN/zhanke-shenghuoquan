import { useCallback, useEffect, useMemo, useState } from 'react'
import { useDebouncedValue } from '../delivery-hooks'
import type { Tone } from '../data'
import {
  deliveryAdminAPI,
  DeliveryAPIError,
  type NotificationChannel,
  type NotificationChannelOverview,
  type NotificationDelivery,
  type NotificationDeliveryPage
} from '../../services/delivery-admin'
import {
  Button,
  EmptyState,
  Field,
  Modal,
  PageHeader,
  Pagination,
  Panel,
  ProgressBar,
  SearchBox,
  Segmented,
  StatCard,
  StatusPill,
  usePreviewToast
} from '../ui'

type NotificationTab = 'overview' | 'deliveries'
type WecomGroup = 'merchant_ops' | 'rider_ops'

// 发送记录每页条数，翻页时把 page/pageSize 原样交给真实接口。
const PAGE_SIZE = 10

const channelLabels: Record<NotificationChannel['channel'], string> = {
  in_app: '小程序通知',
  wechat_subscribe: '微信订阅',
  wechat_official: '微信服务号',
  wecom: '值班群'
}

const deliveryStatusLabels: Record<string, string> = {
  pending: '排队中',
  accepted: '已发出',
  retry_wait: '等待重试',
  blocked: '未发送',
  deadletter: '多次失败'
}

const deliveryStatusTones: Record<string, Tone> = {
  pending: 'info',
  accepted: 'success',
  retry_wait: 'warning',
  blocked: 'warning',
  deadletter: 'danger'
}

const wecomGroupLabels: Record<WecomGroup, string> = {
  merchant_ops: '商家值班群',
  rider_ops: '骑手值班群'
}

const channelLabel = (channel: NotificationDelivery['channel']) => channelLabels[channel] || channel
const statusLabel = (status: string) => deliveryStatusLabels[status] || status
const statusTone = (status: string): Tone => deliveryStatusTones[status] || 'neutral'

const formatDateTime = (value?: string | null) => {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString('zh-CN')
}

const errorText = (reason: unknown) => {
  if (reason instanceof DeliveryAPIError) {
    return reason.status === 409 ? '数据已被他人修改，请刷新后重试' : reason.message
  }
  return reason instanceof Error ? reason.message : String(reason)
}

// 服务号与值班群在渠道计数之外还有真实的配置/绑定状态，逐项映射，不展示后端没有的指标。
function channelConfigText(channel: NotificationChannel, overview: NotificationChannelOverview) {
  if (channel.channel === 'wechat_official') {
    return `服务号${overview.officialAccount.enabled ? '已启用' : '未启用'} · 模板${overview.officialAccount.templateConfigured ? '已配置' : '未配置'}`
  }
  if (channel.channel === 'wecom') {
    return `商家群${overview.wecom.merchantConfigured ? '已配置' : '未配置'} · 骑手群${overview.wecom.riderConfigured ? '已配置' : '未配置'}`
  }
  return channel.enabled ? '已配置' : '未配置'
}

export function NotificationsView() {
  const { notify } = usePreviewToast()
  const [tab, setTab] = useState<NotificationTab>('overview')
  const [search, setSearch] = useState('')
  const [overview, setOverview] = useState<NotificationChannelOverview | null>(null)
  const [overviewLoading, setOverviewLoading] = useState(true)
  const [overviewError, setOverviewError] = useState('')
  const [deliveries, setDeliveries] = useState<NotificationDeliveryPage | null>(null)
  const [deliveriesLoading, setDeliveriesLoading] = useState(true)
  const [deliveriesError, setDeliveriesError] = useState('')
  const [deliveryStatus, setDeliveryStatus] = useState('全部')
  const [page, setPage] = useState(1)
  const [retryCandidate, setRetryCandidate] = useState<NotificationDelivery | null>(null)
  const [retryError, setRetryError] = useState('')
  const [retrying, setRetrying] = useState(false)
  const [testCandidate, setTestCandidate] = useState<WecomGroup | null>(null)
  const [testError, setTestError] = useState('')
  const [testing, setTesting] = useState<WecomGroup | ''>('')
  const [wecomMerchantInput, setWecomMerchantInput] = useState('')
  const [wecomRiderInput, setWecomRiderInput] = useState('')
  const [wecomSaving, setWecomSaving] = useState(false)
  const [wecomError, setWecomError] = useState('')
  const [wecomNotice, setWecomNotice] = useState('')

  const loadOverview = useCallback(async () => {
    setOverviewLoading(true)
    setOverviewError('')
    try {
      setOverview(await deliveryAdminAPI.notificationChannels())
    } catch (reason) {
      setOverviewError(errorText(reason))
    } finally {
      setOverviewLoading(false)
    }
  }, [])

  const debouncedSearch = useDebouncedValue(search, 350)
  const loadDeliveries = useCallback(async () => {
    setDeliveriesLoading(true)
    setDeliveriesError('')
    try {
      const result = await deliveryAdminAPI.notificationDeliveries({
        status: deliveryStatus === '全部' ? undefined : deliveryStatus,
        keyword: debouncedSearch.trim() || undefined,
        page,
        pageSize: PAGE_SIZE
      })
      // 重投后列表可能变短，当前页超出总页数时回退到最后一页。
      if (result.items.length === 0 && result.total > 0 && page > 1) {
        setPage(Math.max(1, Math.ceil(result.total / result.pageSize)))
      } else {
        setDeliveries(result)
      }
    } catch (reason) {
      setDeliveriesError(errorText(reason))
    } finally {
      setDeliveriesLoading(false)
    }
  }, [deliveryStatus, debouncedSearch, page])

  useEffect(() => {
    void loadOverview()
  }, [loadOverview])

  useEffect(() => {
    if (tab === 'deliveries') void loadDeliveries()
  }, [tab, loadDeliveries])

  const totals = useMemo(() => {
    const sums = { pending: 0, accepted: 0, retryWait: 0, blocked: 0, deadletter: 0 }
    for (const channel of overview?.channels ?? []) {
      sums.pending += channel.pending
      sums.accepted += channel.accepted
      sums.retryWait += channel.retryWait
      sums.blocked += channel.blocked
      sums.deadletter += channel.deadletter
    }
    return sums
  }, [overview])

  const totalAll = totals.pending + totals.accepted + totals.retryWait + totals.blocked + totals.deadletter
  const settled = totals.accepted + totals.blocked + totals.deadletter
  const deliveryRate = settled > 0 ? Math.round(totals.accepted / settled * 100) : null
  const attention = totals.retryWait + totals.blocked + totals.deadletter
  const merchantConfigured = overview?.wecom.merchantConfigured === true
  const riderConfigured = overview?.wecom.riderConfigured === true

  // 搜索走服务端（跨页），这里直接使用接口结果。
  const visibleDeliveries = deliveries?.items ?? []
  const deliveryPages = deliveries ? Math.max(1, Math.ceil(deliveries.total / deliveries.pageSize)) : 1

  const openDeliveries = (status: string) => {
    setDeliveryStatus(status)
    setPage(1)
    setSearch('')
    setTab('deliveries')
  }

  const suggestedActions: Array<{ key: string; title: string; description: string; status: string }> = []
  if (overview) {
    if (totals.deadletter > 0) {
      suggestedActions.push({
        key: 'deadletter',
        title: `重投 ${totals.deadletter} 条多次失败消息`,
        description: '进入发送记录，对多次失败消息执行重投。',
        status: 'deadletter'
      })
    }
    if (totals.blocked > 0) {
      suggestedActions.push({
        key: 'blocked',
        title: `检查 ${totals.blocked} 条未发送消息`,
        description: '多为用户未授权或渠道配置缺失，修复后再重投。',
        status: 'blocked'
      })
    }
    if (totals.retryWait > 0) {
      suggestedActions.push({
        key: 'retry_wait',
        title: `等待 ${totals.retryWait} 条消息自动重试`,
        description: '系统会在下次调度时自动重试。',
        status: 'retry_wait'
      })
    }
  }
  suggestedActions.push({
    key: 'all',
    title: '查看全部发送记录',
    description: '核对每次投递的通知方式、接收用户与失败原因。',
    status: '全部'
  })

  const confirmRetry = async () => {
    if (!retryCandidate) return
    setRetrying(true)
    setRetryError('')
    try {
      await deliveryAdminAPI.retryNotificationDelivery(retryCandidate.deliveryId)
      notify('重投已提交，消息已重新进入发送队列')
      setRetryCandidate(null)
      await Promise.all([loadDeliveries(), loadOverview()])
    } catch (reason) {
      setRetryError(errorText(reason))
    } finally {
      setRetrying(false)
    }
  }

  const confirmTest = async () => {
    if (!testCandidate) return
    setTesting(testCandidate)
    setTestError('')
    try {
      const result = await deliveryAdminAPI.testWecomGroup(testCandidate)
      if (!result.accepted) throw new Error('测试消息未被受理，请检查值班群机器人配置')
      notify('测试消息已提交')
      setTestCandidate(null)
      await loadOverview()
    } catch (reason) {
      setTestError(errorText(reason))
    } finally {
      setTesting('')
    }
  }

  return <div>
    <PageHeader
      eyebrow="服务基础设施"
      title="消息通知"
      description="查看各通知渠道的真实配置与最近 24 小时投递结果，异常消息可以直接重投。"
      actions={<Button
        variant="ghost"
        disabled={overviewLoading || (tab === 'deliveries' && deliveriesLoading)}
        onClick={() => {
          void loadOverview()
          if (tab === 'deliveries') void loadDeliveries()
        }}
      >刷新状态</Button>}
    />

    <Segmented
      className="preview-page-tabs"
      value={tab}
      onChange={value => { setTab(value); setSearch('') }}
      options={[
        { value: 'overview', label: '运行概览' },
        { value: 'deliveries', label: '发送记录', badge: deliveries && deliveryStatus === '全部' ? deliveries.total : undefined }
      ]}
    />

    {tab === 'overview' && <div className="preview-notification-overview preview-detail-stack">
      {overviewError && overview && <div className="preview-form-error">{overviewError}</div>}
      {overview && !overview.sendingEnabled && <div className="preview-warning-box">
        <strong>微信消息发送当前未开启</strong>
        <p>小程序内通知不受影响；微信订阅、服务号与值班群提醒要等发送功能开启后才会发出。</p>
      </div>}

      <section className="preview-stat-grid">
        <StatCard label="近 24 小时送达" value={overview ? totals.accepted : '—'} meta="所有通知通道" tone="mint" />
        <StatCard label="正在发送" value={overview ? totals.pending : '—'} meta="排队中的消息" tone="accent" />
        <StatCard
          label="发送异常"
          value={overview ? attention : '—'}
          meta={overview ? `等待重试 ${totals.retryWait} · 未发送 ${totals.blocked} · 多次失败 ${totals.deadletter}` : '等待重试、未发送与多次失败'}
          tone="rose"
        />
        <StatCard
          label="近 24 小时送达率"
          value={overview ? (deliveryRate === null ? '—' : `${deliveryRate}%`) : '—'}
          meta={overview && settled > 0 ? `${settled} 条已出结果，其中 ${totals.accepted} 条送达` : '暂无已出结果的记录'}
          tone="amber"
        />
      </section>

      <div className="preview-notification-grid">
        <Panel
          eyebrow="CHANNEL STATUS"
          title="渠道健康"
          description={overview
            ? `最近刷新 ${formatDateTime(overview.checkedAt) || '—'} · 微信消息发送${overview.sendingEnabled ? '已开启' : '未开启'} · 接收范围：${overview.audience === 'all' ? '全部已授权用户' : '测试账号'}`
            : '从真实通知服务读取渠道配置和近 24 小时投递计数。'}
        >
          {!overview
            ? overviewLoading
              ? <EmptyState title="正在读取渠道状态" description="渠道配置与近 24 小时投递计数正在加载。" />
              : <EmptyState
                title="暂时没有渠道数据"
                description={overviewError || '请确认管理员会话有效后重试。'}
                action={<Button variant="primary" onClick={() => void loadOverview()}>重新加载</Button>}
              />
            : <div className="preview-channel-health-cards">
              {overview.channels.map(channel => {
                const name = channel.label || channelLabels[channel.channel]
                const channelSettled = channel.accepted + channel.blocked + channel.deadletter
                const channelRate = channelSettled > 0 ? Math.round(channel.accepted / channelSettled * 100) : null
                const channelIssues = channel.blocked + channel.deadletter
                return <article key={channel.channel}>
                  <div className="preview-channel-health-cards__head">
                    <span className={`preview-channel-health-cards__icon ${!channel.enabled || channelIssues > 0 ? 'is-warning' : ''}`}>◉</span>
                    <div>
                      <strong>{name}</strong>
                      <small>{channelConfigText(channel, overview)}</small>
                      <small>近 24 小时：发出 {channel.accepted} · 排队 {channel.pending} · 等待重试 {channel.retryWait} · 未发送 {channel.blocked} · 多次失败 {channel.deadletter}</small>
                      {channel.channel === 'wechat_official' && <small>绑定用户：已关注 {overview.officialAccount.following} · 需重新关注 {overview.officialAccount.notFollowing} · 状态待确认 {overview.officialAccount.unknown}</small>}
                    </div>
                    <span>{channelRate === null ? '—' : `${channelRate}%`}</span>
                  </div>
                  <ProgressBar value={channelRate ?? 0} label={`${name}送达占比`} />
                  <div className="preview-channel-health-cards__foot">
                    <StatusPill tone={!channel.enabled ? 'neutral' : channelIssues > 0 ? 'warning' : 'success'}>
                      {!channel.enabled ? '未配置' : channelIssues > 0 ? '需要关注' : '运行正常'}
                    </StatusPill>
                    <p>{channelSettled} 条已出结果中 {channel.accepted} 条送达</p>
                  </div>
                </article>
              })}
            </div>}
        </Panel>

        <Panel eyebrow="NEXT ACTION" title="建议现在处理" description="只列出会影响送达结果的事项。">
          <div className="preview-action-stack">
            {suggestedActions.map((action, index) => <button key={action.key} type="button" onClick={() => openDeliveries(action.status)}>
              <span className="preview-action-stack__index">{String(index + 1).padStart(2, '0')}</span>
              <div><strong>{action.title}</strong><p>{action.description}</p></div><span>→</span>
            </button>)}
          </div>
          <div className="preview-policy-note">
            <span>发送边界</span>
            <p>{overview
              ? `${overview.sendingEnabled ? '微信消息发送已开启' : '微信消息发送未开启'}，当前接收范围：${overview.audience === 'all' ? '全部已授权用户' : '测试账号'}。重投与值班群测试都会调用真实通知服务。`
              : '重投与值班群测试都会调用真实通知服务；渠道状态加载后可查看发送开关与接收范围。'}</p>
          </div>
        </Panel>
      </div>

      <Panel
        eyebrow="WECOM TEST"
        title="值班群发送测试"
        description="通过企业微信机器人发送一条真实测试消息，不关联订单，也不会附带用户手机号或地址。"
      >
        <div className="preview-toolbar">
          <span className="preview-toolbar__hint">
            商家值班群{overview ? (merchantConfigured ? `已配置${overview.wecom.merchantHint ? `（${overview.wecom.merchantHint}）` : ''}` : '未配置') : '状态未知'}
            {' · '}
            骑手值班群{overview ? (riderConfigured ? `已配置${overview.wecom.riderHint ? `（${overview.wecom.riderHint}）` : ''}` : '未配置') : '状态未知'}
          </span>
          <div className="preview-row-actions">
            <Button
              size="sm"
              variant="ghost"
              disabled={!merchantConfigured || testing !== ''}
              onClick={() => { setTestError(''); setTestCandidate('merchant_ops') }}
            >测试商家值班群</Button>
            <Button
              size="sm"
              variant="ghost"
              disabled={!riderConfigured || testing !== ''}
              onClick={() => { setTestError(''); setTestCandidate('rider_ops') }}
            >测试骑手值班群</Button>
          </div>
        </div>
        <div className="preview-wecom-form">
          <Field
            label="商家值班群 Webhook"
            value={wecomMerchantInput}
            onChange={event => { setWecomMerchantInput(event.target.value); setWecomError('') }}
            placeholder={merchantConfigured ? '已配置，留空保持不变；粘贴新地址可替换' : '粘贴企业微信商家值班群机器人 Webhook（留空沿用服务器配置）'}
          />
          <Field
            label="骑手值班群 Webhook"
            value={wecomRiderInput}
            onChange={event => { setWecomRiderInput(event.target.value); setWecomError('') }}
            placeholder={riderConfigured ? '已配置，留空保持不变；粘贴新地址可替换' : '粘贴企业微信骑手值班群机器人 Webhook（留空沿用服务器配置）'}
          />
          <div className="preview-row-actions">
            <Button
              size="sm"
              variant="primary"
              disabled={wecomSaving || !overview || (!wecomMerchantInput.trim() && !wecomRiderInput.trim())}
              onClick={async () => {
                if (!overview) return
                setWecomSaving(true)
                setWecomError('')
                setWecomNotice('')
                try {
                  const payload: { merchantWebhook?: string; riderWebhook?: string; expectedRowVersion: number } = {
                    expectedRowVersion: overview.wecom.rowVersion ?? 1
                  }
                  if (wecomMerchantInput.trim()) payload.merchantWebhook = wecomMerchantInput.trim()
                  if (wecomRiderInput.trim()) payload.riderWebhook = wecomRiderInput.trim()
                  await deliveryAdminAPI.updateWecomSettings(payload)
                  setWecomMerchantInput('')
                  setWecomRiderInput('')
                  setWecomNotice('渠道配置已保存并立即生效')
                  await loadOverview()
                } catch (reason) {
                  setWecomError(errorText(reason))
                } finally {
                  setWecomSaving(false)
                }
              }}
            >{wecomSaving ? '保存中…' : '保存渠道配置'}</Button>
            <Button
              size="sm"
              disabled={wecomSaving}
              onClick={() => { setWecomMerchantInput(''); setWecomRiderInput(''); setWecomError('') }}
            >清空输入</Button>
          </div>
          {wecomError && <div className="preview-form-error" role="alert">{wecomError}</div>}
          {wecomNotice && <div className="preview-policy-note" role="status">{wecomNotice}</div>}
          <p className="preview-policy-note">地址包含群机器人密钥，保存后只显示尾号。只更新填写的群；留空的群沿用服务器配置，不会被停用。保存后立即生效，无需重启服务。</p>
        </div>
      </Panel>

      {(overview?.workers?.length ?? 0) > 0 && <Panel
        eyebrow="WORKERS"
        title="后台投递任务"
        description="这些任务持续运行；任务停止后提醒会中断，需要重启服务恢复。"
      >
        {overview?.workers?.map(worker => <div className="preview-toolbar" key={worker.name}>
          <span className="preview-toolbar__hint">
            <strong>{worker.name}</strong>
            {' · '}
            {worker.running
              ? `启动于 ${formatDateTime(worker.startedAt) || '—'}`
              : [worker.lastError, worker.exitReason].filter(Boolean).join('；') || '已停止'}
          </span>
          <StatusPill tone={worker.running ? 'success' : 'danger'}>{worker.running ? '运行中' : '已停止'}</StatusPill>
        </div>)}
      </Panel>}
    </div>}

    {tab === 'deliveries' && <Panel
      eyebrow="DELIVERY LOG"
      title="发送记录"
      description="每次通知都保留通知方式、接收用户、投递结果与失败原因。"
      actions={<SearchBox value={search} onChange={value => { setSearch(value); setPage(1) }} placeholder="搜索用途、渠道、收件人或失败原因" />}
    >
      <div className="preview-toolbar">
        <Segmented
          value={deliveryStatus}
          onChange={value => { setDeliveryStatus(value); setPage(1); setSearch('') }}
          options={[
            { value: '全部', label: '全部', badge: overview ? totalAll : undefined },
            { value: 'pending', label: '排队中', badge: overview ? totals.pending : undefined },
            { value: 'accepted', label: '已发出', badge: overview ? totals.accepted : undefined },
            { value: 'retry_wait', label: '等待重试', badge: overview ? totals.retryWait : undefined },
            { value: 'blocked', label: '未发送', badge: overview ? totals.blocked : undefined },
            { value: 'deadletter', label: '多次失败', badge: overview ? totals.deadletter : undefined }
          ]}
        />
        <span className="preview-toolbar__hint">
          {deliveriesLoading
            ? '正在读取发送记录…'
            : deliveries
              ? `状态徽标为近 24 小时计数 · 共 ${deliveries.total} 条记录 · 第 ${deliveries.page} / ${deliveryPages} 页`
              : '尚未读取发送记录'}
        </span>
      </div>
      {deliveriesError && deliveries && <div className="preview-form-error">{deliveriesError}</div>}
      {!deliveries
        ? deliveriesLoading
          ? <EmptyState title="正在读取发送记录" description="最近的通知投递结果加载完成后会自动显示。" />
          : <EmptyState
            title="暂时没有发送记录"
            description={deliveriesError || '有订单或私信触发提醒后，每一次实际投递都会出现在这里。'}
            action={<Button variant="primary" onClick={() => void loadDeliveries()}>重新加载</Button>}
          />
        : visibleDeliveries.length === 0
          ? <EmptyState title="没有匹配的记录" description={search.trim() ? '换个搜索词或清空搜索再试。' : '当前状态下还没有发送记录。'} />
          : <div className="preview-table-wrap">
            <table className="preview-table" aria-label="发送记录列表">
              <thead><tr><th>通知用途</th><th>渠道</th><th>接收用户</th><th>状态</th><th>尝试</th><th>时间</th><th /></tr></thead>
              <tbody>{visibleDeliveries.map(record => <tr key={record.deliveryId}>
                <td>
                  <strong>{record.purpose || '未标注用途'}</strong>
                  <span>{record.error ? `${record.error}${record.errorCode ? `（${record.errorCode}）` : ''}` : record.errorCode || `编号 ${record.deliveryId}`}</span>
                </td>
                <td>{channelLabel(record.channel)}</td>
                <td>#{record.recipientId}</td>
                <td>
                  <StatusPill tone={statusTone(record.status)}>{statusLabel(record.status)}</StatusPill>
                  {record.status === 'retry_wait' && record.nextAttemptAt && <span>下次重试 {formatDateTime(record.nextAttemptAt)}</span>}
                </td>
                <td>{record.attemptCount} 次</td>
                <td>
                  {formatDateTime(record.createdAt)}
                  {record.processedAt && <span>完成于 {formatDateTime(record.processedAt)}</span>}
                </td>
                <td>{(record.status === 'blocked' || record.status === 'deadletter') && <Button
                  size="sm"
                  variant="primary"
                  disabled={retrying}
                  onClick={() => { setRetryError(''); setRetryCandidate(record) }}
                >重投</Button>}</td>
              </tr>)}</tbody>
            </table>
          </div>}
      {deliveries && deliveries.total > 0 && <Pagination
        page={deliveries.page}
        pageSize={deliveries.pageSize}
        total={deliveries.total}
        onChange={setPage}
        label="发送记录分页"
      />}
    </Panel>}

    <Modal
      open={Boolean(retryCandidate)}
      title="重投通知"
      description={retryCandidate
        ? `${statusLabel(retryCandidate.status)} · ${channelLabel(retryCandidate.channel)} · 接收用户 #${retryCandidate.recipientId}`
        : ''}
      onClose={() => { if (!retrying) setRetryCandidate(null) }}
      width="sm"
      footer={<>
        <Button disabled={retrying} onClick={() => setRetryCandidate(null)}>取消</Button>
        <Button variant="primary" disabled={retrying} onClick={() => void confirmRetry()}>{retrying ? '正在提交…' : '确认重投'}</Button>
      </>}
    >
      <div className="preview-warning-box">
        <strong>重投会把该消息重新放回发送队列并清零重试次数</strong>
        <p>若渠道配置或用户授权仍未修复，消息可能再次失败。</p>
      </div>
      {retryError && <div className="preview-form-error">{retryError}</div>}
    </Modal>

    <Modal
      open={Boolean(testCandidate)}
      title={testCandidate ? `发送${wecomGroupLabels[testCandidate]}测试` : '发送值班群测试'}
      description="通过企业微信机器人发送一条真实测试消息。"
      onClose={() => { if (!testing) { setTestCandidate(null); setTestError('') } }}
      width="sm"
      footer={<>
        <Button disabled={Boolean(testing)} onClick={() => { setTestCandidate(null); setTestError('') }}>取消</Button>
        <Button variant="primary" disabled={Boolean(testing)} onClick={() => void confirmTest()}>{testing ? '正在提交…' : '发送测试'}</Button>
      </>}
    >
      <div className="preview-form-stack">
        <div className="preview-warning-box">
          <strong>这会向值班群发送真实群消息</strong>
          <p>测试不关联订单，也不会附带用户手机号或地址；若群内未出现消息，请检查机器人是否仍在群内。</p>
        </div>
        {testError && <div className="preview-form-error">{testError}</div>}
      </div>
    </Modal>
  </div>
}
