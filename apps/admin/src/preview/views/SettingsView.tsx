import { useState } from 'react'
import {
  createId,
  FEATURE_GROUPS,
  FEATURE_KEYS,
  type FeatureKey,
  type PreviewSection,
  type PreviewSettings,
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
  const [draft, setDraft] = useState<PreviewSettings>(settings)
  const [saved, setSaved] = useState(settings)
  const { notify } = usePreviewToast()

  const dirty = JSON.stringify(draft) !== JSON.stringify(saved)
  const enabledIn = (keys: Array<{ key: FeatureKey }>) => keys.filter(item => draft.features[item.key]).length
  const wholeGroupOn = (keys: Array<{ key: FeatureKey }>) => keys.every(item => draft.features[item.key])

  const save = () => {
    const next = { ...draft, rowVersion: draft.rowVersion + 1, updatedAt: new Date().toISOString() }
    saveSettings(next)
    setSaved(next)
    setDraft(next)
    notify('功能开关已保存，小程序回到前台后生效')
  }

  return <div>
    <PageHeader
      eyebrow="平台设置"
      title="功能开关"
      description="控制小程序模块、互动能力与新业务开放状态"
      actions={<>
        <Button onClick={() => { setDraft(saved); notify('已恢复到上次保存的配置', 'info') }}>读取已保存配置</Button>
        <Button onClick={() => { setDraft({ ...draft, features: Object.fromEntries(FEATURE_KEYS.map(key => [key, false])) as Record<FeatureKey, boolean> }); notify('已全部关闭，保存后生效', 'info') }}>全部隐藏</Button>
        <Button variant="primary" disabled={!dirty} onClick={save}>保存配置</Button>
      </>}
    />

    <Panel eyebrow="FEATURE POLICY" title="小程序功能显隐中心" description="选择需要开放的模块，保存后控制所有用户的小程序入口和新业务。账号安全、隐私、反馈与已有订单的必要处理保持可用。">
      <div className="preview-saved-bar">
        <span>当前显示{dirty ? '未保存的编辑内容' : '服务器已保存配置'}</span>
        <small>版本 {saved.rowVersion} · 最近更新 {new Date(saved.updatedAt).toLocaleString('zh-CN')}</small>
      </div>

      <section className="preview-policy-block">
        <div className="preview-policy-block__head">
          <strong>小程序展示模式</strong>
          <span>选择后点击“保存配置”，对所有用户统一生效。</span>
        </div>
        <div className="preview-choice-grid">
          {([
            { value: 'standard', title: '原版', description: '恢复原首页、服务和二手等模块，按下方已保存的开关开放' },
            { value: 'campus_info', title: '校园资讯', description: '展示独立的校园指南，保留账号、反馈和既有订单的必要处理' }
          ] as const).map(option => <button
            key={option.value}
            type="button"
            className={`preview-choice${draft.presentationMode === option.value ? ' is-selected' : ''}`}
            aria-pressed={draft.presentationMode === option.value}
            onClick={() => setDraft({ ...draft, presentationMode: option.value })}
          >
            <span className="preview-choice__dot" aria-hidden="true" />
            <strong>{option.title}</strong>
            <small>{option.description}</small>
          </button>)}
        </div>
      </section>

      <section className="preview-policy-block">
        <div className="preview-policy-block__head">
          <strong>发帖与评论权限</strong>
          <span>普通用户不会因此获得后台管理权限；这里只控制社区写入能力。</span>
        </div>
        <div className="preview-choice-grid">
          {([
            { value: 'admin_only', title: '仅管理员', description: '普通用户只能浏览，管理员与超级管理员可发帖评论' },
            { value: 'all_users', title: '全部用户', description: '所有通过身份与禁言检查的用户可发帖评论' }
          ] as const).map(option => <button
            key={option.value}
            type="button"
            className={`preview-choice${draft.postingMode === option.value ? ' is-selected' : ''}`}
            aria-pressed={draft.postingMode === option.value}
            onClick={() => setDraft({ ...draft, postingMode: option.value })}
          >
            <span className="preview-choice__dot" aria-hidden="true" />
            <strong>{option.title}</strong>
            <small>{option.description}</small>
          </button>)}
        </div>
      </section>

      {FEATURE_GROUPS.map(group => <section className="preview-policy-block" key={group.title}>
        <div className="preview-policy-block__head">
          <strong>{group.title}</strong>
          <span>{group.description}</span>
          <em>{enabledIn(group.keys)}/{group.keys.length} 已开启
            <button type="button" onClick={() => {
              const next = !wholeGroupOn(group.keys)
              setDraft(current => ({ ...current, features: { ...current.features, ...Object.fromEntries(group.keys.map(item => [item.key, next])) } as Record<FeatureKey, boolean> }))
            }}>{wholeGroupOn(group.keys) ? '关闭本组' : '开启本组'}</button>
          </em>
        </div>
        <div className="preview-policy-rows">
          {group.keys.map(item => <label className={`preview-policy-row${draft.features[item.key] ? '' : ' is-off'}`} key={item.key}>
            <span><strong>{item.label}</strong><small>{item.description}</small></span>
            <Toggle
              checked={draft.features[item.key]}
              label={item.label}
              onChange={checked => setDraft(current => ({ ...current, features: { ...current.features, [item.key]: checked } as Record<FeatureKey, boolean> }))}
            />
          </label>)}
        </div>
      </section>)}

      <div className="preview-policy-note">
        <span>变更纪律</span>
        <p>生产环境会记录操作人、时间与原因；本地原型只保存到 localStorage，不会影响真实小程序。</p>
      </div>
    </Panel>
  </div>
}
