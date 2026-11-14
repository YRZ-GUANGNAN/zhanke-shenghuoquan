import { useMemo, useState } from 'react'
import {
  channelHealthFrom,
  createId,
  formatTime,
  maskEndpoint,
  templateVariables,
  type MessageTemplateItem,
  type NotificationDelivery,
  type WebhookItem
} from '../data'
import { usePreviewStore } from '../store'
import {
  Button,
  EmptyState,
  Field,
  Modal,
  PageHeader,
  Panel,
  ProgressBar,
  SearchBox,
  Segmented,
  SelectField,
  StatCard,
  StatusPill,
  TextAreaField,
  Toggle,
  usePreviewToast
} from '../ui'

type NotificationTab = 'overview' | 'templates' | 'webhooks' | 'deliveries'

interface TemplateDraft {
  id?: string
  name: string
  type: MessageTemplateItem['type']
  content: string
  enabled: boolean
}

interface WebhookDraft {
  id?: string
  name: string
  type: WebhookItem['type']
  endpoint: string
  enabled: boolean
}

const emptyTemplate = (): TemplateDraft => ({
  name: '',
  type: '新订单',
  content: '收到新订单 {orderNo}\n取件：{pickup}\n送达：{dropoff}',
  enabled: true
})

const emptyWebhook = (): WebhookDraft => ({
  name: '',
  type: '企业微信',
  endpoint: '',
  enabled: true
})

const previewContent = (content: string) => content
  .replace(/\{orderNo\}/g, 'CC20261114031')
  .replace(/\{customerName\}/g, '林同学')
  .replace(/\{pickup\}/g, '北区快递站')
  .replace(/\{dropoff\}/g, '东区 7 号宿舍')
  .replace(/\{amount\}/g, '¥6.80')
  .replace(/\{deliveryTime\}/g, '18:30')
  .replace(/\{riderName\}/g, '周师傅')

export function NotificationsView() {
  const templates = usePreviewStore(state => state.templates)
  const webhooks = usePreviewStore(state => state.webhooks)
  const deliveries = usePreviewStore(state => state.deliveries)
  const addTemplate = usePreviewStore(state => state.addTemplate)
  const updateTemplate = usePreviewStore(state => state.updateTemplate)
  const toggleTemplate = usePreviewStore(state => state.toggleTemplate)
  const addWebhook = usePreviewStore(state => state.addWebhook)
  const updateWebhook = usePreviewStore(state => state.updateWebhook)
  const toggleWebhook = usePreviewStore(state => state.toggleWebhook)
  const testWebhook = usePreviewStore(state => state.testWebhook)
  const testSend = usePreviewStore(state => state.testSend)
  const retryDelivery = usePreviewStore(state => state.retryDelivery)
  const [tab, setTab] = useState<NotificationTab>('overview')
  const [search, setSearch] = useState('')
  const [templateDraft, setTemplateDraft] = useState<TemplateDraft | null>(null)
  const [templateError, setTemplateError] = useState('')
  const [webhookDraft, setWebhookDraft] = useState<WebhookDraft | null>(null)
  const [webhookError, setWebhookError] = useState('')
  const [deliveryStatus, setDeliveryStatus] = useState<'全部' | NotificationDelivery['status']>('全部')
  const [testingTemplate, setTestingTemplate] = useState<MessageTemplateItem | null>(null)
  const [testRecipient, setTestRecipient] = useState('运营测试账号')
  const [testChannel, setTestChannel] = useState<NotificationDelivery['channel']>('小程序通知')
  const { notify } = usePreviewToast()

  const normalizedSearch = search.trim().toLowerCase()
  const filteredTemplates = templates.filter(item =>
    !normalizedSearch || `${item.name}${item.type}${item.content}`.toLowerCase().includes(normalizedSearch)
  )
  const filteredWebhooks = webhooks.filter(item =>
    !normalizedSearch || `${item.name}${item.type}${item.endpoint}`.toLowerCase().includes(normalizedSearch)
  )
  const filteredDeliveries = deliveries.filter(item =>
    (deliveryStatus === '全部' || item.status === deliveryStatus)
    && (!normalizedSearch || `${item.templateName}${item.channel}${item.recipient}`.toLowerCase().includes(normalizedSearch))
  )

  const stats = useMemo(() => ({
    accepted: deliveries.filter(item => item.status === '已送达').length,
    failed: deliveries.filter(item => item.status === '失败').length,
    queued: deliveries.filter(item => item.status === '发送中').length,
    enabledTemplates: templates.filter(item => item.enabled).length
  }), [deliveries, templates])

  const editTemplate = (template?: MessageTemplateItem) => {
    setTemplateError('')
    setTemplateDraft(template ? { ...template } : emptyTemplate())
  }

  const saveTemplate = () => {
    if (!templateDraft?.name.trim()) {
      setTemplateError('模板名称不能为空')
      return
    }
    if (!templateDraft.content.trim()) {
      setTemplateError('模板内容不能为空')
      return
    }
    const template: MessageTemplateItem = {
      id: templateDraft.id || createId('TP'),
      name: templateDraft.name.trim(),
      type: templateDraft.type,
      content: templateDraft.content.trim(),
      enabled: templateDraft.enabled,
      updatedAt: new Date().toISOString()
    }
    if (templateDraft.id) {
      updateTemplate(template)
      notify('消息模板已更新')
    } else {
      addTemplate(template)
      notify('消息模板已创建')
    }
    setTemplateDraft(null)
  }

  const editWebhook = (webhook?: WebhookItem) => {
    setWebhookError('')
    setWebhookDraft(webhook ? { ...webhook } : emptyWebhook())
  }

  const saveWebhook = () => {
    if (!webhookDraft?.name.trim()) {
      setWebhookError('请填写渠道名称')
      return
    }
    if (!/^https:\/\//i.test(webhookDraft.endpoint.trim())) {
      setWebhookError('Webhook 地址必须以 https:// 开头')
      return
    }
    const webhook: WebhookItem = {
      id: webhookDraft.id || createId('WH'),
      name: webhookDraft.name.trim(),
      type: webhookDraft.type,
      endpoint: webhookDraft.endpoint.trim(),
      enabled: webhookDraft.enabled,
      lastTest: webhookDraft.id
        ? webhooks.find(item => item.id === webhookDraft.id)?.lastTest || '未测试'
        : '未测试',
      updatedAt: new Date().toISOString()
    }
    if (webhookDraft.id) {
      updateWebhook(webhook)
      notify('Webhook 配置已更新')
    } else {
      addWebhook(webhook)
      notify('Webhook 配置已创建')
    }
    setWebhookDraft(null)
  }

  const handleTestTemplate = () => {
    if (!testingTemplate) return
    const record = testSend(testingTemplate.id, testChannel, testRecipient.trim() || '运营测试账号')
    notify(record.status === '已送达' ? '测试消息已记录为送达' : record.error || '测试消息已跳过', record.status === '已送达' ? 'success' : 'info')
    setTestingTemplate(null)
    setTab('deliveries')
  }

  const insertVariable = (key: string) => {
    if (!templateDraft) return
    setTemplateDraft({ ...templateDraft, content: `${templateDraft.content}${templateDraft.content ? ' ' : ''}${key}` })
  }

  return <div>
    <PageHeader
      eyebrow="服务基础设施"
      title="消息通知"
      description="把模板、渠道和真实发送结果放在同一处，异常提醒会直接告诉管理员下一步该做什么。"
      actions={<>
        <Button variant="ghost" onClick={() => editWebhook()}>新增 Webhook</Button>
        <Button variant="primary" onClick={() => editTemplate()}>新建消息模板</Button>
      </>}
    />

    <Segmented
      className="preview-page-tabs"
      value={tab}
      onChange={value => { setTab(value); setSearch('') }}
      options={[
        { value: 'overview', label: '运行概览' },
        { value: 'templates', label: '消息模板', badge: templates.length },
        { value: 'webhooks', label: 'Webhook', badge: webhooks.length },
        { value: 'deliveries', label: '发送记录', badge: deliveries.length }
      ]}
    />

    {tab === 'overview' && <div className="preview-notification-overview">
      <section className="preview-stat-grid">
        <StatCard label="近 24 小时送达" value={stats.accepted} meta="所有通知通道" tone="mint" />
        <StatCard label="正在发送" value={stats.queued} meta="队列中的消息" tone="accent" />
        <StatCard label="发送失败" value={stats.failed} meta="可进入记录重试" tone="rose" />
        <StatCard label="近 24 小时送达率" value={`${stats.accepted + stats.failed > 0 ? Math.round(stats.accepted / (stats.accepted + stats.failed) * 100) : 100}%`} meta={`${stats.accepted + stats.failed} 条中 ${stats.accepted} 条送达`} tone="amber" />
      </section>

      <div className="preview-notification-grid">
        <Panel eyebrow="CHANNEL STATUS" title="渠道健康" description="从配置、模板到最近一次发送逐项检查。">
          <div className="preview-channel-health-cards">
            {channelHealthFrom(deliveries).map(channel => <article key={channel.name}>
              <div className="preview-channel-health-cards__head">
                <span className={`preview-channel-health-cards__icon ${channel.rate !== null && channel.rate < 90 ? 'is-warning' : ''}`}>◉</span>
                <div><strong>{channel.name}</strong><small>{channel.failed ? `${channel.failed} 条失败` : '无失败记录'}</small></div>
                <span>{channel.rate === null ? '—' : `${channel.rate}%`}</span>
              </div>
              <ProgressBar value={channel.rate ?? 0} label={`${channel.name} 送达率`} />
              <div className="preview-channel-health-cards__foot">
                <StatusPill tone={channel.rate !== null && channel.rate < 90 ? 'warning' : 'success'}>
                  {channel.rate === null ? '暂无结果' : channel.failed ? '需要关注' : '运行正常'}
                </StatusPill>
                <p>{channel.delivered + channel.failed} 条中 {channel.delivered} 条送达{channel.pending ? `，${channel.pending} 条发送中` : ''}</p>
              </div>
            </article>)}
          </div>
        </Panel>

        <Panel eyebrow="NEXT ACTION" title="建议现在处理" description="只列出会影响送达结果的事项。">
          <div className="preview-action-stack">
            {stats.failed > 0 && <button type="button" onClick={() => { setTab('deliveries'); setDeliveryStatus('失败') }}>
              <span className="preview-action-stack__index">01</span>
              <div><strong>重试 {stats.failed} 条失败消息</strong><p>优先检查订阅授权额度和模板状态。</p></div><span>→</span>
            </button>}
            {webhooks.some(item => item.lastTest === '未测试') && <button type="button" onClick={() => setTab('webhooks')}>
              <span className="preview-action-stack__index">02</span>
              <div><strong>完成 Webhook 测试</strong><p>确认机器人在群内且地址有效。</p></div><span>→</span>
            </button>}
            <button type="button" onClick={() => setTab('templates')}>
              <span className="preview-action-stack__index">03</span>
              <div><strong>检查模板变量</strong><p>避免订单号、地点或骑手信息缺失。</p></div><span>→</span>
            </button>
          </div>
          <div className="preview-policy-note">
            <span>安全边界</span>
            <p>预览环境不会向真实用户、服务号或值班群发送消息。测试操作只生成本地发送记录。</p>
          </div>
        </Panel>
      </div>
    </div>}

    {tab === 'templates' && <Panel
      eyebrow="MESSAGE LIBRARY"
      title="消息模板"
      description="模板内容支持变量替换；停用后，新触发的通知会标记为跳过。"
      actions={<SearchBox value={search} onChange={setSearch} placeholder="搜索模板名称或内容" />}
    >
      {filteredTemplates.length === 0
        ? <EmptyState title="没有匹配的模板" description="调整搜索词，或新建一个消息模板。" action={<Button variant="primary" onClick={() => editTemplate()}>新建模板</Button>} />
        : <div className="preview-template-grid">
          {filteredTemplates.map(template => <article key={template.id} className="preview-template-card">
            <header>
              <div>
                <span className="preview-template-card__type">{template.type}</span>
                <h3>{template.name}</h3>
              </div>
              <Toggle checked={template.enabled} label={`${template.name}启用状态`} onChange={() => {
                toggleTemplate(template.id)
                notify(template.enabled ? '模板已停用' : '模板已启用', 'info')
              }} />
            </header>
            <div className="preview-template-card__preview">
              <span>消息预览</span>
              <p>{previewContent(template.content)}</p>
            </div>
            <footer>
              <span>{formatTime(template.updatedAt)} 更新</span>
              <div>
                <Button size="sm" variant="ghost" onClick={() => setTestingTemplate(template)}>测试发送</Button>
                <Button size="sm" variant="quiet" onClick={() => editTemplate(template)}>编辑</Button>
              </div>
            </footer>
          </article>)}
        </div>}
    </Panel>}

    {tab === 'webhooks' && <Panel
      eyebrow="DELIVERY CHANNELS"
      title="Webhook 渠道"
      description="用于商家、骑手和运营值班群。列表中的地址只显示脱敏信息。"
      actions={<SearchBox value={search} onChange={setSearch} placeholder="搜索渠道名称或类型" />}
    >
      <div className="preview-toolbar">
        <span className="preview-toolbar__hint">共 {filteredWebhooks.length} 个渠道</span>
        <Button size="sm" variant="primary" onClick={() => editWebhook()}>新增 Webhook</Button>
      </div>
      {filteredWebhooks.length === 0
        ? <EmptyState title="暂无 Webhook" description="新增一个企业微信或飞书机器人渠道。" action={<Button variant="primary" onClick={() => editWebhook()}>新增 Webhook</Button>} />
        : <div className="preview-table-wrap">
          <table className="preview-table" aria-label="发送记录列表">
            <thead><tr><th>渠道</th><th>类型</th><th>地址</th><th>最近测试</th><th>启用</th><th /></tr></thead>
            <tbody>{filteredWebhooks.map(webhook => <tr key={webhook.id}>
              <td><strong>{webhook.name}</strong><span>{formatTime(webhook.updatedAt)} 更新</span></td>
              <td><StatusPill tone={webhook.type === '飞书' ? 'info' : 'accent'}>{webhook.type}</StatusPill></td>
              <td><code className="preview-code">{maskEndpoint(webhook.endpoint)}</code></td>
              <td><StatusPill>{webhook.lastTest}</StatusPill></td>
              <td><Toggle checked={webhook.enabled} label={`${webhook.name}启用状态`} onChange={() => {
                toggleWebhook(webhook.id)
                notify(webhook.enabled ? 'Webhook 已停用' : 'Webhook 已启用', 'info')
              }} /></td>
              <td><div className="preview-row-actions">
                <Button size="sm" variant="ghost" onClick={() => {
                  const result = testWebhook(webhook.id)
                  notify(result === '测试成功' ? 'Webhook 测试成功' : 'Webhook 未启用或地址无效，测试失败', result === '测试成功' ? 'success' : 'danger')
                }}>测试</Button>
                <Button size="sm" variant="quiet" onClick={() => editWebhook(webhook)}>编辑</Button>
              </div></td>
            </tr>)}</tbody>
          </table>
        </div>}
    </Panel>}

    {tab === 'deliveries' && <Panel
      eyebrow="DELIVERY LOG"
      title="发送记录"
      description="每次通知都保留模板、渠道、收件人和失败原因。"
      actions={<SearchBox value={search} onChange={setSearch} placeholder="搜索模板、渠道或收件人" />}
    >
      <div className="preview-toolbar">
        <Segmented
          value={deliveryStatus}
          onChange={setDeliveryStatus}
          options={[
            { value: '全部', label: '全部', badge: deliveries.length },
            { value: '失败', label: '失败', badge: deliveries.filter(item => item.status === '失败').length },
            { value: '发送中', label: '发送中' },
            { value: '已送达', label: '已送达' },
            { value: '已跳过', label: '已跳过' }
          ]}
        />
        <span className="preview-toolbar__hint">显示 {filteredDeliveries.length} 条记录</span>
      </div>
      {filteredDeliveries.length === 0
        ? <EmptyState title="没有匹配的记录" description="调整筛选条件后再试。" />
        : <div className="preview-table-wrap">
          <table className="preview-table" aria-label="Webhook 列表">
            <thead><tr><th>消息</th><th>渠道</th><th>收件人</th><th>状态</th><th>尝试</th><th>时间</th><th /></tr></thead>
            <tbody>{filteredDeliveries.map(record => <tr key={record.id}>
              <td><strong>{record.templateName}</strong><span>{record.error || record.id}</span></td>
              <td>{record.channel}</td>
              <td>{record.recipient}</td>
              <td><StatusPill>{record.status}</StatusPill></td>
              <td>{record.attempts} 次</td>
              <td>{formatTime(record.createdAt)}</td>
              <td>{record.status === '失败' && <Button size="sm" variant="primary" onClick={() => {
                retryDelivery(record.id)
                notify('失败消息已重新发送，并记录为送达')
              }}>重试</Button>}</td>
            </tr>)}</tbody>
          </table>
        </div>}
    </Panel>}

    <Modal
      open={Boolean(templateDraft)}
      title={templateDraft?.id ? '编辑消息模板' : '新建消息模板'}
      description="模板保存后可在本地测试发送，不会触达真实用户。"
      onClose={() => setTemplateDraft(null)}
      width="lg"
      footer={<>
        <Button onClick={() => setTemplateDraft(null)}>取消</Button>
        <Button variant="primary" onClick={saveTemplate}>保存模板</Button>
      </>}
    >
      {templateDraft && <div className="preview-template-editor">
        <div className="preview-template-editor__form">
          <div className="preview-form-grid">
            <Field label="模板名称" required value={templateDraft.name} onChange={event => setTemplateDraft({ ...templateDraft, name: event.target.value })} placeholder="例如：新订单值班提醒" />
            <SelectField label="模板类型" value={templateDraft.type} onChange={event => setTemplateDraft({ ...templateDraft, type: event.target.value as MessageTemplateItem['type'] })}>
              <option>新订单</option><option>接单通知</option><option>配送完成</option><option>审核结果</option><option>提现结果</option>
            </SelectField>
          </div>
          <TextAreaField
            label="模板内容"
            required
            value={templateDraft.content}
            rows={10}
            onChange={event => setTemplateDraft({ ...templateDraft, content: event.target.value })}
            hint="支持下面列出的订单变量，未知变量会在预览中原样显示。"
          />
          <div className="preview-variable-list">
            {templateVariables.map(variable => <button key={variable.key} type="button" onClick={() => insertVariable(variable.key)}>
              <code>{variable.key}</code><span>{variable.label}</span><b>+</b>
            </button>)}
          </div>
          {templateError && <div className="preview-form-error">{templateError}</div>}
        </div>
        <aside className="preview-template-editor__preview">
          <div className="preview-phone-bar"><span>消息预览</span><small>小程序通知</small></div>
          <div className="preview-message-preview">
            <span className="preview-message-preview__app">湛科生活圈</span>
            <h3>{templateDraft.name || '未命名模板'}</h3>
            <p>{previewContent(templateDraft.content || '在左侧填写模板内容后，这里会显示预览。')}</p>
            <small>现在</small>
          </div>
          <div className="preview-policy-note">
            <span>发送前检查</span>
            <p>模板停用后，发送记录会显示“已跳过”；变量缺失不会自动发送空消息。</p>
          </div>
        </aside>
      </div>}
    </Modal>

    <Modal
      open={Boolean(webhookDraft)}
      title={webhookDraft?.id ? '编辑 Webhook' : '新增 Webhook'}
      description="地址在保存后只以脱敏形式显示。"
      onClose={() => setWebhookDraft(null)}
      footer={<>
        <Button onClick={() => setWebhookDraft(null)}>取消</Button>
        <Button variant="primary" onClick={saveWebhook}>保存配置</Button>
      </>}
    >
      {webhookDraft && <div className="preview-form-stack">
        <Field label="渠道名称" required value={webhookDraft.name} onChange={event => setWebhookDraft({ ...webhookDraft, name: event.target.value })} placeholder="例如：骑手调度群" />
        <SelectField label="渠道类型" value={webhookDraft.type} onChange={event => setWebhookDraft({ ...webhookDraft, type: event.target.value as WebhookItem['type'] })}>
          <option>企业微信</option><option>飞书</option>
        </SelectField>
        <Field label="Webhook 地址" required value={webhookDraft.endpoint} onChange={event => setWebhookDraft({ ...webhookDraft, endpoint: event.target.value })} placeholder="https://…" hint="必须使用 HTTPS。预览环境不会发起真实网络请求。" />
        <label className="preview-check-line"><Toggle checked={webhookDraft.enabled} label="启用 Webhook" onChange={checked => setWebhookDraft({ ...webhookDraft, enabled: checked })} /><span>保存后立即启用</span></label>
        {webhookError && <div className="preview-form-error">{webhookError}</div>}
      </div>}
    </Modal>

    <Modal
      open={Boolean(testingTemplate)}
      title="测试发送"
      description={`使用“${testingTemplate?.name || ''}”生成一条本地测试记录。`}
      onClose={() => setTestingTemplate(null)}
      width="sm"
      footer={<>
        <Button onClick={() => setTestingTemplate(null)}>取消</Button>
        <Button variant="primary" onClick={handleTestTemplate}>生成测试记录</Button>
      </>}
    >
      <div className="preview-form-stack">
        <SelectField label="发送渠道" value={testChannel} onChange={event => setTestChannel(event.target.value as NotificationDelivery['channel'])}>
          <option>小程序通知</option><option>微信订阅</option><option>服务号</option><option>值班群</option>
        </SelectField>
        <Field label="测试收件人" value={testRecipient} onChange={event => setTestRecipient(event.target.value)} hint="只作为记录标签使用，不会发送真实消息。" />
      </div>
    </Modal>
  </div>
}
