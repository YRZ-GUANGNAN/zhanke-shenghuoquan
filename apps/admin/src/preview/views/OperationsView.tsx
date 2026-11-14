import { useMemo, useState } from 'react'
import {
  formatMoney,
  formatTime,
  isToday,
  type CommunityItem,
  type PreviewSection,
  type ProductItem,
  type ReportItem,
  type ReviewItem
} from '../data'
import { usePreviewStore } from '../store'
import {
  Button,
  Drawer,
  EmptyState,
  ImageCell,
  ImageStrip,
  Modal,
  PageHeader,
  Panel,
  SearchBox,
  Segmented,
  Pagination,
  StatCard,
  StatusPill,
  TextAreaField,
  usePagedList,
  usePreviewToast,
  formatWait
} from '../ui'

type OperationSection = Extract<PreviewSection, 'media' | 'moderation' | 'community' | 'trade' | 'reports'>

// 只对待处理任务展示积压时长，口径与总览页一致。
function waitingLabel(createdAt: string, status: ReviewItem['status']): string {
  if (status !== '待审核' && status !== '审核中') return ''
  const elapsed = formatWait(createdAt)
  return elapsed ? `已等待 ${elapsed}` : ''
}


interface OperationMeta {
  eyebrow: string
  title: string
  description: string
  action: string
}

const metaBySection: Record<OperationSection, OperationMeta> = {
  media: {
    eyebrow: '内容基础设施',
    title: '素材中心',
    description: '检查图片与视频是否已经完成技术处理、审核和业务引用。',
    action: '检查异常素材'
  },
  moderation: {
    eyebrow: '统一审核流',
    title: '审核中心',
    description: '从提交、审核到结论，每一步都记录下来源、风险和原因。',
    action: '处理高风险任务'
  },
  community: {
    eyebrow: '社区运营',
    title: '社区内容',
    description: '查看内容发布状态，并将审核结论连接到公开或下架状态。',
    action: '查看待审内容'
  },
  trade: {
    eyebrow: '交易内容',
    title: '二手商品',
    description: '商品业务状态与审核状态分开管理，下架动作会保留原因。',
    action: '查看出售中商品'
  },
  reports: {
    eyebrow: '用户安全',
    title: '举报处理',
    description: '优先处理影响交易安全与社区体验的举报。',
    action: '处理待办举报'
  }
}

export function OperationsView({ section }: { section: OperationSection }) {
  if (section === 'media') return <MediaOperations />
  if (section === 'moderation') return <ReviewOperations />
  if (section === 'community') return <CommunityOperations />
  if (section === 'trade') return <TradeOperations />
  return <ReportOperations />
}

function MediaOperations() {
  const media = usePreviewStore(state => state.media)
  const setMediaStatus = usePreviewStore(state => state.setMediaStatus)
  const [filter, setFilter] = useState<'全部' | '待审核' | '异常' | '已通过'>('全部')
  const [tab, setTab] = useState<'assets' | 'decisions' | 'events' | 'gc' | 'deadletters'>('assets')
  const [detailId, setDetailId] = useState<string | null>(null)
  const decisions = usePreviewStore(state => state.mediaDecisions)
  const events = usePreviewStore(state => state.mediaEvents)
  const gcJobs = usePreviewStore(state => state.mediaGc)
  const { notify } = usePreviewToast()
  const detail = media.find(item => item.id === detailId) || null
  const filtered = media.filter(item => filter === '全部' || item.status === filter)
  const pagedMedia = usePagedList(filtered, 8)

  return <div>
    <PageHeader
      eyebrow={metaBySection.media.eyebrow}
      title={metaBySection.media.title}
      description={metaBySection.media.description}
      actions={<Button variant="primary" onClick={() => setFilter('异常')}>{metaBySection.media.action}</Button>}
    />
    <div className="preview-subtabs" role="tablist" aria-label="素材模块">
      {([['assets', '素材'], ['decisions', '审核记录'], ['events', '处理记录'], ['gc', '清理记录'], ['deadletters', '异常记录']] as const).map(([key, label]) => <button
        key={key}
        type="button"
        role="tab"
        aria-selected={tab === key}
        className={tab === key ? 'active' : ''}
        onClick={() => setTab(key)}
      >{label}</button>)}
    </div>

    {tab === 'assets' && <><section className="preview-stat-grid">
      <StatCard label="素材总数" value={media.length} meta="图片与视频" tone="accent" />
      <StatCard label="待审核" value={media.filter(item => item.status === '待审核').length} meta="审核后才会对外展示" tone="amber" />
      <StatCard label="异常" value={media.filter(item => item.status === '异常').length} meta="需要重新处理" tone="rose" />
      <StatCard label="已通过" value={media.filter(item => item.status === '已通过').length} meta="可被业务引用" tone="mint" />
    </section>
    <Panel
      eyebrow="ASSET REVIEW"
      title="素材工作区"
      description="异常素材可以在本地重新标记为待审核，模拟处理闭环。"
      actions={<Segmented value={filter} onChange={setFilter} options={[
        { value: '全部', label: '全部' },
        { value: '待审核', label: '待审核' },
        { value: '异常', label: '异常' },
        { value: '已通过', label: '已通过' }
      ]} />}
    >
      {filtered.length === 0 ? <EmptyState title="当前筛选下没有素材" description="调整状态筛选后再试。" /> : <div className="preview-media-grid">
        {pagedMedia.slice.map((asset, index) => <article key={asset.id} className="preview-media-card">
          <button type="button" className={`preview-media-card__visual preview-media-card__visual--${index % 4}`} onClick={() => setDetailId(asset.id)} aria-label={`查看 ${asset.name} 详情`}>
            <span>{asset.kind === '视频' ? '▶' : '▧'}</span>
            <small>{asset.kind}</small>
          </button>
          <div className="preview-media-card__body">
            <div><strong>{asset.name}</strong><StatusPill>{asset.status}</StatusPill></div>
            <dl><div><dt>用途</dt><dd>{asset.purpose}</dd></div><div><dt>大小</dt><dd>{asset.size}</dd></div><div><dt>编号</dt><dd>{asset.id}</dd></div><div><dt>时间</dt><dd>{formatTime(asset.createdAt)}</dd></div></dl>
            {asset.status === '异常'
              ? <Button size="sm" variant="primary" onClick={() => { setMediaStatus(asset.id, '待审核'); notify('素材已重新进入审核队列') }}>重新处理</Button>
              : asset.status === '待审核'
                ? <Button size="sm" variant="ghost" onClick={() => { setMediaStatus(asset.id, '已通过'); notify('素材审核已通过') }}>标记通过</Button>
                : <span className="preview-muted-action">已完成审核</span>}
          </div>
        </article>)}
      </div>}
      <Pagination page={pagedMedia.page} pageSize={pagedMedia.pageSize} total={pagedMedia.total} onChange={pagedMedia.setPage} label="素材列表分页" />
    </Panel>
    </>}

    <Drawer
      open={Boolean(detail)}
      onClose={() => setDetailId(null)}
      title={detail?.name || '素材详情'}
      description={detail ? `${detail.kind} · ${detail.purpose} · ${formatTime(detail.createdAt)}` : undefined}
      footer={detail && <>
        <Button onClick={() => setDetailId(null)}>关闭</Button>
        {detail.status === '异常' && <Button variant="primary" onClick={() => { setMediaStatus(detail.id, '待审核'); notify('素材已重新进入审核队列') }}>重新处理</Button>}
        {detail.status === '待审核' && <Button variant="primary" onClick={() => { setMediaStatus(detail.id, '已通过'); notify('素材审核已通过') }}>标记通过</Button>}
      </>}
    >
      {detail && <div className="preview-detail-stack">
        <div className="preview-detail-banner">
          <div><span>当前状态</span><strong>{detail.status}</strong></div>
          <StatusPill>{detail.status}</StatusPill>
        </div>
        <div className="preview-kv-grid">
          <div><span>用途</span><strong>{detail.purpose}</strong></div>
          <div><span>文件大小</span><strong>{detail.size}</strong></div>
          <div><span>规格</span><strong>{detail.dimensions || '—'}</strong></div>
          <div><span>上传人</span><strong>{detail.owner || '—'}</strong></div>
          <div><span>被引用</span><strong>{detail.referenceCount ?? 0} 处</strong></div>
          <div><span>上传时间</span><strong>{formatTime(detail.createdAt)}</strong></div>
        </div>

        <div className="preview-note-block">
          <span>审核记录</span>
          {decisions.filter(item => item.assetId === detail.id).length === 0 && <p>暂无审核记录</p>}
          {decisions.filter(item => item.assetId === detail.id).map(item => <p key={item.id}>
            {item.result} · {item.note} · {formatTime(item.decidedAt)}
          </p>)}
        </div>

        <div className="preview-note-block">
          <span>处理记录</span>
          {events.filter(item => item.assetId === detail.id).length === 0 && <p>暂无处理记录</p>}
          {events.filter(item => item.assetId === detail.id).map(item => <p key={item.id}>
            {item.action} · {item.status} · {formatTime(item.at)}
          </p>)}
        </div>

        <div className="preview-note-block">
          <span>清理任务</span>
          {gcJobs.filter(item => item.assetId === detail.id).length === 0 && <p>暂无清理任务</p>}
          {gcJobs.filter(item => item.assetId === detail.id).map(item => <p key={item.id}>
            {item.kind} · {item.status} · {formatTime(item.at)}
          </p>)}
        </div>
      </div>}
    </Drawer>

    {tab === 'decisions' && <Panel eyebrow="REVIEW DECISION" title="审核记录" description="记录每次审核的结论与依据，用于事后追溯。">
      <div className="preview-table-wrap"><table className="preview-table" aria-label="素材审核记录">
        <thead><tr><th>记录</th><th>素材</th><th>结论</th><th>依据</th><th>时间</th></tr></thead>
        <tbody>{decisions.map(item => <tr key={item.id}>
          <td><strong>{item.id}</strong></td><td>{item.assetId}</td>
          <td><StatusPill tone={item.result === '通过' ? 'success' : 'danger'}>{item.result}</StatusPill></td>
          <td className="preview-cell-wrap">{item.note}</td><td>{formatTime(item.decidedAt)}</td>
        </tr>)}</tbody>
      </table></div>
    </Panel>}

    {tab === 'events' && <Panel eyebrow="PROCESS EVENT" title="处理记录" description="上传、转码、投影等处理动作的结果。">
      <div className="preview-table-wrap"><table className="preview-table" aria-label="素材处理记录">
        <thead><tr><th>记录</th><th>素材</th><th>动作</th><th>结果</th><th>时间</th></tr></thead>
        <tbody>{events.map(item => <tr key={item.id}>
          <td><strong>{item.id}</strong></td><td>{item.assetId}</td><td>{item.action}</td>
          <td><StatusPill tone={item.status === '已完成' ? 'success' : 'danger'}>{item.status}</StatusPill></td>
          <td>{formatTime(item.at)}</td>
        </tr>)}</tbody>
      </table></div>
    </Panel>}

    {tab === 'gc' && <Panel eyebrow="CLEANUP JOB" title="清理记录" description="删除素材后由后台异步清理对象存储，失败会重试。">
      <div className="preview-table-wrap"><table className="preview-table" aria-label="素材清理记录">
        <thead><tr><th>任务</th><th>素材</th><th>清理内容</th><th>状态</th><th>时间</th></tr></thead>
        <tbody>{gcJobs.map(item => <tr key={item.id}>
          <td><strong>{item.id}</strong></td><td>{item.assetId}</td><td>{item.kind}</td>
          <td><StatusPill tone={item.status === '已完成' ? 'success' : 'warning'}>{item.status}</StatusPill></td>
          <td>{formatTime(item.at)}</td>
        </tr>)}</tbody>
      </table></div>
    </Panel>}

    {tab === 'deadletters' && <Panel eyebrow="DEAD LETTER" title="异常记录" description="失败且已停止重试的任务，需要人工介入。">
      {events.filter(item => item.status === '失败').length === 0 && gcJobs.filter(item => item.status === '待处理').length === 0
        ? <EmptyState title="没有异常记录" description="当前处理与清理任务均正常。" />
        : <div className="preview-table-wrap"><table className="preview-table" aria-label="素材异常记录">
          <thead><tr><th>类型</th><th>对象</th><th>说明</th><th>状态</th><th>时间</th></tr></thead>
          <tbody>
            {events.filter(item => item.status === '失败').map(item => <tr key={item.id}>
              <td>处理任务</td><td>{item.assetId}</td><td className="preview-cell-wrap">{item.action}</td>
              <td><StatusPill tone="danger">失败</StatusPill></td><td>{formatTime(item.at)}</td>
            </tr>)}
            {gcJobs.filter(item => item.status === '待处理').map(item => <tr key={item.id}>
              <td>清理任务</td><td>{item.assetId}</td><td className="preview-cell-wrap">{item.kind}</td>
              <td><StatusPill tone="warning">待处理</StatusPill></td><td>{formatTime(item.at)}</td>
            </tr>)}
          </tbody>
        </table></div>}
    </Panel>}
  </div>
}

function ReviewOperations() {
  const reviews = usePreviewStore(state => state.reviews)
  const decideReview = usePreviewStore(state => state.decideReview)
  const [status, setStatus] = useState<'全部' | ReviewItem['status']>('待审核')
  const [search, setSearch] = useState('')
  const [riskOnly, setRiskOnly] = useState(false)
  const [selected, setSelected] = useState<ReviewItem | null>(null)
  const [decision, setDecision] = useState<'approve' | 'reject'>('approve')
  const [reason, setReason] = useState('')
  const [error, setError] = useState('')
  const { notify } = usePreviewToast()

  const filtered = reviews.filter(item =>
    (!riskOnly || item.risk === '高风险')
    && (status === '全部' || item.status === status)
    && (!search.trim() || `${item.id}${item.title}${item.submitter}${item.kind}`.toLowerCase().includes(search.trim().toLowerCase()))
  )

  const openDecision = (item: ReviewItem, next: 'approve' | 'reject') => {
    setSelected(item)
    setDecision(next)
    setReason('')
    setError('')
  }
  const approveReview = (item: ReviewItem, explanation: string) => {
    decideReview(item.id, '已通过', explanation)
    setSelected(null)
    notify('审核已通过，内容将进入公开流程')
  }
  const rejectReview = (item: ReviewItem, explanation: string) => {
    decideReview(item.id, '已拒绝', explanation)
    setSelected(null)
    notify('审核已拒绝，原因已记录', 'info')
  }
  const submitDecision = () => {
    if (!selected) return
    if (!reason.trim()) {
      setError('请填写审核依据，便于后续追溯')
      return
    }
    if (decision === 'approve') approveReview(selected, reason.trim())
    else rejectReview(selected, reason.trim())
  }

  return <div>
    <PageHeader
      eyebrow={metaBySection.moderation.eyebrow}
      title={metaBySection.moderation.title}
      description={metaBySection.moderation.description}
      actions={<Button
        variant={riskOnly ? 'primary' : 'ghost'}
        onClick={() => { setRiskOnly(current => !current); setStatus('待审核') }}
      >{riskOnly ? '显示全部待审核' : metaBySection.moderation.action}</Button>}
    />
    <section className="preview-stat-grid">
      <StatCard label="待审核" value={reviews.filter(item => item.status === '待审核').length} meta="等待首次处理" tone="amber" />
      <StatCard label="高风险" value={reviews.filter(item => item.risk === '高风险').length} meta="建议优先处理" tone="rose" />
      <StatCard label="审核中" value={reviews.filter(item => item.status === '审核中').length} meta="正在核对资料" tone="accent" />
      <StatCard label="今日已处理" value={reviews.filter(item => item.status === '已通过' || item.status === '已拒绝').length} meta="可查看结论原因" tone="mint" />
    </section>
    <Panel
      eyebrow="REVIEW QUEUE"
      title="审核任务"
      description="通过或拒绝都必须填写原因，所有变化只写入本地预览。"
      actions={<SearchBox value={search} onChange={setSearch} placeholder="搜索任务、标题或提交人" />}
    >
      <div className="preview-toolbar">
        <Segmented value={status} onChange={setStatus} options={[
          { value: '待审核', label: '待审核' },
          { value: '审核中', label: '审核中' },
          { value: '已通过', label: '已通过' },
          { value: '已拒绝', label: '已拒绝' },
          { value: '全部', label: '全部' }
        ]} />
        <span className="preview-toolbar__hint">{filtered.length} 条任务</span>
      </div>
      {filtered.length === 0 ? <EmptyState title="当前没有审核任务" description="切换状态或调整搜索条件。" /> : <div className="preview-review-list">
        {filtered.map(item => <article key={item.id} className={`preview-review-row ${item.risk === '高风险' ? 'is-risk' : ''}`}>
          <span className="preview-review-row__kind">{item.kind}</span>
          <div className="preview-review-row__copy">
            <div><strong>{item.title}</strong><StatusPill>{item.risk}</StatusPill></div>
            <p>{item.id} · {item.submitter} · {item.source} · {formatTime(item.createdAt)}</p>
            {waitingLabel(item.createdAt, item.status) && <p className="preview-review-row__wait">{waitingLabel(item.createdAt, item.status)}</p>}
            {item.reason && <small>审核依据：{item.reason}</small>}
          </div>
          <StatusPill>{item.status}</StatusPill>
          <div className="preview-row-actions">
            {(item.status === '待审核' || item.status === '审核中') ? <>
              <Button size="sm" variant="primary" onClick={() => openDecision(item, 'approve')}>通过</Button>
              <Button size="sm" variant="danger" onClick={() => openDecision(item, 'reject')}>拒绝</Button>
            </> : <Button size="sm" variant="ghost" onClick={() => openDecision(item, 'approve')}>复核</Button>}
          </div>
        </article>)}
      </div>}
    </Panel>
    <Modal
      open={Boolean(selected)}
      title={decision === 'approve' ? '通过审核' : '拒绝审核'}
      description={selected ? `${selected.kind}：${selected.title}` : ''}
      onClose={() => setSelected(null)}
      footer={<>
        <Button onClick={() => setSelected(null)}>取消</Button>
        <Button variant={decision === 'reject' ? 'danger' : 'primary'} onClick={submitDecision}>{decision === 'approve' ? '确认通过' : '确认拒绝'}</Button>
      </>}
    >
      <div className="preview-form-stack">
        <div className="preview-warning-box">
          <strong>{decision === 'approve' ? '通过后会进入公开流程' : '拒绝后不会对外展示'}</strong>
          <p>此操作只会修改当前浏览器里的演示数据。</p>
        </div>
        <TextAreaField label="审核依据" required rows={5} value={reason} onChange={event => setReason(event.target.value)} placeholder="请填写核对结果、风险判断或拒绝原因" />
        {error && <div className="preview-form-error">{error}</div>}
      </div>
    </Modal>
  </div>
}

function CommunityOperations() {
  const community = usePreviewStore(state => state.community)
  const setCommunityStatus = usePreviewStore(state => state.setCommunityStatus)
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState<'全部' | CommunityItem['status']>('全部')
  const { notify } = usePreviewToast()
  const hideComment = usePreviewStore(state => state.hideComment)
  const [detailId, setDetailId] = useState<string | null>(null)
  const detail = community.find(item => item.id === detailId) || null
  const filtered = community.filter(item => (status === '全部' || item.status === status) && (!query || `${item.id}${item.title}${item.author}${item.topic}`.includes(query)))
  const paged = usePagedList(filtered, 8)
  return <div>
    <PageHeader eyebrow={metaBySection.community.eyebrow} title={metaBySection.community.title} description={metaBySection.community.description} actions={<Button variant="primary" onClick={() => setStatus('待审核')}>查看待审内容</Button>} />
    <section className="preview-stat-grid">
      <StatCard label="已公开" value={community.filter(item => item.status === '已公开').length} meta="展示在用户端" tone="mint" />
      <StatCard label="待审核" value={community.filter(item => item.status === '待审核').length} meta="进入统一审核流" tone="amber" />
      <StatCard label="已下架" value={community.filter(item => item.status === '已下架').length} meta="保留处理记录" tone="ink" />
      <StatCard label="今日新增" value={community.filter(item => isToday(item.createdAt)).length} meta="今天提交的内容" tone="accent" />
    </section>
    <Panel eyebrow="COMMUNITY FEED" title="社区内容" description="公开内容可以直接下架，待审内容统一进入审核中心。" actions={<SearchBox value={query} onChange={setQuery} placeholder="搜索标题、作者或话题" />}>
      <div className="preview-toolbar"><Segmented value={status} onChange={setStatus} options={[{ value: '全部', label: '全部' }, { value: '已公开', label: '已公开' }, { value: '待审核', label: '待审核' }, { value: '已下架', label: '已下架' }]} /></div>
      <div className="preview-table-wrap"><table className="preview-table" aria-label="内容列表"><thead><tr><th>内容</th><th>图片</th><th>作者</th><th>话题</th><th>状态</th><th>时间</th><th /></tr></thead><tbody>
        {paged.slice.map(item => <tr key={item.id}><td><button type="button" className="preview-table__primary" onClick={() => setDetailId(item.id)}>{item.title}<small>{item.id}</small></button></td><td><ImageCell count={item.imageCount ?? 0} seed={item.id} label={item.title} /></td><td>{item.author}</td><td>{item.topic}</td><td><StatusPill>{item.status}</StatusPill></td><td>{formatTime(item.createdAt)}</td><td>{item.status === '已公开' && <Button size="sm" variant="danger" onClick={() => { setCommunityStatus(item.id, '已下架'); notify('内容已下架，处理记录已保留', 'info') }}>下架</Button>}</td></tr>)}
      </tbody></table>{filtered.length === 0 && <EmptyState title="没有匹配内容" description="调整筛选条件后再试。" />}</div>
      <Pagination page={paged.page} pageSize={paged.pageSize} total={paged.total} onChange={paged.setPage} label="内容列表分页" />
    </Panel>

    <Drawer
      open={Boolean(detail)}
      onClose={() => setDetailId(null)}
      title={detail?.title || '帖子详情'}
      description={detail ? `${detail.author} · ${detail.topic} · ${formatTime(detail.createdAt)}` : undefined}
      footer={detail && <>
        <Button onClick={() => setDetailId(null)}>关闭</Button>
        {detail.status === '已公开'
          ? <Button variant="danger" onClick={() => { setCommunityStatus(detail.id, '已下架'); notify('内容已下架，处理记录已保留', 'info') }}>下架内容</Button>
          : detail.status === '已下架'
            ? <Button variant="primary" onClick={() => { setCommunityStatus(detail.id, '已公开'); notify('内容已恢复公开') }}>恢复公开</Button>
            : null}
      </>}
    >
      {detail && <div className="preview-detail-stack">
        <div className="preview-detail-banner">
          <div><span>当前状态</span><strong>{detail.status}</strong></div>
          <StatusPill>{detail.status}</StatusPill>
        </div>
        <div className="preview-kv-grid">
          <div><span>作者</span><strong>{detail.author}</strong></div>
          <div><span>话题</span><strong>{detail.topic}</strong></div>
          <div><span>图片</span><strong>{detail.imageCount ?? 0} 张</strong></div>
          <div><span>点赞</span><strong>{detail.likeCount ?? 0}</strong></div>
          <div><span>评论</span><strong>{(detail.comments || []).length} 条</strong></div>
          <div><span>举报</span><strong>{detail.reportCount ?? 0} 次</strong></div>
        </div>
        <div className="preview-note-block"><span>图片（{detail.imageCount ?? 0}）</span><ImageStrip count={detail.imageCount ?? 0} seed={detail.id} label={detail.title} /></div>
        <div className="preview-note-block"><span>正文</span><p>{detail.body || '（该内容没有正文）'}</p></div>
        <div className="preview-note-block">
          <span>评论（{(detail.comments || []).length}）</span>
          {(detail.comments || []).length === 0 && <p>暂无评论</p>}
          {(detail.comments || []).map(comment => <div key={comment.id} className={`preview-comment${comment.status === '已隐藏' ? ' is-hidden' : ''}`}>
            <div><strong>{comment.author}</strong><StatusPill tone={comment.status === '已隐藏' ? 'neutral' : 'success'}>{comment.status}</StatusPill></div>
            <p>{comment.content}</p>
            {comment.status === '正常' && <Button size="sm" variant="ghost" onClick={() => { hideComment(detail.id, comment.id); notify('评论已隐藏', 'info') }}>隐藏评论</Button>}
          </div>)}
        </div>
      </div>}
    </Drawer>
  </div>
}

function TradeOperations() {
  const products = usePreviewStore(state => state.products)
  const setProductStatus = usePreviewStore(state => state.setProductStatus)
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState<'全部' | ProductItem['status']>('全部')
  const [detailId, setDetailId] = useState<string | null>(null)
  const detail = products.find(item => item.id === detailId) || null
  const { notify } = usePreviewToast()
  const filtered = products.filter(item => (status === '全部' || item.status === status) && (!query || `${item.id}${item.title}${item.seller}${item.category}`.includes(query)))
  const paged2 = usePagedList(filtered, 8)
  return <div>
    <PageHeader eyebrow={metaBySection.trade.eyebrow} title={metaBySection.trade.title} description={metaBySection.trade.description} actions={<Button variant="primary" onClick={() => setStatus('出售中')}>查看出售中商品</Button>} />
    <section className="preview-stat-grid">
      <StatCard label="出售中" value={products.filter(item => item.status === '出售中').length} meta="用户可见" tone="mint" />
      <StatCard label="已预留" value={products.filter(item => item.status === '已预留').length} meta="正在沟通交易" tone="accent" />
      <StatCard label="已下架" value={products.filter(item => item.status === '已下架').length} meta="不再展示" tone="ink" />
      <StatCard label="在售金额" value={formatMoney(products.filter(item => item.status === '出售中').reduce((sum, item) => sum + item.priceCents, 0))} meta="商品标价合计" tone="ink" />
    </section>
    <Panel eyebrow="SECONDHAND MARKET" title="商品工作队列" description="审核状态与业务状态分离；下架只影响展示。" actions={<SearchBox value={query} onChange={setQuery} placeholder="搜索商品、卖家或分类" />}>
      <div className="preview-toolbar"><Segmented value={status} onChange={setStatus} options={[{ value: '全部', label: '全部' }, { value: '出售中', label: '出售中' }, { value: '已预留', label: '已预留' }, { value: '已下架', label: '已下架' }]} /></div>
      <div className="preview-table-wrap"><table className="preview-table" aria-label="商品列表"><thead><tr><th>商品</th><th>图片</th><th>卖家</th><th>分类</th><th>价格</th><th>状态</th><th>时间</th><th /></tr></thead><tbody>
        {paged2.slice.map(item => <tr key={item.id}><td><button type="button" className="preview-table__primary" onClick={() => setDetailId(item.id)}>{item.title}<small>{item.id}</small></button></td><td><ImageCell count={item.imageCount ?? 0} seed={item.id} label={item.title} /></td><td>{item.seller}</td><td>{item.category}</td><td><strong>{formatMoney(item.priceCents)}</strong></td><td><StatusPill>{item.status}</StatusPill></td><td>{formatTime(item.createdAt)}</td><td>{item.status !== '已下架' && <Button size="sm" variant="ghost" onClick={() => { setProductStatus(item.id, '已下架'); notify('商品已下架', 'info') }}>下架</Button>}</td></tr>)}
      </tbody></table>{filtered.length === 0 && <EmptyState title="没有匹配商品" description="调整筛选条件后再试。" />}</div>
      <Pagination page={paged2.page} pageSize={paged2.pageSize} total={paged2.total} onChange={paged2.setPage} label="商品列表分页" />
    </Panel>

    <Drawer
      open={Boolean(detail)}
      onClose={() => setDetailId(null)}
      title={detail?.title || '商品详情'}
      description={detail ? `${detail.seller} · ${detail.category} · ${formatTime(detail.createdAt)}` : undefined}
      footer={detail && <>
        <Button onClick={() => setDetailId(null)}>关闭</Button>
        {detail.status !== '已下架'
          ? <Button variant="danger" onClick={() => { setProductStatus(detail.id, '已下架'); notify('商品已下架', 'info') }}>下架商品</Button>
          : <Button variant="primary" onClick={() => { setProductStatus(detail.id, '出售中'); notify('商品已恢复出售') }}>恢复出售</Button>}
      </>}
    >
      {detail && <div className="preview-detail-stack">
        <div className="preview-detail-banner">
          <div><span>当前状态</span><strong>{detail.status}</strong></div>
          <StatusPill>{detail.status}</StatusPill>
        </div>
        <div className="preview-kv-grid">
          <div><span>卖家</span><strong>{detail.seller}</strong></div>
          <div><span>分类</span><strong>{detail.category}</strong></div>
          <div><span>价格</span><strong>{formatMoney(detail.priceCents)}</strong></div>
          <div><span>图片</span><strong>{detail.imageCount ?? 0} 张</strong></div>
          <div><span>收藏</span><strong>{detail.favoriteCount ?? 0}</strong></div>
          <div><span>举报</span><strong>{detail.reportCount ?? 0} 次</strong></div>
        </div>
        <div className="preview-note-block"><span>图片（{detail.imageCount ?? 0}）</span><ImageStrip count={detail.imageCount ?? 0} seed={detail.id} label={detail.title} /></div>
        <div className="preview-note-block"><span>商品描述</span><p>{detail.description || '（无描述）'}</p></div>
      </div>}
    </Drawer>
  </div>
}

function ReportOperations() {
  const reports = usePreviewStore(state => state.reports)
  const setReportStatus = usePreviewStore(state => state.setReportStatus)
  const [filter, setFilter] = useState<'全部' | ReportItem['status']>('全部')
  const [detailId, setDetailId] = useState<string | null>(null)
  const detail = reports.find(item => item.id === detailId) || null
  const { notify } = usePreviewToast()
  const filtered = reports.filter(item => filter === '全部' || item.status === filter)
  const paged3 = usePagedList(filtered, 8)
  return <div>
    <PageHeader eyebrow={metaBySection.reports.eyebrow} title={metaBySection.reports.title} description={metaBySection.reports.description} actions={<Button variant="primary" onClick={() => setFilter('待处理')}>处理待办举报</Button>} />
    <section className="preview-stat-grid">
      <StatCard label="待处理" value={reports.filter(item => item.status === '待处理').length} meta="按影响程度排序" tone="rose" />
      <StatCard label="处理中" value={reports.filter(item => item.status === '处理中').length} meta="已有管理员跟进" tone="amber" />
      <StatCard label="已解决" value={reports.filter(item => item.status === '已解决').length} meta="结论已记录" tone="mint" />
      <StatCard label="全部举报" value={reports.length} meta="当前演示数据" tone="accent" />
    </section>
    <Panel eyebrow="SAFETY QUEUE" title="举报处理" description="处理结论会同步到对应内容模块。">
      <div className="preview-toolbar"><Segmented value={filter} onChange={setFilter} options={[{ value: '全部', label: '全部' }, { value: '待处理', label: '待处理' }, { value: '处理中', label: '处理中' }, { value: '已解决', label: '已解决' }]} /></div>
      <div className="preview-table-wrap"><table className="preview-table" aria-label="举报列表"><thead><tr><th>举报目标</th><th>原因</th><th>举报人</th><th>状态</th><th>时间</th><th /></tr></thead><tbody>
        {paged3.slice.map(item => <tr key={item.id}><td><button type="button" className="preview-table__primary" onClick={() => setDetailId(item.id)}>{item.target}<small>{item.id}</small></button></td><td>{item.reason}</td><td>{item.reporter}</td><td><StatusPill>{item.status}</StatusPill></td><td>{formatTime(item.createdAt)}</td><td>{item.status !== '已解决' && <Button size="sm" variant="primary" onClick={() => { setReportStatus(item.id, '已解决'); notify('举报已标记为已解决') }}>完成处理</Button>}</td></tr>)}
      </tbody></table>{filtered.length === 0 && <EmptyState title="当前没有举报" description="切换状态后再试。" />}</div>
      <Pagination page={paged3.page} pageSize={paged3.pageSize} total={paged3.total} onChange={paged3.setPage} label="举报列表分页" />
    </Panel>

    <Drawer
      open={Boolean(detail)}
      onClose={() => setDetailId(null)}
      title={detail ? `举报 ${detail.id}` : '举报详情'}
      description={detail ? `${detail.target} · ${formatTime(detail.createdAt)}` : undefined}
      footer={detail && <>
        <Button onClick={() => setDetailId(null)}>关闭</Button>
        {detail.status !== '已解决' && <Button variant="primary" onClick={() => { setReportStatus(detail.id, '已解决'); notify('举报已标记为已解决'); }}>标记已解决</Button>}
      </>}
    >
      {detail && <div className="preview-detail-stack">
        <div className="preview-detail-banner">
          <div><span>当前状态</span><strong>{detail.status}</strong></div>
          <StatusPill>{detail.status}</StatusPill>
        </div>
        <div className="preview-kv-grid">
          <div><span>举报目标</span><strong>{detail.target}</strong></div>
          <div><span>举报人</span><strong>{detail.reporter}</strong></div>
          <div><span>提交时间</span><strong>{formatTime(detail.createdAt)}</strong></div>
        </div>
        <div className="preview-note-block"><span>举报原因</span><p>{detail.reason}</p></div>
      </div>}
    </Drawer>
  </div>
}
