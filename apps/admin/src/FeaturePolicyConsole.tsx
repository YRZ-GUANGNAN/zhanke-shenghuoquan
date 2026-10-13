import { useCallback, useEffect, useMemo, useState } from 'react'
import { adminAPI, AdminAPIError } from './services/f-admin'
import { useAdminStore } from './store'
import { AdminButton, AdminNotice, AdminPanel, AdminState } from './components'
import type { FeaturePolicy, FeaturePolicyKey } from './types'

type PolicyGroup = { title: string; description: string; keys: Array<{ key: FeaturePolicyKey; label: string; description: string }> }

const groups: PolicyGroup[] = [
  {
    title: '社区内容',
    description: '只控制用户端入口与写入能力，不删除既有内容。',
    keys: [
      { key: 'forum', label: '论坛入口', description: '隐藏社区首页、帖子列表与旧链接入口' },
      { key: 'post', label: '发帖', description: '允许用户创建新的社区帖子' },
      { key: 'comment', label: '评论', description: '允许用户对社区帖子发表评论' },
      { key: 'following', label: '关注', description: '显示关注关系与关注动态入口' },
      { key: 'likes', label: '点赞', description: '帖子点赞入口与请求' },
      { key: 'favorites', label: '收藏', description: '帖子和商品收藏入口与请求' },
      { key: 'notifications', label: '互动通知', description: '评论与互动通知列表' },
    ],
  },
  {
    title: '交易与服务',
    description: '二手与校园服务可以独立切换，避免关闭外卖时误伤校内工具。',
    keys: [
      { key: 'secondhand', label: '二手市场', description: '商品浏览、详情与历史链接' },
      { key: 'secondhand_publish', label: '发布二手商品', description: '允许创建和编辑新的商品' },
      { key: 'secondhand_chat', label: '二手沟通', description: '商品联系与交易会话入口' },
      { key: 'secondhand_comments', label: '商品留言', description: '商品留言列表与回复，不受社区发言模式影响' },
      { key: 'takeout', label: '外卖商家', description: '合作外卖商家目录，不影响常用校园工具' },
      { key: 'campus_tools', label: '校园服务', description: '校内服务导航与工具入口' },
      { key: 'delivery', label: '跑腿配送', description: '新发单与抢单；已有订单履约和资金不受影响' },
      { key: 'merchant', label: '商家工作台', description: '工作台入口与新业务；历史订单处理仍保留' },
      { key: 'rider', label: '骑手工作台', description: '工作台入口与抢单；已接订单仍可完成' },
      { key: 'identity_application', label: '身份申请', description: '新增骑手/商家申请，不删除已有身份' },
    ],
  },
  {
    title: '交流关系',
    description: '关闭入口不会删除会话、关注或历史记录。',
    keys: [
      { key: 'messages', label: '私信', description: '通用消息与会话入口' },
      { key: 'search', label: '搜索', description: '社区、商品与商家的搜索入口' },
      { key: 'publish', label: '统一发布入口', description: '底部加号与发布选择页，关闭后也限制帖子和商品发布' },
    ],
  },
]

const allKeys = groups.flatMap((group) => group.keys.map((item) => item.key))

export function FeaturePolicyConsole() {
  const sessionUser = useAdminStore((state) => state.user)
  const [data, setData] = useState<FeaturePolicy | null>(null)
  const releaseRestricted = data?.releaseScope === 'campus_info'
  const canEdit = sessionUser?.role === 'superadmin' && !releaseRestricted
  const [draft, setDraft] = useState<Record<FeaturePolicyKey, boolean> | null>(null)
  const [mode, setMode] = useState<FeaturePolicy['postingMode']>('admin_only')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const value = await adminAPI.featurePolicy()
      if (!value.enabled || allKeys.some(key => typeof value.flags[key] !== 'boolean')) throw new Error('服务端功能配置版本不完整，请部署最新后端后重试。')
      const flags = Object.fromEntries(allKeys.map((key) => [key, value.flags[key] === true])) as Record<FeaturePolicyKey, boolean>
      setData(value)
      setDraft(flags)
      setMode(value.postingMode)
    } catch (reason) {
      setError(String(reason))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const changed = useMemo(() => {
    if (!data || !draft) return false
    return mode !== data.postingMode || allKeys.some((key) => draft[key] !== data.flags[key])
  }, [data, draft, mode])

  const save = async () => {
    if (!canEdit || !data || !draft || saving || !changed) return
    setSaving(true)
    setError('')
    setNotice('')
    try {
      const value = await adminAPI.updateFeaturePolicy({ flags: draft, postingMode: mode, expectedRowVersion: data.rowVersion })
      setData(value)
      setDraft({ ...value.flags })
      setMode(value.postingMode)
      setNotice('配置已保存到服务器。小程序回前台立即刷新，使用中最迟约 15 秒更新入口；新的业务请求同时受服务端控制。')
    } catch (reason) {
      setError(reason instanceof AdminAPIError && reason.status === 409 ? '配置已被其他管理员更新。请读取已保存配置后重新修改和保存；本次修改未覆盖服务器。' : String(reason))
    } finally {
      setSaving(false)
    }
  }

  if (loading && !data) return <AdminState kind='loading' title='正在读取功能设置' description='读取服务器保存的实际模块状态。' />
  if (error && !data) return <AdminPanel title='功能设置暂不可用' actions={<AdminButton onClick={() => void load()}>重试</AdminButton>}><AdminState kind='error' title='无法读取配置' description={error} /></AdminPanel>
  if (!draft || !data) return null

  return (
    <div className='feature-policy-console'>
      <AdminPanel
        title='小程序功能显隐中心'
        description='这里显示本发行版本的有效能力。账号安全、隐私、反馈与已有订单的必要处理保持可用。'
        actions={<><AdminButton tone='ghost' loading={loading} disabled={saving} onClick={() => void load()}>读取已保存配置</AdminButton><AdminButton tone='ghost' disabled={!canEdit || saving} onClick={() => setDraft(Object.fromEntries(allKeys.map(key => [key, false])) as Record<FeaturePolicyKey, boolean>)}>全部隐藏</AdminButton><AdminButton tone='primary' loading={saving} disabled={!canEdit || !changed || loading} onClick={() => void save()}>保存配置</AdminButton></>}
      >
        {error && <AdminNotice tone='danger' title='保存失败'>{error}</AdminNotice>}
        {notice && <AdminNotice tone='success'>{notice}</AdminNotice>}
        {releaseRestricted ? <AdminNotice tone='info' title='当前发行范围：校园指南与基础账号服务'>校园指南由核验后的公开资料整理，随内容版本发布。以下业务不属于当前发行范围，不能通过后台重新开启；新增业务需要发布新版本并重新审核。历史数据和已有订单仍按原权限处理。</AdminNotice> : !canEdit && <AdminNotice tone='info' title='当前为只读视图'>只有超级管理员可以保存功能策略。</AdminNotice>}
        <div className='feature-policy-callout'>
          <strong>{releaseRestricted ? '当前显示发行范围限制后的有效配置' : changed ? '有未保存的修改' : '当前显示服务器已保存配置'}</strong>
          <span>版本 {data.rowVersion} · 操作人 #{data.updatedBy} · 最近更新 {data.updatedAt ? new Date(data.updatedAt).toLocaleString('zh-CN') : '尚未更新'}</span>
        </div>
        <section className='feature-policy-mode' aria-label='发帖评论模式'>
          <div><h3>发帖与评论权限</h3><p>普通用户不会因此获得后台管理权限；这里只控制社区写入能力。</p></div>
          <label><input disabled={!canEdit || saving || loading} type='radio' name='posting-mode' checked={mode === 'admin_only'} onChange={() => setMode('admin_only')} /><span><strong>仅管理员</strong><small>普通用户只能浏览，管理员与超级管理员可发帖评论</small></span></label>
          <label><input disabled={!canEdit || saving || loading} type='radio' name='posting-mode' checked={mode === 'all_users'} onChange={() => setMode('all_users')} /><span><strong>全部用户</strong><small>所有通过身份与禁言检查的用户可发帖评论</small></span></label>
        </section>
        <div className='feature-policy-groups'>
          {groups.map((group) => <section className='feature-policy-group' key={group.title}>
            <header><div><h3>{group.title}</h3><p>{group.description}</p></div><span>{group.keys.filter(({ key }) => draft[key]).length}/{group.keys.length} 已开启 <AdminButton tone='text' disabled={!canEdit || saving} onClick={() => setDraft(current => current ? { ...current, ...Object.fromEntries(group.keys.map(({ key }) => [key, !group.keys.every(item => current[item.key])])) } : current)}>{group.keys.every(item => draft[item.key]) ? '关闭本组' : '开启本组'}</AdminButton></span></header>
            <div className='feature-policy-list'>{group.keys.map(({ key, label, description }) => <label className={`feature-policy-row${draft[key] ? '' : ' feature-policy-row--off'}`} key={key}>
              <span className='feature-policy-row__copy'><strong>{label}</strong><small>{description}</small></span>
              <input disabled={!canEdit || saving} type='checkbox' checked={draft[key]} onChange={(event) => setDraft((current) => current ? { ...current, [key]: event.target.checked } : current)} />
            </label>)}</div>
          </section>)}
        </div>
      </AdminPanel>
    </div>
  )
}
