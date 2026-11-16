import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { AdminAPIError, adminAPI } from './services/f-admin'
import { useAdminDialog } from './components'
import type {
  StudentVerificationAdminList,
  StudentVerificationDetail,
  StudentVerificationQuery,
  StudentVerificationRow,
  StudentVerificationStatus,
} from './types'

const pageSize = 20
const proofRefreshLeadTimeMs = 5_000
const proofExpiredMessage = '证明链接已过期，请重新加载'
const emptyPage = (): StudentVerificationAdminList => ({ items: [], page: 1, pageSize, total: 0 })
const statusLabel = (status: StudentVerificationStatus) => ({
  pending: '待审核',
  approved: '已认证',
  rejected: '未通过',
  cancelled: '已解除'
})[status]
const badgeStatus = (status: StudentVerificationStatus) => ({
  pending: 'pending',
  approved: 'active',
  rejected: 'failed',
  cancelled: 'cancelled'
})[status]
const eventLabel = (event: StudentVerificationDetail['events'][number]['eventType']) => ({
  submitted: '已提交',
  resubmitted: '已重新提交',
  approved: '已认证',
  rejected: '未通过',
  cancel_requested: '申请取消',
  cancel_approved: '已解除绑定'
})[event]
const displayTime = (value?: string) => value ? new Date(value).toLocaleString('zh-CN') : '—'
const errorMessage = (reason: unknown) => reason instanceof Error ? reason.message : '操作失败，请重试'

function earliestProofExpiry(proofs: StudentVerificationDetail['proofs']): number | null {
  if (!proofs?.length) return null
  return proofs.reduce((earliest, proof) => {
    const value = Date.parse(proof.expiresAt)
    return Math.min(earliest, Number.isFinite(value) ? value : 0)
  }, Number.POSITIVE_INFINITY)
}

function usableDetail(value: StudentVerificationDetail, now = Date.now()): { value: StudentVerificationDetail; expired: boolean } {
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

export function StudentVerifications() {
  const dialog = useAdminDialog()
  const [data, setData] = useState<StudentVerificationAdminList>(emptyPage)
  const [status, setStatus] = useState<StudentVerificationStatus | ''>('pending')
  const [page, setPage] = useState(1)
  const [selected, setSelected] = useState<StudentVerificationDetail | null>(null)
  const [reason, setReason] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [busy, setBusy] = useState(false)
  const [conflictLockedId, setConflictLockedId] = useState<string | null>(null)
  const mountedRef = useRef(true)
  const listRequestSequenceRef = useRef(0)
  const detailRequestSequenceRef = useRef(0)
  const selectionGenerationRef = useRef(0)
  const selectedIdRef = useRef<string | null>(null)
  const conflictLockedIdRef = useRef<string | null>(null)
  const proofRefreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const loadLatestRef = useRef<(options?: { silent?: boolean }) => Promise<boolean>>(async () => false)

  const clearProofRefreshTimer = useCallback(() => {
    if (proofRefreshTimerRef.current !== null) {
      clearTimeout(proofRefreshTimerRef.current)
      proofRefreshTimerRef.current = null
    }
  }, [])

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
      listRequestSequenceRef.current += 1
      detailRequestSequenceRef.current += 1
      selectionGenerationRef.current += 1
      selectedIdRef.current = null
      conflictLockedIdRef.current = null
      clearProofRefreshTimer()
    }
  }, [clearProofRefreshTimer])

  const load = useCallback(async ({ silent = false }: { silent?: boolean } = {}): Promise<boolean> => {
    if (!mountedRef.current) return false
    const requestId = ++listRequestSequenceRef.current
    setLoading(true)
    if (!silent) setError('')
    const query: StudentVerificationQuery = { page, pageSize }
    if (status) query.status = status
    try {
      const next = await adminAPI.listStudentVerifications(query)
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
  }, [page, status])

  loadLatestRef.current = load

  const fetchDetail = useCallback(async (applicationId: string, mode: 'inspect' | 'proof'): Promise<boolean> => {
    if (!mountedRef.current || selectedIdRef.current !== applicationId) return false
    clearProofRefreshTimer()
    const requestId = ++detailRequestSequenceRef.current
    try {
      const detail = await adminAPI.studentVerification(applicationId)
      if (!mountedRef.current || requestId !== detailRequestSequenceRef.current || selectedIdRef.current !== applicationId) return true
      const prepared = usableDetail(detail)
      setSelected(current => {
        if (mode === 'proof' && current?.applicationId !== applicationId) return current
        return prepared.value
      })
      if (prepared.expired) setError(proofExpiredMessage)
      else if (mode === 'proof') setError(current => current === proofExpiredMessage ? '' : current)
      return true
    } catch (detailError) {
      if (!mountedRef.current || requestId !== detailRequestSequenceRef.current || selectedIdRef.current !== applicationId) return true
      if (mode === 'proof') {
        setSelected(current => current?.applicationId === applicationId ? { ...current, proofs: [] } : current)
        setError(proofExpiredMessage)
      } else {
        selectedIdRef.current = null
        setError(errorMessage(detailError))
      }
      return false
    }
  }, [clearProofRefreshTimer])

  const inspect = useCallback((applicationId: string) => {
    if (!mountedRef.current) return
    selectionGenerationRef.current += 1
    selectedIdRef.current = applicationId
    conflictLockedIdRef.current = null
    setConflictLockedId(null)
    setBusy(false)
    setSelected(null)
    setReason('')
    setError('')
    void fetchDetail(applicationId, 'inspect')
  }, [fetchDetail])

  const closeDetail = () => {
    selectionGenerationRef.current += 1
    detailRequestSequenceRef.current += 1
    selectedIdRef.current = null
    conflictLockedIdRef.current = null
    clearProofRefreshTimer()
    setSelected(null)
    setReason('')
    setBusy(false)
  }

  useEffect(() => { void load() }, [load])

  useEffect(() => {
    clearProofRefreshTimer()
    if (!mountedRef.current || !selected) return
    const expiresAt = earliestProofExpiry(selected.proofs)
    if (expiresAt === null) return
    const applicationId = selected.applicationId
    const delay = Math.max(0, expiresAt - Date.now() - proofRefreshLeadTimeMs)
    proofRefreshTimerRef.current = setTimeout(() => {
      if (!mountedRef.current || selectedIdRef.current !== applicationId) return
      setSelected(current => current?.applicationId === applicationId ? { ...current, proofs: [] } : current)
      void fetchDetail(applicationId, 'proof')
    }, delay)
    return clearProofRefreshTimer
  }, [clearProofRefreshTimer, fetchDetail, selected])

  const applyDecision = async (row: StudentVerificationRow) => {
    const applicationId = row.applicationId
    setReason('')
    const refreshed = await fetchDetail(applicationId, 'inspect')
    if (!mountedRef.current) return
    setSelected(current => current?.applicationId === applicationId ? current : current)
    if (!refreshed) setError('操作已完成，但详情刷新失败，请重新加载')
    void loadLatestRef.current({ silent: true })
  }

  const handleConflict = async (applicationId: string, message: string) => {
    conflictLockedIdRef.current = applicationId
    setConflictLockedId(applicationId)
    const [listRefreshed, detailRefreshed] = await Promise.all([
      loadLatestRef.current({ silent: true }),
      fetchDetail(applicationId, 'inspect'),
    ])
    if (!mountedRef.current) return
    setError(listRefreshed && detailRefreshed ? message : `${message}，刷新失败，请重新加载`)
  }

  const decide = async (action: 'approve' | 'reject') => {
    if (!mountedRef.current || !selected || selected.status !== 'pending' || selectedIdRef.current !== selected.applicationId || conflictLockedIdRef.current === selected.applicationId) return
    const applicationId = selected.applicationId
    const generation = selectionGenerationRef.current
    const normalizedReason = reason.trim()
    if (action === 'reject' && !normalizedReason) {
      setError('驳回原因不能为空')
      return
    }
    if (action === 'approve' && !await dialog.confirm({
      title: '通过学生认证',
      description: `确认通过用户 #${selected.userId}（${selected.realName}）的学生认证吗？`,
      confirmLabel: '确认通过'
    })) return
    if (action === 'reject' && !await dialog.confirm({
      title: '驳回学生认证',
      description: `确认驳回用户 #${selected.userId}（${selected.realName}）的认证申请吗？驳回原因将展示给用户。`,
      confirmLabel: '确认驳回',
      tone: 'danger'
    })) return
    setBusy(true)
    setError('')
    try {
      const row = action === 'approve'
        ? await adminAPI.approveStudentVerification(applicationId, selected.rowVersion)
        : await adminAPI.rejectStudentVerification(applicationId, selected.rowVersion, normalizedReason)
      if (!mountedRef.current || selectionGenerationRef.current !== generation) return
      await applyDecision(row)
    } catch (decisionError) {
      if (!mountedRef.current || selectionGenerationRef.current !== generation) return
      if (decisionError instanceof AdminAPIError && decisionError.status === 409) {
        await handleConflict(applicationId, '该申请已被其他管理员处理')
      } else {
        setError(errorMessage(decisionError))
      }
    } finally {
      if (mountedRef.current) setBusy(false)
    }
  }

  const approveCancel = async () => {
    if (!mountedRef.current || !selected || selected.status !== 'approved' || !selected.cancelRequested || conflictLockedIdRef.current === selected.applicationId) return
    const applicationId = selected.applicationId
    const generation = selectionGenerationRef.current
    const note = reason.trim()
    if (!await dialog.confirm({
      title: '处理取消申请',
      description: `确认解除用户 #${selected.userId}（${selected.realName}）的学生认证吗？解除后用户可重新提交认证。`,
      confirmLabel: '确认解除',
      tone: 'danger'
    })) return
    setBusy(true)
    setError('')
    try {
      const row = await adminAPI.approveCancelStudentVerification(applicationId, selected.rowVersion, note)
      if (!mountedRef.current || selectionGenerationRef.current !== generation) return
      await applyDecision(row)
    } catch (decisionError) {
      if (!mountedRef.current || selectionGenerationRef.current !== generation) return
      if (decisionError instanceof AdminAPIError && decisionError.status === 409) {
        await handleConflict(applicationId, '该申请已被其他管理员处理')
      } else {
        setError(errorMessage(decisionError))
      }
    } finally {
      if (mountedRef.current) setBusy(false)
    }
  }

  const pageCount = Math.max(1, Math.ceil(data.total / data.pageSize))
  return <div className='identity-console'>
    <Panel title='学生认证审核' actions={<button className='delivery-secondary' disabled={loading} onClick={() => void load()}>{loading ? '加载中…' : '刷新'}</button>}>
      <div className='delivery-controls'>
        <label>认证状态<select value={status} onChange={event => { setPage(1); setStatus(event.target.value as StudentVerificationStatus | '') }}>
          <option value=''>全部</option>
          <option value='pending'>待审核</option>
          <option value='approved'>已认证</option>
          <option value='rejected'>未通过</option>
          <option value='cancelled'>已解除</option>
        </select></label>
      </div>
      {error && <div className='delivery-notice' role='alert'><strong>{error}</strong></div>}
      <div className='table-scroll'><table><thead><tr><th>申请编号</th><th>用户 ID</th><th>申请人</th><th>学号</th><th>提交时间</th><th>状态</th></tr></thead><tbody>
        {data.items.map(item => <tr key={item.applicationId}>
          <td><button className='link-button' onClick={() => inspect(item.applicationId)}>{item.applicationId}</button></td>
          <td>#{item.userId}</td>
          <td><strong>{item.realName}</strong><span>{item.college || '未填写学院'}</span></td>
          <td>{item.studentNo}</td>
          <td>{displayTime(item.submittedAt)}</td>
          <td>
            <span className={`delivery-badge delivery-badge--${badgeStatus(item.status)}`}>{statusLabel(item.status)}</span>
            {item.status === 'approved' && item.cancelRequested && <span className='delivery-badge delivery-badge--pending' style={{ marginLeft: 6 }}>取消申请中</span>}
          </td>
        </tr>)}
        {!loading && data.items.length === 0 && <tr><td colSpan={6}>暂无符合条件的学生认证</td></tr>}
      </tbody></table></div>
      <div className='delivery-pagination'>
        <button disabled={data.page <= 1 || loading} onClick={() => setPage(data.page - 1)}>上一页</button>
        <span>第 {data.page} / {pageCount} 页，共 {data.total} 条</span>
        <button disabled={data.page >= pageCount || loading} onClick={() => setPage(data.page + 1)}>下一页</button>
      </div>
    </Panel>
    {selected && <aside className='panel delivery-detail identity-detail'>
      <header><div><span className='eyebrow'>认证编号</span><h2>{selected.applicationId}</h2></div><button onClick={closeDetail}>关闭</button></header>
      <div className='detail-grid'>
        <Fact label='用户号' value={`#${selected.userId}`} />
        <Fact label='状态' value={statusLabel(selected.status)} />
        <Fact label='真实姓名' value={selected.realName} />
        <Fact label='学号' value={selected.studentNo} />
        <Fact label='学院' value={selected.college || '未填写'} />
        <Fact label='提交时间' value={displayTime(selected.submittedAt)} />
        <Fact label='审核时间' value={displayTime(selected.reviewedAt)} />
        <Fact label='审核管理员' value={selected.reviewerUserId ? `#${selected.reviewerUserId}` : '—'} />
        {selected.cancelRequested && <Fact label='取消申请时间' value={displayTime(selected.cancelRequestedAt)} />}
        {selected.reviewReason && <Fact label={selected.status === 'cancelled' ? '处理说明' : '驳回原因'} value={selected.reviewReason} />}
      </div>

      <h4>学生证照片</h4>
      <div className='identity-proofs'>
        {(selected.proofs ?? []).map(proof => <a key={proof.assetId} href={proof.url} target='_blank' rel='noreferrer'>
          <img src={proof.url} alt={`学生证照片 ${proof.assetId}`} />
          <span>{proof.assetId}</span><small>链接有效至 {displayTime(proof.expiresAt)}</small>
        </a>)}
        {(selected.proofs ?? []).length === 0 && <p>暂无可查看的学生证照片</p>}
      </div>

      <h4>审核历史</h4>
      <div className='delivery-timeline identity-events'>
        {selected.events.map(event => <div key={event.eventId}>
          <span>{displayTime(event.createdAt)}</span>
          <strong>{eventLabel(event.eventType)} · {statusLabel(event.status)}</strong>
          <span>操作用户 #{event.actorUserId}{event.reason ? ` · ${event.reason}` : ''}</span>
        </div>)}
        {selected.events.length === 0 && <p>暂无审核记录</p>}
      </div>

      {selected.status === 'pending' && conflictLockedId !== selected.applicationId && <div className='identity-decision'>
        <label>审核依据<textarea value={reason} maxLength={500} placeholder='驳回时必填，通过可不填' onChange={event => setReason(event.target.value)} /></label>
        <div className='delivery-row-actions'>
          <button disabled={busy} onClick={() => void decide('approve')}>{busy ? '处理中…' : '通过'}</button>
          <button className='danger' disabled={busy} onClick={() => void decide('reject')}>{busy ? '处理中…' : '驳回'}</button>
        </div>
      </div>}
      {selected.status === 'approved' && selected.cancelRequested && conflictLockedId !== selected.applicationId && <div className='identity-decision'>
        <label>处理说明<textarea value={reason} maxLength={500} placeholder='可填写解除原因，将记入审核历史' onChange={event => setReason(event.target.value)} /></label>
        <div className='delivery-row-actions'>
          <button className='danger' disabled={busy} onClick={() => void approveCancel()}>{busy ? '处理中…' : '解除绑定'}</button>
        </div>
      </div>}
      {selected.status === 'approved' && !selected.cancelRequested && <p>用户尚未申请取消绑定；如需解除，等待用户提交申请后在此处理。</p>}
    </aside>}
  </div>
}
