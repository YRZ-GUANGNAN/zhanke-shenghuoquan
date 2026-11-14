import { useState } from 'react'
import {
  createId,
  type PreviewSection,
  type ServiceEntry
} from '../data'
import { usePreviewStore } from '../store'
import {
  Button,
  Field,
  Modal,
  PageHeader,
  Panel,
  StatCard,
  StatusPill,
  TextAreaField,
  Toggle,
  usePreviewToast
} from '../ui'

type SettingsSection = Extract<PreviewSection, 'navigation' | 'share-settings' | 'feature-policy'>

export function SettingsView({ section }: { section: SettingsSection }) {
  if (section === 'navigation') return <NavigationSettings />
  if (section === 'share-settings') return <ShareSettings />
  return <FeatureSettings />
}

function NavigationSettings() {
  const entries = usePreviewStore(state => state.serviceEntries)
  const addServiceEntry = usePreviewStore(state => state.addServiceEntry)
  const toggleServiceEntry = usePreviewStore(state => state.toggleServiceEntry)
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState({ title: '', subtitle: '', appId: '', path: '', color: '#684091' })
  const [error, setError] = useState('')
  const { notify } = usePreviewToast()

  // 按真实规则检查配置完整性，不再写死「正常」。
  const configIssues = [
    ...entries.filter(entry => !/^wx[0-9A-Za-z]{16}$/.test(entry.appId)).map(entry => `「${entry.title}」AppID 格式不正确`),
    ...entries.filter(entry => !entry.path.trim()).map(entry => `「${entry.title}」缺少页面路径`),
    ...Object.entries(entries.reduce<Record<number, number>>((acc, entry) => ({ ...acc, [entry.sort]: (acc[entry.sort] || 0) + 1 }), {}))
      .filter(([, count]) => count > 1).map(([sort]) => `排序值 ${sort} 重复`)
  ]

  const submit = () => {
    if (!draft.title.trim() || !draft.path.trim()) {
      setError('请填写入口名称和页面路径')
      return
    }
    if (!/^wx[0-9A-Za-z]{16}$/.test(draft.appId.trim())) {
      setError('AppID 必须是 wx 开头加 16 位字母或数字')
      return
    }
    const entry: ServiceEntry = {
      id: createId('NV'),
      title: draft.title.trim(),
      subtitle: draft.subtitle.trim() || '校园服务入口',
      appId: draft.appId.trim(),
      path: draft.path.trim(),
      color: draft.color,
      enabled: true,
      sort: entries.length * 10 + 10
    }
    addServiceEntry(entry)
    setOpen(false)
    setDraft({ title: '', subtitle: '', appId: '', path: '', color: '#684091' })
    notify('服务入口已创建并启用')
  }

  return <div>
    <PageHeader
      eyebrow="校园服务配置"
      title="服务入口"
      description="维护小程序工具页的真实服务入口，启用前应确认页面路径可用。"
      actions={<Button variant="primary" onClick={() => { setError(''); setOpen(true) }}>新增入口</Button>}
    />
    <section className="preview-stat-grid">
      <StatCard label="服务入口" value={entries.length} meta="当前配置总数" tone="accent" />
      <StatCard label="已启用" value={entries.filter(item => item.enabled).length} meta="用户端可见" tone="mint" />
      <StatCard label="已停用" value={entries.filter(item => !item.enabled).length} meta="保留配置但不展示" tone="ink" />
      <StatCard
        label="配置状态"
        value={configIssues.length === 0 ? '正常' : `${configIssues.length} 项待修`}
        meta={configIssues[0] || 'AppID 与页面路径均完整'}
        tone={configIssues.length === 0 ? 'ink' : 'rose'}
      />
    </section>
    <Panel eyebrow="SERVICE DIRECTORY" title="校园服务目录" description="入口按排序展示。停用后不会删除配置，可随时恢复。">
      <div className="preview-service-grid">
        {entries.sort((a, b) => a.sort - b.sort).map(entry => <article key={entry.id} className={!entry.enabled ? 'is-disabled' : ''}>
          <span className="preview-service-grid__icon" style={{ background: entry.color }}>{entry.title.slice(0, 1)}</span>
          <div className="preview-service-grid__copy">
            <strong>{entry.title}</strong>
            <p>{entry.subtitle}</p>
            <code>{entry.path}</code>
            <code className="preview-service-grid__appid">{entry.appId}</code>
          </div>
          <div className="preview-service-grid__controls"><StatusPill>{entry.enabled ? '已启用' : '已停用'}</StatusPill><Toggle checked={entry.enabled} label={`${entry.title}启用状态`} onChange={() => {
            toggleServiceEntry(entry.id)
            notify(entry.enabled ? '入口已停用' : '入口已启用', 'info')
          }} /></div>
        </article>)}
      </div>
    </Panel>
    <Modal
      open={open}
      title="新增服务入口"
      description="本地原型只验证配置流程，不会触发小程序跳转。"
      onClose={() => setOpen(false)}
      footer={<>
        <Button onClick={() => setOpen(false)}>取消</Button>
        <Button variant="primary" onClick={submit}>创建入口</Button>
      </>}
    >
      <div className="preview-form-stack">
        <Field label="入口名称" required value={draft.title} onChange={event => setDraft({ ...draft, title: event.target.value })} placeholder="例如：校园打印" />
        <Field label="用途说明" value={draft.subtitle} onChange={event => setDraft({ ...draft, subtitle: event.target.value })} placeholder="说明用户可以用它做什么" />
        <Field label="小程序 AppID" required value={draft.appId} onChange={event => setDraft({ ...draft, appId: event.target.value })} placeholder="wx0dacef976daa8e55" hint="必须是 wx 开头加 16 位字母或数字。" />
        <Field label="页面路径" required value={draft.path} onChange={event => setDraft({ ...draft, path: event.target.value })} placeholder="pages/service/index" hint="上线前需要用真实小程序确认路径可打开。" />
        <Field label="图标颜色" type="color" value={draft.color} onChange={event => setDraft({ ...draft, color: event.target.value })} />
        {error && <div className="preview-form-error">{error}</div>}
      </div>
    </Modal>
  </div>
}

function ShareSettings() {
  const settings = usePreviewStore(state => state.settings)
  const saveSettings = usePreviewStore(state => state.saveSettings)
  const [title, setTitle] = useState(settings.share.title)
  const [description, setDescription] = useState(settings.share.description)
  const { notify } = usePreviewToast()

  const save = () => {
    if (!title.trim()) {
      notify('分享标题不能为空', 'danger')
      return
    }
    saveSettings({ ...settings, share: { ...settings.share, title: title.trim(), description: description.trim() } })
    notify('分享设置已保存')
  }

  return <div>
    <PageHeader
      eyebrow="用户传播"
      title="分享设置"
      description="统一维护用户转发时看到的标题、说明和封面，避免不同页面出现不一致的品牌信息。"
      actions={<Button variant="primary" onClick={save}>保存设置</Button>}
    />
    <div className="preview-settings-grid">
      <Panel eyebrow="SHARE CARD" title="分享卡片" description="编辑后右侧会实时预览。">
        <div className="preview-form-stack">
          <Field label="分享标题" required value={title} onChange={event => setTitle(event.target.value)} maxLength={60} />
          <TextAreaField label="分享说明" rows={4} value={description} onChange={event => setDescription(event.target.value)} maxLength={120} />
          <Field label="默认封面" value={settings.share.coverName} readOnly hint="上传与媒体审核会在素材模块中处理。" />
        </div>
      </Panel>
      <Panel eyebrow="LIVE PREVIEW" title="用户看到的分享卡片" description="展示标题、说明和品牌来源。">
        <div className="preview-share-preview">
          <div className="preview-share-preview__cover"><span>湛科生活圈</span><strong>校园服务<br />一站直达</strong><i /></div>
          <div className="preview-share-preview__copy"><span>湛科生活圈</span><strong>{title || '未填写分享标题'}</strong><p>{description || '未填写分享说明'}</p></div>
          <div className="preview-share-preview__brand"><span>湛</span><div><strong>湛科生活圈</strong><small>发现校园好服务</small></div></div>
        </div>
      </Panel>
    </div>
  </div>
}

function FeatureSettings() {
  const settings = usePreviewStore(state => state.settings)
  const saveSettings = usePreviewStore(state => state.saveSettings)
  const [features, setFeatures] = useState(settings.features)
  const { notify } = usePreviewToast()
  const labels: Array<{ key: keyof typeof features; name: string; description: string; risk: string }> = [
    { key: 'delivery', name: '校园配送', description: '允许用户进入配送下单与骑手工作台。', risk: '涉及订单与资金，关闭前需确认在途订单。' },
    { key: 'merchant', name: '商家能力', description: '允许商家管理店铺与商品。', risk: '影响商家接单和营业状态。' },
    { key: 'rider', name: '骑手能力', description: '允许骑手接收新订单。', risk: '关闭后新订单将无法被接取。' },
    { key: 'notifications', name: '消息通知', description: '启用小程序通知和服务号提醒。', risk: '关闭后用户可能收不到关键状态变更。' },
    { key: 'community', name: '社区互动', description: '允许用户浏览和发布社区内容。', risk: '关闭后互动入口将隐藏。' },
    { key: 'takeout', name: '旧版外卖', description: '显示历史外卖入口和只读业务数据。', risk: '仅用于存量数据查看。' }
  ]

  const save = () => {
    saveSettings({ ...settings, features })
    notify('功能开关已保存')
  }

  return <div>
    <PageHeader
      eyebrow="功能治理"
      title="功能开关"
      description="每个开关都说明影响范围与风险，不把高风险配置藏在一排无上下文开关里。"
      actions={<Button variant="primary" onClick={save}>保存开关</Button>}
    />
    <Panel eyebrow="FEATURE POLICY" title="模块开放状态" description="本地原型只改变预览菜单与提示，不会影响真实小程序。">
      <div className="preview-feature-list">
        {labels.map(item => <article key={item.key}>
          <span className="preview-feature-list__icon">{item.name.slice(0, 1)}</span>
          <div className="preview-feature-list__copy"><div><strong>{item.name}</strong><StatusPill tone={features[item.key] ? 'success' : 'neutral'}>{features[item.key] ? '已开启' : '已关闭'}</StatusPill></div><p>{item.description}</p><small>{item.risk}</small></div>
          <Toggle checked={features[item.key]} label={item.name} onChange={checked => setFeatures({ ...features, [item.key]: checked })} />
        </article>)}
      </div>
      <div className="preview-policy-note">
        <span>变更纪律</span>
        <p>生产环境需要保留操作人、时间与原因。预览版本只保存到 localStorage。</p>
      </div>
    </Panel>
  </div>
}
