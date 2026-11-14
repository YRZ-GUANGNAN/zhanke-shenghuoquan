import { useMemo } from 'react'
import type { PreviewSection } from '../data'
import { channelHealthFrom, formatMoney, formatTime, isToday } from '../data'
import { usePreviewStore } from '../store'
import {
  Button,
  PageHeader,
  Panel,
  ProgressBar,
  StatCard,
  StatusPill,
  formatWait
} from '../ui'

export function DashboardView({ navigate }: { navigate: (section: PreviewSection) => void }) {
  const orders = usePreviewStore(state => state.orders)
  const reviews = usePreviewStore(state => state.reviews)
  const deliveries = usePreviewStore(state => state.deliveries)
  const identities = usePreviewStore(state => state.identities)
  const feedbacks = usePreviewStore(state => state.feedbacks)
  const media = usePreviewStore(state => state.media)

  const summary = useMemo(() => {
    const today = orders.filter(order => isToday(order.createdAt))
    return {
      todayOrders: today.length,
      revenueCents: today.filter(order => order.status === '已完成').reduce((total, order) => total + order.platformFeeCents, 0),
      pendingReviews: reviews.filter(review => review.status === '待审核' || review.status === '审核中').length,
      notificationFailures: deliveries.filter(item => item.status === '失败').length,
      activeRiders: new Set(orders.filter(order => order.status === '配送中' && order.rider !== '待分配').map(order => order.rider)).size
    }
  }, [deliveries, orders, reviews])

  // 待办条数必须来自真实队列，不能写死。
  const notificationPending = deliveries.filter(item => item.status === '失败').length
  const reviewPending = reviews.filter(item => item.status === '待审核' || item.status === '审核中').length
  const identityPending = identities.filter(item => item.status === '待认证').length
  const feedbackPending = feedbacks.filter(item => item.status === '待处理').length
  const highRiskPending = reviews.filter(item => item.risk === '高风险' && (item.status === '待审核' || item.status === '审核中')).length

  const tasks = [
    {
      id: 'notification',
      title: `${notificationPending} 条通知发送失败`,
      detail: '可在发送记录中重试，或检查订阅额度与模板状态。',
      tone: 'danger',
      action: '处理通知',
      section: 'notifications' as PreviewSection
    },
    {
      id: 'review',
      title: `${reviewPending} 条内容等待审核`,
      detail: highRiskPending ? `其中 ${highRiskPending} 条为高风险，建议优先处理。` : '当前没有高风险任务。',
      tone: 'warning',
      action: '进入审核',
      section: 'moderation' as PreviewSection
    },
    {
      id: 'identity',
      title: `${identityPending} 份商家/骑手资料待认证`,
      detail: identityPending ? '材料已提交，可以开始核验。' : '当前没有待认证资料。',
      tone: 'info',
      action: '查看资料',
      section: 'identities' as PreviewSection
    },
    {
      id: 'feedback',
      title: `${feedbackPending} 条用户反馈等待首次响应`,
      detail: feedbackPending ? '按提交时间排序，优先处理较早的反馈。' : '当前没有待处理反馈。',
      tone: 'neutral',
      action: '查看反馈',
      section: 'feedbacks' as PreviewSection
    }
  ].filter(task => {
    if (task.id === 'notification') return summary.notificationFailures > 0
    if (task.id === 'review') return summary.pendingReviews > 0
    if (task.id === 'identity') return identities.some(item => item.status === '待认证')
    return feedbacks.some(item => item.status === '待处理')
  })

  const todayOrders = orders.filter(order => isToday(order.createdAt))
  // 按真实订单时间聚合；时间轴覆盖当天实际出现的小时，避免漏掉区间外的订单。
  const orderHours = todayOrders.map(order => new Date(order.createdAt).getHours()).filter(hour => Number.isFinite(hour))
  const firstHour = orderHours.length ? Math.max(0, Math.min(...orderHours) - 1) : 8
  const lastHour = orderHours.length ? Math.min(23, Math.max(...orderHours) + 1) : 16
  const chartHours = Array.from({ length: lastHour - firstHour + 1 }, (_, index) => firstHour + index)
  const hourly = chartHours.map(hour => ({
    hour,
    count: todayOrders.filter(order => new Date(order.createdAt).getHours() === hour).length
  }))
  const chartMax = Math.max(1, ...hourly.map(item => item.count))
  // 超时判断：配送中且进入超过 60 分钟视为可能超时，不再写死"没有超时订单"。
  const overdueOrders = orders.filter(order => order.status === '配送中'
    && Number.isFinite(new Date(order.createdAt).getTime())
    && Date.now() - new Date(order.createdAt).getTime() > 60 * 60 * 1000)

  // 待审核积压：与审核页保持一致的口径，不再写死分钟数。
  const pendingReviewItems = reviews.filter(item => item.status === '待审核' || item.status === '审核中')
  const oldestPending = pendingReviewItems.reduce<{ at: number; createdAt: string } | null>((oldest, item) => {
    const at = new Date(item.createdAt).getTime()
    if (Number.isNaN(at)) return oldest
    return !oldest || at < oldest.at ? { at, createdAt: item.createdAt } : oldest
  }, null)
  // 渠道健康按真实投递记录聚合，和通知页同一份数据。
  const channelHealth = channelHealthFrom(deliveries)

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

    <section className="preview-stat-grid">
      <StatCard label="今日进入订单" value={summary.todayOrders} meta="含待接单与已完成" tone="accent" />
      <StatCard label="今日平台收入" value={formatMoney(summary.revenueCents)} meta="仅统计已完成订单" trend="结算口径" tone="mint" />
      <StatCard label="待审核任务" value={summary.pendingReviews} meta="内容与身份审核合计" trend="需关注" tone="amber" />
      <StatCard label="通知异常" value={summary.notificationFailures} meta="可在通知工作台重试" trend={summary.notificationFailures ? '立即处理' : '暂无异常'} tone="rose" />
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
          {tasks.length === 0 && <div className="preview-all-clear">
            <span>✓</span>
            <div><strong>今天的待办已处理完</strong><p>可以继续查看业务数据，或提前配置明天的工作。</p></div>
          </div>}
        </div>
      </Panel>

      <Panel
        eyebrow="CHANNEL HEALTH"
        title="通知通道"
        description="按近 24 小时的送达率排序，低于 90% 需要排查用户授权或模板配置。"
        actions={<Button size="sm" variant="ghost" onClick={() => navigate('notifications')}>管理</Button>}
      >
        <div className="preview-health-list">
          {channelHealth.length === 0 && <p className="preview-health-list__empty">近 24 小时还没有发送记录，触发一次订单或私信提醒后会出现在这里。</p>}
          {channelHealth.map(channel => <div key={channel.name}>
            <div className="preview-health-list__head">
              <span>
                <strong>{channel.name}</strong>
                <small>{channel.rate === null
                  ? `${channel.pending} 条发送中，暂无结果`
                  : `${channel.delivered + channel.failed} 条中 ${channel.delivered} 条送达${channel.failed ? `，${channel.failed} 条失败` : ''}`}</small>
              </span>
              <b>{channel.rate === null ? '—' : `${channel.rate}%`}</b>
            </div>
            <ProgressBar value={channel.rate ?? 0} label={`${channel.name} 送达率`} />
          </div>)}
        </div>
      </Panel>
    </div>

    <div className="preview-dashboard-grid preview-dashboard-grid--lower">
      <Panel
        eyebrow="DELIVERY FLOW"
        title="今日配送节奏"
        description="每小时新进入与完成订单的变化。"
      >
        <div className="preview-chart" aria-label="今日每小时进入订单数">
          <div className="preview-chart__axis"><span>{chartMax}</span><span>{Math.round(chartMax / 2)}</span><span>0</span></div>
          <div className="preview-chart__plot">
            <div className="preview-chart__bars" role="img" aria-label={`今日每小时进入订单数，共 ${todayOrders.length} 单`}>
              {hourly.map(item => <div key={item.hour} className="preview-chart__bar-slot">
                <div className="preview-chart__bar" style={{ height: `${Math.max(2, Math.round(item.count / chartMax * 100))}%` }}>
                  {item.count > 0 && <span>{item.count}</span>}
                </div>
              </div>)}
            </div>
            <div className="preview-chart__labels">{hourly.map(item => <span key={item.hour}>{String(item.hour).padStart(2, '0')}:00</span>)}</div>
          </div>
        </div>
        <div className="preview-chart-legend">
          <span><i /> 今日进入订单</span>
          <strong>{todayOrders.length} 单</strong>
        </div>
      </Panel>

      <Panel
        eyebrow="TODAY AT A GLANCE"
        title="服务现场"
        description="关键角色和任务的当前状态。"
      >
        <div className="preview-scene-list">
          <div><span className="preview-scene-list__icon">骑</span><span><strong>{summary.activeRiders} 名骑手正在配送</strong><small>{overdueOrders.length ? `${overdueOrders.length} 单配送超过 60 分钟` : '当前没有超时订单'}</small></span><StatusPill tone={summary.activeRiders ? 'success' : 'neutral'}>{summary.activeRiders ? '在线' : '无人在线'}</StatusPill></div>
          <div><span className="preview-scene-list__icon">审</span><span><strong>{summary.pendingReviews} 条审核任务</strong><small>{oldestPending ? `最早一条已等待 ${formatWait(oldestPending.createdAt)}` : '当前没有待审核任务'}</small></span><StatusPill tone={summary.pendingReviews ? 'warning' : 'success'}>{summary.pendingReviews ? '待处理' : '已清空'}</StatusPill></div>
          <div><span className="preview-scene-list__icon">材</span><span><strong>{media.filter(item => item.status === '异常').length} 个素材异常</strong><small>可进入素材中心重新处理</small></span><StatusPill tone="danger">异常</StatusPill></div>
        </div>
        <div className="preview-recent-note">
          <span>最近动态</span>
          <strong>{orders[0] ? formatTime(orders[0].createdAt) : '暂无订单'}</strong>
          <p>{orders[0] ? `${orders[0].customer} 的订单 ${orders[0].id} 已进入${orders[0].status}。` : '今天还没有新订单进入配送。'}</p>
        </div>
      </Panel>
    </div>
  </div>
}
