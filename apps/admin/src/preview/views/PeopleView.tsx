import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type {
  FeedbackRow,
  IdentityApplicationAdminList,
  IdentityApplicationDetail,
  IdentityApplicationEvent,
  IdentityApplicationQuery,
  IdentityApplicationRole,
  IdentityApplicationRow,
  IdentityApplicationStatus,
  Page,
  UserRow
} from '../../types'
import { adminAPI } from '../../services/f-admin'
import { adminAuthAPI, type AdminCredential } from '../../services/admin-auth'
import type { PreviewSection } from '../data'
import {
  Button,
  Drawer,
  EmptyState,
  Field,
  Modal,
  PageHeader,
  Pagination,
  Panel,
  SearchBox,
  Segmented,
  SelectField,
  StatCard,
  StatusPill,
  TextAreaField,
  usePreviewToast
} from '../ui'
import { RealImage, RealImageStrip, adminAssetContentUrl } from '../media-thumbs'

type PeopleSection = Extract<PreviewSection, 'identities' | 'feedbacks' | 'users'>

const PAGE_SIZE = 8
const CONFLICT_MESSAGE = '数据已被他人修改，请刷新后重试'

const identityRoleLabel = (role: IdentityApplicationRole) => role === 'merchant' ? '商家' : '骑手'
const identityStatusLabel = (status: IdentityApplicationStatus) => ({ pending: '待认证', approved: '已通过', rejected: '已拒绝' })[status]
const identityEventLabel = (type: IdentityApplicationEvent['eventType']) => ({
  submitted: '已提交',
  resubmitted: '已重新提交',
  approved: '已通过',
  rejected: '已拒绝'
})[type]

const userRoleLabel: Record<UserRow['role'], string> = { user: '普通用户', admin: '管理员', superadmin: '超级管理员' }
const userRoleText = (value: string) => userRoleLabel[value as UserRow['role']] || value
const userStatusLabel: Record<UserRow['status'], string> = { active: '正常', muted: '禁言', banned: '停用' }
const feedbackStatusLabel: Record<string, string> = { open: '待处理', in_progress: '处理中', resolved: '已解决', ignored: '不处理' }
const feedbackStatusValues = ['open', 'in_progress', 'resolved', 'ignored'] as const
type FeedbackStatus = typeof feedbackStatusValues[number]
type FeedbackFilter = '全部' | FeedbackStatus

const feedbackStatusText = (value: string) => feedbackStatusLabel[value] || value

const displayTime = (value?: string | null) => {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString('zh-CN')
}

// 后端错误信息以英文为主，这里把常见约束翻译成可直接展示的提示。
const errorTranslations: Record<string, string> = {
  forbidden: '当前账号无权执行该操作',
  'cannot modify own administrative account': '不能修改自己的管理员账号',
  'cannot disable or demote the last active superadmin': '不能停用或降级最后一个超级管理员',
  'only a super administrator can manage administrator accounts': '仅超级管理员可以管理后台登录账号',
  'cannot remove your own administrator credential': '不能移除自己的登录账号',
  'target user is not an active administrator': '目标用户不是启用状态的管理员',
  'admin username must be 3-64 characters of letters, digits, dot or underscore': '用户名需为 3–64 位字母、数字、点或下划线',
  'this administrator username is already in use': '该登录用户名已被使用',
  'password must contain 14 to 72 characters': '密码需为 14–72 位',
  'administrator accounts unavailable': '后台登录账号暂时无法读取',
  'save administrator account failed': '保存后台登录账号失败',
  'remove administrator account failed': '移除后台登录账号失败'
}

function errorStatus(reason: unknown): number | undefined {
  if (typeof reason === 'object' && reason !== null && 'status' in reason) {
    const status = (reason as { status?: unknown }).status
    return typeof status === 'number' ? status : undefined
  }
  return undefined
}

function friendlyError(reason: unknown): string {
  const message = reason instanceof Error ? reason.message : ''
  // 409 也可能表示“用户名已被占用”等具体冲突，优先展示可读的具体提示。
  if (message && errorTranslations[message]) return errorTranslations[message]
  if (errorStatus(reason) === 409) return CONFLICT_MESSAGE
  return message || '操作失败，请重试'
}

async function fetchAllUsers(): Promise<UserRow[]> {
  const collected: UserRow[] = []
  let page = 1
  for (;;) {
    const result = await adminAPI.users({ page, pageSize: 100 })
    collected.push(...result.items)
    const pageSize = Math.max(1, result.pageSize)
    if (result.items.length === 0 || collected.length >= result.total || page >= Math.ceil(result.total / pageSize) || page >= 100) break
    page += 1
  }
  return collected
}

export function PeopleView({ section }: { section: PeopleSection }) {
  if (section === 'identities') return <IdentityPeople />
  if (section === 'feedbacks') return <FeedbackPeople />
  return <UserPeople />
}

function IdentityPeople() {
  const { notify } = usePreviewToast()
  const [status, setStatus] = useState<'全部' | IdentityApplicationStatus>('全部')
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const [list, setList] = useState<IdentityApplicationAdminList>({ items: [], page: 1, pageSize: PAGE_SIZE, total: 0 })
  const [loading, setLoading] = useState(false)
  const [listError, setListError] = useState('')
  const [counts, setCounts] = useState<{ pending: number; merchant: number; rider: number; approved: number } | null>(null)
  const [detailId, setDetailId] = useState<string | null>(null)
  const [detail, setDetail] = useState<IdentityApplicationDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState('')
  const [detailRefreshKey, setDetailRefreshKey] = useState(0)
  const [selected, setSelected] = useState<IdentityApplicationRow | null>(null)
  const [decision, setDecision] = useState<'approved' | 'rejected'>('approved')
  const [reason, setReason] = useState('')
  const [decisionError, setDecisionError] = useState('')
  const [busy, setBusy] = useState(false)
  const listRequestRef = useRef(0)
  const detailRequestRef = useRef(0)
  const countsRequestRef = useRef(0)

  const loadList = useCallback(async () => {
    const requestId = ++listRequestRef.current
    setLoading(true)
    setListError('')
    const request: IdentityApplicationQuery = { page, pageSize: PAGE_SIZE }
    if (status !== '全部') request.status = status
    try {
      const next = await adminAPI.listIdentityApplications(request)
      if (requestId !== listRequestRef.current) return
      if (next.items.length === 0 && next.total > 0 && page > 1) {
        setPage(current => Math.max(1, current - 1))
        return
      }
      setList(next)
    } catch (error) {
      if (requestId === listRequestRef.current) setListError(friendlyError(error))
    } finally {
      if (requestId === listRequestRef.current) setLoading(false)
    }
  }, [page, status])

  const loadCounts = useCallback(async () => {
    const requestId = ++countsRequestRef.current
    try {
      const [pending, merchant, rider, approved] = await Promise.all([
        adminAPI.listIdentityApplications({ status: 'pending', page: 1, pageSize: 1 }),
        adminAPI.listIdentityApplications({ role: 'merchant', page: 1, pageSize: 1 }),
        adminAPI.listIdentityApplications({ role: 'rider', page: 1, pageSize: 1 }),
        adminAPI.listIdentityApplications({ status: 'approved', page: 1, pageSize: 1 })
      ])
      if (requestId !== countsRequestRef.current) return
      setCounts({ pending: pending.total, merchant: merchant.total, rider: rider.total, approved: approved.total })
    } catch {
      // 统计失败时保留上一次数据，不阻塞工作队列。
    }
  }, [])

  useEffect(() => { void loadList() }, [loadList])
  useEffect(() => { void loadCounts() }, [loadCounts])

  useEffect(() => {
    if (!detailId) {
      setDetail(null)
      setDetailError('')
      setDetailLoading(false)
      return
    }
    const requestId = ++detailRequestRef.current
    setDetailLoading(true)
    setDetailError('')
    adminAPI.identityApplication(detailId)
      .then(value => {
        if (requestId === detailRequestRef.current) setDetail(value)
      })
      .catch(error => {
        if (requestId !== detailRequestRef.current) return
        setDetail(null)
        setDetailError(friendlyError(error))
      })
      .finally(() => {
        if (requestId === detailRequestRef.current) setDetailLoading(false)
      })
  }, [detailId, detailRefreshKey])

  const openDetail = (applicationId: string) => {
    setDetail(null)
    setDetailId(applicationId)
  }

  const closeDetail = () => {
    setDetailId(null)
    setDetail(null)
  }

  const openDecision = (item: IdentityApplicationRow, next: 'approved' | 'rejected') => {
    setSelected(item)
    setDecision(next)
    setReason('')
    setDecisionError('')
  }

  const decisionTitle = selected
    ? selected.status === 'pending'
      ? (decision === 'approved' ? '通过认证' : '拒绝认证')
      : (decision === 'approved' ? '改判为通过' : '改判为拒绝')
    : '认证结论'

  const submitDecision = async () => {
    if (!selected) return
    const reasonText = reason.trim()
    if (!reasonText) {
      setDecisionError('请填写认证结论依据')
      return
    }
    const target = selected
    setBusy(true)
    setDecisionError('')
    try {
      if (target.status === 'pending') {
        if (decision === 'approved') {
          await adminAPI.approveIdentityApplication(target.applicationId, target.rowVersion, reasonText)
        } else {
          await adminAPI.rejectIdentityApplication(target.applicationId, target.rowVersion, reasonText)
        }
      } else {
        await adminAPI.overrideIdentityApplication(target.applicationId, target.rowVersion, decision, reasonText)
      }
      setSelected(null)
      notify(decision === 'approved' ? '认证资料已通过' : '认证资料已拒绝，原因已记录', decision === 'approved' ? 'success' : 'info')
      if (detailId === target.applicationId) setDetailRefreshKey(key => key + 1)
      await Promise.all([loadList(), loadCounts()])
    } catch (error) {
      setDecisionError(friendlyError(error))
      if (errorStatus(error) === 409) {
        // 版本冲突说明数据已被他人修改：刷新列表、详情与当前结论的 rowVersion，让用户基于最新版本重试。
        if (detailId === target.applicationId) setDetailRefreshKey(key => key + 1)
        void loadList()
        void loadCounts()
        try {
          const fresh = await adminAPI.identityApplication(target.applicationId)
          setSelected(current => current && current.applicationId === target.applicationId ? { ...current, ...fresh } : current)
        } catch {
          // 刷新失败时保留冲突提示，用户可关闭弹窗后重新打开。
        }
      }
    } finally {
      setBusy(false)
    }
  }

  const visibleItems = useMemo(() => {
    const normalized = query.trim().toLowerCase()
    if (!normalized) return list.items
    return list.items.filter(item => `${item.realName} ${item.applicationId} ${identityRoleLabel(item.role)} ${item.merchantName} ${item.merchantAddress} ${item.phone} #${item.userId}`.toLowerCase().includes(normalized))
  }, [list.items, query])

  return <div>
    <PageHeader
      eyebrow="身份与准入"
      title="商家骑手认证"
      description="把材料、风险和结论放在同一个处理面板中，认证结论会写入真实账号准入。"
      actions={<Button variant="primary" onClick={() => { setStatus('pending'); setQuery(''); setPage(1) }}>查看待认证资料</Button>}
    />
    <section className="preview-stat-grid">
      <StatCard label="待认证" value={counts ? counts.pending : '—'} meta="商家与骑手合计" tone="amber" />
      <StatCard label="商家申请" value={counts ? counts.merchant : '—'} meta="含新驻与变更" tone="accent" />
      <StatCard label="骑手申请" value={counts ? counts.rider : '—'} meta="配送服务准入" tone="mint" />
      <StatCard label="已通过" value={counts ? counts.approved : '—'} meta="可正常接单/营业" tone="ink" />
    </section>
    <Panel
      eyebrow="TRUST & ACCESS"
      title="认证工作队列"
      description="列表按状态走真实接口筛选，搜索作用于当前页；打开详情即可核验材料与审核历史。"
      actions={<SearchBox value={query} onChange={setQuery} placeholder="搜索申请人或业务信息" />}
    >
      <div className="preview-toolbar">
        <Segmented
          value={status}
          onChange={value => { setStatus(value); setPage(1) }}
          options={[
            { value: '全部', label: '全部' },
            { value: 'pending', label: '待认证' },
            { value: 'approved', label: '已通过' },
            { value: 'rejected', label: '已拒绝' }
          ]}
        />
        <Button size="sm" variant="ghost" disabled={loading} onClick={() => { void loadList(); void loadCounts() }}>{loading ? '加载中…' : '刷新'}</Button>
      </div>
      {listError && <div className="preview-form-error" role="alert">{listError}</div>}
      {loading && list.items.length === 0
        ? <div className="preview-policy-note">正在加载认证资料…</div>
        : visibleItems.length === 0
          ? <EmptyState title="没有匹配的认证资料" description="调整筛选条件后再试。" />
          : <div className="preview-review-list">
            {visibleItems.map(item => <article key={item.applicationId} className="preview-review-row is-clickable" onClick={() => openDetail(item.applicationId)}>
              <span className="preview-review-row__kind">{identityRoleLabel(item.role)}</span>
              <div className="preview-review-row__copy">
                <div>
                  <button type="button" className="preview-table__primary" onClick={event => { event.stopPropagation(); openDetail(item.applicationId) }}>
                    {item.realName || `用户 #${item.userId}`}<small>{item.applicationId}</small>
                  </button>
                  <StatusPill>{identityStatusLabel(item.status)}</StatusPill>
                </div>
                <p>{item.role === 'merchant'
                  ? [item.merchantName, item.merchantAddress].filter(Boolean).join(' · ') || '未填写经营信息'
                  : item.phone || '未填写联系方式'}</p>
                <small>{item.applicationId} · {displayTime(item.submittedAt)}{item.reviewReason ? ` · 结论依据：${item.reviewReason}` : ''}</small>
              </div>
              <div className="preview-row-actions" onClick={event => event.stopPropagation()}>
                {item.status === 'pending' ? <>
                  <Button size="sm" variant="ghost" onClick={() => openDetail(item.applicationId)}>开始审核</Button>
                  <Button size="sm" variant="primary" onClick={() => openDecision(item, 'approved')}>通过</Button>
                  <Button size="sm" variant="danger" onClick={() => openDecision(item, 'rejected')}>拒绝</Button>
                </> : <Button size="sm" variant="ghost" onClick={() => openDecision(item, item.status === 'approved' ? 'rejected' : 'approved')}>改判</Button>}
              </div>
            </article>)}
          </div>}
      <Pagination page={list.page} pageSize={list.pageSize} total={list.total} onChange={setPage} label="认证资料分页" />
    </Panel>

    <Drawer
      open={detailId !== null}
      onClose={closeDetail}
      title={detail ? `${detail.realName || `用户 #${detail.userId}`} · ${identityRoleLabel(detail.role)}认证` : '认证资料'}
      description={detail ? `提交于 ${displayTime(detail.submittedAt)}` : undefined}
      footer={<>
        <Button onClick={closeDetail}>关闭</Button>
        {detail?.status === 'pending' && <>
          <Button variant="primary" onClick={() => { closeDetail(); openDecision(detail, 'approved') }}>通过认证</Button>
          <Button variant="danger" onClick={() => { closeDetail(); openDecision(detail, 'rejected') }}>拒绝认证</Button>
        </>}
        {detail && detail.status !== 'pending' && <Button onClick={() => { closeDetail(); openDecision(detail, detail.status === 'approved' ? 'rejected' : 'approved') }}>改判结论</Button>}
      </>}
    >
      <IdentityDetailBody
        detail={detail}
        loading={detailLoading}
        error={detailError}
        onRefresh={() => setDetailRefreshKey(key => key + 1)}
      />
    </Drawer>

    <Modal
      open={Boolean(selected)}
      title={decisionTitle}
      description={selected ? `${identityRoleLabel(selected.role)} · ${selected.realName || `用户 #${selected.userId}`}` : ''}
      onClose={() => { if (!busy) setSelected(null) }}
      footer={<>
        <Button onClick={() => setSelected(null)} disabled={busy}>取消</Button>
        <Button variant={decision === 'rejected' ? 'danger' : 'primary'} disabled={busy} onClick={() => void submitDecision()}>{busy ? '提交中…' : decision === 'approved' ? '确认通过' : '确认拒绝'}</Button>
      </>}
    >
      <div className="preview-form-stack">
        <div className="preview-warning-box">
          <strong>认证结论会直接影响业务准入</strong>
          <p>{selected?.status === 'pending' ? '通过后账号即可进入对应工作台；拒绝结论会保留在审核记录中。' : '改判会覆盖当前结论，并保留可追溯的审核记录。'}</p>
        </div>
        <TextAreaField
          label={selected?.status === 'pending' ? '结论依据' : '改判依据'}
          required
          rows={5}
          value={reason}
          onChange={event => setReason(event.target.value)}
          placeholder="例如：证件信息一致，经营地址核验通过"
        />
        {decisionError && <div className="preview-form-error">{decisionError}</div>}
      </div>
    </Modal>
  </div>
}

function IdentityDetailBody({ detail, loading, error, onRefresh }: {
  detail: IdentityApplicationDetail | null
  loading: boolean
  error: string
  onRefresh: () => void
}) {
  if (loading && !detail) return <div className="preview-policy-note">正在加载认证资料…</div>
  if (error && !detail) return <div className="preview-form-error" role="alert">{error}</div>
  if (!detail) return <div className="preview-policy-note">请选择一条认证资料。</div>
  const proofs = detail.proofs ?? []
  const proofsExpired = proofs.length > 0 && proofs.every(proof => {
    const expiresAt = Date.parse(proof.expiresAt)
    return Number.isFinite(expiresAt) && expiresAt <= Date.now()
  })
  return <div className="preview-detail-stack">
    {error && <div className="preview-form-error" role="alert">{error}</div>}
    <div className="preview-detail-banner">
      <div><span>当前状态</span><strong>{identityStatusLabel(detail.status)}</strong></div>
      <StatusPill>{identityStatusLabel(detail.status)}</StatusPill>
    </div>
    <div className="preview-kv-grid">
      <div><span>申请角色</span><strong>{identityRoleLabel(detail.role)}</strong></div>
      <div><span>用户编号</span><strong>#{detail.userId}</strong></div>
      <div><span>联系方式</span><strong>{detail.phone || '—'}</strong></div>
      <div><span>商家名称</span><strong>{detail.merchantName || '—'}</strong></div>
      <div><span>经营/居住地址</span><strong>{detail.merchantAddress || '—'}</strong></div>
      <div><span>申请编号</span><strong>{detail.applicationId}</strong></div>
      <div><span>审核人</span><strong>{detail.reviewerUserId ? `#${detail.reviewerUserId}` : '未审核'}</strong></div>
      <div><span>审核时间</span><strong>{detail.reviewedAt ? displayTime(detail.reviewedAt) : '—'}</strong></div>
    </div>
    <div className="preview-note-block">
      <span>证明材料（{proofs.length || detail.assetIds.length}）</span>
      {proofs.length > 0 ? <RealImageStrip urls={proofs.map(proof => proof.url)} label="证明材料" />
        : <p>{detail.assetIds.length > 0 ? '证明链接已过期或暂不可用，请刷新后重试。' : '申请未附带证明材料。'}</p>}
      {proofsExpired && <p>证明链接已过期，请刷新后查看。</p>}
      <Button size="sm" variant="ghost" onClick={onRefresh} disabled={loading}>{loading ? '刷新中…' : '刷新证明材料'}</Button>
    </div>
    {detail.reviewReason && <div className="preview-note-block"><span>审核结论依据</span><p>{detail.reviewReason}</p></div>}
    <div className="preview-note-block">
      <span>审核历史（{detail.events.length}）</span>
      {detail.events.length === 0
        ? <p>暂无审核记录。</p>
        : <div className="preview-detail-stack">
          {detail.events.map(event => <div key={event.eventId} className="preview-note-block">
            <span>{identityEventLabel(event.eventType)} · {displayTime(event.createdAt)}</span>
            <p>状态：{identityStatusLabel(event.status)} · 操作人 #{event.actorUserId}{event.reason ? ` · ${event.reason}` : ''}</p>
          </div>)}
        </div>}
    </div>
  </div>
}

function FeedbackPeople() {
  const { notify } = usePreviewToast()
  const [status, setStatus] = useState<FeedbackFilter>('全部')
  const [page, setPage] = useState(1)
  const [data, setData] = useState<Page<FeedbackRow>>({ items: [], page: 1, pageSize: PAGE_SIZE, total: 0 })
  const [loading, setLoading] = useState(false)
  const [listError, setListError] = useState('')
  const [counts, setCounts] = useState<Record<FeedbackStatus, number> | null>(null)
  const [selected, setSelected] = useState<FeedbackRow | null>(null)
  const [nextStatus, setNextStatus] = useState<FeedbackStatus>('in_progress')
  const [note, setNote] = useState('')
  const [formError, setFormError] = useState('')
  const [busy, setBusy] = useState(false)
  const listRequestRef = useRef(0)
  const countsRequestRef = useRef(0)

  const loadList = useCallback(async () => {
    const requestId = ++listRequestRef.current
    setLoading(true)
    setListError('')
    try {
      const next = await adminAPI.feedbacks({ page, pageSize: PAGE_SIZE, status: status === '全部' ? undefined : status })
      if (requestId !== listRequestRef.current) return
      if (next.items.length === 0 && next.total > 0 && page > 1) {
        setPage(current => Math.max(1, current - 1))
        return
      }
      setData(next)
    } catch (error) {
      if (requestId === listRequestRef.current) setListError(friendlyError(error))
    } finally {
      if (requestId === listRequestRef.current) setLoading(false)
    }
  }, [page, status])

  const loadCounts = useCallback(async () => {
    const requestId = ++countsRequestRef.current
    try {
      const [open, inProgress, resolved, ignored] = await Promise.all([
        adminAPI.feedbacks({ page: 1, pageSize: 1, status: 'open' }),
        adminAPI.feedbacks({ page: 1, pageSize: 1, status: 'in_progress' }),
        adminAPI.feedbacks({ page: 1, pageSize: 1, status: 'resolved' }),
        adminAPI.feedbacks({ page: 1, pageSize: 1, status: 'ignored' })
      ])
      if (requestId !== countsRequestRef.current) return
      setCounts({ open: open.total, in_progress: inProgress.total, resolved: resolved.total, ignored: ignored.total })
    } catch {
      // 统计失败时保留上一次数据，不阻塞反馈列表。
    }
  }, [])

  useEffect(() => { void loadList() }, [loadList])
  useEffect(() => { void loadCounts() }, [loadCounts])

  const open = (item: FeedbackRow) => {
    const known = feedbackStatusValues.find(value => value === item.status)
    setSelected(item)
    setNextStatus(item.status === 'open' ? 'in_progress' : known ?? 'in_progress')
    setNote(item.adminNote || '')
    setFormError('')
  }

  const submit = async () => {
    if (!selected) return
    if (!note.trim()) {
      setFormError('请填写处理说明')
      return
    }
    if (!selected.id) {
      setFormError('反馈编号缺失，无法更新')
      return
    }
    setBusy(true)
    setFormError('')
    try {
      await adminAPI.updateFeedback(selected.id, nextStatus, note.trim())
      setSelected(null)
      notify('反馈状态与处理说明已更新')
      await Promise.all([loadList(), loadCounts()])
    } catch (error) {
      setFormError(friendlyError(error))
    } finally {
      setBusy(false)
    }
  }

  const totalFeedback = counts ? counts.open + counts.in_progress + counts.resolved + counts.ignored : '—'

  return <div>
    <PageHeader
      eyebrow="用户声音"
      title="意见反馈"
      description="每条反馈都应该有明确状态和处理说明，避免用户问题停留在“已记录”。"
      actions={<Button variant="primary" onClick={() => { setStatus('open'); setPage(1) }}>处理待响应反馈</Button>}
    />
    <section className="preview-stat-grid">
      <StatCard label="待处理" value={counts ? counts.open : '—'} meta="等待首次响应" tone="rose" />
      <StatCard label="处理中" value={counts ? counts.in_progress : '—'} meta="已有管理员跟进" tone="amber" />
      <StatCard label="已解决" value={counts ? counts.resolved : '—'} meta="已给出处理说明" tone="mint" />
      <StatCard label="全部反馈" value={totalFeedback} meta="用户问题与建议" tone="accent" />
    </section>
    <Panel eyebrow="VOICE OF USER" title="反馈处理" description="处理说明会保留在列表中，方便后续追溯；状态筛选走真实接口。">
      <div className="preview-toolbar">
        <Segmented
          value={status}
          onChange={value => { setStatus(value); setPage(1) }}
          options={[
            { value: '全部', label: '全部' },
            { value: 'open', label: '待处理' },
            { value: 'in_progress', label: '处理中' },
            { value: 'resolved', label: '已解决' },
            { value: 'ignored', label: '不处理' }
          ]}
        />
        <Button size="sm" variant="ghost" disabled={loading} onClick={() => { void loadList(); void loadCounts() }}>{loading ? '加载中…' : '刷新'}</Button>
      </div>
      {listError && <div className="preview-form-error" role="alert">{listError}</div>}
      {loading && data.items.length === 0
        ? <div className="preview-policy-note">正在加载反馈…</div>
        : data.items.length === 0
          ? <EmptyState title="没有匹配的反馈" description="调整筛选条件后再试。" />
          : <div className="preview-feedback-list">
            {data.items.map(item => <article key={item.id || `${item.userId}-${item.createdAt}`} className="is-clickable" onClick={() => open(item)}>
              <div className="preview-feedback-list__meta">
                <span><strong>用户 #{item.userId}</strong><small>{item.category} · {displayTime(item.createdAt)}</small></span>
                <StatusPill>{feedbackStatusText(item.status)}</StatusPill>
              </div>
              <p>{item.content}</p>
              {item.images?.length ? <div className="preview-image-strip" role="list" aria-label={`反馈附图 ${item.images.length} 张`}>
                {item.images.map(image => <a
                  key={`${image.assetId}-${image.slot}`}
                  role="listitem"
                  className="preview-image-thumb preview-image-thumb--real"
                  href={adminAssetContentUrl(image.assetId)}
                  target="_blank"
                  rel="noreferrer"
                  onClick={event => event.stopPropagation()}
                >
                  <RealImage src={adminAssetContentUrl(image.assetId)} alt={`反馈附图 ${image.slot + 1}`} />
                </a>)}
              </div> : null}
              {item.adminNote && <div className="preview-feedback-list__note">
                <span>处理说明{item.handledBy ? ` · 处理人 #${item.handledBy}` : ''}{item.handledAt ? ` · ${displayTime(item.handledAt)}` : ''}</span>
                {item.adminNote}
              </div>}
              <div className="preview-feedback-list__footer" onClick={event => event.stopPropagation()}>
                <span>{item.id || '—'}</span>
                <Button size="sm" variant="ghost" onClick={() => open(item)}>{item.status === 'resolved' ? '查看/更新' : '处理反馈'}</Button>
              </div>
            </article>)}
          </div>}
      <Pagination page={data.page} pageSize={data.pageSize} total={data.total} onChange={setPage} label="反馈列表分页" />
    </Panel>
    <Modal
      open={Boolean(selected)}
      title="更新反馈处理"
      description={selected ? `用户 #${selected.userId} · ${selected.category}` : ''}
      onClose={() => { if (!busy) setSelected(null) }}
      footer={<>
        <Button onClick={() => setSelected(null)} disabled={busy}>取消</Button>
        <Button variant="primary" onClick={() => void submit()} disabled={busy}>{busy ? '保存中…' : '保存处理'}</Button>
      </>}
    >
      <div className="preview-form-stack">
        <SelectField label="处理状态" value={nextStatus} onChange={event => setNextStatus(event.target.value as FeedbackStatus)}>
          {feedbackStatusValues.map(value => <option key={value} value={value}>{feedbackStatusLabel[value]}</option>)}
        </SelectField>
        <TextAreaField
          label="处理说明"
          required
          rows={5}
          value={note}
          onChange={event => setNote(event.target.value)}
          placeholder="说明已经做了什么、后续计划或为什么不处理"
          maxLength={500}
        />
        {formError && <div className="preview-form-error">{formError}</div>}
      </div>
    </Modal>
  </div>
}

interface CredentialDraft {
  mode: 'assign' | 'reset'
  userId: number | null
  nickname: string
  username: string
  password: string
}

function UserPeople() {
  const { notify } = usePreviewToast()
  const [page, setPage] = useState(1)
  const [data, setData] = useState<Page<UserRow>>({ items: [], page: 1, pageSize: PAGE_SIZE, total: 0 })
  const [total, setTotal] = useState<number | null>(null)
  const [loading, setLoading] = useState(false)
  const [listError, setListError] = useState('')
  const [query, setQuery] = useState('')
  const [role, setRole] = useState<'全部' | UserRow['role']>('全部')
  const [allUsers, setAllUsers] = useState<UserRow[] | null>(null)
  const [allUsersError, setAllUsersError] = useState('')
  const [credentials, setCredentials] = useState<AdminCredential[]>([])
  const [credentialsLoading, setCredentialsLoading] = useState(false)
  const [credentialsError, setCredentialsError] = useState('')
  const [credentialDraft, setCredentialDraft] = useState<CredentialDraft | null>(null)
  const [credentialFormError, setCredentialFormError] = useState('')
  const [credentialBusy, setCredentialBusy] = useState(false)
  const [pendingCredential, setPendingCredential] = useState<AdminCredential | null>(null)
  const [pendingStatus, setPendingStatus] = useState<{ user: UserRow; status: UserRow['status'] } | null>(null)
  const [pendingRole, setPendingRole] = useState<{ user: UserRow; role: UserRow['role'] } | null>(null)
  const [pendingPhone, setPendingPhone] = useState<UserRow | null>(null)
  const [actionError, setActionError] = useState('')
  const [actionBusy, setActionBusy] = useState(false)
  const usersRequestRef = useRef(0)
  const allUsersRequestRef = useRef(0)
  const credentialsRequestRef = useRef(0)

  const loadUsers = useCallback(async () => {
    const requestId = ++usersRequestRef.current
    setLoading(true)
    setListError('')
    try {
      const next = await adminAPI.users({ page, pageSize: PAGE_SIZE })
      if (requestId !== usersRequestRef.current) return
      setData(next)
      setTotal(next.total)
    } catch (error) {
      if (requestId === usersRequestRef.current) setListError(friendlyError(error))
    } finally {
      if (requestId === usersRequestRef.current) setLoading(false)
    }
  }, [page])

  const loadAllUsers = useCallback(async () => {
    const requestId = ++allUsersRequestRef.current
    try {
      const result = await fetchAllUsers()
      if (requestId !== allUsersRequestRef.current) return
      setAllUsers(result)
      setAllUsersError('')
    } catch (error) {
      if (requestId === allUsersRequestRef.current) setAllUsersError(friendlyError(error))
    }
  }, [])

  const loadCredentials = useCallback(async () => {
    const requestId = ++credentialsRequestRef.current
    setCredentialsLoading(true)
    setCredentialsError('')
    try {
      const result = await adminAuthAPI.credentials()
      if (requestId !== credentialsRequestRef.current) return
      setCredentials(result.items)
    } catch (error) {
      if (requestId !== credentialsRequestRef.current) return
      setCredentialsError(errorStatus(error) === 403 ? '仅超级管理员可以管理后台登录账号' : friendlyError(error))
    } finally {
      if (requestId === credentialsRequestRef.current) setCredentialsLoading(false)
    }
  }, [])

  useEffect(() => { void loadUsers() }, [loadUsers])
  useEffect(() => { void loadAllUsers() }, [loadAllUsers])
  useEffect(() => { void loadCredentials() }, [loadCredentials])

  const refreshUsers = async () => {
    await Promise.all([loadUsers(), loadAllUsers(), loadCredentials()])
  }

  const openStatus = (user: UserRow, status: UserRow['status']) => {
    setPendingStatus({ user, status })
    setActionError('')
  }

  const submitStatus = async () => {
    if (!pendingStatus) return
    setActionBusy(true)
    setActionError('')
    try {
      await adminAPI.updateUserStatus(pendingStatus.user.id, pendingStatus.status)
      notify(pendingStatus.status === 'active' ? '账号已恢复正常' : `账号已${userStatusLabel[pendingStatus.status]}`, pendingStatus.status === 'active' ? 'success' : 'info')
      setPendingStatus(null)
      await refreshUsers()
    } catch (error) {
      setActionError(friendlyError(error))
    } finally {
      setActionBusy(false)
    }
  }

  const openRole = (user: UserRow) => {
    setPendingRole({ user, role: user.role })
    setActionError('')
  }

  const submitRole = async () => {
    if (!pendingRole) return
    if (pendingRole.role === pendingRole.user.role) {
      setActionError('请选择与当前不同的角色')
      return
    }
    setActionBusy(true)
    setActionError('')
    try {
      await adminAPI.updateUserRole(pendingRole.user.id, pendingRole.role)
      notify(`角色已调整为「${userRoleLabel[pendingRole.role]}」`)
      setPendingRole(null)
      await refreshUsers()
    } catch (error) {
      setActionError(friendlyError(error))
    } finally {
      setActionBusy(false)
    }
  }

  const submitPhoneRemoval = async () => {
    if (!pendingPhone) return
    setActionBusy(true)
    setActionError('')
    try {
      await adminAPI.removeUserPhone(pendingPhone.id)
      notify('已解除该用户的手机号绑定')
      setPendingPhone(null)
      await refreshUsers()
    } catch (error) {
      setActionError(friendlyError(error))
    } finally {
      setActionBusy(false)
    }
  }

  const openAssignCredential = () => {
    setCredentialDraft({ mode: 'assign', userId: null, nickname: '', username: '', password: '' })
    setCredentialFormError('')
  }

  const openResetCredential = (credential: AdminCredential) => {
    setCredentialDraft({
      mode: 'reset',
      userId: credential.userId,
      nickname: credential.nickname,
      username: credential.username,
      password: ''
    })
    setCredentialFormError('')
  }

  const submitCredential = async () => {
    if (!credentialDraft) return
    const userId = credentialDraft.userId
    if (userId === null) {
      setCredentialFormError('请选择要分配登录账号的管理员用户')
      return
    }
    const username = credentialDraft.username.trim()
    if (!/^[A-Za-z0-9._]{3,64}$/.test(username)) {
      setCredentialFormError('用户名需为 3–64 位字母、数字、点或下划线')
      return
    }
    const passwordLength = [...credentialDraft.password].length
    if (passwordLength < 14 || passwordLength > 72) {
      setCredentialFormError('密码需为 14–72 位')
      return
    }
    setCredentialBusy(true)
    setCredentialFormError('')
    try {
      await adminAuthAPI.saveCredential(userId, username, credentialDraft.password)
      setCredentialDraft(null)
      notify(credentialDraft.mode === 'assign' ? '登录账号已分配' : '登录密码已重置，旧会话将失效', credentialDraft.mode === 'assign' ? 'success' : 'info')
      await loadCredentials()
    } catch (error) {
      setCredentialFormError(friendlyError(error))
    } finally {
      setCredentialBusy(false)
    }
  }

  const submitCredentialRemoval = async () => {
    if (!pendingCredential) return
    setActionBusy(true)
    setActionError('')
    try {
      await adminAuthAPI.removeCredential(pendingCredential.userId)
      notify('已移除该登录账号，对方将无法登录', 'info')
      setPendingCredential(null)
      await loadCredentials()
    } catch (error) {
      setActionError(friendlyError(error))
    } finally {
      setActionBusy(false)
    }
  }

  const credentialUserIds = useMemo(() => new Set(credentials.map(item => item.userId)), [credentials])
  const credentialCandidates = useMemo(() => (allUsers ?? []).filter(user =>
    (user.role === 'admin' || user.role === 'superadmin') && user.status === 'active' && !credentialUserIds.has(user.id)
  ), [allUsers, credentialUserIds])
  const statistics = useMemo(() => {
    if (!allUsers) return null
    return {
      normalUsers: allUsers.filter(user => user.role === 'user').length,
      admins: allUsers.filter(user => user.role === 'admin' || user.role === 'superadmin').length,
      restricted: allUsers.filter(user => user.status !== 'active').length
    }
  }, [allUsers])

  const filtered = useMemo(() => {
    const normalized = query.trim().toLowerCase()
    return data.items.filter(item => {
      if (role !== '全部' && item.role !== role) return false
      if (!normalized) return true
      const phone = item.phoneBound ? item.maskedPhone ?? '' : ''
      return `${item.nickname} ${item.school} ${item.campus} ${phone} #${item.id}`.toLowerCase().includes(normalized)
    })
  }, [data.items, query, role])

  const roleOptions: Array<{ value: '全部' | UserRow['role']; label: string }> = [
    { value: '全部', label: '全部' },
    { value: 'user', label: '普通用户' },
    { value: 'admin', label: '管理员' },
    { value: 'superadmin', label: '超级管理员' }
  ]

  return <div>
    <PageHeader
      eyebrow="账号与权限"
      title="用户与权限"
      description="账号状态直接影响用户能否互动和进入业务角色，操作前需要明确影响范围。"
      actions={<Button variant="primary" onClick={() => { setRole('admin'); setQuery('') }}>查看管理员</Button>}
    />
    <section className="preview-stat-grid">
      <StatCard label="账号总数" value={total ?? '—'} meta="真实注册账号" tone="accent" />
      <StatCard label="普通用户" value={statistics ? statistics.normalUsers : '—'} meta="社区与交易用户" tone="mint" />
      <StatCard label="管理员" value={statistics ? statistics.admins : '—'} meta="管理员与超级管理员" tone="amber" />
      <StatCard label="受限账号" value={statistics ? statistics.restricted : '—'} meta="禁言或停用" tone="rose" />
    </section>
    <Panel
      eyebrow="ADMIN CREDENTIAL"
      title="后台登录账号"
      description="只有被授予登录账号的管理员才能进入本后台；超级管理员可分配、重置或移除。"
      actions={<Button variant="primary" disabled={!allUsers || credentialsLoading || Boolean(credentialsError)} onClick={openAssignCredential}>分配登录账号</Button>}
    >
      {credentialsError && <div className="preview-form-error" role="alert">{credentialsError}</div>}
      {credentialsLoading && credentials.length === 0 && !credentialsError && <div className="preview-policy-note">正在加载后台登录账号…</div>}
      {!credentialsLoading && !credentialsError && credentials.length === 0
        ? <EmptyState title="暂无后台登录账号" description="可从管理员用户中分配登录账号。" />
        : credentials.length > 0 && <div className="preview-table-wrap"><table className="preview-table" aria-label="后台登录账号列表">
          <thead><tr><th>登录用户名</th><th>管理员</th><th>角色</th><th>状态</th><th>密码更新时间</th><th /></tr></thead>
          <tbody>{credentials.map(item => <tr key={item.credentialId}>
            <td><strong>{item.username}</strong><span>#{item.userId}</span></td>
            <td>{item.nickname || `用户 #${item.userId}`}</td>
            <td>{userRoleText(item.role)}</td>
            <td><StatusPill tone={item.locked ? 'danger' : 'success'}>{item.locked ? '已锁定' : '正常'}</StatusPill></td>
            <td>{displayTime(item.passwordSetAt)}</td>
            <td><div className="preview-row-actions">
              <Button size="sm" variant="ghost" onClick={() => openResetCredential(item)}>重置</Button>
              <Button size="sm" variant="danger" onClick={() => { setActionError(''); setPendingCredential(item) }}>移除</Button>
            </div></td>
          </tr>)}</tbody>
        </table></div>}
    </Panel>

    <Panel
      eyebrow="IDENTITY & ACCESS"
      title="账号工作队列"
      description="分页由真实接口驱动；搜索与角色筛选作用于当前页。"
    >
      <div className="preview-toolbar">
        <SearchBox value={query} onChange={setQuery} placeholder="搜索当前页姓名、手机号或编号" />
        <Segmented value={role} onChange={setRole} options={roleOptions} />
      </div>
      {listError && <div className="preview-form-error" role="alert">{listError}</div>}
      {allUsersError && <div className="preview-form-error" role="alert">账号统计加载失败：{allUsersError}</div>}
      {loading && data.items.length === 0
        ? <div className="preview-policy-note">正在加载用户列表…</div>
        : filtered.length === 0
          ? <EmptyState title="没有匹配的账号" description="调整搜索或角色筛选后再试。" />
          : <div className="preview-table-wrap">
            <table className="preview-table" aria-label="用户列表"><thead><tr><th>用户</th><th>角色</th><th>校区</th><th>状态</th><th>创建时间</th><th>操作</th></tr></thead><tbody>
              {filtered.map(item => <tr key={item.id}>
                <td><div className="preview-user-cell">
                  <span>{(item.nickname || '用').slice(0, 1)}</span>
                  <div>
                    <strong>{item.nickname || `用户 #${item.id}`}</strong>
                    <small>{item.phoneBound ? item.maskedPhone || '已绑定手机号' : '未绑定手机号'} · #{item.id}</small>
                  </div>
                </div></td>
                <td>{userRoleText(item.role)}</td>
                <td>{[item.school, item.campus].filter(Boolean).join(' · ') || '—'}</td>
                <td><StatusPill>{userStatusLabel[item.status]}</StatusPill></td>
                <td>{displayTime(item.createdAt)}</td>
                <td><div className="preview-row-actions">
                  {item.status !== 'active'
                    ? <Button size="sm" variant="ghost" onClick={() => openStatus(item, 'active')}>恢复</Button>
                    : <>
                      <Button size="sm" variant="quiet" onClick={() => openStatus(item, 'muted')}>禁言</Button>
                      <Button size="sm" variant="danger" onClick={() => openStatus(item, 'banned')}>停用</Button>
                    </>}
                  <Button size="sm" variant="ghost" onClick={() => openRole(item)}>调整角色</Button>
                  {item.phoneBound && <Button size="sm" variant="ghost" onClick={() => { setActionError(''); setPendingPhone(item) }}>解除手机号绑定</Button>}
                </div></td>
              </tr>)}
            </tbody></table>
          </div>}
      <Pagination page={data.page} pageSize={data.pageSize} total={data.total} onChange={setPage} label="用户列表分页" />
      <div className="preview-policy-note">
        <span>权限说明</span>
        <p>账号状态与角色变更会立即写入真实账号，并让该用户现有登录会话失效；仅超级管理员可以调整角色与后台登录账号。</p>
      </div>
    </Panel>

    <Modal
      open={Boolean(credentialDraft)}
      title={credentialDraft?.mode === 'reset' ? '重置登录密码' : '分配后台登录账号'}
      description={credentialDraft?.mode === 'reset'
        ? `管理员：${credentialDraft.nickname || `用户 #${credentialDraft.userId}`}`
        : '从管理员用户中选择对象并设置登录凭据。'}
      onClose={() => { if (!credentialBusy) { setCredentialDraft(null); setCredentialFormError('') } }}
      footer={<>
        <Button onClick={() => { setCredentialDraft(null); setCredentialFormError('') }} disabled={credentialBusy}>取消</Button>
        <Button variant="primary" onClick={() => void submitCredential()} disabled={credentialBusy}>{credentialBusy ? '保存中…' : '保存登录账号'}</Button>
      </>}
    >
      {credentialDraft && <div className="preview-form-stack">
        {credentialDraft.mode === 'assign'
          ? <SelectField
            label="管理员用户"
            required
            value={credentialDraft.userId === null ? '' : String(credentialDraft.userId)}
            disabled={credentialBusy}
            onChange={event => {
              const user = credentialCandidates.find(entry => String(entry.id) === event.target.value)
              setCredentialDraft(current => current ? { ...current, userId: user?.id ?? null, nickname: user?.nickname ?? '' } : current)
            }}
          >
            <option value="">请选择管理员用户</option>
            {credentialCandidates.map(user => <option key={user.id} value={user.id}>#{user.id} {user.nickname || '未命名用户'}</option>)}
          </SelectField>
          : <Field label="管理员" value={credentialDraft.nickname || `用户 #${credentialDraft.userId}`} readOnly disabled />}
        {credentialDraft.mode === 'assign' && credentialCandidates.length === 0 && <div className="preview-policy-note">暂无可分配登录账号的管理员用户。请先在用户列表将目标用户角色调整为管理员或超级管理员。</div>}
        <Field
          label="登录用户名"
          required
          value={credentialDraft.username}
          onChange={event => setCredentialDraft(current => current ? { ...current, username: event.target.value } : current)}
          placeholder="例如：ops.zhang"
          hint="3–64 位字母、数字、点或下划线"
          disabled={credentialBusy}
        />
        <Field
          label={credentialDraft.mode === 'assign' ? '初始密码' : '新密码'}
          required
          type="password"
          value={credentialDraft.password}
          onChange={event => setCredentialDraft(current => current ? { ...current, password: event.target.value } : current)}
          placeholder="14–72 位"
          hint="14–72 位字符"
          disabled={credentialBusy}
        />
        {credentialFormError && <div className="preview-form-error">{credentialFormError}</div>}
      </div>}
    </Modal>

    <Modal
      open={Boolean(pendingStatus)}
      title={pendingStatus ? `将账号改为「${userStatusLabel[pendingStatus.status]}」` : ''}
      description={pendingStatus ? `${pendingStatus.user.nickname || `用户 #${pendingStatus.user.id}`} · #${pendingStatus.user.id}` : ''}
      onClose={() => { if (!actionBusy) setPendingStatus(null) }}
      footer={<>
        <Button onClick={() => setPendingStatus(null)} disabled={actionBusy}>取消</Button>
        <Button variant={pendingStatus?.status === 'active' ? 'primary' : 'danger'} onClick={() => void submitStatus()} disabled={actionBusy}>{actionBusy ? '提交中…' : '确认修改'}</Button>
      </>}
    >
      <div className="preview-form-stack">
        <div className="preview-warning-box">
          <strong>{pendingStatus?.status === 'active' ? '恢复后用户可以继续使用全部功能' : '该操作会立即影响用户登录与互动'}</strong>
          <p>操作会写入真实账号状态；停用或禁言会让该用户现有登录会话失效。</p>
        </div>
        {actionError && <div className="preview-form-error">{actionError}</div>}
      </div>
    </Modal>

    <Modal
      open={Boolean(pendingRole)}
      title={pendingRole ? `调整 ${pendingRole.user.nickname || `用户 #${pendingRole.user.id}`} 的角色` : ''}
      description={pendingRole ? `当前角色：${userRoleLabel[pendingRole.user.role]}` : ''}
      onClose={() => { if (!actionBusy) setPendingRole(null) }}
      footer={<>
        <Button onClick={() => setPendingRole(null)} disabled={actionBusy}>取消</Button>
        <Button variant="primary" onClick={() => void submitRole()} disabled={actionBusy || pendingRole?.role === pendingRole?.user.role}>{actionBusy ? '提交中…' : '确认调整'}</Button>
      </>}
    >
      {pendingRole && <div className="preview-form-stack">
        <SelectField label="新角色" value={pendingRole.role} onChange={event => setPendingRole({ ...pendingRole, role: event.target.value as UserRow['role'] })}>
          <option value="user">普通用户</option>
          <option value="admin">管理员</option>
          <option value="superadmin">超级管理员</option>
        </SelectField>
        <div className="preview-warning-box">
          <strong>角色变更会立即生效</strong>
          <p>调整角色会让该用户现有登录会话失效；仅超级管理员可以执行此操作。</p>
        </div>
        {actionError && <div className="preview-form-error">{actionError}</div>}
      </div>}
    </Modal>

    <Modal
      open={Boolean(pendingPhone)}
      title="解除手机号绑定"
      description={pendingPhone ? `${pendingPhone.nickname || `用户 #${pendingPhone.id}`} · ${pendingPhone.maskedPhone || '已绑定手机号'}` : ''}
      onClose={() => { if (!actionBusy) setPendingPhone(null) }}
      footer={<>
        <Button onClick={() => setPendingPhone(null)} disabled={actionBusy}>取消</Button>
        <Button variant="danger" onClick={() => void submitPhoneRemoval()} disabled={actionBusy}>{actionBusy ? '提交中…' : '确认解除'}</Button>
      </>}
    >
      <div className="preview-form-stack">
        <div className="preview-warning-box">
          <strong>解除后该手机号将不再关联此账号</strong>
          <p>用户需要重新绑定手机号后才能使用短信登录或找回账号。</p>
        </div>
        {actionError && <div className="preview-form-error">{actionError}</div>}
      </div>
    </Modal>

    <Modal
      open={Boolean(pendingCredential)}
      title="移除登录账号"
      description={pendingCredential ? `账号：${pendingCredential.username} · ${pendingCredential.nickname || `用户 #${pendingCredential.userId}`}` : ''}
      onClose={() => { if (!actionBusy) setPendingCredential(null) }}
      footer={<>
        <Button onClick={() => setPendingCredential(null)} disabled={actionBusy}>取消</Button>
        <Button variant="danger" onClick={() => void submitCredentialRemoval()} disabled={actionBusy}>{actionBusy ? '提交中…' : '确认移除'}</Button>
      </>}
    >
      <div className="preview-form-stack">
        <div className="preview-warning-box">
          <strong>移除后该管理员将立即失去后台访问权限</strong>
          <p>操作会吊销该账号的全部后台会话。</p>
        </div>
        {actionError && <div className="preview-form-error">{actionError}</div>}
      </div>
    </Modal>
  </div>
}
