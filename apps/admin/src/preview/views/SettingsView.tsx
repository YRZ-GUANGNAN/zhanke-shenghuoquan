import { useCallback, useEffect, useRef, useState } from 'react'
import { FEATURE_GROUPS, FEATURE_KEYS, type FeatureKey, type PreviewSection } from '../data'
import {
  Button,
  EmptyState,
  Field,
  Modal,
  PageHeader,
  Panel,
  Segmented,
  SelectField,
  StatCard,
  StatusPill,
  Toggle,
  usePreviewToast
} from '../ui'
import { AdminAPIError, adminAPI } from '../../services/f-admin'
import { HTTPError } from '../../services/http'
import { navigationItemError } from '../../services/navigation-validation'
import { cardFields, coverError, shareSettingsAPI, type ShareCards, type ShareSettings } from '../../services/share-settings'
import { useAdminStore } from '../../store'
import type {
  ExternalTakeoutMerchant,
  ExternalTakeoutMerchantDirectory,
  ExternalTakeoutMerchantInput,
  FeaturePolicy,
  FeaturePolicyKey,
  NavigationCategory,
  NavigationItem,
  NavigationItemInput,
  PresentationMode
} from '../../types'

type SettingsSection = Extract<PreviewSection, 'navigation' | 'share-settings' | 'feature-policy'>

const CONFLICT_MESSAGE = '数据已被他人修改，请刷新后重试'

// 统一错误口径：409 一律提示刷新重试，其余错误原样展示在页面内。
function requestError(reason: unknown): { conflict: boolean; message: string } {
  if (reason instanceof AdminAPIError || reason instanceof HTTPError) {
    if (reason.status === 409) return { conflict: true, message: CONFLICT_MESSAGE }
    if (reason.status === 0) return { conflict: false, message: '网络连接失败，请稍后重试' }
    if (reason.status === 401) return { conflict: false, message: '登录已失效，请重新登录' }
    if (reason.status === 403) return { conflict: false, message: '当前账号没有权限执行该操作' }
    return { conflict: false, message: reason.message || '操作失败，请稍后重试' }
  }
  return { conflict: false, message: reason instanceof Error ? reason.message : '操作失败，请稍后重试' }
}

const formatDateTime = (value: string) => {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString('zh-CN')
}

export function SettingsView({ section }: { section: SettingsSection }) {
  if (section === 'navigation') return <NavigationSettings />
  if (section === 'share-settings') return <ShareSettingsView />
  return <FeatureSettings />
}

interface ConfirmRequest {
  title: string
  description: string
  confirmLabel: string
  action: () => Promise<void>
}

function ConfirmModal({
  request,
  busy,
  error,
  onCancel,
  onConfirm
}: {
  request: ConfirmRequest | null
  busy: boolean
  error: string
  onCancel: () => void
  onConfirm: () => void
}) {
  return <Modal
    open={Boolean(request)}
    title={request?.title || ''}
    description={request?.description}
    width="sm"
    onClose={() => { if (!busy) onCancel() }}
    footer={<>
      <Button disabled={busy} onClick={onCancel}>取消</Button>
      <Button variant="danger" disabled={busy} onClick={onConfirm}>{busy ? '正在提交…' : request?.confirmLabel || '确认'}</Button>
    </>}
  >
    {error && <div className="preview-form-error">{error}</div>}
  </Modal>
}

interface CategoryDraft {
  id: number | null
  name: string
  sort: string
  enabled: boolean
}

interface ItemDraft {
  id: number | null
  categoryId: number
  title: string
  subtitle: string
  iconText: string
  appId: string
  path: string
  color: string
  sort: string
  enabled: boolean
}

interface MerchantDraft {
  id: number | null
  title: string
  subtitle: string
  iconText: string
  appId: string
  path: string
  color: string
  sort: string
}

const externalMerchantError = (input: ExternalTakeoutMerchantInput): string => {
  const title = input.title.trim()
  const subtitle = input.subtitle.trim()
  const iconText = input.iconText.trim()
  const appId = input.appId.trim()
  const path = input.path.trim().replace(/^\//, '')
  if (!title || [...title].length > 64) return '商家名称必填，最多 64 个字'
  if ([...subtitle].length > 120) return '简介最多 120 个字'
  if (!iconText || [...iconText].length > 12) return '文字图标必填，最多 12 个字'
  if (!/^wx[A-Za-z0-9]{16}$/.test(appId)) return 'AppID 必须是 wx 开头加 16 位字母或数字'
  if (!path || path.length > 240 || /(?:^|[/?])\.\.(?:[/?]|$)|\\|\/\//.test(path) || /^[A-Za-z][A-Za-z0-9+.-]*:/.test(path)) return '页面路径无效，请填写小程序内部路径'
  if (!/^#[0-9A-Fa-f]{6}$/.test(input.color.trim())) return '颜色必须是 #RRGGBB 格式'
  if (!Number.isInteger(input.sort) || input.sort < 0 || input.sort > 10000) return '排序必须是 0 至 10000 的整数'
  return ''
}

function NavigationSettings() {
  const { notify } = usePreviewToast()
  const [categories, setCategories] = useState<NavigationCategory[] | null>(null)
  const [directory, setDirectory] = useState<ExternalTakeoutMerchantDirectory | null>(null)
  const [navError, setNavError] = useState('')
  const [externalError, setExternalError] = useState('')
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState('')
  const [categoryDraft, setCategoryDraft] = useState<CategoryDraft | null>(null)
  const [itemDraft, setItemDraft] = useState<ItemDraft | null>(null)
  const [merchantDraft, setMerchantDraft] = useState<MerchantDraft | null>(null)
  const [formError, setFormError] = useState('')
  const [formSaving, setFormSaving] = useState(false)
  const [confirm, setConfirm] = useState<ConfirmRequest | null>(null)
  const [confirmError, setConfirmError] = useState('')
  const [confirmSaving, setConfirmSaving] = useState(false)

  const loadNavigation = useCallback(async () => {
    try {
      setCategories(await adminAPI.navigation())
      setNavError('')
    } catch (reason) {
      setNavError(requestError(reason).message)
    }
  }, [])

  const loadExternal = useCallback(async () => {
    try {
      setDirectory(await adminAPI.externalTakeoutMerchants())
      setExternalError('')
    } catch (reason) {
      setExternalError(requestError(reason).message)
    }
  }, [])

  useEffect(() => {
    let active = true
    setLoading(true)
    void Promise.all([loadNavigation(), loadExternal()]).finally(() => {
      if (active) setLoading(false)
    })
    return () => { active = false }
  }, [loadNavigation, loadExternal])

  const generalCategories = (categories || []).filter(category => !category.systemKey)
  const generalItems = generalCategories.flatMap(category => category.items || [])
  const configIssues = [
    ...generalItems.filter(item => !/^wx[0-9A-Za-z]{16}$/.test(item.appId)).map(item => `「${item.title}」AppID 格式不正确`),
    ...generalItems.filter(item => !item.path.trim()).map(item => `「${item.title}」缺少页面路径`),
    ...generalCategories.flatMap(category => {
      const counters = new Map<number, number>()
      ;(category.items || []).forEach(item => counters.set(item.sort, (counters.get(item.sort) || 0) + 1))
      return [...counters.entries()].filter(([, count]) => count > 1).map(([sort]) => `「${category.name}」排序值 ${sort} 重复`)
    })
  ]

  const perform = async (
    key: string,
    operation: () => Promise<unknown>,
    success: string,
    reload: () => Promise<void>,
    report: (message: string) => void
  ) => {
    if (busy) return
    setBusy(key)
    report('')
    try {
      await operation()
      await reload()
      notify(success)
    } catch (reason) {
      const failure = requestError(reason)
      if (failure.conflict) {
        notify(failure.message, 'danger')
        await Promise.all([loadNavigation(), loadExternal()])
      } else {
        report(failure.message)
      }
    } finally {
      setBusy('')
    }
  }

  const openCategoryCreate = () => {
    const nextSort = generalCategories.reduce((max, category) => Math.max(max, category.sort), 0) + 10
    setFormError('')
    setCategoryDraft({ id: null, name: '', sort: String(nextSort), enabled: true })
  }

  const openCategoryEdit = (category: NavigationCategory) => {
    setFormError('')
    setCategoryDraft({ id: category.id, name: category.name, sort: String(category.sort), enabled: category.enabled })
  }

  const openItemCreate = (category: NavigationCategory) => {
    const nextSort = (category.items || []).reduce((max, item) => Math.max(max, item.sort), 0) + 10
    setFormError('')
    setItemDraft({
      id: null,
      categoryId: category.id,
      title: '',
      subtitle: '',
      iconText: '',
      appId: '',
      path: '',
      color: '#684091',
      sort: String(nextSort),
      enabled: true
    })
  }

  const openItemEdit = (item: NavigationItem) => {
    setFormError('')
    setItemDraft({
      id: item.id,
      categoryId: item.categoryId,
      title: item.title,
      subtitle: item.subtitle,
      iconText: item.iconText,
      appId: item.appId,
      path: item.path,
      color: item.color || '#684091',
      sort: String(item.sort),
      enabled: item.enabled
    })
  }

  const openMerchantCreate = () => {
    const nextSort = (directory?.items || []).reduce((max, item) => Math.max(max, item.sort), 0) + 10
    setFormError('')
    setMerchantDraft({ id: null, title: '', subtitle: '', iconText: '', appId: '', path: '', color: '#684091', sort: String(nextSort) })
  }

  const openMerchantEdit = (merchant: ExternalTakeoutMerchant) => {
    setFormError('')
    setMerchantDraft({
      id: merchant.id,
      title: merchant.title,
      subtitle: merchant.subtitle,
      iconText: merchant.iconText,
      appId: merchant.appId,
      path: merchant.path,
      color: merchant.color || '#684091',
      sort: String(merchant.sort)
    })
  }

  const submitCategory = async () => {
    if (!categoryDraft || formSaving) return
    const name = categoryDraft.name.trim()
    if (!name) { setFormError('请填写分类名称'); return }
    if ([...name].length > 64) { setFormError('分类名称最多 64 个字'); return }
    if (!categoryDraft.sort.trim() || !Number.isSafeInteger(Number(categoryDraft.sort))) { setFormError('排序请填写整数'); return }
    const input = { name, sort: Number(categoryDraft.sort), enabled: categoryDraft.enabled }
    setFormSaving(true)
    setFormError('')
    try {
      if (categoryDraft.id === null) await adminAPI.createNavigationCategory(input)
      else await adminAPI.updateNavigationCategory(categoryDraft.id, input)
      await loadNavigation()
      notify(categoryDraft.id === null ? '导航分类已创建' : '导航分类已保存')
      setCategoryDraft(null)
    } catch (reason) {
      const failure = requestError(reason)
      if (failure.conflict) {
        notify(failure.message, 'danger')
        await loadNavigation()
      }
      setFormError(failure.message)
    } finally {
      setFormSaving(false)
    }
  }

  const submitItem = async () => {
    if (!itemDraft || formSaving) return
    if (!itemDraft.categoryId) { setFormError('请选择所属分类'); return }
    if (!itemDraft.sort.trim()) { setFormError('排序请填写整数'); return }
    const input: NavigationItemInput = {
      categoryId: itemDraft.categoryId,
      title: itemDraft.title.trim(),
      subtitle: itemDraft.subtitle.trim(),
      iconText: itemDraft.iconText.trim(),
      appId: itemDraft.appId.trim(),
      path: itemDraft.path.trim(),
      color: itemDraft.color,
      sort: Number(itemDraft.sort),
      enabled: itemDraft.enabled
    }
    const validation = navigationItemError(input)
    if (validation) { setFormError(validation); return }
    setFormSaving(true)
    setFormError('')
    try {
      if (itemDraft.id === null) await adminAPI.createNavigationItem(input)
      else await adminAPI.updateNavigationItem(itemDraft.id, input)
      await loadNavigation()
      notify(itemDraft.id === null ? '服务入口已创建' : '服务入口已保存')
      setItemDraft(null)
    } catch (reason) {
      const failure = requestError(reason)
      if (failure.conflict) {
        notify(failure.message, 'danger')
        await loadNavigation()
      }
      setFormError(failure.message)
    } finally {
      setFormSaving(false)
    }
  }

  const submitMerchant = async () => {
    if (!merchantDraft || formSaving) return
    if (!merchantDraft.sort.trim()) { setFormError('排序请填写 0 至 10000 的整数'); return }
    const input: ExternalTakeoutMerchantInput = {
      title: merchantDraft.title.trim(),
      subtitle: merchantDraft.subtitle.trim(),
      iconText: merchantDraft.iconText.trim(),
      appId: merchantDraft.appId.trim(),
      path: merchantDraft.path.trim().replace(/^\//, ''),
      color: merchantDraft.color,
      sort: Number(merchantDraft.sort)
    }
    const validation = externalMerchantError(input)
    if (validation) { setFormError(validation); return }
    setFormSaving(true)
    setFormError('')
    try {
      if (merchantDraft.id === null) await adminAPI.createExternalTakeoutMerchant(input)
      else await adminAPI.updateExternalTakeoutMerchant(merchantDraft.id, input)
      await loadExternal()
      notify(merchantDraft.id === null ? '外部商家已创建' : '外部商家已保存')
      setMerchantDraft(null)
    } catch (reason) {
      const failure = requestError(reason)
      if (failure.conflict) {
        notify(failure.message, 'danger')
        await loadExternal()
      }
      setFormError(failure.message)
    } finally {
      setFormSaving(false)
    }
  }

  const toggleItem = (item: NavigationItem, enabled: boolean) => {
    if (busy) return
    const input: NavigationItemInput = {
      categoryId: item.categoryId,
      title: item.title,
      subtitle: item.subtitle,
      iconText: item.iconText,
      appId: item.appId,
      path: item.path,
      color: item.color,
      sort: item.sort,
      enabled
    }
    void perform(`item-${item.id}`, () => adminAPI.updateNavigationItem(item.id, input), enabled ? `「${item.title}」已启用` : `「${item.title}」已停用`, loadNavigation, setNavError)
  }

  const toggleExternalCategory = () => {
    if (!directory || busy) return
    const next = !directory.categoryEnabled
    void perform('external-category', () => adminAPI.setExternalTakeoutCategoryVisibility(next), next ? '学生端外卖目录已开启' : '学生端外卖目录已关闭', loadExternal, setExternalError)
  }

  const toggleMerchant = (merchant: ExternalTakeoutMerchant, enabled: boolean) => {
    if (busy) return
    void perform(`external-${merchant.id}`, () => adminAPI.setExternalTakeoutVisibility(merchant.id, enabled), enabled ? `「${merchant.title}」已恢复展示` : `「${merchant.title}」已隐藏`, loadExternal, setExternalError)
  }

  const askDeleteCategory = (category: NavigationCategory) => {
    setConfirmError('')
    setConfirm({
      title: '删除导航分类',
      description: `确认删除“${category.name}”吗？分类下的 ${(category.items || []).length} 个入口会一并删除，学生端将不再展示。`,
      confirmLabel: '确认删除',
      action: async () => {
        await adminAPI.deleteNavigationCategory(category.id)
        await loadNavigation()
        notify('导航分类已删除')
      }
    })
  }

  const askDeleteItem = (item: NavigationItem) => {
    setConfirmError('')
    setConfirm({
      title: '删除服务入口',
      description: `确认删除“${item.title}”吗？删除后学生端将不再展示该入口。`,
      confirmLabel: '确认删除',
      action: async () => {
        await adminAPI.deleteNavigationItem(item.id)
        await loadNavigation()
        notify('服务入口已删除')
      }
    })
  }

  const askDeleteMerchant = (merchant: ExternalTakeoutMerchant) => {
    setConfirmError('')
    setConfirm({
      title: '删除外部商家',
      description: `确认删除“${merchant.title}”吗？删除后学生端外卖目录将不再展示该商家。`,
      confirmLabel: '确认删除',
      action: async () => {
        await adminAPI.deleteExternalTakeoutMerchant(merchant.id)
        await loadExternal()
        notify('外部商家已删除')
      }
    })
  }

  const runConfirm = async () => {
    if (!confirm || confirmSaving) return
    setConfirmSaving(true)
    setConfirmError('')
    try {
      await confirm.action()
      setConfirm(null)
    } catch (reason) {
      const failure = requestError(reason)
      if (failure.conflict) {
        notify(failure.message, 'danger')
        await Promise.all([loadNavigation(), loadExternal()])
        setConfirm(null)
      } else {
        setConfirmError(failure.message)
      }
    } finally {
      setConfirmSaving(false)
    }
  }

  return <div>
    <PageHeader
      eyebrow="校园服务配置"
      title="服务入口"
      description="维护小程序校园服务导航的分类与入口，外部商家目录单独管理。"
      actions={<Button variant="primary" disabled={loading || Boolean(busy)} onClick={openCategoryCreate}>新增分类</Button>}
    />
    <section className="preview-stat-grid">
      <StatCard label="服务入口" value={generalItems.length} meta="真实导航配置总数" tone="accent" />
      <StatCard label="已启用" value={generalItems.filter(item => item.enabled).length} meta="用户端可见" tone="mint" />
      <StatCard label="已停用" value={generalItems.filter(item => !item.enabled).length} meta="保留配置但不展示" tone="ink" />
      <StatCard
        label="配置状态"
        value={configIssues.length === 0 ? '正常' : `${configIssues.length} 项待修`}
        meta={configIssues[0] || 'AppID 与页面路径均完整'}
        tone={configIssues.length === 0 ? 'ink' : 'rose'}
      />
    </section>

    {navError && <div className="preview-form-error">{navError}</div>}

    {!categories
      ? <Panel eyebrow="SERVICE DIRECTORY" title="校园服务目录" description="正在从服务器读取导航配置。">
        {loading
          ? <EmptyState title="正在读取服务入口" description="导航分类与入口加载完成后会自动显示。" />
          : <EmptyState title="暂时无法读取服务入口" description={navError || '请稍后重试。'} action={<Button onClick={() => void loadNavigation()}>重新加载</Button>} />}
      </Panel>
      : generalCategories.length === 0
        ? <Panel eyebrow="SERVICE DIRECTORY" title="校园服务目录" description="入口按排序展示。停用后不会删除配置，可随时恢复。">
          <EmptyState title="还没有导航分类" description="先创建一个分类，再向分类中添加小程序入口。" action={<Button variant="primary" onClick={openCategoryCreate}>新增分类</Button>} />
        </Panel>
        : generalCategories.map(category => <Panel
          key={category.id}
          eyebrow={`SORT ${category.sort}`}
          title={category.name}
          description={`${(category.items || []).length} 个入口 · ${category.enabled ? '分类已启用，学生端可见' : '分类已停用，学生端不展示'}`}
          actions={<>
            <Button size="sm" disabled={Boolean(busy) || formSaving} onClick={() => openCategoryEdit(category)}>编辑分类</Button>
            <Button size="sm" disabled={Boolean(busy) || formSaving} onClick={() => openItemCreate(category)}>新增入口</Button>
            <Button size="sm" variant="danger" disabled={Boolean(busy) || formSaving} onClick={() => askDeleteCategory(category)}>删除分类</Button>
          </>}
        >
          {(category.items || []).length === 0
            ? <EmptyState title="该分类下还没有入口" description="新增入口后会在小程序服务页展示。" action={<Button onClick={() => openItemCreate(category)}>新增入口</Button>} />
            : <div className="preview-service-grid">
              {(category.items || []).toSorted((a, b) => a.sort - b.sort).map(item => <article key={item.id} className={!item.enabled ? 'is-disabled' : ''}>
                <span className="preview-service-grid__icon" style={{ background: item.color || '#684091' }}>{item.iconText || item.title.slice(0, 1)}</span>
                <div className="preview-service-grid__copy">
                  <strong>{item.title}</strong>
                  {item.subtitle && <p>{item.subtitle}</p>}
                  <code>{item.path}</code>
                  <code className="preview-service-grid__appid">{item.appId}</code>
                </div>
                <div className="preview-service-grid__controls">
                  <StatusPill>{item.enabled ? '已启用' : '已停用'}</StatusPill>
                  <Toggle checked={item.enabled} label={`${item.title}启用状态`} onChange={checked => toggleItem(item, checked)} />
                  <Button size="sm" variant="quiet" disabled={Boolean(busy) || formSaving} onClick={() => openItemEdit(item)}>编辑</Button>
                  <Button size="sm" variant="quiet" disabled={Boolean(busy) || formSaving} onClick={() => askDeleteItem(item)}>删除</Button>
                </div>
              </article>)}
            </div>}
        </Panel>)}

    <Panel
      eyebrow="EXTERNAL TAKEOUT"
      title="外部商家入口"
      description={directory
        ? `学生端外卖目录${directory.categoryEnabled ? '已开启' : '已关闭'} · 共 ${(directory.items || []).length} 个商家`
        : '管理学生端外卖入口的外部小程序目录，这里只保存展示与跳转配置。'}
      actions={<>
        <Button size="sm" disabled={!directory || Boolean(busy) || formSaving} onClick={toggleExternalCategory}>
          {directory?.categoryEnabled ? '关闭学生端目录' : '开启学生端目录'}
        </Button>
        <Button size="sm" variant="primary" disabled={!directory || Boolean(busy) || formSaving} onClick={openMerchantCreate}>新增商家</Button>
      </>}
    >
      {externalError && <div className="preview-form-error">{externalError}</div>}
      {!directory
        ? loading
          ? <EmptyState title="正在读取外部商家目录" description="外部商家入口加载完成后会自动显示。" />
          : <EmptyState title="暂时无法读取外部商家目录" description={externalError || '请稍后重试。'} action={<Button onClick={() => void loadExternal()}>重新加载</Button>} />
        : (directory.items || []).length === 0
          ? <EmptyState title="还没有外部商家" description="新增后会在学生端外卖入口展示。" action={<Button variant="primary" onClick={openMerchantCreate}>新增商家</Button>} />
          : <div className="preview-service-grid">
            {(directory.items || []).toSorted((a, b) => a.sort - b.sort).map(merchant => <article key={merchant.id} className={!merchant.enabled ? 'is-disabled' : ''}>
              <span className="preview-service-grid__icon" style={{ background: merchant.color || '#684091' }}>{merchant.iconText || merchant.title.slice(0, 1)}</span>
              <div className="preview-service-grid__copy">
                <strong>{merchant.title}</strong>
                {merchant.subtitle && <p>{merchant.subtitle}</p>}
                <code>{merchant.path}</code>
                <code className="preview-service-grid__appid">{merchant.appId}</code>
              </div>
              <div className="preview-service-grid__controls">
                <StatusPill>{merchant.enabled ? '已启用' : '已停用'}</StatusPill>
                <Toggle checked={merchant.enabled} label={`${merchant.title}展示状态`} onChange={checked => toggleMerchant(merchant, checked)} />
                <Button size="sm" variant="quiet" disabled={Boolean(busy) || formSaving} onClick={() => openMerchantEdit(merchant)}>编辑</Button>
                <Button size="sm" variant="quiet" disabled={Boolean(busy) || formSaving} onClick={() => askDeleteMerchant(merchant)}>删除</Button>
              </div>
            </article>)}
          </div>}
    </Panel>

    <Modal
      open={categoryDraft !== null}
      title={categoryDraft?.id === null ? '新增导航分类' : '编辑导航分类'}
      description="分类用于学生端校园服务页的分组展示，排序值越小越靠前。"
      onClose={() => { if (!formSaving) { setCategoryDraft(null); setFormError('') } }}
      footer={<>
        <Button disabled={formSaving} onClick={() => { setCategoryDraft(null); setFormError('') }}>取消</Button>
        <Button variant="primary" disabled={formSaving} onClick={() => void submitCategory()}>{formSaving ? '正在保存…' : categoryDraft?.id === null ? '创建分类' : '保存修改'}</Button>
      </>}
    >
      {categoryDraft && <div className="preview-form-stack">
        <Field label="分类名称" required value={categoryDraft.name} maxLength={64} onChange={event => setCategoryDraft({ ...categoryDraft, name: event.target.value })} placeholder="例如：校园服务" />
        <Field label="排序" type="number" value={categoryDraft.sort} onChange={event => setCategoryDraft({ ...categoryDraft, sort: event.target.value })} hint="整数，数值越小越靠前。" />
        <label className="preview-field"><span>启用状态</span><div><Toggle checked={categoryDraft.enabled} label="分类启用状态" onChange={checked => setCategoryDraft({ ...categoryDraft, enabled: checked })} /></div></label>
        {formError && <div className="preview-form-error">{formError}</div>}
      </div>}
    </Modal>

    <Modal
      open={itemDraft !== null}
      title={itemDraft?.id === null ? '新增服务入口' : '编辑服务入口'}
      description="入口保存后会写入小程序校园服务导航，请确认页面路径真实可用。"
      onClose={() => { if (!formSaving) { setItemDraft(null); setFormError('') } }}
      footer={<>
        <Button disabled={formSaving} onClick={() => { setItemDraft(null); setFormError('') }}>取消</Button>
        <Button variant="primary" disabled={formSaving} onClick={() => void submitItem()}>{formSaving ? '正在保存…' : itemDraft?.id === null ? '创建入口' : '保存修改'}</Button>
      </>}
    >
      {itemDraft && <div className="preview-form-stack">
        <SelectField label="所属分类" value={itemDraft.categoryId} onChange={event => setItemDraft({ ...itemDraft, categoryId: Number(event.target.value) })}>
          {generalCategories.map(category => <option key={category.id} value={category.id}>{category.name}</option>)}
        </SelectField>
        <Field label="入口名称" required value={itemDraft.title} maxLength={64} onChange={event => setItemDraft({ ...itemDraft, title: event.target.value })} placeholder="例如：校园打印" />
        <Field label="用途说明" value={itemDraft.subtitle} maxLength={120} onChange={event => setItemDraft({ ...itemDraft, subtitle: event.target.value })} placeholder="说明用户可以用它做什么" />
        <Field label="图标文字" value={itemDraft.iconText} maxLength={12} onChange={event => setItemDraft({ ...itemDraft, iconText: event.target.value })} placeholder="如：印" hint="最多 12 个字；留空时取入口名称首字。" />
        <Field label="小程序 AppID" required value={itemDraft.appId} onChange={event => setItemDraft({ ...itemDraft, appId: event.target.value })} placeholder="wx0dacef976daa8e55" hint="wx 开头加 16 位字母或数字。" />
        <Field label="页面路径" required value={itemDraft.path} onChange={event => setItemDraft({ ...itemDraft, path: event.target.value })} placeholder="pages/service/index" hint="小程序内部路径，不要以 / 开头。" />
        <Field label="图标颜色" type="color" value={itemDraft.color} onChange={event => setItemDraft({ ...itemDraft, color: event.target.value })} />
        <Field label="排序" type="number" value={itemDraft.sort} onChange={event => setItemDraft({ ...itemDraft, sort: event.target.value })} hint="整数，数值越小越靠前。" />
        <label className="preview-field"><span>启用状态</span><div><Toggle checked={itemDraft.enabled} label="入口启用状态" onChange={checked => setItemDraft({ ...itemDraft, enabled: checked })} /></div></label>
        {formError && <div className="preview-form-error">{formError}</div>}
      </div>}
    </Modal>

    <Modal
      open={merchantDraft !== null}
      title={merchantDraft?.id === null ? '新增外部商家' : '编辑外部商家'}
      description="外部商家使用自己的小程序承接订单，后台只维护入口展示与跳转。"
      onClose={() => { if (!formSaving) { setMerchantDraft(null); setFormError('') } }}
      footer={<>
        <Button disabled={formSaving} onClick={() => { setMerchantDraft(null); setFormError('') }}>取消</Button>
        <Button variant="primary" disabled={formSaving} onClick={() => void submitMerchant()}>{formSaving ? '正在保存…' : merchantDraft?.id === null ? '创建商家' : '保存修改'}</Button>
      </>}
    >
      {merchantDraft && <div className="preview-form-stack">
        <Field label="商家名称" required value={merchantDraft.title} maxLength={64} onChange={event => setMerchantDraft({ ...merchantDraft, title: event.target.value })} placeholder="例如：北区轻食铺" />
        <Field label="简介" value={merchantDraft.subtitle} maxLength={120} onChange={event => setMerchantDraft({ ...merchantDraft, subtitle: event.target.value })} placeholder="说明商家提供的服务" />
        <Field label="文字图标" required value={merchantDraft.iconText} maxLength={12} onChange={event => setMerchantDraft({ ...merchantDraft, iconText: event.target.value })} placeholder="如：食" hint="最多 12 个字。" />
        <Field label="小程序 AppID" required value={merchantDraft.appId} onChange={event => setMerchantDraft({ ...merchantDraft, appId: event.target.value })} placeholder="wx1234567890ABCDEF" hint="wx 开头加 16 位字母或数字。" />
        <Field label="页面路径" required value={merchantDraft.path} onChange={event => setMerchantDraft({ ...merchantDraft, path: event.target.value })} placeholder="pages/index/index" hint="小程序内部路径，不要以 / 开头。" />
        <Field label="主题颜色" type="color" value={merchantDraft.color} onChange={event => setMerchantDraft({ ...merchantDraft, color: event.target.value })} />
        <Field label="排序" type="number" value={merchantDraft.sort} onChange={event => setMerchantDraft({ ...merchantDraft, sort: event.target.value })} hint="0 至 10000 的整数，数值越小越靠前。" />
        {formError && <div className="preview-form-error">{formError}</div>}
      </div>}
    </Modal>

    <ConfirmModal
      request={confirm}
      busy={confirmSaving}
      error={confirmError}
      onCancel={() => { setConfirm(null); setConfirmError('') }}
      onConfirm={() => void runConfirm()}
    />
  </div>
}

const scenes = [
  { key: 'homeTitle', label: '首页', description: '转发小程序首页时展示' },
  { key: 'secondhandTitle', label: '二手市场', description: '转发二手商品列表或详情时展示' },
  { key: 'servicesTitle', label: '校园服务', description: '转发校园服务页时展示' }
] as const
type SceneKey = typeof scenes[number]['key']

const hasControl = (value: string) => /[\u0000-\u001f\u007f]/.test(value)
const shareTitleError = (value: string) => !value.trim()
  ? '分享标题不能为空'
  : [...value.trim()].length > 60 || hasControl(value) ? '分享标题限 60 个字符，不能包含换行或控制字符' : ''
const shareSuffixError = (value: string) => [...value.trim()].length > 60 || hasControl(value)
  ? '标题后缀限 60 个字符，不能包含换行或控制字符'
  : ''

function ShareSettingsView() {
  const { notify } = usePreviewToast()
  const [base, setBase] = useState<ShareSettings | null>(null)
  const [draft, setDraft] = useState<ShareCards | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [scene, setScene] = useState<SceneKey>('homeTitle')
  const [coverBroken, setCoverBroken] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const value = await shareSettingsAPI.read()
      setBase(value)
      setDraft(cardFields(value))
      setCoverBroken(false)
    } catch (reason) {
      setError(requestError(reason).message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const coverUrl = draft?.defaultImageUrl.trim() || ''
  useEffect(() => { setCoverBroken(false) }, [coverUrl])

  const dirty = Boolean(base && draft && (
    draft.homeTitle !== base.homeTitle ||
    draft.secondhandTitle !== base.secondhandTitle ||
    draft.servicesTitle !== base.servicesTitle ||
    draft.defaultImageUrl !== base.defaultImageUrl ||
    draft.detailTitleSuffix !== base.detailTitleSuffix ||
    draft.preferContentImage !== base.preferContentImage
  ))
  const titleErrors = draft ? {
    homeTitle: shareTitleError(draft.homeTitle),
    secondhandTitle: shareTitleError(draft.secondhandTitle),
    servicesTitle: shareTitleError(draft.servicesTitle)
  } : null
  const suffixError = draft ? shareSuffixError(draft.detailTitleSuffix) : ''
  const imageError = draft ? coverError(draft.defaultImageUrl) : ''
  const invalid = Boolean(titleErrors && (titleErrors.homeTitle || titleErrors.secondhandTitle || titleErrors.servicesTitle || suffixError || imageError))
  const sceneItem = scenes.find(item => item.key === scene) || scenes[0]
  const showCover = Boolean(draft && coverUrl && !imageError && !coverBroken)

  const save = async () => {
    if (!base || !draft || saving || invalid || !dirty) return
    setSaving(true)
    setError('')
    try {
      const value = await shareSettingsAPI.save({
        homeTitle: draft.homeTitle.trim(),
        secondhandTitle: draft.secondhandTitle.trim(),
        servicesTitle: draft.servicesTitle.trim(),
        defaultImageUrl: draft.defaultImageUrl.trim(),
        detailTitleSuffix: draft.detailTitleSuffix.trim(),
        preferContentImage: draft.preferContentImage
      }, base.rowVersion)
      setBase(value)
      setDraft(cardFields(value))
      notify('分享设置已保存')
    } catch (reason) {
      const failure = requestError(reason)
      if (failure.conflict) notify(failure.message, 'danger')
      else setError(failure.message)
    } finally {
      setSaving(false)
    }
  }

  return <div>
    <PageHeader
      eyebrow="用户传播"
      title="分享设置"
      description="维护小程序三处场景的分享标题、详情标题后缀与通用封面，保存后由支持分享的新版本小程序读取。"
      actions={<>
        <Button disabled={loading || saving} onClick={() => void load()}>读取已保存配置</Button>
        <Button variant="primary" disabled={loading || saving || !dirty || invalid} onClick={() => void save()}>{saving ? '正在保存…' : '保存设置'}</Button>
      </>}
    />
    {error && <div className="preview-form-error">{error}</div>}
    {base && <div className="preview-saved-bar">
      <span>{dirty ? '有未保存的修改' : '服务器已保存配置'}</span>
      <small>版本 {base.rowVersion} · 最近更新 {formatDateTime(base.updatedAt)}</small>
    </div>}
    <div className="preview-settings-grid">
      <Panel eyebrow="SHARE CARD" title="分享卡片" description="标题限 60 个字符；封面留空时使用小程序内置默认封面。">
        {!draft
          ? <EmptyState
            title={loading ? '正在读取分享配置' : '暂时无法读取分享配置'}
            description={loading ? '读取完成后可直接编辑三处分享标题。' : error || '请稍后重试。'}
            action={loading ? undefined : <Button onClick={() => void load()}>重新加载</Button>}
          />
          : <div className="preview-form-stack">
            <Field label="首页分享标题" required value={draft.homeTitle} maxLength={60} error={titleErrors?.homeTitle} onChange={event => setDraft({ ...draft, homeTitle: event.target.value })} />
            <Field label="二手市场分享标题" required value={draft.secondhandTitle} maxLength={60} error={titleErrors?.secondhandTitle} onChange={event => setDraft({ ...draft, secondhandTitle: event.target.value })} />
            <Field label="校园服务分享标题" required value={draft.servicesTitle} maxLength={60} error={titleErrors?.servicesTitle} onChange={event => setDraft({ ...draft, servicesTitle: event.target.value })} />
            <Field label="详情标题后缀" value={draft.detailTitleSuffix} maxLength={60} error={suffixError} onChange={event => setDraft({ ...draft, detailTitleSuffix: event.target.value })} hint="跟在真实帖子、商品标题或公开主页昵称后；留空则不追加。" />
            <Field label="通用封面地址" value={draft.defaultImageUrl} maxLength={2048} error={imageError} placeholder="https://.../share-cover.jpg" onChange={event => setDraft({ ...draft, defaultImageUrl: event.target.value })} hint="也可直接拖拽下方上传图片，或粘贴本站/腾讯云 COS 的公开 HTTPS JPG/PNG 地址；留空使用内置默认封面。" />
            <CoverUploader onUploaded={url => setDraft(previous => previous ? { ...previous, defaultImageUrl: url } : previous)} />
            <label className="preview-field"><span>详情优先使用真实内容图片</span><div><Toggle checked={draft.preferContentImage} label="详情优先使用真实内容图片" onChange={checked => setDraft({ ...draft, preferContentImage: checked })} /></div></label>
          </div>}
      </Panel>
      <Panel eyebrow="LIVE PREVIEW" title="用户看到的分享卡片" description="切换场景查看标题与封面效果，最终样式以微信为准。">
        {draft && <>
          <Segmented
            value={scene}
            options={scenes.map(item => ({ value: item.key, label: item.label }))}
            onChange={value => setScene(value)}
          />
          <div className="preview-share-preview" style={{ marginTop: 12 }}>
            <div className="preview-share-preview__cover">
              {showCover
                ? <img
                  src={coverUrl}
                  alt={`${sceneItem.label}分享封面预览`}
                  referrerPolicy="no-referrer"
                  onError={() => setCoverBroken(true)}
                  style={{ display: 'block', width: 'calc(100% + 40px)', height: 190, margin: -20, objectFit: 'cover' }}
                />
                : <><span>湛科生活圈</span><strong>校园服务<br />一站直达</strong><i /></>}
            </div>
            <div className="preview-share-preview__copy">
              <span>{sceneItem.label} · {sceneItem.description}</span>
              <strong>{draft[scene] || '未填写分享标题'}</strong>
              <p>{draft.detailTitleSuffix ? `详情标题后缀：${draft.detailTitleSuffix}` : '未设置详情标题后缀'}</p>
            </div>
            <div className="preview-share-preview__brand">
              <span><img className="brand-logo-img" src="/images/brand-logo.png" alt="湛科万事通" /></span>
              <div><strong>湛科生活圈</strong><small>{draft.preferContentImage ? '详情优先使用真实内容图片' : '详情使用通用封面'}</small></div>
            </div>
          </div>
          {coverBroken && <div className="preview-form-error">封面图片加载失败，预览已回退为内置封面；请确认地址可以公开访问。</div>}
        </>}
      </Panel>
    </div>
  </div>
}

function FeatureSettings() {
  const { notify } = usePreviewToast()
  const role = useAdminStore(state => state.user?.role)
  const canEdit = role === 'superadmin'
  const [policy, setPolicy] = useState<FeaturePolicy | null>(null)
  const [flags, setFlags] = useState<Record<FeaturePolicyKey, boolean> | null>(null)
  const [postingMode, setPostingMode] = useState<FeaturePolicy['postingMode']>('admin_only')
  const [presentation, setPresentation] = useState<PresentationMode>('standard')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const apply = useCallback((value: FeaturePolicy) => {
    setPolicy(value)
    setFlags(Object.fromEntries(FEATURE_KEYS.map(key => [key, value.flags[key] === true])) as Record<FeaturePolicyKey, boolean>)
    setPostingMode(value.postingMode)
    setPresentation(value.presentationMode || 'standard')
  }, [])

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const value = await adminAPI.featurePolicy()
      if (!value.enabled) throw new Error('功能配置服务暂未启用，暂时无法读取或保存。')
      if (FEATURE_KEYS.some(key => typeof value.flags[key] !== 'boolean')) throw new Error('功能配置暂时无法读取，请稍后重试。')
      const mode = value.presentationMode as string | undefined
      if (mode !== undefined && mode !== 'standard' && mode !== 'campus_info') throw new Error('展示模式暂不支持，请稍后重试。')
      apply(value)
    } catch (reason) {
      setError(requestError(reason).message)
    } finally {
      setLoading(false)
    }
  }, [apply])

  useEffect(() => { void load() }, [load])

  const canSelectPresentation = policy?.presentationMode === 'standard' || policy?.presentationMode === 'campus_info'
  const dirty = Boolean(policy && flags && (
    postingMode !== policy.postingMode ||
    presentation !== (policy.presentationMode || 'standard') ||
    FEATURE_KEYS.some(key => flags[key] !== policy.flags[key])
  ))
  const enabledIn = (keys: Array<{ key: FeatureKey }>) => flags ? keys.filter(item => flags[item.key]).length : 0
  const wholeGroupOn = (keys: Array<{ key: FeatureKey }>) => Boolean(flags && keys.every(item => flags![item.key]))

  const save = async () => {
    if (!canEdit || !policy || !flags || saving || !dirty) return
    setSaving(true)
    setError('')
    try {
      const value = await adminAPI.updateFeaturePolicy({
        flags,
        postingMode,
        ...(canSelectPresentation ? { presentationMode: presentation } : {}),
        expectedRowVersion: policy.rowVersion
      })
      apply(value)
      notify('功能开关已保存，小程序回到前台后生效')
    } catch (reason) {
      const failure = requestError(reason)
      if (failure.conflict) notify(failure.message, 'danger')
      else setError(failure.message)
    } finally {
      setSaving(false)
    }
  }

  const setAll = (value: boolean) => {
    if (!canEdit || saving) return
    setFlags(Object.fromEntries(FEATURE_KEYS.map(key => [key, value])) as Record<FeaturePolicyKey, boolean>)
  }

  return <div>
    <PageHeader
      eyebrow="平台设置"
      title="功能开关"
      description="控制小程序模块、互动能力与新业务开放状态"
      actions={<>
        <Button disabled={loading || saving} onClick={() => void load()}>读取已保存配置</Button>
        <Button disabled={!canEdit || saving || !flags || loading} onClick={() => setAll(false)}>全部隐藏</Button>
        <Button variant="primary" disabled={!canEdit || !dirty || saving || loading} onClick={() => void save()}>{saving ? '正在保存…' : '保存配置'}</Button>
      </>}
    />

    {!policy || !flags
      ? <Panel eyebrow="FEATURE POLICY" title="小程序功能显隐中心" description="正在读取服务器保存的模块状态。">
        {loading
          ? <EmptyState title="正在读取功能开关" description="20 项功能开关与展示模式加载完成后会自动显示。" />
          : <EmptyState title="暂时无法读取功能配置" description={error || '请稍后重试。'} action={<Button onClick={() => void load()}>重新加载</Button>} />}
      </Panel>
      : <Panel eyebrow="FEATURE POLICY" title="小程序功能显隐中心" description="选择需要开放的模块，保存后控制所有用户的小程序入口和新业务。账号安全、隐私、反馈与已有订单的必要处理保持可用。">
        {error && <div className="preview-form-error">{error}</div>}
        {!canEdit && <div className="preview-policy-note"><span>只读视图</span><p>只有超级管理员可以保存功能策略；当前账号可以查看服务器配置。</p></div>}
        <div className="preview-saved-bar">
          <span>{dirty ? '当前显示未保存的编辑内容' : '当前显示服务器已保存配置'}</span>
          <small>版本 {policy.rowVersion} · 操作人 #{policy.updatedBy} · 最近更新 {formatDateTime(policy.updatedAt)}</small>
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
              disabled={!canEdit || saving || loading || !canSelectPresentation}
              className={`preview-choice${presentation === option.value ? ' is-selected' : ''}`}
              aria-pressed={presentation === option.value}
              onClick={() => setPresentation(option.value)}
            >
              <span className="preview-choice__dot" aria-hidden="true" />
              <strong>{option.title}</strong>
              <small>{option.description}</small>
            </button>)}
          </div>
          {!canSelectPresentation && <div className="preview-policy-note"><span>展示模式</span><p>当前版本暂不支持切换展示模式，下方模块开关仍可正常使用。</p></div>}
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
              disabled={!canEdit || saving || loading}
              className={`preview-choice${postingMode === option.value ? ' is-selected' : ''}`}
              aria-pressed={postingMode === option.value}
              onClick={() => setPostingMode(option.value)}
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
              <button type="button" disabled={!canEdit || saving} onClick={() => {
                if (!canEdit || saving || !flags) return
                const next = !wholeGroupOn(group.keys)
                setFlags({ ...flags, ...Object.fromEntries(group.keys.map(item => [item.key, next])) } as Record<FeaturePolicyKey, boolean>)
              }}>{wholeGroupOn(group.keys) ? '关闭本组' : '开启本组'}</button>
            </em>
          </div>
          <div className="preview-policy-rows">
            {group.keys.map(item => <label className={`preview-policy-row${flags[item.key] ? '' : ' is-off'}`} key={item.key}>
              <span><strong>{item.label}</strong><small>{item.description}</small></span>
              <Toggle
                checked={flags[item.key]}
                label={item.label}
                onChange={checked => {
                  if (!canEdit || saving || !flags) return
                  setFlags({ ...flags, [item.key]: checked } as Record<FeaturePolicyKey, boolean>)
                }}
              />
            </label>)}
          </div>
        </section>)}

        <div className="preview-policy-note">
          <span>变更纪律</span>
          <p>功能开关变更会记录操作人和时间；保存后由小程序读取生效，账号安全、隐私、反馈与已有订单的必要处理保持可用。</p>
        </div>
      </Panel>}
  </div>
}


function CoverUploader({ onUploaded }: { onUploaded: (url: string) => void }) {
  const [drag, setDrag] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const upload = async (file: File) => {
    if (!/^image\/(jpeg|png)$/.test(file.type)) {
      setError('仅支持 JPG 或 PNG 图片')
      return
    }
    if (file.size > 5 * 1024 * 1024) {
      setError('图片需在 5MB 以内')
      return
    }
    const form = new FormData()
    form.append('file', file)
    setBusy(true)
    setError('')
    try {
      const data = await shareSettingsAPI.uploadCover(form)
      onUploaded(data.url)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause))
    } finally {
      setBusy(false)
    }
  }
  return <div
    className={`preview-cover-drop${drag ? ' is-drag' : ''}${busy ? ' is-busy' : ''}`}
    role="button"
    tabIndex={0}
    aria-label="上传分享封面图"
    onClick={() => { if (!busy) inputRef.current?.click() }}
    onKeyDown={event => {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault()
        if (!busy) inputRef.current?.click()
      }
    }}
    onDragOver={event => { event.preventDefault(); if (!busy) setDrag(true) }}
    onDragLeave={() => setDrag(false)}
    onDrop={event => {
      event.preventDefault()
      setDrag(false)
      const file = event.dataTransfer.files?.[0]
      if (file) void upload(file)
    }}
  >
    <input
      ref={inputRef}
      type="file"
      accept="image/jpeg,image/png"
      hidden
      onChange={event => {
        const file = event.target.files?.[0]
        if (file) void upload(file)
        event.target.value = ''
      }}
    />
    {busy
      ? <strong>正在上传封面图…</strong>
      : <>
        <strong>拖拽图片到这里，或点击选择文件</strong>
        <small>支持 JPG/PNG，5MB 以内。上传成功后会自动填入封面地址，再点右上角「保存设置」生效。</small>
      </>}
    {error && <span className="preview-form-error">{error}</span>}
  </div>
}
