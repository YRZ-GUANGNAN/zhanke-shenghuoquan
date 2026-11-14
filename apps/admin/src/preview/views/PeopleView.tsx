import { useMemo, useState } from 'react'
import { formatTime, type FeedbackItem, type IdentityItem, type PreviewSection, type UserItem } from '../data'
import { usePreviewStore } from '../store'
import {
  Button,
  EmptyState,
  Modal,
  PageHeader,
  Panel,
  SearchBox,
  Segmented,
  SelectField,
  StatCard,
  StatusPill,
  TextAreaField,
  usePreviewToast
} from '../ui'

type PeopleSection = Extract<PreviewSection, 'identities' | 'feedbacks' | 'users'>

export function PeopleView({ section }: { section: PeopleSection }) {
  if (section === 'identities') return <IdentityPeople />
  if (section === 'feedbacks') return <FeedbackPeople />
  return <UserPeople />
}

function IdentityPeople() {
  const identities = usePreviewStore(state => state.identities)
  const decideIdentity = usePreviewStore(state => state.decideIdentity)
  const [status, setStatus] = useState<'全部' | IdentityItem['status']>('全部')
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState<IdentityItem | null>(null)
  const [decision, setDecision] = useState<IdentityItem['status']>('已通过')
  const [reason, setReason] = useState('')
  const [error, setError] = useState('')
  const { notify } = usePreviewToast()

  const filtered = identities.filter(item =>
    (status === '全部' || item.status === status)
    && (!query.trim() || `${item.applicant}${item.role}${item.business}`.toLowerCase().includes(query.trim().toLowerCase()))
  )

  const openDecision = (item: IdentityItem, next: IdentityItem['status']) => {
    setSelected(item)
    setDecision(next)
    setReason('')
    setError('')
  }

  const submit = () => {
    if (!selected) return
    if (!reason.trim()) {
      setError('请填写认证结论依据')
      return
    }
    decideIdentity(selected.id, decision, reason.trim())
    setSelected(null)
    notify(decision === '已通过' ? '认证资料已通过' : '认证资料已拒绝，原因已记录', decision === '已通过' ? 'success' : 'info')
  }

  return <div>
    <PageHeader
      eyebrow="身份与准入"
      title="商家骑手认证"
      description="把材料、风险和结论放在同一个处理面板中，避免认证信息散落在不同列表。"
      actions={<Button variant="primary" onClick={() => setStatus('待认证')}>查看待认证资料</Button>}
    />
    <section className="preview-stat-grid">
      <StatCard label="待认证" value={identities.filter(item => item.status === '待认证').length} meta="商家与骑手合计" tone="amber" />
      <StatCard label="商家申请" value={identities.filter(item => item.role === '商家').length} meta="含新驻与变更" tone="accent" />
      <StatCard label="骑手申请" value={identities.filter(item => item.role === '骑手').length} meta="配送服务准入" tone="mint" />
      <StatCard label="已通过" value={identities.filter(item => item.status === '已通过').length} meta="可正常接单/营业" tone="ink" />
    </section>
    <Panel
      eyebrow="TRUST & ACCESS"
      title="认证工作队列"
      description="认证结论只影响本预览数据，不会修改真实账号权限。"
      actions={<SearchBox value={query} onChange={setQuery} placeholder="搜索申请人或业务信息" />}
    >
      <div className="preview-toolbar">
        <Segmented value={status} onChange={setStatus} options={[{ value: '全部', label: '全部' }, { value: '待认证', label: '待认证' }, { value: '已通过', label: '已通过' }, { value: '已拒绝', label: '已拒绝' }]} />
      </div>
      {filtered.length === 0 ? <EmptyState title="没有匹配的认证资料" description="调整筛选条件后再试。" /> : <div className="preview-review-list">
        {filtered.map(item => <article key={item.id} className="preview-review-row">
          <span className="preview-review-row__kind">{item.role}</span>
          <div className="preview-review-row__copy">
            <div><strong>{item.applicant}</strong><StatusPill>{item.status}</StatusPill></div>
            <p>{item.business}</p>
            <small>{item.id} · {formatTime(item.submittedAt)}{item.reason ? ` · 结论依据：${item.reason}` : ''}</small>
          </div>
          <div className="preview-row-actions">
            {item.status === '待认证' ? <>
              <Button size="sm" variant="primary" onClick={() => openDecision(item, '已通过')}>通过</Button>
              <Button size="sm" variant="danger" onClick={() => openDecision(item, '已拒绝')}>拒绝</Button>
            </> : <Button size="sm" variant="ghost" onClick={() => openDecision(item, item.status === '已通过' ? '已拒绝' : '已通过')}>复核</Button>}
          </div>
        </article>)}
      </div>}
    </Panel>
    <Modal
      open={Boolean(selected)}
      title={decision === '已通过' ? '通过认证' : '拒绝认证'}
      description={selected ? `${selected.role} · ${selected.applicant}` : ''}
      onClose={() => setSelected(null)}
      footer={<>
        <Button onClick={() => setSelected(null)}>取消</Button>
        <Button variant={decision === '已拒绝' ? 'danger' : 'primary'} onClick={submit}>{decision === '已通过' ? '确认通过' : '确认拒绝'}</Button>
      </>}
    >
      <div className="preview-form-stack">
        <div className="preview-warning-box">
          <strong>认证会影响业务准入</strong>
          <p>本操作只修改预览状态；生产环境还需要材料核验和权限审核。</p>
        </div>
        <TextAreaField label="结论依据" required rows={5} value={reason} onChange={event => setReason(event.target.value)} placeholder="例如：证件信息一致，经营地址核验通过" />
        {error && <div className="preview-form-error">{error}</div>}
      </div>
    </Modal>
  </div>
}

function FeedbackPeople() {
  const feedbacks = usePreviewStore(state => state.feedbacks)
  const updateFeedbackStatus = usePreviewStore(state => state.updateFeedbackStatus)
  const [status, setStatus] = useState<'全部' | FeedbackItem['status']>('全部')
  const [selected, setSelected] = useState<FeedbackItem | null>(null)
  const [nextStatus, setNextStatus] = useState<FeedbackItem['status']>('处理中')
  const [note, setNote] = useState('')
  const [error, setError] = useState('')
  const { notify } = usePreviewToast()
  const filtered = feedbacks.filter(item => status === '全部' || item.status === status)

  const open = (item: FeedbackItem) => {
    setSelected(item)
    setNextStatus(item.status === '待处理' ? '处理中' : '已解决')
    setNote(item.note || '')
    setError('')
  }
  const submit = () => {
    if (!selected) return
    if (!note.trim()) {
      setError('请填写处理说明')
      return
    }
    updateFeedbackStatus(selected.id, nextStatus, note.trim())
    setSelected(null)
    notify('反馈状态与处理说明已更新')
  }

  return <div>
    <PageHeader
      eyebrow="用户声音"
      title="意见反馈"
      description="每条反馈都应该有明确状态和处理说明，避免用户问题停留在“已记录”。"
      actions={<Button variant="primary" onClick={() => setStatus('待处理')}>处理待响应反馈</Button>}
    />
    <section className="preview-stat-grid">
      <StatCard label="待处理" value={feedbacks.filter(item => item.status === '待处理').length} meta="等待首次响应" tone="rose" />
      <StatCard label="处理中" value={feedbacks.filter(item => item.status === '处理中').length} meta="已有管理员跟进" tone="amber" />
      <StatCard label="已解决" value={feedbacks.filter(item => item.status === '已解决').length} meta="已给出处理说明" tone="mint" />
      <StatCard label="全部反馈" value={feedbacks.length} meta="用户问题与建议" tone="accent" />
    </section>
    <Panel eyebrow="VOICE OF USER" title="反馈处理" description="处理说明会保留在列表中，方便后续追溯。">
      <div className="preview-toolbar"><Segmented value={status} onChange={setStatus} options={[{ value: '全部', label: '全部' }, { value: '待处理', label: '待处理' }, { value: '处理中', label: '处理中' }, { value: '已解决', label: '已解决' }, { value: '不处理', label: '不处理' }]} /></div>
      {filtered.length === 0 ? <EmptyState title="没有匹配的反馈" description="调整筛选条件后再试。" /> : <div className="preview-feedback-list">
        {filtered.map(item => <article key={item.id}>
          <div className="preview-feedback-list__meta"><span><strong>{item.user}</strong><small>{item.category} · {formatTime(item.createdAt)}</small></span><StatusPill>{item.status}</StatusPill></div>
          <p>{item.content}</p>
          {item.note && <div className="preview-feedback-list__note"><span>处理说明</span>{item.note}</div>}
          <div className="preview-feedback-list__footer"><span>{item.id}</span><Button size="sm" variant="ghost" onClick={() => open(item)}>{item.status === '已解决' ? '查看/更新' : '处理反馈'}</Button></div>
        </article>)}
      </div>}
    </Panel>
    <Modal
      open={Boolean(selected)}
      title="更新反馈处理"
      description={selected ? `${selected.user} · ${selected.category}` : ''}
      onClose={() => setSelected(null)}
      footer={<>
        <Button onClick={() => setSelected(null)}>取消</Button>
        <Button variant="primary" onClick={submit}>保存处理</Button>
      </>}
    >
      <div className="preview-form-stack">
        <SelectField label="处理状态" value={nextStatus} onChange={event => setNextStatus(event.target.value as FeedbackItem['status'])}>
          <option>待处理</option><option>处理中</option><option>已解决</option><option>不处理</option>
        </SelectField>
        <TextAreaField label="处理说明" required rows={5} value={note} onChange={event => setNote(event.target.value)} placeholder="说明已经做了什么、后续计划或为什么不处理" />
        {error && <div className="preview-form-error">{error}</div>}
      </div>
    </Modal>
  </div>
}

function UserPeople() {
  const users = usePreviewStore(state => state.users)
  const updateUserStatus = usePreviewStore(state => state.updateUserStatus)
  const [query, setQuery] = useState('')
  const [role, setRole] = useState<'全部' | UserItem['role']>('全部')
  const { notify } = usePreviewToast()
  const filtered = useMemo(() => users.filter(item =>
    (role === '全部' || item.role === role)
    && (!query.trim() || `${item.name}${item.phone}${item.id}`.toLowerCase().includes(query.trim().toLowerCase()))
  ), [query, role, users])

  return <div>
    <PageHeader
      eyebrow="账号与权限"
      title="用户与权限"
      description="账号状态直接影响用户能否互动和进入业务角色，操作前需要明确影响范围。"
      actions={<Button variant="primary" onClick={() => setRole('管理员')}>查看管理员</Button>}
    />
    <section className="preview-stat-grid">
      <StatCard label="账号总数" value={users.length} meta="当前演示账号" tone="accent" />
      <StatCard label="普通用户" value={users.filter(item => item.role === '普通用户').length} meta="社区与交易用户" tone="mint" />
      <StatCard label="商家/骑手" value={users.filter(item => item.role === '商家' || item.role === '骑手').length} meta="业务角色账号" tone="amber" />
      <StatCard label="受限账号" value={users.filter(item => item.status !== '正常').length} meta="禁言或停用" tone="rose" />
    </section>
    <Panel eyebrow="IDENTITY & ACCESS" title="账号工作队列" description="账号状态操作会即时反馈，并在本浏览器的本地数据中保留。">
      <div className="preview-toolbar">
        <SearchBox value={query} onChange={setQuery} placeholder="搜索姓名、手机号或编号" />
        <Segmented value={role} onChange={setRole} options={[{ value: '全部', label: '全部' }, { value: '普通用户', label: '普通用户' }, { value: '商家', label: '商家' }, { value: '骑手', label: '骑手' }, { value: '管理员', label: '管理员' }]} />
      </div>
      {filtered.length === 0 ? <EmptyState title="没有匹配的账号" description="调整搜索或角色筛选后再试。" /> : <div className="preview-table-wrap">
        <table className="preview-table" aria-label="用户列表"><thead><tr><th>用户</th><th>角色</th><th>校区</th><th>状态</th><th>创建时间</th><th>操作</th></tr></thead><tbody>
          {filtered.map(item => <tr key={item.id}>
            <td><div className="preview-user-cell"><span>{item.name.slice(0, 1)}</span><div><strong>{item.name}</strong><small>{item.phone} · {item.id}</small></div></div></td>
            <td>{item.role}</td><td>{item.campus}</td><td><StatusPill>{item.status}</StatusPill></td><td>{item.createdAt}</td>
            <td><div className="preview-row-actions">
              {item.status !== '正常' ? <Button size="sm" variant="ghost" onClick={() => { updateUserStatus(item.id, '正常'); notify('账号已恢复正常') }}>恢复</Button> : <>
                <Button size="sm" variant="quiet" onClick={() => { updateUserStatus(item.id, '禁言'); notify('账号已禁言', 'info') }}>禁言</Button>
                <Button size="sm" variant="danger" onClick={() => { updateUserStatus(item.id, '停用'); notify('账号已停用', 'info') }}>停用</Button>
              </>}
            </div></td>
          </tr>)}
        </tbody></table>
      </div>}
      <div className="preview-policy-note">
        <span>权限说明</span>
        <p>管理员角色修改属于高风险操作。本地原型的“停用/禁言”只影响演示表，不会写入真实账号。</p>
      </div>
    </Panel>
  </div>
}
