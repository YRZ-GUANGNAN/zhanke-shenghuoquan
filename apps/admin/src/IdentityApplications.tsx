import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { AdminAPIError, adminAPI } from './services/f-admin'
import { useAdminDialog } from './components'
import type {
  IdentityApplicationAdminList,
  IdentityApplicationDetail,
  IdentityApplicationQuery,
  IdentityApplicationRole,
  IdentityApplicationRow,
  IdentityApplicationStatus,
} from './types'

const pageSize = 20
const proofRefreshLeadTimeMs = 5_000
const proofExpiredMessage = '证明链接已过期，请重新加载'
const emptyPage = (): IdentityApplicationAdminList => ({ items: [], page: 1, pageSize, total: 0 })
const roleLabel = (role: IdentityApplicationRole) => role === 'rider' ? '骑手' : '商家'
const statusLabel = (status: IdentityApplicationStatus) => ({ pending: '待审核', approved: '已通过', rejected: '已拒绝' })[status]
const eventLabel = (event: 'submitted' | 'resubmitted' | 'approved' | 'rejected') => ({ submitted: '已提交', resubmitted: '已重新提交', approved: '已通过', rejected: '已拒绝' })[event]
const badgeStatus = (status: IdentityApplicationStatus) => ({ pending: 'pending', approved: 'active', rejected: 'failed' })[status]
const displayTime = (value?: string) => value ? new Date(value).toLocaleString('zh-CN') : '—'
const errorMessage = (reason: unknown) => reason instanceof Error ? reason.message : '操作失败，请重试'

export function earliestProofExpiry(proofs: IdentityApplicationDetail['proofs']): number | null {
  if (!proofs?.length) return null
  return proofs.reduce((earliest, proof) => {
    const value = Date.parse(proof.expiresAt)
    return Math.min(earliest, Number.isFinite(value) ? value : 0)
  }, Number.POSITIVE_INFINITY)
}

export function mergeDecisionRow(current: IdentityApplicationDetail | null, decision: IdentityApplicationRow): IdentityApplicationDetail | null {
  if (!current || current.applicationId !== decision.applicationId) return current
  return { ...current, ...decision, proofs: [...(current.proofs ?? [])], events: current.events }
}

function usableDetail(value: IdentityApplicationDetail, now = Date.now()): { value: IdentityApplicationDetail; expired: boolean } {
  const expiresAt = earliestProofExpiry(value.proofs)
  if (expiresAt !== null && expiresAt <= now + proofRefreshLeadTimeMs) {
    return { value: { ...value, proofs: [] }, expired: true }
  }
  return { value, expired: false }
}

function Panel({ title, actions, children }: { title: string; actions?: ReactNode; children: ReactNode }) {
  return <section className='panel delivery-panel'><header className='delivery-header'><h2>{title}</h2>{actions}</header>{children}</section>
}

function Fact({ label, value }: { label: string; value: ReactNode }) {
  return <div className='delivery-fact'><span>{label}</span><strong>{value || '—'}</strong></div>
}

export function IdentityApplications() {
  const dialog = useAdminDialog()
  const [data, setData] = useState<IdentityApplicationAdminList>(emptyPage)
  const [role, setRole] = useState<IdentityApplicationRole | ''>('')
  const [status, setStatus] = useState<IdentityApplicationStatus | ''>('pending')
  const [page, setPage] = useState(1)
  const [selected, setSelected] = useState<IdentityApplicationDetail | null>(null)
  const [reason, setReason] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [busy, setBusy] = useState(false)
  const [conflictLockedApplicationId, setConflictLockedApplicationId] = useState<string | null>(null)
  const mountedRef = useRef(true)
  const listRequestSequenceRef = useRef(0)
  const detailRequestSequenceRef = useRef(0)
  const decisionOperationSequenceRef = useRef(0)
  const selectionGenerationRef = useRef(0)
  const selectedApplicationIdRef = useRef<string | null>(null)
  const conflictLockedApplicationIdRef = useRef<string | null>(null)
  const proofRefreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const loadLatestRef = useRef<(options?: { silent?: boolean }) => Promise<boolean>>(async () => false)

  const clearProofRefreshTimer = useCallback(() => {
    if (proofRefreshTimerRef.current !== null) {
      clearTimeout(proofRefreshTimerRef.current)
      proofRefreshTimerRef.current = null
    }
  }, [])

  const lockConflictDecision = useCallback((applicationId: string) => {
    conflictLockedApplicationIdRef.current = applicationId
    if (mountedRef.current) setConflictLockedApplicationId(applicationId)
  }, [])

  const clearConflictDecisionLock = useCallback((applicationId: string) => {
    if (conflictLockedApplicationIdRef.current !== applicationId) return
    conflictLockedApplicationIdRef.current = null
    if (mountedRef.current) setConflictLockedApplicationId(current => current === applicationId ? null : current)
  }, [])

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
      listRequestSequenceRef.current += 1
      detailRequestSequenceRef.current += 1
      decisionOperationSequenceRef.current += 1
      selectionGenerationRef.current += 1
      selectedApplicationIdRef.current = null
      conflictLockedApplicationIdRef.current = null
      clearProofRefreshTimer()
    }
  }, [clearProofRefreshTimer])

  const load = useCallback(async ({ silent = false }: { silent?: boolean } = {}): Promise<boolean> => {
    if (!mountedRef.current) return false
    const requestId = ++listRequestSequenceRef.current
    setLoading(true)
    if (!silent) setError('')
    const query: IdentityApplicationQuery = { page, pageSize }
    if (role) query.role = role
    if (status) query.status = status
    try {
      const next = await adminAPI.listIdentityApplications(query)
      if (!mountedRef.current || requestId !== listRequestSequenceRef.current) return true
      setData(next)
      return true
    } catch (loadError) {
      if (!mountedRef.current || requestId !== listRequestSequenceRef.current) return true
      if (!silent) setError(errorMessage(loadError))
      return false
    } finally {
      if (mountedRef.current && requestId === listRequestSequenceRef.current) setLoading(false)
    }
  }, [page, role, status])

  loadLatestRef.current = load

  const fetchDetail = useCallback(async (
    applicationId: string,
    mode: 'inspect' | 'decision' | 'proof' | 'conflict',
    decision?: IdentityApplicationRow,
    continuationIsCurrent?: () => boolean,
    onResolved?: (detail: IdentityApplicationDetail) => void,
  ): Promise<boolean> => {
    if (!mountedRef.current || selectedApplicationIdRef.current !== applicationId) return false
    clearProofRefreshTimer()
    const requestId = ++detailRequestSequenceRef.current
    try {
      const detail = await adminAPI.identityApplication(applicationId)
      if (!mountedRef.current || requestId !== detailRequestSequenceRef.current || selectedApplicationIdRef.current !== applicationId || (continuationIsCurrent && !continuationIsCurrent())) return true
      const prepared = usableDetail(detail)
      setSelected(current => {
        if (mode !== 'inspect' && current?.applicationId !== applicationId) return current
        if (mode === 'decision' && decision) {
          const refreshed = detail.status === 'pending'
            ? { ...detail, ...decision, proofs: detail.proofs, events: detail.events }
            : detail
          return usableDetail(refreshed).value
        }
        return prepared.value
      })
      if (prepared.expired) setError(proofExpiredMessage)
      else if (mode === 'proof') setError(current => current === proofExpiredMessage ? '' : current)
      onResolved?.(detail)
      return true
    } catch (detailError) {
      if (!mountedRef.current || requestId !== detailRequestSequenceRef.current || selectedApplicationIdRef.current !== applicationId || (continuationIsCurrent && !continuationIsCurrent())) return true
      if (mode === 'proof') {
        setSelected(current => current?.applicationId === applicationId ? { ...current, proofs: [] } : current)
        setError(proofExpiredMessage)
      } else if (mode === 'inspect') {
        selectedApplicationIdRef.current = null
        setError(errorMessage(detailError))
      }
      return false
    }
  }, [clearProofRefreshTimer])

  const inspect = useCallback((applicationId: string) => {
    if (!mountedRef.current) return Promise.resolve(false)
    const previousApplicationId = selectedApplicationIdRef.current
    if (previousApplicationId) clearConflictDecisionLock(previousApplicationId)
    selectionGenerationRef.current += 1
    decisionOperationSequenceRef.current += 1
    setBusy(false)
    selectedApplicationIdRef.current = applicationId
    detailRequestSequenceRef.current += 1
    clearProofRefreshTimer()
    setSelected(null)
    setReason('')
    setError('')
    return fetchDetail(applicationId, 'inspect')
  }, [clearConflictDecisionLock, clearProofRefreshTimer, fetchDetail])

  useEffect(() => { void load() }, [load])

  useEffect(() => {
    clearProofRefreshTimer()
    if (!mountedRef.current || !selected) return
    const expiresAt = earliestProofExpiry(selected.proofs)
    if (expiresAt === null) return
    const applicationId = selected.applicationId
    const delay = Math.max(0, expiresAt - Date.now() - proofRefreshLeadTimeMs)
    proofRefreshTimerRef.current = setTimeout(() => {
      if (!mountedRef.current || selectedApplicationIdRef.current !== applicationId) return
      setSelected(current => current?.applicationId === applicationId ? { ...current, proofs: [] } : current)
      void fetchDetail(applicationId, 'proof')
    }, delay)
    return clearProofRefreshTimer
  }, [clearProofRefreshTimer, fetchDetail, selected])

  const closeDetail = () => {
    if (!mountedRef.current) return
    const applicationId = selectedApplicationIdRef.current
    if (applicationId) clearConflictDecisionLock(applicationId)
    selectionGenerationRef.current += 1
    decisionOperationSequenceRef.current += 1
    detailRequestSequenceRef.current += 1
    selectedApplicationIdRef.current = null
    clearProofRefreshTimer()
    setBusy(false)
    setSelected(null)
    setReason('')
  }

  const decide = async (action: 'approve' | 'reject') => {
    if (!mountedRef.current || !selected || selected.status !== 'pending' || selectedApplicationIdRef.current !== selected.applicationId || conflictLockedApplicationIdRef.current === selected.applicationId) return
    const applicationId = selected.applicationId
    const selectionGeneration = selectionGenerationRef.current
    const decisionOperation = ++decisionOperationSequenceRef.current
    const decisionIsCurrent = () => mountedRef.current
      && decisionOperationSequenceRef.current === decisionOperation
      && selectionGenerationRef.current === selectionGeneration
      && selectedApplicationIdRef.current === applicationId
    const silentlyRefreshLatestList = () => {
      if (mountedRef.current) void loadLatestRef.current({ silent: true })
    }
    const normalizedReason = reason.trim()
    if (action === 'reject' && !normalizedReason) {
      setError('拒绝原因不能为空')
      return
    }
    if (action === 'approve' && !await dialog.confirm({ title: '通过身份申请', description: `确认通过用户 #${selected.userId} 的${roleLabel(selected.role)}申请吗？通过后用户即可进入对应工作台。`, confirmLabel: '确认通过' })) return
    if (action === 'reject' && !await dialog.confirm({ title: '拒绝身份申请', description: `确认拒绝用户 #${selected.userId} 的${roleLabel(selected.role)}申请吗？拒绝原因将用于审核记录。`, confirmLabel: '确认拒绝', tone: 'danger' })) return
    setBusy(true)
    setError('')
    try {
      const decisionResult = action === 'approve'
        ? await adminAPI.approveIdentityApplication(applicationId, selected.rowVersion)
        : await adminAPI.rejectIdentityApplication(applicationId, selected.rowVersion, normalizedReason)
      if (!mountedRef.current) return
      if (!decisionIsCurrent()) {
        silentlyRefreshLatestList()
        return
      }
      setSelected(current => mergeDecisionRow(current, decisionResult))
      setReason('')
      const [listRefreshed, detailRefreshed] = await Promise.all([
        loadLatestRef.current({ silent: true }),
        fetchDetail(decisionResult.applicationId, 'decision', decisionResult, decisionIsCurrent),
      ])
      if (!mountedRef.current || !decisionIsCurrent()) return
      if (!listRefreshed || !detailRefreshed) setError('审核已完成，但刷新失败，请重新加载')
    } catch (decisionError) {
      if (!mountedRef.current) return
      if (!decisionIsCurrent()) {
        silentlyRefreshLatestList()
        return
      }
      if (decisionError instanceof AdminAPIError && decisionError.status === 409) {
        lockConflictDecision(applicationId)
        const [listRefreshed, detailRefreshed] = await Promise.all([
          loadLatestRef.current({ silent: true }),
          fetchDetail(applicationId, 'conflict', undefined, decisionIsCurrent, () => clearConflictDecisionLock(applicationId)),
        ])
        if (!mountedRef.current || !decisionIsCurrent()) return
        setError(listRefreshed && detailRefreshed ? '该申请已被其他管理员处理' : '该申请已被其他管理员处理，刷新失败，请重新加载')
      } else {
        setError(errorMessage(decisionError))
      }
    } finally {
      if (mountedRef.current && decisionIsCurrent()) setBusy(false)
    }
  }

  const pageCount = Math.max(1, Math.ceil(data.total / data.pageSize))
  return <div className='identity-console'>
    <Panel title='身份申请审核' actions={<button className='delivery-secondary' disabled={loading} onClick={() => void load()}>{loading ? '加载中…' : '刷新'}</button>}>
      <div className='delivery-controls'>
        <label>申请角色<select value={role} onChange={event => { setPage(1); setRole(event.target.value as IdentityApplicationRole | '') }}><option value=''>全部</option><option value='rider'>骑手</option><option value='merchant'>商家</option></select></label>
        <label>审核状态<select value={status} onChange={event => { setPage(1); setStatus(event.target.value as IdentityApplicationStatus | '') }}><option value=''>全部</option><option value='pending'>待审核</option><option value='approved'>已通过</option><option value='rejected'>已拒绝</option></select></label>
      </div>
      {error && <div className='delivery-notice' role='alert'><strong>{error}</strong></div>}
      <div className='table-scroll'><table><thead><tr><th>申请编号</th><th>用户号</th><th>角色</th><th>申请人 / 商家</th><th>提交时间</th><th>状态</th></tr></thead><tbody>
        {data.items.map(item => <tr key={item.applicationId}>
          <td><button className='link-button' onClick={() => void inspect(item.applicationId)}>{item.applicationId}</button></td>
          <td>#{item.userId}</td>
          <td>{roleLabel(item.role)}</td>
          <td><strong>{item.realName}</strong><span>{item.role === 'merchant' ? item.merchantName : item.phone}</span></td>
          <td>{displayTime(item.submittedAt)}</td>
          <td><span className={`delivery-badge delivery-badge--${badgeStatus(item.status)}`}>{statusLabel(item.status)}</span></td>
        </tr>)}
        {!loading && data.items.length === 0 && <tr><td colSpan={6}>暂无符合条件的身份申请</td></tr>}
      </tbody></table></div>
      <div className='delivery-pagination'>
        <button disabled={data.page <= 1 || loading} onClick={() => setPage(data.page - 1)}>上一页</button>
        <span>第 {data.page} / {pageCount} 页，共 {data.total} 条</span>
        <button disabled={data.page >= pageCount || loading} onClick={() => setPage(data.page + 1)}>下一页</button>
      </div>
    </Panel>
    {selected && <ApplicationDetail value={selected} reason={reason} setReason={setReason} busy={busy} decisionLocked={conflictLockedApplicationId === selected.applicationId} close={closeDetail} decide={decide} />}
  </div>
}

function ApplicationDetail({ value, reason, setReason, busy, decisionLocked, close, decide }: {
  value: IdentityApplicationDetail
  reason: string
  setReason: (value: string) => void
  busy: boolean
  decisionLocked: boolean
  close: () => void
  decide: (action: 'approve' | 'reject') => Promise<void>
}) {
  return <aside className='panel delivery-detail identity-detail'>
    <header><div><span className='eyebrow'>申请编号</span><h2>{value.applicationId}</h2></div><button onClick={close}>关闭</button></header>
    <div className='detail-grid'>
      <Fact label='用户号' value={`#${value.userId}`} />
      <Fact label='角色' value={roleLabel(value.role)} />
      <Fact label='状态' value={statusLabel(value.status)} />
      <Fact label='真实姓名' value={value.realName} />
      <Fact label='联系电话' value={value.phone} />
      {value.role === 'merchant' && <><Fact label='商家名称' value={value.merchantName} /><Fact label='商家地址' value={value.merchantAddress} /></>}
      <Fact label='提交时间' value={displayTime(value.submittedAt)} />
      <Fact label='审核时间' value={displayTime(value.reviewedAt)} />
      <Fact label='审核管理员' value={value.reviewerUserId ? `#${value.reviewerUserId}` : '—'} />
      {value.status === 'rejected' && <Fact label='拒绝原因' value={value.reviewReason} />}
    </div>

    <h4>证明图片</h4>
    <div className='identity-proofs'>
      {(value.proofs ?? []).map(proof => <a key={proof.assetId} href={proof.url} target='_blank' rel='noreferrer'>
        <img src={proof.url} alt={`证明图片 ${proof.assetId}`} />
        <span>{proof.assetId}</span><small>链接有效至 {displayTime(proof.expiresAt)}</small>
      </a>)}
      {(value.proofs ?? []).length === 0 && <p>暂无可查看的证明图片</p>}
    </div>

    <h4>审核历史</h4>
    <div className='delivery-timeline identity-events'>
      {value.events.map(event => <div key={event.eventId}><span>{displayTime(event.createdAt)}</span><strong>{eventLabel(event.eventType)} · {statusLabel(event.status)}</strong><span>操作用户 #{event.actorUserId}{event.reason ? ` · ${event.reason}` : ''}</span></div>)}
      {value.events.length === 0 && <p>暂无审核记录</p>}
    </div>

    {value.status === 'pending' && !decisionLocked && <div className='identity-decision'>
      <label>拒绝原因<textarea value={reason} maxLength={500} placeholder='拒绝时必填' onChange={event => setReason(event.target.value)} /></label>
      <div className='delivery-row-actions'>
        <button disabled={busy} onClick={() => void decide('approve')}>{busy ? '处理中…' : '通过'}</button>
        <button className='danger' disabled={busy} onClick={() => void decide('reject')}>{busy ? '处理中…' : '拒绝'}</button>
      </div>
    </div>}
  </aside>
}
