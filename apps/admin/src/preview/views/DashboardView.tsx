import { useMemo } from 'react'
import type { PreviewSection } from '../data'
import { formatMoney, formatTime } from '../data'
import { usePreviewStore } from '../store'
import {
  Button,
  PageHeader,
  Panel,
  ProgressBar,
  StatCard,
  StatusPill
} from '../ui'

export function DashboardView({ navigate }: { navigate: (section: PreviewSection) => void }) {
  const orders = usePreviewStore(state => state.orders)
  const reviews = usePreviewStore(state => state.reviews)
  const deliveries = usePreviewStore(state => state.deliveries)
  const identities = usePreviewStore(state => state.identities)
  const feedbacks = usePreviewStore(state => state.feedbacks)
  const media = usePreviewStore(state => state.media)

  const summary = useMemo(() => {
    const today = orders.filter(order => order.createdAt.startsWith('2026-11-14'))
    return {
      todayOrders: today.length,
      revenueCents: today.filter(order => order.status === '已完成').reduce((total, order) => total + order.platformFeeCents, 0),
      pendingReviews: reviews.filter(review => review.status === '待审核' || review.status === '审核中').length,
      notificationFailures: deliveries.filter(item => item.status === '失败').length,
      activeRiders: new Set(orders.filter(order => order.status === '配送中' && order.rider !== '待分配').map(order => order.rider)).size
    }
  }, [deliveries, orders, reviews])

  const tasks = [
    {
      id: 'notification',
      title: '2 条通知需要重新确认',
      detail: '一条订阅消息失败，另一条模板已停用。',
      tone: 'danger',
      action: '处理通知',
      section: 'notifications' as PreviewSection
    },
    {
      id: 'review',
      title: '3 条内容等待审核',
      detail: '其中 1 条为高风险举报，建议优先处理。',
      tone: 'warning',
      action: '进入审核',
      section: 'moderation' as PreviewSection
    },
    {
      id: 'identity',
      title: '2 份商家/骑手资料待认证',
      detail: '材料齐全，可以开始核验。',
      tone: 'info',
      action: '查看资料',
      section: 'identities' as PreviewSection
    },
    {
      id: 'feedback',
      title: '1 条用户反馈等待首次响应',
      detail: '用户希望通知中心支持订单类型筛选。',
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

  const todayOrders = orders.filter(order => order.createdAt.startsWith('2026-11-14'))
  const channelHealth = [
    { name: '小程序通知中心', value: 100, detail: '运行正常' },
    { name: '微信订阅消息', value: 82, detail: '1 条待重试' },
    { name: '服务号', value: 96, detail: '模板已配置' },
    { name: '值班群', value: 100, detail: '机器人在线' }
  ]

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
      <StatCard label="今日进入订单" value={summary.todayOrders} meta="含待接单与已完成" trend="+12%" tone="accent" marker="↗" />
      <StatCard label="今日平台收入" value={formatMoney(summary.revenueCents)} meta="仅统计已完成订单" trend="结算口径" tone="mint" marker="¥" />
      <StatCard label="待审核任务" value={summary.pendingReviews} meta="内容与身份审核合计" trend="需关注" tone="amber" marker="!" />
      <StatCard label="通知异常" value={summary.notificationFailures} meta="可在通知工作台重试" trend={summary.notificationFailures ? '立即处理' : '暂无异常'} tone="rose" marker="◇" />
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
        description="发送能力从用户授权、模板配置到值班群状态逐项确认。"
        actions={<Button size="sm" variant="ghost" onClick={() => navigate('notifications')}>管理</Button>}
      >
        <div className="preview-health-list">
          {channelHealth.map(channel => <div key={channel.name}>
            <div className="preview-health-list__head">
              <span><strong>{channel.name}</strong><small>{channel.detail}</small></span>
              <b>{channel.value}%</b>
            </div>
            <ProgressBar value={channel.value} label={channel.name} />
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
        <div className="preview-chart" aria-label="今日订单变化趋势">
          <div className="preview-chart__axis"><span>12</span><span>8</span><span>4</span><span>0</span></div>
          <div className="preview-chart__plot">
            <svg viewBox="0 0 600 180" role="img" aria-label="订单趋势折线图">
              <defs>
                <linearGradient id="preview-area" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#7b55aa" stopOpacity=".28" />
                  <stop offset="100%" stopColor="#7b55aa" stopOpacity="0" />
                </linearGradient>
              </defs>
              <path d="M0 154 C55 148 70 106 125 112 S205 150 250 84 S330 52 375 78 S455 136 505 82 S565 38 600 54 L600 180 L0 180 Z" fill="url(#preview-area)" />
              <path d="M0 154 C55 148 70 106 125 112 S205 150 250 84 S330 52 375 78 S455 136 505 82 S565 38 600 54" fill="none" stroke="#684091" strokeWidth="3" strokeLinecap="round" />
            </svg>
            <div className="preview-chart__labels"><span>08:00</span><span>10:00</span><span>12:00</span><span>14:00</span><span>16:00</span></div>
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
          <div><span className="preview-scene-list__icon">骑</span><span><strong>{summary.activeRiders} 名骑手正在配送</strong><small>当前没有超时订单</small></span><StatusPill tone="success">在线</StatusPill></div>
          <div><span className="preview-scene-list__icon">审</span><span><strong>{summary.pendingReviews} 条审核任务</strong><small>最早一条已等待 42 分钟</small></span><StatusPill tone="warning">待处理</StatusPill></div>
          <div><span className="preview-scene-list__icon">材</span><span><strong>{media.filter(item => item.status === '异常').length} 个素材异常</strong><small>可进入素材中心重新处理</small></span><StatusPill tone="danger">异常</StatusPill></div>
        </div>
        <div className="preview-recent-note">
          <span>最近动态</span>
          <strong>{formatTime(orders[0]?.createdAt || new Date().toISOString())}</strong>
          <p>{orders[0]?.customer || '用户'} 的订单 {orders[0]?.id || ''} 已进入{orders[0]?.status || '处理'}。</p>
        </div>
      </Panel>
    </div>
  </div>
}
