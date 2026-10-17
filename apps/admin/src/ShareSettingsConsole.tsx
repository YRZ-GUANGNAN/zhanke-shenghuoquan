import { useEffect, useRef, useState } from 'react'
import { AdminButton, AdminField, AdminNotice, AdminPanel, AdminState, useAdminDialog } from './components'
import { cardFields, coverError, shareSettingsAPI, type ShareCards, type ShareSettings } from './services/share-settings'
import { HTTPError } from './services/http'
import { useAdminStore } from './store'
import './share-settings.css'

// Preserve unsaved edits when switching sections in this authenticated browser tab.
let retained: { owner: number; base: ShareSettings; draft: ShareCards } | null = null
const scenes = [
  { key: 'homeTitle', label: '首页', path: '/pages/home/index' },
  { key: 'secondhandTitle', label: '二手市场', path: '/pages/student-secondhand/index' },
  { key: 'servicesTitle', label: '校园服务', path: '/pages/student-takeout/index' }
] as const

export function ShareSettingsConsole() {
  const owner = useAdminStore(s => s.user?.id || 0)
  const dialog = useAdminDialog()
  const cached = retained?.owner === owner && JSON.stringify(cardFields(retained.base)) !== JSON.stringify(retained.draft) ? retained : null
  const [base, setBase] = useState<ShareSettings | null>(cached?.base || null)
  const [draft, setDraft] = useState<ShareCards | null>(cached?.draft || null)
  const [loading, setLoading] = useState(!cached)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [scene, setScene] = useState<(typeof scenes)[number]>(scenes[0])
  const [imageFailed, setImageFailed] = useState(false)
  const alive = useRef(true)
  const changed = Boolean(base && draft && JSON.stringify(cardFields(base)) !== JSON.stringify(draft))
  const imageError = coverError(draft?.defaultImageUrl || '')
  const invalidTitle = draft ? [draft.homeTitle, draft.secondhandTitle, draft.servicesTitle].some(v => !v.trim()) ||
    [draft.homeTitle, draft.secondhandTitle, draft.servicesTitle, draft.detailTitleSuffix].some(v => [...v.trim()].length > 60 || /[\u0000-\u001f\u007f]/.test(v)) : true

  const load = async () => {
    setLoading(true); setError(''); setNotice('')
    try {
      const value = await shareSettingsAPI.read()
      if (!alive.current) return
      setBase(value); setDraft(cardFields(value))
    } catch (reason) { if (alive.current) setError(reason instanceof Error ? reason.message : '读取失败') }
    finally { if (alive.current) setLoading(false) }
  }
  useEffect(() => {
    alive.current = true
    if (!cached) void load()
    return () => { alive.current = false }
  }, [])
  useEffect(() => {
    if (base && draft) retained = { owner, base, draft }
    const warn = (event: BeforeUnloadEvent) => { if (changed) { event.preventDefault(); event.returnValue = '' } }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [base, draft, owner, changed])
  useEffect(() => { setImageFailed(false) }, [draft?.defaultImageUrl])

  const reload = async () => {
    if (changed && !await dialog.confirm({ title: '放弃未保存的修改？', description: '重新读取会用服务器配置替换当前草稿。', confirmLabel: '重新读取' })) return
    await load()
  }
  const save = async () => {
    if (!base || !draft || saving || !changed || invalidTitle || imageError) return
    setSaving(true); setError(''); setNotice('')
    try {
      const value = await shareSettingsAPI.save(cardFields(draft), base.rowVersion)
      retained = { owner, base: value, draft: cardFields(value) }
      if (!alive.current) return
      setBase(value); setDraft(cardFields(value))
      setNotice('已保存。支持分享的新版本小程序进入页面或回到前台后读取新配置；已经发送的卡片不会被修改。')
    } catch (reason) {
      if (alive.current) setError(reason instanceof HTTPError && reason.status === 409
        ? '其他管理员已更新配置。你的草稿仍保留，请复制需要的文案，再重新读取后保存。'
        : reason instanceof Error ? reason.message : '保存失败，请重新读取确认状态')
    } finally { if (alive.current) setSaving(false) }
  }

  if (!base || !draft) return <AdminPanel title='分享卡片设置' actions={<AdminButton onClick={() => void load()} disabled={loading}>重试</AdminButton>}>
    <AdminState kind={error ? 'error' : 'loading'} title={error ? '无法读取分享配置' : '正在读取分享配置'} description={error || '正在连接服务器。'} />
  </AdminPanel>

  const update = <K extends keyof ShareCards>(key: K, value: ShareCards[K]) => { setDraft(current => current ? { ...current, [key]: value } : current); setNotice('') }
  return <AdminPanel title='微信分享卡片' description='维护用户转发给好友或群聊时看到的标题与封面。详情链接由内容自动生成。'
    actions={<><AdminButton disabled={saving || loading} onClick={() => void reload()}>重新读取</AdminButton><AdminButton tone='primary' loading={saving} disabled={loading || !changed || invalidTitle || Boolean(imageError)} onClick={() => void save()}>保存设置</AdminButton></>}>
    {error && <AdminNotice tone='danger'>{error}</AdminNotice>}
    {notice && <AdminNotice tone='success'>{notice}</AdminNotice>}
    <div className='share-settings-status'><strong>{changed ? '有未保存的修改' : '服务器已保存'}</strong><span>版本 {base.rowVersion} · {new Date(base.updatedAt).toLocaleString('zh-CN')}</span></div>
    <div className='share-settings-layout'>
      <form onSubmit={event => { event.preventDefault(); void save() }} className='share-settings-fields'>
        <fieldset disabled={loading || saving}>
          <legend>页面标题</legend>
          {scenes.map(item => <AdminField key={item.key} label={`${item.label}分享标题`} value={draft[item.key]} maxLength={60} required
            error={!draft[item.key].trim() ? '分享标题不能为空' : undefined} onChange={e => update(item.key, e.target.value)} />)}
          <AdminField label='详情标题后缀' description='跟在真实帖子、商品标题或公开主页昵称后；留空则不追加。' value={draft.detailTitleSuffix} maxLength={60} onChange={e => update('detailTitleSuffix', e.target.value)} />
        </fieldset>
        <fieldset disabled={loading || saving}>
          <legend>分享封面</legend>
          <AdminField label='通用封面地址' type='url' value={draft.defaultImageUrl} maxLength={2048} error={imageError || undefined}
            description='使用本站或已配置下载域名的腾讯云 COS 公开 JPG/PNG，建议 5:4。留空使用小程序内置默认封面，无配图内容也能正常展示。'
            onChange={e => update('defaultImageUrl', e.target.value)} />
          <div className='share-settings-cover-actions'><AdminButton tone='ghost' disabled={!draft.defaultImageUrl} onClick={() => update('defaultImageUrl', '')}>恢复内置封面</AdminButton><span>{draft.defaultImageUrl ? '自定义封面 · 保存后生效' : '正在使用内置默认封面'}</span></div>
          <label className='share-settings-check'><input type='checkbox' checked={draft.preferContentImage} onChange={e => update('preferContentImage', e.target.checked)} />
            <span><strong>详情优先使用真实内容图片</strong><small>帖子、商品使用自身图片；无可用图片则使用通用封面。公开主页使用通用封面。</small></span></label>
        </fieldset>
      </form>
      <aside className='share-settings-preview' aria-label='分享卡片预览'>
        <h3>卡片预览</h3>
        <div className='share-settings-tabs'>{scenes.map(item => <button type='button' key={item.key} aria-pressed={scene.key === item.key} onClick={() => setScene(item)}>{item.label}</button>)}</div>
        <article className='share-card-preview'>
          <div className='share-card-preview__brand'>湛科生活圈</div>
          <h4>{draft[scene.key] || '请填写分享标题'}</h4>
          {draft.defaultImageUrl && !imageError && !imageFailed ? <img src={draft.defaultImageUrl.trim()} alt='当前分享封面' referrerPolicy='no-referrer' onError={() => setImageFailed(true)} />
            : <img src='/images/default-share.png' alt='湛科生活圈内置默认分享封面' />}
          {(imageFailed || imageError) && <p className='share-card-preview__warning'>{imageFailed ? '自定义封面加载失败。当前仅预览内置图，请修正地址或恢复内置封面后保存。' : '封面地址不可用，请修正或恢复内置封面。'}</p>}
          <footer>小程序</footer>
        </article>
        <p className='share-settings-caption'>预览仅展示标题与封面关系，最终样式以微信为准。</p>
        <AdminNotice>未审核内容、关闭的模块和私信页面不开放分享。标题或封面设置不会修改内容访问权限。</AdminNotice>
      </aside>
    </div>
  </AdminPanel>
}
