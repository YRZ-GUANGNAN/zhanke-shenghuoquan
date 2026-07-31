import { useCallback, useEffect, useState } from 'react'
import { AdminAPIError, adminAPI } from './services/f-admin'
import { useAdminDialog } from './components'
import type {
  ExternalTakeoutMerchant,
  ExternalTakeoutMerchantDirectory,
  ExternalTakeoutMerchantInput,
} from './types'

const emptyDraft: ExternalTakeoutMerchantInput = {
  title: '',
  subtitle: '',
  iconText: '',
  appId: '',
  path: '',
  color: '#1677FF',
  sort: 0,
}

function adminErrorMessage(error: unknown): string {
  if (error instanceof AdminAPIError) {
    if (error.status === 0) return '网络连接失败，请检查服务状态后重试'
    if (error.status === 401) return '登录已失效，请重新登录'
    if (error.status === 403) return '当前账号没有管理权限'
    if (error.status === 409) return error.message || '商家配置冲突，请刷新后重试'
    if (error.status >= 500) return '服务暂时不可用，请稍后重试'
    return error.message
  }
  return error instanceof Error ? error.message : '操作失败，请重试'
}

function validateDraft(draft: ExternalTakeoutMerchantInput): string {
  const title = draft.title.trim()
  const subtitle = draft.subtitle.trim()
  const iconText = draft.iconText.trim()
  const appId = draft.appId.trim()
  const path = draft.path.trim().replace(/^\//, '')
  if (!title || Array.from(title).length > 64) return '商家名称必填，最多 64 个字'
  if (Array.from(subtitle).length > 120) return '简介最多 120 个字'
  if (!iconText || Array.from(iconText).length > 12) return '文字图标必填，最多 12 个字'
  if (!/^wx[A-Za-z0-9]{16}$/.test(appId)) return 'AppID 必须是 wx 开头加 16 位字母或数字'
  if (!path || path.length > 240 || /(?:^|[/?])\.\.(?:[/?]|$)|\\|\/\//.test(path) || /^[A-Za-z][A-Za-z0-9+.-]*:/.test(path)) {
    return '页面路径无效，请填写小程序内部路径'
  }
  if (!/^#[0-9A-Fa-f]{6}$/.test(draft.color.trim())) return '颜色必须是 #RRGGBB 格式'
  if (!Number.isInteger(draft.sort) || draft.sort < 0 || draft.sort > 10000) return '排序必须是 0 至 10000 的整数'
  return ''
}

function toDraft(item: ExternalTakeoutMerchant): ExternalTakeoutMerchantInput {
  return {
    title: item.title,
    subtitle: item.subtitle,
    iconText: item.iconText,
    appId: item.appId,
    path: item.path,
    color: item.color,
    sort: item.sort,
  }
}

export function ExternalTakeoutMerchants() {
  const dialog = useAdminDialog()
  const [directory, setDirectory] = useState<ExternalTakeoutMerchantDirectory | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [editingId, setEditingId] = useState<number | null>(null)
  const [draft, setDraft] = useState<ExternalTakeoutMerchantInput>(emptyDraft)
  const [saving, setSaving] = useState(false)
  const [busyAction, setBusyAction] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const value = await adminAPI.externalTakeoutMerchants()
      setDirectory(value)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load().catch((loadError) => setError(adminErrorMessage(loadError)))
  }, [load])

  const mutate = async (key: string, operation: () => Promise<unknown>): Promise<boolean> => {
    setBusyAction(key)
    setError('')
    try {
      await operation()
      await load()
      return true
    } catch (mutationError) {
      setError(adminErrorMessage(mutationError))
      return false
    } finally {
      setBusyAction('')
    }
  }

  const updateDraft = <K extends keyof ExternalTakeoutMerchantInput>(key: K, value: ExternalTakeoutMerchantInput[K]) => {
    setDraft((current) => ({ ...current, [key]: value }))
  }

  const save = async () => {
    const validationError = validateDraft(draft)
    if (validationError) {
      setError(validationError)
      return
    }
    const normalized: ExternalTakeoutMerchantInput = {
      ...draft,
      title: draft.title.trim(),
      subtitle: draft.subtitle.trim(),
      iconText: draft.iconText.trim(),
      appId: draft.appId.trim(),
      path: draft.path.trim().replace(/^\//, ''),
      color: draft.color.trim(),
    }
    setSaving(true)
    try {
      const succeeded = await mutate(
        editingId === null ? 'create' : `save-${editingId}`,
        () => editingId === null
          ? adminAPI.createExternalTakeoutMerchant(normalized)
          : adminAPI.updateExternalTakeoutMerchant(editingId, normalized),
      )
      if (succeeded) {
        setDraft(emptyDraft)
        setEditingId(null)
      }
    } finally {
      setSaving(false)
    }
  }

  const remove = async (item: ExternalTakeoutMerchant) => {
    if (!await dialog.confirm({ title: '删除外部商家', description: `确认删除“${item.title}”吗？删除后学生端将不再展示该入口。`, confirmLabel: '确认删除', tone: 'danger' })) return
    await mutate(`delete-${item.id}`, () => adminAPI.deleteExternalTakeoutMerchant(item.id))
  }

  return (
    <section className='external-merchant-panel'>
      <header className='external-merchant-header'>
        <div>
          <span className='external-merchant-kicker'>学生端外卖入口</span>
          <h2>外部商家小程序目录</h2>
          <p>这里只管理展示和跳转，不接收商品、订单或支付数据。</p>
        </div>
        <button
          type='button'
          disabled={Boolean(busyAction) || !directory}
          onClick={() => directory && void mutate(
            'category-visibility',
            () => adminAPI.setExternalTakeoutCategoryVisibility(!directory.categoryEnabled),
          )}
        >
          {directory?.categoryEnabled ? '关闭学生端目录' : '开启学生端目录'}
        </button>
      </header>

      {error && <div className='external-merchant-error' role='alert'>{error}</div>}

      <div className='external-merchant-form'>
        <label>商家名称<input value={draft.title} maxLength={64} onChange={(event) => updateDraft('title', event.target.value)} /></label>
        <label>商家简介<input value={draft.subtitle} maxLength={120} onChange={(event) => updateDraft('subtitle', event.target.value)} /></label>
        <label>文字图标<input value={draft.iconText} maxLength={12} onChange={(event) => updateDraft('iconText', event.target.value)} /></label>
        <label>AppID<input value={draft.appId} placeholder='wx1234567890ABCDEF' onChange={(event) => updateDraft('appId', event.target.value)} /></label>
        <label className='external-merchant-form__wide'>页面路径<input value={draft.path} placeholder='pages/index/index' onChange={(event) => updateDraft('path', event.target.value)} /></label>
        <label>主题颜色<input value={draft.color} placeholder='#1677FF' onChange={(event) => updateDraft('color', event.target.value)} /></label>
        <label>排序<input type='number' min={0} max={10000} value={draft.sort} onChange={(event) => updateDraft('sort', Number(event.target.value))} /></label>
        <div className='external-merchant-form__actions'>
          <button type='button' disabled={saving || Boolean(busyAction)} onClick={() => void save()}>
            {saving ? '保存中…' : editingId === null ? '新增商家' : '保存修改'}
          </button>
          {editingId !== null && (
            <button type='button' className='external-merchant-secondary' disabled={saving} onClick={() => { setDraft(emptyDraft); setEditingId(null); setError('') }}>
              取消编辑
            </button>
          )}
        </div>
      </div>

      {loading && !directory ? (
        <div className='external-merchant-empty'>正在读取商家目录…</div>
      ) : (
        <div className='table-scroll'>
          <table className='external-merchant-table'>
            <thead><tr><th>商家</th><th>跳转目标</th><th>排序</th><th>展示</th><th>营业</th><th>操作</th></tr></thead>
            <tbody>
              {(directory?.items ?? []).map((item) => (
                <tr key={item.id}>
                  <td>
                    <div className='external-merchant-identity'>
                      <span className='external-merchant-avatar' style={{ backgroundColor: item.color }}>{item.iconText}</span>
                      <span><strong>{item.title}</strong><small>{item.subtitle || '暂无简介'}</small></span>
                    </div>
                  </td>
                  <td><code>{item.appId}</code><small>{item.path}</small></td>
                  <td>{item.sort}</td>
                  <td><span className={`external-merchant-badge external-merchant-badge--${item.enabled ? 'open' : 'off'}`}>{item.enabled ? '显示' : '隐藏'}</span></td>
                  <td><span className={`external-merchant-badge external-merchant-badge--${item.operatingStatus}`}>{item.operatingStatus === 'open' ? '营业中' : '休息中'}</span></td>
                  <td>
                    <div className='external-merchant-row-actions'>
                      <button type='button' disabled={Boolean(busyAction)} onClick={() => { setEditingId(item.id); setDraft(toDraft(item)); setError('') }}>编辑</button>
                      <button type='button' disabled={Boolean(busyAction)} onClick={() => void mutate(`status-${item.id}`, () => adminAPI.setExternalTakeoutOperatingStatus(item.id, item.operatingStatus === 'open' ? 'resting' : 'open'))}>{item.operatingStatus === 'open' ? '暂停营业' : '开始营业'}</button>
                      <button type='button' disabled={Boolean(busyAction)} onClick={() => void mutate(`visibility-${item.id}`, () => adminAPI.setExternalTakeoutVisibility(item.id, !item.enabled))}>{item.enabled ? '隐藏' : '恢复显示'}</button>
                      <button type='button' className='external-merchant-danger' disabled={Boolean(busyAction)} onClick={() => void remove(item)}>删除</button>
                    </div>
                  </td>
                </tr>
              ))}
              {(directory?.items.length ?? 0) === 0 && <tr><td colSpan={6}><div className='external-merchant-empty'>暂无商家，请先新增一个入口。</div></td></tr>}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}
