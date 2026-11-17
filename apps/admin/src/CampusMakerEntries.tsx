import { useCallback, useEffect, useState } from 'react'
import { AdminAPIError, adminAPI } from './services/f-admin'
import { navigationItemError } from './services/navigation-validation'
import {
  AdminButton,
  AdminField,
  AdminNotice,
  AdminPanel,
  AdminRowActions,
  AdminSelect,
  AdminState,
  AdminStatusBadge,
  AdminTableFrame,
  useAdminDialog,
} from './components'
import type { NavigationCategory, NavigationItem, NavigationItemInput } from './types'

const CAMPUS_MAKER_SYSTEM_KEY = 'campus_maker'

type Draft = NavigationItemInput & { id?: number }

function adminErrorMessage(error: unknown): string {
  if (error instanceof AdminAPIError) {
    if (error.status === 0) return '网络连接失败，请检查服务状态后重试'
    if (error.status === 401) return '登录已失效，请重新登录'
    if (error.status === 403) return '当前账号没有管理权限'
    if (error.status === 409) return error.message || '众创入口配置冲突，请刷新后重试'
    if (error.status >= 500) return '服务暂时不可用，请稍后重试'
    return error.message
  }
  return error instanceof Error ? error.message : '操作失败，请重试'
}

function toInput(item: NavigationItem): NavigationItemInput {
  return {
    categoryId: item.categoryId,
    title: item.title,
    subtitle: item.subtitle,
    iconText: item.iconText,
    appId: item.appId,
    path: item.path,
    color: item.color,
    sort: item.sort,
    enabled: item.enabled,
  }
}

function emptyDraft(categoryId: number): Draft {
  return { categoryId, title: '', subtitle: '', iconText: '', appId: '', path: '', color: '#1677FF', sort: 0, enabled: true }
}

export function CampusMakerEntries() {
  const dialog = useAdminDialog()
  const [category, setCategory] = useState<NavigationCategory | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [draft, setDraft] = useState<Draft | null>(null)
  const [busy, setBusy] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const categories = await adminAPI.navigation()
      setCategory(categories.find((value) => value.systemKey === CAMPUS_MAKER_SYSTEM_KEY) ?? null)
      setError('')
    } catch (reason) {
      setError(adminErrorMessage(reason))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const mutate = async (key: string, operation: () => Promise<unknown>, success: string): Promise<boolean> => {
    setBusy(key)
    setError('')
    setNotice('')
    try {
      await operation()
      await load()
      setNotice(success)
      return true
    } catch (reason) {
      setError(adminErrorMessage(reason))
      return false
    } finally {
      setBusy('')
    }
  }

  const updateDraft = <K extends keyof Draft>(key: K, value: Draft[K]) => {
    setDraft((current) => current ? { ...current, [key]: value } : current)
  }

  const saveItem = async () => {
    if (!draft) return
    const validationError = navigationItemError(draft)
    if (validationError) {
      setError(validationError)
      return
    }
    const normalized: NavigationItemInput = {
      ...draft,
      title: draft.title.trim(),
      subtitle: draft.subtitle.trim(),
      iconText: draft.iconText.trim(),
      appId: draft.appId.trim(),
      path: draft.path.trim().replace(/^\//, ''),
      color: draft.color.trim(),
    }
    const succeeded = await mutate(
      draft.id ? `item-${draft.id}` : 'item-new',
      () => draft.id ? adminAPI.updateNavigationItem(draft.id, normalized) : adminAPI.createNavigationItem(normalized),
      draft.id ? `入口“${normalized.title}”已更新。` : `入口“${normalized.title}”已创建。`,
    )
    if (succeeded) setDraft(null)
  }

  const toggleItem = (item: NavigationItem, enabled: boolean) => {
    void mutate(
      `item-${item.id}`,
      () => adminAPI.updateNavigationItem(item.id, { ...toInput(item), enabled }),
      enabled ? `入口“${item.title}”已启用。` : `入口“${item.title}”已停用。`,
    )
  }

  const deleteItem = async (item: NavigationItem) => {
    if (!await dialog.confirm({ title: '删除众创入口', description: `确认删除“${item.title}”吗？删除后学生端「众创」频道将不再展示该入口。`, confirmLabel: '确认删除', tone: 'danger' })) return
    await mutate(`item-${item.id}`, () => adminAPI.deleteNavigationItem(item.id), `入口“${item.title}”已删除。`)
  }

  return (
    <AdminPanel
      title="众创入口"
      description="维护小程序首页「众创」频道的外接小程序入口，保存后小程序刷新即可读取。"
      actions={<>
        <AdminButton tone="ghost" loading={loading} onClick={() => void load()}>刷新</AdminButton>
        <AdminButton tone="primary" disabled={!category || Boolean(busy)} onClick={() => category && setDraft(emptyDraft(category.id))}>新增入口</AdminButton>
      </>}
    >
      <AdminNotice>众创只配置小程序跳转；接入前请向服务方获取正式 AppID 和页面路径，并用手机实际点开验证。</AdminNotice>
      {error && <AdminNotice tone="danger" title="众创操作失败">{error}</AdminNotice>}
      {notice && <AdminNotice tone="success">{notice}</AdminNotice>}
      {draft && <form className="navigation-item-form" onSubmit={(event) => { event.preventDefault(); void saveItem() }}>
        <div className="navigation-item-form__title"><div><strong>{draft.id ? '编辑众创入口' : '新增众创入口'}</strong><span>只填写真实可用的小程序跳转信息</span></div><AdminButton tone="text" onClick={() => setDraft(null)}>取消</AdminButton></div>
        <div className="navigation-item-form__grid">
          <AdminField label="入口名称" required value={draft.title} onChange={(event) => updateDraft('title', event.target.value)} />
          <AdminField label="小程序 AppID" required value={draft.appId} placeholder="wx1234567890ABCDEF" onChange={(event) => updateDraft('appId', event.target.value)} />
          <AdminField label="页面路径" required value={draft.path} placeholder="例如 pages/index/index" onChange={(event) => updateDraft('path', event.target.value)} />
        </div>
        <div className="navigation-item-form__grid">
          <AdminField label="用途说明" value={draft.subtitle} onChange={(event) => updateDraft('subtitle', event.target.value)} />
          <AdminField label="文字图标" required value={draft.iconText} placeholder="1–2 个字" onChange={(event) => updateDraft('iconText', event.target.value)} />
          <AdminField label="主题颜色" value={draft.color} placeholder="#1677FF" onChange={(event) => updateDraft('color', event.target.value)} />
          <AdminField label="排序" type="number" value={draft.sort} onChange={(event) => updateDraft('sort', Number(event.target.value))} />
          <label className="admin-check"><input type="checkbox" checked={draft.enabled} onChange={(event) => updateDraft('enabled', event.target.checked)} /><span>启用入口</span></label>
        </div>
        <div className="navigation-item-form__footer"><AdminButton tone="primary" type="submit" loading={busy === 'item-new' || busy === `item-${draft.id}`}>保存入口</AdminButton></div>
      </form>}
      {!loading && !category ? <AdminState kind="empty" title="尚未创建众创分类" description="请在服务器执行迁移 064_campus_maker_category.sql 后刷新。" /> : !category ? null :
        <AdminTableFrame label="众创频道入口">
          <table className="admin-data-table">
            <thead><tr><th>名称</th><th>小程序 AppID</th><th>页面路径</th><th>排序</th><th>状态</th><th className="admin-table-actions">操作</th></tr></thead>
            <tbody>
              {category.items.map((item) => <tr key={item.id}>
                <td><strong>{item.title}</strong><span>{item.subtitle || `#${item.id}`}</span></td>
                <td><code>{item.appId}</code></td>
                <td className="admin-cell-wrap"><code>{item.path}</code></td>
                <td>{item.sort}</td>
                <td><AdminStatusBadge value={item.enabled ? 'enabled' : 'disabled'} /></td>
                <td className="admin-table-actions"><AdminRowActions>
                  <AdminButton tone="text" disabled={Boolean(busy)} onClick={() => setDraft({ ...toInput(item), id: item.id })}>编辑</AdminButton>
                  <AdminButton tone="text" disabled={Boolean(busy)} onClick={() => toggleItem(item, !item.enabled)}>{item.enabled ? '停用' : '启用'}</AdminButton>
                  <AdminButton tone="danger" disabled={Boolean(busy)} onClick={() => void deleteItem(item)}>删除</AdminButton>
                </AdminRowActions></td>
              </tr>)}
              {category.items.length === 0 && <tr><td colSpan={6}><div className="admin-inline-empty">还没有众创入口，点击右上角「新增入口」添加。</div></td></tr>}
            </tbody>
          </table>
        </AdminTableFrame>}
    </AdminPanel>
  )
}
