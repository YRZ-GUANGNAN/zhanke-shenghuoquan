import { useEffect, useRef, useState } from 'react'
import { AdminButton, AdminField, AdminNotice, AdminPanel, AdminState, useAdminDialog } from './components'
import { cardFields, coverError, shareSettingsAPI, type ShareCards, type ShareSettings } from './services/share-settings'
import { HTTPError } from './services/http'
import { useAdminStore } from './store'
import './share-settings.css'

// Preserve unsaved edits when switching sections in this authenticated browser tab.
type CoverMode = 'builtin' | 'custom'
type EditorDraft = { owner: number; base: ShareSettings; draft: ShareCards; coverMode: CoverMode; customCoverDraft: string }
let retained: EditorDraft | null = null
const sourceOf = (value: string): CoverMode => value.trim() ? 'custom' : 'builtin'
function hasDraft(value: EditorDraft): boolean {
  return JSON.stringify(cardFields(value.base)) !== JSON.stringify(value.draft) ||
    value.coverMode !== sourceOf(value.base.defaultImageUrl) || value.customCoverDraft !== value.base.defaultImageUrl
}
const scenes = [
  { key: 'homeTitle', label: '首页', path: '/pages/home/index' },
  { key: 'secondhandTitle', label: '二手市场', path: '/pages/student-secondhand/index' },
  { key: 'servicesTitle', label: '校园服务', path: '/pages/student-takeout/index' }
] as const

export function ShareSettingsConsole() {
  const owner = useAdminStore(s => s.user?.id || 0)
  return owner ? <ShareSettingsEditor key={owner} owner={owner} /> : null
}

function ShareSettingsEditor({ owner }: { owner: number }) {
  const dialog = useAdminDialog()
  const [cached] = useState(() => retained?.owner === owner && hasDraft(retained) ? retained : null)
  const [base, setBase] = useState<ShareSettings | null>(cached?.base || null)
  const [draft, setDraft] = useState<ShareCards | null>(cached?.draft || null)
  const [coverMode, setCoverMode] = useState<CoverMode>(cached?.coverMode ?? 'builtin')
  const [customCoverDraft, setCustomCoverDraft] = useState(cached?.customCoverDraft ?? '')
  const [loading, setLoading] = useState(!cached)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [scene, setScene] = useState<(typeof scenes)[number]>(scenes[0])
  const [preview, setPreview] = useState<{ key: string; state: 'ready' | 'failed' } | null>(null)
  const [previewAttempt, setPreviewAttempt] = useState(0)
  const alive = useRef(false)
  const operation = useRef<'read' | 'save' | 'confirm' | null>(null)
  const generation = useRef(0)
  const changed = Boolean(base && draft && JSON.stringify(cardFields(base)) !== JSON.stringify(draft))
  const dirty = Boolean(base && draft && hasDraft({ owner, base, draft, coverMode, customCoverDraft }))
  const imageError = coverError(draft?.defaultImageUrl || '')
  const coverValidationError = coverMode === 'custom' && !draft?.defaultImageUrl.trim()
    ? '自定义封面需要填写图片地址，或切换回内置默认封面。'
    : imageError
  const previewUrl = coverMode === 'custom' && !coverValidationError ? draft?.defaultImageUrl.trim() || '' : ''
  const previewKey = `${previewAttempt}:${previewUrl}`
  const currentPreview = useRef(previewKey)
  currentPreview.current = previewKey
  const previewStatus = previewUrl ? preview?.key === previewKey ? preview.state : 'loading' : 'builtin'
  const imageFailed = previewStatus === 'failed'
  const coverReady = coverMode === 'builtin' || previewStatus === 'ready'
  const invalidTitle = draft ? [draft.homeTitle, draft.secondhandTitle, draft.servicesTitle].some(v => !v.trim()) ||
    [draft.homeTitle, draft.secondhandTitle, draft.servicesTitle, draft.detailTitleSuffix].some(v => [...v.trim()].length > 60 || /[\u0000-\u001f\u007f]/.test(v)) : true

  const currentRequest = (version: number) => alive.current && generation.current === version
  const accept = (value: ShareSettings) => {
    setBase(value); setDraft(cardFields(value))
    setCoverMode(sourceOf(value.defaultImageUrl))
    setCustomCoverDraft(value.defaultImageUrl)
  }
  const load = async () => {
    if (!alive.current || operation.current) return
    operation.current = 'read'
    const version = ++generation.current
    setLoading(true); setError(''); setNotice('')
    try {
      const value = await shareSettingsAPI.read()
      if (!currentRequest(version)) return
      accept(value)
      setPreviewAttempt(attempt => attempt + 1)
    } catch (reason) { if (currentRequest(version)) setError(reason instanceof Error ? reason.message : '读取失败') }
    finally { if (currentRequest(version)) { operation.current = null; setLoading(false) } }
  }
  useEffect(() => {
    alive.current = true
    if (!cached) void load()
    return () => { alive.current = false; generation.current += 1; operation.current = null }
  }, [])
  useEffect(() => {
    if (base && draft) retained = { owner, base, draft, coverMode, customCoverDraft }
    const warn = (event: BeforeUnloadEvent) => { if (dirty) { event.preventDefault(); event.returnValue = '' } }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [base, draft, owner, dirty, coverMode, customCoverDraft])

  const reload = async () => {
    if (!alive.current || operation.current) return
    operation.current = 'confirm'
    const version = ++generation.current
    setLoading(true)
    try {
      const confirmed = !dirty || await dialog.confirm({ title: '放弃未保存的修改？', description: '重新读取会用服务器配置替换当前草稿，包括尚未使用的图片地址。', confirmLabel: '重新读取' })
      if (!currentRequest(version)) return
      operation.current = null
      setLoading(false)
      if (confirmed) await load()
    } catch (reason) {
      if (currentRequest(version)) { operation.current = null; setLoading(false); setError(reason instanceof Error ? reason.message : '重新读取失败') }
    }
  }
  const save = async () => {
    if (!alive.current || operation.current || !base || !draft || !changed || invalidTitle || coverValidationError || !coverReady) return
    operation.current = 'save'
    const version = ++generation.current
    setSaving(true); setError(''); setNotice('')
    try {
      const value = await shareSettingsAPI.save({ ...cardFields(draft), defaultImageUrl: draft.defaultImageUrl.trim() }, base.rowVersion)
      if (!currentRequest(version)) return
      retained = { owner, base: value, draft: cardFields(value), coverMode: sourceOf(value.defaultImageUrl), customCoverDraft: value.defaultImageUrl }
      accept(value)
      setNotice('已保存。支持分享的新版本小程序进入页面或回到前台后读取新配置；已经发送的卡片不会被修改。')
    } catch (reason) {
      if (currentRequest(version)) setError(reason instanceof HTTPError && reason.status === 409
        ? '其他管理员已更新配置。你的草稿仍保留，请复制需要的文案，再重新读取后保存。'
        : reason instanceof Error ? reason.message : '保存失败，请重新读取确认状态')
    } finally { if (currentRequest(version)) { operation.current = null; setSaving(false) } }
  }

  if (!base || !draft) return <AdminPanel title='分享卡片设置' actions={<AdminButton onClick={() => void load()} disabled={loading}>重试</AdminButton>}>
    <AdminState kind={error ? 'error' : 'loading'} title={error ? '无法读取分享配置' : '正在读取分享配置'} description={error || '正在连接服务器。'} />
  </AdminPanel>

  const update = <K extends keyof ShareCards>(key: K, value: ShareCards[K]) => { setDraft(current => current ? { ...current, [key]: value } : current); setNotice('') }
  const switchCoverMode = (next: CoverMode) => {
    setPreviewAttempt(attempt => attempt + 1)
    setCoverMode(next)
    if (next === 'builtin') update('defaultImageUrl', '')
    else update('defaultImageUrl', customCoverDraft)
  }
  const updateCover = (value: string) => {
    setPreviewAttempt(attempt => attempt + 1)
    setCustomCoverDraft(value)
    update('defaultImageUrl', value)
  }
  return <AdminPanel title='微信分享卡片' description='维护用户转发给好友或群聊时看到的标题与封面。详情链接由内容自动生成。'
    actions={<><AdminButton disabled={saving || loading} onClick={() => void reload()}>重新读取</AdminButton><AdminButton tone='primary' loading={saving} disabled={loading || !changed || invalidTitle || Boolean(coverValidationError) || !coverReady} onClick={() => void save()}>保存设置</AdminButton></>}>
    {error && <AdminNotice tone='danger'>{error}</AdminNotice>}
    {notice && <AdminNotice tone='success'>{notice}</AdminNotice>}
    <div className='share-settings-status'><strong>{changed ? '有未保存的修改' : dirty ? '封面草稿已暂存，当前配置未变' : '服务器已保存'}</strong><span>版本 {base.rowVersion} · {new Date(base.updatedAt).toLocaleString('zh-CN')}</span></div>
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
          <div className='share-cover-source' role='radiogroup' aria-label='分享封面来源'>
            <label className={`share-cover-source__option${coverMode === 'builtin' ? ' share-cover-source__option--selected' : ''}`}>
              <input type='radio' name='share-cover-source' value='builtin' checked={coverMode === 'builtin'} onChange={() => switchCoverMode('builtin')} />
              <span><strong>内置默认封面</strong><small>使用小程序包内的品牌封面，无需图片地址。</small></span>
            </label>
            <label className={`share-cover-source__option${coverMode === 'custom' ? ' share-cover-source__option--selected' : ''}`}>
              <input type='radio' name='share-cover-source' value='custom' checked={coverMode === 'custom'} onChange={() => switchCoverMode('custom')} />
              <span><strong>自定义图片</strong><small>填写公开的本站或腾讯云 COS JPG/PNG 地址。</small></span>
            </label>
          </div>
          <AdminField label='通用封面地址' type='url' value={draft.defaultImageUrl} maxLength={2048} disabled={coverMode !== 'custom'} error={coverValidationError || undefined}
            description='使用本站或已配置下载域名的腾讯云 COS 公开 JPG/PNG，建议 5:4。预览加载成功后可保存；无需自定义图片时，选择内置默认封面。'
            onChange={e => updateCover(e.target.value)} />
          <div className='share-settings-cover-actions'><AdminButton tone='ghost' disabled={coverMode === 'builtin'} onClick={() => switchCoverMode('builtin')}>恢复内置封面</AdminButton><span>{coverMode === 'custom' ? '自定义封面 · 保存后生效' : '正在使用内置默认封面'}</span></div>
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
          {previewUrl && !imageFailed ? <img key={previewKey} src={previewUrl} alt='当前分享封面' referrerPolicy='no-referrer'
            onLoad={() => { if (alive.current && currentPreview.current === previewKey) setPreview({ key: previewKey, state: 'ready' }) }}
            onError={() => { if (alive.current && currentPreview.current === previewKey) setPreview({ key: previewKey, state: 'failed' }) }} />
            : <img src='/images/default-share.png' alt='湛科生活圈内置默认分享封面' />}
          <div className='share-card-preview__feedback' aria-live='polite'>
            {previewStatus === 'loading' && <p>正在加载自定义封面，成功后可保存。</p>}
            {coverValidationError && <p className='share-card-preview__warning'>{coverValidationError}</p>}
            {imageFailed && <><p className='share-card-preview__warning'>自定义封面加载失败，暂时显示内置图。请重试、更换地址或选择内置默认封面。</p>
              <AdminButton tone='ghost' onClick={() => setPreviewAttempt(attempt => attempt + 1)}>重试预览</AdminButton></>}
          </div>
          <footer>小程序</footer>
        </article>
        <p className='share-settings-caption'>预览仅展示标题与封面关系，最终样式以微信为准。</p>
        <AdminNotice>未审核内容、关闭的模块和私信页面不开放分享。标题或封面设置不会修改内容访问权限。</AdminNotice>
      </aside>
    </div>
  </AdminPanel>
}
