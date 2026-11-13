import { useCallback, useEffect, useState } from 'react'
import {
  AdminBadge,
  AdminButton,
  AdminNotice,
  AdminPanel,
  AdminState,
  AdminStatusBadge,
  AdminToolbar,
  useAdminDialog
} from './components'
import {
  deliveryAdminAPI,
  type NotificationChannel,
  type NotificationChannelOverview
} from './services/delivery-admin'

const channelDescriptions: Record<NotificationChannel['channel'], string> = {
  in_app: '用户打开小程序后可在通知中心查看完整的订单状态记录，是其他提醒的可靠底账。',
  wechat_subscribe: '用户每授权一次，就会增加一次微信订单提醒；额度用完前仍可接收。',
  wechat_official: '用户绑定并关注服务号后，订单状态可以持续通过微信模板消息提醒。',
  wecom: '商家和骑手值班群接收订单汇总，适合多人协同和处理异常。'
}

const channelNames: Record<NotificationChannel['channel'], string> = {
  in_app: '小程序通知中心',
  wechat_subscribe: '小程序订单提醒',
  wechat_official: '微信服务号提醒',
  wecom: '值班群提醒'
}

const number = new Intl.NumberFormat('zh-CN')
const timestamp = (value?: string) => value
  ? new Intl.DateTimeFormat('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit'
    }).format(new Date(value))
  : '尚未刷新'
const errorMessage = (reason: unknown) => reason instanceof Error ? reason.message : String(reason)

export function NotificationConsole() {
  const dialog = useAdminDialog()
  const [value, setValue] = useState<NotificationChannelOverview | null>(null)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [loading, setLoading] = useState(false)
  const [testing, setTesting] = useState<'merchant_ops' | 'rider_ops' | ''>('')

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      setValue(await deliveryAdminAPI.notificationChannels())
    } catch (reason) {
      setError(errorMessage(reason))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const testGroup = async (group: 'merchant_ops' | 'rider_ops') => {
    const label = group === 'merchant_ops' ? '商家值班群' : '骑手值班群'
    if (!await dialog.confirm({
      title: `发送${label}测试`,
      description: `这会通过企业微信机器人发送一条真实测试消息，不关联订单，也不会附带用户手机号或地址。`,
      confirmLabel: '发送测试'
    })) return
    setTesting(group)
    setError('')
    setNotice('')
    try {
      await deliveryAdminAPI.testWecomGroup(group)
      setNotice(`${label}已受理测试消息。若群内未出现，请检查机器人是否仍在群内。`)
      await load()
    } catch (reason) {
      setError(errorMessage(reason))
    } finally {
      setTesting('')
    }
  }

  const accepted = value?.channels.reduce((total, channel) => total + channel.accepted, 0) || 0
  const pending = value?.channels.reduce((total, channel) => total + channel.pending, 0) || 0
  const attention = value?.channels.reduce(
    (total, channel) => total + channel.retryWait + channel.blocked + channel.deadletter,
    0
  ) || 0
  const bindingTotal = value
    ? value.officialAccount.unknown + value.officialAccount.following + value.officialAccount.notFollowing
    : 0

  return <div className="notification-console">
    <AdminPanel
      title="消息通知概览"
      description="查看订单和私信提醒是否正常发出。账号密钥与群机器人地址只在服务端安全配置，管理端不会读取或展示。"
      actions={<AdminButton tone="secondary" loading={loading} onClick={() => void load()}>刷新状态</AdminButton>}
    >
      <AdminToolbar
        summary={<>
          <span>最近刷新</span>
          <strong>{timestamp(value?.checkedAt)}</strong>
        </>}
        actions={<AdminBadge tone={value?.sendingEnabled ? 'success' : 'warning'}>
          {value?.sendingEnabled ? '微信消息发送已开启' : '微信消息发送未开启'}
        </AdminBadge>}
      />
      {error && <AdminNotice tone="danger" title="通知状态读取失败">
        {error}。请确认管理员会话有效，并检查通知服务是否正常运行。
      </AdminNotice>}
      {notice && <AdminNotice tone="success">{notice}</AdminNotice>}
      {!value && loading
        ? <AdminState kind="loading" title="正在读取通知通道" description="配置、投递与绑定聚合数据正在同步。" />
        : !value
          ? <AdminState kind="error" title="暂时没有通知数据" actionLabel="重新加载" onAction={() => void load()} />
          : <>
            <section className="notification-overview" aria-label="通知运行指标">
              <article>
                <span>可接收用户</span>
                <strong className="notification-overview__text">{value.audience === 'all' ? '全部已授权用户' : '测试账号'}</strong>
                <small>{value.audience === 'all' ? '按用户授权和免打扰时间发送' : '当前只向指定测试账号发送'}</small>
              </article>
              <article>
                <span>近 24 小时发出</span>
                <strong>{number.format(accepted)}</strong>
                <small>各通知方式已成功发送的提醒总数</small>
              </article>
              <article>
                <span>排队发送中</span>
                <strong>{number.format(pending)}</strong>
                <small>正在等待发送或已经进入发送流程</small>
              </article>
              <article className={attention > 0 ? 'notification-overview__attention' : ''}>
                <span>发送异常</span>
                <strong>{number.format(attention)}</strong>
                <small>需要重试、结果未知或多次失败的提醒</small>
              </article>
            </section>
            {!value.sendingEnabled && <AdminNotice tone="warning" title="微信消息发送仍处于关闭状态">
              小程序内通知不受影响；小程序提醒和微信服务号会在服务端发送开关开启后发出。
            </AdminNotice>}
          </>}
    </AdminPanel>

    {value && <>
      <AdminPanel
        title="各通知方式状态"
        description="以下为最近 24 小时的数据。配置完成仍需要真实账号和真实订单完成一次验收。"
      >
        <div className="notification-channel-grid">
          {value.channels.map(channel => <article
            className={`notification-channel-card notification-channel-card--${channel.enabled ? 'enabled' : 'disabled'}`}
            key={channel.channel}
          >
            <header>
              <div>
                <span>{channel.enabled ? '已配置' : '未配置'}</span>
                <h3>{channelNames[channel.channel]}</h3>
              </div>
              <AdminStatusBadge value={channel.enabled ? 'enabled' : 'disabled'} />
            </header>
            <p>{channelDescriptions[channel.channel]}</p>
            <dl>
              <div><dt>排队中</dt><dd>{number.format(channel.pending)}</dd></div>
              <div><dt>已发出</dt><dd>{number.format(channel.accepted)}</dd></div>
              <div><dt>等待重试</dt><dd>{number.format(channel.retryWait)}</dd></div>
              <div><dt>未发送</dt><dd>{number.format(channel.blocked)}</dd></div>
              <div><dt>多次失败</dt><dd>{number.format(channel.deadletter)}</dd></div>
            </dl>
          </article>)}
        </div>
      </AdminPanel>

      <div className="notification-detail-grid">
        <AdminPanel
          title="微信服务号绑定"
          description="用户绑定并关注服务号后，订单提醒可以持续发送；关注状态会在真实发送后更新。"
        >
          <div className="notification-readiness">
            <div><span>服务号提醒</span><AdminStatusBadge value={value.officialAccount.enabled ? 'enabled' : 'disabled'} /></div>
            <div><span>订单通知模板</span><AdminStatusBadge value={value.officialAccount.templateConfigured ? 'enabled' : 'disabled'} /></div>
            <div><span>已绑定用户</span><strong>{number.format(bindingTotal)}</strong></div>
          </div>
          <div className="notification-binding-summary">
            <div><span>已关注服务号</span><strong>{number.format(value.officialAccount.following)}</strong></div>
            <div><span>需要重新关注</span><strong>{number.format(value.officialAccount.notFollowing)}</strong></div>
            <div><span>关注状态待确认</span><strong>{number.format(value.officialAccount.unknown)}</strong></div>
          </div>
          <progress
            className="notification-binding-progress"
            max={Math.max(bindingTotal, 1)}
            value={value.officialAccount.following}
            aria-label={`已确认关注 ${value.officialAccount.following} 人，共绑定 ${bindingTotal} 人`}
          />
          <p className="notification-panel-note">
            只有用户主动绑定并关注服务号后，微信提醒才可能送达。未关注、模板审核变化或账号迁移，都需要在真实发送记录中确认。
          </p>
        </AdminPanel>

        <AdminPanel
          title="值班群发送测试"
          description="测试会发送真实群消息，但不会生成订单、资金流水或用户通知。"
        >
          <div className="notification-group-list">
            <div>
              <span><strong>商家值班群</strong><small>接收商家订单和异常提醒</small></span>
              <AdminBadge tone={value.wecom.merchantConfigured ? 'success' : 'neutral'}>
                {value.wecom.merchantConfigured ? '已配置' : '未配置'}
              </AdminBadge>
              <AdminButton
                tone="secondary"
                loading={testing === 'merchant_ops'}
                disabled={Boolean(testing) || !value.wecom.merchantConfigured}
                onClick={() => void testGroup('merchant_ops')}
              >测试群消息</AdminButton>
            </div>
            <div>
              <span><strong>骑手值班群</strong><small>接收骑手调度和异常提醒</small></span>
              <AdminBadge tone={value.wecom.riderConfigured ? 'success' : 'neutral'}>
                {value.wecom.riderConfigured ? '已配置' : '未配置'}
              </AdminBadge>
              <AdminButton
                tone="secondary"
                loading={testing === 'rider_ops'}
                disabled={Boolean(testing) || !value.wecom.riderConfigured}
                onClick={() => void testGroup('rider_ops')}
              >测试群消息</AdminButton>
            </div>
          </div>
          <div className="notification-config-contract">
            <strong>安全说明</strong>
            <span>群机器人地址由服务端安全配置，本页不会读取、显示或修改机器人密钥。</span>
          </div>
        </AdminPanel>
      </div>
    </>}
  </div>
}
