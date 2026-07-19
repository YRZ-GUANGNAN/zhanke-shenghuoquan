import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { AdminAPIError, adminAPI } from './services/f-admin'
import type {
  IdentityApplicationAdminList,
  IdentityApplicationDetail,
  IdentityApplicationQuery,
  IdentityApplicationRole,
  IdentityApplicationStatus,
} from './types'

const pageSize = 20
const emptyPage = (): IdentityApplicationAdminList => ({ items: [], page: 1, pageSize, total: 0 })
const roleLabel = (role: IdentityApplicationRole) => role === 'rider' ? '骑手' : '商家'
const statusLabel = (status: IdentityApplicationStatus) => ({ pending: '待审核', approved: '已通过', rejected: '已拒绝' })[status]
const eventLabel = (event: 'submitted' | 'resubmitted' | 'approved' | 'rejected') => ({ submitted: '已提交', resubmitted: '已重新提交', approved: '已通过', rejected: '已拒绝' })[event]
const badgeStatus = (status: IdentityApplicationStatus) => ({ pending: 'pending', approved: 'active', rejected: 'failed' })[status]
const displayTime = (value?: string) => value ? new Date(value).toLocaleString('zh-CN') : '—'
const errorMessage = (reason: unknown) => reason instanceof Error ? reason.message : '操作失败，请重试'

function Panel({ title, actions, children }: { title: string; actions?: ReactNode; children: ReactNode }) {
  return <section className='panel delivery-panel'><header className='delivery-header'><h2>{title}</h2>{actions}</header>{children}</section>
}

function Fact({ label, value }: { label: string; value: ReactNode }) {
  return <div className='delivery-fact'><span>{label}</span><strong>{value || '—'}</strong></div>
}

export function IdentityApplications() {
  const [data, setData] = useState<IdentityApplicationAdminList>(emptyPage)
  const [role, setRole] = useState<IdentityApplicationRole | ''>('')
  const [status, setStatus] = useState<IdentityApplicationStatus | ''>('pending')
  const [page, setPage] = useState(1)
  const [selected, setSelected] = useState<IdentityApplicationDetail | null>(null)
  const [reason, setReason] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    const query: IdentityApplicationQuery = { page, pageSize }
    if (role) query.role = role
    if (status) query.status = status
    try {
      setData(await adminAPI.listIdentityApplications(query))
    } catch (loadError) {
      setError(errorMessage(loadError))
    } finally {
      setLoading(false)
    }
  }, [page, role, status])

  const inspect = useCallback(async (applicationId: string) => {
    setError('')
    try {
      const detail = await adminAPI.identityApplication(applicationId)
      setSelected(detail)
      setReason('')
    } catch (detailError) {
      setError(errorMessage(detailError))
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const refreshAfterConflict = async (applicationId: string) => {
    await load()
    try {
      setSelected(await adminAPI.identityApplication(applicationId))
      setReason('')
    } catch {
      setSelected(null)
    }
  }

  const decide = async (action: 'approve' | 'reject') => {
    if (!selected || selected.status !== 'pending') return
    const normalizedReason = reason.trim()
    if (action === 'reject' && !normalizedReason) {
      setError('拒绝原因不能为空')
      return
    }
    if (action === 'approve' && !window.confirm(`确认通过用户 #${selected.userId} 的${roleLabel(selected.role)}申请吗？`)) return
    setBusy(true)
    setError('')
    try {
      if (action === 'approve') {
        await adminAPI.approveIdentityApplication(selected.applicationId, selected.rowVersion)
      } else {
        await adminAPI.rejectIdentityApplication(selected.applicationId, selected.rowVersion, normalizedReason)
      }
      await load()
      setSelected(await adminAPI.identityApplication(selected.applicationId))
      setReason('')
    } catch (decisionError) {
      if (decisionError instanceof AdminAPIError && decisionError.status === 409) {
        await refreshAfterConflict(selected.applicationId)
        setError('该申请已被其他管理员处理')
      } else {
        setError(errorMessage(decisionError))
      }
    } finally {
      setBusy(false)
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
    {selected && <ApplicationDetail value={selected} reason={reason} setReason={setReason} busy={busy} close={() => { setSelected(null); setReason('') }} decide={decide} />}
  </div>
}

function ApplicationDetail({ value, reason, setReason, busy, close, decide }: {
  value: IdentityApplicationDetail
  reason: string
  setReason: (value: string) => void
  busy: boolean
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

    {value.status === 'pending' && <div className='identity-decision'>
      <label>拒绝原因<textarea value={reason} maxLength={500} placeholder='拒绝时必填' onChange={event => setReason(event.target.value)} /></label>
      <div className='delivery-row-actions'>
        <button disabled={busy} onClick={() => void decide('approve')}>{busy ? '处理中…' : '通过'}</button>
        <button className='danger' disabled={busy} onClick={() => void decide('reject')}>{busy ? '处理中…' : '拒绝'}</button>
      </div>
    </div>}
  </aside>
}
