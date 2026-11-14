import { useEffect, useState } from 'react'
import type { FeedbackRow, IdentityApplicationAdminList, Page, ReviewTask, Stats } from '../../types'
import type { PreviewSection } from '../data'
import { adminAPI } from '../../services/f-admin'
import {
  deliveryAdminAPI,
  type DeliverySummary,
  type NotificationChannel,
  type NotificationChannelOverview,
  type NotificationDeliveryPage
} from '../../services/delivery-admin'
import {
  Button,
  PageHeader,
  Panel,
  StatCard,
  StatusPill,
  formatWait
} from '../ui'

const REVIEW_PAGE_SIZE = 5

const reviewObjectLabels: Record<string, string> = {
  post: '帖子',
  comment: '评论',
  product: '商品',
  product_comment: '商品留言',
  report: '举报',
  takeout_merchant: '外卖商家',
  takeout_dish: '外卖菜品',
  merchant: '商家',
  rider: '骑手'
}

const reviewObjectLabel = (objectType: string) => reviewObjectLabels[objectType] || objectType || '未知类型'

// 金额与时间统一口径：金额按分转元，时间按本地时区展示。
const formatMoney = (cents: number) => `¥${(cents / 100).toFixed(2)}`
const formatDateTime = (value: string) => {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString('zh-CN')
}

const errorMessage = (reason: unknown) => {
  if (typeof reason === 'object' && reason !== null && 'status' in reason && (reason as { status?: unknown }).status === 409) {
    return '数据已被他人修改，请刷新后重试'
  }
  return reason instanceof Error ? reason.message : '数据加载失败，请稍后重试'
}

const earlier = (oldest: ReviewTask | null, item: ReviewTask) => {
  const at = new Date(item.createdAt).getTime()
  if (!Number.isFinite(at)) return oldest
  if (!oldest) return item
  const oldestAt = new Date(oldest.createdAt).getTime()
  return Number.isFinite(oldestAt) && oldestAt <= at ? oldest : item
}

const later = (latest: ReviewTask | null, item: ReviewTask) => {
  const at = new Date(item.createdAt).getTime()
  if (!Number.isFinite(at)) return latest
  if (!latest) return item
  const latestAt = new Date(latest.createdAt).getTime()
  return Number.isFinite(latestAt) && latestAt >= at ? latest : item
}

// 渠道配置状态只映射 overview 里真实存在的字段，不虚构送达率。
const channelConfigText = (channel: NotificationChannel, overview: NotificationChannelOverview) => {
  if (channel.channel === 'wechat_official') {
    const official = overview.officialAccount
    return `服务号${official.enabled ? '已启用' : '未启用'} · 模板${official.templateConfigured ? '已配置' : '未配置'} · 关注 ${official.following} · 需重新关注 ${official.notFollowing} · 状态待确认 ${official.unknown}`
  }
  if (channel.channel === 'wecom') {
    return `商家群${overview.wecom.merchantConfigured ? '已配置' : '未配置'} · 骑手群${overview.wecom.riderConfigured ? '已配置' : '未配置'}`
  }
  return channel.enabled ? '已启用' : '未配置'
}

interface DashboardData {
  stats: Stats
  submittedReviews: Page<ReviewTask>
  reviewingReviews: Page<ReviewTask>
  // 审核列表按提交时间倒序，最早一条在末页，这里保存补查末页后得到的最早任务。
  oldestPendingReview: ReviewTask | null
  identities: IdentityApplicationAdminList
  feedbacks: Page<FeedbackRow>
  deadletterDeliveries: NotificationDeliveryPage
  blockedDeliveries: NotificationDeliveryPage
  summary: DeliverySummary
  channelOverview: NotificationChannelOverview
}

type TaskTone = 'danger' | 'warning' | 'info' | 'neutral'

export function DashboardView({ navigate }: { navigate: (section: PreviewSection) => void }) {
  const [data, setData] = useState<DashboardData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [reloadToken, setReloadToken] = useState(0)

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      setLoading(true)
      setError('')
      try {
        const [
          stats,
          submittedReviews,
          reviewingReviews,
          identities,
          feedbacks,
          deadletterDeliveries,
          blockedDeliveries,
          summary,
          channelOverview
        ] = await Promise.all([
          adminAPI.stats(),
          adminAPI.reviews({ status: 'submitted', pageSize: REVIEW_PAGE_SIZE }),
          adminAPI.reviews({ status: 'reviewing', pageSize: REVIEW_PAGE_SIZE }),
          adminAPI.listIdentityApplications({ status: 'pending', pageSize: REVIEW_PAGE_SIZE }),
          adminAPI.feedbacks({ status: 'open', pageSize: REVIEW_PAGE_SIZE }),
          deliveryAdminAPI.notificationDeliveries({ status: 'deadletter', pageSize: 1 }),
          deliveryAdminAPI.notificationDeliveries({ status: 'blocked', pageSize: 1 }),
          deliveryAdminAPI.summary(),
          deliveryAdminAPI.notificationChannels()
        ])
        // 首屏只取最新 5 条；最早等待时长需要按 total 补查最后一页，避免展示样本内的假“最早”。
        const lastPages = await Promise.all([
          submittedReviews.total > submittedReviews.items.length
            ? adminAPI.reviews({
              status: 'submitted',
              page: Math.ceil(submittedReviews.total / REVIEW_PAGE_SIZE),
              pageSize: REVIEW_PAGE_SIZE
            })
            : Promise.resolve(null),
          reviewingReviews.total > reviewingReviews.items.length
            ? adminAPI.reviews({
              status: 'reviewing',
              page: Math.ceil(reviewingReviews.total / REVIEW_PAGE_SIZE),
              pageSize: REVIEW_PAGE_SIZE
            })
            : Promise.resolve(null)
        ])
        const pendingItems = [
          ...submittedReviews.items,
          ...reviewingReviews.items,
          ...(lastPages[0]?.items ?? []),
          ...(lastPages[1]?.items ?? [])
        ]
        const oldestPendingReview = pendingItems.reduce<ReviewTask | null>((oldest, item) => earlier(oldest, item), null)
        if (!cancelled) {
          setData({
            stats,
            submittedReviews,
            reviewingReviews,
            oldestPendingReview,
            identities,
            feedbacks,
            deadletterDeliveries,
            blockedDeliveries,
            summary,
            channelOverview
          })
        }
      } catch (reason) {
        if (!cancelled) setError(errorMessage(reason))
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => { cancelled = true }
  }, [reloadToken])

  const summary = data?.summary ?? null
  const channelOverview = data?.channelOverview ?? null
  const pendingReviewTotal = data ? data.submittedReviews.total + data.reviewingReviews.total : null
  const notificationFailures = data ? data.deadletterDeliveries.total + data.blockedDeliveries.total : null
  const identityTotal = data ? data.identities.total : null
  const feedbackTotal = data ? data.feedbacks.total : null
  const latestReview = data
    ? [...data.submittedReviews.items, ...data.reviewingReviews.items].reduce<ReviewTask | null>((latest, item) => later(latest, item), null)
    : null
  const latestIdentity = data?.identities.items[0] ?? null
  const latestFeedback = data?.feedbacks.items[0] ?? null

  const tasks: Array<{ id: string; title: string; detail: string; tone: TaskTone; action: string; section: PreviewSection }> = []
  if (data) {
    if (data.deadletterDeliveries.total + data.blockedDeliveries.total > 0) {
      tasks.push({
        id: 'notification',
        title: `${data.deadletterDeliveries.total + data.blockedDeliveries.total} 条通知发送异常`,
        detail: `多次失败 ${data.deadletterDeliveries.total} 条、未发送 ${data.blockedDeliveries.total} 条，可在通知工作台重试或检查订阅配置。`,
        tone: 'danger',
        action: '处理通知',
        section: 'notifications'
      })
    }
    if (pendingReviewTotal !== null && pendingReviewTotal > 0) {
      tasks.push({
        id: 'review',
        title: `${pendingReviewTotal} 条内容等待审核`,
        detail: latestReview
          ? `最近提交：${reviewObjectLabel(latestReview.objectType)} #${latestReview.objectId}，已等待 ${formatWait(latestReview.createdAt)}。`
          : '审核队列有新的任务待处理。',
        tone: 'warning',
        action: '进入审核',
        section: 'moderation'
      })
    }
    if (data.identities.total > 0) {
      tasks.push({
        id: 'identity',
        title: `${data.identities.total} 份商家/骑手资料待认证`,
        detail: latestIdentity
          ? `最近提交：${latestIdentity.role === 'merchant' ? '商家' : '骑手'}认证资料，已等待 ${formatWait(latestIdentity.submittedAt)}。`
          : '材料已提交，可以开始核验。',
        tone: 'info',
        action: '查看资料',
        section: 'identities'
      })
    }
    if (data.feedbacks.total > 0) {
      tasks.push({
        id: 'feedback',
        title: `${data.feedbacks.total} 条用户反馈等待首次响应`,
        detail: latestFeedback
          ? `最近提交：${latestFeedback.category || '未分类'}反馈，已等待 ${formatWait(latestFeedback.createdAt)}。`
          : '按提交时间排序，优先处理较早的反馈。',
        tone: 'neutral',
        action: '查看反馈',
        section: 'feedbacks'
      })
    }
  }

  // 配送流程按全量订单的当前状态分布展示，避免把“今日”与“历史积压”混在同一张图里。
  const deliveryFlow = summary
    ? [
      { key: 'pendingFeePayment', label: '待支付', count: summary.pendingFeePayment },
      { key: 'readyForClaim', label: '待接单', count: summary.readyForClaim },
      { key: 'inProgress', label: '配送中', count: summary.inProgress },
      { key: 'delivered', label: '已送达', count: summary.delivered },
      { key: 'cancelled', label: '已取消', count: summary.cancelled }
    ]
    : []
  const deliveryFlowMax = Math.max(1, ...deliveryFlow.map(item => item.count))

  return <div className="preview-dashboard">
    <PageHeader
      eyebrow="今日服务节奏"
      title="经营总览"
      description="这里不重复罗列所有数字，只展示会影响履约、审核和用户体验的关键变化。"
      actions={<>
        <Button variant="ghost" onClick={() => navigate('delivery')}>查看全部订单</Button>
        <Button variant="primary" onClick={() => navigate('notifications')}>处理今日待办</Button>
      </>}
    />

    {error && <div className="preview-form-error">
      数据加载失败：{error}
      <Button size="sm" variant="ghost" onClick={() => setReloadToken(token => token + 1)}>重新加载</Button>
    </div>}

    <section className="preview-stat-grid">
      <StatCard
        label="今日进入订单"
        value={summary ? summary.todayOrders : '—'}
        meta="当日新建配送单"
        trend={summary ? '实时' : '—'}
        tone="accent"
      />
      <StatCard
        label="今日平台收入"
        value={summary ? formatMoney(summary.todayPlatformFeeCents) : '—'}
        meta="已完成配送单的平台服务费"
        trend={summary ? '结算口径' : '—'}
        tone="mint"
      />
      <StatCard
        label="待审核任务"
        value={data ? data.stats.totals.pendingReview : '—'}
        meta="全部审核队列合计"
        trend={data ? (data.stats.totals.pendingReview > 0 ? '需关注' : '暂无积压') : '—'}
        tone="amber"
      />
      <StatCard
        label="通知异常"
        value={notificationFailures !== null ? notificationFailures : '—'}
        meta="可在通知工作台重试"
        trend={notificationFailures === null ? '—' : notificationFailures > 0 ? '立即处理' : '暂无异常'}
        tone="rose"
      />
    </section>

    <div className="preview-dashboard-grid">
      <Panel
        eyebrow="ACTION QUEUE"
        title="今日待办"
        description="按对用户体验的影响排序，处理后会自动从队列移除。"
        className="preview-dashboard-tasks"
      >
        <div className="preview-task-list">
          {tasks.map((task, index) => <article key={task.id} className={`preview-task preview-task--${task.tone}`}>
            <div className="preview-task__index">{String(index + 1).padStart(2, '0')}</div>
            <div className="preview-task__copy">
              <strong>{task.title}</strong>
              <p>{task.detail}</p>
            </div>
            <Button variant="ghost" size="sm" onClick={() => navigate(task.section)}>{task.action}<span>→</span></Button>
          </article>)}
          {!data && <p className="preview-health-list__empty">
            {loading ? '正在读取今日待办…' : error ? '待办数据加载失败，请刷新后重试。' : '暂无待办数据。'}
          </p>}
          {data && tasks.length === 0 && <div className="preview-all-clear">
            <span>✓</span>
            <div><strong>今天的待办已处理完</strong><p>可以继续查看业务数据，或提前配置明天的工作。</p></div>
          </div>}
        </div>
      </Panel>

      <Panel
        eyebrow="CHANNEL HEALTH"
        title="通知通道"
        description={channelOverview
          ? `近 24 小时投递状态 · 发送开关${channelOverview.sendingEnabled ? '已开启' : '未开启'} · 接收范围：${channelOverview.audience === 'all' ? '全部授权用户' : '测试账号'}`
          : '从真实通知服务读取各渠道近 24 小时的投递状态。'}
        actions={<Button size="sm" variant="ghost" onClick={() => navigate('notifications')}>管理</Button>}
      >
        <div className="preview-health-list">
          {channelOverview?.channels.map(channel => {
            const issues = channel.blocked + channel.deadletter
            return <div key={channel.channel}>
              <div className="preview-health-list__head">
                <span>
                  <strong>{channel.label || channel.channel}</strong>
                  <small>{channelConfigText(channel, channelOverview)}</small>
                  <small>近 24 小时：已受理 {channel.accepted} · 排队 {channel.pending} · 等待重试 {channel.retryWait} · 未发送 {channel.blocked} · 多次失败 {channel.deadletter}</small>
                </span>
                <b>{issues > 0 ? `${issues} 条异常` : '运行正常'}</b>
              </div>
            </div>
          })}
          {!channelOverview && <p className="preview-health-list__empty">
            {loading ? '正在读取通知渠道…' : '通知渠道数据加载失败，请刷新后重试。'}
          </p>}
          {channelOverview && channelOverview.channels.length === 0 && <p className="preview-health-list__empty">暂无可用的通知渠道。</p>}
        </div>
      </Panel>
    </div>

    <div className="preview-dashboard-grid preview-dashboard-grid--lower">
      <Panel
        eyebrow="DELIVERY FLOW"
        title="配送流程概览"
        description={summary
          ? `全部配送单按当前状态分布；今日新增 ${summary.todayOrders} 单，已完成 ${summary.todayDelivered} 单，配送费 ${formatMoney(summary.todayDeliveryFeeCents)}。`
          : '全部配送单按当前状态的分布。'}
      >
        {deliveryFlow.length > 0
          ? <div className="preview-chart" aria-label="全部配送单按状态分布">
            <div className="preview-chart__axis"><span>{deliveryFlowMax}</span><span>{Math.round(deliveryFlowMax / 2)}</span><span>0</span></div>
            <div className="preview-chart__plot">
              <div className="preview-chart__bars" role="img" aria-label={`全部配送单按状态分布，共 ${summary?.orders ?? 0} 单`}>
                {deliveryFlow.map(item => <div key={item.key} className="preview-chart__bar-slot">
                  <div className="preview-chart__bar" style={{ height: `${Math.max(2, Math.round(item.count / deliveryFlowMax * 100))}%` }}>
                    {item.count > 0 && <span>{item.count}</span>}
                  </div>
                </div>)}
              </div>
              <div className="preview-chart__labels">{deliveryFlow.map(item => <span key={item.key}>{item.label}</span>)}</div>
            </div>
          </div>
          : <p className="preview-health-list__empty">
            {loading ? '正在读取配送概览…' : '配送概览加载失败，请刷新后重试。'}
          </p>}
        <div className="preview-chart-legend">
          <span><i /> 全部配送单</span>
          <strong>{summary ? `${summary.orders} 单` : '—'}</strong>
        </div>
      </Panel>

      <Panel
        eyebrow="TODAY AT A GLANCE"
        title="服务现场"
        description="关键角色和任务的当前状态。"
      >
        <div className="preview-scene-list">
          <div>
            <span className="preview-scene-list__icon">骑</span>
            <span>
              <strong>{summary ? `${summary.inProgress} 单正在配送` : '配送状态加载中'}</strong>
              <small>{summary ? `${summary.readyForClaim} 单待骑手接单，${summary.pendingFeePayment} 单待支付` : '—'}</small>
            </span>
            <StatusPill tone={summary === null ? 'neutral' : summary.inProgress > 0 ? 'success' : 'neutral'}>
              {summary === null ? '加载中' : summary.inProgress > 0 ? '履约中' : '空闲'}
            </StatusPill>
          </div>
          <div>
            <span className="preview-scene-list__icon">审</span>
            <span>
              <strong>{pendingReviewTotal !== null ? `${pendingReviewTotal} 条审核任务` : '审核状态加载中'}</strong>
              <small>{data?.oldestPendingReview
                ? `最早一条已等待 ${formatWait(data.oldestPendingReview.createdAt)}`
                : pendingReviewTotal === 0 ? '当前没有待审核任务' : '—'}</small>
            </span>
            <StatusPill tone={pendingReviewTotal === null ? 'neutral' : pendingReviewTotal > 0 ? 'warning' : 'success'}>
              {pendingReviewTotal === null ? '加载中' : pendingReviewTotal > 0 ? '待处理' : '已清空'}
            </StatusPill>
          </div>
          <div>
            <span className="preview-scene-list__icon">通</span>
            <span>
              <strong>{notificationFailures !== null ? `${notificationFailures} 条通知异常` : '通知状态加载中'}</strong>
              <small>{data ? `多次失败 ${data.deadletterDeliveries.total} 条 · 未发送 ${data.blockedDeliveries.total} 条` : '—'}</small>
            </span>
            <StatusPill tone={notificationFailures === null ? 'neutral' : notificationFailures > 0 ? 'danger' : 'success'}>
              {notificationFailures === null ? '加载中' : notificationFailures > 0 ? '待处理' : '正常'}
            </StatusPill>
          </div>
        </div>
        <div className="preview-recent-note">
          <span>最近动态</span>
          <strong>{latestReview ? formatDateTime(latestReview.createdAt) : '暂无待办'}</strong>
          <p>{latestReview
            ? `${reviewObjectLabel(latestReview.objectType)} #${latestReview.objectId} 已提交审核，等待处理。`
            : '当前没有新的审核任务。'}</p>
        </div>
      </Panel>
    </div>
  </div>
}
